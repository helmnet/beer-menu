package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfcSecret is the RFC 6238 appendix B SHA-1 seed.
var rfcSecret = []byte("12345678901234567890")

func TestTotpRFC6238Vectors(t *testing.T) {
	// Appendix B SHA-1 values, truncated to their last 6 digits.
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"},
	}
	for _, v := range vectors {
		if got := Code(rfcSecret, v.unix/30); got != v.code {
			t.Errorf("Code at %d = %s, want %s", v.unix, got, v.code)
		}
		step, ok := Verify(rfcSecret, v.code, time.Unix(v.unix, 0), 0)
		if !ok || step != v.unix/30 {
			t.Errorf("Verify at %d = %d %v, want step %d", v.unix, step, ok, v.unix/30)
		}
	}
}

func TestTotpVerifyWindowAndReplay(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cur := now.Unix() / 30
	for _, d := range []int64{-1, 0, 1} {
		step, ok := Verify(rfcSecret, Code(rfcSecret, cur+d), now, 0)
		if !ok || step != cur+d {
			t.Errorf("step now%+d = %d %v, want accepted", d, step, ok)
		}
	}
	for _, d := range []int64{-2, 2} {
		if _, ok := Verify(rfcSecret, Code(rfcSecret, cur+d), now, 0); ok {
			t.Errorf("step now%+d accepted", d)
		}
	}
	// A code is refused once its step, or a later one, was used.
	if _, ok := Verify(rfcSecret, Code(rfcSecret, cur), now, cur); ok {
		t.Error("replayed step accepted")
	}
	if _, ok := Verify(rfcSecret, Code(rfcSecret, cur-1), now, cur); ok {
		t.Error("earlier step accepted after a later one was used")
	}
	if step, ok := Verify(rfcSecret, Code(rfcSecret, cur+1), now, cur); !ok || step != cur+1 {
		t.Errorf("later step after use = %d %v, want accepted", step, ok)
	}
}

func TestTotpVerifyInput(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	code := Code(rfcSecret, now.Unix()/30)
	for _, in := range []string{code[:3] + " " + code[3:], code[:3] + "-" + code[3:], " " + code + " "} {
		if _, ok := Verify(rfcSecret, in, now, 0); !ok {
			t.Errorf("%q refused, want spaces and dashes ignored", in)
		}
	}
	for _, in := range []string{"", code[:5], code + "0", "12345a", "١٢٣٤٥٦", "+12345"} {
		if _, ok := Verify(rfcSecret, in, now, 0); ok {
			t.Errorf("%q accepted", in)
		}
	}
	if _, ok := Verify([]byte("another secret 12345"), code, now, 0); ok {
		t.Error("code accepted for another secret")
	}
}

func TestTotpSecretAndURI(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSecret()
	if len(a) != 20 || string(a) == string(b) {
		t.Fatalf("secrets %x %x, want two different 20-byte values", a, b)
	}
	enc := EncodeSecret(a)
	if strings.Contains(enc, "=") || len(enc) != 32 {
		t.Fatalf("EncodeSecret = %q, want 32 base32 characters without padding", enc)
	}
	if raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(enc); err != nil || string(raw) != string(a) {
		t.Fatalf("EncodeSecret does not round-trip: %v", err)
	}
	want := "otpauth://totp/Beer%20Menu:jo%20e?secret=" + enc + "&issuer=Beer%20Menu"
	if got := URI("jo e", a); got != want {
		t.Fatalf("URI = %s, want %s", got, want)
	}
}
