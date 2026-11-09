package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"

	"github.com/helmnet/beer-menu/api/internal/auth"
	"github.com/helmnet/beer-menu/api/internal/store"
)

// resetFixture is a data folder at the newest schema with admin "owner" and one session.
type resetFixture struct {
	dir, oldPassword string
}

func newResetFixture(t *testing.T) resetFixture {
	t.Helper()
	dir := t.TempDir()
	db, err := startup(context.Background(), config{DataDir: dir}, store.Migrations, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	pw := rand.Text()
	a, err := auth.CreateFirstAdmin(db, "owner", pw, fixedNow())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.CreateSession(db, a.ID, fixedNow()); err != nil {
		t.Fatal(err)
	}
	return resetFixture{dir: dir, oldPassword: pw}
}

// state reports whether password signs owner in and how many sessions exist.
func (f resetFixture) state(t *testing.T, password string) (signsIn bool, sessions int) {
	t.Helper()
	db, err := store.Open(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = auth.Authenticate(db, "owner", password)
	if err := db.QueryRow("SELECT count(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	return err == nil, sessions
}

// runReset runs the subcommand with entries as the typed passwords and returns the exit code
// and what it printed.
func runReset(t *testing.T, dir string, args []string, terminal bool, entries ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	read := func(int) ([]byte, error) {
		if len(entries) == 0 {
			t.Fatal("prompted more often than expected")
		}
		e := entries[0]
		entries = entries[1:]
		return []byte(e), nil
	}
	code := resetPassword(dir, args, 0, &out, func(int) bool { return terminal }, read)
	return code, out.String()
}

func TestResetPassword(t *testing.T) {
	logs := captureLog(t)
	f := newResetFixture(t)
	next := rand.Text()
	code, out := runReset(t, f.dir, []string{"owner"}, true, next, next)
	if code != 0 {
		t.Fatalf("exit %d, output %q", code, out)
	}
	if ok, n := f.state(t, next); !ok || n != 0 {
		t.Fatalf("after reset: new password signs in %v, %d sessions; want true, 0", ok, n)
	}
	if ok, _ := f.state(t, f.oldPassword); ok {
		t.Fatal("old password still works")
	}
	if !strings.Contains(logs.String(), "password reset") || !strings.Contains(logs.String(), "username=owner") {
		t.Errorf("log lacks password reset: %s", logs)
	}
	if strings.Contains(out+logs.String(), next) {
		t.Fatal("output or log contains the password")
	}
}

func TestResetPasswordRefuses(t *testing.T) {
	short := strings.Repeat("x", 11)
	valid := rand.Text()
	cases := map[string]struct {
		args     []string
		terminal bool
		entries  []string
	}{
		"mismatch":     {[]string{"owner"}, true, []string{rand.Text(), rand.Text()}},
		"short":        {[]string{"owner"}, true, []string{short, short}},
		"unknown user": {[]string{"nobody"}, true, []string{valid, valid}},
		"no terminal":  {[]string{"owner"}, false, nil},
		"no username":  {nil, true, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			logs := captureLog(t)
			f := newResetFixture(t)
			code, out := runReset(t, f.dir, c.args, c.terminal, c.entries...)
			if code != 1 {
				t.Fatalf("exit %d, want 1; output %q", code, out)
			}
			if ok, n := f.state(t, f.oldPassword); !ok || n != 1 {
				t.Fatalf("state changed: old password %v, %d sessions", ok, n)
			}
			for _, e := range c.entries {
				if strings.Contains(out+logs.String(), e) {
					t.Fatal("output or log contains a typed password")
				}
			}
		})
	}
}

func TestResetPasswordRefusesOldSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ms, err := store.LoadMigrations(store.Migrations)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(db, ms[:1]); err != nil {
		t.Fatal(err)
	}
	db.Close()
	pw := rand.Text()
	if code, out := runReset(t, dir, []string{"owner"}, true, pw, pw); code != 1 || !strings.Contains(out, "schema") {
		t.Fatalf("exit %d, output %q; want 1 naming the schema", code, out)
	}
	if v := userVersion(t, dir); v != 1 {
		t.Fatalf("version = %d, want 1 unchanged", v)
	}
}

func TestResetPasswordRefusesMissingDatabase(t *testing.T) {
	dir := t.TempDir()
	pw := rand.Text()
	if code, out := runReset(t, dir, []string{"owner"}, true, pw, pw); code != 1 || !strings.Contains(out, "no database") {
		t.Fatalf("exit %d, output %q; want 1 saying there is no database", code, out)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("data folder holds %v (err %v), want it left empty", entries, err)
	}
}
