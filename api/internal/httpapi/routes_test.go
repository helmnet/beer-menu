package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/helmnet/beer-menu/api"
)

// undocumented returns "<METHOD> <path>" for every route whose method and path are not an
// operation in the OpenAPI document spec.
func undocumented(t *testing.T, spec []byte, routes []Route) []string {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(spec, &doc); err != nil {
		t.Fatalf("parse OpenAPI document: %v", err)
	}
	var missing []string
	for _, r := range routes {
		if _, ok := doc.Paths[r.Path][strings.ToLower(r.Method)]; !ok {
			missing = append(missing, r.Method+" "+r.Path)
		}
	}
	return missing
}

func TestRoutesDocumented(t *testing.T) {
	if len(Routes) == 0 {
		t.Fatal("Routes is empty")
	}
	for _, r := range undocumented(t, api.OpenAPI, Routes) {
		t.Errorf("route %s is not documented in api/openapi.yaml", r)
	}
}

func TestRoutesDocumentedCatchesMissing(t *testing.T) {
	extra := []Route{
		{Method: http.MethodPost, Path: "/api/health"},
		{Method: http.MethodGet, Path: "/api/undocumented"},
	}
	got := undocumented(t, api.OpenAPI, append(extra, Routes...))
	want := "POST /api/health,GET /api/undocumented"
	if strings.Join(got, ",") != want {
		t.Fatalf("undocumented = %v, want %s", got, want)
	}
}
