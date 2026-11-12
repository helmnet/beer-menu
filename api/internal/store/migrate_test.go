package store

import (
	"database/sql"
	"errors"
	"strings"
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

// TestMigration0003FromVersion2 applies 0003_beers to a version 2 database holding an admin and
// an API token, checks both survive unchanged, and that beers takes its defaults and enforces
// its enum CHECKs.
func TestMigration0003FromVersion2(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 3 {
		t.Fatal("migration 0003 missing")
	}
	db := openTemp(t)
	if err := Apply(db, ms[:2]); err != nil {
		t.Fatal(err)
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec("INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'owner', 'h', 1)")
	mustExec("INSERT INTO api_tokens (id, name, token_hash, scope, created_at, last_used_at) VALUES (1, 'ha', x'01', 'write', 2, 3)")
	if err := Apply(db, ms[2:3]); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != 3 {
		t.Fatalf("version = %d, want 3", v)
	}
	var user, hash string
	var created int64
	if err := db.QueryRow("SELECT username, password_hash, created_at FROM users WHERE id = 1").Scan(&user, &hash, &created); err != nil || user != "owner" || hash != "h" || created != 1 {
		t.Fatalf("admin row = %q %q %d (err %v)", user, hash, created, err)
	}
	var name, scope string
	var tokCreated, lastUsed int64
	if err := db.QueryRow("SELECT name, scope, created_at, last_used_at FROM api_tokens WHERE id = 1").Scan(&name, &scope, &tokCreated, &lastUsed); err != nil || name != "ha" || scope != "write" || tokCreated != 2 || lastUsed != 3 {
		t.Fatalf("token row = %q %q %d %d (err %v)", name, scope, tokCreated, lastUsed, err)
	}

	mustExec("INSERT INTO beers (name, created_at, updated_at) VALUES ('Pale', 1, 1)")
	var status, source string
	if err := db.QueryRow("SELECT status, source FROM beers WHERE name = 'Pale'").Scan(&status, &source); err != nil || status != "ready" || source != "manual" {
		t.Fatalf("defaults = %q %q (err %v), want ready manual", status, source, err)
	}
	for _, q := range []string{
		"INSERT INTO beers (name, status, created_at, updated_at) VALUES ('x', 'brewing', 1, 1)",
		"INSERT INTO beers (name, source, created_at, updated_at) VALUES ('x', 'x', 1, 1)",
	} {
		if _, err := db.Exec(q); err == nil {
			t.Fatalf("%s: want CHECK error", q)
		}
	}
}

// TestMigration0004FromVersion3 applies 0004_taps_kegs to a version 3 database holding an admin,
// an API token and a beer, checks they survive, the seeded tap exists, and kegs enforces one
// current keg per tap and its foreign keys.
func TestMigration0004FromVersion3(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 4 {
		t.Fatal("migration 0004 missing")
	}
	db := openTemp(t)
	if err := Apply(db, ms[:3]); err != nil {
		t.Fatal(err)
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
	mustExec("INSERT INTO api_tokens (id, name, token_hash, scope, created_at, last_used_at) VALUES (1, 'ha', x'01', 'write', 2, 3)")
	mustExec("INSERT INTO beers (id, name, abv, created_at, updated_at) VALUES (1, 'Pale', 5.5, 4, 5)")
	if err := Apply(db, ms[3:4]); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != 4 {
		t.Fatalf("version = %d, want 4", v)
	}
	var user, hash string
	var created int64
	if err := db.QueryRow("SELECT username, password_hash, created_at FROM users WHERE id = 1").Scan(&user, &hash, &created); err != nil || user != "owner" || hash != "h" || created != 1 {
		t.Fatalf("admin row = %q %q %d (err %v)", user, hash, created, err)
	}
	var name, scope string
	var tokCreated, lastUsed int64
	if err := db.QueryRow("SELECT name, scope, created_at, last_used_at FROM api_tokens WHERE id = 1").Scan(&name, &scope, &tokCreated, &lastUsed); err != nil || name != "ha" || scope != "write" || tokCreated != 2 || lastUsed != 3 {
		t.Fatalf("token row = %q %q %d %d (err %v)", name, scope, tokCreated, lastUsed, err)
	}
	var beer string
	var abv float64
	var bCreated, bUpdated int64
	if err := db.QueryRow("SELECT name, abv, created_at, updated_at FROM beers WHERE id = 1").Scan(&beer, &abv, &bCreated, &bUpdated); err != nil || beer != "Pale" || abv != 5.5 || bCreated != 4 || bUpdated != 5 {
		t.Fatalf("beer row = %q %v %d %d (err %v)", beer, abv, bCreated, bUpdated, err)
	}

	var n, tapID, position, enabled int64
	var label string
	if err := db.QueryRow("SELECT count(*), max(id), max(position), max(label), max(enabled) FROM taps").Scan(&n, &tapID, &position, &label, &enabled); err != nil || n != 1 || position != 1 || label != "Tap 1" || enabled != 1 {
		t.Fatalf("taps = %d rows, position %d, label %q, enabled %d (err %v)", n, position, label, enabled, err)
	}

	mustExec("INSERT INTO kegs (id, beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (1, 1, 18.93, 18.93, ?, 1, 1)", tapID)
	var kegType string
	if err := db.QueryRow("SELECT type FROM kegs WHERE id = 1").Scan(&kegType); err != nil || kegType != "corny_5_gal" {
		t.Fatalf("keg type = %q (err %v), want corny_5_gal", kegType, err)
	}
	var idx string
	if _, err := db.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (1, 1, 1, ?, 1, 1)", tapID); err == nil || !strings.Contains(err.Error(), "kegs.tap_id") {
		t.Fatalf("second current keg on a tap: err %v, want unique kegs.tap_id", err)
	}
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'kegs_one_per_tap'").Scan(&idx); err != nil {
		t.Fatalf("index kegs_one_per_tap: %v", err)
	}
	mustExec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, kicked_at, created_at, updated_at) VALUES (1, 1, 1, ?, 2, 1, 1)", tapID)
	mustExec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, kicked_at, created_at, updated_at) VALUES (1, 1, 1, ?, 3, 1, 1)", tapID)
	mustExec("INSERT INTO kegs (beer_id, size_l, remaining_l, created_at, updated_at) VALUES (1, 1, 1, 1, 1)")
	mustExec("INSERT INTO kegs (beer_id, size_l, remaining_l, created_at, updated_at) VALUES (1, 1, 1, 1, 1)")
	mustFail("DELETE FROM beers WHERE id = 1")

	mustExec("DELETE FROM kegs WHERE id = 1")
	mustExec("DELETE FROM taps WHERE id = ?", tapID)
	if err := db.QueryRow("SELECT count(*) FROM kegs WHERE kicked_at IS NOT NULL AND tap_id IS NULL").Scan(&n); err != nil || n != 2 {
		t.Fatalf("kicked kegs with tap_id NULL after tap delete = %d (err %v), want 2", n, err)
	}
}

// TestMigration0005FromVersion4 applies 0005_brewfather to a version 4 database holding two
// manual beers (one with a kicked keg) and a tap, checks they survive with empty sync state,
// the new tables exist with their columns and checks, and beers_source_ref allows one beer per
// imported batch.
func TestMigration0005FromVersion4(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 5 {
		t.Fatal("migration 0005 missing")
	}
	db := openTemp(t)
	if err := Apply(db, ms[:4]); err != nil {
		t.Fatal(err)
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
	mustExec("INSERT INTO settings (key, value) VALUES ('title', 'kept')")
	mustExec(`INSERT INTO beers (id, name, style, abv, ibu, srm, description, brewer, brew_date, status, archived_at, created_at, updated_at)
		VALUES (1, 'Pale', 'APA', 5.5, 35, 6, 'Hoppy', 'Helm', '2026-10-01', 'fermenting', NULL, 4, 5)`)
	mustExec("INSERT INTO beers (id, name, archived_at, created_at, updated_at) VALUES (2, 'Old', 9, 6, 7)")
	mustExec("INSERT INTO kegs (id, beer_id, size_l, remaining_l, tap_id, kicked_at, created_at, updated_at) VALUES (1, 2, 19, 0, NULL, 8, 1, 1)")
	if err := Apply(db, ms[4:5]); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != 5 {
		t.Fatalf("version = %d, want 5", v)
	}

	type row struct {
		name, style, desc, brewer, date, status, source string
		abv, srm                                        float64
		ibu, created, updated                           int64
	}
	var got row
	var archived, synced sql.NullInt64
	var notFound, pending, batchArchived int64
	if err := db.QueryRow(`SELECT name, style, description, brewer, brew_date, status, source, abv, srm, ibu, created_at, updated_at,
		archived_at, synced_at, sync_not_found, sync_archive_pending, sync_batch_archived FROM beers WHERE id = 1`).Scan(
		&got.name, &got.style, &got.desc, &got.brewer, &got.date, &got.status, &got.source, &got.abv, &got.srm, &got.ibu,
		&got.created, &got.updated, &archived, &synced, &notFound, &pending, &batchArchived); err != nil {
		t.Fatal(err)
	}
	if want := (row{"Pale", "APA", "Hoppy", "Helm", "2026-10-01", "fermenting", "manual", 5.5, 6, 35, 4, 5}); got != want || archived.Valid {
		t.Fatalf("beer 1 = %+v archived %v, want %+v", got, archived, want)
	}
	if synced.Valid || notFound != 0 || pending != 0 || batchArchived != 0 {
		t.Fatalf("beer 1 sync state = %v %d %d %d, want NULL 0 0 0", synced, notFound, pending, batchArchived)
	}
	var name string
	if err := db.QueryRow("SELECT name, archived_at, synced_at, sync_not_found, sync_archive_pending, sync_batch_archived FROM beers WHERE id = 2").Scan(
		&name, &archived, &synced, &notFound, &pending, &batchArchived); err != nil || name != "Old" || archived.Int64 != 9 ||
		synced.Valid || notFound != 0 || pending != 0 || batchArchived != 0 {
		t.Fatalf("beer 2 = %q %v %v %d %d %d (err %v)", name, archived, synced, notFound, pending, batchArchived, err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM kegs WHERE id = 1 AND beer_id = 2 AND kicked_at = 8").Scan(&n); err != nil || n != 1 {
		t.Fatalf("kicked keg kept = %d (err %v)", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM taps").Scan(&n); err != nil || n != 1 {
		t.Fatalf("taps = %d (err %v), want 1", n, err)
	}
	var title string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'title'").Scan(&title); err != nil || title != "kept" {
		t.Fatalf("settings row = %q (err %v), want kept", title, err)
	}

	wantColumns := map[string]string{
		"settings":             "key,value,updated_at",
		"provider_connections": "provider,user_id,api_key_sealed,connected_at",
		"sync_runs":            "provider,started_at,finished_at,ok,message,updated,imported,not_found",
		"import_dismissed":     "source,ref,dismissed_at",
	}
	for table, want := range wantColumns {
		rows, err := db.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
		if err != nil {
			t.Fatal(err)
		}
		var cols []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, c)
		}
		rows.Close()
		if got := strings.Join(cols, ","); got != want {
			t.Errorf("%s columns = %q, want %q", table, got, want)
		}
	}

	mustExec("INSERT INTO settings (key, value, updated_at) VALUES ('brewfather.sync_interval', '1h', 10)")
	mustExec("INSERT INTO provider_connections (provider, user_id, api_key_sealed, connected_at) VALUES ('brewfather', 'u', 'v1:x', 1)")
	mustFail("INSERT INTO provider_connections (provider, user_id, api_key_sealed, connected_at) VALUES ('other', 'u', 'v1:x', 1)")
	mustFail("INSERT INTO provider_connections (provider, user_id, api_key_sealed, connected_at) VALUES ('brewfather', 'u2', 'v1:y', 1)")
	mustExec("INSERT INTO sync_runs (provider, started_at) VALUES ('brewfather', 1)")
	if err := db.QueryRow("SELECT updated + imported + not_found FROM sync_runs").Scan(&n); err != nil || n != 0 {
		t.Fatalf("sync_runs count defaults = %d (err %v), want 0", n, err)
	}
	mustExec("INSERT INTO import_dismissed (source, ref, dismissed_at) VALUES ('brewfather', 'b1', 1)")
	mustFail("INSERT INTO import_dismissed (source, ref, dismissed_at) VALUES ('brewfather', 'b1', 2)")

	mustExec("INSERT INTO beers (name, source, source_ref, created_at, updated_at) VALUES ('A', 'brewfather', 'batch-1', 1, 1)")
	if _, err := db.Exec("INSERT INTO beers (name, source, source_ref, created_at, updated_at) VALUES ('B', 'brewfather', 'batch-1', 1, 1)"); err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Fatalf("second beer for batch-1: err %v, want unique beers_source_ref", err)
	}
	mustExec("INSERT INTO beers (name, source, source_ref, created_at, updated_at) VALUES ('C', 'brewfather', 'batch-2', 1, 1)")
	mustExec("INSERT INTO beers (name, source, created_at, updated_at) VALUES ('D', 'brewfather', 1, 1)")
	mustExec("INSERT INTO beers (name, source, created_at, updated_at) VALUES ('E', 'brewfather', 1, 1)")
	var idx string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'beers_source_ref'").Scan(&idx); err != nil {
		t.Fatalf("index beers_source_ref: %v", err)
	}
}

// TestMigration0006FromVersion5 applies 0006_two_factor to a version 5 database holding an
// admin, a session, a token and settings rows, checks they survive with no confirm stamp, and
// that the four 2FA tables exist with their checks, uniqueness and cascades from users.
func TestMigration0006FromVersion5(t *testing.T) {
	ms, err := LoadMigrations(Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 6 {
		t.Fatal("migration 0006 missing")
	}
	db := openTemp(t)
	if err := Apply(db, ms[:5]); err != nil {
		t.Fatal(err)
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
	mustExec("INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at) VALUES (x'01', 1, 2, 3, 4)")
	mustExec("INSERT INTO api_tokens (id, name, token_hash, scope, created_at) VALUES (1, 'ha', x'02', 'read', 5)")
	mustExec("INSERT INTO settings (key, value, updated_at) VALUES ('title', 'kept', 6)")
	if err := Apply(db, ms[5:6]); err != nil {
		t.Fatal(err)
	}
	if v := mustVersion(t, db); v != 6 {
		t.Fatalf("version = %d, want 6", v)
	}

	var n int
	var confirmed sql.NullInt64
	if err := db.QueryRow("SELECT user_id, confirmed_at FROM sessions WHERE id_hash = x'01' AND created_at = 2 AND last_seen_at = 3 AND expires_at = 4").
		Scan(&n, &confirmed); err != nil || n != 1 || confirmed.Valid {
		t.Fatalf("old session = user %d confirmed %v (err %v), want user 1, NULL", n, confirmed, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM users WHERE id = 1 AND username = 'owner' AND password_hash = 'h'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("admin kept = %d (err %v)", n, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM api_tokens WHERE id = 1 AND name = 'ha' AND scope = 'read'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("token kept = %d (err %v)", n, err)
	}
	var title string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'title'").Scan(&title); err != nil || title != "kept" {
		t.Fatalf("settings row = %q (err %v), want kept", title, err)
	}

	wantColumns := map[string]string{
		"sessions":        "id_hash,user_id,created_at,last_seen_at,expires_at,confirmed_at",
		"totp_factors":    "user_id,secret_sealed,last_step,created_at,last_used_at",
		"passkeys":        "id,user_id,name,credential_id,public_key,sign_count,transports,backup_eligible,created_at,last_used_at",
		"recovery_codes":  "user_id,code_hash,used_at",
		"auth_challenges": "id_hash,user_id,kind,webauthn,totp_pending_sealed,created_at,expires_at",
	}
	for table, want := range wantColumns {
		rows, err := db.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
		if err != nil {
			t.Fatal(err)
		}
		var cols []string
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				t.Fatal(err)
			}
			cols = append(cols, c)
		}
		rows.Close()
		if got := strings.Join(cols, ","); got != want {
			t.Errorf("%s columns = %q, want %q", table, got, want)
		}
	}
	var idx string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'passkeys_user_id'").Scan(&idx); err != nil {
		t.Fatalf("index passkeys_user_id: %v", err)
	}

	mustExec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (1, 'v1:x', 0, 1)")
	mustFail("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (1, 'v1:y', 0, 1)")
	mustExec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (1, 'Phone', x'aa', x'bb', 0, '[]', 1, 1)`)
	mustFail(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (1, 'Laptop', x'aa', x'cc', 0, '[]', 0, 1)`)
	mustExec("INSERT INTO recovery_codes (user_id, code_hash) VALUES (1, x'10')")
	mustFail("INSERT INTO recovery_codes (user_id, code_hash) VALUES (1, x'10')")
	mustFail("INSERT INTO recovery_codes (user_id, code_hash) VALUES (99, x'11')")
	for _, kind := range []string{"second_factor", "enroll", "setup"} {
		mustExec("INSERT INTO auth_challenges (id_hash, user_id, kind, created_at, expires_at) VALUES (?, 1, ?, 1, 301)", []byte(kind), kind)
	}
	mustFail("INSERT INTO auth_challenges (id_hash, user_id, kind, created_at, expires_at) VALUES (x'20', 1, 'other', 1, 301)")

	mustExec("DELETE FROM users WHERE id = 1")
	for _, table := range []string{"totp_factors", "passkeys", "recovery_codes", "auth_challenges", "sessions"} {
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s rows after deleting the admin = %d (err %v), want 0", table, n, err)
		}
	}
}
