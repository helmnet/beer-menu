package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

//go:embed migrations/*.sql
var embedded embed.FS

// Migrations holds the embedded migration files at its root, ready for LoadMigrations.
// fs.Sub only fails on an invalid path name, and "migrations" is valid.
var Migrations, _ = fs.Sub(embedded, "migrations")

// ErrNewerDatabase means the database schema version is newer than the newest migration
// this build knows, so the database came from a later release and must not be touched.
var ErrNewerDatabase = errors.New("database is newer than this release")

var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9]+(?:_[a-z0-9]+)*\.sql$`)

// Migration is one numbered schema change.
type Migration struct {
	// Version is the NNNN number from the file name.
	Version int
	// Name is the file name, used in error messages.
	Name string
	// SQL is the file's contents, run in one transaction.
	SQL string
}

// LoadMigrations reads every file at the root of fsys as a migration named
// NNNN_snake_name.sql and returns them sorted by version. It rejects bad names,
// duplicate versions and gaps: the set must be numbered 1..max.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var ms []Migration
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			return nil, fmt.Errorf("bad migration name %q: want NNNN_snake_name.sql", e.Name())
		}
		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		v, _ := strconv.Atoi(m[1]) // the regexp guarantees four digits
		ms = append(ms, Migration{Version: v, Name: e.Name(), SQL: string(body)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Version < ms[j].Version })
	for i, m := range ms {
		if i > 0 && m.Version == ms[i-1].Version {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", m.Version, ms[i-1].Name, m.Name)
		}
		if m.Version != i+1 {
			return nil, fmt.Errorf("migration %s: want version %d, migrations must be numbered 1..max with no gaps", m.Name, i+1)
		}
	}
	return ms, nil
}

// Version returns the database schema version stored in PRAGMA user_version.
func Version(db *sql.DB) (int, error) {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return v, nil
}

// Pending returns the migrations in ms with a version above current, in order.
// ms must be sorted, as LoadMigrations returns it.
func Pending(current int, ms []Migration) []Migration {
	for i, m := range ms {
		if m.Version > current {
			return ms[i:]
		}
	}
	return nil
}

// CheckVersion returns an error wrapping ErrNewerDatabase when current is greater than
// newest, the highest migration version this build knows.
func CheckVersion(current, newest int) error {
	if current > newest {
		return fmt.Errorf("%w: database version %d, newest known migration %d", ErrNewerDatabase, current, newest)
	}
	return nil
}

// Apply runs each migration in its own transaction together with setting
// PRAGMA user_version to its version, so each applies fully or not at all. It stops at
// the first failure, leaving that migration rolled back and earlier ones applied.
func Apply(db *sql.DB, ms []Migration) error {
	for _, m := range ms {
		if err := apply(db, m); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.Name, err)
		}
	}
	return nil
}

func apply(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(m.SQL); err != nil {
		return err
	}
	// PRAGMA cannot take bound parameters; the version is an int, not user input.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.Version)); err != nil {
		return err
	}
	return tx.Commit()
}
