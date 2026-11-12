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

// ConfirmWindow is how long a confirm (or sign-in) lets a session make sensitive 2FA changes
// (research R9).
const ConfirmWindow = 5 * time.Minute

const (
	sessionIdleSecs = 60 * 60 // a session unused this long is invalid (FR-016)
	touchSecs       = 60      // bounds last_seen_at writes to one a minute per session
)

// CreateSession starts a session for userID at now, confirmed at now, and returns its ID: 32
// random bytes, base64url without padding (43 characters). Only the SHA-256 of the raw bytes
// is stored.
func CreateSession(db *sql.DB, userID int64, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	_, err := db.Exec("INSERT INTO sessions (id_hash, user_id, created_at, last_seen_at, expires_at, confirmed_at) VALUES (?, ?, ?, ?, ?, ?)",
		sum[:], userID, now.Unix(), now.Unix(), now.Add(SessionMaxAge).Unix(), now.Unix())
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
// now < expires_at and now < last_seen_at + 1 hour. A valid session's last_seen_at is moved to
// now when it is at least a minute old; an invalid one is deleted with its setup. ok is false for unknown,
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
		return 0, false, deleteSessionKey(db, key)
	}
	if t-seen >= touchSecs {
		if _, err := db.Exec("UPDATE sessions SET last_seen_at = ? WHERE id_hash = ?", t, key); err != nil {
			return 0, false, err
		}
	}
	return userID, true, nil
}

// DeleteSession ends session id (sign-out) and its factor setup. An unknown ID is not an error.
func DeleteSession(db *sql.DB, id string) error {
	key, ok := sessionHash(id)
	if !ok {
		return nil
	}
	return deleteSessionKey(db, key)
}

// deleteSessionKey deletes the session stored under key and its setup challenge in one
// transaction.
func deleteSessionKey(db *sql.DB, key []byte) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM auth_challenges WHERE id_hash = ? AND kind = ?", key, KindSetup); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE id_hash = ?", key); err != nil {
		return err
	}
	return tx.Commit()
}

// ConfirmSession records that session id proved its admin's identity at now (research R9).
func ConfirmSession(db *sql.DB, id string, now time.Time) error {
	key, ok := sessionHash(id)
	if !ok {
		return nil
	}
	_, err := db.Exec("UPDATE sessions SET confirmed_at = ? WHERE id_hash = ?", now.Unix(), key)
	return err
}

// SessionConfirmed reports whether session id was confirmed, or started, at most
// ConfirmWindow before now. Unknown and malformed IDs are false.
func SessionConfirmed(db *sql.DB, id string, now time.Time) (bool, error) {
	key, ok := sessionHash(id)
	if !ok {
		return false, nil
	}
	var at sql.NullInt64
	err := db.QueryRow("SELECT confirmed_at FROM sessions WHERE id_hash = ?", key).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	t := now.Unix()
	return err == nil && at.Valid && at.Int64 <= t && t <= at.Int64+int64(ConfirmWindow/time.Second), err
}

// execer is a *sql.DB or a *sql.Tx, so session deletes can join a password change.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// DeleteUserSessions ends every session of userID, their factor setups and userID's pending
// sign-ins, so a pending started with an old password cannot finish after a reset. db is a
// *sql.DB or *sql.Tx.
func DeleteUserSessions(db execer, userID int64) error {
	if err := DeleteChallengesFor(db, userID); err != nil {
		return err
	}
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// DeleteOtherSessions ends every session of userID except keepID (after a password change),
// their factor setups and userID's pending sign-ins, so a pending started with an old password
// cannot finish after the change. db is a *sql.DB or *sql.Tx.
func DeleteOtherSessions(db execer, userID int64, keepID string) error {
	key, _ := sessionHash(keepID) // a malformed keepID keeps nothing, which is the safe side
	if _, err := db.Exec("DELETE FROM auth_challenges WHERE user_id = ? AND NOT (kind = ? AND id_hash IS ?)", userID, KindSetup, key); err != nil {
		return err
	}
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ? AND id_hash IS NOT ?", userID, key)
	return err
}

// SweepExpired deletes every session that is invalid at now, by the rules of LookupSession,
// and every expired challenge.
func SweepExpired(db *sql.DB, now time.Time) error {
	t := now.Unix()
	if _, err := db.Exec("DELETE FROM sessions WHERE created_at > ? OR expires_at <= ? OR last_seen_at + ? <= ?",
		t, t, sessionIdleSecs, t); err != nil {
		return err
	}
	// Setup rows live no longer than ChallengeTTL, so this also drops those of swept sessions.
	_, err := db.Exec("DELETE FROM auth_challenges WHERE expires_at <= ?", t)
	return err
}
