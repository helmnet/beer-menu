// Package httpapi holds the REST API's HTTP handlers and the single route table.
package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/importer"
)

// Deps holds what handlers need: the open database, the running app version, whether the
// session cookie is marked Secure, the clock, the sign-in limiter, the import service and the
// context of background work.
type Deps struct {
	DB      *sql.DB
	Version string
	// CookieSecure sets Secure on the session cookie. Off only by BEER_MENU_COOKIE_SECURE=false.
	CookieSecure bool
	// Now is the clock for sessions and tokens; tests move it. NewMux uses time.Now if nil.
	Now func() time.Time
	// Limiter counts failed sign-ins. NewMux makes one on Now if nil.
	Limiter *auth.Limiter
	// Importer connects, lists, imports and syncs Brewfather batches.
	Importer *importer.Service
	// Background is the context of work a request starts but does not wait for ("Sync now"); it
	// ends at shutdown. NewMux uses context.Background() if nil.
	Background context.Context
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
	{Method: http.MethodGet, Path: "/api/menu", Access: AccessPublic, Handler: getMenu},
	{Method: http.MethodGet, Path: "/api/docs", Access: AccessPublic, Handler: getDocs},
	{Method: http.MethodGet, Path: "/api/docs/{file}", Access: AccessPublic, Handler: getDocsAsset},
	{Method: http.MethodGet, Path: "/api/openapi.yaml", Access: AccessPublic, Handler: getOpenAPI},
	{Method: http.MethodGet, Path: "/api/session", Access: AccessAdmin, Handler: getSession},
	{Method: http.MethodDelete, Path: "/api/session", Access: AccessAdmin, Handler: signOut},
	{Method: http.MethodPost, Path: "/api/password", Access: AccessAdmin, Handler: changePassword},
	{Method: http.MethodGet, Path: "/api/admins", Access: AccessAdmin, Handler: listAdmins},
	{Method: http.MethodPost, Path: "/api/admins", Access: AccessAdmin, Handler: addAdmin},
	{Method: http.MethodDelete, Path: "/api/admins/{id}", Access: AccessAdmin, Handler: removeAdmin},
	{Method: http.MethodGet, Path: "/api/tokens", Access: AccessAdmin, Handler: listTokens},
	{Method: http.MethodPost, Path: "/api/tokens", Access: AccessAdmin, Handler: createToken},
	{Method: http.MethodDelete, Path: "/api/tokens/{id}", Access: AccessAdmin, Handler: revokeToken},
	{Method: http.MethodGet, Path: "/api/beers", Access: AccessRead, Handler: listBeers},
	{Method: http.MethodPost, Path: "/api/beers", Access: AccessWrite, Handler: createBeer},
	{Method: http.MethodGet, Path: "/api/beers/{id}", Access: AccessRead, Handler: getBeer},
	{Method: http.MethodPatch, Path: "/api/beers/{id}", Access: AccessWrite, Handler: updateBeer},
	{Method: http.MethodDelete, Path: "/api/beers/{id}", Access: AccessWrite, Handler: deleteBeer},
	{Method: http.MethodGet, Path: "/api/srm-colors", Access: AccessRead, Handler: getSrmColors},
	{Method: http.MethodGet, Path: "/api/taps", Access: AccessRead, Handler: listTaps},
	{Method: http.MethodPost, Path: "/api/taps", Access: AccessWrite, Handler: createTap},
	{Method: http.MethodPut, Path: "/api/taps/order", Access: AccessWrite, Handler: reorderTaps},
	{Method: http.MethodGet, Path: "/api/taps/{id}", Access: AccessRead, Handler: getTap},
	{Method: http.MethodPatch, Path: "/api/taps/{id}", Access: AccessWrite, Handler: updateTap},
	{Method: http.MethodDelete, Path: "/api/taps/{id}", Access: AccessWrite, Handler: deleteTap},
	{Method: http.MethodGet, Path: "/api/kegs", Access: AccessRead, Handler: listKegs},
	{Method: http.MethodPost, Path: "/api/kegs", Access: AccessWrite, Handler: createKeg},
	{Method: http.MethodGet, Path: "/api/kegs/{id}", Access: AccessRead, Handler: getKeg},
	{Method: http.MethodPatch, Path: "/api/kegs/{id}", Access: AccessWrite, Handler: updateKeg},
	{Method: http.MethodDelete, Path: "/api/kegs/{id}", Access: AccessWrite, Handler: deleteKeg},
	{Method: http.MethodPost, Path: "/api/kegs/{id}/pours", Access: AccessWrite, Handler: pourKeg},
	{Method: http.MethodGet, Path: "/api/keg-types", Access: AccessRead, Handler: listKegTypes},
	{Method: http.MethodGet, Path: "/api/settings", Access: AccessRead, Handler: getSettings},
	{Method: http.MethodPatch, Path: "/api/settings", Access: AccessWrite, Handler: updateSettings},
	{Method: http.MethodGet, Path: "/api/brewfather", Access: AccessRead, Handler: getBrewfather},
	{Method: http.MethodPut, Path: "/api/brewfather/connection", Access: AccessAdmin, Handler: connectBrewfather},
	{Method: http.MethodDelete, Path: "/api/brewfather/connection", Access: AccessAdmin, Handler: disconnectBrewfather},
	{Method: http.MethodPut, Path: "/api/brewfather/schedule", Access: AccessAdmin, Handler: setBrewfatherSchedule},
	// Write, not read: a fresh list spends the owner's Brewfather quota (FR-030).
	{Method: http.MethodGet, Path: "/api/brewfather/batches", Access: AccessWrite, Handler: listBrewfatherBatches},
	{Method: http.MethodPost, Path: "/api/brewfather/imports", Access: AccessWrite, Handler: importBrewfatherBatches},
	{Method: http.MethodPost, Path: "/api/brewfather/sync", Access: AccessWrite, Handler: syncBrewfather},
	{Method: http.MethodPost, Path: "/api/beers/{id}/sync", Access: AccessWrite, Handler: resyncBeer},
}

// NewMux returns the API handler: every entry of Routes registered as "<METHOD> <path>"
// behind guard for its Access, all wrapped by withSecurity (security headers and refusal of
// cross-site writes).
func NewMux(deps Deps) http.Handler {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Background == nil {
		deps.Background = context.Background()
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
