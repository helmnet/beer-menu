package settings

import (
	"database/sql"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)

// openDB returns a database migrated with the store migrations in a temporary folder.
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

// updatedAt returns the updated_at of key, failing t if the row is missing.
func updatedAt(t *testing.T, db *sql.DB, key string) int64 {
	t.Helper()
	var u int64
	if err := db.QueryRow("SELECT updated_at FROM settings WHERE key = ?", key).Scan(&u); err != nil {
		t.Fatalf("updated_at of %s: %v", key, err)
	}
	return u
}

func TestGetSet(t *testing.T) {
	db := openDB(t)
	if v, ok, err := Get(db, "missing"); err != nil || ok || v != "" {
		t.Fatalf("missing = %q %v (err %v)", v, ok, err)
	}
	if err := Set(db, "a.b", "one", t0); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := Get(db, "a.b"); err != nil || !ok || v != "one" {
		t.Fatalf("after set = %q %v (err %v), want one", v, ok, err)
	}
	if err := Set(db, "a.b", "two", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := Get(db, "a.b"); err != nil || !ok || v != "two" {
		t.Fatalf("after replace = %q %v (err %v), want two", v, ok, err)
	}
	if u := updatedAt(t, db, "a.b"); u != t0.Add(time.Hour).Unix() {
		t.Fatalf("updated_at = %d, want %d", u, t0.Add(time.Hour).Unix())
	}
}

func TestSetInRolledBackTx(t *testing.T) {
	db := openDB(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := Set(tx, "a.b", "one", t0); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := Get(tx, "a.b"); err != nil || !ok || v != "one" {
		t.Fatalf("inside tx = %q %v (err %v)", v, ok, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if v, ok, err := Get(db, "a.b"); err != nil || ok || v != "" {
		t.Fatalf("after rollback = %q %v (err %v), want nothing", v, ok, err)
	}
}
