package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	var got body
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = body{}
		if !decodeJSON(w, r, &got, maxBody) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	if rec := do(h, http.MethodPost, "/", `{"name":"x"}`); rec.Code != 204 || got.Name != "x" {
		t.Fatalf("valid body: status %d, got %+v", rec.Code, got)
	}
	bad := map[string]string{
		"over 8 KiB":    `{"name":"` + strings.Repeat("a", 8<<10) + `"}`,
		"unknown field": `{"name":"x","admin":true}`,
		"trailing data": `{"name":"x"} {}`,
		"wrong shape":   `["x"]`,
		"wrong type":    `{"name":1}`,
		"not JSON":      `name=x`,
		"empty":         ``,
	}
	for name, b := range bad {
		rec := do(h, http.MethodPost, "/", b)
		wantError(t, rec, http.StatusBadRequest, "invalid_request")
		if name == "over 8 KiB" && strings.Contains(rec.Body.String(), "aaaa") {
			t.Errorf("error echoes the body")
		}
	}
}

func TestDecodeJSONLimit(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	const limit = 64
	handler := func(limit int64) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var got body
			if decodeJSON(w, r, &got, limit) {
				w.WriteHeader(http.StatusNoContent)
			}
		})
	}
	// pad returns a valid body of exactly n bytes.
	pad := func(n int) string { return `{"name":"` + strings.Repeat("a", n-len(`{"name":""}`)) + `"}` }

	if rec := do(handler(limit), http.MethodPost, "/", pad(limit)); rec.Code != http.StatusNoContent {
		t.Fatalf("body of exactly the limit: status %d %s", rec.Code, rec.Body)
	}
	rec := do(handler(limit), http.MethodPost, "/", pad(limit+1))
	wantError(t, rec, http.StatusBadRequest, "invalid_request")
	if strings.Contains(rec.Body.String(), "KiB") || strings.ContainsAny(rec.Body.String(), "0123456789") {
		t.Errorf("error message names a size: %s", rec.Body)
	}
	wantError(t, do(handler(maxBody), http.MethodPost, "/", pad(9<<10)), http.StatusBadRequest, "invalid_request")
	if maxBeerBody != 16<<10 {
		t.Errorf("maxBeerBody = %d, want 16 KiB", maxBeerBody)
	}
}

func TestWriteError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusConflict, "setup_done", "Setup is already complete.")
	})
	rec := do(h, http.MethodGet, "/", "")
	if rec.Code != 409 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d, Content-Type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var e map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if len(e) != 2 || e["error"] != "setup_done" || e["message"] != "Setup is already complete." {
		t.Fatalf("body = %s", rec.Body)
	}
}
