package auth

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// MaxPasskeys is how many passkeys one admin may have (FR-013).
const MaxPasskeys = 10

// Errors of passkeys.
var (
	// ErrPasskeysUnavailable means the server has no BEER_MENU_PUBLIC_URL, so it cannot check
	// passkeys (research R5).
	ErrPasskeysUnavailable = errors.New("passkeys need BEER_MENU_PUBLIC_URL")
	// ErrTooManyPasskeys means the admin already has MaxPasskeys passkeys.
	ErrTooManyPasskeys = errors.New("an admin has at most 10 passkeys")
	// ErrNoPasskeys means the admin has no passkey to sign in with.
	ErrNoPasskeys = errors.New("no passkey is set up")
	// ErrPasskeyFailed means a passkey response was refused: no open ceremony for it, or it
	// failed a WebAuthn check. Its message carries only the library's short reason, never the
	// raw response, so it can be logged.
	ErrPasskeyFailed = errors.New("passkey refused")
)

// passkeyUser is an admin as go-webauthn sees them: ID is the 8-byte big-endian user ID.
type passkeyUser struct {
	id    int64
	name  string
	creds []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return binary.BigEndian.AppendUint64(nil, uint64(u.id)) }
func (u passkeyUser) WebAuthnName() string                       { return u.name }
func (u passkeyUser) WebAuthnDisplayName() string                { return u.name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// loadPasskeyUser reads userID's username and stored passkeys.
func loadPasskeyUser(db *sql.DB, userID int64) (passkeyUser, error) {
	u := passkeyUser{id: userID}
	if err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&u.name); errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	} else if err != nil {
		return u, err
	}
	rows, err := db.Query("SELECT credential_id, public_key, sign_count, transports, backup_eligible FROM passkeys WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		return u, err
	}
	defer rows.Close()
	for rows.Next() {
		var c webauthn.Credential
		var transports string
		if err := rows.Scan(&c.ID, &c.PublicKey, &c.Authenticator.SignCount, &transports, &c.Flags.BackupEligible); err != nil {
			return u, err
		}
		if err := json.Unmarshal([]byte(transports), &c.Transport); err != nil {
			return u, err
		}
		u.creds = append(u.creds, c)
	}
	return u, rows.Err()
}

// ceremony is what the webauthn column of a challenge holds: the library's session data and
// whether it is for creating a passkey, so a registration's challenge cannot finish a login
// and the other way round.
type ceremony struct {
	Create  bool                 `json:"create"`
	Session webauthn.SessionData `json:"session"`
}

// refused wraps a WebAuthn check failure as ErrPasskeyFailed with the library's reason
// (protocol.Error.Details: a fixed text, at most with an algorithm or a parse error, never
// Info, which can quote values). Other errors (such as JSON syntax errors that quote the input) give no reason.
func refused(err error) error {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		return fmt.Errorf("%w: %s", ErrPasskeyFailed, pe.Details)
	}
	return ErrPasskeyFailed
}

// ValidatePasskeyName trims name and returns it when it is 1 to 64 code points without control
// characters (FR-013), else a ValidationError.
func ValidatePasskeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", ValidationError("Name the passkey with 1 to 64 characters, no control characters.")
	}
	return name, nil
}

// RenamePasskey sets the name of userID's passkey id to name, trimmed, and returns it. A bad
// name is a ValidationError; a passkey that is not userID's is ErrNotFound.
func RenamePasskey(db *sql.DB, userID, id int64, name string) (Passkey, error) {
	name, err := ValidatePasskeyName(name)
	if err != nil {
		return Passkey{}, err
	}
	pk := Passkey{ID: id, Name: name}
	var created int64
	var used sql.NullInt64
	err = db.QueryRow("UPDATE passkeys SET name = ? WHERE id = ? AND user_id = ? RETURNING created_at, last_used_at", name, id, userID).Scan(&created, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return Passkey{}, ErrNotFound
	}
	pk.CreatedAt, pk.LastUsedAt = unixTime(created), nullTime(used)
	return pk, err
}

// RemovePasskey removes userID's passkey id, ending their sessions other than keepSessionID.
// A passkey that is not userID's is ErrNotFound. The last factor also takes the recovery codes
// (FR-025).
func RemovePasskey(db *sql.DB, userID, id int64, keepSessionID string) error {
	return removeFactors(db, userID, keepSessionID, ErrNotFound, []any{userID, id}, "DELETE FROM passkeys WHERE user_id = ? AND id = ?")
}

// BeginPasskeyRegistration starts adding a passkey for userID on setup challenge id of kind
// (see openSetup): it keeps the creation ceremony on that challenge until it expires,
// replacing any earlier ceremony, and returns the options for the browser (user verification
// required, resident key preferred, no attestation, the admin's passkeys excluded). It returns
// ErrPasskeysUnavailable when wa is nil and ErrTooManyPasskeys at MaxPasskeys.
func BeginPasskeyRegistration(db *sql.DB, wa *webauthn.WebAuthn, id string, kind ChallengeKind, userID int64, now time.Time) (*protocol.CredentialCreation, error) {
	if wa == nil {
		return nil, ErrPasskeysUnavailable
	}
	u, err := loadPasskeyUser(db, userID)
	if err != nil {
		return nil, err
	}
	if len(u.creds) >= MaxPasskeys {
		return nil, ErrTooManyPasskeys
	}
	opts, session, err := wa.BeginRegistration(u,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(webauthn.Credentials(u.creds).CredentialDescriptors()))
	if err != nil {
		return nil, err
	}
	if err := openSetup(db, id, kind, userID, now); err != nil {
		return nil, err
	}
	if err := storeCeremony(db, id, kind, ceremony{Create: true, Session: *session}, now); err != nil {
		return nil, err
	}
	return opts, nil
}

// FinishPasskeyRegistration saves the passkey in response (credential.toJSON() of the
// creation) under name when it answers the open registration on setup challenge id of kind for
// userID, and uses up the challenge (for KindEnroll, the pending sign-in). When it is userID's
// first factor it also makes and returns 10 recovery codes and ends userID's sessions other
// than id (FR-017, FR-024), all in one transaction. A bad name is a
// ValidationError. A refused response, or no open registration, is ErrPasskeyFailed and leaves
// the setup open until it expires. It returns ErrPasskeysUnavailable when wa is nil and
// ErrTooManyPasskeys at MaxPasskeys. For KindEnroll it returns ErrNotFound, changing nothing,
// when userID already has a factor.
func FinishPasskeyRegistration(db *sql.DB, wa *webauthn.WebAuthn, id string, kind ChallengeKind, userID int64, name string, response []byte, now time.Time) (Passkey, []string, error) {
	if wa == nil {
		return Passkey{}, nil, ErrPasskeysUnavailable
	}
	name, err := ValidatePasskeyName(name)
	if err != nil {
		return Passkey{}, nil, err
	}
	c, ok, err := PeekChallenge(db, id, kind, now)
	if err != nil {
		return Passkey{}, nil, err
	}
	var cer ceremony
	if !ok || c.UserID != userID || json.Unmarshal([]byte(c.WebAuthn), &cer) != nil || !cer.Create {
		return Passkey{}, nil, ErrPasskeyFailed
	}
	u, err := loadPasskeyUser(db, userID)
	if err != nil {
		return Passkey{}, nil, err
	}
	if len(u.creds) >= MaxPasskeys {
		return Passkey{}, nil, ErrTooManyPasskeys
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return Passkey{}, nil, refused(err)
	}
	cred, err := wa.CreateCredential(u, cer.Session, parsed)
	if err != nil {
		return Passkey{}, nil, refused(err)
	}
	transports, err := json.Marshal(cred.Transport)
	if err != nil {
		return Passkey{}, nil, err
	}
	if cred.Transport == nil {
		transports = []byte("[]")
	}

	tx, err := db.Begin()
	if err != nil {
		return Passkey{}, nil, err
	}
	defer tx.Rollback()
	// Taking the setup in the transaction makes a response accepted by two concurrent calls save once.
	if _, ok, err := TakeChallenge(tx, id, kind, now); err != nil {
		return Passkey{}, nil, err
	} else if !ok {
		return Passkey{}, nil, ErrPasskeyFailed
	}
	var count int
	if err := tx.QueryRow("SELECT count(*) FROM passkeys WHERE user_id = ?", userID).Scan(&count); err != nil {
		return Passkey{}, nil, err
	}
	if count >= MaxPasskeys {
		return Passkey{}, nil, ErrTooManyPasskeys
	}
	has, err := HasTwoFactor(tx, userID)
	if err != nil {
		return Passkey{}, nil, err
	}
	if has && kind == KindEnroll {
		// A factor added since this enroll pending began (a race with sign-in) must be proven instead.
		return Passkey{}, nil, ErrNotFound
	}
	pk := Passkey{Name: name, CreatedAt: unixTime(now.Unix())}
	err = tx.QueryRow(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (credential_id) DO NOTHING RETURNING id`,
		userID, name, cred.ID, cred.PublicKey, cred.Authenticator.SignCount, string(transports), cred.Flags.BackupEligible, now.Unix()).Scan(&pk.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return Passkey{}, nil, fmt.Errorf("%w: credential already registered", ErrPasskeyFailed)
	}
	if err != nil {
		return Passkey{}, nil, err
	}
	var codes []string
	if !has {
		if codes, err = firstFactorAdded(tx, userID, id); err != nil {
			return Passkey{}, nil, err
		}
	}
	return pk, codes, tx.Commit()
}

// BeginPasskeyLogin starts a passkey check for userID on challenge id of kind: a second_factor
// pending sign-in (id from bm_pending), or for a confirm a setup row keyed by session id, which
// it starts, replacing any earlier setup of that session. The request ceremony replaces any
// earlier one of that challenge. The options allow only userID's passkeys and require user
// verification. It returns ErrPasskeysUnavailable when wa is nil, ErrNoPasskeys when userID
// has none, and ErrPasskeyFailed when there is no such unexpired challenge.
func BeginPasskeyLogin(db *sql.DB, wa *webauthn.WebAuthn, id string, kind ChallengeKind, userID int64, now time.Time) (*protocol.CredentialAssertion, error) {
	if wa == nil {
		return nil, ErrPasskeysUnavailable
	}
	u, err := loadPasskeyUser(db, userID)
	if err != nil {
		return nil, err
	}
	if len(u.creds) == 0 {
		return nil, ErrNoPasskeys
	}
	opts, session, err := wa.BeginLogin(u, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, err
	}
	if kind == KindSetup {
		if err := SetupChallenge(db, id, userID, now); err != nil {
			return nil, err
		}
	} else if c, ok, err := PeekChallenge(db, id, kind, now); err != nil {
		return nil, err
	} else if !ok || c.UserID != userID {
		return nil, ErrPasskeyFailed
	}
	if err := storeCeremony(db, id, kind, ceremony{Session: *session}, now); err != nil {
		return nil, err
	}
	return opts, nil
}

// FinishPasskeyLogin checks response (credential.toJSON() of the assertion) against the request
// ceremony that BeginPasskeyLogin stored on challenge id of kind for userID: origin, RP ID,
// user verification, signature, and a sign count that rises unless it stays zero. The ceremony
// is used up whatever the outcome; the challenge row itself stays for the caller. On success
// it stores the new sign count and now as last used. A refusal is ErrPasskeyFailed. It returns
// ErrPasskeysUnavailable when wa is nil.
func FinishPasskeyLogin(db *sql.DB, wa *webauthn.WebAuthn, id string, kind ChallengeKind, userID int64, response []byte, now time.Time) error {
	if wa == nil {
		return ErrPasskeysUnavailable
	}
	data, ok, err := takeCeremony(db, id, kind, userID, now)
	if err != nil {
		return err
	}
	var cer ceremony
	if !ok || json.Unmarshal([]byte(data), &cer) != nil || cer.Create {
		return ErrPasskeyFailed
	}
	u, err := loadPasskeyUser(db, userID)
	if err != nil {
		return err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return refused(err)
	}
	cred, err := wa.ValidateLogin(u, cer.Session, parsed)
	if err != nil {
		return refused(err)
	}
	if cred.Authenticator.CloneWarning {
		return fmt.Errorf("%w: sign count did not increase", ErrPasskeyFailed)
	}
	// The sign_count condition refuses this assertion if another changed the count since it was read.
	var old uint32
	for _, c := range u.creds {
		if string(c.ID) == string(cred.ID) {
			old = c.Authenticator.SignCount
		}
	}
	res, err := db.Exec("UPDATE passkeys SET sign_count = ?, last_used_at = ? WHERE user_id = ? AND credential_id = ? AND sign_count = ?",
		cred.Authenticator.SignCount, now.Unix(), userID, cred.ID, old)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrPasskeyFailed
	}
	return nil
}

// storeCeremony saves c on challenge id of kind. A challenge gone since the caller saw it is
// ErrPasskeyFailed.
func storeCeremony(db *sql.DB, id string, kind ChallengeKind, c ceremony, now time.Time) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	ok, err := SetChallengeData(db, id, kind, string(b), "", now)
	if err == nil && !ok {
		err = ErrPasskeyFailed
	}
	return err
}
