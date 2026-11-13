package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"
)

// ChallengeKind is what a short-lived step in auth_challenges is for (research R8).
type ChallengeKind string

// Challenge kinds, as stored in auth_challenges.kind.
const (
	// KindSecondFactor is a sign-in whose password was right and whose second factor is owed.
	KindSecondFactor ChallengeKind = "second_factor"
	// KindEnroll is a sign-in whose password was right by an admin who must set up 2FA first.
	KindEnroll ChallengeKind = "enroll"
	// KindSetup holds a signed-in admin's factor setup in progress, keyed by the session.
	KindSetup ChallengeKind = "setup"
)

// ChallengeTTL is how long a challenge lives after it is made (FR-002).
const ChallengeTTL = 5 * time.Minute

// Challenge is one auth_challenges row.
type Challenge struct {
	UserID int64
	Kind   ChallengeKind
	// WebAuthn is the in-flight WebAuthn session as JSON, or "".
	WebAuthn string
	// TotpPendingSealed is the sealed secret of an unconfirmed authenticator, or "".
	TotpPendingSealed string
	CreatedAt         time.Time
}

// querier is a *sql.DB or a *sql.Tx, so challenge and factor steps can join a transaction.
type querier interface {
	execer
	QueryRow(query string, args ...any) *sql.Row
}

// CreateChallenge deletes every expired challenge, then stores a new one of kind for userID
// at now and returns its ID: 32 random bytes, base64url without padding. Only the SHA-256 of
// the raw bytes is stored. It lives ChallengeTTL and can be taken once.
func CreateChallenge(db *sql.DB, userID int64, kind ChallengeKind, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	if err := insertChallenge(db, sum[:], userID, kind, now); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// SetupChallenge starts the factor setup of the admin signed in with sessionID, replacing any
// earlier setup of that session along with its data. It is found with the session ID as its ID
// and ends with the session. A malformed session ID is an error.
func SetupChallenge(db *sql.DB, sessionID string, userID int64, now time.Time) error {
	key, ok := sessionHash(sessionID)
	if !ok {
		return errors.New("setup challenge needs a well-formed session ID")
	}
	return insertChallenge(db, key, userID, KindSetup, now)
}

func insertChallenge(db *sql.DB, key []byte, userID int64, kind ChallengeKind, now time.Time) error {
	t := now.Unix()
	if _, err := db.Exec("DELETE FROM auth_challenges WHERE expires_at <= ?", t); err != nil {
		return err
	}
	_, err := db.Exec("INSERT OR REPLACE INTO auth_challenges (id_hash, user_id, kind, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		key, userID, kind, t, t+int64(ChallengeTTL/time.Second))
	return err
}

// PeekChallenge returns challenge id of kind when it is unexpired at now, without using it
// up. ok is false for unknown, malformed, expired or other-kind IDs; err is set only for
// database failures.
func PeekChallenge(db querier, id string, kind ChallengeKind, now time.Time) (c Challenge, ok bool, err error) {
	return readChallenge(db, false, id, kind, now)
}

// TakeChallenge is PeekChallenge that also deletes the challenge, so it is used once. db is a
// *sql.DB or *sql.Tx.
func TakeChallenge(db querier, id string, kind ChallengeKind, now time.Time) (c Challenge, ok bool, err error) {
	return readChallenge(db, true, id, kind, now)
}

func readChallenge(db querier, take bool, id string, kind ChallengeKind, now time.Time) (c Challenge, ok bool, err error) {
	key, ok := sessionHash(id)
	if !ok {
		return Challenge{}, false, nil
	}
	q := "SELECT user_id, kind, webauthn, totp_pending_sealed, created_at FROM auth_challenges WHERE id_hash = ? AND kind = ? AND expires_at > ?"
	if take {
		q = "DELETE FROM auth_challenges WHERE id_hash = ? AND kind = ? AND expires_at > ? RETURNING user_id, kind, webauthn, totp_pending_sealed, created_at"
	}
	var wa, totp sql.NullString
	var created int64
	err = db.QueryRow(q, key, kind, now.Unix()).Scan(&c.UserID, &c.Kind, &wa, &totp, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return Challenge{}, false, nil
	}
	if err != nil {
		return Challenge{}, false, err
	}
	c.WebAuthn, c.TotpPendingSealed, c.CreatedAt = wa.String, totp.String, unixTime(created)
	return c, true, nil
}

// SetChallengeData stores an in-flight WebAuthn session (JSON) and a sealed pending
// authenticator secret on challenge id of kind, if unexpired at now. An empty value keeps
// what is stored. ok is false when there is no such challenge.
func SetChallengeData(db *sql.DB, id string, kind ChallengeKind, webauthn, totpSealed string, now time.Time) (bool, error) {
	key, ok := sessionHash(id)
	if !ok {
		return false, nil
	}
	res, err := db.Exec(`UPDATE auth_challenges SET webauthn = COALESCE(NULLIF(?, ''), webauthn),
		totp_pending_sealed = COALESCE(NULLIF(?, ''), totp_pending_sealed) WHERE id_hash = ? AND kind = ? AND expires_at > ?`,
		webauthn, totpSealed, key, kind, now.Unix())
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// takeCeremony returns and clears the WebAuthn data of userID's challenge id of kind, if it is
// unexpired at now and has some, so a request ceremony is checked once at most.
func takeCeremony(db *sql.DB, id string, kind ChallengeKind, userID int64, now time.Time) (string, bool, error) {
	key, ok := sessionHash(id)
	if !ok {
		return "", false, nil
	}
	var data string
	err := db.QueryRow("SELECT webauthn FROM auth_challenges WHERE id_hash = ? AND kind = ? AND user_id = ? AND expires_at > ? AND webauthn IS NOT NULL",
		key, kind, userID, now.Unix()).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	// Clearing only the value read makes two concurrent takes of one ceremony succeed once.
	res, err := db.Exec("UPDATE auth_challenges SET webauthn = NULL WHERE id_hash = ? AND webauthn = ?", key, data)
	if err != nil {
		return "", false, err
	}
	n, err := res.RowsAffected()
	return data, n == 1, err
}

// DeleteChallengesFor deletes every challenge of userID: pending sign-ins and setups. db is a
// *sql.DB or *sql.Tx.
func DeleteChallengesFor(db execer, userID int64) error {
	_, err := db.Exec("DELETE FROM auth_challenges WHERE user_id = ?", userID)
	return err
}
