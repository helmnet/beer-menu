package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

type credentialsBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// getSetup handles GET /api/setup (public): 200 {"needed": true} while no admin exists, else
// {"needed": false}. It reveals nothing else (FR-005).
func getSetup(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := auth.AdminCount(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"needed": n == 0})
	})
}

// createFirstAdmin handles POST /api/setup (public): while no admin exists it creates one from
// the Credentials body, signs them in with a new session cookie and returns 201 with the Admin.
// It returns 409 setup_done once an admin exists, 400 invalid_request for bad input and
// 503 busy when password hashing is saturated.
func createFirstAdmin(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body credentialsBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		admin, err := auth.CreateFirstAdmin(deps.DB, body.Username, body.Password, deps.Now())
		if errors.Is(err, auth.ErrSetupDone) {
			writeError(w, http.StatusConflict, "setup_done", "Setup is already complete. Sign in instead.")
			return
		}
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "first admin created", "username", admin.Username)
		if !startSession(w, r, deps, admin.ID) {
			return
		}
		writeJSON(w, http.StatusCreated, admin)
	})
}

// writeAuthError answers an error from package auth: 400 invalid_request with the rule's
// message for a ValidationError, 503 busy with Retry-After for ErrBusy, else 500.
func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var ve auth.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, "invalid_request", ve.Error())
	case errors.Is(err, auth.ErrBusy):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, "busy", "Server busy. Try again shortly.")
	default:
		writeInternal(w, r, err)
	}
}

// startSession creates a new session for adminID and sets its cookie. On failure it answers
// 500 and returns false.
func startSession(w http.ResponseWriter, r *http.Request, deps Deps, adminID int64) bool {
	now := deps.Now()
	id, err := auth.CreateSession(deps.DB, adminID, now)
	if err != nil {
		writeInternal(w, r, err)
		return false
	}
	setSessionCookie(w, id, now.Add(auth.SessionMaxAge), deps.CookieSecure)
	return true
}
