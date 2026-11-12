package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCreateFirstAdmin(t *testing.T) {
	db := openDB(t)
	if n, err := AdminCount(db); err != nil || n != 0 {
		t.Fatalf("AdminCount = %d, %v; want 0", n, err)
	}
	pw := testPassword(t)
	a, err := CreateFirstAdmin(db, "  owner ", pw, t0)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 || a.Username != "owner" || !a.CreatedAt.Equal(t0) {
		t.Fatalf("admin = %+v", a)
	}
	if n, err := AdminCount(db); err != nil || n != 1 {
		t.Fatalf("AdminCount = %d, %v; want 1", n, err)
	}
	var hash string
	if err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", a.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == pw {
		t.Fatal("password stored in plain")
	}
	if ok, err := VerifyPassword(hash, pw); err != nil || !ok {
		t.Fatalf("stored hash does not verify: %v %v", ok, err)
	}
	if _, err := CreateFirstAdmin(db, "second", testPassword(t), t0); !errors.Is(err, ErrSetupDone) {
		t.Fatalf("second CreateFirstAdmin = %v, want ErrSetupDone", err)
	}
}

func TestCreateFirstAdminValidates(t *testing.T) {
	db := openDB(t)
	var ve ValidationError
	if _, err := CreateFirstAdmin(db, "a b", testPassword(t), t0); !errors.As(err, &ve) {
		t.Fatalf("bad username: %v, want ValidationError", err)
	}
	if _, err := CreateFirstAdmin(db, "owner", strings.Repeat("x", 11), t0); !errors.As(err, &ve) {
		t.Fatalf("short password: %v, want ValidationError", err)
	}
	if n, _ := AdminCount(db); n != 0 {
		t.Fatalf("AdminCount = %d after refused input", n)
	}
}

func TestCreateFirstAdminConcurrent(t *testing.T) {
	db := openDB(t)
	pw := testPassword(t)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = CreateFirstAdmin(db, "owner"+string(rune('a'+i)), pw, t0)
		})
	}
	wg.Wait()
	created := 0
	for _, err := range errs {
		switch {
		case err == nil:
			created++
		case !errors.Is(err, ErrSetupDone):
			t.Errorf("unexpected error %v", err)
		}
	}
	if n, _ := AdminCount(db); created != 1 || n != 1 {
		t.Fatalf("created %d, AdminCount %d; want exactly 1", created, n)
	}
}

func TestAuthenticate(t *testing.T) {
	db := openDB(t)
	pw := testPassword(t)
	created, err := CreateFirstAdmin(db, "owner", pw, t0)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Authenticate(db, " OWNER ", pw)
	if err != nil || a.ID != created.ID || a.Username != "owner" {
		t.Fatalf("Authenticate(right) = %+v, %v", a, err)
	}
	if _, err := Authenticate(db, "owner", pw+"x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v, want ErrInvalidCredentials", err)
	}
	if _, err := Authenticate(db, "nobody", pw); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user: %v, want ErrInvalidCredentials", err)
	}
	var ve ValidationError
	if _, err := Authenticate(db, "owner", strings.Repeat("x", 257)); !errors.As(err, &ve) {
		t.Fatalf("257-character password: %v, want ValidationError", err)
	}
	if got, err := GetAdmin(db, created.ID); err != nil || got != a {
		t.Fatalf("GetAdmin = %+v, %v; want %+v", got, err, a)
	}
	if _, err := GetAdmin(db, created.ID+1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetAdmin(unknown) = %v, want ErrNotFound", err)
	}
}

// sessionCount returns how many sessions userID has.
func sessionCount(t *testing.T, db *sql.DB, userID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sessions WHERE user_id = ?", userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func newSession(t *testing.T, db *sql.DB, userID int64) string {
	t.Helper()
	id, err := CreateSession(db, userID, t0)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAddAndListAdmins(t *testing.T) {
	db := openDB(t)
	first, err := CreateFirstAdmin(db, "owner", testPassword(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	pw := testPassword(t)
	second, err := AddAdmin(db, " second ", pw, t0.Add(time.Hour))
	if err != nil || second.Username != "second" || !second.CreatedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("AddAdmin = %+v, %v", second, err)
	}
	if a, err := Authenticate(db, "second", pw); err != nil || a.ID != second.ID {
		t.Fatalf("new admin cannot sign in: %+v, %v", a, err)
	}
	if _, err := AddAdmin(db, "OWNER", testPassword(t), t0); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("AddAdmin(OWNER) = %v, want ErrUsernameTaken", err)
	}
	var ve ValidationError
	if _, err := AddAdmin(db, "a b", testPassword(t), t0); !errors.As(err, &ve) {
		t.Fatalf("bad username: %v, want ValidationError", err)
	}
	if _, err := AddAdmin(db, "third", "short", t0); !errors.As(err, &ve) {
		t.Fatalf("short password: %v, want ValidationError", err)
	}
	list, err := ListAdmins(db)
	if err != nil || len(list) != 2 || list[0] != first || list[1] != second {
		t.Fatalf("ListAdmins = %+v, %v; want oldest first", list, err)
	}
	body, _ := json.Marshal(list)
	if strings.Contains(string(body), "hash") || strings.Contains(string(body), "argon2") {
		t.Fatalf("list exposes password data: %s", body)
	}
}

func TestRemoveAdmin(t *testing.T) {
	db := openDB(t)
	first, err := CreateFirstAdmin(db, "owner", testPassword(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AddAdmin(db, "second", testPassword(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	newSession(t, db, second.ID)
	if name, err := RemoveAdmin(db, second.ID); err != nil || name != "second" {
		t.Fatalf("RemoveAdmin = %q, %v", name, err)
	}
	if n := sessionCount(t, db, second.ID); n != 0 {
		t.Fatalf("removed admin has %d sessions", n)
	}
	if _, err := RemoveAdmin(db, first.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("RemoveAdmin(last) = %v, want ErrLastAdmin", err)
	}
	if _, err := RemoveAdmin(db, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RemoveAdmin(unknown) = %v, want ErrNotFound", err)
	}
	if n, _ := AdminCount(db); n != 1 {
		t.Fatalf("AdminCount = %d, want 1", n)
	}
}

func TestRemoveAdminConcurrentKeepsOne(t *testing.T) {
	db := openDB(t)
	a, err := CreateFirstAdmin(db, "owner", testPassword(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AddAdmin(db, "second", testPassword(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, id := range []int64{a.ID, b.ID} {
		wg.Go(func() {
			if _, err := RemoveAdmin(db, id); err != nil && !errors.Is(err, ErrLastAdmin) {
				t.Errorf("RemoveAdmin(%d) = %v", id, err)
			}
		})
	}
	wg.Wait()
	if n, _ := AdminCount(db); n != 1 {
		t.Fatalf("AdminCount = %d after concurrent removals, want 1", n)
	}
}

func TestChangePassword(t *testing.T) {
	db := openDB(t)
	old := testPassword(t)
	a, err := CreateFirstAdmin(db, "owner", old, t0)
	if err != nil {
		t.Fatal(err)
	}
	keep := newSession(t, db, a.ID)
	newSession(t, db, a.ID)
	next := testPassword(t)

	if err := ChangePassword(db, a.ID, old+"x", next, keep); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current password: %v, want ErrWrongPassword", err)
	}
	if _, err := Authenticate(db, "owner", old); err != nil || sessionCount(t, db, a.ID) != 2 {
		t.Fatalf("refused change altered state: %v, %d sessions", err, sessionCount(t, db, a.ID))
	}
	var ve ValidationError
	if err := ChangePassword(db, a.ID, old, "short", keep); !errors.As(err, &ve) {
		t.Fatalf("short new password: %v, want ValidationError", err)
	}

	if err := ChangePassword(db, a.ID, old, next, keep); err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(db, "owner", old); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password after change: %v", err)
	}
	if _, err := Authenticate(db, "owner", next); err != nil {
		t.Fatalf("new password after change: %v", err)
	}
	if n := sessionCount(t, db, a.ID); n != 1 {
		t.Fatalf("sessions after change = %d, want 1", n)
	}
	if uid, ok, _ := LookupSession(db, keep, t0); !ok || uid != a.ID {
		t.Fatal("the changing session was ended")
	}
}

func TestResetPassword(t *testing.T) {
	db := openDB(t)
	old := testPassword(t)
	a, err := CreateFirstAdmin(db, "owner", old, t0)
	if err != nil {
		t.Fatal(err)
	}
	newSession(t, db, a.ID)
	newSession(t, db, a.ID)
	next := testPassword(t)
	if err := ResetPassword(db, "OWNER", next); err != nil {
		t.Fatal(err)
	}
	if _, err := Authenticate(db, "owner", next); err != nil {
		t.Fatalf("new password after reset: %v", err)
	}
	if _, err := Authenticate(db, "owner", old); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password after reset: %v", err)
	}
	if n := sessionCount(t, db, a.ID); n != 0 {
		t.Fatalf("sessions after reset = %d, want 0", n)
	}
	if err := ResetPassword(db, "nobody", testPassword(t)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResetPassword(unknown) = %v, want ErrNotFound", err)
	}
	var ve ValidationError
	if err := ResetPassword(db, "owner", "short"); !errors.As(err, &ve) {
		t.Fatalf("short password: %v, want ValidationError", err)
	}
}

func TestAdminTwoFactorFlag(t *testing.T) {
	db := openDB(t)
	plain := insertUser(t, db, "plain")
	withTotp := insertUser(t, db, "totp")
	withKey := insertUser(t, db, "key")
	codesOnly := insertUser(t, db, "codes")
	if _, err := db.Exec("INSERT INTO totp_factors (user_id, secret_sealed, last_step, created_at) VALUES (?, 'v1:x', 0, 1)", withTotp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO passkeys (user_id, name, credential_id, public_key, sign_count, transports, backup_eligible, created_at)
		VALUES (?, 'Phone', x'aa', x'bb', 0, '[]', 0, 1)`, withKey); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRecoveryCodes(db, codesOnly); err != nil {
		t.Fatal(err)
	}
	want := map[int64]bool{plain: false, withTotp: true, withKey: true, codesOnly: false}
	list, err := ListAdmins(db)
	if err != nil || len(list) != 4 {
		t.Fatalf("ListAdmins = %+v, %v", list, err)
	}
	for _, a := range list {
		if a.TwoFactor != want[a.ID] {
			t.Errorf("ListAdmins %s two_factor = %v, want %v", a.Username, a.TwoFactor, want[a.ID])
		}
		got, err := GetAdmin(db, a.ID)
		if err != nil || got.TwoFactor != want[a.ID] {
			t.Errorf("GetAdmin %s two_factor = %v (err %v), want %v", a.Username, got.TwoFactor, err, want[a.ID])
		}
	}
	body, _ := json.Marshal(list[1])
	if !strings.Contains(string(body), `"two_factor":true`) {
		t.Fatalf("Admin JSON = %s, want two_factor", body)
	}
}

func TestCheckPassword(t *testing.T) {
	db := openDB(t)
	pw := testPassword(t)
	a, err := CreateFirstAdmin(db, "owner", pw, t0)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword(db, a.ID, pw); err != nil {
		t.Fatalf("right password: %v", err)
	}
	if err := CheckPassword(db, a.ID, pw+"x"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password: %v, want ErrWrongPassword", err)
	}
	if err := CheckPassword(db, a.ID+1, pw); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown admin: %v, want ErrNotFound", err)
	}
}
