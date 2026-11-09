# Implementation Plan: Taps and Kegs

**Branch**: `compmodder26/feature_4` | **Date**: 2026-11-09 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-taps-and-kegs/spec.md`

## Summary

Add taps and kegs to the Go API and the admin app. Migration 0004 adds `taps` (seeded with one
tap) and `kegs`. A keg is on a tap, waiting off-tap (on deck), or kicked (history); a partial
unique index allows one current keg per tap. Thirteen endpoints follow feature 3's access model:
reads need a session or any token, writes a session or a `write` token. PATCH on a keg corrects
it, sets its level, moves it on or off a tap, or kicks it; `POST /api/kegs/{id}/pours` subtracts
a pour atomically, for Home Assistant and keg scales. `GET /api/keg-types` serves the fixed table
of nine keg types with default sizes and drawing shape. Beer reads gain `on_tap`; archiving a
beer with a current keg and deleting a beer with any keg are refused. The admin app gains a Taps
panel with on-deck kegs, a keg form, keg history, gallon and fl oz conversion, and a sanke keg
drawing in `BeerArt`. Details are in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http` mux, `modernc.org/sqlite`, `golang.org/x/crypto`.
No new Go or web dependencies.

**Storage**: SQLite `/data/beermenu.db`; migration `0004_taps_kegs.sql` adds `taps` and `kegs`
([data-model.md](./data-model.md)). DSN gains `_txlock=immediate` (research R3).

**Testing**: `go test -race` on real SQLite for the `kegs` package, beer guards, handlers,
route-walk auth and CSRF, migration 3 to 4, concurrency (pours, racing keg on one tap); Vitest +
Testing Library for `units.ts`, `BeerArt`, `TapsPanel`, `KegForm`, `KegHistory`, `BeersPanel`;
redocly lint; the existing `x-access` contract test.

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE, behind NPM

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: 20 taps and 1,000 kicked kegs list in under 1 s on the NAS (SC-005)

**Constraints**: request bodies stay at the 8 KiB default; volumes in litres in the API, gallons
and fl oz only in the admin app (FR-065); artwork uses SVG presentation attributes so the nginx
CSP is unchanged; pour and level endpoints are a compatibility contract (principle I)

**Scale/Scope**: one household, tens of taps, hundreds to a few thousand kegs; 13 new operations;
3 new web components, 1 new drawing, 1 helper module

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Every capability is an endpoint with `x-access`; keg types served by the API so clients share one table; pours and level sets reachable with a `write` token and documented as stable | Pass |
| II. Go + SQLite | stdlib only; versioned migration 0004; volumes stored in litres; gallons only in the web display | Pass |
| III. React Clients | React + TS; same origin; `BeerArt` stays one reusable component for feature 5 | Pass |
| IV. Tests Required | Handler tests on real SQLite; route-walk auth per route (no credential, `read` vs `write`, revoked, CSRF, session and `write` succeed); migration 0004 applied to v3; concurrency tests; component tests | Pass |
| V. LAN Security | Nothing new is public; all writes need a session or `write` token and refuse cross-site; input validated, unknown fields refused, bodies capped; ids from paths parsed as positive integers | Pass |
| VI. Documented | Doc comments on the new package, handlers and components | Pass |
| VII. Simplicity | No new dependencies; kick, move and level via PATCH like archive; one extra route (pours) justified by concurrency; no pour log or stats | Pass |
| Deployment constraints | Images, users and volumes unchanged; backups still cover all data | Pass |

Post-design re-check (after Phase 1): still passes. Two additions beyond the spec's wording:
`GET /api/keg-types` (read access, keeps the type table defined once, as `/api/srm-colors` did)
and `_txlock=immediate` on the DSN (all transactions take the write lock up front; existing
transactions already write, so behavior is unchanged).

## Project Structure

### Documentation (this feature)

```text
specs/004-taps-and-kegs/
├── plan.md                     # This file
├── research.md                 # Phase 0 decisions R1 to R9
├── data-model.md               # taps, kegs, states, API representations
├── quickstart.md               # Validation guide
├── contracts/
│   └── taps-kegs.openapi.yaml  # Fragment merged into api/openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md                    # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── openapi.yaml                        # + taps, kegs tags, paths, schemas; Beer.on_tap; 409s; 0.4.0
└── internal/
    ├── store/open.go                   # + _txlock=immediate (R3)
    ├── store/migrations/
    │   └── 0004_taps_kegs.sql          # taps (seeded), kegs, indexes
    ├── store/migrate_test.go           # + 0004 applied to a v3 database
    ├── beers/beers.go                  # + on_tap, archive and delete guards (R6)
    ├── kegs/                           # new package: no HTTP
    │   ├── taps.go                     # Tap, List, Get, Create, Update, Reorder, Delete
    │   ├── kegs.go                     # Keg, List, Get, Create, Update, Pour, Delete
    │   ├── types.go                    # keg type table (R4)
    │   ├── validate.go                 # inputs, rounding, FieldErrors, conflict errors (R5)
    │   └── *_test.go
    └── httpapi/
        ├── routes.go                   # + 13 routes
        ├── beers.go                    # 409 for the new beer errors
        ├── taps.go, kegs.go            # handlers, writeKegError
        └── *_test.go                   # handlers; route-walk auth generalized to every route

web/src/
├── api.ts                              # + Tap, Keg, KegType types; Beer.on_tap
├── units.ts                            # litres <-> US gallons, fl oz (R8)
├── pages/AdminPage.tsx                 # + TapsPanel, KegHistory
├── components/BeerArt.tsx              # + kegShape prop, SankeKeg drawing
├── components/BeersPanel.tsx           # passes onTap={beer.on_tap}
├── components/BeerForm.tsx             # preview passes onTap
├── components/TapsPanel.tsx            # taps, keg actions, on deck
├── components/KegForm.tsx              # add/edit keg
├── components/KegHistory.tsx           # kicked kegs
└── **/*.test.ts(x)
```

**Structure Decision**: keep the two-folder layout. A new `internal/kegs` package mirrors
`internal/beers`: rules and SQL with no HTTP. `httpapi` stays the single route table.

## Complexity Tracking

No constitution violations. Table intentionally empty.
