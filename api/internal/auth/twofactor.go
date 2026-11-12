package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/helmnet/beer-menu/api/internal/secrets"
	"github.com/helmnet/beer-menu/api/internal/settings"
)

// Errors of adding an authenticator.
var (
	// ErrAlreadySetUp means the admin already has an authenticator (one at most, FR-012).
	ErrAlreadySetUp = errors.New("an authenticator is already set up")
	// ErrTotpUnavailable means the server has no encryption key, so it cannot store an
	// authenticator secret (research R6).
	ErrTotpUnavailable = errors.New("authenticator apps need an encryption key")
	// ErrNoFactor means the admin has no second factor to turn off.
	ErrNoFactor = errors.New("no second factor is set up")
	// ErrTwoFactorRequired means the change would leave the admin without a second factor while
	// 2FA is required for every admin (FR-035).
	ErrTwoFactorRequired = errors.New("two-factor sign-in is required")
)

// requireKey is the settings key of "require 2FA for every admin" (FR-026).
const requireKey = "require_2fa"

// TwoFactorState is an admin's 2FA state as GET /api/2fa shows it. It never holds a secret,
// public key or credential ID. LoadTwoFactorState fills all but the availability fields, which
// depend on the server's configuration.
type TwoFactorState struct {
	Enabled           bool        `json:"enabled"`
	Required          bool        `json:"required"`
	TotpAvailable     bool        `json:"totp_available"`
	PasskeysAvailable bool        `json:"passkeys_available"`
	PasskeysOrigin    *string     `json:"passkeys_origin"`
	Totp              *TotpFactor `json:"totp"`
	Passkeys          []Passkey   `json:"passkeys"`
	RecoveryCodesLeft int         `json:"recovery_codes_left"`
}

// TotpFactor is an admin's authenticator app as listed to them.
type TotpFactor struct {
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// Passkey is a stored passkey as listed to its admin.
type Passkey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// HasTwoFactor reports whether userID has a second factor: an authenticator or a passkey.
// Recovery codes alone do not count. db is a *sql.DB or *sql.Tx.
func HasTwoFactor(db querier, userID int64) (bool, error) {
	var has bool
	err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM totp_factors WHERE user_id = ?) OR EXISTS (SELECT 1 FROM passkeys WHERE user_id = ?)",
		userID, userID).Scan(&has)
	return has, err
}

// LoadTwoFactorState returns userID's factors, recovery codes left and the require setting.
// Passkeys are oldest first and never nil.
func LoadTwoFactorState(db *sql.DB, userID int64) (TwoFactorState, error) {
	s := TwoFactorState{Passkeys: []Passkey{}}
	var err error
	if s.Required, err = RequireTwoFactor(db); err != nil {
		return s, err
	}
	if s.RecoveryCodesLeft, err = RecoveryCodesLeft(db, userID); err != nil {
		return s, err
	}
	var created int64
	var used sql.NullInt64
	err = db.QueryRow("SELECT created_at, last_used_at FROM totp_factors WHERE user_id = ?", userID).Scan(&created, &used)
	if err == nil {
		s.Totp = &TotpFactor{CreatedAt: unixTime(created), LastUsedAt: nullTime(used)}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return s, err
	}
	rows, err := db.Query("SELECT id, name, created_at, last_used_at FROM passkeys WHERE user_id = ? ORDER BY created_at, id", userID)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Passkey
		if err := rows.Scan(&p.ID, &p.Name, &created, &used); err != nil {
			return s, err
		}
		p.CreatedAt, p.LastUsedAt = unixTime(created), nullTime(used)
		s.Passkeys = append(s.Passkeys, p)
	}
	s.Enabled = s.Totp != nil || len(s.Passkeys) > 0
	return s, rows.Err()
}

// totpPurpose binds a sealed authenticator secret to its admin, so it cannot be moved to
// another admin's row (research R6).
func totpPurpose(userID int64) string { return "totp:" + strconv.FormatInt(userID, 10) }

// CheckTotp reports whether code is valid now for userID's authenticator, and if so records
// its step and now as last used, so the code cannot be used again (FR-010). It is false when
// the admin has no authenticator or box is nil. A secret that box cannot open is an error.
func CheckTotp(db *sql.DB, box *secrets.Box, userID int64, code string, now time.Time) (bool, error) {
	if box == nil {
		return false, nil
	}
	var sealed string
	var last int64
	err := db.QueryRow("SELECT secret_sealed, last_step FROM totp_factors WHERE user_id = ?", userID).Scan(&sealed, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	secret, err := box.Open(sealed, totpPurpose(userID))
	if err != nil {
		return false, err
	}
	step, ok := Verify([]byte(secret), code, now, last)
	if !ok {
		return false, nil
	}
	// The last_step condition makes two concurrent uses of one code accept only one.
	res, err := db.Exec("UPDATE totp_factors SET last_step = ?, last_used_at = ? WHERE user_id = ? AND last_step < ?",
		step, now.Unix(), userID, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// StartTotpSetup starts adding an authenticator for userID on setup challenge id of kind (see
// openSetup): it makes a new secret, keeps it sealed for userID (purpose totp:<user_id>) on
// that challenge until it expires, replacing any earlier secret, and returns it as base32 and
// as the otpauth URI for username. It returns ErrTotpUnavailable when box is nil and
// ErrAlreadySetUp when userID has an authenticator.
func StartTotpSetup(db *sql.DB, box *secrets.Box, id string, kind ChallengeKind, userID int64, username string, now time.Time) (secret, uri string, err error) {
	if box == nil {
		return "", "", ErrTotpUnavailable
	}
	var has bool
	if err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM totp_factors WHERE user_id = ?)", userID).Scan(&has); err != nil {
		return "", "", err
	}
	if has {
		return "", "", ErrAlreadySetUp
	}
	raw, err := NewSecret()
	if err != nil {
		return "", "", err
	}
	sealed, err := box.Seal(string(raw), totpPurpose(userID))
	if err != nil {
		return "", "", err
	}
	if err := openSetup(db, id, kind, userID, now); err != nil {
		return "", "", err
	}
	if ok, err := SetChallengeData(db, id, kind, "", sealed, now); err != nil || !ok {
		return "", "", errors.Join(errors.New("store pending authenticator secret"), err)
	}
	return EncodeSecret(raw), URI(username, raw), nil
}

// FinishTotpSetup saves the authenticator that setup challenge id of kind holds for userID
// when code is valid for it now, and uses up the challenge (for KindEnroll, the pending
// sign-in). The accepted step is stored as last_step, so the same code cannot then sign in.
// When it is userID's first factor it also makes and returns 10 recovery codes and ends
// userID's sessions other than id (FR-017, FR-024), all in one transaction. ok is false, and
// nothing changes, for a wrong code or when no unexpired challenge with a secret is open for
// id and userID. It returns ErrTotpUnavailable when box is nil and ErrAlreadySetUp when userID
// has an authenticator.
func FinishTotpSetup(db *sql.DB, box *secrets.Box, id string, kind ChallengeKind, userID int64, code string, now time.Time) (recoveryCodes []string, ok bool, err error) {
	if box == nil {
		return nil, false, ErrTotpUnavailable
	}
	c, ok, err := PeekChallenge(db, id, kind, now)
	if err != nil || !ok || c.UserID != userID || c.TotpPendingSealed == "" {
		return nil, false, err
	}
	secret, err := box.Open(c.TotpPendingSealed, totpPurpose(userID))
	if err != nil {
		return nil, false, err
	}
	step, ok := Verify([]byte(secret), code, now, 0)
	if !ok {
		return nil, false, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	// Taking the setup in the transaction makes a code accepted by two concurrent calls save once.
	if _, ok, err := TakeChallenge(tx, id, kind, now); err != nil || !ok {
		return nil, false, err
	}
	has, err := HasTwoFactor(tx, userID)
	if err != nil {
		return nil, false, err
	}
	res, err := tx.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, ?, ?, ?) ON CONFLICT (user_id) DO NOTHING",
		userID, c.TotpPendingSealed, step, now.Unix())
	if err != nil {
		return nil, false, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, false, err
	} else if n == 0 {
		return nil, false, ErrAlreadySetUp
	}
	if !has {
		if recoveryCodes, err = replaceRecoveryCodes(tx, userID); err != nil {
			return nil, false, err
		}
		if err := DeleteOtherSessions(tx, userID, id); err != nil {
			return nil, false, err
		}
	}
	return recoveryCodes, true, tx.Commit()
}

// openSetup readies challenge id of kind to hold a factor setup for userID. For KindSetup, id is
// the signed-in session's ID and it starts that session's setup, replacing any earlier one. For
// KindEnroll, id is an enroll pending sign-in, which must be unexpired and userID's, else
// ErrNotFound; the setup lives on it, so it ends with that sign-in (research R10). Any other
// kind is an error.
func openSetup(db *sql.DB, id string, kind ChallengeKind, userID int64, now time.Time) error {
	switch kind {
	case KindSetup:
		return SetupChallenge(db, id, userID, now)
	case KindEnroll:
		c, ok, err := PeekChallenge(db, id, kind, now)
		if err == nil && (!ok || c.UserID != userID) {
			err = ErrNotFound
		}
		return err
	}
	return errors.New("factor setup needs a setup or enroll challenge")
}

// normalizeRecoveryCode drops spaces and dashes and lowercases code (research R7).
func normalizeRecoveryCode(code string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(code))
}

// NewRecoveryCodes makes 10 new recovery codes for userID, each 10 random base32 characters
// (50 bits) shown as "xxxxx-xxxxx", and replaces any earlier set in one transaction (FR-019).
// Only the SHA-256 of each normalized code is stored, so the caller must show them now or never.
func NewRecoveryCodes(db *sql.DB, userID int64) ([]string, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	codes, err := replaceRecoveryCodes(tx, userID)
	if err != nil {
		return nil, err
	}
	return codes, tx.Commit()
}

// replaceRecoveryCodes is NewRecoveryCodes inside the caller's transaction.
func replaceRecoveryCodes(tx *sql.Tx, userID int64) ([]string, error) {
	if _, err := tx.Exec("DELETE FROM recovery_codes WHERE user_id = ?", userID); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	for i := range codes {
		// rand.Text is base32 with 5 random bits per character.
		c := strings.ToLower(rand.Text()[:10])
		sum := sha256.Sum256([]byte(c))
		if _, err := tx.Exec("INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)", userID, sum[:]); err != nil {
			return nil, err
		}
		codes[i] = c[:5] + "-" + c[5:]
	}
	return codes, nil
}

// UseRecoveryCode spends userID's unused recovery code matching code (spaces, dashes and case
// ignored) at now and reports whether there was one. A used code never works again (FR-018).
// db is a *sql.DB or *sql.Tx.
func UseRecoveryCode(db execer, userID int64, code string, now time.Time) (bool, error) {
	sum := sha256.Sum256([]byte(normalizeRecoveryCode(code)))
	res, err := db.Exec("UPDATE recovery_codes SET used_at = ? WHERE user_id = ? AND code_hash = ? AND used_at IS NULL",
		now.Unix(), userID, sum[:])
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// SignInWithRecoveryCode finishes second_factor pending sign-in pendingID of userID with
// recovery code code: in one transaction it uses up the pending sign-in and spends the code,
// so a code is never spent without its sign-in finishing, nor the other way round. A wrong or
// used code is false with nothing changed. A pending sign-in that is gone, expired or not
// userID's is ErrNotFound.
func SignInWithRecoveryCode(db *sql.DB, pendingID string, userID int64, code string, now time.Time) (bool, error) {
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	c, ok, err := TakeChallenge(tx, pendingID, KindSecondFactor, now)
	if err != nil {
		return false, err
	}
	if !ok || c.UserID != userID {
		return false, ErrNotFound
	}
	used, err := UseRecoveryCode(tx, userID, code, now)
	if err != nil || !used {
		return false, err
	}
	return true, tx.Commit()
}

// RecoveryCodesLeft returns how many unused recovery codes userID has. db is a *sql.DB or
// *sql.Tx.
func RecoveryCodesLeft(db querier, userID int64) (int, error) {
	var n int
	err := db.QueryRow("SELECT count(*) FROM recovery_codes WHERE user_id = ? AND used_at IS NULL", userID).Scan(&n)
	return n, err
}

// ClearTwoFactor deletes all of userID's factors, recovery codes, challenges and sessions in
// one transaction, for another admin's clear (FR-027) and the reset-2fa command (FR-029). It
// returns ErrNotFound for an unknown admin.
func ClearTwoFactor(db *sql.DB, userID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var one int
	if err := tx.QueryRow("SELECT 1 FROM users WHERE id = ?", userID).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	for _, table := range []string{"totp_factors", "passkeys", "recovery_codes", "auth_challenges", "sessions"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE user_id = ?", userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RemoveTotp removes userID's authenticator, ending their sessions other than keepSessionID.
// It returns ErrNotFound when userID has none. See removeFactors for the cleanup.
func RemoveTotp(db *sql.DB, userID int64, keepSessionID string) error {
	return removeFactors(db, userID, keepSessionID, ErrNotFound, []any{userID}, "DELETE FROM totp_factors WHERE user_id = ?")
}

// TurnOffTwoFactor removes all of userID's factors and recovery codes, ending their sessions
// other than keepSessionID. It returns ErrNoFactor when userID has no factor.
func TurnOffTwoFactor(db *sql.DB, userID int64, keepSessionID string) error {
	return removeFactors(db, userID, keepSessionID, ErrNoFactor, []any{userID},
		"DELETE FROM totp_factors WHERE user_id = ?", "DELETE FROM passkeys WHERE user_id = ?")
}

// removeFactors runs each of dels with args in one transaction with its cleanup, returning
// missing when they delete no row, and ErrTwoFactorRequired, changing nothing, when they would
// leave userID without a factor while 2FA is required (FR-035). The cleanup ends userID's sessions other than keepSessionID (FR-024) and
// their pending steps, and when no factor is left deletes their recovery codes (FR-025).
func removeFactors(db *sql.DB, userID int64, keepSessionID string, missing error, args []any, dels ...string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int64
	for _, q := range dels {
		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		k, err := res.RowsAffected()
		if err != nil {
			return err
		}
		n += k
	}
	if n == 0 {
		return missing
	}
	has, err := HasTwoFactor(tx, userID)
	if err != nil {
		return err
	}
	if !has {
		// Read in the transaction that deleted, so a concurrent "require" cannot slip in between.
		if required, err := RequireTwoFactor(tx); err != nil {
			return err
		} else if required {
			return ErrTwoFactorRequired
		}
		if _, err := tx.Exec("DELETE FROM recovery_codes WHERE user_id = ?", userID); err != nil {
			return err
		}
	}
	if err := DeleteChallengesFor(tx, userID); err != nil {
		return err
	}
	if err := DeleteOtherSessions(tx, userID, keepSessionID); err != nil {
		return err
	}
	return tx.Commit()
}

// RequireTwoFactor reports whether every admin must have 2FA: settings key require_2fa is
// exactly "1". Missing or any other value reads as off (FR-026).
func RequireTwoFactor(q settings.Querier) (bool, error) {
	v, _, err := settings.Get(q, requireKey)
	return v == "1", err
}

// SetRequireTwoFactor stores the require setting as "1" or "0" at now.
func SetRequireTwoFactor(q settings.Querier, on bool, now time.Time) error {
	v := "0"
	if on {
		v = "1"
	}
	return settings.Set(q, requireKey, v, now)
}

// SetRequireTwoFactorBy stores the require setting as on for admin userID, who must have a
// second factor (FR-033): otherwise it returns ErrNoFactor and stores nothing. The write comes
// first in one transaction with the check, so a factor removed meanwhile cannot leave the
// setting on while its admin has none.
func SetRequireTwoFactorBy(db *sql.DB, userID int64, on bool, now time.Time) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := SetRequireTwoFactor(tx, on, now); err != nil {
		return err
	}
	has, err := HasTwoFactor(tx, userID)
	if err != nil {
		return err
	}
	if !has {
		return ErrNoFactor
	}
	return tx.Commit()
}
