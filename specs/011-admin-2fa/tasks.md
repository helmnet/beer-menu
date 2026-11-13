---
description: "Task list for feature 11: two-factor sign-in for admins"
---

# Tasks: Two-Factor Sign-In for Admins

**Input**: Design documents from `specs/011-admin-2fa/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md),
[contracts/two-factor.openapi.yaml](./contracts/two-factor.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable, and every auth rule needs its
own test: unauthenticated access, tokens refused, CSRF on every browser write (including the
public second-step routes), and the sign-in rate limit. Go tests run against real SQLite files
in `t.TempDir()`, never mocks of the database. Migration 0006 gets an upgrade test on a v5
database holding data. React components with logic get Vitest tests; run Vitest with
`--maxWorkers=2`. Write each test task before its implementation task and see it fail first.

**Organization**: the migration, the challenge table, confirm, the pending cookie, the new
access level, TOTP math and recovery-code storage are foundational, because the sign-in step,
setup, recovery and the require setting all use them. Stories then run in priority order: US1
(P1), US2 (P2), US3 (P2), US4 (P3), US5 (P3).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US5)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`). 2FA rules and SQL in
  `api/internal/auth/`; handlers in `api/internal/httpapi/`; CLI in `api/cmd/beer-menu-api/`.
- Web: `web/` (Vite + React + TypeScript). Components in `web/src/components/`, pages in
  `web/src/pages/`.
- Test helpers: `api/internal/httpapi/helpers_test.go` (`newTestDeps`, `testClock`,
  `insertUser`, `insertToken`, `do`, `sessionCookie`, `wantError`);
  `api/internal/httpapi/routes_test.go` (contract `x-access` check);
  `api/internal/httpapi/auth_walk_test.go` (every route × every credential);
  `api/internal/store/migrate_test.go` (per-migration upgrade tests); `web/src/testApi.ts`
  (`stubApi` and fixtures).

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- `internal/auth` knows nothing of HTTP, so the `reset-2fa` command and the handlers share it.
- New Go dependency: only `github.com/go-webauthn/webauthn` v0.18.2. New web dependency: only
  `uqr` 0.1.3. TOTP uses the standard library only (research R3).
- No `style` attribute in markup, no `<style>` element, no `innerHTML`; the CSP stays unchanged.
- Constitution 1.4.0 (principle V amended with this feature) is the rule: the second-step and cancel routes are public only on a pending sign-in from a correct password, and an `enroll` pending is accepted on the factor setup routes and nowhere else.
- Every new route refuses API tokens (403 `forbidden`, via `AccessAdmin`/`AccessEnroll`), and
  every new write route sits behind the existing cross-site refusal in `withSecurity`.
- Error responses never echo codes, secrets, passwords or credentials, and logs never contain
  them; log the username, the event and the time.
- Fixed values (verbatim from research and data-model):
  - TOTP: "20-byte random secret, SHA-1, 6 digits, 30-second step, accept steps `now-1 .. now+1`";
    "accept only a later [step]" than `last_step`; "Compare codes in constant time". Secret
    sealed with `secrets.Box`, "purpose `totp:<user_id>`". otpauth URI:
    `otpauth://totp/Beer%20Menu:<username>?secret=...&issuer=Beer%20Menu`.
  - Recovery codes: "10 codes, each 10 random base32 characters (50 bits), shown as
    `xxxxx-xxxxx`", "Stored as SHA-256 of the normalized code (dashes and spaces removed,
    lowercased)"; JSON pattern `^[a-z2-7]{5}-[a-z2-7]{5}$`.
  - Passkey name: "1–64 code points, trimmed, no control characters".
  - Challenges: `kind` "CHECK in (`second_factor`, `enroll`, `setup`)"; "`expires_at` =
    `created_at + 300`"; single use; key is SHA-256 of a random 32-byte ID.
  - `bm_pending` cookie: "HttpOnly, SameSite=Strict, Path=/api, Secure unless
    `BEER_MENU_COOKIE_SECURE=false`, Max-Age 300".
  - Confirm window: "sensitive changes need it within 300 s"; sign-in stamps `confirmed_at = now`.
  - `require_2fa` setting: "`"1"` / `"0"`", "missing or anything else reads as off".
  - Code request bodies: `code` string, "minLength: 1, maxLength: 32", "Spaces and dashes
    ignored."; WebAuthn response bodies "Max 16 KiB".
  - Error codes: `invalid_code`, `confirm_required`, `two_factor_required`, `totp_unavailable`,
    `passkeys_unavailable`, `passkey_failed`, `already_set_up`, `no_factor`, `cannot_clear_self`, `too_many_passkeys`.

---

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 [P] Add `github.com/go-webauthn/webauthn v0.18.2` to `api/go.mod` and `api/go.sum` (`go get`, then `go mod tidy`); confirm `CGO_ENABLED=0 go build ./...` still makes a static binary
- [X] T002 [P] Add `uqr` pinned to exactly `0.1.3` to `web/package.json` dependencies and `web/package-lock.json` (`npm install --save-exact uqr@0.1.3`)
- [X] T003 Write tests in `api/cmd/beer-menu-api/startup_test.go` for `loadPublicURL`: unset gives "passkeys off" with no error; `https://taplist.example.org` gives origin `https://taplist.example.org` and RP ID `taplist.example.org`; a port is kept in the origin; `http://localhost:5173` is allowed; `http://taplist.example.org`, an IP host such as `https://192.168.1.10`, a value with a path other than `/`, a query, a fragment or no host, and an unparsable value are startup errors that do not echo the value (research R5: a bad value refuses startup, it never silently turns passkeys off)
- [X] T004 Implement `loadPublicURL(getenv)` in `api/cmd/beer-menu-api/startup.go` reading `BEER_MENU_PUBLIC_URL` per T003, and in `api/cmd/beer-menu-api/main.go` build a `*webauthn.WebAuthn` (RPDisplayName `Beer Menu`, RPID, RPOrigins = [origin]) when set, log `passkeys not configured, no public URL set` when not, and refuse startup on error (research R5)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: storage, short-lived steps, the confirm proof, the pending cookie and the enroll
access level that every story uses.

### Tests for the foundation (write first, see them fail)

- [X] T005 [P] Add `TestMigration0006FromVersion5` to `api/internal/store/migrate_test.go`: a v5 database holding a user, a session, a token and settings rows upgrades to v6; existing rows survive; `sessions.confirmed_at` is NULL on old rows; the four new tables exist with their constraints (`auth_challenges.kind` CHECK refuses `other`; `passkeys.credential_id` UNIQUE; deleting the user cascades to `totp_factors`, `passkeys`, `recovery_codes`, `auth_challenges`)
- [X] T006 [P] Write `api/internal/auth/totp_test.go`: RFC 6238 appendix B SHA-1 vectors (truncated to 6 digits); `Verify` accepts steps now-1, now, now+1 and refuses now±2; returns the matched step; refuses a step not later than `lastStep` (replay, and reuse across the ±1 window); refuses non-digit and wrong-length input after stripping spaces and dashes; `NewSecret` returns 20 bytes; `URI` matches the otpauth form with the username escaped
- [X] T007 [P] Write `api/internal/auth/challenges_test.go`: `CreateChallenge` returns a 43-char base64url ID and stores only its SHA-256; `TakeChallenge` returns the row once and never again (single use); `PeekChallenge` reads without consuming; a row older than 300 s is not found; expired rows are deleted on the next insert; kind mismatch is not found; `SetChallengeData` stores WebAuthn JSON and a sealed TOTP secret; `DeleteChallengesFor(userID)` removes every row of that admin; a `setup` row keyed by a session hash is deleted by `DeleteSession`
- [X] T008 [P] Write `api/internal/auth/twofactor_test.go` (foundation part): `HasTwoFactor` is false with none, true with a `totp_factors` row or a `passkeys` row, false with only recovery codes; `NewRecoveryCodes` returns 10 codes matching `^[a-z2-7]{5}-[a-z2-7]{5}$`, stores 10 SHA-256 rows and replaces any earlier set in one transaction; `RecoveryCodesLeft`; `ClearTwoFactor(userID)` deletes factors, codes, challenges and all sessions in one transaction; `RequireTwoFactor`/`SetRequireTwoFactor` read `"1"` as on and missing, `"0"` or anything else as off
- [X] T009 [P] Extend `api/internal/auth/sessions_test.go`: `CreateSession` stamps `confirmed_at = now`; `ConfirmSession` sets it; `SessionConfirmed(id, now)` is true within 300 s and false after; `ListAdmins` and `GetAdmin` in `api/internal/auth/users_test.go` return `TwoFactor` true only for admins with a factor
- [X] T010 [P] Write `api/internal/httpapi/guard_test.go` cases for `AccessEnroll`: a valid session passes; a `bm_pending` of kind `enroll` passes and puts `Pending` (not `AdminID` session) in the principal; kinds `second_factor` and `setup` do not pass; an expired or unknown pending is 401; an API token is 403; an `enroll` pending on an `AccessAdmin` route is 401
- [X] T011 [P] Write `api/internal/httpapi/session_test.go` cases for the password step: admin without 2FA still gets 200 + Admin + session cookie and no `bm_pending`; admin with 2FA gets 202 `{"next":"second_factor","methods":[...]}` (methods list `totp` only when a TOTP row exists, `passkey` only when passkeys exist and WebAuthn is configured, `recovery` only when unused codes remain; `passkeys_origin` present exactly when `passkey` is listed) with a `bm_pending` cookie per the conventions and no session cookie; with `require_2fa` on and no factor, 202 `{"next":"enroll"}`; a wrong password gives the identical 401 body and the same `auth.VerifyCount()` delta whether or not the admin has 2FA (FR-004); a cross-site request is 403. `DELETE /api/session/pending`: with a `second_factor` or `enroll` pending it deletes the challenge, clears `bm_pending` and answers 204, and the old cookie is then 401 on every second-step and setup route; with no or an unknown cookie it still answers 204; with only a session cookie it leaves the session alone; cross-site is 403
- [X] T012 [P] Write `api/internal/httpapi/twofactor_test.go` (foundation part) for `POST /api/2fa/confirm` and `GET /api/2fa`: confirm with the right password and no factor is 204 and stamps the session; with a factor, a password alone is 400; right password + valid code is 204; right password + unused recovery code is 204 and spends it; wrong password is 403 `wrong_password`, wrong code 403 `invalid_code`, and each counts in the limiter (6th attempt 429 with `Retry-After`); more than one proof field is 400; tokens are 403; cross-site 403. `GET /api/2fa` returns `TwoFactorState` with `totp_available` false when `Deps.Box` is nil and `passkeys_available` false and `passkeys_origin` null when `Deps.WebAuthn` is nil, `passkeys_origin` equal to the configured origin when set, and never any secret
- [X] T013 [P] Write `web/src/components/ConfirmIdentity.test.tsx`: asks for the password only when `hasFactor` is false; with a factor, shows a code field and a "Use a recovery code" switch; posts to `/api/2fa/confirm` with exactly one proof field; shows `wrong_password`, `invalid_code` and 429 messages; calls `onConfirmed` on 204 and `onCancel` on cancel
- [X] T014 [P] Write `web/src/components/RecoveryCodes.test.tsx`: shows all 10 codes in a list, a warning that they are shown once, Copy (calls `navigator.clipboard.writeText` with one code per line) and Download (creates a `beer-menu-recovery-codes.txt` Blob link); the Done button stays disabled until "I saved these codes" is checked

### Implementation for the foundation

- [X] T015 Write `api/internal/store/migrations/0006_two_factor.sql` exactly per `data-model.md`: `totp_factors(user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE, secret_sealed TEXT NOT NULL, last_step INTEGER NOT NULL, created_at INTEGER NOT NULL, last_used_at INTEGER)`; `passkeys(id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, name TEXT NOT NULL, credential_id BLOB NOT NULL UNIQUE, public_key BLOB NOT NULL, sign_count INTEGER NOT NULL, transports TEXT NOT NULL, backup_eligible INTEGER NOT NULL, created_at INTEGER NOT NULL, last_used_at INTEGER)` with index `passkeys_user_id`; `recovery_codes(user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, code_hash BLOB NOT NULL, used_at INTEGER, PRIMARY KEY (user_id, code_hash))`; `auth_challenges(id_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, kind TEXT NOT NULL CHECK (kind IN ('second_factor','enroll','setup')), webauthn TEXT, totp_pending_sealed TEXT, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL)`; `ALTER TABLE sessions ADD COLUMN confirmed_at INTEGER`
- [X] T016 [P] Implement `api/internal/auth/totp.go` per T006 and research R3: `NewSecret() ([]byte, error)`, `EncodeSecret` (base32 no padding), `URI(username, secret)`, `Code(secret, step)`, `Verify(secret, code string, now time.Time, lastStep int64) (step int64, ok bool)` with `hmac.Equal`
- [X] T017 [P] Implement `api/internal/auth/challenges.go` per T007 and research R8: kinds `KindSecondFactor`, `KindEnroll`, `KindSetup`; `CreateChallenge(db, userID, kind, now)`, `SetupChallenge(db, sessionID, userID, now)` (replaces any earlier setup row for that session), `PeekChallenge`, `TakeChallenge`, `SetChallengeData`, `DeleteChallengesFor`; `ChallengeTTL = 5 * time.Minute`
- [X] T018 Implement `api/internal/auth/twofactor.go` (foundation part) per T008: `HasTwoFactor`, `NewRecoveryCodes`, `UseRecoveryCode(tx, userID, code, now) (bool, error)` (normalizes, marks `used_at`), `RecoveryCodesLeft`, `ClearTwoFactor`, `RequireTwoFactor`, `SetRequireTwoFactor` (via `settings.Get`/`settings.Set`, key `require_2fa`), and `TwoFactorState` struct matching the contract schema
- [X] T019 Update `api/internal/auth/sessions.go` per T009: `CreateSession` writes `confirmed_at`; add `ConfirmSession`, `SessionConfirmed`, const `ConfirmWindow = 5 * time.Minute`; `DeleteSession` and every path that ends sessions also delete `auth_challenges` rows keyed by those session hashes; update `api/internal/auth/users.go` so `Admin` gains `TwoFactor bool \`json:"two_factor"\`` filled by `GetAdmin` and `ListAdmins` with one `EXISTS` subquery each
- [X] T020 Update `api/internal/httpapi/routes.go` and `api/internal/httpapi/guard.go` per T010 and research R10: `Deps` gains `Box *secrets.Box` and `WebAuthn *webauthn.WebAuthn`; add `AccessEnroll` (`String()` = `"enroll"`); `principal` gains `Pending string` and `PendingUserID int64`; `resolve` reads the `bm_pending` cookie only when there is no Authorization header and no valid session cookie; pass `box` and the WebAuthn value into `Deps` in `api/cmd/beer-menu-api/main.go`
- [X] T021 Add pending-cookie helpers in `api/internal/httpapi/session.go` (`setPendingCookie`, `clearPendingCookie`, `pendingFrom(r, kind)`) per the conventions, and change `signIn` per T011: after `Authenticate` succeeds, if `HasTwoFactor` create a `second_factor` challenge and answer 202 with `methods`; else if `RequireTwoFactor` create an `enroll` challenge and answer 202 `{"next":"enroll"}`; else start the session as today; log `sign-in password ok, second factor owed` with the username. Add `DELETE /api/session/pending` (`AccessPublic`, handler `cancelSignIn`) per T011 and research R11: `TakeChallenge` the pending of either kind if present, `clearPendingCookie`, 204; register it in `api/internal/httpapi/routes.go`
- [X] T022 Implement `POST /api/2fa/confirm` and `GET /api/2fa` in new `api/internal/httpapi/twofactor.go` per T012 and research R9: limiter `Check`/`Done` on the admin's username around the password and proof; a helper `requireConfirmed(w, r, deps, p) bool` answering 403 `confirm_required` (an `enroll` principal counts as confirmed); register both routes in `Routes` (`/api/2fa/confirm` `AccessAdmin`, `GET /api/2fa` `AccessEnroll`)
- [X] T023 Add the foundation's paths and schemas to `api/openapi.yaml` from `contracts/two-factor.openapi.yaml` (the 202 on `POST /api/session`, `/api/session/pending`, `/api/2fa` GET, `/api/2fa/confirm`, `PendingCookie`, `SignInPending`, `CodeBody`, `ConfirmBody`, `TwoFactorState`, the shared responses, `Admin.two_factor`), document `x-access: enroll` in the intro, bump `info.version` to `0.10.0`, and teach `api/internal/httpapi/routes_test.go` and `api/internal/httpapi/auth_walk_test.go` the `enroll` level
- [X] T024 [P] Add types to `web/src/api.ts` (`Admin.two_factor`, `SignInPending`, `TwoFactorState`, `Passkey`, `TotpSetup`, `FactorAdded`, `RecoveryCodesReply`) and fixtures to `web/src/testApi.ts`
- [X] T025 [P] Implement `web/src/components/ConfirmIdentity.tsx` per T013 inside the existing `Modal`
- [X] T026 [P] Implement `web/src/components/RecoveryCodes.tsx` per T014
- [X] T027 Add page `two-factor` ("Two-Factor Sign-In", `ShieldCheck` icon) to the Administration group in `web/src/components/SideNav.tsx` and render a new `TwoFactorPanel` in `web/src/pages/AdminPage.tsx`; create `web/src/components/TwoFactorPanel.tsx` showing state from `GET /api/2fa` (on/off, required, recovery codes left) with a test in `web/src/components/TwoFactorPanel.test.tsx` and an update to `web/src/components/SideNav.test.tsx`

**Checkpoint**: migration applies, confirm works, password step answers 202 for 2FA admins,
and the panel shows state. No factor can be added yet.

---

## Phase 3: User Story 1 - Turn on an authenticator app and use it to sign in (Priority: P1) 🎯 MVP

**Goal**: an admin adds an authenticator app, saves recovery codes, and signs in with password
plus code.

**Independent Test**: enable an authenticator for an admin, sign out, and confirm sign-in needs
both the password and a valid current code, and that a wrong or old code is refused
(quickstart 2).

### Tests for User Story 1

- [X] T028 [P] [US1] Extend `api/internal/auth/twofactor_test.go`: `StartTotpSetup` seals a new secret into the session's `setup` challenge with purpose `totp:<user_id>` and returns secret + URI; `FinishTotpSetup` with a valid code saves the row with `last_step`, deletes the pending secret, and returns 10 recovery codes and ends the admin's other sessions only when this is the first factor (FR-017, FR-024); a wrong code saves nothing and keeps the setup open; an existing TOTP row gives `ErrAlreadySetUp`; a nil `Box` gives `ErrTotpUnavailable`; `CheckTotp` updates `last_step` and `last_used_at` and refuses a replayed step; a sealed secret moved to another admin's row fails to open
- [X] T029 [P] [US1] Extend `api/internal/httpapi/twofactor_test.go`: `POST /api/2fa/totp/setup` needs a recent confirm (403 `confirm_required` after 301 s), returns `secret` and `uri`, 409 `already_set_up`, 409 `totp_unavailable` without a `Box`; `POST /api/2fa/totp` 201 with `recovery_codes` on first factor, 400 `invalid_code`, 403 without confirm; tokens 403; cross-site 403; an `enroll` pending gets 401 from both routes (replaced in T062 by the enroll flow)
- [X] T030 [P] [US1] Extend `api/internal/httpapi/session_test.go` for `POST /api/session/totp`: no or expired `bm_pending` is 401 `unauthenticated`; a valid code gives 200 + Admin + new session cookie, clears `bm_pending`, and the pending cannot be used again; a wrong code is 401 `invalid_code`, the pending stays usable until it expires, and failures share the password limiter (5 wrong codes then 429 even with a valid code); a reused code is 401; an `enroll` pending is 401; cross-site is 403; the admin removed or factors cleared mid-step gives 401
- [X] T031 [P] [US1] Write `web/src/components/QrCode.test.tsx` (renders one `<svg>` with one `<path>`, `role="img"` and an `aria-label`, no `style` attribute) and `web/src/components/TwoFactorSetup.test.tsx` for the authenticator path: shows the QR and the secret in groups of 4; a wrong code shows the error and keeps the form; a valid code shows `RecoveryCodes` when returned, then calls `onDone`; shows "not available" when `totp_available` is false
- [X] T032 [P] [US1] Extend `web/src/components/LoginForm.test.tsx`: a 202 `second_factor` reply switches to a code step (`autoComplete="one-time-code"`, `inputMode="numeric"`); a valid code calls `onSignedIn`; 401 `invalid_code` shows "That code did not work."; 401 `unauthenticated` returns to the password step with "Sign-in timed out, start again."; 429 shows the wait message; Cancel calls `DELETE /api/session/pending` and returns to the password step

### Implementation for User Story 1

- [X] T033 [US1] Implement `StartTotpSetup`, `FinishTotpSetup`, `CheckTotp` and errors `ErrAlreadySetUp`, `ErrTotpUnavailable` in `api/internal/auth/twofactor.go` per T028
- [X] T034 [US1] Implement `POST /api/2fa/totp/setup` and `POST /api/2fa/totp` in `api/internal/httpapi/twofactor.go` per T029 (both `AccessEnroll`; until T064 replaces it, an `enroll` principal gets 401 `unauthenticated` from both, so a principal with no session never reaches the session-only code), and `POST /api/session/totp` in `api/internal/httpapi/session.go` per T030 (`AccessPublic`, requires a `second_factor` pending, `TakeChallenge` only on success, then `startSession`); register the routes in `api/internal/httpapi/routes.go`
- [X] T035 [US1] Add `/api/2fa/totp/setup`, `POST /api/2fa/totp`, `/api/session/totp`, `TotpSetup`, `FactorAdded`, `SignedIn`, `SecondStepFailed`, `ConfirmRequired`, `TwoFactorConflict` to `api/openapi.yaml` from the contract
- [X] T036 [P] [US1] Implement `web/src/components/QrCode.tsx` per T031 using `uqr`'s `encode()` and one SVG path built from the module matrix
- [X] T037 [US1] Implement the authenticator path of `web/src/components/TwoFactorSetup.tsx` per T031 and wire "Add authenticator app" in `web/src/components/TwoFactorPanel.tsx` (opens `ConfirmIdentity` first when the session's confirm has expired, detected by a 403 `confirm_required`)
- [X] T038 [US1] Add the second step to `web/src/components/LoginForm.tsx` per T032 (code step only; passkey and recovery options come in US2 and US3)

**Checkpoint**: US1 works alone (quickstart 2 and 3). Shippable MVP.

---

## Phase 4: User Story 2 - Add a passkey and sign in with fingerprint or face (Priority: P2)

**Goal**: an admin adds named passkeys and uses the device prompt as the second step; also as
a confirm proof.

**Independent Test**: on a supported device, add a passkey, sign out, and confirm sign-in
completes by approving the device prompt; a removed passkey no longer works (quickstart 4).

### Tests for User Story 2

- [X] T039 [P] [US2] Write the test-only software authenticator in `api/internal/auth/authtest/authenticator.go` per research R13: ES256 key, `"none"` attestation, configurable origin, RP ID, UV flag and sign count; `Register(options) json` and `Assert(options) json` producing `toJSON()`-shaped responses
- [X] T040 [P] [US2] Write `api/internal/auth/passkeys_test.go` using `authtest`: begin/finish registration refuses an 11th passkey with `ErrTooManyPasskeys` (cap "at most 10 per admin"); stores `credential_id`, `public_key`, `sign_count`, `transports` and `backup_eligible`, names validated ("1–64 code points, trimmed, no control characters"), first passkey returns recovery codes and ends other sessions; begin/finish login succeeds and updates `sign_count` and `last_used_at`; refused: wrong origin, wrong RP ID, UV flag off, another admin's credential, unknown credential, challenge already used or older than 300 s, a non-zero sign count that does not increase; `excludeCredentials` lists the admin's passkeys; a nil WebAuthn gives `ErrPasskeysUnavailable`
- [X] T041 [P] [US2] Extend `api/internal/httpapi/twofactor_test.go` and `api/internal/httpapi/session_test.go`: `POST /api/2fa/passkeys/options` and `POST /api/2fa/passkeys` (confirm required, 409 `passkeys_unavailable`, 409 `too_many_passkeys` for an 11th, 400 bad name, body over 16 KiB 400); `POST /api/session/passkey/options` (requires `second_factor` pending, `userVerification` `required`, `allowCredentials` = only this admin's); `POST /api/session/passkey` (200 + session; 401 `passkey_failed` counted by the limiter); `POST /api/2fa/confirm/options` and confirm with `passkey`; tokens 403; cross-site 403
- [X] T042 [P] [US2] Write `web/src/webauthn.test.ts`: `passkeysSupported(origin)` is false when `origin` is null or differs from `location.origin` (another host name or the IP address, FR-016), without `isSecureContext`, without `PublicKeyCredential`, or without `parseCreationOptionsFromJSON`/`parseRequestOptionsFromJSON`; `createPasskey(options)` and `getPasskey(options)` call `navigator.credentials` with the parsed options and return `toJSON()`; a `NotAllowedError` (cancel) maps to a "cancelled" result, not an error message
- [X] T043 [P] [US2] Extend `web/src/components/TwoFactorSetup.test.tsx`, `web/src/components/LoginForm.test.tsx` and `web/src/components/ConfirmIdentity.test.tsx`: add a passkey with a name; "not available here" when the browser lacks support, `passkeys_available` is false, or `passkeys_origin` differs from `location.origin` (with the message naming the address to use); login hides the passkey method when `passkeys_origin` differs; a 409 `too_many_passkeys` shows "You have 10 passkeys. Remove one before adding another." and "Add passkey" is disabled with that hint when 10 are listed (FR-013); login shows a method choice when `methods` has more than one, a passkey success signs in, a cancel keeps the step open and offers the code; confirm with passkey

### Implementation for User Story 2

- [X] T044 [US2] Implement `api/internal/auth/passkeys.go` per T040: a `webauthn.User` adapter over the admin (ID = 8-byte big-endian user id, name = username) and its stored credentials; `BeginPasskeyRegistration`, `FinishPasskeyRegistration` (`AuthenticatorSelection{ResidentKey: preferred, UserVerification: required}`, attestation none), `MaxPasskeys = 10` checked in both begin and finish registration (`ErrTooManyPasskeys`), `BeginPasskeyLogin`, `FinishPasskeyLogin` (UV required), session data JSON stored in the challenge row's `webauthn` column; `ErrPasskeysUnavailable`
- [X] T045 [US2] Implement the passkey handlers in `api/internal/httpapi/twofactor.go` (`/api/2fa/passkeys/options`, `POST /api/2fa/passkeys` as `AccessEnroll`; `/api/2fa/confirm/options` as `AccessAdmin`; `passkey` proof in confirm; `/api/2fa/passkeys/options` and `POST /api/2fa/passkeys` answer 401 to an `enroll` principal until T064, as in T034) and `api/internal/httpapi/session.go` (`/api/session/passkey/options`, `/api/session/passkey` as `AccessPublic` with a `second_factor` pending), bodies capped at 16 KiB; register the routes in `api/internal/httpapi/routes.go`
- [X] T046 [US2] Add the passkey paths and `Passkey`, `PasskeyName`, `WebAuthnOptions`, `WebAuthnResponse` schemas to `api/openapi.yaml` from the contract
- [X] T047 [P] [US2] Implement `web/src/webauthn.ts` per T042
- [X] T048 [US2] Add the passkey path to `web/src/components/TwoFactorSetup.tsx` (including the `too_many_passkeys` message and the disabled "Add passkey" at 10, per T043), the method choice and passkey step to `web/src/components/LoginForm.tsx`, and the passkey proof to `web/src/components/ConfirmIdentity.tsx` per T043; list passkeys in `web/src/components/TwoFactorPanel.tsx`

**Checkpoint**: quickstart 4 on a real phone and laptop, installed PWA and browser tab.

---

## Phase 5: User Story 3 - Recover when a second factor is lost (Priority: P2)

**Goal**: sign in with a recovery code, replace codes, another admin clears factors, the
operator resets a sole admin.

**Independent Test**: use a recovery code to sign in and confirm it cannot be used again; clear
an admin's factors as another admin and as the operator, and confirm the admin can then sign in
with the password alone (quickstart 5).

### Tests for User Story 3

- [X] T049 [P] [US3] Extend `api/internal/httpapi/session_test.go` for `POST /api/session/recovery`: valid unused code gives 200 + session and spends it; the same code again is 401 `invalid_code`; codes accepted with or without the dash and in any case; failures counted by the limiter; requires a `second_factor` pending; cross-site 403
- [X] T050 [P] [US3] Extend `api/internal/httpapi/twofactor_test.go` and `api/internal/httpapi/admins_test.go`: `POST /api/2fa/recovery-codes` (confirm required; returns 10 new codes; old unused codes then refused at sign-in; 409 `no_factor` without 2FA); `DELETE /api/admins/{id}/2fa` deletes the other admin's factors, codes, challenges and sessions (their cookie is then 401), own id is 409 `cannot_clear_self`, unknown id 404, works while `require_2fa` is on, token 403; `GET /api/admins` shows `two_factor`
- [X] T051 [P] [US3] Write `api/cmd/beer-menu-api/reset2fa_test.go` mirroring `resetpassword_test.go`: usage error without exactly one username; unknown username fails and changes nothing; a known username clears factors, codes, challenges and sessions and logs the username and time; `require_2fa` is left as it was
- [X] T052 [P] [US3] Extend `web/src/components/LoginForm.test.tsx` (recovery code step, `autoComplete="off"`), `web/src/components/TwoFactorPanel.test.tsx` (codes left, warning at 2 or fewer, "New recovery codes" after confirm shows `RecoveryCodes`) and `web/src/components/AdminsPanel.test.tsx` (2FA badge per admin; "Clear 2FA" only on other admins with 2FA; asks before clearing; refreshes after)

### Implementation for User Story 3

- [X] T053 [US3] Implement `POST /api/session/recovery` in `api/internal/httpapi/session.go` (`AccessPublic`, `second_factor` pending, `UseRecoveryCode` in the same transaction as taking the challenge), `POST /api/2fa/recovery-codes` in `api/internal/httpapi/twofactor.go`, and `DELETE /api/admins/{id}/2fa` in `api/internal/httpapi/admins.go` (log `admin 2fa cleared` with both usernames); register in `api/internal/httpapi/routes.go`
- [X] T054 [US3] Implement `api/cmd/beer-menu-api/reset2fa.go` (`resetTwoFactor(dataDir, args, out)` calling `auth.ClearTwoFactor` by username) and dispatch `reset-2fa` in `api/cmd/beer-menu-api/main.go` beside `reset-password`; update the command doc comment at the top of `main.go`
- [X] T055 [US3] Add `/api/session/recovery`, `/api/2fa/recovery-codes`, `/api/admins/{id}/2fa` and `RecoveryCodes` to `api/openapi.yaml` from the contract
- [X] T056 [US3] Implement the recovery step in `web/src/components/LoginForm.tsx`, codes-left and "New recovery codes" in `web/src/components/TwoFactorPanel.tsx`, and the badge and "Clear 2FA" action in `web/src/components/AdminsPanel.tsx` per T052

**Checkpoint**: quickstart 5.

---

## Phase 6: User Story 4 - Manage own second factors (Priority: P3)

**Goal**: list, rename and remove factors; turn 2FA off.

**Independent Test**: add two factors, remove one, and confirm only the other works at sign-in;
remove the last one and confirm sign-in needs only the password.

### Tests for User Story 4

- [X] T057 [P] [US4] Extend `api/internal/httpapi/twofactor_test.go`: `PATCH /api/2fa/passkeys/{id}` renames (no confirm needed; 400 bad name; another admin's id 404); `DELETE /api/2fa/passkeys/{id}`, `DELETE /api/2fa/totp` and `DELETE /api/2fa` need a recent confirm, end the caller's other sessions (this one kept), and removing the last factor deletes recovery codes (FR-025) so the next sign-in is password-only; another admin's passkey id 404; `GET /api/2fa` never includes `secret_sealed`, `public_key` or `credential_id`
- [X] T058 [P] [US4] Extend `web/src/components/TwoFactorPanel.test.tsx`: each factor shows type, name, added and last-used time; rename passkey inline; remove asks `ConfirmIdentity` when needed then removes; "Turn off 2FA" confirms and refreshes to the off state

### Implementation for User Story 4

- [X] T059 [US4] Implement `RenamePasskey`, `RemovePasskey`, `RemoveTotp` (last-factor cleanup) and `TurnOffTwoFactor` in `api/internal/auth/twofactor.go` and `api/internal/auth/passkeys.go`, and the `PATCH`/`DELETE /api/2fa/passkeys/{id}`, `DELETE /api/2fa/totp` and `DELETE /api/2fa` handlers in `api/internal/httpapi/twofactor.go`; register in `api/internal/httpapi/routes.go`; add the paths to `api/openapi.yaml`
- [X] T060 [US4] Implement the factor list, rename, remove and turn-off actions in `web/src/components/TwoFactorPanel.tsx` per T058

**Checkpoint**: spec User Story 4 acceptance scenarios pass.

---

## Phase 7: User Story 5 - Require 2FA for every admin (Priority: P3)

**Goal**: a session-only setting that forces every admin to have a factor; admins without one
can only enroll after the password.

**Independent Test**: turn the setting on, sign in as an admin with no factor, and confirm the
only thing they can do is set up a factor (or sign out); admins can no longer remove their last
factor while the setting is on (quickstart 6).

### Tests for User Story 5

- [X] T061 [P] [US5] Extend `api/internal/httpapi/twofactor_test.go`: `PUT /api/2fa/required` needs a recent confirm and a factor of the caller's own (409 `no_factor`), 400 for a non-boolean, token 403, writes `require_2fa`; turning it on ends no sessions; while on, removing the last factor or `DELETE /api/2fa` is 409 `two_factor_required`; `GET /api/settings` and `PATCH /api/settings` neither show nor accept `require_2fa`
- [X] T062 [P] [US5] Extend `api/internal/httpapi/twofactor_test.go` for the enroll flow (replacing the interim 401 cases from T029 and T041): with an `enroll` pending, `GET /api/2fa`, `POST /api/2fa/totp/setup`, `POST /api/2fa/totp`, `POST /api/2fa/passkeys/options` and `POST /api/2fa/passkeys` work without a confirm; the first confirmed factor returns `admin` and `recovery_codes`, sets the session cookie, clears `bm_pending`, and the pending cannot be reused; every `AccessAdmin`, `AccessRead` and `AccessWrite` route is 401 with only an enroll pending (extend `api/internal/httpapi/auth_walk_test.go`); an expired or cancelled (`DELETE /api/session/pending`) enroll pending is 401
- [X] T063 [P] [US5] Extend `web/src/pages/AdminPage.test.tsx` (a 202 `enroll` shows only the enroll view with `TwoFactorSetup` and Sign out, no `SideNav`; Sign out calls `DELETE /api/session/pending` and shows the login form; finishing shows recovery codes then the normal shell) and `web/src/components/TwoFactorPanel.test.tsx` (require switch disabled with a hint when the caller has no factor; toggling asks `ConfirmIdentity`; remove-last and turn-off show the `two_factor_required` message naming the setting)

### Implementation for User Story 5

- [X] T064 [US5] Implement `PUT /api/2fa/required` and the `two_factor_required` checks in `api/internal/httpapi/twofactor.go` and `api/internal/auth/twofactor.go`; implement the enroll principal in the setup handlers (treated as confirmed; on the first factor `TakeChallenge`, `startSession`, clear `bm_pending`, include `admin` in the reply); register the route and add `/api/2fa/required` to `api/openapi.yaml`
- [X] T065 [US5] Implement the enroll view in `web/src/pages/AdminPage.tsx` (driven by `LoginForm`'s 202 `enroll`) and the require switch in `web/src/components/TwoFactorPanel.tsx` per T063
- [X] T065a [US5] Owner decision 2026-11-12: `PUT /api/2fa/required` refuses to turn the setting ON when the server has no encryption key AND no `BEER_MENU_PUBLIC_URL` (no factor could be enrolled; turning OFF is unaffected and still needs the admin's own factor); add the error to `api/openapi.yaml` and `contracts/two-factor.openapi.yaml`, show it in `web/src/components/TwoFactorPanel.tsx`, with Go and web tests
- [X] T065b [US5] Owner decision 2026-11-12: at startup, `api/cmd/beer-menu-api` logs a warning when `require_2fa` is on and the server has neither an encryption key nor `BEER_MENU_PUBLIC_URL` (admins without a factor cannot finish signing in), with a Go test
- [X] T065c [US4] Owner decision 2026-11-12 (security review): `DELETE /api/admins/{id}/2fa` (clear another admin's 2FA) requires a recent identity confirm (`requireConfirmed`, 403 `confirm_required`) like the other 2FA changes; update `api/openapi.yaml`, `contracts/two-factor.openapi.yaml`, spec FR-027, and `web/src/components/AdminsPanel.tsx` (confirm then retry, as `TwoFactorPanel` does), with Go and web tests
- [X] T065d [US4] Owner decision 2026-11-12 (security review): `POST /api/admins` (add admin) and `DELETE /api/admins/{id}` (remove admin) require a recent identity confirm (`setupCaller`, 403 `confirm_required`), like T065c; update `api/openapi.yaml`, `contracts/two-factor.openapi.yaml` if it lists confirm routes, spec, docs, and `web/src/components/AdminsPanel.tsx` (Add admin modal: confirm then retry), with Go and web tests
- [X] T065e [US4] Owner decision 2026-11-13 (security review): `POST /api/tokens` (create API token) and `POST /api/password` (change own password) require a recent identity confirm (403 `confirm_required`, like T065c/T065d; the confirm asks for the second factor when the admin has one); update `api/openapi.yaml`, spec, docs, README, and `web/src/components/TokensPanel.tsx` and `PasswordForm.tsx` (confirm then retry), with Go and web tests

**Checkpoint**: quickstart 6.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T066 [P] Run the contract check and Redocly lint (`api/internal/httpapi/routes_test.go`, `npx @redocly/cli lint api/openapi.yaml`); diff `api/openapi.yaml` against `contracts/two-factor.openapi.yaml` and fix any drift
- [X] T067 [P] Document `BEER_MENU_PUBLIC_URL` (API container only; scheme + host [+ port], exactly the browser's address; passkeys off when unset) and `reset-2fa <username>` in `README.md`, and add a commented-out `BEER_MENU_PUBLIC_URL: https://taplist.example.org` to the api service in `docker-compose.yaml` (no real host name, constitution V)
- [X] T068 [P] Update the user and API docs in `docs/` for two-factor sign-in, recovery and the require setting
- [X] T069 Run `govulncheck ./...` in `api/` and record the result, including the new indirect modules, in `specs/011-admin-2fa/research.md` R1
- [X] T070 Run quickstart scenarios 2–7 from `specs/011-admin-2fa/quickstart.md`, including scenario 4 on a real phone (installed PWA and browser tab) and laptop; record any deviation in `specs/011-admin-2fa/research.md`
- [X] T071 Full gate: `go test -race ./...` in `api/`; `npx vitest run --maxWorkers=2`, `npm run lint`, `npm run build` in `web/`; `docker compose build`. The PR description states the constitution principle V amendment (1.3.0 to 1.4.0) and its reason, as the constitution's Governance section requires

---

## Dependencies & Execution Order

### Phase dependencies

- Setup (Phase 1): none. T001 ∥ T002; T003 → T004 (T004 also needs T001).
- Foundational (Phase 2): after Setup. Blocks every story.
- US1 (Phase 3): after Foundational. The MVP.
- US2 (Phase 4): after Foundational. Independent of US1 in the API; shares `TwoFactorSetup.tsx`,
  `LoginForm.tsx` and `ConfirmIdentity.tsx`, so run after US1.
- US3 (Phase 5): after Foundational; its sign-in test needs a factor to exist, so after US1
  (US2 not needed).
- US4 (Phase 6): after US1 and US2 (removes and renames both factor types).
- US5 (Phase 7): after US1 (enroll uses the authenticator path); passkey enroll after US2.
- Polish (Phase 8): after every story.

### Within each phase

- Tests first and failing, then implementation.
- Phase 2 Go: T015 → T016, T017 (parallel) → T018 → T019 → T020 → T021 → T022 → T023.
- Phase 2 web: T024 → T025 ∥ T026 → T027.
- Each story: auth package → handlers → OpenAPI → web.

### Parallel opportunities

- Phase 1: T001 ∥ T002.
- Phase 2 tests: T005–T014 all [P] (different files). Go track (T015–T023) ∥ web track
  (T024–T027).
- Each story's test tasks are [P] with each other.
- Phase 8: T066 ∥ T067 ∥ T068.

### Parallel example: Phase 2

```text
Go track:  T005..T012 -> T015 -> (T016 ∥ T017) -> T018 -> T019 -> T020 -> T021 -> T022 -> T023
Web track: T013, T014 -> T024 -> (T025 ∥ T026) -> T027
```

### Parallel example: User Story 2

```text
T039 -> T040 ; T041 ; T042 ; T043   (tests, in parallel except T040 needs T039)
T044 -> T045 -> T046                 (Go)
T047 -> T048                         (web)
```

## Implementation Strategy

### MVP first

1. Phase 1 and Phase 2: storage, confirm, two-step sign-in plumbing.
2. Phase 3 (US1): authenticator app with recovery codes shown. Stop and validate with
   quickstart 2 and 3. This is a shippable MVP: password alone no longer opens the app for
   admins who turn it on.

### Incremental delivery

3. US2 passkeys (validate on a real device), then US3 recovery (needed before relying on 2FA
   day to day), then US4 management, then US5 the require setting.
4. Polish: docs, `govulncheck`, real-device quickstart, full gate.
