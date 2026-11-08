// Package httpapi holds the REST API's HTTP handlers and the single route table.
package httpapi

import (
	"database/sql"
	"net/http"
)

// Deps holds what handlers need: the open database and the running app version.
type Deps struct {
	DB      *sql.DB
	Version string
}

// Route is one API endpoint. Every Route must be documented in api/openapi.yaml.
type Route struct {
	// Method is the HTTP method, such as http.MethodGet.
	Method string
	// Path is the ServeMux path pattern. It must equal the path key in api/openapi.yaml.
	Path string
	// Handler builds the endpoint's handler from the shared dependencies.
	Handler func(Deps) http.Handler
}

// Routes is the single source of the API's routes. NewMux registers exactly these.
var Routes = []Route{
	{Method: http.MethodGet, Path: "/api/health", Handler: health},
}

// NewMux returns a mux with every entry of Routes registered as "<METHOD> <path>".
func NewMux(deps Deps) *http.ServeMux {
	mux := http.NewServeMux()
	for _, r := range Routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler(deps))
	}
	return mux
}
