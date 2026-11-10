package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/importer"
	"github.com/helmnet/beer-menu/api/internal/importer/importertest"
)

// statusFields is every field of the contract's BrewfatherStatus, sorted.
const statusFields = "configured,connected,interval,last_sync,reconnect_needed,retry_at,sync_running"

// fakeProvider points deps' import service at a new fake provider and returns it.
func fakeProvider(deps Deps) *importertest.Fake {
	fake := importertest.New()
	deps.Importer.Factory = fake.Factory()
	return fake
}

// connectFake connects deps' import service to a new fake provider with fake credentials.
func connectFake(t *testing.T, deps Deps) *importertest.Fake {
	t.Helper()
	fake := fakeProvider(deps)
	if err := deps.Importer.Connect(context.Background(), "fake-user-"+rand.Text(), "fake-key-"+rand.Text()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return fake
}

// closeGate closes the rate gate of deps' import service until until, through one provider
// call refused with a rate limit.
func closeGate(t *testing.T, deps Deps, fake *importertest.Fake, until time.Time) {
	t.Helper()
	fake.SetErr("ListBatches", &importer.RateLimitedError{Until: until})
	if _, err := deps.Importer.Batches(context.Background(), true); err == nil {
		t.Fatal("rate limit did not close the gate")
	}
	fake.SetErr("ListBatches", nil)
}

// shortDeadline shortens providerDeadline for one test.
func shortDeadline(t *testing.T) {
	old := providerDeadline
	providerDeadline = 50 * time.Millisecond
	t.Cleanup(func() { providerDeadline = old })
}

// wantRateLimited fails unless rec is 429 provider_rate_limited with retry_at until and a
// Retry-After of at least 1 whole second matching it.
func wantRateLimited(t *testing.T, rec *httptest.ResponseRecorder, now, until time.Time) {
	t.Helper()
	wantError(t, rec, 429, "provider_rate_limited")
	body := decodeObject(t, rec.Body.String())
	retryAt, err := time.Parse(time.RFC3339, fmt.Sprint(body["retry_at"]))
	if objectKeys(body) != "error,message,retry_at" || err != nil || !retryAt.Equal(until) {
		t.Fatalf("429 body = %s", rec.Body)
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs < 1 || secs != max(1, int((until.Sub(now)+time.Second-1)/time.Second)) {
		t.Fatalf("Retry-After = %q, want whole seconds until %v", rec.Header().Get("Retry-After"), until)
	}
}

func TestGetBrewfather(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	for who, h := range map[string][]string{"session": c.session, "read token": c.read, "write token": c.write} {
		rec := do(mux, http.MethodGet, "/api/brewfather", "", h...)
		if rec.Code != 200 {
			t.Fatalf("%s: GET = %d %s", who, rec.Code, rec.Body)
		}
		if got := objectKeys(decodeObject(t, rec.Body.String())); got != statusFields {
			t.Fatalf("%s: status fields = %s, want %s", who, got, statusFields)
		}
	}
	connectFake(t, deps)
	rec := do(mux, http.MethodGet, "/api/brewfather", "", c.read...)
	want := `{"configured":true,"connected":true,"reconnect_needed":false,"interval":"off","sync_running":false,"retry_at":null,"last_sync":null}`
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("connected status = %d %s", rec.Code, rec.Body)
	}
}

func TestConnectBrewfather(t *testing.T) {
	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := fakeProvider(deps)
	sameOrigin := append([]string{"Sec-Fetch-Site", "same-origin"}, c.session...)
	key := "fake-key-" + rand.Text()
	body := `{"user_id":"fake-user","api_key":"` + key + `"}`

	rec := do(mux, http.MethodPut, "/api/brewfather/connection", body, sameOrigin...)
	if rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	st := decodeObject(t, rec.Body.String())
	if objectKeys(st) != statusFields || st["connected"] != true {
		t.Fatalf("PUT status = %s", rec.Body)
	}
	if _, k := fake.Credentials(); k != key || fake.Calls("Check") != 1 {
		t.Fatalf("Check calls %d", fake.Calls("Check"))
	}

	for name, bad := range map[string]string{
		"not JSON":      `{`,
		"unknown field": `{"user_id":"u","api_key":"k","x":1}`,
		"missing key":   `{"user_id":"u"}`,
		"long user":     `{"user_id":"` + strings.Repeat("u", 201) + `","api_key":"k"}`,
		"long key":      `{"user_id":"u","api_key":"` + strings.Repeat("k", 501) + `"}`,
		"control":       `{"user_id":"u","api_key":"k\u0000k"}`,
		"over 8 KiB":    `{"user_id":"u","api_key":"` + strings.Repeat(" ", 9<<10) + `k"}`,
	} {
		rec := do(mux, http.MethodPut, "/api/brewfather/connection", bad, c.session...)
		wantError(t, rec, 400, "invalid_request")
		if objectKeys(decodeObject(t, rec.Body.String())) != "error,message" {
			t.Fatalf("%s: body = %s", name, rec.Body)
		}
	}
	if fake.Calls("Check") != 1 {
		t.Fatalf("invalid bodies called the provider: %d", fake.Calls("Check"))
	}

	for code, err := range map[string]error{
		"credentials_refused":  importer.ErrCredentialsRefused,
		"no_batch_access":      importer.ErrNoBatchAccess,
		"provider_unreachable": importer.ErrUnreachable,
	} {
		fake.SetErr("Check", err)
		status := 400
		if code == "provider_unreachable" {
			status = 502
		}
		wantError(t, do(mux, http.MethodPut, "/api/brewfather/connection", body, c.session...), status, code)
	}
	until := clock.Now().Add(90*time.Second + 300*time.Millisecond)
	fake.SetErr("Check", &importer.RateLimitedError{Until: until})
	wantRateLimited(t, do(mux, http.MethodPut, "/api/brewfather/connection", body, c.session...), clock.Now(), until)
	// Status still connected: failures keep the stored account.
	if st := decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.session...).Body.String()); st["connected"] != true {
		t.Fatalf("status after failures = %v", st)
	}

	// A gate closing in under a second still asks for at least 1 second.
	clock.Advance(90 * time.Second)
	wantRateLimited(t, do(mux, http.MethodPut, "/api/brewfather/connection", body, c.session...), clock.Now(), until)

	rec = do(mux, http.MethodDelete, "/api/brewfather/connection", "", sameOrigin...)
	if rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodDelete, "/api/brewfather/connection", "", c.session...); rec.Code != 204 {
		t.Fatalf("second DELETE = %d", rec.Code)
	}
	if st := decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.session...).Body.String()); st["connected"] != false {
		t.Fatalf("status after DELETE = %v", st)
	}
}

func TestBrewfatherConnectionAdminOnly(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := connectFake(t, deps)
	body := `{"user_id":"u","api_key":"fake-key-` + rand.Text() + `"}`
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		wantError(t, do(mux, method, "/api/brewfather/connection", body), 401, "unauthenticated")
		wantError(t, do(mux, method, "/api/brewfather/connection", body, c.read...), 403, "forbidden")
		wantError(t, do(mux, method, "/api/brewfather/connection", body, c.write...), 403, "forbidden")
		wantError(t, do(mux, method, "/api/brewfather/connection", body, c.crossSite...), 403, "forbidden")
	}
	if fake.Calls("Check") != 1 {
		t.Fatalf("refused requests called the provider")
	}
	if st, _ := deps.Importer.Status(context.Background()); !st.Connected {
		t.Fatal("a refused DELETE disconnected")
	}
}

func TestBrewfatherNotConfigured(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.Importer.Box = nil
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := fakeProvider(deps)
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/connection", `{"user_id":"u","api_key":"k"}`, c.session...), 503, "not_configured")
	wantError(t, do(mux, http.MethodGet, "/api/brewfather/batches", "", c.session...), 503, "not_configured")
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a"]}`, c.session...), 503, "not_configured")
	if st := decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.session...).Body.String()); st["configured"] != false {
		t.Fatalf("status = %v", st)
	}
	if fake.TotalCalls() != 0 {
		t.Fatal("not configured but the provider was called")
	}
}

func TestListBrewfatherBatches(t *testing.T) {
	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	wantError(t, do(mux, http.MethodGet, "/api/brewfather/batches", "", c.session...), 409, "not_connected")

	fake := connectFake(t, deps)
	n := int64(7)
	fake.SetBatches(
		importer.Batch{Ref: "a", Name: "Pale", Number: &n, Status: "ready", BrewDate: ptr("2026-10-01")},
		importer.Batch{Ref: "b", Name: "Stout", Status: "fermenting"},
		importer.Batch{Ref: "z", Name: "Old", Status: "ready", Archived: true},
	)
	wantError(t, do(mux, http.MethodGet, "/api/brewfather/batches", "", c.read...), 403, "forbidden")
	wantError(t, do(mux, http.MethodGet, "/api/brewfather/batches?refresh=maybe", "", c.write...), 400, "invalid_request")

	rec := do(mux, http.MethodGet, "/api/brewfather/batches", "", c.write...)
	want := `{"batches":[{"id":"a","name":"Pale","number":7,"status":"ready","brew_date":"2026-10-01","beer_id":null},` +
		`{"id":"b","name":"Stout","number":null,"status":"fermenting","brew_date":null,"beer_id":null}],"fetched_at":"2026-11-08T12:00:00Z"}`
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("GET batches = %d %s\nwant %s", rec.Code, rec.Body, want)
	}
	if rec := do(mux, http.MethodGet, "/api/brewfather/batches", "", c.session...); rec.Code != 200 || fake.Calls("ListBatches") != 1 {
		t.Fatalf("cached GET = %d, list calls %d", rec.Code, fake.Calls("ListBatches"))
	}
	if rec := do(mux, http.MethodGet, "/api/brewfather/batches?refresh=true", "", c.session...); rec.Code != 200 || fake.Calls("ListBatches") != 2 {
		t.Fatalf("refresh GET = %d, list calls %d", rec.Code, fake.Calls("ListBatches"))
	}

	until := clock.Now().Add(2 * time.Minute)
	closeGate(t, deps, fake, until)
	calls := fake.TotalCalls()
	wantRateLimited(t, do(mux, http.MethodGet, "/api/brewfather/batches?refresh=true", "", c.session...), clock.Now(), until)
	wantRateLimited(t, do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a"]}`, c.session...), clock.Now(), until)
	if fake.TotalCalls() != calls {
		t.Fatalf("closed gate but provider calls %d -> %d", calls, fake.TotalCalls())
	}
}

func TestImportBrewfatherBatches(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a"]}`, c.session...), 409, "not_connected")
	fake := connectFake(t, deps)
	fake.SetBatches(
		importer.Batch{Ref: "a", Name: "Pale", Status: "ready", ABV: ptr(5.25)},
		importer.Batch{Ref: "b", Name: "Stout", Status: "fermenting"},
		importer.Batch{Ref: "z", Name: "Old", Status: "ready", Archived: true},
	)

	many := make([]string, 51)
	for i := range many {
		many[i] = fmt.Sprintf("%q", fmt.Sprint(i))
	}
	for name, bad := range map[string]string{
		"missing":    `{}`,
		"empty":      `{"batch_ids":[]}`,
		"over 50":    `{"batch_ids":[` + strings.Join(many, ",") + `]}`,
		"duplicated": `{"batch_ids":["a","a"]}`,
		"non-string": `{"batch_ids":[1]}`,
		"empty id":   `{"batch_ids":[""]}`,
		"long id":    `{"batch_ids":["` + strings.Repeat("x", 101) + `"]}`,
		"unknown":    `{"batch_ids":["a"],"x":1}`,
	} {
		rec := do(mux, http.MethodPost, "/api/brewfather/imports", bad, c.session...)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"error":"invalid_request"`) {
			t.Errorf("%s: %d %s, want 400 invalid_request", name, rec.Code, rec.Body)
		}
	}
	if fake.Calls("GetBatch") != 0 {
		t.Fatal("invalid bodies called the provider")
	}
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a"]}`, c.read...), 403, "forbidden")

	rec := do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a","b","z","nope"]}`, c.write...)
	if rec.Code != 200 {
		t.Fatalf("import = %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Results) != 4 {
		t.Fatalf("import body = %s", rec.Body)
	}
	if objectKeys(decodeObject(t, rec.Body.String())) != "results" {
		t.Fatalf("import body fields = %s", rec.Body)
	}
	var outcomes []string
	for _, r := range out.Results {
		if objectKeys(r) != "adjusted,batch_id,beer,message,outcome" {
			t.Fatalf("result fields = %v", r)
		}
		outcomes = append(outcomes, r["batch_id"].(string)+"="+r["outcome"].(string))
	}
	if strings.Join(outcomes, ",") != "a=imported,b=imported,z=archived,nope=not_found" {
		t.Fatalf("outcomes = %v", outcomes)
	}
	pale := out.Results[0]["beer"].(map[string]any)
	sync, _ := pale["sync"].(map[string]any)
	if len(pale) != len(beerFields) || pale["source"] != "brewfather" || pale["source_ref"] != "a" || sync == nil || sync["synced_at"] != "2026-11-08T12:00:00Z" ||
		fmt.Sprint(out.Results[0]["adjusted"]) != "[abv]" {
		t.Fatalf("imported beer = %v", out.Results[0])
	}

	// An imported fermenting beer with no keg is on deck (spec 2.9).
	menu := do(mux, http.MethodGet, "/api/menu", "").Body.String()
	if !strings.Contains(menu, `"name":"Stout"`) {
		t.Fatalf("menu lacks the fermenting import: %s", menu)
	}
}

func TestBrewfatherDeadline(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := connectFake(t, deps)
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Pale", Status: "ready"}, importer.Batch{Ref: "b", Name: "Stout", Status: "ready"})
	shortDeadline(t)
	fake.SetBlock(true)

	start := time.Now()
	wantError(t, do(mux, http.MethodGet, "/api/brewfather/batches", "", c.session...), 502, "provider_unreachable")
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/connection", `{"user_id":"u","api_key":"fake-key"}`, c.session...), 502, "provider_unreachable")
	// The import contract answers 200 with every batch failed rather than 502.
	rec := do(mux, http.MethodPost, "/api/brewfather/imports", `{"batch_ids":["a","b"]}`, c.session...)
	if rec.Code != 200 || strings.Count(rec.Body.String(), `"outcome":"failed"`) != 2 || !strings.Contains(rec.Body.String(), "Not attempted") {
		t.Fatalf("import past the deadline = %d %s", rec.Code, rec.Body)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("requests took %v; the deadline did not apply", d)
	}
}

// importFake imports batch ref through deps' import service and returns the beer's id.
func importFake(t *testing.T, deps Deps, ref string) string {
	t.Helper()
	res, err := deps.Importer.Import(context.Background(), []string{ref})
	if err != nil || res[0].Outcome != importer.OutcomeImported {
		t.Fatalf("import %s = %+v (err %v)", ref, res, err)
	}
	return strconv.FormatInt(res[0].Beer.ID, 10)
}

func TestResyncBeer(t *testing.T) {
	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := connectFake(t, deps)
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Pale", Status: "fermenting", ABV: ptr(5.0)},
		importer.Batch{Ref: "b", Name: "Stout", Status: "fermenting"})
	id, kegged := importFake(t, deps, "a"), importFake(t, deps, "b")
	insertKeg(t, deps, kegged, "")
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Renamed", Status: "ready", ABV: ptr(5.56)},
		importer.Batch{Ref: "b", Name: "Stout", Status: "ready", Archived: true})

	wantError(t, do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.read...), 403, "forbidden")
	rec := do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.write...)
	if rec.Code != 200 || objectKeys(decodeObject(t, rec.Body.String())) != "adjusted,beer" {
		t.Fatalf("re-sync = %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Beer     map[string]any `json:"beer"`
		Adjusted []string       `json:"adjusted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Beer) != len(beerFields) || out.Beer["name"] != "Pale" || out.Beer["abv"] != 5.6 || out.Beer["status"] != "ready" ||
		fmt.Sprint(out.Adjusted) != "[abv]" {
		t.Fatalf("re-sync body = %s", rec.Body)
	}

	// The archive waits on the current keg, and GET shows it.
	if rec := do(mux, http.MethodPost, "/api/beers/"+kegged+"/sync", "", c.session...); rec.Code != 200 {
		t.Fatalf("re-sync with keg = %d %s", rec.Code, rec.Body)
	}
	if body := do(mux, http.MethodGet, "/api/beers/"+kegged, "", c.read...).Body.String(); !strings.Contains(body, `"archive_pending":true`) ||
		!strings.Contains(body, `"archived_at":null`) {
		t.Fatalf("GET after archive pending = %s", body)
	}
	// A missing batch: 200, marked not found.
	fake.SetBatches()
	if rec := do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.session...); rec.Code != 200 || fmt.Sprint(decodeObject(t, rec.Body.String())["adjusted"]) != "[]" {
		t.Fatalf("re-sync of a missing batch = %d %s", rec.Code, rec.Body)
	}
	if body := do(mux, http.MethodGet, "/api/beers/"+id, "", c.read...).Body.String(); !strings.Contains(body, `"not_found":true`) {
		t.Fatalf("GET after not found = %s", body)
	}

	manual := insertBeer(t, deps, "Manual")
	wantError(t, do(mux, http.MethodPost, "/api/beers/"+manual+"/sync", "", c.session...), 409, "not_imported")
	for _, bad := range []string{"999", "0", "-1", "x"} {
		wantError(t, do(mux, http.MethodPost, "/api/beers/"+bad+"/sync", "", c.session...), 404, "not_found")
	}

	until := clock.Now().Add(time.Minute)
	fake.SetErr("GetBatch", &importer.RateLimitedError{Until: until})
	wantRateLimited(t, do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.session...), clock.Now(), until)
	calls := fake.TotalCalls()
	wantRateLimited(t, do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.session...), clock.Now(), until)
	if fake.TotalCalls() != calls {
		t.Fatal("closed gate but the provider was called")
	}
}

func TestResyncBeerDeadline(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	fake := connectFake(t, deps)
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Pale", Status: "ready"})
	id := importFake(t, deps, "a")
	shortDeadline(t)
	fake.SetBlock(true)
	start := time.Now()
	wantError(t, do(mux, http.MethodPost, "/api/beers/"+id+"/sync", "", c.session...), 502, "provider_unreachable")
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("re-sync took %v; the deadline did not apply", d)
	}
}

func TestSetBrewfatherSchedule(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	rec := do(mux, http.MethodPut, "/api/brewfather/schedule", `{"interval":"1h"}`, append([]string{"Sec-Fetch-Site", "same-origin"}, c.session...)...)
	if rec.Code != 200 {
		t.Fatalf("PUT = %d %s", rec.Code, rec.Body)
	}
	if st := decodeObject(t, rec.Body.String()); objectKeys(st) != statusFields || st["interval"] != "1h" {
		t.Fatalf("PUT status = %s", rec.Body)
	}
	for name, bad := range map[string]string{
		"unknown interval": `{"interval":"2h"}`,
		"missing":          `{}`,
		"not a string":     `{"interval":15}`,
		"extra field":      `{"interval":"off","x":1}`,
	} {
		rec := do(mux, http.MethodPut, "/api/brewfather/schedule", bad, c.session...)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), `"error":"invalid_request"`) {
			t.Errorf("%s: %d %s, want 400 invalid_request", name, rec.Code, rec.Body)
		}
	}
	body := `{"interval":"off"}`
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/schedule", body), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/schedule", body, c.read...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/schedule", body, c.write...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPut, "/api/brewfather/schedule", body, c.crossSite...), 403, "forbidden")
	if st := decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.read...).Body.String()); st["interval"] != "1h" {
		t.Fatalf("interval after refused writes = %v", st["interval"])
	}
}

func TestSyncBrewfather(t *testing.T) {
	deps, _ := newTestDeps(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.Background = ctx
	mux := NewMux(deps)
	c := newCreds(t, deps)
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/sync", "", c.session...), 409, "not_connected")
	fake := connectFake(t, deps)
	fake.SetBatches(importer.Batch{Ref: "a", Name: "Pale", Status: "fermenting"})
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/sync", "", c.read...), 403, "forbidden")

	rec := do(mux, http.MethodPost, "/api/brewfather/sync", "", c.write...)
	if rec.Code != 202 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	if st := decodeObject(t, rec.Body.String()); objectKeys(st) != statusFields || st["sync_running"] != true {
		t.Fatalf("POST status = %s", rec.Body)
	}
	deps.Importer.Wait()
	st := decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.read...).Body.String())
	last, _ := st["last_sync"].(map[string]any)
	if st["sync_running"] != false || last == nil || last["ok"] != true || last["imported"] != 1.0 || last["finished_at"] == nil {
		t.Fatalf("status after sync = %v", st)
	}

	// A second sync while one runs is refused.
	fake.SetBlock(true)
	if rec := do(mux, http.MethodPost, "/api/brewfather/sync", "", c.session...); rec.Code != 202 {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodPost, "/api/brewfather/sync", "", c.session...), 409, "sync_running")
	// Shutdown ends the background sync.
	cancel()
	deps.Importer.Wait()
	st = decodeObject(t, do(mux, http.MethodGet, "/api/brewfather", "", c.read...).Body.String())
	if last, _ := st["last_sync"].(map[string]any); st["sync_running"] != false || last["ok"] != false {
		t.Fatalf("status after a cancelled sync = %v", st)
	}
}

func ptr[T any](v T) *T { return &v }
