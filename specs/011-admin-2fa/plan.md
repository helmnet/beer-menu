# Implementation Plan: Two-Factor Sign-In for Admins

**Branch**: `compmodder26/2fa` | **Date**: 2026-11-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/011-admin-2fa/spec.md`

## Summary

Admins can add an authenticator app (TOTP) and any number of passkeys as second factors, with 10
single-use recovery codes. Sign-in becomes two steps for admins with 2FA: the password step
answers 202 and sets a short-lived, single-use `bm_pending` cookie, and a second-step route
(code, recovery code or passkey) starts the session. Biometrics come from passkeys with
`userVerification: "required"`. Sensitive changes need a "confirm" (password plus a factor)
stamped on the session for 5 minutes. A session-only setting requires 2FA for all admins; an
admin without a factor then gets an `enroll` pending that only the factor setup routes accept.
Another admin, or a new `reset-2fa` command, can clear an admin's factors. API tokens are
untouched and refused on every new route.

TOTP is our own RFC 6238 code on the standard library. WebAuthn verification uses
`go-webauthn/webauthn`; the browser side uses the built-in WebAuthn JSON helpers. The QR code is
drawn as SVG from `uqr`. Details in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 7 with React 19 on Node 26 (web build)

**Primary Dependencies**: existing `net/http`, `modernc.org/sqlite`, `golang.org/x/crypto`,
`internal/secrets`; React, Vite, Vitest + Testing Library, `lucide-react`. **New**:
`github.com/go-webauthn/webauthn` v0.18.2 (R1) and `uqr` 0.1.3 (R4), both justified.

**Storage**: SQLite. Migration `0006_two_factor.sql` (v5 to v6): tables `totp_factors`,
`passkeys`, `recovery_codes`, `auth_challenges`; column `sessions.confirmed_at`; settings row
`require_2fa` ([data-model.md](./data-model.md)).

**Testing**: `go test -race` on real SQLite: `auth` unit tests (TOTP vectors and replay, recovery
codes, pending and setup challenges, confirm window, clear and reset); `httpapi` handler tests for
every new route and rule, including tokens refused, cross-site refused, limiter shared with the
second step, identical 401 for wrong password, and the route walk with the new `enroll` level;
passkey ceremonies with a test-only software authenticator (R13); migration test from v5;
`reset-2fa` command test; contract test and Redocly lint. Vitest + Testing Library
(`--maxWorkers=2`): `LoginForm` second step, `TwoFactorPanel`, `TwoFactorSetup`,
`RecoveryCodes`, `ConfirmIdentity`, `QrCode`, `AdminsPanel` badge and clear, `webauthn.ts`
support checks, `AdminPage` enroll view. Manual real-device passkey run (quickstart 4).

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE behind NPM over HTTPS; admin on
laptop and phone browsers and the installed PWA

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: authenticator setup under 2 minutes (SC-001); passkey second step under
10 s (SC-003); the second step adds one SHA-256 or one signature check, no argon2id

**Constraints**: constitution V (HttpOnly SameSite cookies, CSRF on every write, rate-limited
sign-in, secrets sealed with the outside key, token rules unchanged); CSP unchanged (QR as SVG
elements, no `innerHTML`, no style attributes); `POST /api/session` keeps 200 + Admin for admins
without 2FA; passkeys need HTTPS on a host name (R5)

**Scale/Scope**: a handful of admins; 19 new routes; 1 migration; 1 new env var; 1 new CLI
command; 6 new web modules, 5 changed

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Every 2FA action is an API route documented in `api/openapi.yaml` (0.10.0); the admin app only calls the API. `POST /api/session` gains a 202 that only 2FA admins trigger; 200 is unchanged | Pass |
| II. Go + SQLite | `net/http` routes; one versioned migration with an upgrade test; `go-webauthn` is pure Go, so the binary stays static | Pass |
| III. React Clients | Same React + TS build; no display change | Pass |
| IV. Tests Required | Real-SQLite tests for every rule in the spec; auth tests for unauthenticated, tokens refused, CSRF on each new write (including the public second-step routes), limiter on the second step and confirm; migration test; component tests for each new and changed component | Pass |
| V. LAN Security | Second-step and cancel routes are later sign-in steps, public under the 1.4.0 wording, need the pending cookie and refuse cross-site requests; TOTP secrets sealed with `BEER_MENU_SECRET_KEY` and never returned after setup; recovery codes and pending IDs stored hashed; no host name committed (`BEER_MENU_PUBLIC_URL` from env); recovery stays off the network for the sole-admin case (`reset-2fa`); `enroll` pending accepted only on the setup routes, as 1.4.0 allows | Pass (after amendment) |
| VI. Documented | Doc comments on every new exported Go symbol, handler and React component; `x-access: enroll` documented in the OpenAPI intro | Pass |
| VII. Simplicity | Two dependencies, each justified (R1, R4); TOTP on the standard library; one challenge table for all short-lived steps; one confirm route instead of proof fields on six routes; enroll reuses the setup routes | Pass (dependencies justified) |
| Deployment constraints | Still two containers; new optional env var `BEER_MENU_PUBLIC_URL` added to README and `docker-compose.yaml` example; backup before migration as usual | Pass |

Post-design re-check (after Phase 1): passes against constitution 1.4.0. `/speckit-analyze`
found that the second-step routes and the `enroll` pending did not fit principle V as written in
1.3.0 (public list, and "session or token" for everything else). This feature amends principle V
(1.3.0 to 1.4.0, MINOR) to name the later sign-in steps as public, only on a pending sign-in made
by a correct password, and to allow the `enroll` pending on the factor setup routes only. No other
spec or plan depends on the changed wording. R9 asks for an existing factor before adding another
(stricter than FR-023).

## Project Structure

### Documentation (this feature)

```text
specs/011-admin-2fa/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/two-factor.openapi.yaml
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── go.mod                                  # + github.com/go-webauthn/webauthn v0.18.2
├── openapi.yaml                            # routes and schemas per contracts/; 0.9.0 -> 0.10.0
├── cmd/beer-menu-api/
│   ├── main.go                             # BEER_MENU_PUBLIC_URL; reset-2fa dispatch; WebAuthn + Box into Deps
│   ├── startup.go                          # parse and check BEER_MENU_PUBLIC_URL (R5)
│   ├── reset2fa.go                         # new: reset-2fa <username> (R12)
│   └── reset2fa_test.go
└── internal/
    ├── store/migrations/0006_two_factor.sql
    ├── store/migrate_test.go               # v5 -> v6 upgrade case
    ├── auth/
    │   ├── totp.go, totp_test.go           # new: RFC 6238 generate/verify, secret, otpauth URI
    │   ├── twofactor.go, twofactor_test.go # new: factors CRUD, recovery codes, clear, has-2FA, require setting
    │   ├── challenges.go, challenges_test.go # new: pending/enroll/setup rows, expiry, single use
    │   ├── passkeys.go, passkeys_test.go   # new: go-webauthn user adapter, register/verify, sign count
    │   ├── sessions.go                     # confirmed_at stamp and check; delete setup row with session
    │   ├── users.go                        # Admin.TwoFactor in GetAdmin/ListAdmins
    │   └── authtest/authenticator.go       # new, test-only software authenticator (R13)
    └── httpapi/
        ├── routes.go                       # 19 routes; Deps gains WebAuthn, Box
        ├── guard.go                        # AccessEnroll; principal gains Pending
        ├── session.go                      # 202 path in signIn; second-step handlers
        ├── twofactor.go                    # new: /api/2fa/* handlers
        ├── admins.go                       # DELETE /api/admins/{id}/2fa
        ├── twofactor_test.go, session_test.go, admins_test.go, auth_walk_test.go, guard_test.go

web/
├── package.json                            # + uqr 0.1.3
└── src/
    ├── api.ts                              # Admin.two_factor; TwoFactorState, SignInPending, ... types
    ├── webauthn.ts, webauthn.test.ts       # new: support check (incl. origin match), create/get via JSON helpers
    ├── components/LoginForm.tsx            # 202 -> second step (code / passkey / recovery) or enroll
    ├── components/TwoFactorPanel.tsx       # new: state, list, rename, remove, new codes, require switch
    ├── components/TwoFactorSetup.tsx       # new: add authenticator (QR + key) or passkey; used by panel and enroll
    ├── components/ConfirmIdentity.tsx      # new: password + factor modal (reuses Modal)
    ├── components/RecoveryCodes.tsx        # new: show once, copy, download (.txt via Blob)
    ├── components/QrCode.tsx               # new: uqr matrix as one SVG path
    ├── components/AdminsPanel.tsx          # 2FA badge; "Clear 2FA" for other admins
    ├── components/SideNav.tsx              # Administration › Two-Factor Sign-In
    ├── pages/AdminPage.tsx                 # "two-factor" page; enroll view after 202 next=enroll
    └── *.test.tsx for each new and changed component

README.md, docker-compose.yaml              # BEER_MENU_PUBLIC_URL; reset-2fa recovery note
.specify/memory/constitution.md             # principle V amended, 1.3.0 -> 1.4.0 (done with this plan)
```

**Structure Decision**: keep the two-folder layout. All 2FA rules and SQL live in
`internal/auth` (which knows nothing of HTTP, so `reset-2fa` shares it, as `reset-password`
does). HTTP wiring stays in `internal/httpapi`, with one new access level in the existing guard.
The web app adds one page and one setup component that both the page and the enroll view use.

## Complexity Tracking

None. The two new dependencies are justified in research R1 and R4, as principle VII requires.
