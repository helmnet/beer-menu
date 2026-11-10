package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	rand.Read(k)
	return k
}

func newBox(t *testing.T, key []byte) *Box {
	t.Helper()
	b, err := NewBox(key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseKey(t *testing.T) {
	key := randomKey(t)
	enc := base64.StdEncoding.EncodeToString(key)
	got, err := ParseKey(" \n" + enc + "\t\n")
	if err != nil || string(got) != string(key) {
		t.Fatalf("ParseKey(valid) = %x (err %v)", got, err)
	}
	sized := func(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }
	for name, in := range map[string]string{
		"empty":      "",
		"blank":      "  \n",
		"not base64": "not*base64*" + rand.Text(),
		"url base64": strings.NewReplacer("+", "-", "/", "_").Replace(base64.StdEncoding.EncodeToString(append([]byte{0xfb, 0xff}, make([]byte, 30)...))),
		"16 bytes":   sized(16),
		"31 bytes":   sized(31),
		"33 bytes":   sized(33),
	} {
		_, err := ParseKey(in)
		if err == nil {
			t.Errorf("%s: ParseKey accepted it", name)
			continue
		}
		if s := strings.TrimSpace(in); s != "" && strings.Contains(err.Error(), s) {
			t.Errorf("%s: error %q contains the input", name, err)
		}
	}
}

func TestNewBoxRefusesWrongLength(t *testing.T) {
	if _, err := NewBox(make([]byte, 16)); err == nil {
		t.Fatal("NewBox accepted a 16-byte key")
	}
}

func TestSealOpen(t *testing.T) {
	box := newBox(t, randomKey(t))
	plain := "fake-key-" + rand.Text()
	a, err := box.Seal(plain, "brewfather.api_key")
	if err != nil {
		t.Fatal(err)
	}
	b, err := box.Seal(plain, "brewfather.api_key")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two seals of the same plaintext are equal; nonce not random")
	}
	for _, s := range []string{a, b} {
		if !strings.HasPrefix(s, "v1:") {
			t.Fatalf("sealed %q lacks v1: prefix", s)
		}
		if strings.Contains(s, plain) || strings.Contains(s, base64.StdEncoding.EncodeToString([]byte(plain))) {
			t.Fatal("sealed value contains the plaintext")
		}
		got, err := box.Open(s, "brewfather.api_key")
		if err != nil || got != plain {
			t.Fatalf("Open = %q (err %v), want the plaintext", got, err)
		}
	}
}

func TestOpenRefuses(t *testing.T) {
	box := newBox(t, randomKey(t))
	plain := "fake-key-" + rand.Text()
	sealed, err := box.Seal(plain, "brewfather.api_key")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, "v1:"))
	flipped := append([]byte(nil), raw...)
	flipped[len(flipped)-1] ^= 1
	cases := map[string]struct {
		box             *Box
		sealed, purpose string
	}{
		"other key":     {newBox(t, randomKey(t)), sealed, "brewfather.api_key"},
		"other purpose": {box, sealed, "other.purpose"},
		"flipped byte":  {box, "v1:" + base64.StdEncoding.EncodeToString(flipped), "brewfather.api_key"},
		"truncated":     {box, "v1:" + base64.StdEncoding.EncodeToString(raw[:8]), "brewfather.api_key"},
		"empty body":    {box, "v1:", "brewfather.api_key"},
		"v2 prefix":     {box, "v2:" + strings.TrimPrefix(sealed, "v1:"), "brewfather.api_key"},
		"no prefix":     {box, strings.TrimPrefix(sealed, "v1:"), "brewfather.api_key"},
		"not base64":    {box, "v1:***", "brewfather.api_key"},
	}
	for name, c := range cases {
		got, err := c.box.Open(c.sealed, c.purpose)
		if !errors.Is(err, ErrCannotOpen) || got != "" {
			t.Errorf("%s: Open = %q, %v; want ErrCannotOpen", name, got, err)
		}
		if err != nil && (strings.Contains(err.Error(), plain) || strings.Contains(err.Error(), c.sealed)) {
			t.Errorf("%s: error %q leaks input", name, err)
		}
	}
}
