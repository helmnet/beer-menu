package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/secrets"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// keepBackups is how many app-made backups stay in the backups folder.
const keepBackups = 5

// config is the runtime configuration read from the environment.
// It holds the first-admin password and the encryption key, so it must never be logged.
type config struct {
	DataDir      string
	Port         string
	CookieSecure bool
	// AdminUsername and AdminPassword create the first admin at startup when both are set
	// and no admin exists (research R9).
	AdminUsername string
	AdminPassword string
	// SecretKey seals third-party credentials; nil means Brewfather import is not configured.
	SecretKey []byte
	// PublicOrigin and RPID come from BEER_MENU_PUBLIC_URL; empty means passkeys are off.
	PublicOrigin string
	RPID         string
}

// loadSecretKey reads the encryption key from the file named by BEER_MENU_SECRET_KEY_FILE,
// which wins, or from BEER_MENU_SECRET_KEY: standard base64 of 32 bytes, surrounding
// whitespace ignored. Neither set gives nil. A set variable whose file cannot be read or whose
// value is not a valid key is an error naming the variable, never the value.
func loadSecretKey(getenv func(string) string, readFile func(string) ([]byte, error)) ([]byte, error) {
	if path := getenv("BEER_MENU_SECRET_KEY_FILE"); path != "" {
		raw, err := readFile(path)
		if err != nil {
			return nil, fmt.Errorf("read BEER_MENU_SECRET_KEY_FILE: %w", err)
		}
		key, err := secrets.ParseKey(string(raw))
		if err != nil {
			return nil, fmt.Errorf("BEER_MENU_SECRET_KEY_FILE: %w", err)
		}
		return key, nil
	}
	if v := getenv("BEER_MENU_SECRET_KEY"); v != "" {
		key, err := secrets.ParseKey(v)
		if err != nil {
			return nil, fmt.Errorf("BEER_MENU_SECRET_KEY: %w", err)
		}
		return key, nil
	}
	return nil, nil
}

// errPublicURL refuses startup for a bad BEER_MENU_PUBLIC_URL. It never holds the value.
var errPublicURL = errors.New("BEER_MENU_PUBLIC_URL must be https://<host name>[:port] (or http://localhost[:port]) with no path, query or fragment")

// loadPublicURL reads BEER_MENU_PUBLIC_URL, the address admins open the app at, and returns
// its origin (lowercase scheme and host, port kept) and its host name as the WebAuthn RP ID.
// Unset gives ("", "", nil): passkeys are off. A set value must be https with a host name, not
// an IP address, or http on localhost, with no user info, path beyond "/", query or fragment;
// anything else is an error that never contains the value (research R5), so a typo stops
// startup instead of silently turning passkeys off.
func loadPublicURL(getenv func(string) string) (origin, rpID string, err error) {
	v := getenv("BEER_MENU_PUBLIC_URL")
	if v == "" {
		return "", "", nil
	}
	u, err := url.Parse(v)
	// url.Parse errors quote the value, so they are dropped.
	if err != nil || u.Opaque != "" || u.User != nil || (u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(v, "#") {
		return "", "", errPublicURL
	}
	host := strings.ToLower(u.Hostname())
	scheme := strings.ToLower(u.Scheme)
	if host == "" || net.ParseIP(host) != nil || !(scheme == "https" || (scheme == "http" && host == "localhost")) {
		return "", "", errPublicURL
	}
	return scheme + "://" + strings.ToLower(u.Host), host, nil
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

// prepareAuth runs after startup has migrated db to the newest schema and before the API
// serves requests. It deletes expired sessions, then handles the first-admin settings: with
// neither set it does nothing; with both set and no admin it creates that admin; with an
// admin present it ignores them. Only one set, or values that break the username or
// password rules, return an error so the API exits. Errors and logs never hold the password.
func prepareAuth(ctx context.Context, db *sql.DB, cfg config, now func() time.Time) error {
	if err := auth.SweepExpired(db, now()); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	if cfg.AdminUsername == "" && cfg.AdminPassword == "" {
		return nil
	}
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		return errors.New("set both BEER_MENU_ADMIN_USERNAME and BEER_MENU_ADMIN_PASSWORD, or neither")
	}
	admin, err := auth.CreateFirstAdmin(db, cfg.AdminUsername, cfg.AdminPassword, now())
	if errors.Is(err, auth.ErrSetupDone) {
		slog.InfoContext(ctx, "first admin settings ignored, an admin exists")
		return nil
	}
	if err != nil {
		return fmt.Errorf("first admin from BEER_MENU_ADMIN_USERNAME and BEER_MENU_ADMIN_PASSWORD: %w", err)
	}
	slog.InfoContext(ctx, "first admin created", "username", admin.Username)
	return nil
}
