package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/kegs"
)

func TestListKegTypes(t *testing.T) {
	deps, _ := newTestDeps(t)
	rec := do(NewMux(deps), http.MethodGet, "/api/keg-types", "", newCreds(t, deps).read...)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || rec.Code != 200 || len(list) != 9 {
		t.Fatalf("GET /api/keg-types = %d %s (err %v)", rec.Code, rec.Body, err)
	}
	for i, kt := range list {
		want := kegs.Types[i]
		if len(kt) != 4 || kt["type"] != want.Type || kt["label"] != want.Label || kt["size_l"] != want.SizeL || kt["shape"] != want.Shape {
			t.Errorf("keg type %d = %v, want %+v", i, kt, want)
		}
	}
}

// kegFields is every field the contract's Keg schema requires.
var kegFields = []string{"id", "beer_id", "beer_name", "beer_color", "type", "size_l", "remaining_l", "tap_id", "tap_label",
	"state", "tapped_at", "kicked_at", "created_at", "updated_at"}

// wantKeg fails unless rec is status code with a Keg body holding exactly the contract's fields,
// and returns the keg.
func wantKeg(t *testing.T, rec *httptest.ResponseRecorder, code int) map[string]any {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, code, rec.Body)
	}
	k := decodeObject(t, rec.Body.String())
	for _, f := range kegFields {
		if _, ok := k[f]; !ok {
			t.Errorf("keg lacks %s: %s", f, rec.Body)
		}
	}
	if len(k) != len(kegFields) {
		t.Errorf("keg has extra fields: %s", rec.Body)
	}
	return k
}

// wantFields fails unless rec is 400 invalid_request with exactly the field errors named.
func wantFields(t *testing.T, rec *httptest.ResponseRecorder, names ...string) {
	t.Helper()
	wantError(t, rec, 400, "invalid_request")
	fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any)
	if len(fields) != len(names) {
		t.Fatalf("fields = %s, want %v", rec.Body, names)
	}
	for _, n := range names {
		if fields[n] == nil {
			t.Fatalf("fields = %s, want %v", rec.Body, names)
		}
	}
}

// insertKeg adds a keg of beer on tap ("" for waiting) and returns its id as a string.
func insertKeg(t *testing.T, deps Deps, beer, tap string) string {
	t.Helper()
	var tapID any
	if tap != "" {
		tapID = tap
	}
	var id string
	if err := deps.DB.QueryRow("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 18.93, 18.93, ?, ?, ?) RETURNING id",
		beer, tapID, t0.Unix(), t0.Unix()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestKegWrites(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	beer := insertBeer(t, deps, "Pale")

	k := wantKeg(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`}`, c.write...), 201)
	if k["state"] != "waiting" || k["size_l"] != 18.93 || k["type"] != "corny_5_gal" || k["beer_name"] != "Pale" || k["tap_id"] != nil {
		t.Fatalf("waiting keg = %v", k)
	}
	waiting := idOf(k)
	onTap := wantKeg(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`,"type":"sanke_half","tap_id":1}`, c.session...), 201)
	if onTap["state"] != "on_tap" || onTap["tap_id"] != 1.0 || onTap["tap_label"] != "Tap 1" || onTap["size_l"] != 58.67 || onTap["tapped_at"] == nil {
		t.Fatalf("keg on tap = %v", onTap)
	}
	wantFields(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":999}`, c.write...), "beer_id")
	wantFields(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`,"tap_id":99}`, c.write...), "tap_id")
	wantFields(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`,"type":"keg"}`, c.write...), "type")
	wantFields(t, do(mux, http.MethodPost, "/api/kegs", `{}`, c.write...), "beer_id")
	wantError(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`,"state":"kicked"}`, c.write...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/kegs", `{"beer_id":`+beer+`,"tap_id":1}`, c.write...), 409, "tap_occupied")

	rec := do(mux, http.MethodGet, "/api/taps", "", c.read...)
	if tp := decodeList(t, rec.Body.String())[0]; tp["keg"] == nil || idOf(tp["keg"].(map[string]any)) != idOf(onTap) {
		t.Fatalf("GET /api/taps = %s, want the keg embedded", rec.Body)
	}

	path := "/api/kegs/" + waiting
	wantError(t, do(mux, http.MethodPatch, "/api/kegs/"+waiting, `{"tap_id":1}`, c.write...), 409, "tap_occupied")
	if k := wantKeg(t, do(mux, http.MethodPatch, "/api/kegs/"+idOf(onTap), `{"tap_id":null}`, c.write...), 200); k["state"] != "waiting" || k["tap_label"] != "Tap 1" {
		t.Fatalf("take off = %v", k)
	}
	if k := wantKeg(t, do(mux, http.MethodPatch, path, `{"tap_id":1}`, c.write...), 200); k["state"] != "on_tap" {
		t.Fatalf("put on = %v", k)
	}
	wantError(t, do(mux, http.MethodPatch, path, `{}`, c.write...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPatch, path, `{"state":"x"}`, c.write...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPatch, "/api/kegs/999", `{"size_l":1}`, c.write...), 404, "not_found")

	if rec := do(mux, http.MethodDelete, path, "", c.write...); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodDelete, path, "", c.write...), 404, "not_found")
	wantError(t, do(mux, http.MethodGet, path, "", c.read...), 404, "not_found")
}

func TestKegReads(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	pale, stout := insertBeer(t, deps, "Pale"), insertBeer(t, deps, "Stout")
	onTap := insertKeg(t, deps, pale, "1")
	waiting := insertKeg(t, deps, stout, "")

	list := func(query string) string {
		t.Helper()
		rec := do(mux, http.MethodGet, "/api/kegs"+query, "", c.read...)
		if rec.Code != 200 {
			t.Fatalf("GET /api/kegs%s = %d %s", query, rec.Code, rec.Body)
		}
		var ids []string
		for _, k := range decodeList(t, rec.Body.String()) {
			ids = append(ids, idOf(k))
		}
		return strings.Join(ids, ",")
	}
	for query, want := range map[string]string{
		"":                               onTap + "," + waiting,
		"?state=current":                 onTap + "," + waiting,
		"?state=waiting":                 waiting,
		"?state=kicked":                  "",
		"?beer_id=" + stout:              waiting,
		"?tap_id=1":                      onTap,
		"?beer_id=999":                   "",
		"?beer_id=" + pale + "&tap_id=1": onTap,
	} {
		if got := list(query); got != want {
			t.Errorf("GET /api/kegs%s = %s, want %s", query, got, want)
		}
	}
	for _, q := range []string{"?state=x", "?beer_id=abc", "?beer_id=0", "?tap_id=-1", "?tap_id="} {
		wantError(t, do(mux, http.MethodGet, "/api/kegs"+q, "", c.read...), 400, "invalid_request")
	}
	if k := wantKeg(t, do(mux, http.MethodGet, "/api/kegs/"+onTap, "", c.read...), 200); k["beer_name"] != "Pale" || k["state"] != "on_tap" {
		t.Fatalf("GET keg = %v", k)
	}
	for _, p := range []string{"/api/kegs/999", "/api/kegs/0", "/api/kegs/abc"} {
		wantError(t, do(mux, http.MethodGet, p, "", c.read...), 404, "not_found")
	}
}

func TestPourAndLevel(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	path := "/api/kegs/" + insertKeg(t, deps, insertBeer(t, deps, "Pale"), "1")

	if k := wantKeg(t, do(mux, http.MethodPost, path+"/pours", `{"amount_l":0.5}`, c.write...), 200); k["remaining_l"] != 18.43 {
		t.Fatalf("pour = %v", k)
	}
	wantFields(t, do(mux, http.MethodPost, path+"/pours", `{}`, c.write...), "amount_l")
	wantFields(t, do(mux, http.MethodPost, path+"/pours", `{"amount_l":0}`, c.write...), "amount_l")
	wantError(t, do(mux, http.MethodPost, path+"/pours", `{"amount_l":0.5,"unit":"l"}`, c.write...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/kegs/999/pours", `{"amount_l":0.5}`, c.write...), 404, "not_found")
	if k := wantKeg(t, do(mux, http.MethodPatch, path, `{"remaining_l":10}`, c.write...), 200); k["remaining_l"] != 10.0 {
		t.Fatalf("set level = %v", k)
	}
}

func TestKickKeg(t *testing.T) {
	deps, clock := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	beer := insertBeer(t, deps, "Pale")
	first := insertKeg(t, deps, beer, "1")
	second := insertKeg(t, deps, beer, "")

	wantFields(t, do(mux, http.MethodPatch, "/api/kegs/"+first, `{"kicked":false}`, c.write...), "kicked")
	k := wantKeg(t, do(mux, http.MethodPatch, "/api/kegs/"+first, `{"kicked":true}`, c.write...), 200)
	if k["state"] != "kicked" || k["kicked_at"] == nil || k["tap_label"] != "Tap 1" {
		t.Fatalf("kick = %v", k)
	}
	if again := wantKeg(t, do(mux, http.MethodPatch, "/api/kegs/"+first, `{"kicked":true}`, c.write...), 200); again["kicked_at"] != k["kicked_at"] {
		t.Fatalf("repeat kick = %v", again)
	}
	wantError(t, do(mux, http.MethodPatch, "/api/kegs/"+first, `{"remaining_l":1}`, c.write...), 409, "keg_kicked")
	wantError(t, do(mux, http.MethodPost, "/api/kegs/"+first+"/pours", `{"amount_l":0.5}`, c.write...), 409, "keg_kicked")

	clock.Advance(time.Hour)
	wantKeg(t, do(mux, http.MethodPatch, "/api/kegs/"+second, `{"kicked":true}`, c.write...), 200)
	rec := do(mux, http.MethodGet, "/api/kegs?state=kicked", "", c.read...)
	if list := decodeList(t, rec.Body.String()); len(list) != 2 || idOf(list[0]) != second || idOf(list[1]) != first {
		t.Fatalf("history = %s", rec.Body)
	}
}
