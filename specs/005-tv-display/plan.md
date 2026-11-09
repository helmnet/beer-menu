# Implementation Plan: TV Display

**Branch**: `compmodder26/feature_5` | **Date**: 2026-11-09 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-tv-display/spec.md`

## Summary

Add the public `GET /api/menu` and replace the `/display` placeholder with the TV tap list. The
menu is built by `kegs.GetMenu` from three reads: enabled taps in position order with their current
keg and beer, waiting kegs, and conditioning or fermenting beers with no current keg. It returns
only display fields, with the keg level as a 0..1 fraction (no volumes, ids or timestamps). The
display polls it every 10 seconds, keeps the last good menu through failures, shows an
"offline" notice after 12 failed checks, and reloads itself about once a day after a good
check. `BeerArt` gains a `level` prop that fills the glass or keg to that fraction; the TV and
the admin Taps panel use it, and the panel's `<meter>` goes. Layout is a CSS grid from the
app's first stylesheet, with size classes by tap count. No schema change, no new dependencies.
Details are in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http` mux, `modernc.org/sqlite`. No new Go or web
dependencies.

**Storage**: SQLite, read only for this feature; no migration ([data-model.md](./data-model.md))

**Testing**: `go test -race` on real SQLite for `kegs.GetMenu` (selection, order, exclusions,
level) and the handler (no credential, exact field set, 405, no-store); `TestPublicRoutes`
updated; Vitest + Testing Library with fake timers for `DisplayPage` (render, polling, last menu
kept, offline notice, daily reload), `BeerArt` (level geometry, sliver, aria-label) and
`TapsPanel` (art instead of meter); redocly lint; the existing `x-access` contract test

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE behind NPM; display in Fully
Kiosk (Android System WebView) on a Fire TV Stick at 1920×1080

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: menu read under 1 s with 20 taps, 50 on-deck items and 1,000 kegs
(SC-006); a change visible on the TV within 30 s (SC-002)

**Constraints**: no inline styles (nginx CSP has no `style-src`); no proxy changes (polling,
research R1); nothing on the display depends on the TV clock (R2); the menu read is a
compatibility contract (principle I)

**Scale/Scope**: one TV, up to about 12 taps tuned (more still fit); 1 new endpoint; 1 page
rewritten, 1 component extended, 1 panel touched, 1 stylesheet

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | The TV reads the same API any client can; `GET /api/menu` is in `api/openapi.yaml` with `x-access: public` and documented as stable | Pass |
| II. Go + SQLite | stdlib only; no migration needed; volumes stay litres in storage, the menu sends a fraction | Pass |
| III. React Clients | `/display` is React + TS in the same build and origin; dark, 3 m sizes, polls without reload, recovers from drops and restarts, reloads only after a good check | Pass |
| IV. Tests Required | Handler and query tests on real SQLite; public-route pin updated; tests that every other beer, tap and keg read still needs a credential (existing route walk) and that the menu refuses other methods; component tests for the page, art and panel | Pass |
| V. LAN Security | The only new public route is a menu read, which V names as public; separate response structs carry display fields only; read-only, no body, no-store | Pass |
| VI. Documented | Doc comments on `Menu`, its types, `getMenu`, `DisplayPage`, and the `level` prop | Pass |
| VII. Simplicity | Polling over SSE; no rate limiter, no version-check reload, no new package or dependency; one stylesheet | Pass |
| Deployment constraints | Images, users, volumes and nginx config unchanged | Pass |

Post-design re-check (after Phase 1): still passes. One addition beyond the spec's wording:
`apiFetch` gains an optional `AbortSignal` argument for the display's 8-second timeout (R9);
existing callers are unchanged.

## Project Structure

### Documentation (this feature)

```text
specs/005-tv-display/
├── plan.md                  # This file
├── research.md              # Phase 0 decisions R1 to R11
├── data-model.md            # Menu shape and selection rules
├── quickstart.md            # Validation guide
├── contracts/
│   └── menu.openapi.yaml    # Fragment merged into api/openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md                 # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── openapi.yaml                     # + menu tag, GET /api/menu, Menu schemas; 0.5.0
└── internal/
    ├── kegs/
    │   ├── menu.go                  # Menu, MenuTap, OnDeckItem, MenuBeer, MenuKeg; GetMenu(db)
    │   └── menu_test.go             # selection, order, exclusions, level, empty menu
    └── httpapi/
        ├── routes.go                # + GET /api/menu, AccessPublic
        ├── menu.go                  # getMenu handler
        ├── menu_test.go             # no credential, exact fields, 405, no-store, session/token too
        └── routes_test.go           # TestPublicRoutes want + GET /api/menu

web/src/
├── api.ts                           # + Menu types; apiFetch optional signal
├── testApi.ts                       # + menu() fixture
├── components/BeerArt.tsx           # + level prop (R5)
├── components/BeerArt.test.tsx      # + level geometry, sliver, aria-label
├── components/TapsPanel.tsx         # art with level replaces <meter>
├── components/TapsPanel.test.tsx    # meter assertions updated
├── pages/DisplayPage.tsx            # tap list, on deck, polling, offline, daily reload
├── pages/DisplayPage.css            # dark theme, grid size classes, on-deck strip
└── pages/DisplayPage.test.tsx       # new
```

**Structure Decision**: keep the two-folder layout. The menu query joins the `kegs` package,
which already owns taps, kegs and keg types; `httpapi` stays the single route table.

## Complexity Tracking

No constitution violations. Table intentionally empty.
