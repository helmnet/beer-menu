package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"unicode/utf8"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// getTwoFactor handles GET /api/2fa (enroll): 200 with the caller's TwoFactorState. The
// availability fields say whether this server can add an authenticator (an encryption key is
// set) and passkeys (BEER_MENU_PUBLIC_URL is set, whose origin is passkeys_origin). It never
// returns a secret, public key or credential ID.
func getTwoFactor(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := principalFrom(r.Context())
		state, err := auth.LoadTwoFactorState(deps.DB, p.userID())
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		state.TotpAvailable = deps.Box != nil
		if deps.WebAuthn != nil {
			state.PasskeysAvailable = true
			state.PasskeysOrigin = &deps.WebAuthn.Config.RPOrigins[0]
		}
		writeJSON(w, http.StatusOK, state)
	})
}

// maxWebAuthnBody bounds request bodies that carry a WebAuthn response.
const maxWebAuthnBody = 16 << 10

type confirmBody struct {
	Password     string          `json:"password"`
	Code         *string         `json:"code"`
	RecoveryCode *string         `json:"recovery_code"`
	Passkey      json.RawMessage `json:"passkey"`
}

// confirmIdentity handles POST /api/2fa/confirm (admin): it checks the current password and,
// when the caller has a second factor, exactly one proof (code, recovery_code, which is spent,
// or passkey, an assertion for the options of POST /api/2fa/confirm/options), then stamps the
// session as confirmed for auth.ConfirmWindow and answers 204 (research R9). A missing, extra
// or unexpected proof is 400. The rate limit for the username is checked first (429); a wrong
// password is 403 wrong_password, a wrong code 403 invalid_code and a refused passkey 403
// passkey_failed, and each counts as a sign-in failure.
func confirmIdentity(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body confirmBody
		if !decodeJSON(w, r, &body, maxWebAuthnBody) {
			return
		}
		p, _ := principalFrom(r.Context())
		admin, err := auth.GetAdmin(deps.DB, p.AdminID)
		if errors.Is(err, auth.ErrNotFound) {
			// Removed between the session lookup and now.
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
			return
		}
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		proofs := 0
		for _, f := range []bool{body.Code != nil, body.RecoveryCode != nil, len(body.Passkey) > 0} {
			if f {
				proofs++
			}
		}
		if (admin.TwoFactor && proofs != 1) || (!admin.TwoFactor && proofs != 0) {
			writeError(w, http.StatusBadRequest, "invalid_request", "Send the password, plus one code when two-factor sign-in is on.")
			return
		}
		if retry, ok := deps.Limiter.Check(admin.Username); !ok {
			slog.WarnContext(r.Context(), "sign-in rate limited", "username", admin.Username)
			writeRateLimited(w, retry)
			return
		}
		failed := false
		defer func() { deps.Limiter.Done(admin.Username, failed) }()
		err = auth.CheckPassword(deps.DB, admin.ID, body.Password)
		if errors.Is(err, auth.ErrWrongPassword) {
			failed = true
			slog.WarnContext(r.Context(), "confirm failed, wrong password", "username", admin.Username)
			writeError(w, http.StatusForbidden, "wrong_password", "The current password is wrong.")
			return
		}
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		ok, failCode, failMsg := true, "invalid_code", "That code did not work."
		switch {
		case body.Code != nil:
			ok, err = auth.CheckTotp(deps.DB, deps.Box, admin.ID, *body.Code, deps.Now())
		case body.RecoveryCode != nil:
			ok, err = auth.UseRecoveryCode(deps.DB, admin.ID, *body.RecoveryCode, deps.Now())
		case len(body.Passkey) > 0:
			ok, err = checkPasskey(r, deps, p.SessionID, auth.KindSetup, admin, body.Passkey)
			failCode, failMsg = "passkey_failed", "The passkey was not accepted."
		}
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		if !ok {
			failed = true
			slog.WarnContext(r.Context(), "confirm failed, wrong second factor", "username", admin.Username)
			writeError(w, http.StatusForbidden, failCode, failMsg)
			return
		}
		if err := auth.ConfirmSession(deps.DB, p.SessionID, deps.Now()); err != nil {
			writeInternal(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "identity confirmed", "username", admin.Username)
		w.WriteHeader(http.StatusNoContent)
	})
}

// requireConfirmed reports whether principal p may make a sensitive 2FA change now: its
// session was confirmed or signed in within auth.ConfirmWindow, or it is an enroll pending,
// whose password was just checked. Otherwise it answers 403 confirm_required (or 500) and
// returns false.
func requireConfirmed(w http.ResponseWriter, r *http.Request, deps Deps, p principal) bool {
	if p.Pending != "" {
		return true
	}
	ok, err := auth.SessionConfirmed(deps.DB, p.SessionID, deps.Now())
	if err != nil {
		writeInternal(w, r, err)
		return false
	}
	if !ok {
		writeError(w, http.StatusForbidden, "confirm_required", "Confirm it is you first.")
	}
	return ok
}

// codeBody is the CodeBody request of the code routes: one code, spaces and dashes ignored.
type codeBody struct {
	Code string `json:"code"`
}

// decodeCode reads a CodeBody, answering 400 invalid_request and returning false unless code
// is 1 to 32 characters.
func decodeCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body codeBody
	if !decodeJSON(w, r, &body, maxBody) {
		return "", false
	}
	if n := utf8.RuneCountInString(body.Code); n < 1 || n > 32 {
		writeError(w, http.StatusBadRequest, "invalid_request", "Send a code of 1 to 32 characters.")
		return "", false
	}
	return body.Code, true
}

// setupCaller returns the principal (a session, or on AccessEnroll routes an enroll pending)
// and admin of a factor setup or removal route once the caller may make the change: an admin
// removed since the credential lookup is 401, and a session without a recent confirm is 403
// confirm_required; an enroll pending counts as confirmed. Otherwise it answers and returns
// false.
func setupCaller(w http.ResponseWriter, r *http.Request, deps Deps) (principal, auth.Admin, bool) {
	p, _ := principalFrom(r.Context())
	admin, err := auth.GetAdmin(deps.DB, p.userID())
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
		return p, admin, false
	}
	if err != nil {
		writeInternal(w, r, err)
		return p, admin, false
	}
	return p, admin, requireConfirmed(w, r, deps, p)
}

// writeTotpSetupError answers an error of auth.StartTotpSetup or auth.FinishTotpSetup: 409
// totp_unavailable without an encryption key, 409 already_set_up for a second authenticator,
// 401 for an enroll pending gone meanwhile, else 500.
func writeTotpSetupError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
	case errors.Is(err, auth.ErrTotpUnavailable):
		writeError(w, http.StatusConflict, "totp_unavailable", "Authenticator apps are off: the server has no encryption key.")
	case errors.Is(err, auth.ErrAlreadySetUp):
		writeError(w, http.StatusConflict, "already_set_up", "An authenticator app is already set up.")
	default:
		writeInternal(w, r, err)
	}
}

// totpSetupReply is the TotpSetup body of POST /api/2fa/totp/setup.
type totpSetupReply struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// startTotpSetup handles POST /api/2fa/totp/setup (enroll): after the setupCaller checks it
// answers 200 with a new authenticator secret (base32) and its otpauth URI, held unconfirmed on
// the server for 5 minutes (with an enroll pending, until that sign-in expires). 409 totp_unavailable without an
// encryption key, 409 already_set_up when the caller has an authenticator.
func startTotpSetup(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		id, kind := p.setupChallenge()
		secret, uri, err := auth.StartTotpSetup(deps.DB, deps.Box, id, kind, admin.ID, admin.Username, deps.Now())
		if err != nil {
			writeTotpSetupError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, totpSetupReply{Secret: secret, URI: uri})
	})
}

// factorAdded is the FactorAdded body: the passkey when one was added, recovery codes only
// when this was the first factor, and the admin only when it finished an enroll sign-in.
type factorAdded struct {
	Passkey       *auth.Passkey `json:"passkey,omitempty"`
	RecoveryCodes []string      `json:"recovery_codes,omitempty"`
	Admin         *auth.Admin   `json:"admin,omitempty"`
}

// writeFactorAdded answers 201 with reply for a factor admin added. For an enroll principal
// the factor finishes the sign-in (research R10): the auth finish already used up the pending
// sign-in, so it clears bm_pending, starts a session as POST /api/session does and adds the
// admin to reply.
func writeFactorAdded(w http.ResponseWriter, r *http.Request, deps Deps, p principal, admin auth.Admin, reply factorAdded) {
	if p.Pending != "" {
		clearPendingCookie(w, deps)
		if !startSession(w, r, deps, admin.ID) {
			return
		}
		admin.TwoFactor = true
		reply.Admin = &admin
		slog.InfoContext(r.Context(), "admin signed in", "username", admin.Username, "enrolled", true)
	}
	writeJSON(w, http.StatusCreated, reply)
}

// finishTotpSetup handles POST /api/2fa/totp (enroll): after the setupCaller checks, a CodeBody
// valid for the secret from POST /api/2fa/totp/setup saves the authenticator and answers 201
// FactorAdded; for the first factor that carries 10 recovery codes and the caller's other
// sessions end. With an enroll pending it also signs in (see writeFactorAdded). A wrong code, or no open setup, is 400
// invalid_code and saves nothing; 409 as for the setup start.
func finishTotpSetup(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, ok := decodeCode(w, r)
		if !ok {
			return
		}
		p, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		id, kind := p.setupChallenge()
		codes, ok, err := auth.FinishTotpSetup(deps.DB, deps.Box, id, kind, admin.ID, code, deps.Now())
		if err != nil {
			writeTotpSetupError(w, r, err)
			return
		}
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_code", "That code did not work.")
			return
		}
		slog.InfoContext(r.Context(), "authenticator added", "username", admin.Username, "first_factor", codes != nil)
		writeFactorAdded(w, r, deps, p, admin, factorAdded{RecoveryCodes: codes})
	})
}

// checkPasskey reports whether assertion passes the passkey check that BeginPasskeyLogin stored
// on challenge id of kind for admin. A refusal, or passkeys being off, is false with a nil error
// and logs the library's fixed reason, never the assertion.
func checkPasskey(r *http.Request, deps Deps, id string, kind auth.ChallengeKind, admin auth.Admin, assertion []byte) (bool, error) {
	err := auth.FinishPasskeyLogin(deps.DB, deps.WebAuthn, id, kind, admin.ID, assertion, deps.Now())
	if errors.Is(err, auth.ErrPasskeyFailed) || errors.Is(err, auth.ErrPasskeysUnavailable) {
		slog.InfoContext(r.Context(), "passkey check failed", "username", admin.Username, "reason", err.Error())
		return false, nil
	}
	return err == nil, err
}

// writePasskeyError answers an error of the auth passkey functions: 409 passkeys_unavailable
// without BEER_MENU_PUBLIC_URL, 409 too_many_passkeys at auth.MaxPasskeys, 409 no_factor with
// no passkey to check, 400 for a bad name, 401 for an admin removed meanwhile, else 500.
func writePasskeyError(w http.ResponseWriter, r *http.Request, err error) {
	var ve auth.ValidationError
	switch {
	case errors.Is(err, auth.ErrPasskeysUnavailable):
		writeError(w, http.StatusConflict, "passkeys_unavailable", "Passkeys are off: the server has no public URL.")
	case errors.Is(err, auth.ErrTooManyPasskeys):
		writeError(w, http.StatusConflict, "too_many_passkeys", "You have 10 passkeys. Remove one before adding another.")
	case errors.Is(err, auth.ErrNoPasskeys):
		writeError(w, http.StatusConflict, "no_factor", "No passkey is set up.")
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_request", ve.Error())
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
	default:
		writeInternal(w, r, err)
	}
}

// startPasskeySetup handles POST /api/2fa/passkeys/options (enroll): after the setupCaller
// checks it answers 200 with WebAuthn creation options for a new passkey (user verification
// required, resident key preferred, no attestation, the caller's passkeys excluded), held on
// the server for 5 minutes (with an enroll pending, until that sign-in expires). 409 passkeys_unavailable without
// BEER_MENU_PUBLIC_URL, 409 too_many_passkeys at 10.
func startPasskeySetup(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		id, kind := p.setupChallenge()
		opts, err := auth.BeginPasskeyRegistration(deps.DB, deps.WebAuthn, id, kind, admin.ID, deps.Now())
		if err != nil {
			writePasskeyError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, opts)
	})
}

// passkeySetupBody is the body of POST /api/2fa/passkeys.
type passkeySetupBody struct {
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
}

// finishPasskeySetup handles POST /api/2fa/passkeys (enroll): after the setupCaller checks, a name (1 to 64 characters, trimmed, no control characters) and the
// credential.toJSON() of a creation for the options of POST /api/2fa/passkeys/options save the
// passkey and answer 201 FactorAdded with it; for the first factor that carries 10 recovery
// codes and the caller's other sessions end. With an enroll pending it also signs in (see
// writeFactorAdded). A bad name or body (over 16 KiB) is 400
// invalid_request, a refused credential or no open setup 400 passkey_failed; 409 as for the
// options.
func finishPasskeySetup(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body passkeySetupBody
		if !decodeJSON(w, r, &body, maxWebAuthnBody) {
			return
		}
		if len(body.Credential) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "Send the passkey's name and credential.")
			return
		}
		p, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		id, kind := p.setupChallenge()
		pk, codes, err := auth.FinishPasskeyRegistration(deps.DB, deps.WebAuthn, id, kind, admin.ID, body.Name, body.Credential, deps.Now())
		if errors.Is(err, auth.ErrPasskeyFailed) {
			slog.WarnContext(r.Context(), "passkey not added", "username", admin.Username, "reason", err.Error())
			writeError(w, http.StatusBadRequest, "passkey_failed", "The passkey could not be added. Try again.")
			return
		}
		if err != nil {
			writePasskeyError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "passkey added", "username", admin.Username, "first_factor", codes != nil)
		writeFactorAdded(w, r, deps, p, admin, factorAdded{Passkey: &pk, RecoveryCodes: codes})
	})
}

// confirmPasskeyOptions handles POST /api/2fa/confirm/options (admin): 200 with WebAuthn request
// options for a passkey proof in POST /api/2fa/confirm, allowing only the caller's passkeys with
// user verification required, held on the session for 5 minutes. 409 passkeys_unavailable
// without BEER_MENU_PUBLIC_URL, 409 no_factor when the caller has no passkey.
func confirmPasskeyOptions(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := principalFrom(r.Context())
		opts, err := auth.BeginPasskeyLogin(deps.DB, deps.WebAuthn, p.SessionID, auth.KindSetup, p.AdminID, deps.Now())
		if err != nil {
			writePasskeyError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, opts)
	})
}

// recoveryCodesReply is the RecoveryCodes body.
type recoveryCodesReply struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// replaceRecoveryCodes handles POST /api/2fa/recovery-codes (admin): after the setupCaller
// checks it makes 10 new recovery codes, which end every earlier one (FR-019), and answers 200
// with them, shown once. 409 no_factor when the caller has no second factor.
func replaceRecoveryCodes(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		// shortcut: a factor removed between this check and the insert leaves unused codes, which
		// sign in nobody (sign-in offers recovery only with a factor); make it one transaction if
		// that ever matters.
		if !admin.TwoFactor {
			writeError(w, http.StatusConflict, "no_factor", "Turn on two-factor sign-in first.")
			return
		}
		codes, err := auth.NewRecoveryCodes(deps.DB, admin.ID)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "recovery codes replaced", "username", admin.Username)
		writeJSON(w, http.StatusOK, recoveryCodesReply{RecoveryCodes: codes})
	})
}

// passkeyNameBody is the body of PATCH /api/2fa/passkeys/{id}.
type passkeyNameBody struct {
	Name string `json:"name"`
}

// renamePasskey handles PATCH /api/2fa/passkeys/{id} (admin): it renames one of the caller's
// passkeys (1 to 64 characters, trimmed, no control characters) and answers 200 with the
// Passkey. It needs no recent confirm (research R9). A bad name is 400 invalid_request; a
// passkey that is not the caller's is 404 not_found.
func renamePasskey(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		var body passkeyNameBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		p, _ := principalFrom(r.Context())
		pk, err := auth.RenamePasskey(deps.DB, p.AdminID, id, body.Name)
		var ve auth.ValidationError
		switch {
		case errors.As(err, &ve):
			writeError(w, http.StatusBadRequest, "invalid_request", ve.Error())
		case errors.Is(err, auth.ErrNotFound):
			writeNotFound(w)
		case err != nil:
			writeInternal(w, r, err)
		default:
			writeJSON(w, http.StatusOK, pk)
		}
	})
}

// removeFactor answers a factor removal route: after the setupCaller checks it runs remove for
// the caller, keeping this session, and answers 204 logging event. remove's auth.ErrNotFound is
// 404 not_found, auth.ErrNoFactor 409 no_factor and auth.ErrTwoFactorRequired 409
// two_factor_required, whose message names the setting.
func removeFactor(w http.ResponseWriter, r *http.Request, deps Deps, event string, remove func(userID int64, keepSessionID string) error) {
	p, admin, ok := setupCaller(w, r, deps)
	if !ok {
		return
	}
	err := remove(admin.ID, p.SessionID)
	switch {
	case errors.Is(err, auth.ErrNotFound):
		writeNotFound(w)
	case errors.Is(err, auth.ErrNoFactor):
		writeError(w, http.StatusConflict, "no_factor", "Two-factor sign-in is already off.")
	case errors.Is(err, auth.ErrTwoFactorRequired):
		writeError(w, http.StatusConflict, "two_factor_required", requiredMessage)
	case err != nil:
		writeInternal(w, r, err)
	default:
		slog.InfoContext(r.Context(), event, "username", admin.Username)
		w.WriteHeader(http.StatusNoContent)
	}
}

// removePasskey handles DELETE /api/2fa/passkeys/{id} (admin): with a recent confirm it removes
// one of the caller's passkeys and ends their other sessions (FR-024); the last factor also
// takes the recovery codes (FR-025). 204; a passkey that is not the caller's is 404 not_found.
func removePasskey(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		removeFactor(w, r, deps, "passkey removed", func(userID int64, keep string) error {
			return auth.RemovePasskey(deps.DB, userID, id, keep)
		})
	})
}

// removeTotp handles DELETE /api/2fa/totp (admin): with a recent confirm it removes the
// caller's authenticator, by the rules of removePasskey. 404 not_found when there is none.
func removeTotp(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		removeFactor(w, r, deps, "authenticator removed", func(userID int64, keep string) error {
			return auth.RemoveTotp(deps.DB, userID, keep)
		})
	})
}

// turnOffTwoFactor handles DELETE /api/2fa (admin): with a recent confirm it removes all of the
// caller's factors and recovery codes and ends their other sessions, then answers 204. 409
// no_factor when two-factor sign-in is already off.
func turnOffTwoFactor(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		removeFactor(w, r, deps, "two-factor turned off", func(userID int64, keep string) error {
			return auth.TurnOffTwoFactor(deps.DB, userID, keep)
		})
	})
}

// requiredMessage answers a change refused because it would leave the caller without a factor
// while every admin must have one; it names the setting (FR-035).
const requiredMessage = "\"Require two-factor sign-in for all admins\" is on, so you must keep at least one way to sign in. Add another first, or turn that setting off."

// requiredBody is the body of PUT /api/2fa/required. Required is a pointer so a missing or null
// field is told apart from false.
type requiredBody struct {
	Required *bool `json:"required"`
}

// setTwoFactorRequired handles PUT /api/2fa/required (admin): after the setupCaller checks
// (a recent confirm), it stores whether every admin must have 2FA and answers 204, logging who
// changed it to what. The caller must have a factor of their own, else 409 no_factor (FR-033).
// Turning it on when the server can enroll no factor (no encryption key and no
// BEER_MENU_PUBLIC_URL) is 409 two_factor_unavailable, so no admin is locked out of enrolling.
// A body without a boolean required is 400. It ends no sessions: admins without a factor enroll
// at their next sign-in (FR-036).
func setTwoFactorRequired(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body requiredBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		if body.Required == nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Send required as true or false.")
			return
		}
		_, admin, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		if *body.Required && deps.Box == nil && deps.WebAuthn == nil {
			writeError(w, http.StatusConflict, "two_factor_unavailable", "No admin could set up two-factor sign-in: the server has no encryption key and no public URL.")
			return
		}
		err := auth.SetRequireTwoFactorBy(deps.DB, admin.ID, *body.Required, deps.Now())
		switch {
		case errors.Is(err, auth.ErrNoFactor):
			writeError(w, http.StatusConflict, "no_factor", "Turn on two-factor sign-in for yourself first.")
		case err != nil:
			writeInternal(w, r, err)
		default:
			slog.InfoContext(r.Context(), "two-factor requirement changed", "username", admin.Username, "required", *body.Required)
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
