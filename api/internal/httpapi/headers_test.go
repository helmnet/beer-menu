package httpapi

import (
	"net/http"
	"testing"
)

func TestAPISecurityHeaders(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
	}
	cases := []struct {
		method, path string
		noStore      bool
	}{
		{http.MethodGet, "/api/health", false},
		{http.MethodPost, "/api/health", false}, // 405 from the mux
		{http.MethodGet, "/api/setup", true},
		{http.MethodGet, "/api/session", true}, // 401 from the guard
		{http.MethodGet, "/api/nothing", true}, // 404
	}
	for _, c := range cases {
		rec := do(mux, c.method, c.path, "")
		for k, v := range want {
			if got := rec.Header().Values(k); len(got) != 1 || got[0] != v {
				t.Errorf("%s %s: %s = %q, want exactly [%q]", c.method, c.path, k, got, v)
			}
		}
		cc := rec.Header().Values("Cache-Control")
		if c.noStore && (len(cc) != 1 || cc[0] != "no-store") {
			t.Errorf("%s %s: Cache-Control = %q, want [no-store]", c.method, c.path, cc)
		}
		if !c.noStore && len(cc) != 0 {
			t.Errorf("%s %s: Cache-Control = %q, want none", c.method, c.path, cc)
		}
	}
}

func TestCrossOriginWritesRefused(t *testing.T) {
	var reached bool
	h := withSecurity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	cases := []struct {
		name    string
		headers []string
		want    int
	}{
		{"cross-site fetch metadata", []string{"Sec-Fetch-Site", "cross-site"}, 403},
		{"foreign Origin", []string{"Origin", "http://evil.example"}, 403},
		{"same-origin fetch metadata", []string{"Sec-Fetch-Site", "same-origin"}, 204},
		{"matching Origin", []string{"Origin", "http://localhost:8080"}, 204},
		{"no browser headers", nil, 204},
	}
	for _, c := range cases {
		reached = false
		// The absolute URL sets the request Host to localhost:8080.
		rec := do(h, http.MethodDelete, "http://localhost:8080/api/session", "", c.headers...)
		if rec.Code != c.want || reached != (c.want == 204) {
			t.Errorf("%s: status %d, reached %v; want %d", c.name, rec.Code, reached, c.want)
		}
		if c.want == 403 {
			wantError(t, rec, 403, "forbidden")
			if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("%s: refused response lacks security headers", c.name)
			}
		}
	}
	// Safe methods are never refused, so links from other sites still load.
	reached = false
	if rec := do(h, http.MethodGet, "/api/session", "", "Sec-Fetch-Site", "cross-site"); rec.Code != 204 || !reached {
		t.Errorf("cross-site GET: status %d, want 204", rec.Code)
	}
}
