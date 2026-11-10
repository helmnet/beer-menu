package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

const defaultSettings = `{"title":"","theme":"dark","units":"us","page_seconds":15}`

func TestGetSettings(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for who, h := range map[string][]string{"session": c.session, "read token": c.read, "write token": c.write} {
		rec := do(mux, http.MethodGet, "/api/settings", "", h...)
		if rec.Code != 200 || rec.Body.String() != defaultSettings {
			t.Errorf("GET /api/settings with %s = %d %s", who, rec.Code, rec.Body)
		}
	}
}

func TestUpdateSettings(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	rec := do(mux, http.MethodPatch, "/api/settings", `{"title":"  Helm Taproom ","page_seconds":20}`, c.session...)
	want := `{"title":"Helm Taproom","theme":"dark","units":"us","page_seconds":20}`
	if rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("PATCH with session = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPatch, "/api/settings", `{"theme":"light","units":"metric"}`, c.write...)
	want = `{"title":"Helm Taproom","theme":"light","units":"metric","page_seconds":20}`
	if rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("PATCH with write token = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodGet, "/api/settings", "", c.read...); rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("GET after PATCH = %d %s", rec.Code, rec.Body)
	}
}

func TestUpdateSettingsRefuses(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	secret := "SecretTitle" + strings.Repeat("x", 50)
	for body, field := range map[string]string{
		`{"page_seconds":4}`:                     "page_seconds",
		`{"page_seconds":301}`:                   "page_seconds",
		`{"page_seconds":15.5}`:                  "page_seconds",
		`{"page_seconds":"20"}`:                  "page_seconds",
		`{"page_seconds":null}`:                  "page_seconds",
		`{"title":"` + secret + `"}`:             "title",
		`{"title":"` + secret + `\u0007"}`:       "title",
		`{"title":null}`:                         "title",
		`{"title":7}`:                            "title",
		`{"theme":"blue"}`:                       "theme",
		`{"theme":null}`:                         "theme",
		`{"units":"imperial"}`:                   "units",
		`{"units":null}`:                         "units",
		`{"title":"Fine","page_seconds":4}`:      "page_seconds",
		`{"title":"Fine","page_seconds":"slow"}`: "page_seconds",
	} {
		rec := do(mux, http.MethodPatch, "/api/settings", body, c.write...)
		wantError(t, rec, 400, "invalid_request")
		fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any)
		if len(fields) != 1 || fields[field] == nil {
			t.Errorf("PATCH %s: fields = %s, want only %s", body, rec.Body, field)
		}
		if strings.Contains(rec.Body.String(), "SecretTitle") {
			t.Errorf("PATCH %s: response echoes the title: %s", body, rec.Body)
		}
	}
	for _, body := range []string{
		`{}`,
		`{"logo":"x"}`,
		`{"title":"Helm"} {}`,
		`{"title":"` + strings.Repeat("x", 9000) + `"}`,
	} {
		rec := do(mux, http.MethodPatch, "/api/settings", body, c.write...)
		wantError(t, rec, 400, "invalid_request")
	}
	if rec := do(mux, http.MethodGet, "/api/settings", "", c.read...); rec.Body.String() != defaultSettings {
		t.Fatalf("a refused PATCH changed settings: %s", rec.Body)
	}
}
