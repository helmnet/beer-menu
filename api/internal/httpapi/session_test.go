package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

// createAdmin makes the first admin "owner" with a random password and returns the password.
func createAdmin(t *testing.T, deps Deps) string {
	t.Helper()
	pw := testPassword()
	if _, err := auth.CreateFirstAdmin(deps.DB, "owner", pw, deps.Now()); err != nil {
		t.Fatal(err)
	}
	return pw
}

// postSession posts credentials to /api/session and returns the response.
func postSession(h http.Handler, username, password string, headers ...string) *httptest.ResponseRecorder {
	return do(h, http.MethodPost, "/api/session", credentials(username, password), headers...)
}

// captureLog sends slog output to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func cookieHeader(c *http.Cookie) []string { return []string{"Cookie", c.Name + "=" + c.Value} }

func TestSignInAndOut(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)

	planted := "planted-session-value"
	rec := postSession(mux, "OWNER", pw, "Cookie", sessionCookieName+"="+planted)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"username":"owner"`) {
		t.Fatalf("sign-in = %d %s", rec.Code, rec.Body)
	}
	c := sessionCookie(rec)
	if c == nil || c.Value == planted || len(c.Value) != 43 {
		t.Fatalf("cookie = %+v, want a fresh session ID", c)
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || !c.Secure {
		t.Fatalf("cookie attributes = %+v", c)
	}
	if !c.Expires.Equal(t0.Add(30 * 24 * time.Hour)) {
		t.Fatalf("cookie expires %v, want 30 days", c.Expires)
	}
	if got := do(mux, http.MethodGet, "/api/session", "", cookieHeader(c)...); got.Code != 200 || !strings.Contains(got.Body.String(), `"username":"owner"`) {
		t.Fatalf("GET /api/session = %d %s", got.Code, got.Body)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", ""), 401, "unauthenticated")

	out := do(mux, http.MethodDelete, "/api/session", "", cookieHeader(c)...)
	if out.Code != 204 {
		t.Fatalf("sign-out = %d %s", out.Code, out.Body)
	}
	if cleared := sessionCookie(out); cleared == nil || cleared.MaxAge >= 0 || cleared.Value != "" {
		t.Fatalf("sign-out cookie = %+v, want cleared", cleared)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", cookieHeader(c)...), 401, "unauthenticated")

	again := sessionCookie(postSession(mux, "owner", pw))
	clock.Advance(7 * 24 * time.Hour)
	wantError(t, do(mux, http.MethodGet, "/api/session", "", cookieHeader(again)...), 401, "unauthenticated")

	if !strings.Contains(logs.String(), "admin signed in") || !strings.Contains(logs.String(), "username=owner") {
		t.Errorf("log lacks sign-in event: %s", logs)
	}
	for _, secret := range []string{pw, c.Value, again.Value} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("log contains a secret")
		}
	}
}

func TestSignInCookieNotSecureWhenDisabled(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.CookieSecure = false
	pw := createAdmin(t, deps)
	c := sessionCookie(postSession(NewMux(deps), "owner", pw))
	if c == nil || c.Secure {
		t.Fatalf("cookie = %+v, want not Secure", c)
	}
}

func TestSignInFailuresLookAlike(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	createAdmin(t, deps)
	mux := NewMux(deps)

	before := auth.VerifyCount()
	wrong := postSession(mux, "owner", testPassword())
	mid := auth.VerifyCount()
	unknown := postSession(mux, "nobody", testPassword())
	after := auth.VerifyCount()

	wantError(t, wrong, 401, "invalid_credentials")
	if wrong.Body.String() != unknown.Body.String() || wrong.Code != unknown.Code {
		t.Fatalf("wrong password %d %s vs unknown user %d %s", wrong.Code, wrong.Body, unknown.Code, unknown.Body)
	}
	if mid-before != 1 || after-mid != 1 {
		t.Fatalf("verifies: wrong password %d, unknown user %d; want 1 each", mid-before, after-mid)
	}
	if sessionCookie(wrong) != nil || sessionCookie(unknown) != nil {
		t.Fatal("failed sign-in set a cookie")
	}
	if !strings.Contains(logs.String(), "sign-in failed") {
		t.Errorf("log lacks sign-in failed: %s", logs)
	}
	wantError(t, postSession(mux, "owner", strings.Repeat("x", 257)), 400, "invalid_request")
}

func TestSessionRoutesNeedSession(t *testing.T) {
	deps, _ := newTestDeps(t)
	createAdmin(t, deps)
	mux := NewMux(deps)
	secret, _ := insertToken(t, deps.DB, auth.ScopeWrite, false)
	for _, m := range []string{http.MethodGet, http.MethodDelete} {
		wantError(t, do(mux, m, "/api/session", "", "Authorization", "Bearer "+secret), 403, "forbidden")
	}
}

func TestSessionCSRF(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)
	pw := testPassword()

	setup := do(mux, http.MethodPost, "/api/setup", credentials("owner", pw), "Sec-Fetch-Site", "cross-site")
	wantError(t, setup, 403, "forbidden")
	if n, _ := auth.AdminCount(deps.DB); n != 0 {
		t.Fatal("cross-site setup created an admin")
	}
	if _, err := auth.CreateFirstAdmin(deps.DB, "owner", pw, deps.Now()); err != nil {
		t.Fatal(err)
	}

	for range 6 {
		rec := postSession(mux, "owner", pw, "Sec-Fetch-Site", "cross-site")
		wantError(t, rec, 403, "forbidden")
		if sessionCookie(rec) != nil {
			t.Fatal("cross-site sign-in set a cookie")
		}
	}
	if _, ok := deps.Limiter.Check("owner"); !ok {
		t.Fatal("cross-site sign-ins counted toward the rate limit")
	}
	deps.Limiter.Done("owner", false)

	c := sessionCookie(postSession(mux, "owner", pw, "Sec-Fetch-Site", "same-origin"))
	if c == nil {
		t.Fatal("same-origin sign-in failed")
	}
	out := do(mux, http.MethodDelete, "/api/session", "", append(cookieHeader(c), "Sec-Fetch-Site", "cross-site")...)
	wantError(t, out, 403, "forbidden")
	if got := do(mux, http.MethodGet, "/api/session", "", cookieHeader(c)...); got.Code != 200 {
		t.Fatalf("session after refused cross-site sign-out = %d", got.Code)
	}
}

func TestSignInRateLimited(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	spoof := func(i int) []string {
		ip := "spoofed-client-" + strconv.Itoa(i) // any value: the API must ignore these headers
		return []string{"X-Forwarded-For", ip, "X-Real-IP", ip}
	}
	for i := range 5 {
		wantError(t, postSession(mux, "owner", testPassword(), spoof(i)...), 401, "invalid_credentials")
	}
	before := auth.VerifyCount()
	for i, user := range []string{"owner", "OWNER"} {
		rec := postSession(mux, user, pw, spoof(10+i)...)
		wantError(t, rec, 429, "too_many_attempts")
		if _, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil {
			t.Fatalf("Retry-After = %q, want integer seconds", rec.Header().Get("Retry-After"))
		}
		if sessionCookie(rec) != nil {
			t.Fatal("limited sign-in set a cookie")
		}
	}
	if got := auth.VerifyCount() - before; got != 0 {
		t.Fatalf("limited attempts ran %d password verifies, want 0", got)
	}
	if !strings.Contains(logs.String(), "sign-in rate limited") || !strings.Contains(logs.String(), "username=owner") {
		t.Errorf("log lacks rate-limit event: %s", logs)
	}
	clock.Advance(15 * time.Minute)
	if rec := postSession(mux, "owner", pw); rec.Code != 200 {
		t.Fatalf("sign-in after the window = %d %s", rec.Code, rec.Body)
	}
}

func TestSignInRetryAfterRoundsUp(t *testing.T) {
	deps, clock := newTestDeps(t)
	createAdmin(t, deps)
	mux := NewMux(deps)
	for range 5 {
		postSession(mux, "owner", testPassword())
	}
	clock.Advance(90*time.Second + 500*time.Millisecond)
	rec := postSession(mux, "owner", testPassword())
	// 15m - 90.5s = 809.5s, rounded up.
	if got := rec.Header().Get("Retry-After"); got != "810" {
		t.Fatalf("Retry-After = %q, want 810", got)
	}
}

// passwordChange returns a PasswordChange JSON body.
func passwordChange(current, next string) string {
	b, _ := json.Marshal(map[string]string{"current_password": current, "new_password": next})
	return string(b)
}

func TestChangePassword(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	old := createAdmin(t, deps)
	mux := NewMux(deps)
	this := signInAs(t, mux, "owner", old)
	other := signInAs(t, mux, "owner", old)
	next := testPassword()

	wantError(t, do(mux, http.MethodPost, "/api/password", passwordChange(old, "short"), this...), 400, "invalid_request")
	if rec := do(mux, http.MethodPost, "/api/password", passwordChange(old, next), this...); rec.Code != 204 {
		t.Fatalf("POST /api/password = %d %s", rec.Code, rec.Body)
	}
	wantError(t, postSession(mux, "owner", old), 401, "invalid_credentials")
	wantError(t, do(mux, http.MethodGet, "/api/session", "", other...), 401, "unauthenticated")
	if got := do(mux, http.MethodGet, "/api/session", "", this...); got.Code != 200 {
		t.Fatalf("changing session after change = %d", got.Code)
	}
	if !strings.Contains(logs.String(), "password changed") || !strings.Contains(logs.String(), "username=owner") {
		t.Errorf("log lacks password changed: %s", logs)
	}
	for _, pw := range []string{old, next} {
		if strings.Contains(logs.String(), pw) {
			t.Fatal("log contains a password")
		}
	}
}

func TestChangePasswordWrongCountsTowardLimit(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	this := signInAs(t, mux, "owner", pw)
	// One sign-in failure plus four wrong current passwords reach the limit of 5.
	wantError(t, postSession(mux, "owner", testPassword()), 401, "invalid_credentials")
	for range 4 {
		wantError(t, do(mux, http.MethodPost, "/api/password", passwordChange(testPassword(), testPassword()), this...), 403, "wrong_password")
	}
	wantError(t, do(mux, http.MethodPost, "/api/password", passwordChange(pw, testPassword()), this...), 429, "too_many_attempts")
	wantError(t, postSession(mux, "OWNER", pw), 429, "too_many_attempts")
	if _, err := auth.Authenticate(deps.DB, "owner", pw); err != nil {
		t.Fatalf("password changed despite refusals: %v", err)
	}
}

func TestSignInConcurrentGuessesLimited(t *testing.T) {
	deps, _ := newTestDeps(t)
	createAdmin(t, deps)
	mux := NewMux(deps)
	// All requests pass the limit check at once, before any failure is recorded, unless the
	// check holds an attempt until the result is known.
	const n = 20
	codes := make(chan int, n)
	var start sync.WaitGroup
	start.Add(1)
	for range n {
		go func() {
			start.Wait()
			codes <- postSession(mux, "owner", testPassword()).Code
		}()
	}
	start.Done()
	guesses := 0
	for range n {
		switch c := <-codes; c {
		case 401:
			guesses++
		case 429, 503:
		default:
			t.Errorf("status %d, want 401, 429 or 503", c)
		}
	}
	if guesses > 5 {
		t.Fatalf("%d concurrent wrong passwords checked, want at most 5", guesses)
	}
}
