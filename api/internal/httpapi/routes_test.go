package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/helmnet/beer-menu/api"
)

// contractProblems returns one line per way routes break the OpenAPI document spec: a route
// whose method and path are not an operation, whose Access differs from the operation's
// x-access, whose security is not [] exactly when it is public, or that is AccessRead without
// being a GET.
func contractProblems(t *testing.T, spec []byte, routes []Route) []string {
	t.Helper()
	type operation struct {
		Access   string            `yaml:"x-access"`
		Security *[]map[string]any `yaml:"security"`
	}
	var doc struct {
		Paths map[string]map[string]operation `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	var problems []string
	for _, r := range routes {
		name := r.Method + " " + r.Path
		if r.Access == AccessRead && r.Method != http.MethodGet {
			problems = append(problems, name+": AccessRead on a non-GET route")
		}
		op, ok := doc.Paths[r.Path][strings.ToLower(r.Method)]
		if !ok {
			problems = append(problems, name+": undocumented")
			continue
		}
		if op.Access != r.Access.String() {
			problems = append(problems, name+": x-access "+op.Access+", route "+r.Access.String())
		}
		emptySecurity := op.Security != nil && len(*op.Security) == 0
		if emptySecurity != (r.Access == AccessPublic) {
			problems = append(problems, name+": security [] must match public access")
		}
	}
	return problems
}

func TestRoutesDocumented(t *testing.T) {
	if len(Routes) == 0 {
		t.Fatal("Routes is empty")
	}
	for _, p := range contractProblems(t, api.OpenAPI, Routes) {
		t.Errorf("route %s (api/openapi.yaml)", p)
	}
}

// TestPublicRoutes pins the public routes to constitution principle V (health, setup status,
// first-admin creation, sign-in), so a new route can't go public by mistake. Feature 5 adds
// GET /api/menu, the only menu read, and must update want.
func TestPublicRoutes(t *testing.T) {
	var public []string
	for _, r := range Routes {
		if r.Access == AccessPublic {
			public = append(public, r.Method+" "+r.Path)
		}
	}
	want := "GET /api/health,GET /api/setup,POST /api/setup,POST /api/session"
	if got := strings.Join(public, ","); got != want {
		t.Fatalf("public routes = %s, want %s", got, want)
	}
}

func TestRoutesDocumentedCatchesMissing(t *testing.T) {
	extra := []Route{
		{Method: http.MethodPost, Path: "/api/health", Access: AccessPublic},
		{Method: http.MethodGet, Path: "/api/undocumented", Access: AccessPublic},
	}
	got := contractProblems(t, api.OpenAPI, append(extra, Routes...))
	want := "POST /api/health: undocumented,GET /api/undocumented: undocumented"
	if strings.Join(got, ",") != want {
		t.Fatalf("problems = %v, want %s", got, want)
	}
}

func TestRoutesDocumentedCatchesAccessMismatch(t *testing.T) {
	spec := []byte(`
paths:
  /api/a:
    get:
      x-access: read
  /api/b:
    get:
      security: []
  /api/c:
    post:
      x-access: read
  /api/d:
    get:
      x-access: admin
      security: []
  /api/e:
    get:
      x-access: public
`)
	got := contractProblems(t, spec, []Route{
		{Method: http.MethodGet, Path: "/api/a", Access: AccessWrite},
		{Method: http.MethodGet, Path: "/api/b", Access: AccessPublic},
		{Method: http.MethodPost, Path: "/api/c", Access: AccessRead},
		{Method: http.MethodGet, Path: "/api/d", Access: AccessAdmin},
		{Method: http.MethodGet, Path: "/api/e", Access: AccessPublic},
	})
	want := []string{
		"GET /api/a: x-access read, route write",
		"GET /api/b: x-access , route public",
		"POST /api/c: AccessRead on a non-GET route",
		"GET /api/d: security [] must match public access",
		"GET /api/e: security [] must match public access",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
