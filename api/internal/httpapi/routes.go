// Package httpapi holds the REST API's HTTP handlers and the single route table.
package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// Deps holds what handlers need: the open database, the running app version, whether the
// session cookie is marked Secure, the clock and the sign-in limiter.
type Deps struct {
	DB      *sql.DB
	Version string
	// CookieSecure sets Secure on the session cookie. Off only by BEER_MENU_COOKIE_SECURE=false.
	CookieSecure bool
	// Now is the clock for sessions and tokens; tests move it. NewMux uses time.Now if nil.
	Now func() time.Time
	// Limiter counts failed sign-ins. NewMux makes one on Now if nil.
	Limiter *auth.Limiter
}

// Route is one API endpoint. Every Route must be documented in api/openapi.yaml with an
// x-access equal to its Access.
type Route struct {
	// Method is the HTTP method, such as http.MethodGet.
	Method string
	// Path is the ServeMux path pattern. It must equal the path key in api/openapi.yaml.
	Path string
	// Access is who may call the route. The zero value, AccessAdmin, needs a session.
	Access Access
	// Handler builds the endpoint's handler from the shared dependencies.
	Handler func(Deps) http.Handler
}

// Routes is the single source of the API's routes. NewMux registers exactly these.
var Routes = []Route{
	{Method: http.MethodGet, Path: "/api/health", Access: AccessPublic, Handler: health},
	{Method: http.MethodGet, Path: "/api/setup", Access: AccessPublic, Handler: getSetup},
	{Method: http.MethodPost, Path: "/api/setup", Access: AccessPublic, Handler: createFirstAdmin},
	{Method: http.MethodPost, Path: "/api/session", Access: AccessPublic, Handler: signIn},
	{Method: http.MethodGet, Path: "/api/session", Access: AccessAdmin, Handler: getSession},
	{Method: http.MethodDelete, Path: "/api/session", Access: AccessAdmin, Handler: signOut},
	{Method: http.MethodPost, Path: "/api/password", Access: AccessAdmin, Handler: changePassword},
	{Method: http.MethodGet, Path: "/api/admins", Access: AccessAdmin, Handler: listAdmins},
	{Method: http.MethodPost, Path: "/api/admins", Access: AccessAdmin, Handler: addAdmin},
	{Method: http.MethodDelete, Path: "/api/admins/{id}", Access: AccessAdmin, Handler: removeAdmin},
	{Method: http.MethodGet, Path: "/api/tokens", Access: AccessAdmin, Handler: listTokens},
	{Method: http.MethodPost, Path: "/api/tokens", Access: AccessAdmin, Handler: createToken},
	{Method: http.MethodDelete, Path: "/api/tokens/{id}", Access: AccessAdmin, Handler: revokeToken},
}

// NewMux returns the API handler: every entry of Routes registered as "<METHOD> <path>"
// behind guard for its Access, all wrapped by withSecurity (security headers and refusal of
// cross-site writes).
func NewMux(deps Deps) http.Handler {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Limiter == nil {
		deps.Limiter = auth.NewLimiter(deps.Now)
	}
	mux := http.NewServeMux()
	for _, r := range Routes {
		mux.Handle(r.Method+" "+r.Path, guard(deps, r.Access, r.Handler(deps)))
	}
	return withSecurity(mux)
}
