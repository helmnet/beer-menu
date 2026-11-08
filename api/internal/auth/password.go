// Package auth holds the authentication rules and their SQL: password hashing, sessions, API
// tokens, admins and the sign-in rate limiter. It knows nothing about HTTP, so the API
// handlers and the reset-password command share one implementation.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters (research R1): the OWASP minimum, m=19 MiB, t=2, p=1.
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	saltLen      = 16
	keyLen       = 32
)

// ErrBusy means every password-hash slot stayed taken for the whole wait. Callers answer 503.
var ErrBusy = errors.New("too many password checks running")

// ValidationError is input that breaks a rule. Its text is a user-facing message that is safe
// to return to the client; it never contains the input.
type ValidationError string

func (e ValidationError) Error() string { return string(e) }

// Each argon2id call needs 19 MiB, so at most 4 run at once to bound memory (research R2).
var (
	slots    = make(chan struct{}, 4)
	slotWait = 5 * time.Second
)

var verifies atomic.Int64

// dummyHash is verified against for unknown usernames, so they cost the same as a wrong
// password (research R3). Made on first use, not at init, so the healthcheck and
// reset-password processes never pay a 19 MiB hash they do not need.
var dummyHash = sync.OnceValue(func() string {
	h, err := hashPassword(rand.Text())
	if err != nil {
		panic(err)
	}
	return h
})

var usernameRule = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func acquire() error {
	timer := time.NewTimer(slotWait)
	defer timer.Stop()
	select {
	case slots <- struct{}{}:
		return nil
	case <-timer.C:
		return ErrBusy
	}
}

func release() { <-slots }

// HashPassword returns an argon2id PHC string for password with a fresh random salt:
// "$argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>". It returns ErrBusy when no hash slot frees
// up in time. It does not validate the password; call ValidatePassword first.
func HashPassword(password string) (string, error) {
	if err := acquire(); err != nil {
		return "", err
	}
	defer release()
	return hashPassword(password)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches the argon2id PHC string encoded, using
// the parameters stored in encoded and a constant-time compare. It returns an error for a
// malformed hash, and ErrBusy when no hash slot frees up in time.
func VerifyPassword(encoded, password string) (bool, error) {
	if err := acquire(); err != nil {
		return false, err
	}
	defer release()
	return verifyPassword(encoded, password)
}

func verifyPassword(encoded, password string) (bool, error) {
	verifies.Add(1)
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("password hash is not argon2id PHC format")
	}
	var version int
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("password hash has an unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil || iterations == 0 || threads == 0 {
		return false, errors.New("password hash has bad argon2 parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("password hash salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errors.New("password hash key is malformed")
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// DummyVerify runs one password verify against a fixed hash and discards the result, so a
// sign-in for an unknown username takes as long as a wrong password. The first call also makes
// the fixed hash, so the API calls it once before serving. It returns ErrBusy like
// VerifyPassword.
func DummyVerify(password string) error {
	_, err := VerifyPassword(dummyHash(), password)
	return err
}

// VerifyCount returns how many password verifies (real or dummy) this process has run. Tests
// use it to check that failure paths do the same work.
func VerifyCount() int64 { return verifies.Load() }

// ValidatePassword returns a ValidationError unless password has 12 to 256 Unicode code
// points. Passwords are never trimmed and have no other rules (FR-007).
func ValidatePassword(password string) error {
	if n := utf8.RuneCountInString(password); n < 12 || n > 256 {
		return ValidationError("Password must be 12 to 256 characters.")
	}
	return nil
}

// ValidateUsername trims surrounding spaces from username and returns it, or a
// ValidationError unless the result is 1 to 64 ASCII letters, digits, ".", "_" or "-"
// (FR-006).
func ValidateUsername(username string) (string, error) {
	u := strings.TrimSpace(username)
	if !usernameRule.MatchString(u) {
		return "", ValidationError("Username must be 1 to 64 letters, digits, dots, underscores or hyphens.")
	}
	return u, nil
}
