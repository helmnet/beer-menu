# Implementation Plan: Brewfather Import

**Branch**: `compmodder26/feature_6` | **Date**: 2026-11-09 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/006-brewfather-import/spec.md`

## Summary

Connect one Brewfather account, list its batches, import them as beers, re-sync imported beers
(measurements, status and archive only), never list or import archived batches, and run a scheduled sync inside the API that also
auto-imports Brewing, Fermenting and Conditioning batches. Three new Go packages:
`brewfather` (the only code that knows Brewfather's API) implements `importer.Provider`;
`importer` holds the provider-neutral service (connection, cache, rate gate, import, sync,
scheduler); `secrets` seals the API key with AES-256-GCM using a key from
`BEER_MENU_SECRET_KEY_FILE` or `BEER_MENU_SECRET_KEY`. Migration 0005 adds a shared `settings`
table, the connection, last sync run and dismissals, and sync columns plus a one-beer-per-batch
index on `beers`. Eight routes, a Brewfather admin panel and Brewfather markers in the beer
list. The API image gains a CA bundle so it can reach Brewfather over HTTPS. No new
dependencies. Details in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http`, `modernc.org/sqlite`; stdlib `crypto/aes`,
`crypto/cipher`, `crypto/rand`, `net/http` client. No new Go or web dependencies.

**Storage**: SQLite; migration `0005_brewfather.sql` (v4 to v5), see
[data-model.md](./data-model.md)

**Testing**: `go test -race` on real SQLite: `secrets` unit tests; `brewfather` client against
`httptest.NewTLSServer`; `importer` with a fake provider and fake clock; `beers.Sanitize`
round-trip; migration from a v4 database; `httpapi` handlers with a fake provider factory,
route walk fixtures for the new read/write routes, admin-only refusal of tokens, secret-leak
checks over responses, captured logs, the database file and a backup; startup config tests.
Vitest + Testing Library for `BrewfatherPanel`, `BeersPanel`, `AdminPage` stubs. Redocly lint
and the `x-access` contract test.

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE behind NPM; outbound HTTPS from
the API container to `api.brewfather.app`

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: a full sync reads only batches that are not archived: about 5 Brewfather
calls (one per active status) however many batches are archived (SC-005 allows 10);
Brewfather actions fail within 20 s when it is down (15 s client timeout, SC-006)

**Constraints**: Brewfather allows 500 calls/hour per key; responses read with a 4 MiB cap;
the key is never returned, logged or stored in plain form; no inline styles (CSP); the API
image is `scratch` (needs a CA bundle, has no tzdata, so brew dates are UTC)

**Scale/Scope**: one account, up to a few hundred batches; 8 routes; 3 new Go packages; 1 new
panel, 2 panels touched

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Every action (connect, list, import, re-sync, schedule, sync now, status) is a route in `api/openapi.yaml`; the panel only calls them. `Beer` gains `sync` as an added field; nothing removed | Pass |
| II. Go + SQLite | Stdlib only; versioned migration 0005; Brewfather behind `importer.Provider`; the core sees only `importer.Batch` and importer error types | Pass |
| III. React Clients | Panel is React + TS in the same build; the TV is untouched and gains on-deck beers through existing status rules | Pass |
| IV. Tests Required | Real-SQLite handler and service tests; fake provider and TLS test server, no real key; route walk covers every new read/write route; admin-only routes tested against tokens and cross-site; migration from v4; component tests | Pass |
| V. LAN Security | Key sealed with AES-256-GCM, key held outside the DB (env or Docker secret), never returned or logged (tests check responses, logs, DB file, backup); connect and schedule admin-only; quota-spending routes write; imported data cleaned by the beer rules; outbound calls HTTPS-only, bounded in time and size, no redirects | Pass |
| VI. Documented | Doc comments on every exported symbol in the three new packages, each handler and `BrewfatherPanel`; README documents the two env vars and key generation | Pass |
| VII. Simplicity | One provider, no plugin registry; in-memory cache and rate gate; scheduler is a one-minute tick in the API process; no new dependency | Pass |
| Deployment constraints | Still two containers. API image adds only a CA bundle file. Backups hold the sealed key only; a restore without the encryption key loses only the connection | Pass |

Post-design re-check (after Phase 1): still passes. One convention extended, recorded below:
`GET /api/brewfather/batches` is the first GET route with write access.

## Project Structure

### Documentation (this feature)

```text
specs/006-brewfather-import/
├── plan.md                        # This file
├── research.md                    # Phase 0 decisions R1 to R14
├── data-model.md                  # Migration 0005, sync state, Brewfather mapping
├── quickstart.md                  # Validation guide
├── contracts/
│   └── brewfather.openapi.yaml    # Fragment merged into api/openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md                       # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── Dockerfile                          # build: apk add ca-certificates; final: copy CA bundle
├── openapi.yaml                        # + brewfather tag, 8 operations, schemas; Beer.sync; 0.6.0
├── cmd/beer-menu-api/
│   ├── main.go                         # read secret key env; build importer.Service; start Run(ctx)
│   ├── startup.go                      # config gains secret key; loadSecretKey (file wins)
│   └── startup_test.go                 # key file/env precedence, malformed refuses start, value never printed
└── internal/
    ├── secrets/
    │   ├── secrets.go                  # Box, NewBox, Seal, Open, ErrCannotOpen, ParseKey
    │   └── secrets_test.go
    ├── brewfather/
    │   ├── client.go                   # Client (BaseURL, HTTP client), New(userID, key) → importer.Provider
    │   ├── mapping.go                  # Brewfather JSON → importer.Batch (FR-012, FR-013)
    │   └── client_test.go              # TLS test server: auth, paging, mapping, errors, limits
    ├── importer/
    │   ├── provider.go                 # Provider, Factory, Batch, error types
    │   ├── service.go                  # Service: Status, Connect, Disconnect, SetInterval, Batches (cache)
    │   ├── importing.go                # Import, Resync, Sync, dismissal clear
    │   ├── scheduler.go                # Run(ctx), Tick(ctx)
    │   ├── store.go                    # settings, provider_connections, sync_runs SQL
    │   ├── importertest/fake.go         # fake Provider for tests
    │   └── *_test.go                   # real SQLite, fake clock
    ├── beers/
    │   ├── beers.go                    # Beer.Sync; columns + sync cols; CreateImported; ApplySync; Delete records dismissal
    │   ├── sanitize.go                 # Sanitize(Input) (Values, []string)
    │   └── *_test.go
    ├── store/migrations/
    │   └── 0005_brewfather.sql
    └── httpapi/
        ├── routes.go                   # + 8 routes; Deps.Importer
        ├── brewfather.go               # handlers + writeImporterError
        ├── brewfather_test.go          # behavior, admin-only vs tokens, no secret in responses/logs
        └── auth_walk_test.go           # fixtures for new read/write routes; cross-site case skipped for GET

web/src/
├── api.ts                              # + Brewfather types; Beer.sync
├── testApi.ts                          # + brewfatherStatus(), batch() fixtures; beer() gains sync
├── components/BrewfatherPanel.tsx      # new (R13)
├── components/BrewfatherPanel.test.tsx
├── components/BeersPanel.tsx           # source label, Re-sync, not-found / archive-pending notes
├── components/BeersPanel.test.tsx
├── pages/AdminPage.tsx                 # + BrewfatherPanel; beerChanges counter
└── pages/AdminPage.test.tsx            # stub GET /api/brewfather

docker-compose.yaml                     # commented BEER_MENU_SECRET_KEY_FILE + secrets example
README.md                               # encryption key: what, how to generate, what losing it means
```

**Structure Decision**: keep the two-folder layout. Brewfather knowledge lives only in
`internal/brewfather`; `internal/importer` is the provider-neutral core that `httpapi` and
`main` use; beer SQL stays in `internal/beers`.

## Complexity Tracking

| Deviation | Why needed | Simpler alternative rejected because |
| --- | --- | --- |
| `GET /api/brewfather/batches` with `x-access: write` (first GET that is not read) | Listing spends the owner's Brewfather quota; read tokens (Home Assistant dashboards) must not burn it (FR-030) | Read access: any read token could exhaust the 500/hour budget. `POST` for a read: misuses the method and breaks caching semantics. The route walk skips only its cross-site case, which does not apply to GET |
| API image gains a CA bundle | `scratch` has no roots, so HTTPS to Brewfather fails | Embedding roots via `x509roots/fallback` is a new module dependency and its roots go stale with the binary |
