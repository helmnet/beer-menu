package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFlowLeaksNoSecrets runs setup, sign-in, token create and use, password change and
// sign-out, then checks that no password, session ID or token secret is in the log, in any
// response body except the token's creation, or in any response header other than the
// Set-Cookie that hands out a session (SC-005).
func TestFlowLeaksNoSecrets(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	type step struct {
		name string
		rec  *httptest.ResponseRecorder
	}
	var steps []step
	call := func(name string, want int, method, path, body string, headers ...string) *httptest.ResponseRecorder {
		t.Helper()
		rec := do(mux, method, path, body, headers...)
		if rec.Code != want {
			t.Fatalf("%s = %d, want %d; body %s", name, rec.Code, want, rec.Body)
		}
		steps = append(steps, step{name, rec})
		return rec
	}

	first, second := testPassword(), testPassword()
	setupCookie := sessionCookie(call("setup", 201, http.MethodPost, "/api/setup", credentials("owner", first)))
	signInCookie := sessionCookie(call("sign-in", 200, http.MethodPost, "/api/session", credentials("owner", first)))
	cookie := cookieHeader(signInCookie)
	created := call("create token", 201, http.MethodPost, "/api/tokens", `{"name":"script","scope":"write"}`, cookie...)
	var tok struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &tok); err != nil || tok.Secret == "" {
		t.Fatalf("token body %s (%v)", created.Body, err)
	}
	call("use token", 403, http.MethodGet, "/api/admins", "", "Authorization", "Bearer "+tok.Secret)
	call("list tokens", 200, http.MethodGet, "/api/tokens", "", cookie...)
	call("wrong sign-in", 401, http.MethodPost, "/api/session", credentials("owner", second))
	call("change password", 204, http.MethodPost, "/api/password", passwordChange(first, second), cookie...)
	call("sign-out", 204, http.MethodDelete, "/api/session", "", cookie...)

	secrets := map[string]string{
		"first password": first, "second password": second, "token secret": tok.Secret,
		"setup session": setupCookie.Value, "sign-in session": signInCookie.Value,
	}
	for what, s := range secrets {
		if strings.Contains(logs.String(), s) {
			t.Errorf("log contains the %s", what)
		}
		for _, st := range steps {
			if strings.Contains(st.rec.Body.String(), s) && !(st.name == "create token" && s == tok.Secret) {
				t.Errorf("%s response body contains the %s", st.name, what)
			}
			for name, values := range st.rec.Header() {
				if name != "Set-Cookie" && strings.Contains(fmt.Sprint(values), s) {
					t.Errorf("%s response header %s contains the %s", st.name, name, what)
				}
			}
		}
	}
}
