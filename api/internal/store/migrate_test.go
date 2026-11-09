package store

import (
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mustVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	v, err := Version(db)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func file(sql string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(sql)} }

func TestFreshDatabaseMigratesToNewest(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("no embedded migrations")
	}
	db := openTemp(t)
	if v := mustVersion(t, db); v != 0 {
		t.Fatalf("fresh database version = %d, want 0", v)
	}
	if err := Apply(db, Pending(0, ms)); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != len(ms) {
		t.Fatalf("version = %d, want %d", v, len(ms))
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('title', 'x')"); err != nil {
		t.Fatalf("baseline settings table: %v", err)
	}
}

func TestEachEmbeddedMigrationFromPreviousVersion(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range ms {
		t.Run(m.Name, func(t *testing.T) {
			db := openTemp(t)
			if err := Apply(db, ms[:i]); err != nil {
				t.Fatal(err)
			}
			if v := mustVersion(t, db); v != m.Version-1 {
				t.Fatalf("previous version = %d, want %d", v, m.Version-1)
			}
			if err := Apply(db, Pending(m.Version-1, ms)[:1]); err != nil {
				t.Fatal(err)
			}
			if v := mustVersion(t, db); v != m.Version {
				t.Fatalf("version = %d, want %d", v, m.Version)
			}
		})
	}
}

func TestLoadMigrationsRejects(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"bad name no digits":  {"abcd_x.sql": file("SELECT 1")},
		"bad name short":      {"001_x.sql": file("SELECT 1")},
		"bad name dash":       {"0001-x.sql": file("SELECT 1")},
		"bad name upper":      {"0001_X.sql": file("SELECT 1")},
		"bad name extension":  {"0001_x.txt": file("SELECT 1")},
		"duplicate":           {"0001_a.sql": file("SELECT 1"), "0001_b.sql": file("SELECT 1")},
		"gap":                 {"0001_a.sql": file("SELECT 1"), "0003_c.sql": file("SELECT 1")},
		"does not start at 1": {"0002_b.sql": file("SELECT 1")},
		"zero":                {"0000_a.sql": file("SELECT 1")},
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadMigrations(fsys); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestLoadMigrationsSorts(t *testing.T) {
	ms, err := LoadMigrations(fstest.MapFS{
		"0002_b.sql": file("SELECT 2"),
		"0001_a.sql": file("SELECT 1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].Version != 1 || ms[1].Version != 2 || ms[1].SQL != "SELECT 2" {
		t.Fatalf("got %+v", ms)
	}
}

func TestPending(t *testing.T) {
	ms := []Migration{{Version: 1}, {Version: 2}, {Version: 3}}
	if got := Pending(0, ms); len(got) != 3 {
		t.Errorf("Pending(0) = %d migrations, want 3", len(got))
	}
	if got := Pending(2, ms); len(got) != 1 || got[0].Version != 3 {
		t.Errorf("Pending(2) = %+v, want [3]", got)
	}
	if got := Pending(3, ms); len(got) != 0 {
		t.Errorf("Pending(3) = %+v, want none", got)
	}
}

func TestApplyFailureRollsBackThatMigration(t *testing.T) {
	ms, err := LoadMigrations(fstest.MapFS{
		"0001_a.sql": file("CREATE TABLE a (x INTEGER);"),
		"0002_b.sql": file("CREATE TABLE b (x INTEGER); INSERT INTO a VALUES (1); INSERT INTO missing VALUES (1);"),
	})
	if err != nil {
		t.Fatal(err)
	}
	db := openTemp(t)
	if err := Apply(db, ms); err == nil {
		t.Fatal("want error from failing migration 2")
	}
	if v := mustVersion(t, db); v != 1 {
		t.Fatalf("version = %d, want 1", v)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'b'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("table b exists after rollback (count %d, err %v)", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM a").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows in a after rollback = %d (err %v), want 0", n, err)
	}
}

func TestCheckVersion(t *testing.T) {
	if err := CheckVersion(2, 2); err != nil {
		t.Errorf("CheckVersion(2, 2) = %v, want nil", err)
	}
	if err := CheckVersion(1, 2); err != nil {
		t.Errorf("CheckVersion(1, 2) = %v, want nil", err)
	}
	if err := CheckVersion(3, 2); !errors.Is(err, ErrNewerDatabase) {
		t.Errorf("CheckVersion(3, 2) = %v, want ErrNewerDatabase", err)
	}
}

// TestMigration0002FromVersion1 applies 0002_auth to a version 1 database holding data and
// checks the data survives and the auth tables enforce their constraints.
func TestMigration0002FromVersion1(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 2 {
		t.Fatal("migration 0002 missing")
	}
	db := openTemp(t)
	if err := Apply(db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO settings (key, value) VALUES ('title', 'kept')"); err != nil {
		t.Fatal(err)
	}
	if err := Apply(db, ms[1:2]); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
	var title string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'title'").Scan(&title); err != nil || title != "kept" {
		t.Fatalf("settings row = %q (err %v), want kept", title, err)
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustFail := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err == nil {
			t.Fatalf("%s: want constraint error", q)
		}
	}
	mustExec("INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'owner', 'h', 1)")
	mustFail("INSERT INTO users (username, password_hash, created_at) VALUES ('OWNER', 'h', 1)")
	mustExec("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES ('t', x'01', 'read', 1)")
	mustExec("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES ('t', x'02', 'write', 1)")
	mustFail("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES ('t', x'03', 'admin', 1)")
	mustFail("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES ('t', x'01', 'read', 1)")
	mustExec("INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at) VALUES (x'aa', 1, 1, 1, 2)")
	mustFail("INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at) VALUES (x'bb', 99, 1, 1, 2)")
	mustExec("DELETE FROM users WHERE id = 1")
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sessions").Scan(&n); err != nil || n != 0 {
		t.Fatalf("sessions after user delete = %d (err %v), want 0", n, err)
	}
}
