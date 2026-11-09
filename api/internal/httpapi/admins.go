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
// returns 201 with the Admin. A username taken ignoring case gives 409 username_taken, bad
// input 400 invalid_request, saturated hashing 503 busy.
func addAdmin(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
// last admin gives 409 last_admin; an unknown or non-positive id gives 404 not_found.
func removeAdmin(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
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
