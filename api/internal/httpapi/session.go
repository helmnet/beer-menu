package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// signIn handles POST /api/session (public). It checks the per-username rate limit before
// any hashing (429 too_many_attempts with Retry-After in whole seconds), then the password.
// Unknown usernames and wrong passwords get the same 401 invalid_credentials and count as a
// failure for that username. Success always mints a new session (FR-017), sets its cookie and
// returns 200 with the Admin. Bad input gives 400, saturated hashing 503 busy.
func signIn(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body credentialsBody
		if !decodeJSON(w, r, &body) {
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
		if !startSession(w, r, deps, admin.ID) {
			return
		}
		slog.InfoContext(r.Context(), "admin signed in", "username", admin.Username)
		writeJSON(w, http.StatusOK, admin)
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
		if !decodeJSON(w, r, &body) {
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
