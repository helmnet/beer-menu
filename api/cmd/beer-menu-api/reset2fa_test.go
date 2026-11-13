package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// newReset2faFixture is a data folder at the newest schema with admin "owner" holding an
// authenticator row, recovery codes, a pending sign-in and a session, and require_2fa on.
func newReset2faFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := startup(context.Background(), config{DataDir: dir}, store.Migrations, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a, err := auth.CreateFirstAdmin(db, "owner", rand.Text(), fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, 'v1:x', 0, 1)", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewRecoveryCodes(db, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CreateChallenge(db, a.ID, auth.KindSecondFactor, fixedNow()); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CreateSession(db, a.ID, fixedNow()); err != nil {
		t.Fatal(err)
	}
	if err := auth.SetRequireTwoFactor(db, true, fixedNow()); err != nil {
		t.Fatal(err)
	}
	return dir
}

// reset2faRows returns how many 2FA, challenge and session rows exist, and the require setting.
func reset2faRows(t *testing.T, dir string) (rows int, required bool) {
	t.Helper()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"totp_factors", "passkeys", "recovery_codes", "auth_challenges", "sessions"} {
		var n int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		rows += n
	}
	required, err = auth.RequireTwoFactor(db)
	if err != nil {
		t.Fatal(err)
	}
	return rows, required
}

func TestResetTwoFactor(t *testing.T) {
	logs := captureLog(t)
	dir := newReset2faFixture(t)
	var out bytes.Buffer
	if code := resetTwoFactor(dir, []string{"OWNER"}, &out); code != 0 {
		t.Fatalf("exit %d, output %q", code, out.String())
	}
	if rows, required := reset2faRows(t, dir); rows != 0 || !required {
		t.Fatalf("after reset: %d rows, required %v; want 0, true", rows, required)
	}
	if !strings.Contains(logs.String(), "two-factor reset") || !strings.Contains(logs.String(), "username=owner") || !strings.Contains(logs.String(), "time=") {
		t.Errorf("log lacks the reset: %s", logs)
	}
}

func TestResetTwoFactorRefuses(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown user": {"nobody"},
		"no username":  nil,
		"two names":    {"owner", "other"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := newReset2faFixture(t)
			before, _ := reset2faRows(t, dir)
			var out bytes.Buffer
			if code := resetTwoFactor(dir, args, &out); code != 1 {
				t.Fatalf("exit %d, want 1; output %q", code, out.String())
			}
			if after, required := reset2faRows(t, dir); after != before || !required {
				t.Fatalf("rows %d, want %d unchanged; required %v", after, before, required)
			}
		})
	}
}

func TestResetTwoFactorRefusesMissingDatabase(t *testing.T) {
	var out bytes.Buffer
	if code := resetTwoFactor(t.TempDir(), []string{"owner"}, &out); code != 1 || !strings.Contains(out.String(), "no database") {
		t.Fatalf("exit %d, output %q; want 1 saying there is no database", code, out.String())
	}
}
