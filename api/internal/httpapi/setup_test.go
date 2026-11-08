package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// credentials returns a Credentials JSON body.
func credentials(username, password string) string {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return string(b)
}

func TestSetup(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)

	if rec := do(mux, http.MethodGet, "/api/setup", ""); rec.Code != 200 || rec.Body.String() != `{"needed":true}` {
		t.Fatalf("GET /api/setup = %d %s", rec.Code, rec.Body)
	}
	pw := testPassword()
	wantError(t, do(mux, http.MethodPost, "/api/setup", credentials("owner", strings.Repeat("x", 11))), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/setup", credentials("bad name", pw)), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/setup", `{"username":"owner"}`), 400, "invalid_request")

	rec := do(mux, http.MethodPost, "/api/setup", credentials("owner", pw))
	if rec.Code != 201 {
		t.Fatalf("POST /api/setup = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), pw) {
		t.Fatal("response contains the password")
	}
	var admin struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &admin); err != nil || admin.Username != "owner" || admin.CreatedAt != "2026-11-08T12:00:00Z" {
		t.Fatalf("admin body = %s (%v)", rec.Body, err)
	}
	c := sessionCookie(rec)
	if c == nil {
		t.Fatal("no session cookie")
	}
	got := do(mux, http.MethodGet, "/api/session", "", "Cookie", c.Name+"="+c.Value)
	if got.Code != 200 || !strings.Contains(got.Body.String(), `"username":"owner"`) {
		t.Fatalf("GET /api/session with setup cookie = %d %s", got.Code, got.Body)
	}

	if rec := do(mux, http.MethodGet, "/api/setup", ""); rec.Body.String() != `{"needed":false}` {
		t.Fatalf("GET /api/setup after setup = %s", rec.Body)
	}
	again := do(mux, http.MethodPost, "/api/setup", credentials("intruder", testPassword()))
	wantError(t, again, 409, "setup_done")
	if sessionCookie(again) != nil {
		t.Fatal("refused setup set a cookie")
	}
	if n, _ := auth.AdminCount(deps.DB); n != 1 {
		t.Fatalf("AdminCount = %d after refused setup", n)
	}
}
