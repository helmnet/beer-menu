package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/helmnet/beer-menu/api/internal/auth/authtest"
)

const (
	testOrigin = "https://taplist.example.org"
	testRPID   = "taplist.example.org"
)

func newWebAuthn(t *testing.T) *webauthn.WebAuthn {
	t.Helper()
	wa, err := webauthn.New(&webauthn.Config{RPDisplayName: "Beer Menu", RPID: testRPID, RPOrigins: []string{testOrigin}})
	if err != nil {
		t.Fatal(err)
	}
	return wa
}

// mustJSON marshals v or fails the test.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// registerPasskey runs a whole registration of a for userID signed in with sid and returns the
// recovery codes, failing on any error.
func registerPasskey(t *testing.T, db *sql.DB, wa *webauthn.WebAuthn, sid string, userID int64, a *authtest.Authenticator, name string, now time.Time) (Passkey, []string) {
	t.Helper()
	opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	pk, codes, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, userID, name, a.Register(t, mustJSON(t, opts)), now)
	if err != nil {
		t.Fatalf("finish registration: %v", err)
	}
	return pk, codes
}

func TestPasskeyRegistration(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	other := mustCreateSession(t, db, uid, t0)

	a := authtest.New(t, testOrigin, testRPID)
	a.BackupEligible = true
	pk, codes := registerPasskey(t, db, wa, sid, uid, a, "  Phone ", t0)
	if pk.Name != "Phone" || pk.ID == 0 || !pk.CreatedAt.Equal(t0) || pk.LastUsedAt != nil {
		t.Fatalf("passkey = %+v", pk)
	}
	if len(codes) != 10 {
		t.Fatalf("first passkey gave %d recovery codes, want 10", len(codes))
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions WHERE user_id = ?", uid); n != 1 {
		t.Fatalf("%d sessions after the first factor, want 1", n)
	}
	if _, ok, _ := LookupSession(db, other, t0); ok {
		t.Fatal("other session survived the first factor")
	}
	var credID, pub []byte
	var count int64
	var transports string
	var be bool
	if err := db.QueryRow("SELECT credential_id, public_key, sign_count, transports, backup_eligible FROM passkeys WHERE id = ?", pk.ID).
		Scan(&credID, &pub, &count, &transports, &be); err != nil {
		t.Fatal(err)
	}
	if string(credID) != string(a.CredentialID) || len(pub) == 0 || count != 0 || transports != `["internal"]` || !be {
		t.Fatalf("stored credential_id %x public_key %d bytes sign_count %d transports %s backup_eligible %v", credID, len(pub), count, transports, be)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges"); n != 0 {
		t.Fatalf("%d challenges left after registration", n)
	}

	// A second passkey: excludeCredentials names the first, no new codes, sessions kept.
	third := mustCreateSession(t, db, uid, t0)
	opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	ex := opts.Response.CredentialExcludeList
	if len(ex) != 1 || string(ex[0].CredentialID) != string(a.CredentialID) {
		t.Fatalf("excludeCredentials = %+v", ex)
	}
	sel := opts.Response.AuthenticatorSelection
	if sel.UserVerification != "required" || sel.ResidentKey != "preferred" || opts.Response.Attestation != "none" {
		t.Fatalf("selection = %+v attestation %q", sel, opts.Response.Attestation)
	}
	b := authtest.New(t, testOrigin, testRPID)
	_, codes, err = FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Laptop", b.Register(t, mustJSON(t, opts)), t0)
	if err != nil || codes != nil {
		t.Fatalf("second passkey = %v codes %v", err, codes)
	}
	if _, ok, _ := LookupSession(db, third, t0); !ok {
		t.Fatal("a later factor ended other sessions")
	}
	if left, _ := RecoveryCodesLeft(db, uid); left != 10 {
		t.Fatalf("codes left = %d", left)
	}
}

func TestPasskeyName(t *testing.T) {
	for in, want := range map[string]string{" Phone ": "Phone", strings.Repeat("ä", 64): strings.Repeat("ä", 64), "Pixel 9 🔑": "Pixel 9 🔑"} {
		if got, err := ValidatePasskeyName(in); err != nil || got != want {
			t.Errorf("ValidatePasskeyName(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "   ", strings.Repeat("a", 65), "a\x07b", "tab\there", "line\nbreak", "\u0085"} {
		var ve ValidationError
		if _, err := ValidatePasskeyName(in); !errors.As(err, &ve) {
			t.Errorf("ValidatePasskeyName(%q) = %v, want a ValidationError", in, err)
		}
	}

	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	a := authtest.New(t, testOrigin, testRPID)
	var ve ValidationError
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "", a.Register(t, mustJSON(t, opts)), t0); !errors.As(err, &ve) {
		t.Fatalf("empty name = %v", err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM passkeys"); n != 0 {
		t.Fatal("bad name saved a passkey")
	}
}

// insertPasskeys stores n placeholder passkeys for userID.
func insertPasskeys(t *testing.T, db *sql.DB, userID int64, n int) {
	t.Helper()
	for range n {
		if _, err := db.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
			VALUES (?, 'Key', randomblob(16), x'bb', 0, '[]', 0, 1)`, userID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPasskeyCap(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	insertPasskeys(t, db, uid, MaxPasskeys-1)
	opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0)
	if err != nil {
		t.Fatalf("10th passkey begin = %v", err)
	}
	// Another tab reached the cap between begin and finish.
	insertPasskeys(t, db, uid, 1)
	a := authtest.New(t, testOrigin, testRPID)
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Eleventh", a.Register(t, mustJSON(t, opts)), t0); !errors.Is(err, ErrTooManyPasskeys) {
		t.Fatalf("11th passkey finish = %v, want ErrTooManyPasskeys", err)
	}
	if _, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0); !errors.Is(err, ErrTooManyPasskeys) {
		t.Fatalf("11th passkey begin = %v, want ErrTooManyPasskeys", err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM passkeys WHERE user_id = ?", uid); n != MaxPasskeys {
		t.Fatalf("%d passkeys, want %d", n, MaxPasskeys)
	}
}

func TestPasskeysUnavailable(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	if _, err := BeginPasskeyRegistration(db, nil, sid, KindSetup, uid, t0); !errors.Is(err, ErrPasskeysUnavailable) {
		t.Errorf("begin registration = %v", err)
	}
	if _, _, err := FinishPasskeyRegistration(db, nil, sid, KindSetup, uid, "Phone", []byte("{}"), t0); !errors.Is(err, ErrPasskeysUnavailable) {
		t.Errorf("finish registration = %v", err)
	}
	if _, err := BeginPasskeyLogin(db, nil, sid, KindSetup, uid, t0); !errors.Is(err, ErrPasskeysUnavailable) {
		t.Errorf("begin login = %v", err)
	}
	if err := FinishPasskeyLogin(db, nil, sid, KindSetup, uid, []byte("{}"), t0); !errors.Is(err, ErrPasskeysUnavailable) {
		t.Errorf("finish login = %v", err)
	}
}

func TestPasskeyRegistrationRefused(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	for name, bend := range map[string]func(a *authtest.Authenticator){
		"wrong origin": func(a *authtest.Authenticator) { a.Origin = "https://other.example.org" },
		"wrong RP ID":  func(a *authtest.Authenticator) { a.RPID = "other.example.org" },
		"UV off":       func(a *authtest.Authenticator) { a.UV = false },
	} {
		opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0)
		if err != nil {
			t.Fatal(err)
		}
		a := authtest.New(t, testOrigin, testRPID)
		bend(a)
		if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", a.Register(t, mustJSON(t, opts)), t0); !errors.Is(err, ErrPasskeyFailed) {
			t.Errorf("%s = %v, want ErrPasskeyFailed", name, err)
		}
	}
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", []byte(`{"id":"x"}`), t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("malformed response = %v", err)
	}

	// Expired: the setup is older than 300 s.
	opts, err := BeginPasskeyRegistration(db, wa, sid, KindSetup, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	resp := authtest.New(t, testOrigin, testRPID).Register(t, mustJSON(t, opts))
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", resp, t0.Add(ChallengeTTL)); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("expired setup = %v", err)
	}
	// Another session cannot finish this session's setup.
	sid2 := mustCreateSession(t, db, uid, t0)
	if _, _, err := FinishPasskeyRegistration(db, wa, sid2, KindSetup, uid, "Phone", resp, t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("other session = %v", err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM passkeys"); n != 0 {
		t.Fatalf("refused registrations saved %d passkeys", n)
	}
	// The right response still works once, then the setup is used up.
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", resp, t0); err != nil {
		t.Fatalf("valid response = %v", err)
	}
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", resp, t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("replayed registration = %v", err)
	}
	// A confirm's request options cannot finish a registration.
	if _, err := BeginPasskeyLogin(db, wa, sid, KindSetup, uid, t0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FinishPasskeyRegistration(db, wa, sid, KindSetup, uid, "Phone", resp, t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("registration on login options = %v", err)
	}
}

// passkeyLogin begins a login on challenge id of kind for userID and returns a's assertion.
func passkeyLogin(t *testing.T, db *sql.DB, wa *webauthn.WebAuthn, id string, kind ChallengeKind, userID int64, a *authtest.Authenticator, now time.Time) []byte {
	t.Helper()
	opts, err := BeginPasskeyLogin(db, wa, id, kind, userID, now)
	if err != nil {
		t.Fatal(err)
	}
	return a.Assert(t, mustJSON(t, opts))
}

func TestPasskeyLogin(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	otherUID := insertUser(t, db, "other")
	sid := mustCreateSession(t, db, uid, t0)
	a := authtest.New(t, testOrigin, testRPID)
	pk, _ := registerPasskey(t, db, wa, sid, uid, a, "Phone", t0)
	b := authtest.New(t, testOrigin, testRPID)
	registerPasskey(t, db, wa, mustCreateSession(t, db, otherUID, t0), otherUID, b, "Theirs", t0)

	pending, err := CreateChallenge(db, uid, KindSecondFactor, t0)
	if err != nil {
		t.Fatal(err)
	}
	opts, err := BeginPasskeyLogin(db, wa, pending, KindSecondFactor, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Response.UserVerification != "required" {
		t.Fatalf("userVerification = %q", opts.Response.UserVerification)
	}
	if allow := opts.Response.AllowedCredentials; len(allow) != 1 || string(allow[0].CredentialID) != string(a.CredentialID) {
		t.Fatalf("allowCredentials = %+v, want only the admin's passkey", allow)
	}
	a.SignCount = 3
	resp := a.Assert(t, mustJSON(t, opts))
	now := t0.Add(time.Minute)
	if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, uid, resp, now); err != nil {
		t.Fatalf("login = %v", err)
	}
	var count, used int64
	if err := db.QueryRow("SELECT sign_count, last_used_at FROM passkeys WHERE id = ?", pk.ID).Scan(&count, &used); err != nil {
		t.Fatal(err)
	}
	if count != 3 || used != now.Unix() {
		t.Fatalf("sign_count %d last_used_at %d", count, used)
	}
	// The challenge is single use, and the pending sign-in itself stays for its owner to take.
	if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, uid, resp, now); !errors.Is(err, ErrPasskeyFailed) {
		t.Fatalf("replayed assertion = %v", err)
	}
	if _, ok, _ := PeekChallenge(db, pending, KindSecondFactor, now); !ok {
		t.Fatal("the passkey step consumed the pending sign-in")
	}

	// The same flow as a confirm, on the session's setup row.
	a.SignCount = 4
	if err := FinishPasskeyLogin(db, wa, sid, KindSetup, uid, passkeyLogin(t, db, wa, sid, KindSetup, uid, a, now), now); err != nil {
		t.Fatalf("confirm login = %v", err)
	}
}

func TestPasskeyLoginRefused(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	otherUID := insertUser(t, db, "other")
	sid := mustCreateSession(t, db, uid, t0)
	a := authtest.New(t, testOrigin, testRPID)
	registerPasskey(t, db, wa, sid, uid, a, "Phone", t0)
	theirs := authtest.New(t, testOrigin, testRPID)
	registerPasskey(t, db, wa, mustCreateSession(t, db, otherUID, t0), otherUID, theirs, "Theirs", t0)
	pending, err := CreateChallenge(db, uid, KindSecondFactor, t0)
	if err != nil {
		t.Fatal(err)
	}
	refused := func(name string, who *authtest.Authenticator, at time.Time) {
		t.Helper()
		resp := passkeyLogin(t, db, wa, pending, KindSecondFactor, uid, who, t0)
		if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, uid, resp, at); !errors.Is(err, ErrPasskeyFailed) {
			t.Errorf("%s = %v, want ErrPasskeyFailed", name, err)
		}
	}
	bent := func(f func(c *authtest.Authenticator)) *authtest.Authenticator {
		c := *a
		f(&c)
		return &c
	}
	refused("wrong origin", bent(func(c *authtest.Authenticator) { c.Origin = "https://192.168.1.10" }), t0)
	refused("wrong RP ID", bent(func(c *authtest.Authenticator) { c.RPID = "other.example.org" }), t0)
	refused("UV off", bent(func(c *authtest.Authenticator) { c.UV = false }), t0)
	refused("another admin's credential", theirs, t0)
	refused("unknown credential", authtest.New(t, testOrigin, testRPID), t0)
	refused("backup flag changed", bent(func(c *authtest.Authenticator) { c.BackupEligible = true }), t0)

	resp := passkeyLogin(t, db, wa, pending, KindSecondFactor, uid, a, t0)
	if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, otherUID, resp, t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("finish for another admin = %v", err)
	}
	resp = passkeyLogin(t, db, wa, pending, KindSecondFactor, uid, a, t0)
	if err := FinishPasskeyLogin(db, wa, pending, KindEnroll, uid, resp, t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("finish on another kind = %v", err)
	}
	if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, uid, []byte(`{"id":1}`), t0); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("malformed or used = %v", err)
	}

	// Sign counts: a non-zero count must go up.
	a.SignCount = 5
	if err := FinishPasskeyLogin(db, wa, pending, KindSecondFactor, uid, passkeyLogin(t, db, wa, pending, KindSecondFactor, uid, a, t0), t0); err != nil {
		t.Fatalf("count 5 = %v", err)
	}
	for _, n := range []uint32{5, 4, 0} {
		a.SignCount = n
		refused("sign count "+string(rune('0'+n)), a, t0)
	}
	if n := countRows(t, db, "SELECT sign_count FROM passkeys WHERE user_id = ?", uid); n != 5 {
		t.Fatalf("sign_count = %d after refusals, want 5", n)
	}

	// The challenge lives 300 s from the pending sign-in.
	late, err := CreateChallenge(db, uid, KindSecondFactor, t0)
	if err != nil {
		t.Fatal(err)
	}
	a.SignCount = 6
	resp = passkeyLogin(t, db, wa, late, KindSecondFactor, uid, a, t0)
	if err := FinishPasskeyLogin(db, wa, late, KindSecondFactor, uid, resp, t0.Add(ChallengeTTL)); !errors.Is(err, ErrPasskeyFailed) {
		t.Errorf("expired challenge = %v", err)
	}
}

func TestPasskeyLoginWithoutPasskeys(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	pending, err := CreateChallenge(db, uid, KindSecondFactor, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BeginPasskeyLogin(db, newWebAuthn(t), pending, KindSecondFactor, uid, t0); !errors.Is(err, ErrNoPasskeys) {
		t.Fatalf("begin without passkeys = %v, want ErrNoPasskeys", err)
	}
	if _, err := BeginPasskeyLogin(db, newWebAuthn(t), "unknown", KindSecondFactor, uid, t0); err == nil {
		t.Fatal("begin on an unknown pending succeeded")
	}
}

func TestPasskeyRegistrationOnEnrollPending(t *testing.T) {
	db := openDB(t)
	wa := newWebAuthn(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	pending := mustChallenge(t, db, uid, KindEnroll, t0)
	if _, err := BeginPasskeyRegistration(db, wa, pending, KindEnroll, other, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another admin's pending = %v, want ErrNotFound", err)
	}
	opts, err := BeginPasskeyRegistration(db, wa, pending, KindEnroll, uid, t0)
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges WHERE kind = 'setup'"); n != 0 {
		t.Fatalf("enroll setup made %d setup rows", n)
	}
	rival := mustChallenge(t, db, uid, KindEnroll, t0) // another browser that gave the password
	pk, codes, err := FinishPasskeyRegistration(db, wa, pending, KindEnroll, uid, "Phone", authtest.New(t, testOrigin, testRPID).Register(t, mustJSON(t, opts)), t0)
	if err != nil || pk.Name != "Phone" || len(codes) != 10 {
		t.Fatalf("finish = %+v, %d codes, %v", pk, len(codes), err)
	}
	if _, ok := peek(t, db, pending, KindEnroll, t0); ok {
		t.Fatal("the enroll pending is still usable")
	}
	if _, ok := peek(t, db, rival, KindEnroll, t0); ok {
		t.Fatal("another enroll pending survived the first factor")
	}
}
