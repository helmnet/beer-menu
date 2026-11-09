# Implementation Plan: Beer Library

**Branch**: `compmodder26/feature_3` | **Date**: 2026-11-09 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/003-beer-library/spec.md`

## Summary

Add the beer library to the Go API and the admin app. A new `beers` table (migration 0003)
holds each beer's details, status, source fields and archived time. Five beer endpoints follow
the feature 2 access model: list and read need a session or any token (`read` or `write`);
create, PATCH (edit, archive, restore) and delete need a session or a `write` token. The full
library is not a menu read; feature 5 adds the public `GET /api/menu` for the TV. PATCH uses
merge-patch semantics. The API owns one SRM color table: every beer read includes its `color`,
and `GET /api/srm-colors` serves the table for the admin form's live preview. The admin app
gains a beer library panel, an add/edit form, and a `BeerArt` component that draws original
inline-SVG artwork filled with the beer's color: a pint glass for beers on a tap (feature 4), a
corny keg otherwise, so every beer is a keg in this feature. There are no uploads.
Details are in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http` mux, `modernc.org/sqlite`, `golang.org/x/crypto`.
No new Go or web dependencies.

**Storage**: SQLite `/data/beermenu.db`; migration `0003_beers.sql` adds `beers`
([data-model.md](./data-model.md)). No files outside the database.

**Testing**: `go test` on real SQLite for the `beers` package, handlers, per-route auth and
CSRF, and migration 2 to 3; Vitest + Testing Library for `BeerArt`, `BeerForm`, `BeersPanel`;
redocly lint; the existing route `x-access` contract test (research R11).

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE, behind NPM

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: list of 500 beers under 1 s on the NAS (SC-006); artwork makes no network
request (SC-005)

**Constraints**: beer write bodies at most 16 KiB, other routes stay at 8 KiB (R5); artwork uses
SVG presentation attributes so the nginx CSP is unchanged (R7); source fields read-only (R3)

**Scale/Scope**: one household, hundreds of beers; 6 new operations (5 beer, 1 color table);
3 new web components

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Every capability is an endpoint documented with `x-access`. The color table is served by the API, so all clients share it. Artwork is presentation of API data (`status`, `color`), not a capability | Pass |
| II. Go + SQLite | stdlib only; versioned migration 0003; `source`/`source_ref` are generic columns, no Brewfather-specific code | Pass |
| III. React Clients | React + TS; same origin; `BeerArt` sized by `viewBox` for reuse on the 1080p display | Pass |
| IV. Tests Required | Handler tests on real SQLite; per write route: unauthenticated, `read` vs `write`, revoked token, CSRF; per read route: no credential and revoked token 401; migration 0003 applied to v2; component tests for all three components | Pass |
| V. LAN Security | Nothing public: beer list, beer and color table need a session or any token (the public menu read is feature 5's `GET /api/menu`); every write needs a session or `write` token and refuses cross-site requests; all input validated, unknown fields refused, body capped; no uploads, so no file-handling surface | Pass |
| VI. Documented | Doc comments on the new package, handlers and components | Pass |
| VII. Simplicity | No new dependencies; archive via PATCH instead of extra routes; no index or columns for features 4 and 6 | Pass |
| Deployment constraints | Images, users and volumes unchanged; backups still cover all data (database only) | Pass |

Post-design re-check (after Phase 1): still passes. One addition beyond the spec's endpoint
list, `GET /api/srm-colors`, needs a session or any token like the other beer reads, and keeps
the table defined once (R6).

## Project Structure

### Documentation (this feature)

```text
specs/003-beer-library/
├── plan.md                  # This file
├── research.md              # Phase 0 decisions R1 to R11
├── data-model.md            # beers table, API representation, color table
├── quickstart.md            # Validation guide
├── contracts/
│   └── beers.openapi.yaml   # Fragment merged into api/openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md                 # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── openapi.yaml                        # + beers tag, paths, schemas; version 0.3.0
└── internal/
    ├── store/migrations/
    │   └── 0003_beers.sql              # beers table
    ├── store/migrate_test.go           # + 0003 applied to a v2 database
    ├── beers/                          # new package: no HTTP
    │   ├── beers.go                    # Beer type, Create, Get, List, Update, Delete
    │   ├── validate.go                 # field rules, FieldErrors (R4)
    │   ├── optional.go                 # Optional[T] for merge-patch (R2)
    │   ├── srm.go                      # color table and lookup (R6)
    │   └── *_test.go
    └── httpapi/
        ├── routes.go                   # + 6 routes
        ├── json.go                     # decodeJSON takes a byte limit (R5)
        ├── beers.go                    # handlers
        └── beers_test.go               # handlers, auth per route, CSRF

web/src/
├── api.ts                              # + Beer, BeerStatus types; ApiError gains fields map
├── pages/AdminPage.tsx                 # + BeersPanel in the signed-in view
├── components/BeerArt.tsx              # inline-SVG glass and keg (R7)
├── components/BeerForm.tsx             # add/edit with live preview
├── components/BeersPanel.tsx           # library list, archived toggle, actions
└── components/*.test.tsx
```

**Structure Decision**: keep the two-folder layout. A new `internal/beers` package mirrors
`internal/auth`: rules and SQL with no HTTP, so feature 6's importer can reuse them. `httpapi`
stays the single route table. `BeerArt` is a standalone component so feature 5's display imports
it unchanged.

## Complexity Tracking

No constitution violations. Table intentionally empty.
