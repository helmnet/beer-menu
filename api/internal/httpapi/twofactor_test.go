package httpapi

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/auth/authtest"
)

// adminID returns the ID of the admin named username.
func adminID(t *testing.T, deps Deps, username string) int64 {
	t.Helper()
	var id int64
	if err := deps.DB.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// addTotp gives userID an authenticator sealed with deps.Box and returns its secret.
func addTotp(t *testing.T, deps Deps, userID int64) []byte {
	t.Helper()
	secret, err := auth.NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := deps.Box.Seal(string(secret), "totp:"+strconv.FormatInt(userID, 10))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deps.DB.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, ?, 0, ?)", userID, sealed, t0.Unix()); err != nil {
		t.Fatal(err)
	}
	return secret
}

// addPasskey stores a placeholder passkey row for userID.
func addPasskey(t *testing.T, deps Deps, userID int64) {
	t.Helper()
	if _, err := deps.DB.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, 'Phone', randomblob(16), x'bb', 0, '[]', 0, ?)`, userID, t0.Unix()); err != nil {
		t.Fatal(err)
	}
}

// testWebAuthn returns a relying party for https://taplist.example.org.
func testWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "Beer Menu", RPID: "taplist.example.org", RPOrigins: []string{"https://taplist.example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	return wa
}

// confirmJSON returns a ConfirmBody JSON with the given fields.
func confirmJSON(fields map[string]string) string {
	b, _ := json.Marshal(fields)
	return string(b)
}

func TestConfirmWithoutFactor(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	clock.Advance(10 * time.Minute)
	if ok, _ := auth.SessionConfirmed(deps.DB, sid, clock.Now()); ok {
		t.Fatal("sign-in confirm lasted 10 minutes")
	}
	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw}), session...); rec.Code != 204 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if ok, _ := auth.SessionConfirmed(deps.DB, sid, clock.Now()); !ok {
		t.Fatal("confirm did not stamp the session")
	}
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": testPassword()}), session...), 403, "wrong_password")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw, "code": "123456"}), session...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", `{"password":"x","other":1}`, session...), 400, "invalid_request")
	if !strings.Contains(logs.String(), "identity confirmed") || strings.Contains(logs.String(), pw) {
		t.Errorf("log = %s, want the confirm event and no password", logs)
	}
}

func TestConfirmWithFactor(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	secret := addTotp(t, deps, uid)
	codes, err := auth.NewRecoveryCodes(deps.DB, uid)
	if err != nil {
		t.Fatal(err)
	}
	mux := NewMux(deps)
	session := sessionFor(t, deps, uid)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	confirm := func(fields map[string]string, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(fields), append(headers, session...)...)
	}
	code := func() string { return auth.Code(secret, clock.Now().Unix()/30) }

	wantError(t, confirm(map[string]string{"password": pw}), 400, "invalid_request")
	wantError(t, confirm(map[string]string{"password": pw, "code": code(), "recovery_code": codes[0]}), 400, "invalid_request")
	clock.Advance(10 * time.Minute)
	if rec := confirm(map[string]string{"password": pw, "code": code()}); rec.Code != 204 {
		t.Fatalf("password + code = %d %s", rec.Code, rec.Body)
	}
	if ok, _ := auth.SessionConfirmed(deps.DB, sid, clock.Now()); !ok {
		t.Fatal("confirm did not stamp the session")
	}
	// The same code again is a replay.
	wantError(t, confirm(map[string]string{"password": pw, "code": code()}), 403, "invalid_code")
	clock.Advance(time.Minute)
	if rec := confirm(map[string]string{"password": pw, "recovery_code": strings.ToUpper(codes[0])}); rec.Code != 204 {
		t.Fatalf("password + recovery code = %d %s", rec.Code, rec.Body)
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, uid); n != 9 {
		t.Fatalf("codes left = %d, want 9", n)
	}
	wantError(t, confirm(map[string]string{"password": pw, "recovery_code": codes[0]}), 403, "invalid_code")
	wantError(t, confirm(map[string]string{"password": testPassword(), "code": code()}), 403, "wrong_password")
	clock.Advance(time.Minute)
	wantError(t, confirm(map[string]string{"password": pw, "code": "000000"}), 403, "invalid_code")
	wantError(t, confirm(map[string]string{"password": pw, "recovery_code": "aaaaa-aaaaa"}), 403, "invalid_code")
	// The five failures above reach the limit; the next is refused even if right.
	rec := confirm(map[string]string{"password": pw, "code": code()})
	wantError(t, rec, 429, "too_many_attempts")
	if _, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	wantError(t, postSession(mux, "owner", pw), 429, "too_many_attempts")
}

func TestTwoFactorRoutesRefuseTokensAndCrossSite(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		bearer := []string{"Authorization", "Bearer " + secret}
		wantError(t, do(mux, http.MethodGet, "/api/2fa", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw}), bearer...), 403, "forbidden")
	}
	wantError(t, do(mux, http.MethodGet, "/api/2fa", ""), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw})), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw}),
		append(session, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
}

func TestGetTwoFactor(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	get := func(d Deps, headers ...string) map[string]any {
		t.Helper()
		rec := do(NewMux(d), http.MethodGet, "/api/2fa", "", headers...)
		if rec.Code != 200 {
			t.Fatalf("GET /api/2fa = %d %s", rec.Code, rec.Body)
		}
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	session := signInAs(t, NewMux(deps), "owner", pw)
	got := get(deps, session...)
	want := `{"enabled":false,"passkeys":[],"passkeys_available":false,"passkeys_origin":null,"recovery_codes_left":0,"required":false,"totp":null,"totp_available":true}`
	if b, _ := json.Marshal(got); string(b) != want {
		t.Fatalf("state = %s, want %s", b, want)
	}

	addTotp(t, deps, uid)
	addPasskey(t, deps, uid)
	auth.NewRecoveryCodes(deps.DB, uid)
	auth.SetRequireTwoFactor(deps.DB, true, t0)
	withKeys := deps
	withKeys.WebAuthn = testWebAuthn(t)
	withKeys.Box = nil
	rec := do(NewMux(withKeys), http.MethodGet, "/api/2fa", "", session...)
	got = get(withKeys, session...)
	if got["enabled"] != true || got["required"] != true || got["totp_available"] != false || got["passkeys_available"] != true ||
		got["passkeys_origin"] != "https://taplist.example.org" || got["recovery_codes_left"] != 10.0 || got["totp"] == nil {
		t.Fatalf("state = %v", got)
	}
	if pks, _ := got["passkeys"].([]any); len(pks) != 1 {
		t.Fatalf("passkeys = %v", got["passkeys"])
	}
	for _, leak := range []string{"secret", "sealed", "v1:", "public_key", "credential_id", "code_hash"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Fatalf("state leaks %q: %s", leak, rec.Body)
		}
	}

	enroll, err := auth.CreateChallenge(deps.DB, uid, auth.KindEnroll, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := get(deps, "Cookie", pendingCookieName+"="+enroll); got["enabled"] != true {
		t.Fatalf("state with an enroll pending = %v", got)
	}
}

func TestRequireConfirmed(t *testing.T) {
	deps, clock := newTestDeps(t)
	uid := insertUser(t, deps.DB, "owner")
	sid, err := auth.CreateSession(deps.DB, uid, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	check := func(p principal) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		if ok := requireConfirmed(rec, httptest.NewRequest(http.MethodPost, "/x", nil), deps, p); ok != (rec.Code == 200 && rec.Body.Len() == 0) {
			t.Fatalf("requireConfirmed = %v with response %d %s", ok, rec.Code, rec.Body)
		}
		return rec
	}
	if rec := check(principal{AdminID: uid, SessionID: sid}); rec.Body.Len() != 0 {
		t.Fatalf("fresh session = %d %s", rec.Code, rec.Body)
	}
	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, check(principal{AdminID: uid, SessionID: sid}), 403, "confirm_required")
	// The password of an enroll pending was just checked.
	if rec := check(principal{Pending: "x", PendingUserID: uid}); rec.Body.Len() != 0 {
		t.Fatalf("enroll pending = %d %s", rec.Code, rec.Body)
	}
}

// startTotp posts /api/2fa/totp/setup with headers and returns the decoded secret, failing
// unless it is 200 with a secret and URI.
func startTotp(t *testing.T, mux http.Handler, headers ...string) []byte {
	t.Helper()
	rec := do(mux, http.MethodPost, "/api/2fa/totp/setup", "", headers...)
	if rec.Code != 200 {
		t.Fatalf("totp setup = %d %s", rec.Code, rec.Body)
	}
	var body struct{ Secret, URI string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(body.Secret)
	if err != nil || len(secret) != 20 || body.URI != auth.URI("owner", secret) {
		t.Fatalf("setup body = %s (err %v)", rec.Body, err)
	}
	return secret
}

// codeJSON returns a CodeBody JSON for code.
func codeJSON(code string) string { return `{"code":"` + code + `"}` }

func TestTotpSetup(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	other := signInAs(t, mux, "owner", pw)

	secret := startTotp(t, mux, session...)
	finish := func(code string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/2fa/totp", codeJSON(code), session...)
	}
	wantError(t, finish("000000x"), 400, "invalid_code")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", `{"code":""}`, session...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON(strings.Repeat("1", 33)), session...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", `{"code":"1","x":1}`, session...), 400, "invalid_request")
	code := auth.Code(secret, clock.Now().Unix()/30)
	rec := finish(code[:3] + " " + code[3:])
	if rec.Code != 201 {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body)
	}
	var added struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || len(added.RecoveryCodes) != 10 {
		t.Fatalf("finish body = %s", rec.Body)
	}
	if has, _ := auth.HasTwoFactor(deps.DB, uid); !has {
		t.Fatal("no authenticator saved")
	}
	if got := do(mux, http.MethodGet, "/api/session", "", other...); got.Code != 401 {
		t.Fatalf("other session after the first factor = %d", got.Code)
	}
	if got := do(mux, http.MethodGet, "/api/session", "", session...); got.Code != 200 {
		t.Fatalf("own session after the first factor = %d", got.Code)
	}
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", session...), 409, "already_set_up")
	wantError(t, finish(code), 400, "invalid_code")
	out := logs.String()
	if !strings.Contains(out, "authenticator added") || !strings.Contains(out, "username=owner") {
		t.Errorf("log lacks the event: %s", out)
	}
	for _, leak := range append([]string{base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), code, pw}, added.RecoveryCodes...) {
		if strings.Contains(out, leak) {
			t.Fatalf("log contains a secret: %s", out)
		}
	}
}

func TestTotpSetupNeedsConfirm(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	secret := startTotp(t, mux, session...)
	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", session...), 403, "confirm_required")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON(auth.Code(secret, clock.Now().Unix()/30)), session...), 403, "confirm_required")
	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw}), session...); rec.Code != 204 {
		t.Fatalf("confirm = %d", rec.Code)
	}
	startTotp(t, mux, session...)
}

func TestTotpSetupUnavailable(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	session := signInAs(t, NewMux(deps), "owner", pw)
	deps.Box = nil
	mux := NewMux(deps)
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", session...), 409, "totp_unavailable")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON("123456"), session...), 409, "totp_unavailable")
}

func TestTotpSetupRefusals(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		bearer := []string{"Authorization", "Bearer " + secret}
		wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON("123456"), bearer...), 403, "forbidden")
	}
	crossSite := append(session, "Sec-Fetch-Site", "cross-site")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", crossSite...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON("123456"), crossSite...), 403, "forbidden")
	// A second_factor pending is not an enroll pending.
	second, err := auth.CreateChallenge(deps.DB, uid, auth.KindSecondFactor, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	pending := []string{"Cookie", pendingCookieName + "=" + second}
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp/setup", "", pending...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON("123456"), pending...), 401, "unauthenticated")
	if n, _ := auth.HasTwoFactor(deps.DB, uid); n {
		t.Fatal("refused request added a factor")
	}
}

// newAuthenticator returns a software passkey for testWebAuthn's origin and RP ID.
func newAuthenticator(t *testing.T) *authtest.Authenticator {
	return authtest.New(t, "https://taplist.example.org", "taplist.example.org")
}

// passkeyBody returns the finishPasskeySetup body for name and credential.
func passkeyBody(t *testing.T, name string, credential []byte) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"name": name, "credential": json.RawMessage(credential)})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// passkeyOptions posts to path with headers and returns the 200 options body.
func passkeyOptions(t *testing.T, mux http.Handler, path string, headers ...string) []byte {
	t.Helper()
	return postOptions(t, mux, path, "", headers...)
}

// postOptions is passkeyOptions with a request body.
func postOptions(t *testing.T, mux http.Handler, path, body string, headers ...string) []byte {
	t.Helper()
	rec := do(mux, http.MethodPost, path, body, headers...)
	if rec.Code != 200 {
		t.Fatalf("%s = %d %s", path, rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}

// registerPasskeyHTTP adds a as name through the passkey setup routes with headers and returns
// the finish response.
func registerPasskeyHTTP(t *testing.T, mux http.Handler, a *authtest.Authenticator, name string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	opts := passkeyOptions(t, mux, "/api/2fa/passkeys/options", headers...)
	return do(mux, http.MethodPost, "/api/2fa/passkeys", passkeyBody(t, name, a.Register(t, opts)), headers...)
}

// optionsOf decodes {"publicKey": {...}} options.
func optionsOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var o struct {
		PublicKey map[string]any `json:"publicKey"`
	}
	if err := json.Unmarshal(body, &o); err != nil || o.PublicKey == nil {
		t.Fatalf("options = %s (%v)", body, err)
	}
	return o.PublicKey
}

func TestPasskeySetup(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	other := signInAs(t, mux, "owner", pw)

	opts := optionsOf(t, passkeyOptions(t, mux, "/api/2fa/passkeys/options", session...))
	sel, _ := opts["authenticatorSelection"].(map[string]any)
	if sel["userVerification"] != "required" || sel["residentKey"] != "preferred" || opts["attestation"] != "none" {
		t.Fatalf("creation options = %v", opts)
	}
	a := newAuthenticator(t)
	rec := registerPasskeyHTTP(t, mux, a, " Phone ", session...)
	if rec.Code != 201 {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body)
	}
	var added struct {
		Passkey       *auth.Passkey `json:"passkey"`
		RecoveryCodes []string      `json:"recovery_codes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil || added.Passkey == nil || added.Passkey.Name != "Phone" || len(added.RecoveryCodes) != 10 {
		t.Fatalf("finish body = %s", rec.Body)
	}
	if got := do(mux, http.MethodGet, "/api/session", "", other...); got.Code != 401 {
		t.Fatalf("other session after the first factor = %d", got.Code)
	}

	// A second passkey: the first is excluded, no new codes.
	opts = optionsOf(t, passkeyOptions(t, mux, "/api/2fa/passkeys/options", session...))
	if ex, _ := opts["excludeCredentials"].([]any); len(ex) != 1 {
		t.Fatalf("excludeCredentials = %v", opts["excludeCredentials"])
	}
	rec = registerPasskeyHTTP(t, mux, newAuthenticator(t), "Laptop", session...)
	if rec.Code != 201 || strings.Contains(rec.Body.String(), "recovery_codes") || !strings.Contains(rec.Body.String(), `"name":"Laptop"`) {
		t.Fatalf("second passkey = %d %s", rec.Code, rec.Body)
	}

	// Bad names, a refused response and an oversized body save nothing.
	for _, name := range []string{"", "   ", strings.Repeat("x", 65), "bell\x07"} {
		wantError(t, registerPasskeyHTTP(t, mux, newAuthenticator(t), name, session...), 400, "invalid_request")
	}
	wrong := newAuthenticator(t)
	wrong.Origin = "https://192.168.1.10"
	rec = registerPasskeyHTTP(t, mux, wrong, "Wrong", session...)
	wantError(t, rec, 400, "passkey_failed")
	wantFixedMessage(t, rec, "The passkey could not be added. Try again.")
	passkeyOptions(t, mux, "/api/2fa/passkeys/options", session...)
	big := passkeyBody(t, "Big", []byte(`{"id":"`+strings.Repeat("a", 16<<10)+`"}`))
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", big, session...), 400, "invalid_request")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", `{"name":"x","credential":{},"x":1}`, session...), 400, "invalid_request")
	if n := countPasskeys(t, deps, "owner"); n != 2 {
		t.Fatalf("%d passkeys, want 2", n)
	}

	out := logs.String()
	if !strings.Contains(out, "passkey added") || !strings.Contains(out, "username=owner") {
		t.Errorf("log lacks the event: %s", out)
	}
	for _, leak := range append([]string{b64(a.CredentialID), pw}, added.RecoveryCodes...) {
		if strings.Contains(out, leak) {
			t.Fatalf("log contains a secret: %s", out)
		}
	}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// countPasskeys returns how many passkeys username has.
func countPasskeys(t *testing.T, deps Deps, username string) int {
	t.Helper()
	var n int
	if err := deps.DB.QueryRow("SELECT count(*) FROM passkeys WHERE user_id = ?", adminID(t, deps, username)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPasskeySetupCap(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	for range auth.MaxPasskeys - 1 {
		addPasskey(t, deps, uid)
	}
	opts := passkeyOptions(t, mux, "/api/2fa/passkeys/options", session...)
	addPasskey(t, deps, uid)
	rec := do(mux, http.MethodPost, "/api/2fa/passkeys", passkeyBody(t, "Eleventh", newAuthenticator(t).Register(t, opts)), session...)
	wantError(t, rec, 409, "too_many_passkeys")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", session...), 409, "too_many_passkeys")
	if n := countPasskeys(t, deps, "owner"); n != auth.MaxPasskeys {
		t.Fatalf("%d passkeys, want %d", n, auth.MaxPasskeys)
	}
}

func TestPasskeySetupRefusals(t *testing.T) {
	deps, clock := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	opts := passkeyOptions(t, mux, "/api/2fa/passkeys/options", session...)
	body := passkeyBody(t, "Phone", newAuthenticator(t).Register(t, opts))

	noWA := deps
	noWA.WebAuthn = nil
	off := NewMux(noWA)
	wantError(t, do(off, http.MethodPost, "/api/2fa/passkeys/options", "", session...), 409, "passkeys_unavailable")
	wantError(t, do(off, http.MethodPost, "/api/2fa/passkeys", body, session...), 409, "passkeys_unavailable")

	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		bearer := []string{"Authorization", "Bearer " + secret}
		wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", body, bearer...), 403, "forbidden")
	}
	crossSite := append(session, "Sec-Fetch-Site", "cross-site")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", crossSite...), 403, "forbidden")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", body, crossSite...), 403, "forbidden")
	// A second_factor pending is not an enroll pending.
	second, err := auth.CreateChallenge(deps.DB, uid, auth.KindSecondFactor, deps.Now())
	if err != nil {
		t.Fatal(err)
	}
	pending := []string{"Cookie", pendingCookieName + "=" + second}
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", pending...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", body, pending...), 401, "unauthenticated")

	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", session...), 403, "confirm_required")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys", body, session...), 403, "confirm_required")
	if has, _ := auth.HasTwoFactor(deps.DB, uid); has {
		t.Fatal("refused request added a factor")
	}
}

// confirmPasskeyBody returns a ConfirmBody with password and a passkey assertion.
func confirmPasskeyBody(t *testing.T, password string, assertion []byte, extra map[string]any) string {
	t.Helper()
	m := map[string]any{"password": password, "passkey": json.RawMessage(assertion)}
	for k, v := range extra {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConfirmWithPasskey(t *testing.T) {
	deps, clock := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	a := newAuthenticator(t)
	if rec := registerPasskeyHTTP(t, mux, a, "Phone", session...); rec.Code != 201 {
		t.Fatalf("register = %d %s", rec.Code, rec.Body)
	}
	clock.Advance(10 * time.Minute)
	assertion := func(who *authtest.Authenticator) []byte {
		t.Helper()
		body := postOptions(t, mux, "/api/2fa/confirm/options", confirmJSON(map[string]string{"password": pw}), session...)
		opts := optionsOf(t, body)
		if allow, _ := opts["allowCredentials"].([]any); len(allow) != 1 || opts["userVerification"] != "required" {
			t.Fatalf("request options = %v", opts)
		}
		return who.Assert(t, body)
	}
	confirm := func(body string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/2fa/confirm", body, session...)
	}

	resp := assertion(a)
	if rec := confirm(confirmPasskeyBody(t, pw, resp, nil)); rec.Code != 204 {
		t.Fatalf("password + passkey = %d %s", rec.Code, rec.Body)
	}
	if ok, _ := auth.SessionConfirmed(deps.DB, sid, clock.Now()); !ok {
		t.Fatal("confirm did not stamp the session")
	}
	wantError(t, confirm(confirmPasskeyBody(t, pw, resp, nil)), 403, "passkey_failed")
	wrong := *a
	wrong.Origin = "https://192.168.1.10"
	rec := confirm(confirmPasskeyBody(t, pw, assertion(&wrong), nil))
	wantError(t, rec, 403, "passkey_failed")
	wantFixedMessage(t, rec, "The passkey was not accepted.")
	wantError(t, confirm(confirmPasskeyBody(t, pw, assertion(a), map[string]any{"code": "123456"})), 400, "invalid_request")
	wantError(t, confirm(confirmPasskeyBody(t, testPassword(), assertion(a), nil)), 403, "wrong_password")
	big := confirmPasskeyBody(t, pw, []byte(`{"id":"`+strings.Repeat("a", 16<<10)+`"}`), nil)
	wantError(t, confirm(big), 400, "invalid_request")

	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm/options", "", "Authorization", "Bearer "+secret), 403, "forbidden")
	}
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm/options", ""), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm/options", "", append(session, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	noWA := deps
	noWA.WebAuthn = nil
	wantError(t, do(NewMux(noWA), http.MethodPost, "/api/2fa/confirm/options", confirmJSON(map[string]string{"password": pw}), session...), 409, "passkeys_unavailable")
}

func TestConfirmOptionsChecksPassword(t *testing.T) {
	deps, clock := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	sid := strings.TrimPrefix(session[1], sessionCookieName+"=")
	if rec := registerPasskeyHTTP(t, mux, newAuthenticator(t), "Phone", session...); rec.Code != 201 {
		t.Fatalf("register = %d %s", rec.Code, rec.Body)
	}
	options := func(body string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/2fa/confirm/options", body, session...)
	}
	right := confirmJSON(map[string]string{"password": pw})
	wrong := confirmJSON(map[string]string{"password": testPassword()})

	wantError(t, options(""), 400, "invalid_request")
	wantError(t, options(`{"password":"x","other":1}`), 400, "invalid_request")
	rec := options(wrong)
	wantError(t, rec, 403, "wrong_password")
	wantFixedMessage(t, rec, "The current password is wrong.")
	if _, ok, err := auth.TakeChallenge(deps.DB, sid, auth.KindSetup, clock.Now()); err != nil || ok {
		t.Fatalf("wrong password left a passkey challenge (ok %v, err %v)", ok, err)
	}
	// A right password is no failure: these do not use up the limit.
	for range 6 {
		if rec := options(right); rec.Code != 200 {
			t.Fatalf("options = %d %s", rec.Code, rec.Body)
		}
	}
	for range 4 {
		wantError(t, options(wrong), 403, "wrong_password")
	}
	// Five wrong passwords reach the limit; the next is refused even if right.
	rec = options(right)
	wantError(t, rec, 429, "too_many_attempts")
	if _, err := strconv.Atoi(rec.Header().Get("Retry-After")); err != nil {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	wantError(t, postSession(mux, "owner", pw), 429, "too_many_attempts")
}

func TestConfirmOptionsWithoutPasskey(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	wantError(t, do(mux, http.MethodPost, "/api/2fa/confirm/options", confirmJSON(map[string]string{"password": pw}), session...), 409, "no_factor")
}

func TestNewRecoveryCodes(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	post := func(headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPost, "/api/2fa/recovery-codes", "", headers...)
	}
	wantError(t, post(session...), 409, "no_factor")

	addTotp(t, deps, uid)
	old, err := auth.NewRecoveryCodes(deps.DB, uid)
	if err != nil {
		t.Fatal(err)
	}
	rec := post(session...)
	if rec.Code != 200 {
		t.Fatalf("new codes = %d %s", rec.Code, rec.Body)
	}
	var reply struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil || len(reply.RecoveryCodes) != 10 {
		t.Fatalf("reply = %s (%v)", rec.Body, err)
	}
	for _, c := range reply.RecoveryCodes {
		if !recoveryCodeRule.MatchString(c) {
			t.Fatalf("code %q breaks the pattern", c)
		}
	}
	// An old code no longer signs in; a new one does.
	pending := passwordStep(t, mux, "owner", pw)
	wantError(t, do(mux, http.MethodPost, "/api/session/recovery", codeJSON(old[0]), pending...), 401, "invalid_code")
	if rec := do(mux, http.MethodPost, "/api/session/recovery", codeJSON(reply.RecoveryCodes[0]), pending...); rec.Code != 200 {
		t.Fatalf("new code sign-in = %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(logs.String(), "recovery codes replaced") || strings.Contains(logs.String(), reply.RecoveryCodes[1]) {
		t.Errorf("log = %s, want the event and no code", logs)
	}

	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, post(session...), 403, "confirm_required")
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		wantError(t, post("Authorization", "Bearer "+secret), 403, "forbidden")
	}
	wantError(t, post(), 401, "unauthenticated")
	wantError(t, post(append(session, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
}

// recoveryCodeRule is the RecoveryCodes item pattern of the contract.
var recoveryCodeRule = regexp.MustCompile(`^[a-z2-7]{5}-[a-z2-7]{5}$`)

// passkeyIDs returns userID's passkey IDs, oldest first.
func passkeyIDs(t *testing.T, deps Deps, userID int64) []string {
	t.Helper()
	rows, err := deps.DB.Query("SELECT id FROM passkeys WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	return ids
}

func TestRenamePasskey(t *testing.T) {
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	other := insertUser(t, deps.DB, "other")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	addPasskey(t, deps, uid)
	addPasskey(t, deps, other)
	mine, theirs := passkeyIDs(t, deps, uid)[0], passkeyIDs(t, deps, other)[0]
	patch := func(id, body string, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodPatch, "/api/2fa/passkeys/"+id, body, append(headers, session...)...)
	}
	// Renaming needs no recent confirm.
	clock.Advance(auth.ConfirmWindow + time.Second)
	rec := patch(mine, `{"name":"  Work laptop  "}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"name":"Work laptop"`) || !strings.Contains(rec.Body.String(), `"id":`+mine) {
		t.Fatalf("rename = %d %s", rec.Code, rec.Body)
	}
	for _, leak := range []string{"public_key", "credential_id", "sign_count"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Fatalf("rename leaks %s: %s", leak, rec.Body)
		}
	}
	wantError(t, patch(mine, `{"name":"   "}`), 400, "invalid_request")
	wantError(t, patch(mine, `{"name":"`+strings.Repeat("x", 65)+`"}`), 400, "invalid_request")
	wantError(t, patch(mine, `{"name":"a\u0007b"}`), 400, "invalid_request")
	wantError(t, patch(mine, `{"name":"x","extra":1}`), 400, "invalid_request")
	wantError(t, patch(theirs, `{"name":"Mine now"}`), 404, "not_found")
	wantError(t, patch("999", `{"name":"x"}`), 404, "not_found")
	wantError(t, patch(mine, `{"name":"x"}`, "Sec-Fetch-Site", "cross-site"), 403, "forbidden")
	var name string
	deps.DB.QueryRow("SELECT name FROM passkeys WHERE user_id = ?", other).Scan(&name)
	if name != "Phone" {
		t.Fatalf("other admin's passkey renamed to %q", name)
	}
}

// factorAdmin makes "owner" with an authenticator, two passkeys and recovery codes, signs in
// twice (this session and another), and returns the mux, both Cookie headers and the ID.
func factorAdmin(t *testing.T) (Deps, *testClock, http.Handler, []string, []string, int64) {
	t.Helper()
	deps, clock := newTestDeps(t)
	createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	addTotp(t, deps, uid)
	addPasskey(t, deps, uid)
	addPasskey(t, deps, uid)
	if _, err := auth.NewRecoveryCodes(deps.DB, uid); err != nil {
		t.Fatal(err)
	}
	return deps, clock, NewMux(deps), sessionFor(t, deps, uid), sessionFor(t, deps, uid), uid
}

func TestRemoveFactors(t *testing.T) {
	logs := captureLog(t)
	deps, _, mux, session, otherSession, uid := factorAdmin(t)
	other := insertUser(t, deps.DB, "other")
	addPasskey(t, deps, other)
	ids := passkeyIDs(t, deps, uid)
	del := func(path string, headers ...string) *httptest.ResponseRecorder {
		return do(mux, http.MethodDelete, path, "", append(headers, session...)...)
	}

	wantError(t, del("/api/2fa/passkeys/"+passkeyIDs(t, deps, other)[0]), 404, "not_found")
	if rec := del("/api/2fa/passkeys/" + ids[0]); rec.Code != 204 {
		t.Fatalf("remove passkey = %d %s", rec.Code, rec.Body)
	}
	wantError(t, del("/api/2fa/passkeys/"+ids[0]), 404, "not_found")
	wantError(t, do(mux, http.MethodGet, "/api/session", "", otherSession...), 401, "unauthenticated")
	if got := do(mux, http.MethodGet, "/api/session", "", session...); got.Code != 200 {
		t.Fatalf("this session = %d", got.Code)
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, uid); n != 10 {
		t.Fatalf("codes left = %d with factors left, want 10", n)
	}
	if rec := del("/api/2fa/totp"); rec.Code != 204 {
		t.Fatalf("remove authenticator = %d %s", rec.Code, rec.Body)
	}
	wantError(t, del("/api/2fa/totp"), 404, "not_found")
	if n, _ := auth.RecoveryCodesLeft(deps.DB, uid); n != 10 {
		t.Fatalf("codes left = %d with a passkey left, want 10", n)
	}
	// The last factor takes the recovery codes with it (FR-025).
	if rec := del("/api/2fa/passkeys/" + ids[1]); rec.Code != 204 {
		t.Fatalf("remove last passkey = %d %s", rec.Code, rec.Body)
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, uid); n != 0 {
		t.Fatalf("codes left = %d after the last factor, want 0", n)
	}
	if !strings.Contains(logs.String(), "passkey removed") || !strings.Contains(logs.String(), "authenticator removed") {
		t.Errorf("log lacks the removals: %s", logs)
	}
	wantError(t, del("/api/2fa/passkeys/"+passkeyIDs(t, deps, other)[0]), 404, "not_found")
	if len(passkeyIDs(t, deps, other)) != 1 {
		t.Fatal("other admin's passkey removed")
	}
}

func TestRemoveLastFactorSignInPasswordOnly(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	addTotp(t, deps, uid)
	if _, err := auth.NewRecoveryCodes(deps.DB, uid); err != nil {
		t.Fatal(err)
	}
	if rec := postSession(mux, "owner", pw); rec.Code != 202 {
		t.Fatalf("sign-in with a factor = %d", rec.Code)
	}
	if rec := do(mux, http.MethodDelete, "/api/2fa/totp", "", session...); rec.Code != 204 {
		t.Fatalf("remove = %d %s", rec.Code, rec.Body)
	}
	if rec := postSession(mux, "owner", pw); rec.Code != 200 || sessionCookie(rec) == nil {
		t.Fatalf("sign-in after the last factor = %d %s, want 200 with a session", rec.Code, rec.Body)
	}
}

func TestTurnOffTwoFactor(t *testing.T) {
	logs := captureLog(t)
	deps, _, mux, session, otherSession, uid := factorAdmin(t)
	if _, err := auth.CreateChallenge(deps.DB, uid, auth.KindSecondFactor, deps.Now()); err != nil {
		t.Fatal(err)
	}
	if rec := do(mux, http.MethodDelete, "/api/2fa", "", session...); rec.Code != 204 {
		t.Fatalf("turn off = %d %s", rec.Code, rec.Body)
	}
	state, err := auth.LoadTwoFactorState(deps.DB, uid)
	if err != nil || state.Enabled || state.RecoveryCodesLeft != 0 {
		t.Fatalf("state = %+v (%v), want off with no codes", state, err)
	}
	var pendings int
	deps.DB.QueryRow("SELECT count(*) FROM auth_challenges WHERE user_id = ? AND kind = 'second_factor'", uid).Scan(&pendings)
	if pendings != 0 {
		t.Fatalf("%d pending sign-ins left", pendings)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", otherSession...), 401, "unauthenticated")
	if got := do(mux, http.MethodGet, "/api/session", "", session...); got.Code != 200 {
		t.Fatalf("this session = %d", got.Code)
	}
	wantError(t, do(mux, http.MethodDelete, "/api/2fa", "", session...), 409, "no_factor")
	if !strings.Contains(logs.String(), "two-factor turned off") {
		t.Errorf("log lacks the event: %s", logs)
	}
}

func TestRemoveFactorsRefusals(t *testing.T) {
	deps, clock, mux, session, _, uid := factorAdmin(t)
	id := passkeyIDs(t, deps, uid)[0]
	routes := [][2]string{{http.MethodDelete, "/api/2fa/passkeys/" + id}, {http.MethodDelete, "/api/2fa/totp"}, {http.MethodDelete, "/api/2fa"}, {http.MethodPatch, "/api/2fa/passkeys/" + id}}
	for _, r := range routes {
		body := ""
		if r[0] == http.MethodPatch {
			body = `{"name":"x"}`
		}
		for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
			secret, _ := insertToken(t, deps.DB, scope, false)
			wantError(t, do(mux, r[0], r[1], body, "Authorization", "Bearer "+secret), 403, "forbidden")
		}
		wantError(t, do(mux, r[0], r[1], body), 401, "unauthenticated")
		wantError(t, do(mux, r[0], r[1], body, append(session, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	}
	clock.Advance(auth.ConfirmWindow + time.Second)
	for _, r := range routes[:3] {
		wantError(t, do(mux, r[0], r[1], "", session...), 403, "confirm_required")
	}
	state, err := auth.LoadTwoFactorState(deps.DB, uid)
	if err != nil || state.Totp == nil || len(state.Passkeys) != 2 || state.RecoveryCodesLeft != 10 {
		t.Fatalf("refused requests changed the state: %+v (%v)", state, err)
	}
}

// putRequired sends PUT /api/2fa/required with body and headers.
func putRequired(mux http.Handler, body string, headers ...string) *httptest.ResponseRecorder {
	return do(mux, http.MethodPut, "/api/2fa/required", body, headers...)
}

func TestSetTwoFactorRequired(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	uid := adminID(t, deps, "owner")
	mux := NewMux(deps)
	session := signInAs(t, mux, "owner", pw)
	// An admin without a factor, signed in before the setting goes on (FR-036).
	otherPW := testPassword()
	if rec := do(mux, http.MethodPost, "/api/admins", credentials("second", otherPW), session...); rec.Code != 201 {
		t.Fatalf("add admin = %d", rec.Code)
	}
	other := signInAs(t, mux, "second", otherPW)

	wantError(t, putRequired(mux, `{"required":true}`, session...), 409, "no_factor")
	addTotp(t, deps, uid)
	for _, bad := range []string{`{"required":"yes"}`, `{"required":1}`, `{"required":null}`, `{}`, `{"required":true,"x":1}`, `true`} {
		wantError(t, putRequired(mux, bad, session...), 400, "invalid_request")
	}
	if on, _ := auth.RequireTwoFactor(deps.DB); on {
		t.Fatal("a refused request turned the setting on")
	}
	if rec := putRequired(mux, `{"required":true}`, session...); rec.Code != 204 {
		t.Fatalf("turn on = %d %s", rec.Code, rec.Body)
	}
	if on, _ := auth.RequireTwoFactor(deps.DB); !on {
		t.Fatal("setting not stored")
	}
	for _, s := range [][]string{session, other} {
		if rec := do(mux, http.MethodGet, "/api/session", "", s...); rec.Code != 200 {
			t.Fatalf("a session ended when the setting went on: %d", rec.Code)
		}
	}
	if !strings.Contains(logs.String(), "two-factor requirement changed") || !strings.Contains(logs.String(), "username=owner") || !strings.Contains(logs.String(), "required=true") {
		t.Errorf("log lacks who changed it to what: %s", logs)
	}

	// The display settings neither show nor take it (research R10).
	if rec := do(mux, http.MethodGet, "/api/settings", "", session...); rec.Code != 200 || strings.Contains(rec.Body.String(), "require") {
		t.Fatalf("GET /api/settings = %d %s", rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodPatch, "/api/settings", `{"require_2fa":false}`, session...), 400, "invalid_request")

	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		wantError(t, putRequired(mux, `{"required":false}`, "Authorization", "Bearer "+secret), 403, "forbidden")
	}
	wantError(t, putRequired(mux, `{"required":false}`), 401, "unauthenticated")
	wantError(t, putRequired(mux, `{"required":false}`, append(session, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, putRequired(mux, `{"required":false}`, session...), 403, "confirm_required")
	if on, _ := auth.RequireTwoFactor(deps.DB); !on {
		t.Fatal("a refused request turned the setting off")
	}
	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": pw, "code": auth.Code(addedSecret(t, deps, uid), clock.Now().Unix()/30)}), session...); rec.Code != 204 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if rec := putRequired(mux, `{"required":false}`, session...); rec.Code != 204 {
		t.Fatalf("turn off = %d %s", rec.Code, rec.Body)
	}
	if on, _ := auth.RequireTwoFactor(deps.DB); on {
		t.Fatal("setting still on")
	}
}

// TestSetTwoFactorRequiredUnavailable: the setting cannot go on while the server can enroll no
// factor (no encryption key and no public URL); it can still go off, and either method is enough.
func TestSetTwoFactorRequiredUnavailable(t *testing.T) {
	deps, _ := newTestDeps(t)
	pw := createAdmin(t, deps)
	session := signInAs(t, NewMux(deps), "owner", pw)
	addTotp(t, deps, adminID(t, deps, "owner"))

	none := deps
	none.Box, none.WebAuthn = nil, nil
	wantError(t, putRequired(NewMux(none), `{"required":true}`, session...), 409, "two_factor_unavailable")
	if on, _ := auth.RequireTwoFactor(deps.DB); on {
		t.Fatal("a refused request turned the setting on")
	}

	passkeysOnly := none
	passkeysOnly.WebAuthn = testWebAuthn(t)
	if rec := putRequired(NewMux(passkeysOnly), `{"required":true}`, session...); rec.Code != 204 {
		t.Fatalf("turn on with passkeys only = %d %s", rec.Code, rec.Body)
	}
	if rec := putRequired(NewMux(none), `{"required":false}`, session...); rec.Code != 204 {
		t.Fatalf("turn off with no method = %d %s", rec.Code, rec.Body)
	}
	if on, _ := auth.RequireTwoFactor(deps.DB); on {
		t.Fatal("setting still on")
	}
}

// addedSecret returns userID's stored authenticator secret, opened with deps.Box.
func addedSecret(t *testing.T, deps Deps, userID int64) []byte {
	t.Helper()
	var sealed string
	if err := deps.DB.QueryRow("SELECT secret_sealed FROM totp_factors WHERE user_id = ?", userID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	secret, err := deps.Box.Open(sealed, "totp:"+strconv.FormatInt(userID, 10))
	if err != nil {
		t.Fatal(err)
	}
	return []byte(secret)
}

func TestRemoveLastFactorWhileRequired(t *testing.T) {
	deps, _, mux, session, _, uid := factorAdmin(t)
	if err := auth.SetRequireTwoFactor(deps.DB, true, deps.Now()); err != nil {
		t.Fatal(err)
	}
	for _, id := range passkeyIDs(t, deps, uid) {
		if rec := do(mux, http.MethodDelete, "/api/2fa/passkeys/"+id, "", session...); rec.Code != 204 {
			t.Fatalf("remove a passkey that is not the last = %d %s", rec.Code, rec.Body)
		}
	}
	rec := do(mux, http.MethodDelete, "/api/2fa/totp", "", session...)
	wantError(t, rec, 409, "two_factor_required")
	if !strings.Contains(rec.Body.String(), "Require two-factor sign-in") {
		t.Fatalf("message does not name the setting: %s", rec.Body)
	}
	wantError(t, do(mux, http.MethodDelete, "/api/2fa", "", session...), 409, "two_factor_required")
	state, err := auth.LoadTwoFactorState(deps.DB, uid)
	if err != nil || state.Totp == nil || state.RecoveryCodesLeft != 10 {
		t.Fatalf("refused removals changed the state: %+v (%v)", state, err)
	}
}

// enrollStep turns "require 2FA" on and gives the password of "owner", who has no factor,
// failing unless the answer is 202 enroll with a bm_pending cookie, which it returns.
func enrollStep(t *testing.T, deps Deps, mux http.Handler, pw string) []string {
	t.Helper()
	if err := auth.SetRequireTwoFactor(deps.DB, true, deps.Now()); err != nil {
		t.Fatal(err)
	}
	rec := postSession(mux, "owner", pw)
	c := pendingCookie(rec)
	if rec.Code != 202 || c == nil || sessionCookie(rec) != nil || !strings.Contains(rec.Body.String(), `"next":"enroll"`) {
		t.Fatalf("password step = %d %s", rec.Code, rec.Body)
	}
	return cookieHeader(c)
}

// enrolled checks rec is the 201 that finishes an enroll: a FactorAdded with the admin and 10
// recovery codes, a session cookie that works, and bm_pending cleared. It returns the session.
func enrolled(t *testing.T, mux http.Handler, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var added struct {
		Admin         *auth.Admin `json:"admin"`
		RecoveryCodes []string    `json:"recovery_codes"`
	}
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &added) != nil || added.Admin == nil || added.Admin.Username != "owner" || !added.Admin.TwoFactor || len(added.RecoveryCodes) != 10 {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body)
	}
	pc := pendingCookie(rec)
	if pc == nil || pc.MaxAge >= 0 {
		t.Fatalf("bm_pending not cleared: %v", pc)
	}
	c := sessionCookie(rec)
	if c == nil {
		t.Fatal("no session cookie")
	}
	session := cookieHeader(c)
	if got := do(mux, http.MethodGet, "/api/session", "", session...); got.Code != 200 {
		t.Fatalf("new session = %d", got.Code)
	}
	return session
}

func TestEnrollWithTotp(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	old := signInAs(t, mux, "owner", pw)
	pending := enrollStep(t, deps, mux, pw)

	rec := do(mux, http.MethodGet, "/api/2fa", "", pending...)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"required":true`) {
		t.Fatalf("GET /api/2fa = %d %s", rec.Code, rec.Body)
	}
	secret := startTotp(t, mux, pending...)
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON("00000x"), pending...), 400, "invalid_code")
	code := auth.Code(secret, clock.Now().Unix()/30)
	enrolled(t, mux, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON(code), pending...))
	// The pending sign-in is used up, and the first factor ended the older session.
	wantError(t, do(mux, http.MethodGet, "/api/2fa", "", pending...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/2fa/totp", codeJSON(code), pending...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodGet, "/api/session", "", old...), 401, "unauthenticated")
	if !strings.Contains(logs.String(), "admin signed in") || strings.Contains(logs.String(), code) {
		t.Errorf("log = %s", logs)
	}
}

func TestEnrollWithPasskey(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	pending := enrollStep(t, deps, mux, pw)
	enrolled(t, mux, registerPasskeyHTTP(t, mux, newAuthenticator(t), "Phone", pending...))
	wantError(t, do(mux, http.MethodPost, "/api/2fa/passkeys/options", "", pending...), 401, "unauthenticated")
	if n := countPasskeys(t, deps, "owner"); n != 1 {
		t.Fatalf("%d passkeys, want 1", n)
	}
}

func TestEnrollPendingExpiredOrCancelled(t *testing.T) {
	deps, clock := newTestDeps(t)
	deps.WebAuthn = testWebAuthn(t)
	pw := createAdmin(t, deps)
	mux := NewMux(deps)
	routes := [][2]string{{http.MethodGet, "/api/2fa"}, {http.MethodPost, "/api/2fa/totp/setup"}, {http.MethodPost, "/api/2fa/passkeys/options"}}

	pending := enrollStep(t, deps, mux, pw)
	if rec := do(mux, http.MethodDelete, "/api/session/pending", "", pending...); rec.Code != 204 {
		t.Fatalf("cancel = %d", rec.Code)
	}
	for _, r := range routes {
		wantError(t, do(mux, r[0], r[1], "", pending...), 401, "unauthenticated")
	}

	pending = enrollStep(t, deps, mux, pw)
	clock.Advance(auth.ChallengeTTL)
	for _, r := range routes {
		wantError(t, do(mux, r[0], r[1], "", pending...), 401, "unauthenticated")
	}
	if has, _ := auth.HasTwoFactor(deps.DB, adminID(t, deps, "owner")); has {
		t.Fatal("a refused enroll added a factor")
	}
}
