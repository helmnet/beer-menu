# Research: Two-Factor Sign-In for Admins

Feature: [spec.md](./spec.md) · Plan: [plan.md](./plan.md)

## R1. Passkeys (WebAuthn) on the server: `go-webauthn/webauthn`

**Decision**: add `github.com/go-webauthn/webauthn` v0.18.2 (released 2026-09-19). It builds the
creation and request options, and verifies the browser's attestation and assertion: challenge,
origin, RP ID hash, user-presence and user-verification flags, signature and sign count.

**Rationale**: WebAuthn verification parses CBOR and COSE keys and checks signatures over
authenticator data. Getting any of it wrong silently accepts forged sign-ins, so a widely used,
maintained library is safer than our own code. It is the de facto Go WebAuthn library (used by
Gitea, Authelia, Pocket ID). Constitution VII asks for a justified dependency; this is it.

New indirect modules it brings: `fxamacker/cbor/v2`, `go-webauthn/x`, `golang-jwt/jwt/v5`
(metadata service, unused here), `google/go-tpm` (TPM attestation, unused because we ask for no
attestation), `go-viper/mapstructure/v2`, `tinylib/msgp`, `x448/float16`. All pure Go, so the
static binary rule (II) holds. `govulncheck` runs in the security review.

**govulncheck, 2026-11-12** (T069): `govulncheck ./...` in `api/` (govulncheck v1.9.0, Go
1.27.3, vulnerability DB updated 2026-11-11) finds no vulnerability the code calls and none in
an imported package. The go.mod additions are `go-webauthn/webauthn` v0.18.2 and the indirect
`fxamacker/cbor/v2` v2.9.4, `go-viper/mapstructure/v2` v2.5.0, `go-webauthn/x` v0.3.1,
`golang-jwt/jwt/v5` v5.3.1, `google/go-tpm` v0.9.8, `philhofer/fwd` v1.2.0 (pulled in by
`tinylib/msgp`), `tinylib/msgp` v1.6.4 and `x448/float16` v0.8.4; none has a known
vulnerability. The TOTP code is our own on the standard library, so it adds no module. The one
module-level finding, GO-2026-5932 (`golang.org/x/crypto/openpgp` is unmaintained and unsafe),
is in `golang.org/x/crypto` v0.58.0, which predates this feature; we never import `openpgp`,
and the fix is only in an untagged pseudo-version, so the module stays until a tagged release.

**Alternatives considered**:
- Own WebAuthn verification on `crypto/ecdsa` and a hand-written CBOR reader: rejected, too much
  security-critical parsing to own.
- `duo-labs/webauthn`: the old name of the same project, no longer maintained.

## R2. Passkeys in the browser: native JSON helpers, no library

**Decision**: call `navigator.credentials.create()` and `.get()` directly, converting with
`PublicKeyCredential.parseCreationOptionsFromJSON`, `parseRequestOptionsFromJSON` and
`credential.toJSON()`. Passkeys are offered only when all of these hold: `window.isSecureContext`,
`PublicKeyCredential` exists with the two parse functions, and the API reports passkeys
available (R5).

**Rationale**: these helpers ship in every current engine (Chrome 129+, Firefox 119+, Safari
18.4+), so `@simplewebauthn/browser` would only add the base64url conversion they already do.
This includes the installed PWA, which runs in the same engine as the browser.

**Biometrics**: there is no separate web biometric API. Setting `userVerification: "required"`
makes the device ask for fingerprint, face, Windows Hello or the device PIN; the server sees only
the UV flag in signed authenticator data (FR-014).

**Alternatives considered**: `@simplewebauthn/browser` (rejected: duplicates built-ins).

## R3. Authenticator codes (TOTP): own code on the standard library

**Decision**: implement RFC 6238 TOTP in `internal/auth/totp.go` with `crypto/hmac`,
`crypto/sha1` and `encoding/base32`: 20-byte random secret, SHA-1, 6 digits, 30-second step,
accept steps `now-1 .. now+1` (FR-009). Store the last accepted step and accept only a later one
(FR-010), which also blocks reuse of a code across the two steps of a ±1 window. Compare codes in
constant time. Tests use the RFC 6238 appendix B vectors.

**Rationale**: TOTP is about 30 lines; the parameters are the ones every authenticator app
supports by default. Constitution VII prefers the standard library over `pquerna/otp`.

**Alternatives considered**: `github.com/pquerna/otp` (rejected: a dependency for 30 lines);
SHA-256 or 8 digits (rejected: some apps ignore those parameters and show wrong codes).

## R4. QR code for authenticator setup: `uqr` in the web app

**Decision**: add `uqr` 0.1.3 (no dependencies). The setup screen calls its `encode()` for the
`otpauth://totp/...` URI and draws the module matrix as one SVG `<path>` in a `QrCode`
component. The secret is also shown as text, in groups of 4, for typing in by hand.

**Rationale**: rendering as React SVG elements needs no `innerHTML`, no style attributes and no
image from the API, so the CSP is unchanged. Encoding QR (Reed–Solomon, masks) is not worth
owning.

**Alternatives considered**: server-side PNG via a Go QR library (rejected: second dependency,
and the secret would travel as an image the API must build); `qrcode` npm (rejected: larger,
has dependencies).

## R5. Relying party: new `BEER_MENU_PUBLIC_URL`

**Decision**: a new optional env var `BEER_MENU_PUBLIC_URL`, for example
`https://beer.example.lan`. Its origin is the only allowed WebAuthn origin, and its host name is
the RP ID. When unset, passkeys are off: `GET /api/2fa` reports `passkeys_available: false`
and the passkey routes answer 409 `passkeys_unavailable`. When set, it MUST be `https://` with a
host name (not an IP address), an optional port, and no path beyond `/`, query or fragment;
`http://localhost` is also allowed, for local development. Any other value refuses startup with
an error that does not echo the value, so a typo shows at once instead of silently turning
passkeys off.

The configured origin is returned as `passkeys_origin` in `GET /api/2fa` and in the 202
`second_factor` reply. The web app offers passkeys only when `location.origin` equals it, so an
admin who opens the app under another host name (an alias or the IP address) is told passkeys
are not available here instead of getting a prompt that always fails (FR-016). The check runs
in the browser because same-origin `GET` requests carry no `Origin` header, and `Host` comes
through two proxies.

**Rationale**: WebAuthn needs a fixed RP ID; deriving it from the request `Host` would let a
different host name register credentials that never work again under the real one. The app runs
behind NPM, so it cannot know its public name otherwise. No host name is committed (V).

**Alternatives considered**: derive from `Origin`/`Host` per request (rejected, above); require
it at startup (rejected: breaks existing deployments that do not want passkeys).

## R6. Authenticator secrets at rest

**Decision**: seal the TOTP secret with the existing `secrets.Box` (AES-256-GCM,
`BEER_MENU_SECRET_KEY`), purpose `"totp:<user id>"`, so a sealed value cannot be moved to another
admin's row. Without a key, authenticator apps are off (`totp_available: false`, routes 409
`totp_unavailable`), as Brewfather already is. Passkeys and recovery codes need no key.

**Rationale**: FR-011 and constitution V require encryption with the key outside the database.
The pending secret during setup is sealed the same way (R8).

## R7. Recovery codes

**Decision**: 10 codes, each 10 random base32 characters (50 bits), shown as `xxxxx-xxxxx`.
Stored as SHA-256 of the normalized code (dashes and spaces removed, lowercased). Using a code
sets `used_at`; a new set deletes the old rows in the same transaction (FR-019).

**Rationale**: 50 bits of randomness, behind the 5-per-15-minute limit, cannot be guessed, so a
slow hash adds nothing (same reasoning as API tokens in feature 2). SHA-256 lets lookup be one
indexed query.

## R8. Server-held state between steps

**Decision**: one new table, `auth_challenges`, holds every short-lived step, keyed by the
SHA-256 of a random 32-byte ID:

| `kind` | Meaning | Carried by |
| --- | --- | --- |
| `second_factor` | correct password given, second factor owed (FR-002) | `bm_pending` cookie |
| `enroll` | correct password given, 2FA required, no factor yet (FR-034) | `bm_pending` cookie |

Columns also hold an in-flight WebAuthn session (challenge JSON) and an in-flight TOTP setup
secret (sealed). For a signed-in admin, the same two columns live on a row keyed by the session
hash, kind `setup`. Rows expire 5 minutes after creation, are single use, are deleted with their
admin (FK cascade), and are deleted when the admin's factors are cleared or removed. Expired rows
are deleted on each new insert.

The `bm_pending` cookie: HttpOnly, SameSite=Strict, Path=/api, Secure unless
`BEER_MENU_COOKIE_SECURE=false`, Max-Age 300. Ending a pending step clears it.

**Rationale**: matches how sessions already work (hashed ID in SQLite, cookie holds the raw ID),
survives nothing it should not, and one table covers all three needs. Being cookie-bound and
SameSite=Strict ties it to the browser that gave the password (FR-002).

**Alternatives considered**: in-memory map (rejected: needs separate invalidation when factors
are cleared or the admin is removed, which the FK cascade gives for free); signed stateless
token (rejected: cannot be single-use or revoked).

## R9. Proving identity before sensitive changes ("confirm")

**Decision**: `POST /api/2fa/confirm` takes the current password and, if the admin has any
factor, one second factor (code, recovery code or passkey assertion). Success stamps the session
with `confirmed_at`. Adding or removing a factor, new recovery codes, turning 2FA off,
changing the "require" setting, clearing another admin's factors, adding or removing an
admin, creating an API token and changing your own password all need `confirmed_at`
within the last 5 minutes, else 403 `confirm_required`. Renaming a passkey does not.

This is slightly stricter than FR-023 for adding a factor (it also asks for an existing factor):
a stolen session plus a known password must not be able to add the thief's own passkey.

Failed confirms count toward the per-username limiter, like a wrong current password does today.

**Rationale**: one proof step instead of password and factor fields on six endpoints. The web
app asks once and then runs the change.

**Alternatives considered**: password and factor on every sensitive request (rejected: passkey
assertions would need an options round-trip per action).

## R10. Required 2FA and the limited state

**Decision**: settings key `require_2fa` (`"1"`/`"0"`, missing reads as off) in the existing
`settings` table, written only by `PUT /api/2fa/required` (session + confirm, caller must have a
factor, FR-033). It is not part of `GET`/`PATCH /api/settings`, which tokens may call.

A new access level `AccessEnroll` (`x-access: enroll`) allows a full session **or** a
`bm_pending` of kind `enroll`. Only the factor setup routes use it. With an `enroll` pending, the
password counts as just confirmed. The first confirmed factor deletes the pending row, starts a
session, sets its cookie and returns the admin with the recovery codes.

**Rationale**: reuses the setup routes instead of a parallel enroll API; every other route stays
`AccessAdmin`, which an enroll pending never satisfies, so FR-034's "nothing else" holds by
construction and is checked by the existing route-table walk test. Constitution V (1.4.0)
allows exactly this exception: the factor setup routes may accept an `enroll` pending, and no
other route may.

## R11. Sign-in responses

**Decision**: `POST /api/session` keeps 200 + Admin for admins without 2FA (backward compatible).
With 2FA it answers 202 `{"next":"second_factor","methods":[...]}`; in the required-but-none case
202 `{"next":"enroll"}`. Both set `bm_pending`, and neither sets the session cookie. Wrong
passwords keep the same 401 whatever the 2FA state (FR-004). The second step is
`POST /api/session/totp`, `/recovery` or `/passkey` (with `/passkey/options` first); success is
200 + Admin + session cookie, exactly like today's sign-in.

These second-step routes are `AccessPublic` in the route table (no session exists yet) but each
requires a valid `bm_pending` of kind `second_factor`; without one they answer 401
`unauthenticated`. They count failures for the pending admin's username in the existing
limiter. Constitution V (1.4.0, amended by this feature) names these later sign-in steps as
public, on condition that they act only on a pending sign-in created by a correct password;
they sit behind the same cross-site refusal.

`DELETE /api/session/pending` (`AccessPublic`) cancels an unfinished sign-in: it deletes the
challenge named by `bm_pending`, if any, clears the cookie and answers 204 whether or not one
existed. The login Cancel button and the enroll view's Sign out call it. Without it, a pending
step would stay usable in that browser for up to 5 minutes after the admin walked away, and an
`enroll` pending would let the next person at that browser finish setup and get a session.

## R12. Operator reset

**Decision**: `beer-menu-api reset-2fa <username>` beside `reset-password`, sharing its file
layout. It deletes the admin's TOTP row, passkeys, recovery codes, pending steps and sessions in
one transaction and logs username and time (FR-029). It does not touch `require_2fa`; the admin
then enrolls at next sign-in if it is on.

## R13. Testing passkeys without a device

**Decision**: a test-only software authenticator in `internal/auth/authtest` (ES256 key,
`"none"` attestation, CBOR via the already-indirect `fxamacker/cbor/v2`) produces registration
and assertion responses for Go handler tests, including bad origin, bad RP ID, missing UV flag,
wrong user, replayed challenge and unknown credential. Web tests stub `navigator.credentials`.
A real device run is a manual quickstart step (scenario 4).

## R14. Real-device quickstart (T070), 2026-11-13

The owner ran the quickstart on real devices against production (`v0.10.x` plus PR #38).

- **Authenticator app (TOTP):** worked first time.
- **Passkeys (1Password):** the first two attempts to add one failed with "Error validating the
  authenticator response". In go-webauthn v0.18.2 that message comes only from the RP ID hash,
  user presence and user verification checks. The third attempt worked, and the cause is not
  known. 1Password asked for a fingerprint even with the vault unlocked, so a missing
  user-verification flag is unlikely. PR #39 now logs the library's detail line for these
  checks; if it happens again, read the log before changing the user-verification setting.
- **Passkey sign-in:** works on the laptop, and on the phone in the installed app and in a
  browser tab.
- **Change after testing:** when passkeys are the only second factor, the sign-in step now
  opens the passkey prompt by itself (PR #40). The owner checked it works.
