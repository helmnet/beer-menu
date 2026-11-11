package httpapi

import (
	"embed"
	"net/http"

	"github.com/helmnet/beer-menu/api"
)

// docsFiles holds the docs page, its start script and the vendored Swagger UI (feature 9).
// Files are named one by one, never the directory, so README.md and fetch.sh are not embedded.
//
//go:embed docs/index.html docs/init.js docs/swagger-ui/swagger-ui-bundle.js docs/swagger-ui/swagger-ui.css docs/swagger-ui/swagger-ui-bundle.js.LICENSE.txt docs/swagger-ui/LICENSE docs/swagger-ui/NOTICE
var docsFiles embed.FS

// docsCSP replaces the API's default CSP on the docs page and its assets only: Swagger UI needs
// same-origin scripts, styles, data: images (its CSS icons) and a fetch of /api/openapi.yaml.
const docsCSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// docsAsset is one file served under /api/docs/{file}: its path in docsFiles and its type.
type docsAsset struct{ path, contentType string }

// docsAssets is the fixed allowlist for /api/docs/{file}. The requested name is only ever a
// key here, never part of a path, so traversal ("../go.mod") or another file cannot be served.
// Types are explicit: the scratch image has no mime table and every response has nosniff.
var docsAssets = map[string]docsAsset{
	"init.js":                          {"docs/init.js", "text/javascript; charset=utf-8"},
	"swagger-ui-bundle.js":             {"docs/swagger-ui/swagger-ui-bundle.js", "text/javascript; charset=utf-8"},
	"swagger-ui.css":                   {"docs/swagger-ui/swagger-ui.css", "text/css; charset=utf-8"},
	"LICENSE":                          {"docs/swagger-ui/LICENSE", "text/plain; charset=utf-8"},
	"NOTICE":                           {"docs/swagger-ui/NOTICE", "text/plain; charset=utf-8"},
	"swagger-ui-bundle.js.LICENSE.txt": {"docs/swagger-ui/swagger-ui-bundle.js.LICENSE.txt", "text/plain; charset=utf-8"},
}

// writeDocsFile writes one embedded file with its type and the docs CSP.
func writeDocsFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	b, err := docsFiles.ReadFile(path)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Security-Policy", docsCSP)
	w.Write(b)
}

// getDocs serves the read-only Swagger UI page for this API. Public (constitution V).
func getDocs(Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeDocsFile(w, r, "docs/index.html", "text/html; charset=utf-8")
	})
}

// getDocsAsset serves one allowlisted file of the docs page by exact name; any other name is
// 404. Public (constitution V).
func getDocsAsset(Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := docsAssets[r.PathValue("file")]
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "Not found.")
			return
		}
		writeDocsFile(w, r, a.path, a.contentType)
	})
}

// getOpenAPI serves the embedded API contract, api/openapi.yaml, as application/yaml. Public
// (constitution V): it holds no secrets and is what the docs page renders.
func getOpenAPI(Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set explicitly: the scratch image has no mime table (research R4).
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(api.OpenAPI)
	})
}
