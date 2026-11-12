package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/helmnet/beer-menu/api/internal/secrets"
)

func TestHasTwoFactor(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	has := func() bool {
		t.Helper()
		ok, err := HasTwoFactor(db, uid)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if has() {
		t.Fatal("2FA with no factor")
	}
	if _, err := NewRecoveryCodes(db, uid); err != nil {
		t.Fatal(err)
	}
	if has() {
		t.Fatal("recovery codes alone count as 2FA")
	}
	if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, 'v1:x', 0, 1)", uid); err != nil {
		t.Fatal(err)
	}
	if !has() {
		t.Fatal("TOTP row does not count")
	}
	db.Exec("DELETE FROM totp_factors")
	if _, err := db.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, 'Phone', x'aa', x'bb', 0, '[]', 0, 1)`, uid); err != nil {
		t.Fatal(err)
	}
	if !has() {
		t.Fatal("passkey row does not count")
	}
}

var recoveryCodeRule = regexp.MustCompile(`^[a-z2-7]{5}-[a-z2-7]{5}$`)

func TestRecoveryCodes(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	first, err := NewRecoveryCodes(db, uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 10 {
		t.Fatalf("got %d codes, want 10", len(first))
	}
	seen := map[string]bool{}
	for _, c := range first {
		if !recoveryCodeRule.MatchString(c) || seen[c] {
			t.Fatalf("code %q is malformed or repeated", c)
		}
		seen[c] = true
		norm := c[:5] + c[6:]
		sum := sha256.Sum256([]byte(norm))
		if n := countRows(t, db, "SELECT count(*) FROM recovery_codes WHERE user_id = ? AND code_hash = ? AND used_at IS NULL", uid, sum[:]); n != 1 {
			t.Fatalf("code %q not stored as SHA-256 of %q", c, norm)
		}
	}
	left := func(id int64) int {
		t.Helper()
		n, err := RecoveryCodesLeft(db, id)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := left(uid); n != 10 {
		t.Fatalf("left = %d, want 10", n)
	}

	use := func(id int64, code string, now time.Time) bool {
		t.Helper()
		ok, err := UseRecoveryCode(db, id, code, now)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if use(other, first[0], t0) {
		t.Fatal("code used by another admin")
	}
	upper := []byte(first[0][:5] + " " + first[0][6:])
	for i := range upper {
		if upper[i] >= 'a' && upper[i] <= 'z' {
			upper[i] -= 'a' - 'A'
		}
	}
	if !use(uid, string(upper), t0) {
		t.Fatal("code in upper case with a space refused")
	}
	if use(uid, first[0], t0) {
		t.Fatal("code used twice")
	}
	if n := countRows(t, db, "SELECT count(*) FROM recovery_codes WHERE user_id = ? AND used_at = ?", uid, t0.Unix()); n != 1 {
		t.Fatalf("used_at rows = %d, want 1", n)
	}
	if n := left(uid); n != 9 {
		t.Fatalf("left = %d, want 9", n)
	}
	if use(uid, "aaaaa-aaaaa", t0) {
		t.Fatal("unknown code accepted")
	}

	second, err := NewRecoveryCodes(db, uid)
	if err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM recovery_codes WHERE user_id = ?", uid); n != 10 || left(uid) != 10 {
		t.Fatalf("rows after a new set = %d, want 10", n)
	}
	if use(uid, first[1], t0) {
		t.Fatal("code of the replaced set accepted")
	}
	if !use(uid, second[0], t0) {
		t.Fatal("code of the new set refused")
	}
}

func TestClearTwoFactor(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	for _, id := range []int64{uid, other} {
		if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, 'v1:x', 0, 1)", id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
			VALUES (?, 'Phone', ?, x'bb', 0, '[]', 0, 1)`, id, []byte{byte(id)}); err != nil {
			t.Fatal(err)
		}
		if _, err := NewRecoveryCodes(db, id); err != nil {
			t.Fatal(err)
		}
		mustChallenge(t, db, id, KindSecondFactor, t0)
		mustCreateSession(t, db, id, t0)
	}
	if err := ClearTwoFactor(db, uid); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"totp_factors", "passkeys", "recovery_codes", "auth_challenges", "sessions"} {
		if n := countRows(t, db, "SELECT count(*) FROM "+table+" WHERE user_id = ?", uid); n != 0 {
			t.Errorf("%s rows of the cleared admin = %d", table, n)
		}
		if n := countRows(t, db, "SELECT count(*) FROM "+table+" WHERE user_id = ?", other); n == 0 {
			t.Errorf("%s rows of another admin deleted", table)
		}
	}
	if err := ClearTwoFactor(db, 999); err != ErrNotFound {
		t.Fatalf("unknown admin = %v, want ErrNotFound", err)
	}
}

func TestRequireTwoFactorSetting(t *testing.T) {
	db := openDB(t)
	get := func() bool {
		t.Helper()
		on, err := RequireTwoFactor(db)
		if err != nil {
			t.Fatal(err)
		}
		return on
	}
	if get() {
		t.Fatal("missing setting reads as on")
	}
	if err := SetRequireTwoFactor(db, true, t0); err != nil {
		t.Fatal(err)
	}
	if !get() {
		t.Fatal("on reads as off")
	}
	var v string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'require_2fa'").Scan(&v); err != nil || v != "1" {
		t.Fatalf("stored %q (err %v), want 1", v, err)
	}
	if err := SetRequireTwoFactor(db, false, t0); err != nil {
		t.Fatal(err)
	}
	if get() {
		t.Fatal("off reads as on")
	}
	for _, junk := range []string{"true", "yes", "on", " 1"} {
		db.Exec("UPDATE settings SET value = ? WHERE key = 'require_2fa'", junk)
		if get() {
			t.Fatalf("%q reads as on", junk)
		}
	}
}

// testBox returns a Box on a fresh random key.
func testBox(t *testing.T) *secrets.Box {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

// insertTotp stores an authenticator for userID with secret sealed by box, last used at step
// lastStep.
func insertTotp(t *testing.T, db *sql.DB, box *secrets.Box, userID int64, secret []byte, lastStep int64) {
	t.Helper()
	sealed, err := box.Seal(string(secret), "totp:"+strconv.FormatInt(userID, 10))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, ?, ?, ?)", userID, sealed, lastStep, t0.Unix()); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTotp(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	secret, _ := NewSecret()
	step := t0.Unix() / 30
	check := func(b *secrets.Box, id int64, code string, now time.Time) bool {
		t.Helper()
		ok, err := CheckTotp(db, b, id, code, now)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if check(box, uid, Code(secret, step), t0) {
		t.Fatal("code accepted with no authenticator")
	}
	insertTotp(t, db, box, uid, secret, 0)
	if !check(box, uid, Code(secret, step), t0) {
		t.Fatal("current code refused")
	}
	var last, used int64
	if err := db.QueryRow("SELECT last_step, last_used_at FROM totp_factors WHERE user_id = ?", uid).Scan(&last, &used); err != nil ||
		last != step || used != t0.Unix() {
		t.Fatalf("last_step %d last_used_at %d (err %v), want %d %d", last, used, err, step, t0.Unix())
	}
	if check(box, uid, Code(secret, step), t0) {
		t.Fatal("replayed code accepted")
	}
	if check(box, uid, "000000", t0.Add(30*time.Second)) && Code(secret, step+1) != "000000" {
		t.Fatal("wrong code accepted")
	}
	if check(nil, uid, Code(secret, step+1), t0.Add(30*time.Second)) {
		t.Fatal("code accepted without a key")
	}
	if ok, err := CheckTotp(db, testBox(t), uid, Code(secret, step+1), t0.Add(30*time.Second)); ok || err == nil {
		t.Fatalf("another key = %v %v, want an error", ok, err)
	}
	// A sealed secret moved to another admin's row does not open.
	db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) SELECT ?, secret_sealed, 0, 1 FROM totp_factors WHERE user_id = ?", other, uid)
	if _, err := CheckTotp(db, box, other, Code(secret, step+1), t0.Add(30*time.Second)); err == nil {
		t.Fatal("moved secret opened")
	}
}

func TestLoadTwoFactorState(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	s, err := LoadTwoFactorState(db, uid)
	if err != nil || s.Enabled || s.Required || s.Totp != nil || s.Passkeys == nil || len(s.Passkeys) != 0 || s.RecoveryCodesLeft != 0 {
		t.Fatalf("empty state = %+v (err %v)", s, err)
	}
	secret, _ := NewSecret()
	insertTotp(t, db, box, uid, secret, 0)
	db.Exec("UPDATE totp_factors SET last_used_at = 99")
	if _, err := db.Exec(`INSERT INTO passkeys (id, user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at, last_used_at)
		VALUES (7, ?, 'Phone', x'aa', x'bb', 0, '[]', 0, 5, NULL)`, uid); err != nil {
		t.Fatal(err)
	}
	NewRecoveryCodes(db, uid)
	SetRequireTwoFactor(db, true, t0)
	s, err = LoadTwoFactorState(db, uid)
	if err != nil || !s.Enabled || !s.Required || s.RecoveryCodesLeft != 10 {
		t.Fatalf("state = %+v (err %v)", s, err)
	}
	if s.Totp == nil || !s.Totp.CreatedAt.Equal(t0) || s.Totp.LastUsedAt == nil || s.Totp.LastUsedAt.Unix() != 99 {
		t.Fatalf("totp = %+v", s.Totp)
	}
	if len(s.Passkeys) != 1 || s.Passkeys[0].ID != 7 || s.Passkeys[0].Name != "Phone" || s.Passkeys[0].CreatedAt.Unix() != 5 || s.Passkeys[0].LastUsedAt != nil {
		t.Fatalf("passkeys = %+v", s.Passkeys)
	}
}

func TestStartTotpSetup(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	if _, _, err := StartTotpSetup(db, nil, sid, uid, "owner", t0); !errors.Is(err, ErrTotpUnavailable) {
		t.Fatalf("no key = %v, want ErrTotpUnavailable", err)
	}
	secret, uri, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base32NoPad.DecodeString(secret)
	if err != nil || len(raw) != 20 {
		t.Fatalf("secret %q is not 20 bytes of base32 (err %v)", secret, err)
	}
	if uri != URI("owner", raw) {
		t.Fatalf("uri = %q", uri)
	}
	c, ok := peek(t, db, sid, KindSetup, t0)
	if !ok || c.UserID != uid || c.TotpPendingSealed == "" || strings.Contains(c.TotpPendingSealed, secret) {
		t.Fatalf("setup challenge = %+v, %v", c, ok)
	}
	if opened, err := box.Open(c.TotpPendingSealed, "totp:"+strconv.FormatInt(uid, 10)); err != nil || opened != string(raw) {
		t.Fatalf("pending secret does not open as totp:<user_id> (err %v)", err)
	}
	// A second start replaces the first secret.
	again, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil || again == secret {
		t.Fatalf("second start = %q (err %v), want a new secret", again, err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM auth_challenges WHERE user_id = ?", uid); n != 1 {
		t.Fatalf("setup rows = %d, want 1", n)
	}
	insertTotp(t, db, box, uid, raw, 0)
	if _, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0); !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("with an authenticator = %v, want ErrAlreadySetUp", err)
	}
}

func TestFinishTotpSetup(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	other := mustCreateSession(t, db, uid, t0)
	finish := func(b *secrets.Box, code string, now time.Time) ([]string, bool, error) {
		t.Helper()
		return FinishTotpSetup(db, b, sid, uid, code, now)
	}
	if _, ok, err := finish(box, "123456", t0); ok || err != nil {
		t.Fatalf("no setup open = %v %v, want false", ok, err)
	}
	enc, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32NoPad.DecodeString(enc)
	step := t0.Unix() / 30
	if _, _, err := finish(nil, Code(secret, step), t0); !errors.Is(err, ErrTotpUnavailable) {
		t.Fatalf("no key = %v, want ErrTotpUnavailable", err)
	}
	wrong := "000000"
	if Code(secret, step) == wrong || Code(secret, step-1) == wrong || Code(secret, step+1) == wrong {
		wrong = "000001"
	}
	if codes, ok, err := finish(box, wrong, t0); ok || err != nil || codes != nil {
		t.Fatalf("wrong code = %v %v %v", codes, ok, err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM totp_factors"); n != 0 {
		t.Fatal("wrong code saved an authenticator")
	}
	if _, ok := peek(t, db, sid, KindSetup, t0); !ok {
		t.Fatal("wrong code ended the setup")
	}
	// Another session cannot finish this session's setup.
	if _, ok, err := FinishTotpSetup(db, box, other, uid, Code(secret, step), t0); ok || err != nil {
		t.Fatalf("other session = %v %v", ok, err)
	}
	codes, ok, err := finish(box, Code(secret, step), t0)
	if err != nil || !ok || len(codes) != 10 {
		t.Fatalf("valid code = %v %v %v, want 10 recovery codes", codes, ok, err)
	}
	var last, created int64
	if err := db.QueryRow("SELECT last_step, created_at FROM totp_factors WHERE user_id = ?", uid).Scan(&last, &created); err != nil || last != step || created != t0.Unix() {
		t.Fatalf("row last_step %d created_at %d (err %v)", last, created, err)
	}
	if _, ok := peek(t, db, sid, KindSetup, t0); ok {
		t.Fatal("pending secret kept after the authenticator was saved")
	}
	if n, _ := RecoveryCodesLeft(db, uid); n != 10 {
		t.Fatalf("recovery codes stored = %d", n)
	}
	if _, ok := lookup(t, db, sid, t0); !ok {
		t.Fatal("the setting-up session ended")
	}
	if _, ok := lookup(t, db, other, t0); ok {
		t.Fatal("other session kept after the first factor")
	}
	// The saved secret works for sign-in, and not with the step already used.
	if ok, err := CheckTotp(db, box, uid, Code(secret, step), t0); ok || err != nil {
		t.Fatalf("setup code reused = %v %v", ok, err)
	}
	if ok, err := CheckTotp(db, box, uid, Code(secret, step+1), t0.Add(30*time.Second)); !ok || err != nil {
		t.Fatalf("next code = %v %v", ok, err)
	}
	if _, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0); !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("second authenticator = %v", err)
	}
}

func TestFinishTotpSetupNotFirstFactor(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	if _, err := db.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, 'Phone', x'aa', x'bb', 0, '[]', 0, 1)`, uid); err != nil {
		t.Fatal(err)
	}
	old, err := NewRecoveryCodes(db, uid)
	if err != nil {
		t.Fatal(err)
	}
	sid := mustCreateSession(t, db, uid, t0)
	other := mustCreateSession(t, db, uid, t0)
	enc, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32NoPad.DecodeString(enc)
	codes, ok, err := FinishTotpSetup(db, box, sid, uid, Code(secret, t0.Unix()/30), t0)
	if err != nil || !ok || codes != nil {
		t.Fatalf("second factor = %v %v %v, want no new codes", codes, ok, err)
	}
	if _, ok := lookup(t, db, other, t0); !ok {
		t.Fatal("a second factor ended other sessions")
	}
	if used, _ := UseRecoveryCode(db, uid, old[0], t0); !used {
		t.Fatal("a second factor replaced the recovery codes")
	}
}

func TestFinishTotpSetupExpired(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	enc, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32NoPad.DecodeString(enc)
	later := t0.Add(ChallengeTTL)
	if _, ok, err := FinishTotpSetup(db, box, sid, uid, Code(secret, later.Unix()/30), later); ok || err != nil {
		t.Fatalf("expired setup = %v %v, want false", ok, err)
	}
}

func TestFinishTotpSetupAfterAnotherSaved(t *testing.T) {
	db := openDB(t)
	box := testBox(t)
	uid := insertUser(t, db, "owner")
	sid := mustCreateSession(t, db, uid, t0)
	enc, _, err := StartTotpSetup(db, box, sid, uid, "owner", t0)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := base32NoPad.DecodeString(enc)
	other, _ := NewSecret()
	insertTotp(t, db, box, uid, other, 0)
	if _, ok, err := FinishTotpSetup(db, box, sid, uid, Code(secret, t0.Unix()/30), t0); ok || !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("finish after another was saved = %v %v, want ErrAlreadySetUp", ok, err)
	}
	if _, ok := peek(t, db, sid, KindSetup, t0); !ok {
		t.Fatal("refused finish ended the setup")
	}
}

func TestSignInWithRecoveryCode(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	codes, err := NewRecoveryCodes(db, uid)
	if err != nil {
		t.Fatal(err)
	}
	pending := mustChallenge(t, db, uid, KindSecondFactor, t0)
	if ok, err := SignInWithRecoveryCode(db, pending, uid, "aaaaa-aaaaa", t0); ok || err != nil {
		t.Fatalf("wrong code = %v, %v", ok, err)
	}
	if _, ok, _ := PeekChallenge(db, pending, KindSecondFactor, t0); !ok {
		t.Fatal("wrong code used up the pending sign-in")
	}
	if ok, err := SignInWithRecoveryCode(db, pending, other, codes[0], t0); ok || !errors.Is(err, ErrNotFound) {
		t.Fatalf("another admin's pending = %v, %v; want ErrNotFound", ok, err)
	}
	if n, _ := RecoveryCodesLeft(db, uid); n != 10 {
		t.Fatalf("codes left = %d, want 10", n)
	}
	// A refused attempt on another admin's pending rolled back, so the pending is still there.
	if ok, err := SignInWithRecoveryCode(db, pending, uid, codes[0], t0); !ok || err != nil {
		t.Fatalf("right code = %v, %v", ok, err)
	}
	if _, ok, _ := PeekChallenge(db, pending, KindSecondFactor, t0); ok {
		t.Fatal("pending sign-in not used up")
	}
	if ok, err := SignInWithRecoveryCode(db, pending, uid, codes[1], t0); ok || !errors.Is(err, ErrNotFound) {
		t.Fatalf("used pending = %v, %v; want ErrNotFound", ok, err)
	}
	if n, _ := RecoveryCodesLeft(db, uid); n != 9 {
		t.Fatalf("codes left = %d, want 9: a code without a pending was spent", n)
	}
}

// insertPasskeyRow stores a placeholder passkey named name for userID and returns its ID.
func insertPasskeyRow(t *testing.T, db *sql.DB, userID int64, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, ?, randomblob(16), x'bb', 0, '[]', 0, 1) RETURNING id`, userID, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRenamePasskey(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	id := insertPasskeyRow(t, db, uid, "Phone")
	theirs := insertPasskeyRow(t, db, other, "Phone")
	pk, err := RenamePasskey(db, uid, id, "  Laptop ")
	if err != nil || pk.ID != id || pk.Name != "Laptop" || pk.CreatedAt.Unix() != 1 {
		t.Fatalf("rename = %+v, %v", pk, err)
	}
	var ve ValidationError
	if _, err := RenamePasskey(db, uid, id, "\t"); !errors.As(err, &ve) {
		t.Fatalf("blank name = %v, want ValidationError", err)
	}
	if _, err := RenamePasskey(db, uid, theirs, "Mine"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another admin's passkey = %v, want ErrNotFound", err)
	}
}

// factorFixture gives userID an authenticator, two passkeys, recovery codes, a pending sign-in
// and two sessions, and returns the passkey IDs and the session to keep.
func factorFixture(t *testing.T, db *sql.DB, userID int64) (passkeys []int64, keep string) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, 'v1:x', 0, 1)", userID); err != nil {
		t.Fatal(err)
	}
	passkeys = []int64{insertPasskeyRow(t, db, userID, "A"), insertPasskeyRow(t, db, userID, "B")}
	if _, err := NewRecoveryCodes(db, userID); err != nil {
		t.Fatal(err)
	}
	mustChallenge(t, db, userID, KindSecondFactor, t0)
	keep = mustCreateSession(t, db, userID, t0)
	mustCreateSession(t, db, userID, t0)
	return passkeys, keep
}

func TestRemoveFactors(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	other := insertUser(t, db, "other")
	pks, keep := factorFixture(t, db, uid)
	otherPks, _ := factorFixture(t, db, other)
	left := func(table string) int {
		return countRows(t, db, "SELECT count(*) FROM "+table+" WHERE user_id = ?", uid)
	}

	if err := RemovePasskey(db, uid, otherPks[0], keep); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another admin's passkey = %v, want ErrNotFound", err)
	}
	if left("sessions") != 2 {
		t.Fatal("a refused removal ended sessions")
	}
	if err := RemovePasskey(db, uid, pks[0], keep); err != nil {
		t.Fatal(err)
	}
	if left("passkeys") != 1 || left("sessions") != 1 || left("auth_challenges") != 0 || left("recovery_codes") != 10 {
		t.Fatalf("after one passkey: %d passkeys, %d sessions, %d challenges, %d codes", left("passkeys"), left("sessions"), left("auth_challenges"), left("recovery_codes"))
	}
	if _, ok, _ := LookupSession(db, keep, t0); !ok {
		t.Fatal("the caller's session ended")
	}
	if err := RemovePasskey(db, uid, pks[0], keep); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed twice = %v, want ErrNotFound", err)
	}
	if err := RemoveTotp(db, uid, keep); err != nil {
		t.Fatal(err)
	}
	if err := RemoveTotp(db, uid, keep); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no authenticator = %v, want ErrNotFound", err)
	}
	if left("recovery_codes") != 10 {
		t.Fatal("codes deleted while a passkey is left")
	}
	if err := RemovePasskey(db, uid, pks[1], keep); err != nil {
		t.Fatal(err)
	}
	if left("recovery_codes") != 0 {
		t.Fatal("the last factor left its recovery codes (FR-025)")
	}
	if n := countRows(t, db, "SELECT count(*) FROM recovery_codes WHERE user_id = ?", other); n != 10 {
		t.Fatalf("other admin's codes = %d", n)
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions WHERE user_id = ?", other); n != 2 {
		t.Fatalf("other admin's sessions = %d", n)
	}
}

func TestTurnOffTwoFactor(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "owner")
	_, keep := factorFixture(t, db, uid)
	if err := TurnOffTwoFactor(db, uid, keep); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"totp_factors", "passkeys", "recovery_codes", "auth_challenges"} {
		if n := countRows(t, db, "SELECT count(*) FROM "+table+" WHERE user_id = ?", uid); n != 0 {
			t.Errorf("%s rows left = %d", table, n)
		}
	}
	if n := countRows(t, db, "SELECT count(*) FROM sessions WHERE user_id = ?", uid); n != 1 {
		t.Fatalf("sessions = %d, want only the caller's", n)
	}
	if err := TurnOffTwoFactor(db, uid, keep); !errors.Is(err, ErrNoFactor) {
		t.Fatalf("already off = %v, want ErrNoFactor", err)
	}
}

func TestFindAdmin(t *testing.T) {
	db := openDB(t)
	uid := insertUser(t, db, "Owner")
	if a, err := FindAdmin(db, " owner "); err != nil || a.ID != uid || a.Username != "Owner" {
		t.Fatalf("FindAdmin = %+v, %v", a, err)
	}
	if _, err := FindAdmin(db, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown = %v, want ErrNotFound", err)
	}
}
