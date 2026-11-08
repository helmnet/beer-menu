# Implementation Plan: Authentication and API Tokens

**Branch**: `compmodder26/auth` | **Date**: 2026-11-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-auth-api-tokens/spec.md`

## Summary

Add admins, sessions and API tokens to the Go API, and the screens for them to the admin app.
Passwords are hashed with argon2id. Sessions are server-side rows keyed by a hashed random ID in
an HttpOnly, SameSite=Strict cookie. CSRF uses the standard library's
`http.CrossOriginProtection`. Tokens are `bmt_` secrets stored as SHA-256 with a `read` or
`write` scope. Every route declares an access level (`public`, `read`, `write`, `admin`, default
`admin`) that `NewMux` enforces and a test checks against `x-access` in the OpenAPI document.
Login is rate limited in memory per username, never by client address. The first
admin comes from env vars at startup or a setup screen. A `reset-password` subcommand recovers a
locked-out install. The API and nginx add security headers. Details are in
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http` mux and `modernc.org/sqlite`; new
`golang.org/x/crypto` (argon2id) and `golang.org/x/term` (no-echo prompt), research R15. No new
web dependencies.

**Storage**: SQLite `/data/beermenu.db`; migration `0002_auth.sql` adds `users`, `sessions`,
`api_tokens` ([data-model.md](./data-model.md)). Rate-limit counters in memory.

**Testing**: `go test` on real SQLite files for handlers, guard, limiter, migration 1 to 2,
startup admin creation and `reset-password`; Vitest + Testing Library for the new components;
redocly lint; route `x-access` contract test.

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE, behind NPM

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: sign-in under 1 s on the NAS (SC-008); per-request auth is one indexed
lookup plus at most one write a minute

**Constraints**: no client-address trust for limiting; at most 4 concurrent password hashes;
8 KiB JSON bodies; no secrets in logs, responses (except the one-time token) or plain in the DB

**Scale/Scope**: one household, a few admins and tokens; 13 operations, 12 of them new (see contract)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Every capability is an endpoint in `api/openapi.yaml` with `x-access`; the admin app uses only these. `reset-password` is an operator tool on the same binary, not a client capability (FR-042 forbids it over HTTP) | Pass |
| II. Go + SQLite | stdlib mux and `CrossOriginProtection`; static binary; versioned migration 0002; no provider code | Pass |
| III. React Clients | React + TS; same origin through nginx; display unaffected except headers (checked in quickstart §6) | Pass |
| IV. Tests Required | Tests for each rule: unauthenticated, `read` vs `write`, revoked token, CSRF on session writes, rate limiting; migration 0002 applied to v1; handler tests on real SQLite; component tests | Pass |
| V. LAN Security | No default credentials; argon2id; HttpOnly SameSite cookie; CSRF on every write; tokens shown once, hashed, Bearer only, revocable, last-used, scoped; public only for menu reads (none yet) and the bootstrap endpoints: health, setup status, first-admin creation, sign-in (constitution v1.2.0 wording); rate limited; validated input; no secrets in repo (env vars from `BEER_MENU_<VAR>` repo secrets) | Pass |
| VI. Documented | Doc comments on exported symbols, handlers and components; reviewed | Pass |
| VII. Simplicity | Two Go team modules, both justified (R15); no router, CSRF or session library; limiter and sessions in a few hundred lines | Pass |
| Deployment constraints | Unchanged images and users; backups now hold hashes, already `0600`/`0700`; restore note added | Pass |

Post-design re-check (after Phase 1): still passes. Both new modules are older than two weeks.

## Project Structure

### Documentation (this feature)

```text
specs/002-auth-api-tokens/
├── plan.md              # This file
├── research.md          # Phase 0 decisions R1 to R15
├── data-model.md        # users, sessions, api_tokens, access table
├── quickstart.md        # Validation guide
├── contracts/
│   └── openapi.yaml     # Full API contract (copied to api/openapi.yaml)
├── checklists/
│   └── requirements.md
└── tasks.md             # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── go.mod                              # + golang.org/x/crypto, golang.org/x/term
├── openapi.yaml                        # replaced by contracts/openapi.yaml
├── cmd/beer-menu-api/
│   ├── main.go                         # + reset-password dispatch, admin env vars, cookie config
│   ├── startup.go                      # + first admin from env after migrate, session sweep
│   └── resetpassword.go                # reset-password subcommand (R10)
└── internal/
    ├── store/migrations/
    │   └── 0002_auth.sql               # users, sessions, api_tokens
    ├── auth/                           # new package: no HTTP
    │   ├── password.go                 # argon2id hash/verify, PHC format, semaphore, rules
    │   ├── limiter.go                  # per-username failure limiter
    │   ├── users.go                    # create first/add/list/remove admins, change password
    │   ├── sessions.go                 # create, look up, touch, delete sessions
    │   ├── tokens.go                   # create, list, revoke, look up tokens
    │   └── *_test.go
    └── httpapi/
        ├── routes.go                   # + Access field, guard in NewMux, headers + CSRF wrap
        ├── guard.go                    # credential resolution and access check (R7)
        ├── headers.go                  # API security headers, no-store (R11)
        ├── json.go                     # body limits, decode, error responses (R12)
        ├── setup.go                    # GET/POST /api/setup
        ├── session.go                  # POST/GET/DELETE /api/session, POST /api/password
        ├── admins.go                   # GET/POST /api/admins, DELETE /api/admins/{id}
        ├── tokens.go                   # GET/POST /api/tokens, DELETE /api/tokens/{id}
        └── *_test.go                   # + x-access contract test, per-rule auth tests

web/
├── nginx/default.conf                  # + security headers in location / (R11)
└── src/
    ├── api.ts                          # fetch wrapper, error codes
    ├── pages/AdminPage.tsx             # setup / login / signed-in switch
    ├── components/SetupForm.tsx
    ├── components/LoginForm.tsx
    ├── components/AdminsPanel.tsx
    ├── components/TokensPanel.tsx      # one-time secret display
    ├── components/PasswordForm.tsx
    └── **/*.test.tsx

docker-compose.yaml                     # + commented admin env vars, BEER_MENU_COOKIE_SECURE
README.md                               # + first admin, cookie setting, reset-password, restore note
```

**Structure Decision**: keep the two-folder layout from feature 1. A new `internal/auth`
package holds the auth rules and SQL with no HTTP, so `httpapi` handlers and the
`reset-password` subcommand share one implementation. `httpapi` stays the single route table.

## Complexity Tracking

No constitution violations. Table intentionally empty.
