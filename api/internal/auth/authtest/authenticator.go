// Package authtest is a software passkey authenticator for tests (research R13). It answers
// WebAuthn creation and request options with the JSON a browser's credential.toJSON() would
// send, so handler tests can register and use passkeys without a device. Only tests import it.
package authtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// Authenticator is one software passkey: an ES256 key with "none" attestation. Its fields say
// what the next Register or Assert claims, so tests can make each check fail on purpose.
type Authenticator struct {
	// Origin is the origin put in the client data.
	Origin string
	// RPID is the relying party ID whose hash is put in the authenticator data.
	RPID string
	// UV sets the user-verified flag.
	UV bool
	// BackupEligible sets the backup-eligible flag.
	BackupEligible bool
	// SignCount is the counter put in the next assertion; Assert does not change it.
	SignCount uint32
	// CredentialID is the credential's ID, random from New.
	CredentialID []byte

	key        *ecdsa.PrivateKey
	userHandle []byte
}

// New returns an authenticator for origin and rpID with user verification on, a fresh key and
// a random 16-byte credential ID.
func New(t testing.TB, origin, rpID string) *Authenticator {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	rand.Read(id)
	return &Authenticator{Origin: origin, RPID: rpID, UV: true, CredentialID: id, key: key}
}

// options is the part of {"publicKey": ...} creation or request options the authenticator reads.
type options struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		User      struct {
			ID protocol.URLEncodedBase64 `json:"id"`
		} `json:"user"`
	} `json:"publicKey"`
}

func parseOptions(t testing.TB, raw []byte) options {
	t.Helper()
	var o options
	if err := json.Unmarshal(raw, &o); err != nil || o.PublicKey.Challenge == "" {
		t.Fatalf("authtest: bad options %s: %v", raw, err)
	}
	return o
}

// Register answers creation options (JSON {"publicKey": ...}) with a registration response
// shaped like PublicKeyCredential.toJSON(), and remembers the user handle for later assertions.
func (a *Authenticator) Register(t testing.TB, creationOptions []byte) []byte {
	t.Helper()
	o := parseOptions(t, creationOptions)
	a.userHandle = o.PublicKey.User.ID
	point, err := a.key.PublicKey.Bytes() // 0x04 || X || Y
	if err != nil {
		t.Fatal(err)
	}
	cose, err := webauthncbor.Marshal(map[int64]any{
		1:  int64(webauthncose.EllipticKey),
		3:  int64(webauthncose.AlgES256),
		-1: int64(webauthncose.P256),
		-2: point[1:33],
		-3: point[33:],
	})
	if err != nil {
		t.Fatal(err)
	}
	attested := make([]byte, 16) // zero AAGUID
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(a.CredentialID)))
	attested = append(attested, a.CredentialID...)
	attested = append(attested, cose...)
	authData := a.authData(protocol.FlagAttestedCredentialData, 0, attested)
	attObj, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	if err != nil {
		t.Fatal(err)
	}
	return a.response(t, map[string]any{
		"attestationObject":  b64(attObj),
		"clientDataJSON":     b64(a.clientData(t, "webauthn.create", o.PublicKey.Challenge)),
		"transports":         []string{"internal"},
		"publicKeyAlgorithm": int64(webauthncose.AlgES256),
	})
}

// Assert answers request options (JSON {"publicKey": ...}) with an assertion response shaped
// like PublicKeyCredential.toJSON(), signed with this authenticator's key and SignCount.
func (a *Authenticator) Assert(t testing.TB, requestOptions []byte) []byte {
	t.Helper()
	o := parseOptions(t, requestOptions)
	authData := a.authData(0, a.SignCount, nil)
	clientData := a.clientData(t, "webauthn.get", o.PublicKey.Challenge)
	hash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), hash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return a.response(t, map[string]any{
		"authenticatorData": b64(authData),
		"clientDataJSON":    b64(clientData),
		"signature":         b64(sig),
		"userHandle":        b64(a.userHandle),
	})
}

func (a *Authenticator) authData(flags protocol.AuthenticatorFlags, count uint32, attested []byte) []byte {
	flags |= protocol.FlagUserPresent
	if a.UV {
		flags |= protocol.FlagUserVerified
	}
	if a.BackupEligible {
		flags |= protocol.FlagBackupEligible
	}
	rp := sha256.Sum256([]byte(a.RPID))
	data := append(rp[:], byte(flags))
	data = binary.BigEndian.AppendUint32(data, count)
	return append(data, attested...)
}

func (a *Authenticator) clientData(t testing.TB, ceremony, challenge string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"type": ceremony, "challenge": challenge, "origin": a.Origin, "crossOrigin": false})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func (a *Authenticator) response(t testing.TB, response map[string]any) []byte {
	t.Helper()
	id := b64(a.CredentialID)
	b, err := json.Marshal(map[string]any{
		"id": id, "rawId": id, "type": "public-key", "authenticatorAttachment": "platform",
		"response": response, "clientExtensionResults": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
