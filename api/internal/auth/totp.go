package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP parameters (research R3): the defaults every authenticator app supports.
const (
	totpSecretLen = 20
	totpStep      = 30
	totpDigits    = 6
)

var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret returns a new random 20-byte authenticator secret.
func NewSecret() ([]byte, error) {
	s := make([]byte, totpSecretLen)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	return s, nil
}

// EncodeSecret returns secret as base32 without padding, the form authenticator apps take.
func EncodeSecret(secret []byte) string { return base32NoPad.EncodeToString(secret) }

// URI returns the otpauth:// URI for secret that the setup QR code holds:
// otpauth://totp/Beer%20Menu:<username>?secret=<base32>&issuer=Beer%20Menu.
func URI(username string, secret []byte) string {
	return "otpauth://totp/Beer%20Menu:" + url.PathEscape(username) + "?secret=" + EncodeSecret(secret) + "&issuer=Beer%20Menu"
}

// Code returns the 6-digit RFC 6238 code (HMAC-SHA-1) of secret for the 30-second step.
func Code(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, n%1_000_000)
}

// Verify reports whether code, with spaces and dashes ignored, is secret's code for the step
// at now or the one before or after it, and returns that step. A step not later than lastStep
// is refused, so a code works once and an earlier code never works after a later one (FR-009,
// FR-010). Codes are compared in constant time.
func Verify(secret []byte, code string, now time.Time, lastStep int64) (step int64, ok bool) {
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	if len(code) != totpDigits || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	cur := now.Unix() / totpStep
	for s := cur - 1; s <= cur+1; s++ {
		if s > lastStep && hmac.Equal([]byte(Code(secret, s)), []byte(code)) {
			return s, true
		}
	}
	return 0, false
}
