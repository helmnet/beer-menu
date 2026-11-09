package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Scope is what an API token may do: read data, or read and write data. No token can manage
// admins, tokens or passwords (FR-033).
type Scope string

// Token scopes, as stored in api_tokens.scope.
const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
)

// TokenPrefix starts every token secret, so secret scanners can find leaked tokens.
const TokenPrefix = "bmt_"

// Token is an API token as listed to admins. It never holds the secret or its hash.
type Token struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Scope      Scope      `json:"scope"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// CreateToken stores a new token named name (trimmed, 1 to 64 characters) with scope at now
// and returns it with its secret: "bmt_" and 32 random bytes in base64url without padding.
// Only the secret's SHA-256 is stored, so the caller must show the secret now or never. It
// returns a ValidationError for a bad name or a scope other than read or write.
func CreateToken(db *sql.DB, name string, scope Scope, now time.Time) (Token, string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 {
		return Token{}, "", ValidationError("Name must be 1 to 64 characters.")
	}
	if scope != ScopeRead && scope != ScopeWrite {
		return Token{}, "", ValidationError("Scope must be read or write.")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Token{}, "", err
	}
	secret := TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(secret))
	res, err := db.Exec("INSERT INTO api_tokens (name, token_hash, scope, created_at) VALUES (?, ?, ?, ?)",
		name, sum[:], scope, now.Unix())
	if err != nil {
		return Token{}, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Token{}, "", err
	}
	return Token{ID: id, Name: name, Scope: scope, CreatedAt: unixTime(now.Unix())}, secret, nil
}

// ListTokens returns every token, revoked ones included, newest first. It never reads the
// hash.
func ListTokens(db *sql.DB) ([]Token, error) {
	rows, err := db.Query("SELECT id, name, scope, created_at, last_used_at, revoked_at FROM api_tokens ORDER BY created_at DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []Token{}
	for rows.Next() {
		var tok Token
		var created int64
		var lastUsed, revoked sql.NullInt64
		if err := rows.Scan(&tok.ID, &tok.Name, &tok.Scope, &created, &lastUsed, &revoked); err != nil {
			return nil, err
		}
		tok.CreatedAt, tok.LastUsedAt, tok.RevokedAt = unixTime(created), nullTime(lastUsed), nullTime(revoked)
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}

// RevokeToken marks token id revoked at now and returns its name; LookupToken refuses it from
// then on. Revoking a revoked token succeeds and keeps the first revoke time. It returns
// ErrNotFound for an unknown id.
func RevokeToken(db *sql.DB, id int64, now time.Time) (string, error) {
	var name string
	err := db.QueryRow("UPDATE api_tokens SET revoked_at = COALESCE(revoked_at, ?) WHERE id = ? RETURNING name", now.Unix(), id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

// LookupToken returns the active token whose secret is secret. ok is false when secret lacks
// the bmt_ prefix, matches no token, or matches a revoked one; err is set only for database
// failures. A found token's last_used_at is set to now when it is NULL or at least a minute old.
func LookupToken(db *sql.DB, secret string, now time.Time) (tok Token, ok bool, err error) {
	if !strings.HasPrefix(secret, TokenPrefix) {
		return Token{}, false, nil
	}
	sum := sha256.Sum256([]byte(secret))
	var created int64
	var lastUsed, revoked sql.NullInt64
	err = db.QueryRow("SELECT id, name, scope, created_at, last_used_at, revoked_at FROM api_tokens WHERE token_hash = ?", sum[:]).
		Scan(&tok.ID, &tok.Name, &tok.Scope, &created, &lastUsed, &revoked)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && revoked.Valid) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, err
	}
	tok.CreatedAt = unixTime(created)
	t := now.Unix()
	if !lastUsed.Valid || t-lastUsed.Int64 >= touchSecs {
		if _, err := db.Exec("UPDATE api_tokens SET last_used_at = ? WHERE id = ?", t, tok.ID); err != nil {
			return Token{}, false, err
		}
		lastUsed = sql.NullInt64{Int64: t, Valid: true}
	}
	tok.LastUsedAt = nullTime(lastUsed)
	return tok, true, nil
}

// unixTime converts stored Unix seconds to a UTC time, which marshals as RFC 3339.
func unixTime(s int64) time.Time { return time.Unix(s, 0).UTC() }

func nullTime(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := unixTime(v.Int64)
	return &t
}
