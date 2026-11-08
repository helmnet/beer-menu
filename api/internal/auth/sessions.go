package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

// SessionMaxAge is how long a session lasts after sign-in, however often it is used (FR-016).
const SessionMaxAge = 30 * 24 * time.Hour

const (
	sessionIdleSecs = 7 * 24 * 60 * 60 // a session unused this long is invalid (FR-016)
	touchSecs       = 60               // bounds last_seen_at writes to one a minute per session
)

// CreateSession starts a session for userID at now and returns its ID: 32 random bytes,
// base64url without padding (43 characters). Only the SHA-256 of the raw bytes is stored.
func CreateSession(db *sql.DB, userID int64, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	_, err := db.Exec("INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		sum[:], userID, now.Unix(), now.Unix(), now.Add(SessionMaxAge).Unix())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// sessionHash returns the stored key for a session ID, or false if id is not a well-formed ID.
func sessionHash(id string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}

// LookupSession returns the user of session id when it is valid at now: created_at <= now,
// now < expires_at and now < last_seen_at + 7 days. A valid session's last_seen_at is moved to
// now when it is at least a minute old; an invalid one is deleted. ok is false for unknown,
// malformed or invalid IDs; err is set only for database failures.
func LookupSession(db *sql.DB, id string, now time.Time) (userID int64, ok bool, err error) {
	key, ok := sessionHash(id)
	if !ok {
		return 0, false, nil
	}
	var created, seen, expires int64
	err = db.QueryRow("SELECT user_id, created_at, last_seen_at, expires_at FROM sessions WHERE id_hash = ?", key).
		Scan(&userID, &created, &seen, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	t := now.Unix()
	if t < created || t >= expires || t >= seen+sessionIdleSecs {
		_, err := db.Exec("DELETE FROM sessions WHERE id_hash = ?", key)
		return 0, false, err
	}
	if t-seen >= touchSecs {
		if _, err := db.Exec("UPDATE sessions SET last_seen_at = ? WHERE id_hash = ?", t, key); err != nil {
			return 0, false, err
		}
	}
	return userID, true, nil
}

// DeleteSession ends session id (sign-out). An unknown ID is not an error.
func DeleteSession(db *sql.DB, id string) error {
	key, ok := sessionHash(id)
	if !ok {
		return nil
	}
	_, err := db.Exec("DELETE FROM sessions WHERE id_hash = ?", key)
	return err
}

// execer is a *sql.DB or a *sql.Tx, so session deletes can join a password change.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// DeleteUserSessions ends every session of userID. db is a *sql.DB or *sql.Tx.
func DeleteUserSessions(db execer, userID int64) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// DeleteOtherSessions ends every session of userID except keepID (after a password change).
// db is a *sql.DB or *sql.Tx.
func DeleteOtherSessions(db execer, userID int64, keepID string) error {
	key, _ := sessionHash(keepID) // a malformed keepID keeps nothing, which is the safe side
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ? AND id_hash IS NOT ?", userID, key)
	return err
}

// SweepExpired deletes every session that is invalid at now, by the rules of LookupSession.
func SweepExpired(db *sql.DB, now time.Time) error {
	t := now.Unix()
	_, err := db.Exec("DELETE FROM sessions WHERE created_at > ? OR expires_at <= ? OR last_seen_at + ? <= ?",
		t, t, sessionIdleSecs, t)
	return err
}
