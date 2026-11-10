// Package secrets seals third-party credentials, such as the Brewfather API key, for storage
// in the database with AES-256-GCM. The 32-byte key comes from the environment or a Docker
// secret and never touches the database, so a copy of the database file or a backup alone
// cannot reveal a sealed value.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

// keySize is the encryption key length in bytes (AES-256).
const keySize = 32

const version = "v1:"

// ErrCannotOpen means a sealed value could not be opened: wrong key, wrong purpose, tampered,
// truncated or of an unknown version. It never says which, nor includes the value.
var ErrCannotOpen = errors.New("sealed value cannot be opened")

// ParseKey decodes s, standard base64 of exactly keySize bytes with surrounding whitespace
// ignored. Its errors never contain s.
func ParseKey(s string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, errors.New("key is not standard base64")
	}
	if len(key) != keySize {
		return nil, errors.New("key must be 32 bytes, base64 encoded (openssl rand -base64 32)")
	}
	return key, nil
}

// Box seals and opens values with one key. It is safe for concurrent use.
type Box struct{ aead cipher.AEAD }

// NewBox returns a Box for key, which must be keySize bytes.
func NewBox(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, errors.New("key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plain and returns "v1:" + base64(nonce ‖ ciphertext), with a random 12-byte
// nonce. purpose is bound as additional data, so the value opens only with the same purpose.
func (b *Box) Seal(plain, purpose string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), []byte(purpose))
	return version + base64.StdEncoding.EncodeToString(out), nil
}

// Open reverses Seal for the same key and purpose. Any failure is ErrCannotOpen.
func (b *Box) Open(sealed, purpose string) (string, error) {
	body, ok := strings.CutPrefix(sealed, version)
	if !ok {
		return "", ErrCannotOpen
	}
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil || len(raw) < b.aead.NonceSize() {
		return "", ErrCannotOpen
	}
	n := b.aead.NonceSize()
	plain, err := b.aead.Open(nil, raw[:n], raw[n:], []byte(purpose))
	if err != nil {
		return "", ErrCannotOpen
	}
	return string(plain), nil
}
