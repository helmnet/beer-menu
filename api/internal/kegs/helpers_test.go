package kegs

import (
	"database/sql"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

// openDB returns a migrated database in a temporary folder. Migration 0004 seeds one tap.
func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(db, ms); err != nil {
		t.Fatal(err)
	}
	return db
}

// exec runs q, failing t on error.
func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// insertBeer adds a beer named name, with srm when not nil, and returns its id.
func insertBeer(t *testing.T, db *sql.DB, name string, srm any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("INSERT INTO beers (name, srm, created_at, updated_at) VALUES (?, ?, ?, ?) RETURNING id", name, srm, t0.Unix(), t0.Unix()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// insertKeg adds an 18.93 L keg of beer on tap (nil for waiting), kicked at kicked (nil for
// current), and returns its id.
func insertKeg(t *testing.T, db *sql.DB, beer int64, tap, kicked any) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, kicked_at, created_at, updated_at) VALUES (?, 18.93, 18.93, ?, ?, ?, ?) RETURNING id",
		beer, tap, kicked, t0.Unix(), t0.Unix()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func ptr[T any](v T) *T { return &v }
