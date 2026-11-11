package httpapi

import (
	"bytes"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

const apiCSP = "default-src 'none'; frame-ancestors 'none'"

// TestOpenAPI checks GET /api/openapi.yaml is public and serves exactly api/openapi.yaml from
// disk, so a stale or altered embed is caught (FR-002, SC-004).
func TestOpenAPI(t *testing.T) {
	want, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	for name, hdrs := range map[string][]string{
		"none":    nil,
		"session": c.session,
		"read":    c.read,
		"invalid": {"Authorization", "Bearer bmt_invalid"},
	} {
		rec := do(mux, http.MethodGet, "/api/openapi.yaml", "", hdrs...)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/yaml" || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Fatalf("%s: GET /api/openapi.yaml = %d %q, body equal %v", name, rec.Code, rec.Header().Get("Content-Type"), bytes.Equal(rec.Body.Bytes(), want))
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") != apiCSP {
			t.Fatalf("%s: headers = %v", name, rec.Header())
		}
	}
	if rec := do(mux, http.MethodHead, "/api/openapi.yaml", ""); rec.Code != 200 {
		t.Fatalf("HEAD = %d", rec.Code)
	}
	if rec := do(mux, http.MethodPost, "/api/openapi.yaml", ""); rec.Code != 405 {
		t.Fatalf("POST = %d", rec.Code)
	}
}

// wantDocsCSP is docsCSP spelled out, so a change to the policy must also change this test.
const wantDocsCSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// TestDocsPage checks the docs page is public, same-origin only (no inline script, no absolute
// URL) and is the only route sent the docs CSP besides its assets.
func TestDocsPage(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	rec := do(mux, http.MethodGet, "/api/docs", "")
	h := rec.Header()
	if rec.Code != 200 || h.Get("Content-Type") != "text/html; charset=utf-8" || h.Get("Content-Security-Policy") != wantDocsCSP ||
		h.Get("X-Frame-Options") != "DENY" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("GET /api/docs = %d %v", rec.Code, h)
	}
	page := rec.Body.String()
	for _, ref := range []string{`"/api/docs/swagger-ui.css"`, `"/api/docs/swagger-ui-bundle.js"`, `"/api/docs/init.js"`} {
		if !strings.Contains(page, ref) {
			t.Errorf("page lacks %s", ref)
		}
	}
	for _, tag := range regexp.MustCompile(`<script(\s[^>]*)?>`).FindAllString(page, -1) {
		if !strings.Contains(tag, " src=") {
			t.Errorf("inline script tag %s", tag)
		}
	}
	if strings.Contains(page, "http://") || strings.Contains(page, "https://") {
		t.Errorf("page has an absolute URL:\n%s", page)
	}

	for name, hdrs := range map[string][]string{
		"session": c.session,
		"read":    c.read,
		"invalid": {"Authorization", "Bearer bmt_invalid"},
	} {
		r := do(mux, http.MethodGet, "/api/docs", "", hdrs...)
		if r.Code != 200 || r.Body.String() != page || r.Header().Get("Content-Security-Policy") != wantDocsCSP {
			t.Errorf("%s: GET /api/docs = %d", name, r.Code)
		}
	}
	if r := do(mux, http.MethodHead, "/api/docs", ""); r.Code != 200 {
		t.Errorf("HEAD /api/docs = %d", r.Code)
	}
	for _, p := range []string{"/api/docs", "/api/docs/init.js"} {
		if r := do(mux, http.MethodPost, p, ""); r.Code != 405 {
			t.Errorf("POST %s = %d", p, r.Code)
		}
	}
	// The docs CSP must not leak to other routes.
	for _, p := range []string{"/api/health", "/api/menu"} {
		if got := do(mux, http.MethodGet, p, "").Header().Get("Content-Security-Policy"); got != apiCSP {
			t.Errorf("%s CSP = %q", p, got)
		}
	}
}

// TestDocsAssets checks each allowlisted file is served byte-identical to disk with its type and
// the docs CSP, and that any other name, including traversal, is 404.
func TestDocsAssets(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)

	for name, tc := range map[string]struct{ disk, ctype string }{
		"init.js":                          {"docs/init.js", "text/javascript; charset=utf-8"},
		"swagger-ui-bundle.js":             {"docs/swagger-ui/swagger-ui-bundle.js", "text/javascript; charset=utf-8"},
		"swagger-ui.css":                   {"docs/swagger-ui/swagger-ui.css", "text/css; charset=utf-8"},
		"LICENSE":                          {"docs/swagger-ui/LICENSE", "text/plain; charset=utf-8"},
		"NOTICE":                           {"docs/swagger-ui/NOTICE", "text/plain; charset=utf-8"},
		"swagger-ui-bundle.js.LICENSE.txt": {"docs/swagger-ui/swagger-ui-bundle.js.LICENSE.txt", "text/plain; charset=utf-8"},
	} {
		want, err := os.ReadFile(tc.disk)
		if err != nil {
			t.Fatal(err)
		}
		rec := do(mux, http.MethodGet, "/api/docs/"+name, "")
		if rec.Code != 200 || rec.Header().Get("Content-Type") != tc.ctype || rec.Header().Get("Content-Security-Policy") != wantDocsCSP ||
			rec.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(rec.Body.Bytes(), want) {
			t.Errorf("GET /api/docs/%s = %d %v", name, rec.Code, rec.Header())
		}
	}

	// Only the allowlist is served: not other embedded or disk files, not traversal.
	for _, p := range []string{"/api/docs/x.js", "/api/docs/index.html", "/api/docs/README.md", "/api/docs/fetch.sh", "/api/docs/..%2fgo.mod", "/api/docs/%2e%2e"} {
		wantError(t, do(mux, http.MethodGet, p, ""), 404, "not_found")
	}
	if rec := do(mux, http.MethodGet, "/api/docs/", ""); rec.Code != 404 {
		t.Errorf("GET /api/docs/ = %d", rec.Code)
	}
}

// TestDocsInitReadOnly guards the read-only Swagger UI config against edits (FR-004, FR-005).
func TestDocsInitReadOnly(t *testing.T) {
	deps, _ := newTestDeps(t)
	js := do(NewMux(deps), http.MethodGet, "/api/docs/init.js", "").Body.String()
	for _, want := range []string{"supportedSubmitMethods: []", `url: "/api/openapi.yaml"`, "validatorUrl: null", "queryConfigEnabled: false", "showExtensions: true", "authorizeBtn:", "authorizeOperationBtn:"} {
		if !strings.Contains(js, want) {
			t.Errorf("init.js lacks %s", want)
		}
	}
}
