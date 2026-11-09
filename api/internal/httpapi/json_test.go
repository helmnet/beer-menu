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
		if !decodeJSON(w, r, &got) {
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
