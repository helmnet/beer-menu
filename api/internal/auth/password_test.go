package auth

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

// testPassword returns a random 24-character password so no fixed password is in the repo.
func testPassword(t testing.TB) string {
	t.Helper()
	return rand.Text()[:24]
}

func TestHashVerifyRoundTrip(t *testing.T) {
	pw := testPassword(t)
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash %q lacks the PHC prefix", h)
	}
	if strings.Contains(h, pw) {
		t.Fatal("hash contains the password")
	}
	ok, err := VerifyPassword(h, pw)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword(right) = %v, %v; want true", ok, err)
	}
	ok, err = VerifyPassword(h, pw+"x")
	if err != nil || ok {
		t.Fatalf("VerifyPassword(wrong) = %v, %v; want false", ok, err)
	}
	h2, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if h2 == h {
		t.Fatal("two hashes of one password are equal; salt missing")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2id$v=19$m=x,t=2,p=1$a$b", "$bcrypt$v=19$m=19456,t=2,p=1$AAAA$AAAA"} {
		if _, err := VerifyPassword(h, "whatever-password"); err == nil {
			t.Errorf("VerifyPassword(%q) = nil error, want error", h)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := map[string]bool{
		strings.Repeat("a", 12):  true,
		strings.Repeat("a", 256): true,
		strings.Repeat("é", 12):  true, // 24 bytes, 12 code points
		strings.Repeat("é", 256): true,
		strings.Repeat("a", 11):  false,
		strings.Repeat("é", 11):  false, // 22 bytes, still 11 code points
		strings.Repeat("a", 257): false,
		"":                       false,
	}
	for pw, want := range cases {
		err := ValidatePassword(pw)
		if (err == nil) != want {
			t.Errorf("ValidatePassword(%d runes) = %v, want ok %v", len([]rune(pw)), err, want)
		}
		var ve ValidationError
		if err != nil && !errors.As(err, &ve) {
			t.Errorf("ValidatePassword error %v is not a ValidationError", err)
		}
	}
	if err := ValidatePassword("  " + strings.Repeat("a", 10)); err != nil {
		t.Errorf("spaces count as characters: %v", err)
	}
}

func TestValidateUsername(t *testing.T) {
	ok := map[string]string{
		"owner":                       "owner",
		"  owner  ":                   "owner",
		"a.b_c-1":                     "a.b_c-1",
		strings.Repeat("a", 64):       strings.Repeat("a", 64),
		" " + strings.Repeat("A", 64): strings.Repeat("A", 64),
	}
	for in, want := range ok {
		got, err := ValidateUsername(in)
		if err != nil || got != want {
			t.Errorf("ValidateUsername(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", strings.Repeat("a", 65), "a b", "ü", "a/b"} {
		if _, err := ValidateUsername(in); err == nil {
			t.Errorf("ValidateUsername(%q) = nil error, want error", in)
		}
	}
}

func TestHashSemaphore(t *testing.T) {
	if cap(slots) != 4 {
		t.Fatalf("semaphore size = %d, want 4", cap(slots))
	}
	oldWait := slotWait
	slotWait = 50 * time.Millisecond
	t.Cleanup(func() { slotWait = oldWait })
	for range cap(slots) {
		slots <- struct{}{}
	}
	pw := testPassword(t)
	if _, err := HashPassword(pw); !errors.Is(err, ErrBusy) {
		t.Fatalf("HashPassword with all slots taken = %v, want ErrBusy", err)
	}
	if _, err := VerifyPassword(dummyHash(), pw); !errors.Is(err, ErrBusy) {
		t.Fatalf("VerifyPassword with all slots taken = %v, want ErrBusy", err)
	}
	if err := DummyVerify(pw); !errors.Is(err, ErrBusy) {
		t.Fatalf("DummyVerify with all slots taken = %v, want ErrBusy", err)
	}
	<-slots
	if _, err := HashPassword(pw); err != nil {
		t.Fatalf("HashPassword with a free slot = %v", err)
	}
	for range cap(slots) - 1 {
		<-slots
	}
}

func TestDummyVerifyCounts(t *testing.T) {
	before := VerifyCount()
	if err := DummyVerify(testPassword(t)); err != nil {
		t.Fatal(err)
	}
	h, err := HashPassword(testPassword(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPassword(h, "wrong-password-here"); err != nil {
		t.Fatal(err)
	}
	if got := VerifyCount() - before; got != 2 {
		t.Fatalf("VerifyCount grew by %d, want 2", got)
	}
}

// BenchmarkVerifyPassword measures one argon2id verify, to check the cost on the target machine.
func BenchmarkVerifyPassword(b *testing.B) {
	pw := testPassword(b)
	h, err := HashPassword(pw)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := VerifyPassword(h, pw); err != nil {
			b.Fatal(err)
		}
	}
}
