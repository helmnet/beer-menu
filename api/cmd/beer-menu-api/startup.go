package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/helmnet/beer-menu/api/internal/store"
)

// keepBackups is how many app-made backups stay in the backups folder.
const keepBackups = 5

// config is the runtime configuration read from the environment.
type config struct {
	DataDir string
	Port    string
}

// startup prepares the database before the API serves requests: it checks the data folder
// is writable, opens the database, refuses a database newer than migrationsFS, backs it up
// when it has data and migrations are pending, then applies them. It returns the open
// database, or an error with the database closed. A failed backup applies nothing.
func startup(ctx context.Context, cfg config, migrationsFS fs.FS, now func() time.Time) (*sql.DB, error) {
	if err := store.CheckWritable(cfg.DataDir); err != nil {
		return nil, err
	}
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "database opened", "path", filepath.Join(cfg.DataDir, store.DBFile))
	if err := migrate(ctx, db, cfg.DataDir, migrationsFS, now); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(ctx context.Context, db *sql.DB, dataDir string, migrationsFS fs.FS, now func() time.Time) error {
	ms, err := store.LoadMigrations(migrationsFS)
	if err != nil {
		return err
	}
	current, err := store.Version(db)
	if err != nil {
		return err
	}
	newest := len(ms) // LoadMigrations guarantees versions 1..len(ms)
	slog.InfoContext(ctx, "schema version", "current", current, "newest", newest)
	if err := store.CheckVersion(current, newest); err != nil {
		return err
	}
	if current == newest {
		return nil
	}
	// A version 0 database is brand new and has no data to protect.
	if current > 0 {
		path, err := store.Backup(db, dataDir, current, now())
		if err != nil {
			return fmt.Errorf("backup before migrating, no migration applied: %w", err)
		}
		slog.InfoContext(ctx, "database backed up", "path", path)
		if err := store.PruneBackups(filepath.Dir(path), keepBackups); err != nil {
			return err
		}
	}
	pending := store.Pending(current, ms)
	if err := store.Apply(db, pending); err != nil {
		return err
	}
	slog.InfoContext(ctx, "migrations applied", "from", current, "to", newest, "count", len(pending))
	return nil
}
