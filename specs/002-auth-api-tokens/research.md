# Research: Authentication and API Tokens

Phase 0 decisions for [plan.md](./plan.md). Each entry gives the decision, why, and what was
rejected. Requirement IDs refer to [spec.md](./spec.md).

## R1. Password hashing

- **Decision**: argon2id from `golang.org/x/crypto/argon2`, parameters m=19 MiB, t=2, p=1,
  16-byte random salt, 32-byte key. Stored as a PHC string
  (`$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`), so parameters can be raised later and old
  hashes still verify. Compare with `crypto/subtle.ConstantTimeCompare`.
- **Rationale**: these are the OWASP minimum argon2id settings. They hash in well under 100 ms
  on a NAS CPU (SC-008). argon2id has no input length cap, so the 256-character limit (FR-007)
  needs no pre-hashing. The PHC string keeps the parameters with each hash.
- **Alternatives**: bcrypt (same `x/crypto` module) silently truncates at 72 bytes, which
  conflicts with a 256-character limit. scrypt is not named by the constitution.

## R2. Bounding hash cost

- **Decision**: a package-level semaphore allows at most 4 password hashes at once (login,
  setup, add admin, password change, reset). A request that cannot get a slot in 5 seconds gets
  `503` with `Retry-After`.
- **Rationale**: the rate limit counts failures, not concurrent attempts. Without a cap, a burst
  of parallel logins needs 19 MiB each and could exhaust the NAS's memory.
- **Alternatives**: a global request limiter (more code, also slows reads); no cap (memory risk).

## R3. Equal-time failures (FR-014)

- **Decision**: for an unknown username, verify the password against a fixed dummy hash made at
  startup, then return the same `401 invalid_credentials` body as a wrong password.
- **Rationale**: both paths then do one argon2id verify, so timing does not reveal usernames.
- **Alternatives**: skip the hash for unknown users (leaks usernames by timing).

## R4. Sessions

- **Decision**: a session ID is 32 random bytes from `crypto/rand`, sent base64url in the cookie
  `bm_session`. The database stores only the SHA-256 of the ID. Cookie attributes: `HttpOnly`,
  `SameSite=Strict`, `Path=/`, no `Max-Age` beyond the absolute expiry, and `Secure` unless
  `BEER_MENU_COOKIE_SECURE=false`. Each request looks the session up by hash; it is valid when
  `now < expires_at` (30 days from sign-in) and `now < last_seen_at + 7 days`. `last_seen_at`
  is written at most once a minute. A new ID is minted at every sign-in (FR-017); sign-out,
  password change, reset and admin removal delete rows (FR-011, FR-012, FR-015).
- **Rationale**: server-side rows make sign-out real and let removal end sessions at once.
  Hashing the ID means a leaked backup holds no usable sessions. `SameSite=Strict` is fine
  because the admin app is never entered from another site's link with a needed session.
- **Cookie `Secure` default**: on. Browsers accept `Secure` cookies on `http://localhost`, so the
  example compose file works locally, and production runs behind NPM with TLS. Plain-HTTP access
  by LAN IP needs `BEER_MENU_COOKIE_SECURE=false`, documented in the README. The API never infers
  this from `X-Forwarded-Proto`, which a client can forge.
- **Alternatives**: signed stateless cookies (cannot be revoked); `__Host-` cookie prefix
  (requires `Secure`, so the name would change with the setting).

## R5. CSRF (FR-018)

- **Decision**: wrap the whole mux in the standard library's `http.CrossOriginProtection`
  (Go 1.25+). It refuses non-safe requests whose `Sec-Fetch-Site` is cross-site, or, without
  that header, whose `Origin` host differs from `Host`. Requests with neither header (curl,
  scripts, Home Assistant) pass, which is correct because they carry no ambient cookie. Plus
  `SameSite=Strict` on the cookie as a second layer.
- **Rationale**: no tokens to thread through the React app, no new dependency, and it covers
  every write by construction (constitution V: "on every write"), including the public
  sign-in and first-admin creation (constitution v1.2.0: public writes refuse cross-site
  browser requests). It runs before the guard and the handlers, so a refused cross-site
  sign-in never reaches the rate limiter.
- **Host check behind proxies**: nginx sends `Host $host` and NPM forwards the original Host by
  default, so `Origin` and `Host` match. Browsers that send `Sec-Fetch-Site` (all current ones,
  including Fully Kiosk's WebView) never reach the Host comparison. The quickstart checks it.
- **Alternatives**: double-submit token (client code and a second cookie); custom-header check
  (works, but the stdlib handler already does the stronger thing).

## R6. API tokens (FR-023 to FR-029)

- **Decision**: secret = `bmt_` + base64url(32 random bytes), about 256 bits (FR-025). Stored as
  SHA-256 in a unique column; lookup is by hash. Accepted only from
  `Authorization: Bearer <secret>` (FR-028). `last_used_at` is updated only when it is older
  than 1 minute (FR-029). Revocation sets `revoked_at`; the row stays (FR-027).
- **Rationale**: with 256 random bits, a fast hash is safe; a slow hash would cost every API
  call. The prefix is greppable and suits gitleaks custom rules.
- **Alternatives**: argon2id per request (slow on every call); ID + secret pairs (more format,
  no gain at this scale).

## R7. Authorization model (FR-030 to FR-035)

- **Decision**: add an `Access` field to `httpapi.Route` with four values: `AccessAdmin`
  (session only), `AccessRead` (session, read token or write token), `AccessWrite` (session or
  write token) and `AccessPublic`. `AccessAdmin` is the zero value, so a route that declares
  nothing requires a session (FR-035). `NewMux` wraps each handler in a guard for its access.
  Credential order: a bearer header, if present, decides the request; a cookie never satisfies
  a request that sent a bearer header. No credential: `401`; wrong scope or a token on an admin
  route: `403`.
- **Contract check**: each OpenAPI operation carries `x-access: public|read|write|admin`. The
  existing route test is extended: every route's `Access` must equal its operation's `x-access`,
  and `security` must be `[]` exactly for public routes. Routes with `AccessRead` must be `GET`,
  so a read token can never reach a write by mislabelling.
- **No data endpoints yet**: read and write scope rules are tested on the guard with stub
  handlers, so no test-only route ships.
- **Alternatives**: per-handler checks (easy to forget); a router library with middleware
  groups (new dependency, constitution VII).

## R8. Login rate limiting (FR-019 to FR-022)

- **Decision**: in memory, per lowercased username, 5 failures in a sliding 15-minute window.
  Checked before the password is hashed, so
  a limited attempt costs no hash. Over the limit: `429 too_many_attempts` with `Retry-After` in
  seconds until the oldest counted failure leaves the window. A success does not clear the
  per-username count (an attacker who knows one password gains nothing). Wrong current passwords
  on password change count against the same username.
- **Concurrent attempts**: an attempt that passes the check counts as a failure until its result
  is known, so parallel guesses cannot all pass before the first failure is recorded. When only
  such attempts fill the limit, `Retry-After` is 1 second.
- **Bounded memory**: a username with failures in the window is never evicted, since evicting it
  would reset its count and let a username spray buy more guesses at a victim (security review,
  2026-11-08). Expired entries are pruned on access, and all expired ones are swept when 10,000
  or more usernames are tracked. Every failure follows a password verify, which the hash
  semaphore (R2) throttles, so the map holds at most the failures of one 15-minute window.
- **No global cap**: a cap across all usernames would let any LAN device block every sign-in
  indefinitely, and adds nothing for real usernames, which the per-username limit already
  holds to 5 guesses per 15 minutes. Dropped on 2026-11-08.
- **No client address**: the API never reads `X-Forwarded-For`, `X-Real-IP` or the peer address
  for limiting (FR-020). The peer is always nginx.
- **Alternatives**: persist counters in SQLite (a restart would not reset them, but a LAN
  attacker cannot restart the server; extra writes on every failure).

## R9. First admin (FR-001 to FR-005)

- **Decision**: env vars `BEER_MENU_ADMIN_USERNAME` and `BEER_MENU_ADMIN_PASSWORD`. After
  migrations, if both are set and the users table is empty, startup creates that admin. If an
  admin exists, they are ignored (logged at info, without the password). If only one is set or
  the values break the rules, startup exits with an error. Otherwise `GET /api/setup` reports
  `{"needed": true}` and `POST /api/setup` creates the first admin and signs them in.
- **Atomic**: both paths use `INSERT INTO users (...) SELECT ... WHERE NOT EXISTS (SELECT 1 FROM
  users)` and check rows affected, so concurrent setups create at most one admin (FR-004).
- **Alternatives**: a `_FILE` variant for Docker secrets (not needed: TrueNAS deploys pass repo
  secrets as env; can be added later).

## R10. `reset-password` subcommand (FR-041, FR-042)

- **Decision**: `beer-menu-api reset-password <username>` in `cmd/beer-menu-api`, dispatched like
  `healthcheck`. It refuses unless stdin is a terminal (`golang.org/x/term.IsTerminal`), reads
  the password twice with echo off (`term.ReadPassword`), validates it, opens the database with
  `store.Open`, refuses if the schema version is not the newest (the API must have migrated it),
  then in one transaction updates the hash and deletes that admin's sessions. Unknown username:
  exit 1, nothing changed. Logs `password reset` with the username.
- **Rationale**: the scratch image has no shell; the binary is the only tool inside. SQLite WAL
  and `busy_timeout(5000)` let it write while the API runs. Sessions are looked up per request,
  so deletion takes effect at once.
- **Alternatives**: read stdin without a terminal check (would echo the password and allow
  piping from history-prone shells); an env var reset flag (password left in config).

## R11. Security headers (FR-036, FR-037)

- **Decision**:
  - **API** (a wrapper around the mux): `X-Content-Type-Options: nosniff`,
    `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`,
    `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`, and
    `Cache-Control: no-store` on every `/api` response except health.
  - **nginx** (`location /` only, so proxied API headers are not doubled): `nosniff`,
    `X-Frame-Options: DENY`, `Referrer-Policy: same-origin`, and
    `Content-Security-Policy: default-src 'self'; img-src 'self' data:; object-src 'none';
    base-uri 'self'; form-action 'self'; frame-ancestors 'none'`, all with `always`.
- **Rationale**: the API emits JSON only, so `default-src 'none'` is safe. Vite's build emits no
  inline scripts, and React sets styles through the DOM, which CSP does not block. HSTS belongs
  to NPM, which terminates TLS.
- **Alternatives**: all headers in nginx (tests of the Go API alone would not see them).

## R12. Request validation

- **Decision**: JSON bodies are read through `http.MaxBytesReader` (8 KiB) and decoded with
  `DisallowUnknownFields`. Usernames are trimmed, then must match `^[A-Za-z0-9._-]{1,64}$`;
  stored with `COLLATE NOCASE` (ASCII case folding matches the ASCII-only rule). Passwords 12 to
  256 characters, counted as Unicode code points, never trimmed. Token names 1 to 64 characters
  after trimming. Errors are `{"error": "<code>", "message": "<text>"}`.
- **Alternatives**: Unicode usernames (needs Unicode case folding and confusable handling; not
  worth it for a household).

## R13. Admin app

- **Decision**: no new web dependencies and no router library. `AdminPage` calls
  `GET /api/setup`, then `GET /api/session`, and shows `SetupForm`, `LoginForm` or the signed-in
  view with three sections: admins, tokens and change password. A small `api.ts` wraps `fetch`
  (same origin, JSON, error codes). The new token's secret is shown once with a copy button and
  dropped from state when the dialog closes.
- **Alternatives**: React Router or a data library (constitution VII; three screens do not need
  them).

## R14. Logging (FR-038)

- **Decision**: `slog` events `admin signed in`, `sign-in failed`, `sign-in rate limited`,
  `admin added`, `admin removed`, `password changed`, `password reset`, `token created`,
  `token revoked`, each with `username` or `token_name`. Request bodies, cookies and
  `Authorization` headers are never logged. A test captures log output during a full flow and
  asserts no password, session ID or token secret appears (SC-005).

## R15. New dependencies

| Module | Version | Released | Why |
| --- | --- | --- | --- |
| `golang.org/x/crypto` | v0.58.0 | 2026-10-09 | argon2id (R1); Go team module |
| `golang.org/x/term` | v0.47.0 | 2026-10-09 | no-echo password prompt and terminal check (R10) |

Both are older than two weeks on 2026-11-08. `golang.org/x/sys` is already an indirect
dependency. No web dependencies are added.
