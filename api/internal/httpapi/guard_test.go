package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/auth"
)

func TestGuardAccessTable(t *testing.T) {
	deps, _ := newTestDeps(t)
	uid := insertUser(t, deps.DB, "owner")
	sid, err := auth.CreateSession(deps.DB, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	readSecret, readID := insertToken(t, deps.DB, auth.ScopeRead, false)
	writeSecret, writeID := insertToken(t, deps.DB, auth.ScopeWrite, false)
	revokedSecret, _ := insertToken(t, deps.DB, auth.ScopeWrite, true)

	var seen principal
	var reached bool
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, reached = principalFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	cookie := []string{"Cookie", sessionCookieName + "=" + sid}
	bearer := func(v string) []string { return []string{"Authorization", v} }

	type cred struct {
		name    string
		headers []string
	}
	none := cred{"none", nil}
	session := cred{"session", cookie}
	readTok := cred{"read token", bearer("Bearer " + readSecret)}
	writeTok := cred{"write token", bearer("Bearer " + writeSecret)}

	cases := []struct {
		access Access
		cred   cred
		want   int
	}{
		{AccessPublic, none, 204}, {AccessPublic, session, 204}, {AccessPublic, readTok, 204}, {AccessPublic, writeTok, 204},
		{AccessRead, none, 401}, {AccessRead, session, 204}, {AccessRead, readTok, 204}, {AccessRead, writeTok, 204},
		{AccessWrite, none, 401}, {AccessWrite, session, 204}, {AccessWrite, readTok, 403}, {AccessWrite, writeTok, 204},
		{AccessAdmin, none, 401}, {AccessAdmin, session, 204}, {AccessAdmin, readTok, 403}, {AccessAdmin, writeTok, 403},
	}
	for _, c := range cases {
		reached = false
		rec := do(guard(deps, c.access, stub), http.MethodGet, "/x", "", c.cred.headers...)
		if rec.Code != c.want {
			t.Errorf("%s with %s: status %d, want %d", c.access, c.cred.name, rec.Code, c.want)
		}
		if (rec.Code == 204) != (reached || c.access == AccessPublic) {
			t.Errorf("%s with %s: handler reached = %v", c.access, c.cred.name, reached)
		}
		switch {
		case c.want == 401:
			wantError(t, rec, 401, "unauthenticated")
		case c.want == 403:
			wantError(t, rec, 403, "forbidden")
		}
	}

	bad := []string{"Bearer", "Bearer ", "Basic x", "Bearer bmt_", "bmt_", "Bearer " + revokedSecret, "Bearer bmt_" + strings.Repeat("unknown", 6) + "x"}
	for _, access := range []Access{AccessRead, AccessWrite, AccessAdmin} {
		for _, h := range bad {
			rec := do(guard(deps, access, stub), http.MethodGet, "/x", "", append(bearer(h), cookie...)...)
			wantError(t, rec, 401, "unauthenticated")
		}
		// Two Authorization headers are ambiguous, so neither is trusted.
		rec := do(guard(deps, access, stub), http.MethodGet, "/x", "", append(bearer("Bearer "+writeSecret), bearer("Bearer "+writeSecret)...)...)
		wantError(t, rec, 401, "unauthenticated")
	}
	if rec := do(guard(deps, AccessAdmin, stub), http.MethodGet, "/x", "", "Cookie", sessionCookieName+"=forged"); rec.Code != 401 {
		t.Errorf("forged cookie: status %d, want 401", rec.Code)
	}

	check := func(headers []string, access Access, want principal) {
		t.Helper()
		reached = false
		do(guard(deps, access, stub), http.MethodGet, "/x", "", headers...)
		if !reached {
			t.Fatal("handler saw no principal")
		}
		if seen.AdminID != want.AdminID || seen.SessionID != want.SessionID || (seen.Token == nil) != (want.Token == nil) {
			t.Fatalf("principal = %+v, want %+v", seen, want)
		}
		if want.Token != nil && (seen.Token.ID != want.Token.ID || seen.Token.Scope != want.Token.Scope) {
			t.Fatalf("principal token = %+v, want %+v", seen.Token, want.Token)
		}
	}
	// The auth scheme name is case-insensitive (RFC 9110).
	check(bearer("bearer "+writeSecret), AccessWrite, principal{Token: &auth.Token{ID: writeID, Scope: auth.ScopeWrite}})
	check(cookie, AccessAdmin, principal{AdminID: uid, SessionID: sid})
	check(readTok.headers, AccessRead, principal{Token: &auth.Token{ID: readID, Scope: auth.ScopeRead}})
	check(writeTok.headers, AccessWrite, principal{Token: &auth.Token{ID: writeID, Scope: auth.ScopeWrite}})
}
