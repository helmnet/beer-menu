package httpapi

import (
	"encoding/json"
	"fmt"
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
	"status", "source", "source_ref", "archived_at", "created_at", "updated_at"}

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

// beerTable returns every beers row as text, to prove a request changed nothing.
func beerTable(t *testing.T, deps Deps) string {
	t.Helper()
	rows, err := deps.DB.Query("SELECT * FROM beers ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out strings.Builder
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(&out, vals...)
	}
	return out.String()
}

// TestBeerWritesRefuseWeakCredentials walks every non-GET /api/beers route in Routes, so a beer
// write registered later is covered too, and checks it refuses missing, read-scope, revoked
// and cross-site credentials without changing the table (FR-020, FR-021, SC-003).
func TestBeerWritesRefuseWeakCredentials(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	id := insertBeer(t, deps, "Pale")
	bodies := map[string]string{http.MethodPost: `{"name":"x"}`, http.MethodPatch: `{"abv":1}`, http.MethodDelete: ""}

	var tested []string
	for _, r := range Routes {
		if !strings.HasPrefix(r.Path, "/api/beers") || r.Method == http.MethodGet {
			continue
		}
		name := r.Method + " " + r.Path
		tested = append(tested, name)
		if r.Access != AccessWrite {
			t.Errorf("%s: access %s, want write", name, r.Access)
		}
		path := strings.ReplaceAll(r.Path, "{id}", id)
		body, ok := bodies[r.Method]
		if !ok {
			t.Fatalf("%s: no test body for method", name)
		}
		before := beerTable(t, deps)
		wantError(t, do(mux, r.Method, path, body), 401, "unauthenticated")
		wantError(t, do(mux, r.Method, path, body, c.read...), 403, "forbidden")
		wantError(t, do(mux, r.Method, path, body, c.revoked...), 401, "unauthenticated")
		wantError(t, do(mux, r.Method, path, body, c.crossSite...), 403, "forbidden")
		if after := beerTable(t, deps); after != before {
			t.Errorf("%s: a refused request changed beers:\n%s\n%s", name, before, after)
		}
	}
	want := "POST /api/beers,PATCH /api/beers/{id},DELETE /api/beers/{id}"
	if got := strings.Join(tested, ","); got != want {
		t.Fatalf("beer write routes tested = %s, want %s", got, want)
	}
}

// TestBeerReadsNeedCredential walks every GET /api/beers* and /api/srm-colors route in Routes,
// so a beer read can't silently become public again: each must be AccessRead and refuse a
// caller with no credential (FR-010). Bad-credential cases live in TestGuardAccessTable.
// Feature 5 adds the public menu endpoint.
func TestBeerReadsNeedCredential(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	id := insertBeer(t, deps, "Pale")

	var tested []string
	for _, r := range Routes {
		if r.Method != http.MethodGet || (!strings.HasPrefix(r.Path, "/api/beers") && r.Path != "/api/srm-colors") {
			continue
		}
		name := r.Method + " " + r.Path
		tested = append(tested, name)
		if r.Access != AccessRead {
			t.Errorf("%s: access %s, want read", name, r.Access)
		}
		path := strings.ReplaceAll(r.Path, "{id}", id)
		wantError(t, do(mux, r.Method, path, ""), 401, "unauthenticated")
	}
	// The guard runs before the handler, so an unknown id is not told apart from a known one.
	for _, p := range []string{"/api/beers/999", "/api/beers/abc"} {
		wantError(t, do(mux, http.MethodGet, p, ""), 401, "unauthenticated")
	}
	want := "GET /api/beers,GET /api/beers/{id},GET /api/srm-colors"
	if got := strings.Join(tested, ","); got != want {
		t.Fatalf("beer read routes tested = %s, want %s", got, want)
	}
}

// TestBeerReadsAnyCredential is the success side of TestBeerReadsNeedCredential: every beer read
// works with a read token, a write token or a session, and beers carry their SRM color.
func TestBeerReadsAnyCredential(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	rec := do(mux, http.MethodPost, "/api/beers", `{"name":"Amber","srm":7}`, c.write...)
	amber := strconv.FormatFloat(decodeObject(t, rec.Body.String())["id"].(float64), 'f', -1, 64)
	plain := insertBeer(t, deps, "Plain")

	for name, h := range map[string][]string{"read token": c.read, "write token": c.write, "session": c.session} {
		if rec := do(mux, http.MethodGet, "/api/beers", "", h...); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Amber") {
			t.Errorf("list with %s = %d %s", name, rec.Code, rec.Body)
		}
		rec := do(mux, http.MethodGet, "/api/beers/"+amber, "", h...)
		if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["color"] != beers.SRMColors[6] || b["srm"] != 7.0 {
			t.Errorf("amber with %s = %d %s", name, rec.Code, rec.Body)
		}
		rec = do(mux, http.MethodGet, "/api/beers/"+plain, "", h...)
		if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["color"] != nil || !strings.Contains(rec.Body.String(), `"color":null`) {
			t.Errorf("plain with %s = %d %s", name, rec.Code, rec.Body)
		}
		if rec := do(mux, http.MethodGet, "/api/srm-colors", "", h...); rec.Code != 200 {
			t.Errorf("srm-colors with %s = %d %s", name, rec.Code, rec.Body)
		}
	}
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
