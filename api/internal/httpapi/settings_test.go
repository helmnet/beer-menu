package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/store"
)

// defaultTV is the JSON tail of every feature 10 setting at its default, closing the object.
const defaultTV = `"layout":"grid","title_align":"left","title_font":"system","text_font":"system",` +
	`"show_style":true,"show_abv":true,"show_ibu":true,"show_srm":false,"show_description":false,` +
	`"title_color":"","text_color":""}`

const defaultSettings = `{"title":"","theme":"dark","units":"us","page_seconds":15,` + defaultTV

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
	want := `{"title":"Helm Taproom","theme":"dark","units":"us","page_seconds":20,` + defaultTV
	if rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("PATCH with session = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPatch, "/api/settings", `{"theme":"light","units":"metric"}`, c.write...)
	want = `{"title":"Helm Taproom","theme":"light","units":"metric","page_seconds":20,` + defaultTV
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
		`{"show_ibu":"true"}`:                    "show_ibu",
		`{"show_ibu":1}`:                         "show_ibu",
		`{"show_style":null}`:                    "show_style",
		`{"show_abv":null}`:                      "show_abv",
		`{"show_srm":null}`:                      "show_srm",
		`{"show_description":0}`:                 "show_description",
		`{"layout":null}`:                        "layout",
		`{"layout":3}`:                           "layout",
		`{"layout":"tiles"}`:                     "layout",
		`{"title_align":null}`:                   "title_align",
		`{"title_font":null}`:                    "title_font",
		`{"text_font":["inter"]}`:                "text_font",
		`{"title_color":null}`:                   "title_color",
		`{"text_color":null}`:                    "text_color",
		`{"title_color":"SecretTitle"}`:          "title_color",
		`{"layout":"list","text_color":"red"}`:   "text_color",
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

func TestUpdateTVSettings(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	body := `{"layout":"columns","title_align":"center","title_font":"pacifico","text_font":"oswald",` +
		`"show_style":false,"show_abv":false,"show_ibu":false,"show_srm":true,"show_description":true,` +
		`"title_color":"#FFAA00","text_color":"#a1B2c3"}`
	want := `{"title":"","theme":"dark","units":"us","page_seconds":15,` +
		`"layout":"columns","title_align":"center","title_font":"pacifico","text_font":"oswald",` +
		`"show_style":false,"show_abv":false,"show_ibu":false,"show_srm":true,"show_description":true,` +
		`"title_color":"#ffaa00","text_color":"#a1b2c3"}`
	if rec := do(mux, http.MethodPatch, "/api/settings", body, c.write...); rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("PATCH = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodGet, "/api/settings", "", c.read...); rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("GET after PATCH = %d %s", rec.Code, rec.Body)
	}
	rec := do(mux, http.MethodPatch, "/api/settings", `{"title_color":""}`, c.session...)
	if rec.Code != 200 || decodeObject(t, rec.Body.String())["title_color"] != "" {
		t.Fatalf("reset title_color = %d %s", rec.Code, rec.Body)
	}
}

// TestUpdateTVSettingsAuth checks a new field gets the existing write rules: no credential is
// 401 and a read token is 403, with nothing stored.
func TestUpdateTVSettingsAuth(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	wantError(t, do(mux, http.MethodPatch, "/api/settings", `{"show_ibu":false}`), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPatch, "/api/settings", `{"show_ibu":false}`, c.read...), 403, "forbidden")
	var n int
	if err := deps.DB.QueryRow("SELECT COUNT(*) FROM settings WHERE key LIKE 'display.%'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("refused PATCH stored %d rows (err %v)", n, err)
	}
}

// TestTVSettingsOnVersion5Data is the upgrade case: a version-5 database with feature 7 rows and
// no feature 10 rows reads every new setting at its default and keeps the feature 7 values.
func TestTVSettingsOnVersion5Data(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	if _, err := deps.DB.Exec(`INSERT INTO settings (key, value, updated_at) VALUES
		('display.title', 'Helm', 1), ('display.theme', 'light', 1), ('display.units', 'metric', 1),
		('display.page_seconds', '20', 1)`); err != nil {
		t.Fatal(err)
	}
	// Feature 10 added no migration; later features may, so only a lower version is wrong.
	if v, err := store.Version(deps.DB); err != nil || v < 5 {
		t.Fatalf("version = %d (err %v), want at least 5", v, err)
	}
	want := `{"title":"Helm","theme":"light","units":"metric","page_seconds":20,` + defaultTV
	if rec := do(mux, http.MethodGet, "/api/settings", "", c.read...); rec.Code != 200 || rec.Body.String() != want {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
}
