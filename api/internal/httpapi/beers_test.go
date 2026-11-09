package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/beers"
)

// beerFields is every field the contract's Beer schema requires.
var beerFields = []string{"id", "name", "style", "abv", "ibu", "srm", "color", "description", "brewer", "brew_date",
	"status", "source", "source_ref", "archived_at", "on_tap", "created_at", "updated_at"}

// decodeObject fails unless body is a JSON object, and returns it.
func decodeObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	return m
}

// insertBeer adds a beer row with only a name and returns its id as a string.
func insertBeer(t *testing.T, deps Deps, name string) string {
	t.Helper()
	var id string
	if err := deps.DB.QueryRow("INSERT INTO beers (name, created_at, updated_at) VALUES (?, ?, ?) RETURNING id", name, t0.Unix(), t0.Unix()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestBeerReads(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	rec := do(mux, http.MethodGet, "/api/beers", "", c.read...)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list = %d %s, want 200 []", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'none'") {
		t.Fatalf("list headers = %v", rec.Header())
	}

	id := insertBeer(t, deps, "Pale")
	rec = do(mux, http.MethodGet, "/api/beers/"+id, "", c.read...)
	if rec.Code != 200 {
		t.Fatalf("GET beer = %d %s", rec.Code, rec.Body)
	}
	b := decodeObject(t, rec.Body.String())
	for _, f := range beerFields {
		if _, ok := b[f]; !ok {
			t.Errorf("beer lacks %s: %s", f, rec.Body)
		}
	}
	if len(b) != len(beerFields) || b["name"] != "Pale" || b["color"] != nil || b["abv"] != nil || b["status"] != "ready" || b["source"] != "manual" {
		t.Fatalf("beer = %s", rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("beer Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}

	rec = do(mux, http.MethodGet, "/api/beers", "", c.read...)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != 200 || len(list) != 1 || list[0]["name"] != "Pale" {
		t.Fatalf("list = %d %s (err %v)", rec.Code, rec.Body, err)
	}

	for _, p := range []string{"/api/beers/999", "/api/beers/0", "/api/beers/-1", "/api/beers/abc"} {
		wantError(t, do(mux, http.MethodGet, p, "", c.read...), 404, "not_found")
	}

	rec = do(mux, http.MethodPost, "/api/beers", `{"name":"Amber","srm":7}`, c.write...)
	amber := strconv.FormatFloat(decodeObject(t, rec.Body.String())["id"].(float64), 'f', -1, 64)
	rec = do(mux, http.MethodGet, "/api/beers/"+amber, "", c.read...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["color"] != beers.SRMColors[6] || b["srm"] != 7.0 {
		t.Fatalf("amber = %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(do(mux, http.MethodGet, "/api/beers/"+id, "", c.read...).Body.String(), `"color":null`) {
		t.Fatal("beer without SRM lacks color null")
	}
}

// creds holds header pairs for each kind of caller of a beer route.
type creds struct {
	read, revoked, write, session, crossSite []string
}

// newCreds makes a read token, a revoked write token, a write token and an admin session.
func newCreds(t *testing.T, deps Deps) creds {
	t.Helper()
	readSecret, _ := insertToken(t, deps.DB, auth.ScopeRead, false)
	revokedSecret, _ := insertToken(t, deps.DB, auth.ScopeWrite, true)
	writeSecret, _ := insertToken(t, deps.DB, auth.ScopeWrite, false)
	session := sessionFor(t, deps, insertUser(t, deps.DB, "owner"))
	return creds{
		read:      []string{"Authorization", "Bearer " + readSecret},
		revoked:   []string{"Authorization", "Bearer " + revokedSecret},
		write:     []string{"Authorization", "Bearer " + writeSecret},
		session:   session,
		crossSite: append([]string{"Sec-Fetch-Site", "cross-site"}, session...),
	}
}

func TestCreateBeer(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	rec := do(mux, http.MethodPost, "/api/beers", `{"name":"Honey IPA","style":"American IPA","abv":6.7,"ibu":55,"srm":7,"brewer":"Helm","brew_date":"2026-10-01","status":"ready"}`, c.write...)
	if rec.Code != 201 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	b := decodeObject(t, rec.Body.String())
	if len(b) != len(beerFields) || b["name"] != "Honey IPA" || b["abv"] != 6.7 || b["ibu"] != 55.0 || b["source"] != "manual" || b["source_ref"] != nil || b["description"] != nil {
		t.Fatalf("created = %s", rec.Body)
	}
	if rec := do(mux, http.MethodPost, "/api/beers", `{"name":"Session beer"}`, c.session...); rec.Code != 201 {
		t.Fatalf("POST with session = %d %s", rec.Code, rec.Body)
	}
	// The contract allows status null on create and treats it as absent.
	if rec := do(mux, http.MethodPost, "/api/beers", `{"name":"No status","status":null}`, c.write...); rec.Code != 201 || decodeObject(t, rec.Body.String())["status"] != "ready" {
		t.Fatalf("POST status null = %d %s, want 201 ready", rec.Code, rec.Body)
	}

	rec = do(mux, http.MethodPost, "/api/beers", `{"name":"X","abv":150}`, c.write...)
	wantError(t, rec, 400, "invalid_request")
	fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any)
	if len(fields) != 1 || fields["abv"] == nil {
		t.Fatalf("fields = %s", rec.Body)
	}
	for _, body := range []string{`{"name":"X","source":"brewfather"}`, `{"name":"X","source_ref":"b-1"}`, `{"name":"X"} {}`} {
		rec := do(mux, http.MethodPost, "/api/beers", body, c.write...)
		wantError(t, rec, 400, "invalid_request")
		if strings.Contains(rec.Body.String(), `"fields"`) {
			t.Errorf("%s: decode error has fields: %s", body, rec.Body)
		}
	}

	// pad returns a create body of exactly n bytes.
	pad := func(n int) string {
		return `{"name":"P","description":"` + strings.Repeat("d", n-len(`{"name":"P","description":""}`)) + `"}`
	}
	// The description rule refuses 16 KiB of text, so only the size check decides which wins:
	// at the limit the body decodes and fails validation; one byte more fails to decode.
	rec = do(mux, http.MethodPost, "/api/beers", pad(maxBeerBody), c.write...)
	if fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any); rec.Code != 400 || fields["description"] == nil {
		t.Fatalf("16 KiB body = %d %s, want a description field error", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPost, "/api/beers", pad(maxBeerBody+1), c.write...)
	wantError(t, rec, 400, "invalid_request")
	if strings.Contains(rec.Body.String(), `"fields"`) {
		t.Fatalf("16 KiB + 1 body was decoded: %s", rec.Body)
	}
	// A 2000-character description of 6-byte escapes still fits.
	if rec := do(mux, http.MethodPost, "/api/beers", `{"name":"Big","description":"`+strings.Repeat(`\u00e9`, 2000)+`"}`, c.write...); rec.Code != 201 {
		t.Fatalf("2000 escaped characters = %d %s", rec.Code, rec.Body)
	}
}

func TestUpdateBeer(t *testing.T) {
	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	rec := do(mux, http.MethodPost, "/api/beers", `{"name":"Pale","style":"APA","abv":5}`, c.write...)
	id := decodeObject(t, rec.Body.String())["id"].(float64)
	path := "/api/beers/" + strconv.FormatFloat(id, 'f', -1, 64)

	clock.Advance(time.Hour)
	rec = do(mux, http.MethodPatch, path, `{"abv":5.4}`, c.write...)
	b := decodeObject(t, rec.Body.String())
	if rec.Code != 200 || b["abv"] != 5.4 || b["style"] != "APA" || b["name"] != "Pale" || b["updated_at"] == b["created_at"] {
		t.Fatalf("PATCH abv = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPatch, path, `{"style":null}`, c.session...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["style"] != nil || b["abv"] != 5.4 {
		t.Fatalf("PATCH style null = %d %s", rec.Code, rec.Body)
	}

	for _, body := range []string{`{}`, `{"source":"brewfather"}`, `{"source_ref":"x"}`, `{"name":null}`} {
		wantError(t, do(mux, http.MethodPatch, path, body, c.write...), 400, "invalid_request")
	}
	rec = do(mux, http.MethodPatch, path, `{"abv":150}`, c.write...)
	if fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any); rec.Code != 400 || fields["abv"] == nil {
		t.Fatalf("PATCH abv 150 = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodPatch, "/api/beers/999", `{"abv":1}`, c.write...), 404, "not_found")
	wantError(t, do(mux, http.MethodPatch, "/api/beers/abc", `{"abv":1}`, c.write...), 404, "not_found")
}

func TestSRMColors(t *testing.T) {
	deps, _ := newTestDeps(t)
	rec := do(NewMux(deps), http.MethodGet, "/api/srm-colors", "", newCreds(t, deps).read...)
	var body struct {
		Colors []string `json:"colors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != 200 {
		t.Fatalf("GET /api/srm-colors = %d %s (err %v)", rec.Code, rec.Body, err)
	}
	if len(body.Colors) != len(beers.SRMColors) || len(decodeObject(t, rec.Body.String())) != 1 {
		t.Fatalf("body = %s", rec.Body)
	}
	for i, c := range body.Colors {
		if c != beers.SRMColors[i] {
			t.Fatalf("colors[%d] = %s, want %s", i, c, beers.SRMColors[i])
		}
	}
}

func TestArchiveBeer(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	id := insertBeer(t, deps, "Pale")
	path := "/api/beers/" + id
	listed := func(query string) string {
		t.Helper()
		rec := do(mux, http.MethodGet, "/api/beers"+query, "", c.read...)
		if rec.Code != 200 {
			t.Fatalf("GET /api/beers%s = %d %s", query, rec.Code, rec.Body)
		}
		return rec.Body.String()
	}

	rec := do(mux, http.MethodPatch, path, `{"archived":true}`, c.write...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["archived_at"] == nil {
		t.Fatalf("archive = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(listed(""), "Pale") || strings.Contains(listed("?archived=false"), "Pale") {
		t.Fatal("archived beer in the default list")
	}
	if !strings.Contains(listed("?archived=true"), "Pale") {
		t.Fatal("archived beer missing from ?archived=true")
	}
	if b := decodeObject(t, do(mux, http.MethodGet, path, "", c.read...).Body.String()); b["archived_at"] == nil {
		t.Fatal("GET by id lost archived_at")
	}
	wantError(t, do(mux, http.MethodGet, "/api/beers?archived=yes", "", c.read...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPatch, path, `{"archived":null}`, c.write...), 400, "invalid_request")

	rec = do(mux, http.MethodPatch, path, `{"archived":false}`, c.session...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["archived_at"] != nil {
		t.Fatalf("restore = %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(listed(""), "Pale") || strings.Contains(listed("?archived=true"), "Pale") {
		t.Fatal("restored beer not back in the default list")
	}
}

func TestDeleteBeer(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for name, h := range map[string][]string{"write token": c.write, "session": c.session} {
		path := "/api/beers/" + insertBeer(t, deps, "Gone")
		if rec := do(mux, http.MethodDelete, path, "", h...); rec.Code != 204 || rec.Body.Len() != 0 {
			t.Fatalf("DELETE with %s = %d %s", name, rec.Code, rec.Body)
		}
		wantError(t, do(mux, http.MethodGet, path, "", h...), 404, "not_found")
		wantError(t, do(mux, http.MethodDelete, path, "", h...), 404, "not_found")
	}
	for _, p := range []string{"/api/beers/999", "/api/beers/0", "/api/beers/x"} {
		wantError(t, do(mux, http.MethodDelete, p, "", c.write...), 404, "not_found")
	}
}

func TestBeerKegConflicts(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	id := insertBeer(t, deps, "Pale")
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 1, 1, 1, 1, 1)", id); err != nil {
		t.Fatal(err)
	}
	rec := do(mux, http.MethodGet, "/api/beers", "", c.read...)
	if list := decodeList(t, rec.Body.String()); len(list) != 1 || list[0]["on_tap"] != true {
		t.Fatalf("GET /api/beers = %s, want on_tap true", rec.Body)
	}
	wantError(t, do(mux, http.MethodPatch, "/api/beers/"+id, `{"archived":true}`, c.write...), 409, "beer_has_keg")
	wantError(t, do(mux, http.MethodDelete, "/api/beers/"+id, "", c.write...), 409, "beer_has_kegs")
	if _, err := deps.DB.Exec("UPDATE kegs SET kicked_at = 2"); err != nil {
		t.Fatal(err)
	}
	if b := decodeObject(t, do(mux, http.MethodGet, "/api/beers/"+id, "", c.read...).Body.String()); b["on_tap"] != false {
		t.Fatalf("on_tap after kick = %v", b["on_tap"])
	}
	wantError(t, do(mux, http.MethodDelete, "/api/beers/"+id, "", c.write...), 409, "beer_has_kegs")
	if rec := do(mux, http.MethodPatch, "/api/beers/"+id, `{"archived":true}`, c.write...); rec.Code != 200 {
		t.Fatalf("archive with only kicked kegs = %d %s", rec.Code, rec.Body)
	}
}
