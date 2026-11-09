package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

type tokenBody struct {
	Name  string     `json:"name"`
	Scope auth.Scope `json:"scope"`
}

// listTokens handles GET /api/tokens (admin): 200 with every token, newest first, revoked
// ones included. Bodies never hold a secret or hash (FR-026).
func listTokens(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens, err := auth.ListTokens(deps.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, tokens)
	})
}

// createToken handles POST /api/tokens (admin): it creates a token from the TokenRequest body
// and returns 201 with the NewToken, the only response that ever holds the secret (FR-024).
// A bad name or scope gives 400 invalid_request.
func createToken(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body tokenBody
		if !decodeJSON(w, r, &body, maxBody) {
			return
		}
		tok, secret, err := auth.CreateToken(deps.DB, body.Name, body.Scope, deps.Now())
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "token created", "token_name", tok.Name, "scope", tok.Scope)
		writeJSON(w, http.StatusCreated, struct {
			auth.Token
			Secret string `json:"secret"`
		}{tok, secret})
	})
}

// revokeToken handles DELETE /api/tokens/{id} (admin): 204 once the token is revoked, which
// takes effect on its next request; revoking a revoked token also gives 204. An unknown or
// non-positive id gives 404 not_found.
func revokeToken(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		name, err := auth.RevokeToken(deps.DB, id, deps.Now())
		if errors.Is(err, auth.ErrNotFound) {
			writeNotFound(w)
			return
		}
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		slog.InfoContext(r.Context(), "token revoked", "token_name", name)
		w.WriteHeader(http.StatusNoContent)
	})
}

// pathID returns the positive integer {id} path value. Otherwise it answers 404 not_found
// and returns false.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeNotFound(w)
		return 0, false
	}
	return id, true
}

func writeNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not_found", "Not found.")
}
