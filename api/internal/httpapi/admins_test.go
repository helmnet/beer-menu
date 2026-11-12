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

// signInAs signs username in through the API and returns its Cookie header pair.
func signInAs(t *testing.T, h http.Handler, username, password string) []string {
	t.Helper()
	c := sessionCookie(postSession(h, username, password))
	if c == nil {
		t.Fatalf("sign-in as %s failed", username)
	}
	return cookieHeader(c)
}

func TestAdmins(t *testing.T) {
	logs := captureLog(t)
	deps, _ := newTestDeps(t)
	ownerPW := createAdmin(t, deps)
	mux := NewMux(deps)
	owner := signInAs(t, mux, "owner", ownerPW)

	pw := testPassword()
	rec := do(mux, http.MethodPost, "/api/admins", credentials("second", pw), owner...)
	if rec.Code != 201 || strings.Contains(rec.Body.String(), pw) {
		t.Fatalf("POST /api/admins = %d %s", rec.Code, rec.Body)
	}
	var second auth.Admin
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil || second.Username != "second" {
		t.Fatalf("admin body = %s (%v)", rec.Body, err)
	}
	secondCookie := signInAs(t, mux, "second", pw)
	wantError(t, do(mux, http.MethodPost, "/api/admins", credentials("SECOND", testPassword()), owner...), 409, "username_taken")
	wantError(t, do(mux, http.MethodPost, "/api/admins", credentials("third", "short"), owner...), 400, "invalid_request")

	list := do(mux, http.MethodGet, "/api/admins", "", owner...)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"username":"owner"`) || !strings.Contains(list.Body.String(), `"username":"second"`) {
		t.Fatalf("GET /api/admins = %d %s", list.Code, list.Body)
	}
	if strings.Contains(list.Body.String(), "argon2") || strings.Contains(list.Body.String(), "hash") {
		t.Fatalf("list exposes password data: %s", list.Body)
	}

	path := "/api/admins/" + strconv.FormatInt(second.ID, 10)
	if rec := do(mux, http.MethodDelete, path, "", owner...); rec.Code != 204 {
		t.Fatalf("DELETE %s = %d %s", path, rec.Code, rec.Body)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", secondCookie...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodDelete, path, "", owner...), 404, "not_found")
	wantError(t, do(mux, http.MethodDelete, "/api/admins/0", "", owner...), 404, "not_found")
	wantError(t, do(mux, http.MethodDelete, "/api/admins/1", "", owner...), 409, "last_admin")

	out := logs.String()
	for _, event := range []string{"admin added", "admin removed", "username=second"} {
		if !strings.Contains(out, event) {
			t.Errorf("log lacks %q: %s", event, out)
		}
	}
	if strings.Contains(out, pw) {
		t.Fatal("log contains a password")
	}
}

func TestRemoveSelfSignsOut(t *testing.T) {
	deps, _ := newTestDeps(t)
	ownerPW := createAdmin(t, deps)
	mux := NewMux(deps)
	owner := signInAs(t, mux, "owner", ownerPW)
	pw := testPassword()
	if rec := do(mux, http.MethodPost, "/api/admins", credentials("second", pw), owner...); rec.Code != 201 {
		t.Fatalf("add = %d", rec.Code)
	}
	second := signInAs(t, mux, "second", pw)
	rec := do(mux, http.MethodDelete, "/api/admins/2", "", second...)
	if rec.Code != 204 {
		t.Fatalf("remove self = %d %s", rec.Code, rec.Body)
	}
	if c := sessionCookie(rec); c == nil || c.MaxAge >= 0 {
		t.Fatalf("remove self cookie = %+v, want cleared", c)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", second...), 401, "unauthenticated")
	if got := do(mux, http.MethodGet, "/api/session", "", owner...); got.Code != 200 {
		t.Fatalf("other admin's session = %d", got.Code)
	}
}

func TestAddRemoveAdminNeedConfirm(t *testing.T) {
	deps, clock := newTestDeps(t)
	ownerPW := createAdmin(t, deps)
	mux := NewMux(deps)
	owner := signInAs(t, mux, "owner", ownerPW)
	pw := testPassword()
	if rec := do(mux, http.MethodPost, "/api/admins", credentials("second", pw), owner...); rec.Code != 201 {
		t.Fatalf("add = %d", rec.Code)
	}
	second := adminID(t, deps, "second")
	// The owner's sign-in no longer counts as a recent confirm.
	clock.Advance(auth.ConfirmWindow + time.Second)

	wantError(t, do(mux, http.MethodPost, "/api/admins", credentials("third", testPassword()), owner...), 403, "confirm_required")
	// The confirm check runs before the body is read, so a taken username is not revealed.
	wantError(t, do(mux, http.MethodPost, "/api/admins", credentials("second", testPassword()), owner...), 403, "confirm_required")
	path := "/api/admins/" + strconv.FormatInt(second, 10)
	wantError(t, do(mux, http.MethodDelete, path, "", owner...), 403, "confirm_required")
	// The confirm check runs before the lookup, so an unconfirmed caller cannot probe ids.
	wantError(t, do(mux, http.MethodDelete, "/api/admins/999", "", owner...), 403, "confirm_required")
	if n, _ := auth.AdminCount(deps.DB); n != 2 {
		t.Fatalf("AdminCount = %d, want 2 (nothing added or removed)", n)
	}

	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": ownerPW}), owner...); rec.Code != 204 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodPost, "/api/admins", credentials("third", testPassword()), owner...); rec.Code != 201 {
		t.Fatalf("add after confirm = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodDelete, path, "", owner...); rec.Code != 204 {
		t.Fatalf("remove after confirm = %d %s", rec.Code, rec.Body)
	}
	if n, _ := auth.AdminCount(deps.DB); n != 2 {
		t.Fatalf("AdminCount = %d, want 2", n)
	}
}

func TestAdminRoutesRefuseTokens(t *testing.T) {
	deps, _ := newTestDeps(t)
	createAdmin(t, deps)
	mux := NewMux(deps)
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		bearer := []string{"Authorization", "Bearer " + secret}
		wantError(t, do(mux, http.MethodGet, "/api/admins", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/admins", credentials("x", testPassword()), bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodDelete, "/api/admins/1", "", bearer...), 403, "forbidden")
		wantError(t, do(mux, http.MethodPost, "/api/password", passwordChange(testPassword(), testPassword()), bearer...), 403, "forbidden")
	}
	if n, _ := auth.AdminCount(deps.DB); n != 1 {
		t.Fatalf("AdminCount = %d", n)
	}
}

func TestClearAdminTwoFactor(t *testing.T) {
	logs := captureLog(t)
	deps, clock := newTestDeps(t)
	ownerPW := createAdmin(t, deps)
	mux := NewMux(deps)
	owner := signInAs(t, mux, "owner", ownerPW)
	pw := testPassword()
	if rec := do(mux, http.MethodPost, "/api/admins", credentials("second", pw), owner...); rec.Code != 201 {
		t.Fatalf("add = %d", rec.Code)
	}
	// The owner's sign-in no longer counts as a recent confirm.
	clock.Advance(auth.ConfirmWindow + time.Second)
	second := adminID(t, deps, "second")
	secondSession := signInAs(t, mux, "second", pw)
	addTotp(t, deps, second)
	addPasskey(t, deps, second)
	if _, err := auth.NewRecoveryCodes(deps.DB, second); err != nil {
		t.Fatal(err)
	}
	pending := passwordStep(t, mux, "second", pw)
	if err := auth.SetRequireTwoFactor(deps.DB, true, t0); err != nil {
		t.Fatal(err)
	}

	list := do(mux, http.MethodGet, "/api/admins", "", owner...).Body.String()
	if !strings.Contains(list, `"username":"owner","created_at":"2026-11-08T12:00:00Z","two_factor":false`) || !strings.Contains(list, `"username":"second"`) || !strings.Contains(list, `"two_factor":true`) {
		t.Fatalf("GET /api/admins = %s", list)
	}

	path := "/api/admins/" + strconv.FormatInt(second, 10) + "/2fa"
	wantError(t, do(mux, http.MethodDelete, path, "", owner...), 403, "confirm_required")
	// The confirm check runs before the lookup, so an unconfirmed caller cannot probe ids.
	wantError(t, do(mux, http.MethodDelete, "/api/admins/999/2fa", "", owner...), 403, "confirm_required")
	if has, _ := auth.HasTwoFactor(deps.DB, second); !has {
		t.Fatal("cleared without a confirm")
	}
	// The owner has no factor of their own, so the password alone confirms.
	if rec := do(mux, http.MethodPost, "/api/2fa/confirm", confirmJSON(map[string]string{"password": ownerPW}), owner...); rec.Code != 204 {
		t.Fatalf("confirm = %d %s", rec.Code, rec.Body)
	}
	if rec := do(mux, http.MethodDelete, path, "", owner...); rec.Code != 204 {
		t.Fatalf("DELETE %s = %d %s", path, rec.Code, rec.Body)
	}
	if has, _ := auth.HasTwoFactor(deps.DB, second); has {
		t.Fatal("factors left")
	}
	if n, _ := auth.RecoveryCodesLeft(deps.DB, second); n != 0 {
		t.Fatalf("%d recovery codes left", n)
	}
	wantError(t, do(mux, http.MethodGet, "/api/session", "", secondSession...), 401, "unauthenticated")
	wantError(t, do(mux, http.MethodPost, "/api/session/totp", `{"code":"123456"}`, pending...), 401, "unauthenticated")
	if got := do(mux, http.MethodGet, "/api/session", "", owner...); got.Code != 200 {
		t.Fatalf("caller's session = %d", got.Code)
	}

	wantError(t, do(mux, http.MethodDelete, "/api/admins/1/2fa", "", owner...), 409, "cannot_clear_self")
	wantError(t, do(mux, http.MethodDelete, "/api/admins/999/2fa", "", owner...), 404, "not_found")
	wantError(t, do(mux, http.MethodDelete, "/api/admins/0/2fa", "", owner...), 404, "not_found")
	clock.Advance(auth.ConfirmWindow + time.Second)
	wantError(t, do(mux, http.MethodDelete, path, "", owner...), 403, "confirm_required")
	wantError(t, do(mux, http.MethodDelete, path, "", append(owner, "Sec-Fetch-Site", "cross-site")...), 403, "forbidden")
	for _, scope := range []auth.Scope{auth.ScopeRead, auth.ScopeWrite} {
		secret, _ := insertToken(t, deps.DB, scope, false)
		wantError(t, do(mux, http.MethodDelete, path, "", "Authorization", "Bearer "+secret), 403, "forbidden")
	}
	if !strings.Contains(logs.String(), "admin 2fa cleared") || !strings.Contains(logs.String(), "username=second") || !strings.Contains(logs.String(), "by=owner") {
		t.Errorf("log lacks the clear: %s", logs)
	}
}
