package auth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"testing"
	"time"
)

func mustChallenge(t *testing.T, db *sql.DB, userID int64, kind ChallengeKind, now time.Time) string {
	t.Helper()
	id, err := CreateChallenge(db, userID, kind, now)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func peek(t *testing.T, db *sql.DB, id string, kind ChallengeKind, now time.Time) (Challenge, bool) {
	t.Helper()
	c, ok, err := PeekChallenge(db, id, kind, now)
	if err != nil {
		t.Fatal(err)
	}
	return c, ok
}

func TestCreateChallengeStoresOnlyHash(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustChallenge(t, db, uid, KindSecondFactor, t0)
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if len(id) != 43 || err != nil || len(raw) != 32 {
		t.Fatalf("challenge ID %q is not base64url of 32 bytes: %v", id, err)
	}
	sum := sha256.Sum256(raw)
	var kind string
	var created, expires int64
	if err := db.QueryRow("SELECT kind, created_at, expires_at FROM auth_challenges WHERE id_hash = ? AND user_id = ?", sum[:], uid).
		Scan(&kind, &created, &expires); err != nil {
		t.Fatalf("no row keyed by SHA-256 of the ID: %v", err)
	}
	if kind != "second_factor" || created != t0.Unix() || expires != t0.Unix()+300 {
		t.Fatalf("row = %s %d %d", kind, created, expires)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges WHERE id_hash = ?", id); n != 0 {
		t.Fatal("plain challenge ID stored")
	}
}

func TestChallengeSingleUseAndExpiry(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustChallenge(t, db, uid, KindSecondFactor, t0)

	if c, ok := peek(t, db, id, KindSecondFactor, t0.Add(time.Minute)); !ok || c.UserID != uid || c.Kind != KindSecondFactor {
		t.Fatalf("peek = %+v %v", c, ok)
	}
	if _, ok := peek(t, db, id, KindSecondFactor, t0.Add(time.Minute)); !ok {
		t.Fatal("peek consumed the challenge")
	}
	if _, ok := peek(t, db, id, KindEnroll, t0); ok {
		t.Fatal("found with another kind")
	}
	if _, ok, err := TakeChallenge(db, id, KindEnroll, t0); ok || err != nil {
		t.Fatalf("take with another kind = %v %v", ok, err)
	}
	c, ok, err := TakeChallenge(db, id, KindSecondFactor, t0.Add(time.Minute))
	if err != nil || !ok || c.UserID != uid {
		t.Fatalf("take = %+v %v %v", c, ok, err)
	}
	if _, ok, _ := TakeChallenge(db, id, KindSecondFactor, t0.Add(time.Minute)); ok {
		t.Fatal("challenge taken twice")
	}
	if _, ok := peek(t, db, id, KindSecondFactor, t0); ok {
		t.Fatal("taken challenge still found")
	}

	old := mustChallenge(t, db, uid, KindEnroll, t0)
	if _, ok := peek(t, db, old, KindEnroll, t0.Add(300*time.Second-time.Second)); !ok {
		t.Fatal("challenge gone before 300 s")
	}
	if _, ok := peek(t, db, old, KindEnroll, t0.Add(300*time.Second)); ok {
		t.Fatal("challenge found at 300 s")
	}
	if _, ok, _ := TakeChallenge(db, old, KindEnroll, t0.Add(301*time.Second)); ok {
		t.Fatal("expired challenge taken")
	}
	for _, bad := range []string{"", "not-an-id", id + "x"} {
		if _, ok := peek(t, db, bad, KindEnroll, t0); ok {
			t.Fatalf("malformed ID %q found", bad)
		}
	}

	mustChallenge(t, db, uid, KindSecondFactor, t0.Add(time.Hour))
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 1 {
		t.Fatalf("rows after an insert = %d, want expired rows deleted", n)
	}
}

func TestSetChallengeData(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	id := mustChallenge(t, db, uid, KindSecondFactor, t0)
	if ok, err := SetChallengeData(db, id, KindSecondFactor, `{"challenge":"x"}`, "", t0); !ok || err != nil {
		t.Fatalf("set webauthn = %v %v", ok, err)
	}
	if ok, err := SetChallengeData(db, id, KindSecondFactor, "", "v1:sealed", t0); !ok || err != nil {
		t.Fatalf("set totp = %v %v", ok, err)
	}
	c, _ := peek(t, db, id, KindSecondFactor, t0)
	if c.WebAuthn != `{"challenge":"x"}` || c.TotpPendingSealed != "v1:sealed" {
		t.Fatalf("data = %q %q, want both kept", c.WebAuthn, c.TotpPendingSealed)
	}
	if ok, _ := SetChallengeData(db, id, KindEnroll, "{}", "", t0); ok {
		t.Fatal("set data with another kind")
	}
	if ok, _ := SetChallengeData(db, id, KindSecondFactor, "{}", "", t0.Add(5*time.Minute)); ok {
		t.Fatal("set data on an expired challenge")
	}
}

func TestDeleteChallengesFor(t *testing.T) {
	db := openDB(t)
	a := insertUser(t, db, "a")
	b := insertUser(t, db, "b")
	mustChallenge(t, db, a, KindSecondFactor, t0)
	mustChallenge(t, db, a, KindEnroll, t0)
	if err := SetupChallenge(db, mustCreateSession(t, db, a, t0), a, t0); err != nil {
		t.Fatal(err)
	}
	keep := mustChallenge(t, db, b, KindSecondFactor, t0)
	if err := DeleteChallengesFor(db, a); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges WHERE user_id = ?", a); n != 0 {
		t.Fatalf("rows of a left = %d", n)
	}
	if _, ok := peek(t, db, keep, KindSecondFactor, t0); !ok {
		t.Fatal("another admin's challenge deleted")
	}
}

func TestSetupChallengeFollowsSession(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	if err := SetupChallenge(db, sid, uid, t0); err != nil {
		t.Fatal(err)
	}
	if ok, err := SetChallengeData(db, sid, KindSetup, "", "v1:old", t0); !ok || err != nil {
		t.Fatalf("set data = %v %v", ok, err)
	}
	// A new setup replaces the earlier one, data included.
	if err := SetupChallenge(db, sid, uid, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	c, ok := peek(t, db, sid, KindSetup, t0.Add(time.Minute))
	if !ok || c.TotpPendingSealed != "" || c.CreatedAt != t0.Add(time.Minute) {
		t.Fatalf("setup after replace = %+v %v", c, ok)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 1 {
		t.Fatalf("setup rows = %d, want 1", n)
	}
	if err := SetupChallenge(db, "not-a-session", uid, t0); err == nil {
		t.Fatal("setup for a malformed session ID")
	}
	if err := DeleteSession(db, sid); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 0 {
		t.Fatalf("setup rows after sign-out = %d, want 0", n)
	}
}
