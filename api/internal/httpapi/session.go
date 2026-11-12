package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// pendingCookieName is the cookie of an unfinished sign-in (research R8).
const pendingCookieName = "bm_pending"

// setPendingCookie sends pending sign-in id as the HttpOnly, SameSite=Strict bm_pending cookie
// for /api, living as long as the challenge, with Secure when deps.CookieSecure is set.
func setPendingCookie(w http.ResponseWriter, deps Deps, id string) {
	http.SetCookie(w, &http.Cookie{
		Name: pendingCookieName, Value: id, Path: "/api", MaxAge: int(auth.ChallengeTTL / time.Second),
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: deps.CookieSecure,
	})
}

// clearPendingCookie tells the browser to drop bm_pending.
func clearPendingCookie(w http.ResponseWriter, deps Deps) {
	http.SetCookie(w, &http.Cookie{
		Name: pendingCookieName, Value: "", Path: "/api", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: deps.CookieSecure,
	})
}

// pendingFrom returns the ID and challenge of the request's bm_pending sign-in of kind, without
// using it up. ok is false when there is no cookie or it names no unexpired challenge of kind.
func pendingFrom(deps Deps, r *http.Request, kind auth.ChallengeKind) (id string, c auth.Challenge, ok bool, err error) {
	cookie, err := r.Cookie(pendingCookieName)
	if err != nil {
		return "", auth.Challenge{}, false, nil
	}
	c, ok, err = auth.PeekChallenge(deps.DB, cookie.Value, kind, deps.Now())
	return cookie.Value, c, ok, err
}

// secondFactorPending is the 202 body of POST /api/session for next=second_factor (contract
// SignInPending).
type secondFactorPending struct {
	Next           string   `json:"next"`
	Methods        []string `json:"methods"`
	PasskeysOrigin string   `json:"passkeys_origin,omitempty"`
}

// signIn handles POST /api/session (public). It checks the per-username rate limit before
// any hashing (429 too_many_attempts with Retry-After in whole seconds), then the password.
// Unknown usernames and wrong passwords get the same 401 invalid_credentials and count as a
// failure for that username, whatever the admin's 2FA state (FR-004). For an admin with 2FA a
// right password answers 202 {"next":"second_factor","methods":[...]} and a bm_pending cookie;
// for an admin without one while 2FA is required, 202 {"next":"enroll"} and a bm_pending
// cookie. Otherwise success always mints a new session (FR-017), sets its cookie and returns
// 200 with the Admin. Bad input gives 400, saturated hashing 503 busy.
func signIn(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body credentialsBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		username, err := auth.ValidateUsername(body.Username)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		if retry, ok := deps.Limiter.Check(username); !ok {
			slog.WarnContext(r.Context(), "sign-in rate limited", "username", username)
			writeRateLimited(w, retry)
			return
		}
		failed := false
		defer func() { deps.Limiter.Done(username, failed) }()
		admin, err := auth.Authenticate(deps.DB, username, body.Password)
		if errors.Is(err, auth.ErrInvalidCredentials) {
			failed = true
			slog.WarnContext(r.Context(), "sign-in failed", "username", username)
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Wrong username or password.")
			return
		}
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		state, err := auth.LoadTwoFactorState(deps.DB, admin.ID)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		if state.Enabled || state.Required {
			kind, reply := auth.KindEnroll, any(map[string]string{"next": "enroll"})
			if state.Enabled {
				kind, reply = auth.KindSecondFactor, secondFactorReply(deps, state)
			}
			id, err := auth.CreateChallenge(deps.DB, admin.ID, kind, deps.Now())
			if err != nil {
				writeInternal(w, r, err)
				return
			}
			setPendingCookie(w, deps, id)
			slog.InfoContext(r.Context(), "sign-in password ok, second factor owed", "username", admin.Username)
			writeJSON(w, http.StatusAccepted, reply)
			return
		}
		if !startSession(w, r, deps, admin.ID) {
			return
		}
		slog.InfoContext(r.Context(), "admin signed in", "username", admin.Username)
		writeJSON(w, http.StatusOK, admin)
	})
}

// secondFactorReply lists the second steps an admin with state can finish sign-in with: the
// authenticator when the server holds the key to open it, passkeys when the server can check
// them (with their origin), and recovery while unused codes remain.
func secondFactorReply(deps Deps, state auth.TwoFactorState) secondFactorPending {
	reply := secondFactorPending{Next: "second_factor", Methods: []string{}}
	if state.Totp != nil && deps.Box != nil {
		reply.Methods = append(reply.Methods, "totp")
	}
	if len(state.Passkeys) > 0 && deps.WebAuthn != nil {
		reply.Methods = append(reply.Methods, "passkey")
		reply.PasskeysOrigin = deps.WebAuthn.Config.RPOrigins[0]
	}
	if state.RecoveryCodesLeft > 0 {
		reply.Methods = append(reply.Methods, "recovery")
	}
	return reply
}

// signInTotp handles POST /api/session/totp (public, but only with a bm_pending second_factor
// sign-in, else 401 unauthenticated): it finishes sign-in with an authenticator code, by the
// rules of finishSecondStep. A wrong or reused code is 401 invalid_code.
func signInTotp(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, ok := decodeCode(w, r)
		if !ok {
			return
		}
		finishSecondStep(w, r, deps, "totp", stepFailure{"invalid_code", "That code did not work.", "sign-in failed, wrong code"}, false,
			func(_ string, admin auth.Admin) (bool, error) {
				return checkTotp(r, deps, admin, code)
			})
	})
}

// signInRecovery handles POST /api/session/recovery (public, but only with a bm_pending
// second_factor sign-in, else 401 unauthenticated): it finishes sign-in with a recovery code
// (spaces, dashes and case ignored), by the rules of finishSecondStep. The code is spent in the
// same transaction that uses up the pending sign-in. A wrong or spent code is 401 invalid_code.
func signInRecovery(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, ok := decodeCode(w, r)
		if !ok {
			return
		}
		finishSecondStep(w, r, deps, "recovery", stepFailure{"invalid_code", "That code did not work.", "sign-in failed, wrong recovery code"}, true,
			func(id string, admin auth.Admin) (bool, error) {
				return auth.SignInWithRecoveryCode(deps.DB, id, admin.ID, code, deps.Now())
			})
	})
}

// signInPasskeyOptions handles POST /api/session/passkey/options (public, but only with a
// bm_pending second_factor sign-in, else 401 unauthenticated): 200 with WebAuthn request
// options allowing only the pending admin's passkeys, user verification required. A new call
// replaces the earlier challenge. 409 passkeys_unavailable without BEER_MENU_PUBLIC_URL, 409
// no_factor when the admin has no passkey.
func signInPasskeyOptions(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, c, ok, err := pendingFrom(deps, r, auth.KindSecondFactor)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		var opts any
		if ok {
			opts, err = auth.BeginPasskeyLogin(deps.DB, deps.WebAuthn, id, auth.KindSecondFactor, c.UserID, deps.Now())
		}
		switch {
		case !ok || errors.Is(err, auth.ErrPasskeyFailed) || errors.Is(err, auth.ErrNotFound):
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign-in timed out, start again.")
		case err != nil:
			writePasskeyError(w, r, err)
		default:
			writeJSON(w, http.StatusOK, opts)
		}
	})
}

// signInPasskey handles POST /api/session/passkey (public, but only with a bm_pending
// second_factor sign-in, else 401 unauthenticated): it finishes sign-in with the
// credential.toJSON() of an assertion (at most 16 KiB) for the options of
// POST /api/session/passkey/options, by the rules of finishSecondStep. A refused passkey is
// 401 passkey_failed; the options must then be fetched again.
func signInPasskey(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var assertion json.RawMessage
		if !decodeJSON(w, r, &assertion, maxWebAuthnBody) {
			return
		}
		finishSecondStep(w, r, deps, "passkey", stepFailure{"passkey_failed", "The passkey was not accepted.", "sign-in failed, passkey refused"}, false,
			func(id string, admin auth.Admin) (bool, error) {
				return checkPasskey(r, deps, id, auth.KindSecondFactor, admin, assertion)
			})
	})
}

// stepFailure is how a refused second step answers: 401 with code and message, logging event.
type stepFailure struct{ code, message, event string }

// finishSecondStep finishes the request's bm_pending second_factor sign-in with check, which
// reports whether the proof is right for the pending admin. Without such a pending, or with
// its admin gone, it answers 401 unauthenticated. The admin's rate limit is checked first
// (429). A refused proof answers fail, counts as a sign-in failure, and leaves the pending
// sign-in usable until it expires. A right one uses up the pending sign-in (check already did
// when checkTakes is set), clears its cookie, starts a session as POST /api/session does and
// answers 200 with the Admin. A check that finds the pending sign-in gone returns
// auth.ErrNotFound, answered 401 unauthenticated.
func finishSecondStep(w http.ResponseWriter, r *http.Request, deps Deps, method string, fail stepFailure, checkTakes bool, check func(pendingID string, admin auth.Admin) (bool, error)) {
	id, c, ok, err := pendingFrom(deps, r, auth.KindSecondFactor)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	var admin auth.Admin
	if ok {
		admin, err = auth.GetAdmin(deps.DB, c.UserID)
	}
	if errors.Is(err, auth.ErrNotFound) || !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign-in timed out, start again.")
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if retry, ok := deps.Limiter.Check(admin.Username); !ok {
		slog.WarnContext(r.Context(), "sign-in rate limited", "username", admin.Username)
		writeRateLimited(w, retry)
		return
	}
	failed := false
	defer func() { deps.Limiter.Done(admin.Username, failed) }()
	ok, err = check(id, admin)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign-in timed out, start again.")
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if !ok {
		failed = true
		slog.WarnContext(r.Context(), fail.event, "username", admin.Username)
		writeError(w, http.StatusUnauthorized, fail.code, fail.message)
		return
	}
	if !checkTakes {
		if _, ok, err := auth.TakeChallenge(deps.DB, id, auth.KindSecondFactor, deps.Now()); err != nil || !ok {
			// Cancelled, cleared or expired between the check and now.
			if err == nil {
				writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign-in timed out, start again.")
			} else {
				writeInternal(w, r, err)
			}
			return
		}
	}
	clearPendingCookie(w, deps)
	if !startSession(w, r, deps, admin.ID) {
		return
	}
	slog.InfoContext(r.Context(), "admin signed in", "username", admin.Username, "second_factor", method)
	writeJSON(w, http.StatusOK, admin)
}

// cancelSignIn handles DELETE /api/session/pending (public): it deletes the second_factor or
// enroll pending sign-in named by bm_pending, if any, clears the cookie and answers 204
// whether or not one existed (research R11). It never touches a session.
func cancelSignIn(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(pendingCookieName); err == nil {
			for _, kind := range []auth.ChallengeKind{auth.KindSecondFactor, auth.KindEnroll} {
				if _, _, err := auth.TakeChallenge(deps.DB, c.Value, kind, deps.Now()); err != nil {
					writeInternal(w, r, err)
					return
				}
			}
		}
		clearPendingCookie(w, deps)
		w.WriteHeader(http.StatusNoContent)
	})
}

// writeRateLimited answers 429 too_many_attempts with Retry-After, retry rounded up to whole
// seconds.
func writeRateLimited(w http.ResponseWriter, retry time.Duration) {
	secs := max(1, int((retry+time.Second-1)/time.Second))
	mins := (secs + 59) / 60
	unit := "minutes"
	if mins == 1 {
		unit = "minute"
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeError(w, http.StatusTooManyRequests, "too_many_attempts",
		fmt.Sprintf("Too many failed attempts. Try again in %d %s.", mins, unit))
}

// getSession handles GET /api/session (admin): 200 with the signed-in Admin.
func getSession(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusOK, admin)
	})
}

// signOut handles DELETE /api/session (admin): it deletes this session on the server, clears
// the cookie and returns 204. A copy of the old cookie is refused from then on (FR-015).
func signOut(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := principalFrom(r.Context())
		if err := auth.DeleteSession(deps.DB, p.SessionID); err != nil {
			writeInternal(w, r, err)
			return
		}
		clearSessionCookie(w, deps.CookieSecure)
		w.WriteHeader(http.StatusNoContent)
	})
}

type passwordChangeBody struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// changePassword handles POST /api/password (admin): it sets the signed-in admin's password
// from the PasswordChange body, ends their other sessions, keeps this one and returns 204
// (FR-012). Wrong current passwords give 403 wrong_password and count toward the sign-in
// rate limit for this username, which is checked first (429 too_many_attempts). A bad new
// password gives 400 invalid_request, saturated hashing 503 busy.
func changePassword(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body passwordChangeBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		p, _ := principalFrom(r.Context())
		admin, err := auth.GetAdmin(deps.DB, p.AdminID)
		if err == nil {
			if retry, ok := deps.Limiter.Check(admin.Username); !ok {
				slog.WarnContext(r.Context(), "sign-in rate limited", "username", admin.Username)
				writeRateLimited(w, retry)
				return
			}
			defer func() { deps.Limiter.Done(admin.Username, errors.Is(err, auth.ErrWrongPassword)) }()
			err = auth.ChangePassword(deps.DB, p.AdminID, body.CurrentPassword, body.NewPassword, p.SessionID)
		}
		switch {
		case errors.Is(err, auth.ErrNotFound):
			// Removed between the session lookup and now.
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
		case errors.Is(err, auth.ErrWrongPassword):
			slog.WarnContext(r.Context(), "password change failed", "username", admin.Username)
			writeError(w, http.StatusForbidden, "wrong_password", "The current password is wrong.")
		case err != nil:
			writeAuthError(w, r, err)
		default:
			slog.InfoContext(r.Context(), "password changed", "username", admin.Username)
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
