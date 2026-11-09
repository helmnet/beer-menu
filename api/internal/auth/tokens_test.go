package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

// insertToken stores a token row for a fresh random secret and returns the secret and ID.
func insertToken(t *testing.T, db *sql.DB, name string, scope Scope) (string, int64) {
	t.Helper()
	secret := "bmt_" + base64.RawURLEncoding.EncodeToString([]byte(rand.Text()))
	sum := sha256.Sum256([]byte(secret))
	res, err := db.Exec("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES (?, ?, ?, ?)", name, sum[:], scope, t0.Unix())
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return secret, id
}

func lookupToken(t *testing.T, db *sql.DB, secret string, now time.Time) (Token, bool) {
	t.Helper()
	tok, ok, err := LookupToken(db, secret, now)
	if err != nil {
		t.Fatal(err)
	}
	return tok, ok
}

func TestLookupToken(t *testing.T) {
	db := openDB(t)
	readSecret, readID := insertToken(t, db, "ha", ScopeRead)
	writeSecret, _ := insertToken(t, db, "scale", ScopeWrite)

	tok, ok := lookupToken(t, db, readSecret, t0)
	if !ok || tok.ID != readID || tok.Scope != ScopeRead || tok.Name != "ha" {
		t.Fatalf("read token = %+v, %v", tok, ok)
	}
	if tok, ok := lookupToken(t, db, writeSecret, t0); !ok || tok.Scope != ScopeWrite {
		t.Fatalf("write token = %+v, %v", tok, ok)
	}
	if _, ok := lookupToken(t, db, "bmt_unknown", t0); ok {
		t.Fatal("unknown secret found")
	}
	// The same row looked up without the prefix must fail even if the hash matched.
	noPrefix := readSecret[len("bmt_"):]
	sum := sha256.Sum256([]byte(noPrefix))
	if _, err := db.Exec("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES ('np', ?, 'read', 0)", sum[:]); err != nil {
		t.Fatal(err)
	}
	if _, ok := lookupToken(t, db, noPrefix, t0); ok {
		t.Fatal("secret without bmt_ prefix found")
	}
	if _, err := db.Exec("UPDATE api_tokens SET revoked_at = ? WHERE id = ?", t0.Unix(), readID); err != nil {
		t.Fatal(err)
	}
	if _, ok := lookupToken(t, db, readSecret, t0); ok {
		t.Fatal("revoked token found")
	}
}

func TestLookupTokenLastUsed(t *testing.T) {
	db := openDB(t)
	secret, id := insertToken(t, db, "ha", ScopeRead)
	lastUsed := func() sql.NullInt64 {
		var v sql.NullInt64
		if err := db.QueryRow("SELECT last_used_at FROM api_tokens WHERE id = ?", id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := lastUsed(); v.Valid {
		t.Fatalf("last_used_at before use = %v, want NULL", v)
	}
	lookupToken(t, db, secret, t0)
	if v := lastUsed(); !v.Valid || v.Int64 != t0.Unix() {
		t.Fatalf("last_used_at after first use = %v", v)
	}
	lookupToken(t, db, secret, t0.Add(59*time.Second))
	if v := lastUsed(); v.Int64 != t0.Unix() {
		t.Fatalf("last_used_at rewritten within a minute: %v", v)
	}
	lookupToken(t, db, secret, t0.Add(61*time.Second))
	if v := lastUsed(); v.Int64 != t0.Add(61*time.Second).Unix() {
		t.Fatalf("last_used_at = %v, want updated", v)
	}
}

func TestCreateToken(t *testing.T) {
	db := openDB(t)
	tok, secret, err := CreateToken(db, "  home-assistant ", ScopeRead, t0)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^bmt_[A-Za-z0-9_-]{43}$`).MatchString(secret) {
		t.Fatalf("secret %q has the wrong shape", secret)
	}
	if tok.ID == 0 || tok.Name != "home-assistant" || tok.Scope != ScopeRead || !tok.CreatedAt.Equal(t0) || tok.LastUsedAt != nil || tok.RevokedAt != nil {
		t.Fatalf("token = %+v", tok)
	}
	var stored []byte
	if err := db.QueryRow("SELECT token_hash FROM api_tokens WHERE id = ?", tok.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256([]byte(secret)); string(stored) != string(sum[:]) {
		t.Fatal("stored hash is not the SHA-256 of the secret")
	}
	if found, ok := lookupToken(t, db, secret, t0); !ok || found.ID != tok.ID {
		t.Fatalf("new token not found: %+v %v", found, ok)
	}
	_, other, err := CreateToken(db, "x", ScopeWrite, t0)
	if err != nil || other == secret {
		t.Fatalf("second token: %v, same secret %v", err, other == secret)
	}
}

func TestCreateTokenValidates(t *testing.T) {
	db := openDB(t)
	var ve ValidationError
	for _, c := range []struct {
		name  string
		scope Scope
	}{
		{"", ScopeRead},
		{"   ", ScopeRead},
		{strings.Repeat("a", 65), ScopeRead},
		{"ok", "admin"},
		{"ok", ""},
	} {
		if _, _, err := CreateToken(db, c.name, c.scope, t0); !errors.As(err, &ve) {
			t.Errorf("CreateToken(%q, %q) = %v, want ValidationError", c.name, c.scope, err)
		}
	}
	if _, _, err := CreateToken(db, strings.Repeat("é", 64), ScopeRead, t0); err != nil {
		t.Errorf("64-character name refused: %v", err)
	}
}

func TestListAndRevokeTokens(t *testing.T) {
	db := openDB(t)
	older, _, err := CreateToken(db, "older", ScopeRead, t0)
	if err != nil {
		t.Fatal(err)
	}
	newer, newerSecret, err := CreateToken(db, "newer", ScopeWrite, t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	list, err := ListTokens(db)
	if err != nil || len(list) != 2 || list[0].ID != newer.ID || list[1].ID != older.ID {
		t.Fatalf("ListTokens = %+v, %v; want newest first", list, err)
	}
	body, _ := json.Marshal(list)
	if !strings.Contains(string(body), `"last_used_at":null`) || !strings.Contains(string(body), `"revoked_at":null`) {
		t.Fatalf("unset times not null: %s", body)
	}
	if strings.Contains(string(body), "secret") || strings.Contains(string(body), "hash") || strings.Contains(string(body), newerSecret) {
		t.Fatalf("list exposes secret data: %s", body)
	}

	revokeAt := t0.Add(2 * time.Hour)
	if name, err := RevokeToken(db, newer.ID, revokeAt); err != nil || name != "newer" {
		t.Fatalf("RevokeToken = %q, %v", name, err)
	}
	if _, err := RevokeToken(db, newer.ID, revokeAt.Add(time.Hour)); err != nil {
		t.Fatalf("revoking a revoked token = %v, want nil", err)
	}
	list, _ = ListTokens(db)
	if list[0].RevokedAt == nil || !list[0].RevokedAt.Equal(revokeAt) {
		t.Fatalf("revoked_at = %v, want %v (unchanged by the second revoke)", list[0].RevokedAt, revokeAt)
	}
	if _, ok := lookupToken(t, db, newerSecret, revokeAt); ok {
		t.Fatal("revoked token still found")
	}
	if _, err := RevokeToken(db, newer.ID+100, revokeAt); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RevokeToken(unknown) = %v, want ErrNotFound", err)
	}
}
