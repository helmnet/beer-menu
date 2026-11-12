---

description: "Task list for feature 2: authentication and API tokens"
---

# Tasks: Authentication and API Tokens

**Input**: Design documents from `specs/002-auth-api-tokens/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks; migration 0002 is applied to a version 1 database;
auth has a test for each rule (unauthenticated, `read` vs `write`, revoked token, CSRF on
session writes, rate limiting); React components with logic get Vitest tests. Write each test
task before its implementation task and see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US6)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`); new package `api/internal/auth`
  holds rules and SQL with no HTTP
- Web: `web/` (Vite + React + TypeScript), new components in `web/src/components/`
- Repo root: `docker-compose.yaml`, `README.md`

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No secrets, credentials, private hostnames or IP addresses in any file, test fixtures
  included: generate test passwords in the test (constitution V).
- Only the dependencies in research R15. No router, session, CSRF or web libraries.
- Never log a password, session ID, token secret, cookie or `Authorization` header (FR-038).
- Never read `X-Forwarded-For`, `X-Real-IP` or the peer address for any auth decision (FR-020).
- Error bodies are `{"error": "<code>", "message": "<text>"}` with the codes in the contract.
- Times: `INTEGER` Unix seconds in SQLite, RFC 3339 in JSON. Inject `now func() time.Time` where
  a test needs to move time.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: dependencies and the contract the rest of the feature builds against.

- [X] T001 Add `golang.org/x/crypto` v0.58.0 and `golang.org/x/term` v0.47.0 to `api/go.mod` and `api/go.sum` with `go get`, then `go mod tidy`; confirm both release dates are at least two weeks before the work date (research R15)
- [X] T002 [P] Replace `api/openapi.yaml` with `specs/002-auth-api-tokens/contracts/openapi.yaml` and confirm `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` reports no errors
- [X] T003 [P] Create `web/src/api.ts`: `apiFetch<T>(method, path, body?)` that sends JSON same-origin, returns parsed JSON (or `undefined` on 204), and throws `ApiError {status, code, message, retryAfter?}` built from the error body and `Retry-After` header; add `web/src/api.test.ts` covering success, 204, an error body and `Retry-After`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: schema, password rules, sessions, token lookup, the access guard, CSRF and API
headers. Every story depends on these.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Tests for Foundational

- [X] T004 [P] Migration test in `api/internal/store/migrate_test.go`: apply 0001 to a new database, insert a `settings` row, apply 0002, and assert schema version 2, the `settings` row kept, and tables `users`, `sessions`, `api_tokens` exist with the constraints below (a second `OWNER` insert after `owner` fails; `scope = 'admin'` fails; deleting a user deletes its sessions)
- [X] T005 [P] Password tests in `api/internal/auth/password_test.go`: hash/verify round trip; wrong password fails; the PHC string starts `$argon2id$v=19$m=19456,t=2,p=1$`; two hashes of one password differ; `ValidatePassword` accepts 12 and 256 code points and rejects 11, 257 and an empty string, counting code points not bytes (e.g. 12 `é`); `ValidateUsername` trims, then accepts `owner`, `a.b_c-1`, a 64-character name, and rejects empty, 65 characters, a space inside, `ü` and `/`; the semaphore lets at most 4 hashes run at once and returns `ErrBusy` after its wait; add `BenchmarkVerifyPassword` so the hash cost can be checked on the target machine
- [X] T006 [P] Session store tests in `api/internal/auth/sessions_test.go` with an injected clock: create returns a 43-character base64url ID and stores only its SHA-256; lookup succeeds before expiry; fails at 1 hour idle; fails at 30 days even when used every 30 minutes; `last_seen_at` is written at most once a minute; `DeleteSession`, `DeleteUserSessions` and `DeleteOtherSessions` remove the right rows; a session looked up with `now` earlier than `created_at` is invalid; `SweepExpired` removes only invalid rows
- [X] T007 [P] Token lookup tests in `api/internal/auth/tokens_test.go`: a token inserted through a test helper is found by its secret and returns its scope; an unknown secret, a revoked token and a secret without the `bmt_` prefix are not found; `last_used_at` goes from NULL to now on first use and is not rewritten within 1 minute
- [X] T008 [P] Guard tests in `api/internal/httpapi/guard_test.go` using stub handlers wrapped by `guard` for each access level, covering every cell of the access table in `data-model.md`: no credential gives 401 on `read`, `write`, `admin`; session passes all; `read` token passes `read`, gets 403 on `write` and `admin`; `write` token passes `read` and `write`, gets 403 on `admin`; revoked, unknown and malformed bearer (`Bearer`, `Bearer `, `Basic x`, `bmt_` only) give 401 even with a valid session cookie also sent; `public` passes with no credential; the handler sees the resolved principal (admin ID or token ID and scope)
- [X] T009 [P] Contract test in `api/internal/httpapi/routes_test.go`: extend `undocumented` checks so every route's `Access` equals its operation's `x-access`, and `security` is `[]` exactly when `x-access` is `public`; also assert that every route with `AccessRead` uses `GET`; add failing-case tests like `TestRoutesDocumentedCatchesMissing` for a mismatched access, a route with no `x-access`, and a `POST` route marked `AccessRead`
- [X] T010 [P] Middleware tests in `api/internal/httpapi/headers_test.go`: through `NewMux`, every `/api` response has `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, each exactly once; every `/api` response except `/api/health` has `Cache-Control: no-store`; a `DELETE` with `Sec-Fetch-Site: cross-site` and one with `Origin: http://evil.example` and `Host: localhost:8080` get 403; the same request with `Sec-Fetch-Site: same-origin`, and one with neither header, reach the handler
- [X] T011 [P] JSON helper tests in `api/internal/httpapi/json_test.go`: a body over 8 KiB, unknown fields, trailing data and a wrong content shape each give `400 invalid_request`; `writeError` emits the `Error` schema

### Implementation for Foundational

- [X] T012 Write `api/internal/store/migrations/0002_auth.sql` per `data-model.md`: `users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE COLLATE NOCASE, password_hash TEXT NOT NULL, created_at INTEGER NOT NULL)`; `sessions (id_hash BLOB PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, created_at INTEGER NOT NULL, last_seen_at INTEGER NOT NULL, expires_at INTEGER NOT NULL)` with index on `sessions(user_id)`; `api_tokens (id INTEGER PRIMARY KEY, name TEXT NOT NULL, token_hash BLOB NOT NULL UNIQUE, scope TEXT NOT NULL CHECK (scope IN ('read','write')), created_at INTEGER NOT NULL, last_used_at INTEGER, revoked_at INTEGER)` (makes T004 pass)
- [X] T013 Implement `api/internal/auth/password.go` (research R1, R2, R3, R12): `HashPassword`, `VerifyPassword` (constant-time compare, parses PHC parameters), `ValidatePassword` ("12 to 256 characters", code points, never trimmed), `ValidateUsername` (trim, then `^[A-Za-z0-9._-]{1,64}$`), a package semaphore of 4 with a 5-second wait returning `ErrBusy`, and a `DummyVerify` that checks against a hash made once at package init (makes T005 pass)
- [X] T014 Implement `api/internal/auth/sessions.go` (research R4): `CreateSession(db, userID, now) (id string)`, `LookupSession(db, id, now) (userID, ok)` enforcing "`created_at <= now` and `now < expires_at` and `now < last_seen_at + 1 hour`" and touching `last_seen_at` at most once a minute, `DeleteSession`, `DeleteUserSessions`, `DeleteOtherSessions(db, userID, keepID)`, `SweepExpired(db, now)` (makes T006 pass)
- [X] T015 Implement token lookup in `api/internal/auth/tokens.go` (research R6): `LookupToken(db, secret, now) (Token, ok)` that requires the `bmt_` prefix, looks up by SHA-256, rejects `revoked_at IS NOT NULL`, and updates `last_used_at` only when NULL or older than 60 seconds; define `Scope` constants `ScopeRead`, `ScopeWrite` (makes T007 pass)
- [X] T016 Implement `api/internal/httpapi/json.go`: `decodeJSON(w, r, dst)` with `http.MaxBytesReader` 8 KiB and `DisallowUnknownFields`, `writeJSON(w, status, v)`, `writeError(w, status, code, message)`; plus the session cookie helpers `setSessionCookie(w, id, expires, secure)` (`bm_session`, `Path=/`, `HttpOnly`, `SameSite=Strict`, `Secure` when configured) and `clearSessionCookie(w, secure)` (makes T011 pass)
- [X] T017 Implement `api/internal/httpapi/guard.go` and update `api/internal/httpapi/routes.go` (research R7): type `Access` with `AccessAdmin` as the zero value, then `AccessRead`, `AccessWrite`, `AccessPublic`; add `Access` to `Route`; add `CookieSecure bool` and `Now func() time.Time` to `Deps`; `guard` resolves a bearer header first (any `Authorization` header that is not a valid `Bearer bmt_...` token is 401), else the `bm_session` cookie, then applies the access table and stores the principal in the request context with an accessor `principalFrom(ctx)`; `NewMux` wraps each route in `guard`; mark `/api/health` `AccessPublic` (makes T008 and T009 pass)
- [X] T018 Implement `api/internal/httpapi/headers.go` and wrap the mux in `NewMux` (research R5, R11): an outer handler that sets the four API security headers and `Cache-Control: no-store` for every path except `/api/health`, around `http.NewCrossOriginProtection().Handler(mux)`; make `NewMux` return `http.Handler` and update `api/cmd/beer-menu-api/main.go` and existing tests to match (makes T010 pass)
- [X] T019 Update `api/cmd/beer-menu-api/main.go` and `api/cmd/beer-menu-api/startup.go`: read `BEER_MENU_COOKIE_SECURE` (default true; only the exact value `false` turns it off) into `config` and `Deps`; after migrations call `auth.SweepExpired`; document the new env var in the package comment

**Checkpoint**: `go test ./...` passes; the API serves only `/api/health` publicly, refuses every
other request, and has CSRF and headers in place.

---

## Phase 3: User Story 1 - Create the first admin (Priority: P1) 🎯 MVP

**Goal**: a fresh install gets its first admin from env vars at startup or from a setup screen,
exactly once.

**Independent Test**: start against an empty data folder, create the first admin by either
method, and confirm a second attempt is refused (quickstart §1).

### Tests for User Story 1

- [X] T020 [P] [US1] Store tests in `api/internal/auth/users_test.go`: `CreateFirstAdmin` succeeds on an empty table and returns `ErrSetupDone` when any admin exists; 10 goroutines calling it at once create exactly one row; `AdminCount` reports 0 then 1; the stored hash verifies and is not the password
- [X] T021 [P] [US1] Handler tests in `api/internal/httpapi/setup_test.go`: `GET /api/setup` gives `{"needed":true}` then `{"needed":false}`; `POST /api/setup` with valid credentials gives 201, the `Admin` body, and a `bm_session` cookie that then passes `GET /api/session`; a second `POST` gives `409 setup_done` and changes nothing; an 11-character password and a bad username give `400 invalid_request`; the response never contains the password
- [X] T022 [P] [US1] Startup tests in `api/cmd/beer-menu-api/startup_test.go`: with `BEER_MENU_ADMIN_USERNAME` and `BEER_MENU_ADMIN_PASSWORD` set and no admins, startup creates the admin; with an admin present, it ignores them and the existing password still verifies; only one of the two set, or an 11-character password, makes startup return an error and create no admin; captured log output never contains the password
- [X] T023 [P] [US1] Component test in `web/src/components/SetupForm.test.tsx` and `web/src/pages/AdminPage.test.tsx` with `fetch` stubbed: `AdminPage` shows `SetupForm` when `GET /api/setup` returns `needed: true`; submitting calls `POST /api/setup`, then shows the signed-in view; a 400 shows the server message; a 409 switches to the login form

### Implementation for User Story 1

- [X] T024 [US1] Implement `CreateFirstAdmin(db, username, password, now)` and `AdminCount(db)` in `api/internal/auth/users.go`, using "`INSERT INTO users (...) SELECT ... WHERE NOT EXISTS (SELECT 1 FROM users)`" and rows affected, after `ValidateUsername`/`ValidatePassword` and hashing (makes T020 pass)
- [X] T025 [US1] Implement `GET /api/setup` and `POST /api/setup` in `api/internal/httpapi/setup.go`, both `AccessPublic`; on success create a session and set the cookie; log `first admin created` with `username`; register in `api/internal/httpapi/routes.go` (makes T021 pass)
- [X] T026 [US1] Add first-admin creation to `api/cmd/beer-menu-api/startup.go` after migrations (research R9): both env vars set and no admin → create and log `first admin created`; admin exists → log `first admin settings ignored, an admin exists`; exactly one set or invalid values → return an error so the API exits; read the vars in `main.go` into `config` (makes T022 pass)
- [X] T027 [US1] Implement `web/src/components/SetupForm.tsx` (username, password, confirm password, client-side 12-character check, server message on error) and update `web/src/pages/AdminPage.tsx` to fetch `/api/setup` and `/api/session` on load and pick setup, login or signed-in view; the signed-in view is a placeholder shell with the username until US2 (makes T023 pass)

**Checkpoint**: quickstart §1 passes.

---

## Phase 4: User Story 2 - Log in and log out (Priority: P1)

**Goal**: admins sign in and out with server-side sessions; nothing non-public works without a
credential; cross-site writes fail.

**Independent Test**: sign in, call a protected endpoint, sign out, call it again with the same
cookie and get 401 (quickstart §2).

### Tests for User Story 2

- [X] T028 [P] [US2] Handler tests in `api/internal/httpapi/session_test.go`: `POST /api/session` with the right password gives 200, the `Admin` body and a cookie with `HttpOnly`, `SameSite=Strict`, `Path=/` and `Secure` (and no `Secure` when `CookieSecure` is false); a cookie value sent before sign-in is not the one returned (FR-017); unknown username and wrong password give byte-identical `401 invalid_credentials` bodies and both call one password verify (assert through a counting hook on `DummyVerify`/`VerifyPassword`); `GET /api/session` returns the admin; `DELETE /api/session` gives 204, clears the cookie, and a copy of the old cookie then gets 401; an expired session gets 401
- [X] T029 [P] [US2] CSRF test in `api/internal/httpapi/session_test.go`: with a valid session cookie, `DELETE /api/session` with `Sec-Fetch-Site: cross-site` gives 403 and the session still works afterwards (constitution IV "CSRF on session writes"); with no admin, a `POST /api/setup` with `Sec-Fetch-Site: cross-site` gives 403 and creates no admin; a `POST /api/session` with `Sec-Fetch-Site: cross-site` and the right password gives 403, sets no cookie, and does not count toward the rate limit
- [X] T030 [P] [US2] Component tests in `web/src/components/LoginForm.test.tsx` and `web/src/pages/AdminPage.test.tsx`: `AdminPage` shows `LoginForm` when setup is not needed and `GET /api/session` is 401; a successful sign-in shows the signed-in view; a 401 shows "Wrong username or password."; the sign-out button calls `DELETE /api/session` and returns to the login form

### Implementation for User Story 2

- [X] T031 [US2] Implement `Authenticate(db, username, password)` and `GetAdmin(db, id)` in `api/internal/auth/users.go`: lookup ignoring case; unknown user runs `DummyVerify` and returns `ErrInvalidCredentials`, same as a wrong password; returns `ErrBusy` from the semaphore
- [X] T032 [US2] Implement `POST /api/session` (`AccessPublic`), `GET /api/session` and `DELETE /api/session` (`AccessAdmin`) in `api/internal/httpapi/session.go`; sign-in always mints a new session; `ErrBusy` gives `503 busy` with `Retry-After: 5`; log `admin signed in` and `sign-in failed` with `username`; register in `routes.go` (makes T028, T029 pass)
- [X] T033 [US2] Implement `web/src/components/LoginForm.tsx` and the signed-in shell in `web/src/pages/AdminPage.tsx` with the username and a sign-out button; on any 401 from a later call, return to the login form (makes T030 pass)

**Checkpoint**: quickstart §2 passes.

---

## Phase 5: User Story 3 - Slow down password guessing (Priority: P1)

**Goal**: failed sign-ins are limited per username, never by client address.

**Independent Test**: six failed sign-ins with a changing `X-Forwarded-For`; the sixth gets 429,
and so does the right password until the window passes (quickstart §3).

### Tests for User Story 3

- [X] T034 [P] [US3] Limiter tests in `api/internal/auth/limiter_test.go` with an injected clock: 5 failures for `owner` allow, the 6th check refuses with a retry-after equal to the time until the oldest failure is 15 minutes old; `OWNER` and `owner` share a count; another username is unaffected; counts clear as the window slides; a success does not clear the count; concurrent attempts count until done; a spray of 10,001 other usernames does not evict `owner`; expired usernames are swept
- [X] T035 [P] [US3] Handler tests in `api/internal/httpapi/session_test.go`: 5 wrong passwords then the right one gives `429 too_many_attempts` with an integer `Retry-After`; each request sets a different `X-Forwarded-For` and `X-Real-IP` and the result is the same; a limited attempt does not call the password verify; after advancing the clock 15 minutes the right password gives 200; log output has `sign-in rate limited` with `username`

### Implementation for User Story 3

- [X] T036 [US3] Implement `api/internal/auth/limiter.go` (research R8): `NewLimiter(now)`, `Check(username) (retryAfter time.Duration, ok bool)`, `Done(username, failed)`; "5 per username" in a sliding 15-minute window, attempts in flight count as failures until `Done`, usernames with live failures never evicted and expired ones swept at 10,000 tracked, keyed by lowercased username, pruned on access, mutex-guarded (makes T034 pass)
- [X] T037 [US3] Add a `*auth.Limiter` to `Deps`, create it in `main.go`, and in `POST /api/session` check before verifying and record failures after; refuse with `429` and `Retry-After` in whole seconds rounded up (makes T035 pass)
- [X] T038 [US3] Show the rate-limit message from `ApiError.retryAfter` in `web/src/components/LoginForm.tsx` and add a test case to `web/src/components/LoginForm.test.tsx`

**Checkpoint**: quickstart §3 passes. All three P1 stories done: this is the MVP.

---

## Phase 6: User Story 4 - Create, list and revoke API tokens (Priority: P2)

**Goal**: admins create scoped tokens shown once, list them without secrets, and revoke them.

**Independent Test**: create a `read` and a `write` token, list them, use one on an admin route
(403), revoke one and see it rejected (quickstart §4).

### Tests for User Story 4

- [X] T039 [P] [US4] Store tests in `api/internal/auth/tokens_test.go`: `CreateToken` returns a secret matching `^bmt_[A-Za-z0-9_-]{43}$` and stores only its SHA-256; two secrets differ; name is trimmed and must be "1 to 64 characters"; scope must be `read` or `write`; `ListTokens` returns newest first with `last_used_at` and `revoked_at` NULL where unset and no secret or hash field; `RevokeToken` sets `revoked_at`, is a no-op on a revoked token, and returns `ErrNotFound` for an unknown ID; a revoked token fails `LookupToken`
- [X] T040 [P] [US4] Handler tests in `api/internal/httpapi/tokens_test.go`: `POST /api/tokens` gives 201 with `secret` once; `GET /api/tokens` never contains the secret or `token_hash`; `DELETE /api/tokens/{id}` gives 204 and the token then gets 401 on the next request; unknown ID gives 404; a `read` and a `write` token both get 403 on all three token routes (FR-033); invalid name or scope gives 400; log has `token created` and `token revoked` with `token_name` and never the secret
- [X] T041 [P] [US4] Component test in `web/src/components/TokensPanel.test.tsx`: lists tokens with scope, created, last used ("never" when null) and revoked state; creating shows the secret with a copy button; after closing the dialog the secret is not in the DOM and cannot be shown again; revoke asks for confirmation, calls `DELETE`, and shows the token as revoked

### Implementation for User Story 4

- [X] T042 [US4] Implement `CreateToken(db, name, scope, now)`, `ListTokens(db)` and `RevokeToken(db, id, now)` in `api/internal/auth/tokens.go`: 32 bytes from `crypto/rand`, `bmt_` + base64url without padding (makes T039 pass)
- [X] T043 [US4] Implement `GET /api/tokens`, `POST /api/tokens` and `DELETE /api/tokens/{id}` in `api/internal/httpapi/tokens.go`, all `AccessAdmin`, parse `{id}` with `r.PathValue` and reject non-positive integers with 404; register in `routes.go` (makes T040 pass)
- [X] T044 [US4] Implement `web/src/components/TokensPanel.tsx` and add it to the signed-in view in `web/src/pages/AdminPage.tsx` (makes T041 pass)

**Checkpoint**: quickstart §4 passes.

---

## Phase 7: User Story 5 - Add and remove admins (Priority: P2)

**Goal**: admins add and remove admins, change their own password, and an operator can reset a
password from the server.

**Independent Test**: add a second admin, sign in as them, remove the first, and confirm the
first's sessions end; reset a password with the subcommand (quickstart §5).

### Tests for User Story 5

- [X] T045 [P] [US5] Store tests in `api/internal/auth/users_test.go`: `AddAdmin` creates a user and returns `ErrUsernameTaken` for `OWNER` when `owner` exists; `ListAdmins` returns oldest first with no hash field; `RemoveAdmin` deletes the user and its sessions, returns `ErrLastAdmin` when one admin remains and `ErrNotFound` for an unknown ID; two concurrent removals of the last two admins leave exactly one; `ChangePassword` with the wrong current password returns `ErrWrongPassword` and changes nothing, with the right one updates the hash and deletes the admin's other sessions but keeps the given one; `ResetPassword` updates the hash and deletes all that admin's sessions, and returns `ErrNotFound` for an unknown username
- [X] T046 [P] [US5] Handler tests in `api/internal/httpapi/admins_test.go` and `api/internal/httpapi/session_test.go`: `POST /api/admins` 201 and the new admin can sign in; duplicate (any case) gives `409 username_taken`; `GET /api/admins` lists without hashes; `DELETE /api/admins/{id}` 204 and the removed admin's cookie gets 401 at once; removing yourself works and signs you out; the last admin gives `409 last_admin`; `POST /api/password` with the right current password gives 204, the old password then fails, another session of the same admin gets 401 and this one still works; a wrong current password gives `403 wrong_password` and counts toward the sign-in limit for that username; a `write` token gets 403 on all these routes; logs have `admin added`, `admin removed`, `password changed`
- [X] T047 [P] [US5] Subcommand tests in `api/cmd/beer-menu-api/resetpassword_test.go` with injected stdin/stdout and an `isTerminal` stub: matching entries reset the password and end sessions; mismatched entries, an 11-character password, an unknown username, a non-terminal stdin and a database not at the newest schema version each exit 1 and change nothing; output and log never contain the password
- [X] T048 [P] [US5] Component tests in `web/src/components/AdminsPanel.test.tsx` and `web/src/components/PasswordForm.test.tsx`: list, add (409 message), remove with confirmation (409 last-admin message, removing yourself returns to login); password form requires matching new entries, shows `wrong_password` and rate-limit messages, and shows success

### Implementation for User Story 5

- [X] T049 [US5] Implement `AddAdmin`, `ListAdmins`, `RemoveAdmin` ("`DELETE FROM users WHERE id = ? AND (SELECT COUNT(*) FROM users) > 1`", then distinguish last-admin from not-found), `ChangePassword` and `ResetPassword` in `api/internal/auth/users.go`; `ResetPassword` updates the hash and deletes sessions in one transaction (makes T045 pass)
- [X] T050 [US5] Implement `GET /api/admins`, `POST /api/admins`, `DELETE /api/admins/{id}` in `api/internal/httpapi/admins.go` and `POST /api/password` in `api/internal/httpapi/session.go`, all `AccessAdmin`; the password route uses the limiter for wrong current passwords; register in `routes.go` (makes T046 pass)
- [X] T051 [US5] Implement `api/cmd/beer-menu-api/resetpassword.go` (research R10) and dispatch `reset-password <username>` in `main.go` next to `healthcheck`: refuse unless `term.IsTerminal(stdin)`, prompt twice with `term.ReadPassword`, validate, open the database with `store.Open`, refuse unless `store.Version` equals the newest migration, call `auth.ResetPassword`, log `password reset` with `username`, exit 0 or 1 (makes T047 pass)
- [X] T052 [US5] Implement `web/src/components/AdminsPanel.tsx` and `web/src/components/PasswordForm.tsx` and add both to the signed-in view in `web/src/pages/AdminPage.tsx` (makes T048 pass)

**Checkpoint**: quickstart §5 passes.

---

## Phase 8: User Story 6 - Browser hardening (Priority: P2)

**Goal**: admin and display pages carry the browser security headers; the display still works.

**Independent Test**: request `/`, `/display` and `/api/setup` and check each header appears once
(quickstart §6).

- [X] T053 [US6] Add to `location /` in `web/nginx/default.conf` (not the server block, so `/api` responses keep only the API's headers; research R11), each with `always`: `X-Content-Type-Options nosniff`, `X-Frame-Options DENY`, `Referrer-Policy same-origin`, `Content-Security-Policy "default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"`
- [X] T054 [US6] Extend the compose smoke test in `.github/workflows/ci.yml` to assert, with `curl -sI`, that `/` and `/display` carry the four headers once each and `/api/setup` carries `X-Frame-Options` exactly once and `Cache-Control: no-store`
- [X] T055 [US6] Build the web image, load `/` and `/display` in a browser, and confirm no CSP violations in the console; record the result in the PR description

**Checkpoint**: quickstart §6 passes.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T056 [P] Secrets-in-logs test in `api/internal/httpapi/flow_test.go`: run setup, sign-in, token create and use, password change and sign-out through `NewMux` with `slog` captured, then assert no password, session ID or token secret appears in the logs, and none appears in any response except the one `POST /api/tokens` body (SC-005)
- [X] T057 [P] Update `docker-compose.yaml`: commented `environment:` entries for `BEER_MENU_ADMIN_USERNAME`, `BEER_MENU_ADMIN_PASSWORD: ${BEER_MENU_ADMIN_PASSWORD}` and `BEER_MENU_COOKIE_SECURE`, with a one-line note each and no real values
- [X] T058 [P] Update `README.md`: first admin (env vars or setup screen), `BEER_MENU_COOKIE_SECURE` and when to set it to `false`, `docker compose exec -it api /beer-menu-api reset-password <username>`, token prefix `bmt_` for secret scanners, and the restore note "tokens revoked since the backup must be revoked again"
- [X] T059 [P] Create `.gitleaks.toml` at the repo root with `[extend] useDefault = true` and a rule `beer-menu-api-token` matching `bmt_[A-Za-z0-9_-]{43}`; gitleaks in `.github/workflows/secrets.yml` reads `<source>/.gitleaks.toml` by default, so confirm with a local run that a planted fake token in an untracked file is reported, then delete it
- [X] T060 Confirm `api/openapi.yaml` still matches `specs/002-auth-api-tokens/contracts/openapi.yaml` (copy back any change made during implementation) and that the redocly lint reports no errors
- [X] T061 Run `go vet ./...`, `go test ./...`, `npm run lint`, `npm test`, `npm run build` and the compose smoke test locally; all pass
- [X] T062 Run quickstart §1 to §6 against the built images with `--pull never --build` from a clean data folder and record results in the PR; §7 (behind NPM) runs after the release deploys to TrueNAS; record the §7 sign-in time (SC-008)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: depends on T001 (deps) and T002 (contract, for T009). Blocks all
  stories.
- **US1 (Phase 3)**: after Foundational.
- **US2 (Phase 4)**: after Foundational. Its tests need an admin, created through
  `CreateFirstAdmin` (T024), so do US1 first or create the admin with a test helper.
- **US3 (Phase 5)**: after US2 (it changes `POST /api/session`).
- **US4 (Phase 6)**: after Foundational; UI needs the signed-in shell from US2 (T033).
- **US5 (Phase 7)**: after US2 (sign-in, shell) and US3 (limiter used by `POST /api/password`).
- **US6 (Phase 8)**: after Setup; independent of the other stories, but must merge in the same
  PR as the first admin write action.
- **Polish (Phase 9)**: after all stories.

### User Story Dependencies

```text
Setup ─> Foundational ─┬─> US1 ─> US2 ─> US3 ─> US5
                       ├─> US4 (UI after US2)
                       └─> US6
```

### Within Each User Story

- Tests first, and see them fail.
- `internal/auth` store functions before `httpapi` handlers before web components.
- Register each route in `routes.go` with its `Access` in the same task as the handler.

### Parallel Opportunities

- T002 and T003 in Setup.
- T004 to T011 (all Foundational tests) together; then T013, T014, T015, T016 in parallel
  after T012; T017 after T014 and T015; T018 after T017.
- Within each story, all `[P]` test tasks together.
- US4 and US6 can run beside US1 to US3.

---

## Parallel Example: User Story 4

```bash
# Tests together:
Task: "Store tests in api/internal/auth/tokens_test.go (T039)"
Task: "Handler tests in api/internal/httpapi/tokens_test.go (T040)"
Task: "Component test in web/src/components/TokensPanel.test.tsx (T041)"
```

---

## Implementation Strategy

### MVP First (P1 stories)

1. Phase 1 and Phase 2.
2. US1, US2, US3: a first admin, sign-in and sign-out, rate limiting.
3. **Stop and validate**: quickstart §1 to §3.

### Incremental Delivery

1. Add US4 (tokens), then US5 (admins, passwords, reset), then US6 (nginx headers).
2. Polish, then one PR for the whole feature, because US6 must ship with the first admin
   write action and the dev-pipeline reviews the feature as a whole.

### Notes

- Every code task goes through the dev-pipeline (coding-agent, code-quality, code-security).
- Commit after each task or logical group.
- Stop at any checkpoint to validate the story on its own.
