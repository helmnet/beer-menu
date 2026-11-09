package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// tapFields is every field the contract's Tap schema requires.
var tapFields = []string{"id", "position", "label", "enabled", "keg", "created_at", "updated_at"}

// insertTap adds a tap at the end labelled label and returns its id as a string.
func insertTap(t *testing.T, deps Deps, label string) string {
	t.Helper()
	var id string
	if err := deps.DB.QueryRow(`INSERT INTO taps (position, label, created_at, updated_at)
		SELECT COALESCE(MAX(position), 0) + 1, ?, ?, ? FROM taps RETURNING id`, label, t0.Unix(), t0.Unix()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// decodeList fails unless body is a JSON array of objects, and returns it.
func decodeList(t *testing.T, body string) []map[string]any {
	t.Helper()
	var list []map[string]any
	if err := json.Unmarshal([]byte(body), &list); err != nil || list == nil {
		t.Fatalf("body %s: not an array (err %v)", body, err)
	}
	return list
}

// idOf returns the id of a decoded object as a path segment.
func idOf(m map[string]any) string { return strconv.FormatFloat(m["id"].(float64), 'f', -1, 64) }

func TestTapReads(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)
	rec := do(mux, http.MethodGet, "/api/taps", "", c.read...)
	list := decodeList(t, rec.Body.String())
	if rec.Code != 200 || len(list) != 1 || list[0]["label"] != "Tap 1" || !strings.Contains(rec.Body.String(), `"keg":null`) {
		t.Fatalf("GET /api/taps = %d %s", rec.Code, rec.Body)
	}
	for _, f := range tapFields {
		if _, ok := list[0][f]; !ok {
			t.Errorf("tap lacks %s", f)
		}
	}
	if len(list[0]) != len(tapFields) {
		t.Errorf("tap has extra fields: %s", rec.Body)
	}
	rec = do(mux, http.MethodGet, "/api/taps/"+idOf(list[0]), "", c.read...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["label"] != "Tap 1" {
		t.Fatalf("GET tap = %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"/api/taps/999", "/api/taps/0", "/api/taps/abc"} {
		wantError(t, do(mux, http.MethodGet, p, "", c.read...), 404, "not_found")
	}
}

func TestTapWrites(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	c := newCreds(t, deps)

	rec := do(mux, http.MethodPost, "/api/taps", `{}`, c.write...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 201 || b["label"] != "Tap 2" || b["position"] != 2.0 || b["enabled"] != true {
		t.Fatalf("POST {} = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPost, "/api/taps", `{"label":"Nitro"}`, c.session...)
	nitro := decodeObject(t, rec.Body.String())
	if rec.Code != 201 || nitro["label"] != "Nitro" || nitro["position"] != 3.0 {
		t.Fatalf("POST Nitro = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPost, "/api/taps", `{"label":""}`, c.write...)
	if fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any); rec.Code != 400 || fields["label"] == nil {
		t.Fatalf("POST empty label = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodPost, "/api/taps", `{"position":3}`, c.write...), 400, "invalid_request")

	path := "/api/taps/" + idOf(nitro)
	rec = do(mux, http.MethodPatch, path, `{"enabled":false}`, c.write...)
	if b := decodeObject(t, rec.Body.String()); rec.Code != 200 || b["enabled"] != false || b["label"] != "Nitro" {
		t.Fatalf("PATCH enabled = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodPatch, path, `{}`, c.write...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPatch, "/api/taps/999", `{"enabled":true}`, c.write...), 404, "not_found")

	rec = do(mux, http.MethodGet, "/api/taps", "", c.read...)
	var ids []string
	for _, tp := range decodeList(t, rec.Body.String()) {
		ids = append(ids, idOf(tp))
	}
	rec = do(mux, http.MethodPut, "/api/taps/order", `{"ids":[`+ids[2]+`,`+ids[0]+`,`+ids[1]+`]}`, c.write...)
	list := decodeList(t, rec.Body.String())
	if rec.Code != 200 || len(list) != 3 || idOf(list[0]) != ids[2] || list[0]["position"] != 1.0 || idOf(list[2]) != ids[1] {
		t.Fatalf("PUT order = %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, http.MethodPut, "/api/taps/order", `{"ids":[`+ids[0]+`]}`, c.write...)
	if fields, _ := decodeObject(t, rec.Body.String())["fields"].(map[string]any); rec.Code != 400 || fields["ids"] == nil {
		t.Fatalf("PUT bad order = %d %s", rec.Code, rec.Body)
	}

	if rec := do(mux, http.MethodDelete, "/api/taps/"+ids[1], "", c.write...); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodDelete, "/api/taps/"+ids[1], "", c.write...), 404, "not_found")
	beer := insertBeer(t, deps, "Pale")
	if _, err := deps.DB.Exec("INSERT INTO kegs (beer_id, size_l, remaining_l, tap_id, created_at, updated_at) VALUES (?, 1, 1, ?, 1, 1)", beer, ids[2]); err != nil {
		t.Fatal(err)
	}
	wantError(t, do(mux, http.MethodDelete, "/api/taps/"+ids[2], "", c.write...), 409, "tap_has_keg")
	if rec := do(mux, http.MethodDelete, "/api/taps/"+ids[0], "", c.write...); rec.Code != 204 {
		t.Fatalf("DELETE = %d %s", rec.Code, rec.Body)
	}
	if _, err := deps.DB.Exec("DELETE FROM kegs"); err != nil {
		t.Fatal(err)
	}
	wantError(t, do(mux, http.MethodDelete, "/api/taps/"+ids[2], "", c.write...), 409, "last_tap")
}
