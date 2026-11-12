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
	"github.com/helmnet/beer-menu/api/internal/auth/authtest"
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
	clock.Advance(time.Hour)
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

// pendingCookie returns the bm_pending cookie set by rec, or nil.
func pendingCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == pendingCookieName {
			return c
		}
	}
	return nil
}

func TestSignInWithoutTwoFactorUnchanged(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	rec := postSession(NewMux(deps), "owner", pw)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"username":"owner"`) || !strings.Contains(rec.Body.String(), `"two_factor":false`) {
		t.Fatalf("sign-in = %d %s", rec.Code, rec.Body)
	}
	if sessionCookie(rec) == nil || pendingCookie(rec) != nil {
		t.Fatal("want a session cookie and no bm_pending")
	}
}

func TestSignInSecondFactorPending(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	addTotp(t, deps, uid)

	post := func(d Deps) *httptest.ResponseRecorder {
		t.Helper()
		rec := postSession(NewMux(d), "owner", pw)
		if rec.Code != 202 {
			t.Fatalf("sign-in = %d %s, want 202", rec.Code, rec.Body)
		}
		if sessionCookie(rec) != nil {
			t.Fatal("202 set a session cookie")
		}
		return rec
	}
	rec := post(deps)
	if got := rec.Body.String(); got != `{"next":"second_factor","methods":["totp"]}` {
		t.Fatalf("body = %s", got)
	}
	c := pendingCookie(rec)
	if c == nil || len(c.Value) != 43 || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/api" || !c.Secure || c.MaxAge != 300 {
		t.Fatalf("bm_pending = %+v", c)
	}
	if _, ok, err := auth.PeekChallenge(deps.DB, c.Value, auth.KindSecondFactor, deps.Now()); !ok || err != nil {
		t.Fatalf("no second_factor challenge for the cookie: %v", err)
	}
	if !strings.Contains(logs.String(), "sign-in password ok, second factor owed") || !strings.Contains(logs.String(), "username=owner") {
		t.Errorf("log lacks the pending event: %s", logs)
	}
	if strings.Contains(logs.String(), c.Value) || strings.Contains(logs.String(), pw) {
		t.Fatal("log contains a secret")
	}

	if _, err := auth.NewRecoveryCodes(deps.DB, uid); err != nil {
		t.Fatal(err)
	}
	addPasskey(t, deps, uid)
	// Passkeys are listed only when the server can check them, the authenticator only with a key.
	if got := post(deps).Body.String(); got != `{"next":"second_factor","methods":["totp","recovery"]}` {
		t.Fatalf("without WebAuthn = %s", got)
	}
	withKeys := deps
	withKeys.WebAuthn = testWebAuthn(t)
	if got := post(withKeys).Body.String(); got != `{"next":"second_factor","methods":["totp","passkey","recovery"],"passkeys_origin":"https://taplist.example.org"}` {
		t.Fatalf("with WebAuthn = %s", got)
	}
	noBox := deps
	noBox.Box = nil
	if got := post(noBox).Body.String(); got != `{"next":"second_factor","methods":["recovery"]}` {
		t.Fatalf("without a key = %s", got)
	}

	insecure := deps
	insecure.CookieSecure = false
	if c := pendingCookie(post(insecure)); c == nil || c.Secure {
		t.Fatalf("bm_pending = %+v, want not Secure", c)
	}
	wantError(t, postSession(NewMux(deps), "owner", pw, "Sec-Fetch-Site", "cross-site"), 403, "forbidden")
}

func TestSignInEnrollPending(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	if err := auth.SetRequireTwoFactor(deps.DB, true, t0); err != nil {
		t.Fatal(err)
	}
	rec := postSession(NewMux(deps), "owner", pw)
	if rec.Code != 202 || rec.Body.String() != `{"next":"enroll"}` || sessionCookie(rec) != nil {
		t.Fatalf("sign-in = %d %s", rec.Code, rec.Body)
	}
	c := pendingCookie(rec)
	if c == nil {
		t.Fatal("no bm_pending")
	}
	if _, ok, _ := auth.PeekChallenge(deps.DB, c.Value, auth.KindEnroll, deps.Now()); !ok {
		t.Fatal("no enroll challenge for the cookie")
	}
	// With a factor, the second step wins over enrolling.
	addTotp(t, deps, adminID(t, deps, "owner"))
	if rec := postSession(NewMux(deps), "owner", pw); rec.Code != 202 || !strings.Contains(rec.Body.String(), `"next":"second_factor"`) {
		t.Fatalf("with a factor = %d %s", rec.Code, rec.Body)
	}
}

func TestSignInWrongPasswordSameWithTwoFactor(t *testing.T) {
	deps, _ := newTestDeps(t)
	createAdmin(t, deps)
	if _, err := auth.AddAdmin(deps.DB, "second", testPassword(), t0); err != nil {
		t.Fatal(err)
	}
	addTotp(t, deps, adminID(t, deps, "second"))
	mux := NewMux(deps)
	before := auth.VerifyCount()
	plain := postSession(mux, "owner", testPassword())
	mid := auth.VerifyCount()
	twoFA := postSession(mux, "second", testPassword())
	after := auth.VerifyCount()
	wantError(t, twoFA, 401, "invalid_credentials")
	if plain.Code != twoFA.Code || plain.Body.String() != twoFA.Body.String() {
		t.Fatalf("without 2FA %d %s vs with %d %s", plain.Code, plain.Body, twoFA.Code, twoFA.Body)
	}
	if mid-before != after-mid {
		t.Fatalf("verifies %d vs %d", mid-before, after-mid)
	}
	if pendingCookie(twoFA) != nil {
		t.Fatal("wrong password set bm_pending")
	}
}

func TestCancelSignIn(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	cancel := func(headers ...string) *httptest.ResponseRecorder {
		t.Helper()
		rec := do(mux, http.MethodDelete, "/api/session/pending", "", headers...)
		if rec.Code != 204 {
			t.Fatalf("cancel = %d %s", rec.Code, rec.Body)
		}
		if c := pendingCookie(rec); c == nil || c.MaxAge >= 0 || c.Value != "" || c.Path != "/api" {
			t.Fatalf("cancel cookie = %+v, want cleared", c)
		}
		return rec
	}

	for _, kind := range []auth.ChallengeKind{auth.KindSecondFactor, auth.KindEnroll} {
		id, err := auth.CreateChallenge(deps.DB, uid, kind, deps.Now())
		if err != nil {
			t.Fatal(err)
		}
		cookie := []string{"Cookie", pendingCookieName + "=" + id}
		wantError(t, do(mux, http.MethodDelete, "/api/session/pending", "", append(cookie, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
		if _, ok, _ := auth.PeekChallenge(deps.DB, id, kind, deps.Now()); !ok {
			t.Fatalf("cross-site cancel deleted the %s pending", kind)
		}
		cancel(cookie...)
		if _, ok, _ := auth.PeekChallenge(deps.DB, id, kind, deps.Now()); ok {
			t.Fatalf("%s pending kept after cancel", kind)
		}
		wantError(t, do(mux, http.MethodGet, "/api/2fa", "", cookie...), 401, "unauthenticated")
	}
	cancel()
	cancel("Cookie", pendingCookieName+"=unknown")

	session := signInAs(t, mux, "owner", pw)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	if err := auth.SetupChallenge(deps.DB, sid, uid, deps.Now()); err != nil {
		t.Fatal(err)
	}
	// A session ID sent as bm_pending must not reach the session's setup row.
	cancel(append(session, "Cookie", pendingCookieName+"="+sid)...)
	if got := do(mux, http.MethodGet, "/api/session", "", session...); got.Code != 200 {
		t.Fatalf("session after cancel = %d", got.Code)
	}
	if _, ok, _ := auth.PeekChallenge(deps.DB, sid, auth.KindSetup, deps.Now()); !ok {
		t.Fatal("cancel deleted a setup row")
	}
}

// totpPending gives "owner" an authenticator and posts the password, returning the secret and
// the bm_pending cookie header of the 202.
func totpPending(t *testing.T, deps Deps, mux http.Handler, pw string) ([]byte, []string) {
	t.Helper()
	secret := addTotp(t, deps, adminID(t, deps, "owner"))
	return secret, passwordStep(t, mux, "owner", pw)
}

// passwordStep posts username's password, wanting 202, and returns the bm_pending cookie header.
func passwordStep(t *testing.T, mux http.Handler, username, pw string) []string {
	t.Helper()
	rec := postSession(mux, username, pw)
	c := pendingCookie(rec)
	if rec.Code != 202 || c == nil {
		t.Fatalf("password step = %d %s", rec.Code, rec.Body)
	}
	return cookieHeader(c)
}

func TestSignInTotp(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	secret, pending := totpPending(t, deps, mux, pw)
	post := func(code string, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code+`"}`, headers...)
	}
	code := auth.Code(secret, clock.Now().Unix()/30)

	wantError(t, post(code), 401, "unauthenticated")
	wantError(t, post(code, "Cookie", pendingCookieName+"=unknown"), 401, "unauthenticated")
	wantError(t, post(code, append(pending, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":""}`, pending...), 400, "invalid_request")
	wrong := "000000"
	if wrong == code {
		wrong = "000001"
	}
	rec := post(wrong, pending...)
	wantError(t, rec, 401, "invalid_code")
	if !strings.Contains(rec.Body.String(), "That code did not work.") || sessionCookie(rec) != nil {
		t.Fatalf("wrong code = %s", rec.Body)
	}
	rec = post(code, pending...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"username":"owner"`) || !strings.Contains(rec.Body.String(), `"two_factor":true`) {
		t.Fatalf("valid code = %d %s", rec.Code, rec.Body)
	}
	session := sessionCookie(rec)
	if session == nil || len(session.Value) != 43 {
		t.Fatal("no session cookie")
	}
	if c := pendingCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("bm_pending = %+v, want cleared", c)
	}
	if got := do(mux, http.MethodGet, "/api/session", "", cookieHeader(session)...); got.Code != 200 {
		t.Fatalf("new session = %d", got.Code)
	}
	clock.Advance(30 * time.Second)
	wantError(t, post(auth.Code(secret, clock.Now().Unix()/30), pending...), 401, "unauthenticated")
	out := logs.String()
	if !strings.Contains(out, "admin signed in") || !strings.Contains(out, "sign-in failed, wrong code") {
		t.Errorf("log lacks the events: %s", out)
	}
	for _, leak := range []string{code, wrong, pending[1], session.Value} {
		if strings.Contains(out, leak) {
			t.Fatalf("log contains a secret: %s", out)
		}
	}
}

func TestSignInTotpReusedCode(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	secret, pending := totpPending(t, deps, mux, pw)
	code := auth.Code(secret, clock.Now().Unix()/30)
	if rec := do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code+`"}`, pending...); rec.Code != 200 {
		t.Fatalf("first use = %d %s", rec.Code, rec.Body)
	}
	again := passwordStep(t, mux, "owner", pw)
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code+`"}`, again...), 401, "invalid_code")
}

func TestSignInTotpPendingExpiresAndLimits(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	secret, pending := totpPending(t, deps, mux, pw)
	code := func() string { return auth.Code(secret, clock.Now().Unix()/30) }
	// Four wrong codes plus the wrong password make five failures for the username.
	wantError(t, postSession(mux, "owner", testPassword()), 401, "invalid_credentials")
	for range 4 {
		wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"12345"}`, pending...), 401, "invalid_code")
	}
	rec := do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code()+`"}`, pending...)
	wantError(t, rec, 429, "too_many_attempts")
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
	// The pending outlives the limit only until it expires.
	clock.Advance(auth.ChallengeTTL)
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code()+`"}`, pending...), 401, "unauthenticated")
}

func TestSignInTotpWrongCodeKeepsPending(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	secret, pending := totpPending(t, deps, mux, pw)
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"12345"}`, pending...), 401, "invalid_code")
	clock.Advance(4 * time.Minute)
	if rec := do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+auth.Code(secret, clock.Now().Unix()/30)+`"}`, pending...); rec.Code != 200 {
		t.Fatalf("valid code after a wrong one = %d %s", rec.Code, rec.Body)
	}
}

func TestSignInTotpRefusesOtherPendings(t *testing.T) {
	deps, clock := newTestDeps(t)
	createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	secret := addTotp(t, deps, uid)
	code := auth.Code(secret, clock.Now().Unix()/30)
	enroll, err := auth.CreateChallenge(deps.DB, uid, auth.KindEnroll, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code+`"}`, "Cookie", pendingCookieName+"="+enroll), 401, "unauthenticated")
	// A session ID sent as bm_pending is not a pending sign-in.
	session := sessionFor(t, deps, uid)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+code+`"}`, "Cookie", pendingCookieName+"="+sid), 401, "unauthenticated")
}

func TestSignInTotpFactorsGoneMidStep(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	secret, pending := totpPending(t, deps, mux, pw)
	uid := adminID(t, deps, "owner")
	if err := auth.ClearTwoFactor(deps.DB, uid); err != nil {
		t.Fatal(err)
	}
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+auth.Code(secret, clock.Now().Unix()/30)+`"}`, pending...), 401, "unauthenticated")

	// An admin removed mid-step: the pending goes with them.
	if _, err := auth.AddAdmin(deps.DB, "second", pw, t0); err != nil {
		t.Fatal(err)
	}
	second := adminID(t, deps, "second")
	secret = addTotp(t, deps, second)
	second2 := passwordStep(t, mux, "second", pw)
	if _, err := auth.RemoveAdmin(deps.DB, second); err != nil {
		t.Fatal(err)
	}
	clock.Advance(30 * time.Second)
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"`+auth.Code(secret, clock.Now().Unix()/30)+`"}`, second2...), 401, "unauthenticated")
}

// passkeyAdmin makes "owner" with WebAuthn on and passkey a registered through the API, and
// returns the mux, the password and a.
func passkeyAdmin(t *testing.T) (Deps, *testClock, http.Handler, string, *authtest.Authenticator) {
	t.Helper()
	deps, clock := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	a := newAuthenticator(t)
	if rec := registerPasskeyHTTP(t, mux, a, "Phone", signInAs(t, mux, "owner", pw)...); rec.Code != 201 {
		t.Fatalf("register = %d %s", rec.Code, rec.Body)
	}
	return deps, clock, mux, pw, a
}

// passkeyAssertion gets sign-in request options with pending and returns who's assertion.
func passkeyAssertion(t *testing.T, mux http.Handler, who *authtest.Authenticator, pending []string) []byte {
	t.Helper()
	return who.Assert(t, passkeyOptions(t, mux, "/api/session/passkey/options", pending...))
}

func TestSignInPasskey(t *testing.T) {
	logs := captureLog(t)
	deps, _, mux, pw, a := passkeyAdmin(t)
	if _, err := auth.AddAdmin(deps.DB, "second", testPassword(), t0); err != nil {
		t.Fatal(err)
	}
	theirs := newAuthenticator(t)
	if rec := registerPasskeyHTTP(t, mux, theirs, "Theirs", sessionFor(t, deps, adminID(t, deps, "second"))...); rec.Code != 201 {
		t.Fatalf("second admin's passkey = %d %s", rec.Code, rec.Body)
	}
	rec := postSession(mux, "owner", pw)
	if rec.Code != 202 || !strings.Contains(rec.Body.String(), `"passkey"`) || !strings.Contains(rec.Body.String(), `"passkeys_origin":"https://taplist.example.org"`) {
		t.Fatalf("password step = %d %s", rec.Code, rec.Body)
	}
	pending := cookieHeader(pendingCookie(rec))

	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", ""), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", "Cookie", pendingCookieName+"=unknown"), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", append(pending, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	body := passkeyOptions(t, mux, "/api/session/passkey/options", pending...)
	opts := optionsOf(t, body)
	allow, _ := opts["allowCredentials"].([]any)
	if opts["userVerification"] != "required" || len(allow) != 1 || allow[0].(map[string]any)["id"] != b64(a.CredentialID) {
		t.Fatalf("request options = %v, want only this admin's passkey", opts)
	}

	post := func(body []byte, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/session/passkey", string(body), headers...)
	}
	resp := a.Assert(t, body)
	wantError(t, post(resp), 401, "unauthenticated")
	wantError(t, post(resp, append(pending, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	wantError(t, post([]byte(`{"id":"`+strings.Repeat("a", 16<<10)+`"}`), pending...), 400, "invalid_request")
	rec = post(passkeyAssertion(t, mux, theirs, pending), pending...)
	wantError(t, rec, 401, "passkey_failed")
	if sessionCookie(rec) != nil {
		t.Fatal("refused passkey set a session cookie")
	}
	resp = passkeyAssertion(t, mux, a, pending)
	rec = post(resp, pending...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"username":"owner"`) || !strings.Contains(rec.Body.String(), `"two_factor":true`) {
		t.Fatalf("passkey sign-in = %d %s", rec.Code, rec.Body)
	}
	session := sessionCookie(rec)
	if session == nil {
		t.Fatal("no session cookie")
	}
	if c := pendingCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("bm_pending = %+v, want cleared", c)
	}
	if got := do(mux, http.MethodGet, "/api/session", "", cookieHeader(session)...); got.Code != 200 {
		t.Fatalf("new session = %d", got.Code)
	}
	wantError(t, post(resp, pending...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", pending...), 401, "unauthenticated")

	out := logs.String()
	if !strings.Contains(out, "second_factor=passkey") || !strings.Contains(out, "sign-in failed, passkey refused") {
		t.Errorf("log lacks the events: %s", out)
	}
	for _, leak := range []string{b64(a.CredentialID), pending[1], session.Value} {
		if strings.Contains(out, leak) {
			t.Fatalf("log contains a secret: %s", out)
		}
	}
}

func TestSignInPasskeyLimits(t *testing.T) {
	_, clock, mux, pw, a := passkeyAdmin(t)
	pending := passwordStep(t, mux, "owner", pw)
	wrong := *a
	wrong.Origin = "https://192.168.1.10"
	for range 5 {
		wantError(t, do(mux, http.MethodPost, "/api/session/passkey", string(passkeyAssertion(t, mux, &wrong, pending)), pending...), 401, "passkey_failed")
	}
	rec := do(mux, http.MethodPost, "/api/session/passkey", string(passkeyAssertion(t, mux, a, pending)), pending...)
	wantError(t, rec, 429, "too_many_attempts")
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
	// The pending outlives the limit only until it expires.
	clock.Advance(auth.ChallengeTTL)
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", pending...), 401, "unauthenticated")
}

func TestSignInPasskeyRefusesOtherPendings(t *testing.T) {
	deps, _, mux, _, a := passkeyAdmin(t)
	uid := adminID(t, deps, "owner")
	enroll, err := auth.CreateChallenge(deps.DB, uid, auth.KindEnroll, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	cookie := []string{"Cookie", pendingCookieName + "=" + enroll}
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", cookie...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey", string(a.Assert(t, []byte(`{"publicKey":{"challenge":"AAAAAAAAAAAAAAAAAAAAAA"}}`))), cookie...), 401, "unauthenticated")
}

func TestSignInPasskeyUnavailable(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	addTotp(t, deps, uid)
	addPasskey(t, deps, uid)
	mux := NewMux(deps)
	pending := passwordStep(t, mux, "owner", pw)
	wantError(t, do(mux, http.MethodPost, "/api/session/passkey/options", "", pending...), 409, "passkeys_unavailable")
}

// recoveryPending gives "owner" an authenticator and recovery codes, posts the password and
// returns the codes and the bm_pending cookie header of the 202.
func recoveryPending(t *testing.T, deps Deps, mux http.Handler, pw string) ([]string, []string) {
	t.Helper()
	uid := adminID(t, deps, "owner")
	addTotp(t, deps, uid)
	codes, err := auth.NewRecoveryCodes(deps.DB, uid)
	if err != nil {
		t.Fatal(err)
	}
	return codes, passwordStep(t, mux, "owner", pw)
}

func TestSignInRecovery(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	codes, pending := recoveryPending(t, deps, mux, pw)
	post := func(code string, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/session/recovery", `{"code":"`+code+`"}`, headers...)
	}

	wantError(t, post(codes[0]), 401, "unauthenticated")
	wantError(t, post(codes[0], "Cookie", pendingCookieName+"=unknown"), 401, "unauthenticated")
	wantError(t, post(codes[0], append(pending, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPost, "/api/session/recovery", `{"code":""}`, pending...), 400, "invalid_request")
	rec := post("aaaaa-aaaaa", pending...)
	wantError(t, rec, 401, "invalid_code")
	if sessionCookie(rec) != nil {
		t.Fatal("wrong code set a session cookie")
	}
	rec = post(codes[0], pending...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"username":"owner"`) {
		t.Fatalf("valid code = %d %s", rec.Code, rec.Body)
	}
	session := sessionCookie(rec)
	if session == nil {
		t.Fatal("no session cookie")
	}
	if c := pendingCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("bm_pending = %+v, want cleared", c)
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, adminID(t, deps, "owner")); n != 9 {
		t.Fatalf("codes left = %d, want 9", n)
	}
	// The pending is used up, and the spent code fails on a new one.
	wantError(t, post(codes[1], pending...), 401, "unauthenticated")
	again := passwordStep(t, mux, "owner", pw)
	wantError(t, post(codes[0], again...), 401, "invalid_code")
	// Without the dash and in capitals works.
	if rec := post(strings.ToUpper(strings.ReplaceAll(codes[1], "-", "")), again...); rec.Code != 200 {
		t.Fatalf("code without dash in capitals = %d %s", rec.Code, rec.Body)
	}
	out := logs.String()
	if !strings.Contains(out, "second_factor=recovery") || !strings.Contains(out, "sign-in failed, wrong recovery code") {
		t.Errorf("log lacks the events: %s", out)
	}
	for _, leak := range []string{codes[0], codes[1], pending[1], session.Value} {
		if strings.Contains(out, leak) {
			t.Fatalf("log contains a secret: %s", out)
		}
	}
}

func TestSignInRecoveryLimitsAndKinds(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	codes, pending := recoveryPending(t, deps, mux, pw)
	for range 5 {
		wantError(t, do(mux, http.MethodPost, "/api/session/recovery", `{"code":"aaaaa-aaaaa"}`, pending...), 401, "invalid_code")
	}
	rec := do(mux, http.MethodPost, "/api/session/recovery", `{"code":"`+codes[0]+`"}`, pending...)
	wantError(t, rec, 429, "too_many_attempts")
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("no Retry-After")
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, adminID(t, deps, "owner")); n != 10 {
		t.Fatalf("codes left = %d, want 10: a limited request spent one", n)
	}

	enroll, err := auth.CreateChallenge(deps.DB, adminID(t, deps, "owner"), auth.KindEnroll, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	wantError(t, do(mux, http.MethodPost, "/api/session/recovery", `{"code":"`+codes[0]+`"}`, "Cookie", pendingCookieName+"="+enroll), 401, "unauthenticated")
}

func TestSignInRecoveryCancelledKeepsCode(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	codes, pending := recoveryPending(t, deps, mux, pw)
	if rec := do(mux, http.MethodDelete, "/api/session/pending", "", pending...); rec.Code != 204 {
		t.Fatalf("cancel = %d", rec.Code)
	}
	wantError(t, do(mux, http.MethodPost, "/api/session/recovery", `{"code":"`+codes[0]+`"}`, pending...), 401, "unauthenticated")
	if n, _ := auth.RecoveryCodesLeft(deps.DB, adminID(t, deps, "owner")); n != 10 {
		t.Fatalf("codes left = %d, want 10", n)
	}
}
