package auth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/store"
)

var t0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// openDB returns a migrated database in a temporary folder.
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

// insertUser adds a user row with a placeholder hash and returns its ID.
func insertUser(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	res, err := db.Exec("INSERT INTO users (username, password_hash, created_at) VALUES (?, 'x', ?)", username, t0.Unix())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func countRows(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func mustCreateSession(t *testing.T, db *sql.DB, userID int64, now time.Time) string {
	t.Helper()
	id, err := CreateSession(db, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func lookup(t *testing.T, db *sql.DB, id string, now time.Time) (int64, bool) {
	t.Helper()
	uid, ok, err := LookupSession(db, id, now)
	if err != nil {
		t.Fatal(err)
	}
	return uid, ok
}

func TestCreateSessionStoresOnlyHash(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustCreateSession(t, db, uid, t0)
	if len(id) != 43 {
		t.Fatalf("session ID length = %d, want 43", len(id))
	}
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != 32 {
		t.Fatalf("session ID %q is not base64url of 32 bytes: %v", id, err)
	}
	sum := sha256.Sum256(raw)
	var created, seen, expires int64
	err = db.QueryRow("SELECT created_at, last_seen_at, expires_at FROM sessions WHERE id_hash = ?", sum[:]).Scan(&created, &seen, &expires)
	if err != nil {
		t.Fatalf("no row keyed by SHA-256 of the ID: %v", err)
	}
	if created != t0.Unix() || seen != t0.Unix() || expires != t0.Add(30*24*time.Hour).Unix() {
		t.Fatalf("times = %d %d %d", created, seen, expires)
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions WHERE id_hash = ?", id); n != 0 {
		t.Fatal("plain session ID stored")
	}
	if other := mustCreateSession(t, db, uid, t0); other == id {
		t.Fatal("two session IDs are equal")
	}
}

func TestLookupSessionExpiry(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustCreateSession(t, db, uid, t0)

	if got, ok := lookup(t, db, id, t0.Add(59*time.Minute)); !ok || got != uid {
		t.Fatalf("lookup before expiry = %d, %v", got, ok)
	}
	if _, ok := lookup(t, db, "not-a-session", t0); ok {
		t.Fatal("unknown ID found")
	}

	idle := mustCreateSession(t, db, uid, t0)
	if _, ok := lookup(t, db, idle, t0.Add(time.Hour-time.Second)); !ok {
		t.Fatal("session invalid just before 1 hour idle")
	}
	// that use moved last_seen_at, so the session now lasts until 1 hour after it
	if _, ok := lookup(t, db, idle, t0.Add(2*time.Hour-2*time.Second)); !ok {
		t.Fatal("session invalid just before 1 hour after its last use")
	}
	if _, ok := lookup(t, db, idle, t0.Add(3*time.Hour-2*time.Second)); ok {
		t.Fatal("session valid at 1 hour idle")
	}

	unused := mustCreateSession(t, db, uid, t0)
	if _, ok := lookup(t, db, unused, t0.Add(time.Hour)); ok {
		t.Fatal("unused session valid at 1 hour")
	}
	// the failed lookup deleted it, so an earlier clock cannot revive it
	if _, ok := lookup(t, db, unused, t0.Add(time.Minute)); ok {
		t.Fatal("idle-expired session revived")
	}

	busy := mustCreateSession(t, db, uid, t0)
	for d := 30 * time.Minute; d < 30*24*time.Hour; d += 30 * time.Minute {
		if _, ok := lookup(t, db, busy, t0.Add(d)); !ok {
			t.Fatalf("session used every 30 minutes invalid at %v", d)
		}
	}
	if _, ok := lookup(t, db, busy, t0.Add(30*24*time.Hour)); ok {
		t.Fatal("session valid at 30 days")
	}

	early := mustCreateSession(t, db, uid, t0)
	if _, ok := lookup(t, db, early, t0.Add(-time.Second)); ok {
		t.Fatal("session valid before its created_at")
	}
}

func TestLookupSessionTouchesAtMostOnceAMinute(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustCreateSession(t, db, uid, t0)
	seen := func() int64 {
		var s int64
		if err := db.QueryRow("SELECT last_seen_at FROM sessions").Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	lookup(t, db, id, t0.Add(59*time.Second))
	if s := seen(); s != t0.Unix() {
		t.Fatalf("last_seen_at rewritten within a minute: %d", s)
	}
	lookup(t, db, id, t0.Add(61*time.Second))
	if s := seen(); s != t0.Add(61*time.Second).Unix() {
		t.Fatalf("last_seen_at = %d, want touched", s)
	}
}

func TestDeleteSessions(t *testing.T) {
	db := openDB(t)
	a := insertUser(t, db, "a")
	b := insertUser(t, db, "b")
	a1, a2, a3 := mustCreateSession(t, db, a, t0), mustCreateSession(t, db, a, t0), mustCreateSession(t, db, a, t0)
	b1 := mustCreateSession(t, db, b, t0)

	if err := DeleteSession(db, a1); err != nil {
		t.Fatal(err)
	}
	if _, ok := lookup(t, db, a1, t0); ok {
		t.Fatal("deleted session still valid")
	}
	if err := DeleteOtherSessions(db, a, a2); err != nil {
		t.Fatal(err)
	}
	if _, ok := lookup(t, db, a2, t0); !ok {
		t.Fatal("kept session gone")
	}
	if _, ok := lookup(t, db, a3, t0); ok {
		t.Fatal("other session kept")
	}
	if _, ok := lookup(t, db, b1, t0); !ok {
		t.Fatal("another user's session deleted")
	}
	if err := DeleteUserSessions(db, a); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions WHERE user_id = ?", a); n != 0 {
		t.Fatalf("user sessions left = %d", n)
	}
	if _, ok := lookup(t, db, b1, t0); !ok {
		t.Fatal("another user's session deleted")
	}
}

func TestSweepExpired(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	mustCreateSession(t, db, uid, t0.Add(-31*24*time.Hour)) // past absolute expiry
	mustCreateSession(t, db, uid, t0.Add(-time.Hour))       // idle 1 hour
	mustCreateSession(t, db, uid, t0.Add(time.Hour))        // created in the future
	live := mustCreateSession(t, db, uid, t0.Add(-59*time.Minute))
	if err := SweepExpired(db, t0); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions"); n != 1 {
		t.Fatalf("sessions left = %d, want 1", n)
	}
	if _, ok := lookup(t, db, live, t0); !ok {
		t.Fatal("live session swept")
	}
}

func TestSessionConfirm(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustCreateSession(t, db, uid, t0)
	confirmed := func(now time.Time) bool {
		t.Helper()
		ok, err := SessionConfirmed(db, id, now)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	var at int64
	if err := db.QueryRow("SELECT confirmed_at FROM sessions").Scan(&at); err != nil || at != t0.Unix() {
		t.Fatalf("confirmed_at after sign-in = %d (err %v), want now", at, err)
	}
	if !confirmed(t0.Add(300*time.Second)) || confirmed(t0.Add(301*time.Second)) {
		t.Fatal("sign-in confirm not good for exactly 300 s")
	}
	if err := ConfirmSession(db, id, t0.Add(20*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !confirmed(t0.Add(24*time.Minute)) || confirmed(t0.Add(26*time.Minute)) {
		t.Fatal("ConfirmSession not good for 300 s")
	}
	if confirmed(t0.Add(19 * time.Minute)) {
		t.Fatal("confirm counted before it happened")
	}
	if ok, err := SessionConfirmed(db, "not-a-session", t0); ok || err != nil {
		t.Fatalf("malformed session confirmed = %v %v", ok, err)
	}
	db.Exec("UPDATE sessions SET confirmed_at = NULL")
	if confirmed(t0) {
		t.Fatal("NULL confirmed_at counts as confirmed")
	}
}

func TestSessionEndDeletesSetupChallenge(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	setup := func(now time.Time) string {
		t.Helper()
		sid := mustCreateSession(t, db, uid, now)
		if err := SetupChallenge(db, sid, uid, now); err != nil {
			t.Fatal(err)
		}
		return sid
	}
	keep := setup(t0)
	setup(t0)
	if err := DeleteOtherSessions(db, uid, keep); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 1 {
		t.Fatalf("setup rows after DeleteOtherSessions = %d, want 1", n)
	}
	if _, ok := peek(t, db, keep, KindSetup, t0); !ok {
		t.Fatal("kept session lost its setup")
	}
	if err := DeleteUserSessions(db, uid); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 0 {
		t.Fatalf("setup rows after DeleteUserSessions = %d, want 0", n)
	}
	idle := setup(t0)
	if _, ok := lookup(t, db, idle, t0.Add(time.Hour)); ok {
		t.Fatal("idle session valid")
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 0 {
		t.Fatalf("setup rows after an idle session ended = %d, want 0", n)
	}
	setup(t0.Add(-time.Hour))
	mustChallenge(t, db, uid, KindSecondFactor, t0.Add(-time.Hour))
	if err := SweepExpired(db, t0); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 0 {
		t.Fatalf("challenges after sweep = %d, want 0", n)
	}
}
