package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// listAdmins handles GET /api/admins (admin): 200 with every admin, oldest first, with no
// password data (FR-010).
func listAdmins(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admins, err := auth.ListAdmins(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, admins)
	})
}

// addAdmin handles POST /api/admins (admin): it creates an admin from the Credentials body and
// returns 201 with the Admin. The setupCaller checks run first (403 confirm_required without a
// recent confirm), so an unconfirmed caller cannot probe usernames. A username taken ignoring
// case gives 409 username_taken, bad input 400 invalid_request, saturated hashing 503 busy.
func addAdmin(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := setupCaller(w, r, deps); !ok {
			return
		}
		var body credentialsBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		admin, err := auth.AddAdmin(deps.DB, body.Username, body.Password, deps.Now())
		if errors.Is(err, auth.ErrUsernameTaken) {
			writeError(w, http.StatusConflict, "username_taken", "That username is already in use.")
			return
		}
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "admin added", "username", admin.Username)
		writeJSON(w, http.StatusCreated, admin)
	})
}

// removeAdmin handles DELETE /api/admins/{id} (admin): it removes the admin and ends all their
// sessions at once (FR-011), then returns 204. Removing yourself also clears your cookie. The
// setupCaller checks run before any lookup (403 confirm_required without a recent confirm). The
// last admin gives 409 last_admin; an unknown or non-positive id gives 404 not_found.
func removeAdmin(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if _, _, ok := setupCaller(w, r, deps); !ok {
			return
		}
		username, err := auth.RemoveAdmin(deps.DB, id)
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeNotFound(w)
			return
		case errors.Is(err, auth.ErrLastAdmin):
			writeError(w, http.StatusConflict, "last_admin", "The last admin cannot be removed.")
			return
		case err != nil:
			writeInternal(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "admin removed", "username", username)
		if p, _ := principalFrom(r.Context()); p.AdminID == id {
			clearSessionCookie(w, deps.CookieSecure)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// clearAdminTwoFactor handles DELETE /api/admins/{id}/2fa (admin): it deletes all of another
// admin's factors, recovery codes, pending steps and sessions (FR-027), logs both usernames and
// answers 204. It works while 2FA is required; that admin then enrolls at next sign-in. The
// caller's own id is 409 cannot_clear_self; then the setupCaller checks apply (403
// confirm_required without a recent confirm); an unknown or non-positive id is 404 not_found.
func clearAdminTwoFactor(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		if p, _ := principalFrom(r.Context()); id == p.AdminID {
			writeError(w, http.StatusConflict, "cannot_clear_self", "You cannot clear your own two-factor sign-in here.")
			return
		}
		_, caller, ok := setupCaller(w, r, deps)
		if !ok {
			return
		}
		target, err := auth.GetAdmin(deps.DB, id)
		if err == nil {
			err = auth.ClearTwoFactor(deps.DB, id)
		}
		switch {
		case errors.Is(err, auth.ErrNotFound):
			writeNotFound(w)
		case err != nil:
			writeInternal(w, r, err)
		default:
			slog.InfoContext(r.Context(), "admin 2fa cleared", "username", target.Username, "by", caller.Username)
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
