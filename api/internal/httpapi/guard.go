package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// Access is who may call a route (research R7). The zero value is AccessAdmin, so a route
// that declares nothing needs a session (FR-035).
type Access int

// Access levels, matching x-access in api/openapi.yaml.
const (
	// AccessAdmin allows only a signed-in admin session.
	AccessAdmin Access = iota
	// AccessRead allows a session, a read token or a write token. Only for GET routes.
	AccessRead
	// AccessWrite allows a session or a write token.
	AccessWrite
	// AccessPublic allows anyone, with or without credentials.
	AccessPublic
)

// String returns the x-access name of a: "admin", "read", "write" or "public".
func (a Access) String() string {
	switch a {
	case AccessAdmin:
		return "admin"
	case AccessRead:
		return "read"
	case AccessWrite:
		return "write"
	case AccessPublic:
		return "public"
	}
	return "unknown"
}

// principal is the credential a guarded request was allowed with: an admin session
// (AdminID and SessionID set) or an API token (Token set).
type principal struct {
	AdminID   int64
	SessionID string
	Token     *auth.Token
}

type principalKey struct{}

// principalFrom returns the principal guard stored in ctx; ok is false on public routes.
func principalFrom(ctx context.Context) (p principal, ok bool) {
	p, ok = ctx.Value(principalKey{}).(principal)
	return p, ok
}

// guard enforces access on h. Public routes pass untouched. Otherwise an Authorization header,
// if present, alone decides: it must be exactly one "Bearer bmt_..." for an active token, or
// the request gets 401 even with a valid session cookie. Without one, the bm_session cookie
// must name a valid session. No valid credential gives 401; a token on a write route without
// write scope, or on an admin route, gives 403. The principal goes into the request context.
func guard(deps Deps, access Access, h http.Handler) http.Handler {
	if access == AccessPublic {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := resolve(deps, r)
		switch {
		case err != nil:
			writeInternal(w, r, err)
			return
		case p == nil:
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sign in or send a valid API token.")
			return
		case p.Token != nil && (access == AccessAdmin || (access == AccessWrite && p.Token.Scope != auth.ScopeWrite)):
			writeError(w, http.StatusForbidden, "forbidden", "This credential cannot do that.")
			return
		}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, *p)))
	})
}

// resolve returns the request's valid credential, or nil when it has none or a bad one.
func resolve(deps Deps, r *http.Request) (*principal, error) {
	if authz := r.Header.Values("Authorization"); len(authz) > 0 {
		scheme, secret, found := strings.Cut(authz[0], " ")
		if len(authz) != 1 || !found || !strings.EqualFold(scheme, "Bearer") {
			return nil, nil
		}
		tok, ok, err := auth.LookupToken(deps.DB, secret, deps.Now())
		if err != nil || !ok {
			return nil, err
		}
		return &principal{Token: &tok}, nil
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, nil
	}
	uid, ok, err := auth.LookupSession(deps.DB, c.Value, deps.Now())
	if err != nil || !ok {
		return nil, err
	}
	return &principal{AdminID: uid, SessionID: c.Value}, nil
}
