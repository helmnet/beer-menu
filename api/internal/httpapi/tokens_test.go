package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// sessionFor starts a session for adminID and returns its Cookie header pair.
func sessionFor(t *testing.T, deps Deps, adminID int64) []string {
	t.Helper()
	id, err := auth.CreateSession(deps.DB, adminID, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	return []string{"Cookie", sessionCookieName + "=" + id}
}

func TestTokens(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	cookie := sessionFor(t, deps, insertUser(t, deps.DB, "owner"))

	rec := do(mux, http.MethodPost, "/api/tokens", `{"name":"script","scope":"read"}`, cookie...)
	if rec.Code != 201 {
		t.Fatalf("POST /api/tokens = %d %s", rec.Code, rec.Body)
	}
	var created struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Scope  string `json:"scope"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.Name != "script" || created.Scope != "read" || !strings.HasPrefix(created.Secret, "bmt_") {
		t.Fatalf("created body = %s (%v)", rec.Body, err)
	}
	for _, key := range []string{`"created_at":`, `"last_used_at":null`, `"revoked_at":null`} {
		if !strings.Contains(rec.Body.String(), key) {
			t.Errorf("created body lacks %s: %s", key, rec.Body)
		}
	}

	list := do(mux, http.MethodGet, "/api/tokens", "", cookie...)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"name":"script"`) {
		t.Fatalf("GET /api/tokens = %d %s", list.Code, list.Body)
	}
	if strings.Contains(list.Body.String(), created.Secret) || strings.Contains(list.Body.String(), "secret") || strings.Contains(list.Body.String(), "token_hash") {
		t.Fatalf("list exposes the secret: %s", list.Body)
	}

	bearer := []string{"Authorization", "Bearer " + created.Secret}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", bearer...), 403, "forbidden")
	path := "/api/tokens/" + strconv.FormatInt(created.ID, 10)
	if rec := do(mux, http.MethodDelete, path, "", cookie...); rec.Code != 204 {
		t.Fatalf("DELETE %s = %d %s", path, rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", bearer...), 401, "unauthenticated")
	if rec := do(mux, http.MethodDelete, path, "", cookie...); rec.Code != 204 {
		t.Fatalf("second DELETE = %d, want 204", rec.Code)
	}
	if list := do(mux, http.MethodGet, "/api/tokens", "", cookie...); strings.Contains(list.Body.String(), `"revoked_at":null`) {
		t.Fatalf("revoked token not marked: %s", list.Body)
	}

	for _, p := range []string{"/api/tokens/999", "/api/tokens/0", "/api/tokens/-1", "/api/tokens/x"} {
		wantError(t, do(mux, http.MethodDelete, p, "", cookie...), 404, "not_found")
	}
	for _, body := range []string{`{"name":"","scope":"read"}`, `{"name":"x","scope":"admin"}`, `{"name":"x"}`, `{"name":"x","scope":"read","extra":1}`} {
		wantError(t, do(mux, http.MethodPost, "/api/tokens", body, cookie...), 400, "invalid_request")
	}

	out := logs.String()
	if !strings.Contains(out, "token created") || !strings.Contains(out, "token revoked") || !strings.Contains(out, "token_name=script") {
		t.Errorf("log lacks token events: %s", out)
	}
	if strings.Contains(out, created.Secret) {
		t.Fatal("log contains the token secret")
	}
}

func TestTokenRoutesRefuseTokens(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	_, id := insertToken(t, deps.DB, auth.ScopeRead, false)
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		bearer := []string{"Authorization", "Bearer " + secret}
		wantError(t, do(mux, http.MethodGet, "/api/tokens", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/tokens", `{"name":"x","scope":"write"}`, bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodDelete, "/api/tokens/"+strconv.FormatInt(id, 10), "", bearer...), 403, "forbidden")
	}
}

func TestCreateTokenNeedsConfirm(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	owner := signInAs(t, mux, "owner", pw)
	// The sign-in no longer counts as a recent confirm.
	clock.Advance(auth.ConfirmWindow + time.Second)

	wantError(t, do(mux, http.MethodPost, "/api/tokens", `{"name":"script","scope":"write"}`, owner...), 403, "confirm_required")
	// The confirm check runs before the body is read.
	wantError(t, do(mux, http.MethodPost, "/api/tokens", `{"name":"","scope":"admin"}`, owner...), 403, "confirm_required")
	if tokens, err := auth.ListTokens(deps.DB); err != nil || len(tokens) != 0 {
		t.Fatalf("ListTokens = %v, %v; want none created", tokens, err)
	}

	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw}), owner...); rec.Code != 204 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodPost, "/api/tokens", `{"name":"script","scope":"write"}`, owner...); rec.Code != 201 {
		t.Fatalf("create after confirm = %d %s", rec.Code, rec.Body)
	}
}
