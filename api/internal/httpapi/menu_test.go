package httpapi

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// objectKeys returns the sorted keys of the JSON object v, or "not an object".
func objectKeys(v any) string {
	o, ok := v.(map[string]any)
	if !ok {
		return "not an object"
	}
	var ks []string
	for k := range o {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return strings.Join(ks, ",")
}

func TestMenu(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	beer := insertBeer(t, deps, "Pale")
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 40, 10, 1, 1, 1)", beer); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, type, size_l, remaining_l, created_at, updated_at) VALUES (?, 'sanke_half', 58.67, 58.67, 1, 1)", beer); err != nil {
		t.Fatal(err)
	}

	rec := do(mux, http.MethodGet, "/api/menu", "")
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("GET /api/menu = %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	body := rec.Body.String()
	want := `{"taps":[{"label":"Tap 1","beer":{"name":"Pale","style":null,"abv":null,"ibu":null,"color":null},"keg":{"shape":"corny","level":0.25}}],` +
		`"on_deck":[{"beer":{"name":"Pale","style":null,"abv":null,"ibu":null,"color":null},"keg":{"shape":"sanke","level":1},"status":null}],"display":{"title":"","theme":"dark","page_seconds":15}}`
	if body != want {
		t.Fatalf("body =\n%s\nwant\n%s", body, want)
	}
	m := decodeObject(t, body)
	if got := objectKeys(m); got != "display,on_deck,taps" {
		t.Fatalf("menu keys = %s", got)
	}
	tap := m["taps"].([]any)[0]
	item := m["on_deck"].([]any)[0]
	for what, pair := range map[string][2]any{
		"tap":       {tap, "beer,keg,label"},
		"tap beer":  {tap.(map[string]any)["beer"], "abv,color,ibu,name,style"},
		"tap keg":   {tap.(map[string]any)["keg"], "level,shape"},
		"item":      {item, "beer,keg,status"},
		"item beer": {item.(map[string]any)["beer"], "abv,color,ibu,name,style"},
		"item keg":  {item.(map[string]any)["keg"], "level,shape"},
		"display":   {m["display"], "page_seconds,theme,title"},
	} {
		if got := objectKeys(pair[0]); got != pair[1] {
			t.Errorf("%s keys = %s, want %s", what, got, pair[1])
		}
	}

	for who, h := range map[string][]string{"session": c.session, "read token": c.read, "write token": c.write} {
		if rec := do(mux, http.MethodGet, "/api/menu", "", h...); rec.Code != 200 || rec.Body.String() != body {
			t.Errorf("GET /api/menu with %s = %d %s", who, rec.Code, rec.Body)
		}
	}

	before := dataTables(t, deps)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, h := range [][]string{nil, c.write, c.session} {
			if rec := do(mux, method, "/api/menu", `{"taps":[]}`, h...); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /api/menu = %d, want 405", method, rec.Code)
			}
		}
	}
	if after := dataTables(t, deps); after != before {
		t.Fatalf("a refused menu write changed data:\n%s\n%s", before, after)
	}

	for _, p := range []string{"/api/beers", "/api/taps", "/api/kegs", "/api/keg-types", "/api/srm-colors"} {
		wantError(t, do(mux, http.MethodGet, p, ""), 401, "unauthenticated")
	}
}

// TestMenuDisplay checks the public menu's display: the patched title, theme and page interval,
// never units or any other setting, with taps and on_deck unchanged.
func TestMenuDisplay(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	before := decodeObject(t, do(mux, http.MethodGet, "/api/menu", "").Body.String())
	if rec := do(mux, http.MethodPatch, "/api/settings", `{"title":"Helm Taproom","theme":"light","units":"metric","page_seconds":20}`, c.write...); rec.Code != 200 {
		t.Fatalf("PATCH /api/settings = %d %s", rec.Code, rec.Body)
	}
	if _, err := deps.DB.Exec("INSERT INTO settings (key, value) VALUES ('brewfather.sync_interval', '1h')"); err != nil {
		t.Fatal(err)
	}
	rec := do(mux, http.MethodGet, "/api/menu", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/menu = %d %s", rec.Code, rec.Body)
	}
	if !strings.HasSuffix(rec.Body.String(), `,"display":{"title":"Helm Taproom","theme":"light","page_seconds":20}}`) {
		t.Fatalf("menu = %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "metric") || strings.Contains(rec.Body.String(), "1h") {
		t.Fatalf("menu leaks a non-public setting: %s", rec.Body)
	}
	after := decodeObject(t, rec.Body.String())
	for _, k := range []string{"taps", "on_deck"} {
		if !reflect.DeepEqual(after[k], before[k]) {
			t.Errorf("%s changed: %v, was %v", k, after[k], before[k])
		}
	}
}

// TestMenuWithoutSettings checks that a settings read error serves the menu with the default
// display rather than failing the TV's only read (spec edge case "settings cannot be read").
func TestMenuWithoutSettings(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	beer := insertBeer(t, deps, "Pale")
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 40, 10, 1, 1, 1)", beer); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, created_at, updated_at) VALUES (?, 40, 40, 1, 1)", beer); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.DB.Exec("DROP TABLE settings"); err != nil {
		t.Fatal(err)
	}
	rec := do(mux, http.MethodGet, "/api/menu", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/menu = %d %s", rec.Code, rec.Body)
	}
	m := decodeObject(t, rec.Body.String())
	if len(m["taps"].([]any)) != 1 || len(m["on_deck"].([]any)) != 1 {
		t.Fatalf("menu = %s", rec.Body)
	}
	if !strings.HasSuffix(rec.Body.String(), `,"display":{"title":"","theme":"dark","page_seconds":15}}`) {
		t.Fatalf("menu = %s, want the default display", rec.Body)
	}
}
