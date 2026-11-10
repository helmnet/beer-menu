# Implementation Plan: Display Settings

**Branch**: `compmodder26/feature_7` | **Date**: 2026-11-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/007-display-settings/spec.md`

## Summary

Four display settings (title, TV theme dark/light, admin volume units US/metric, TV page
interval 5 to 300 s), stored as rows in the existing `settings` table with defaults in code, so
no migration. A new `settings` package owns all `settings` SQL (the generic get/set move here
from `importer`). Two routes: `GET /api/settings` (read) and `PATCH /api/settings` (write,
merge patch, all-or-nothing). The public `GET /api/menu` gains a `display` object (title, theme,
page interval, not units), so the TV picks changes up on its existing 10 s poll with no new
public route. The TV draws a title row, a light theme through CSS custom properties under a
`light` class (no inline styles), and turns pages at the set interval. The admin app gains a
Settings panel and shows and takes volumes in the chosen units. No new dependencies. Details in
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: existing `net/http`, `modernc.org/sqlite`; React, Vite, Vitest +
Testing Library. No new Go or web dependencies.

**Storage**: SQLite, existing `settings` table; no schema change, database stays at version 5
(see [data-model.md](./data-model.md))

**Testing**: `go test -race` on real SQLite: `settings` unit tests (defaults, bad stored values,
validation, all-or-nothing, importer key still works); `httpapi` handler tests for both routes
and the menu's `display`; route walk fixtures for the two new routes; an upgrade test on a v5
database with feature 6 data and no display rows; contract test (`x-access` both ways) and
Redocly lint. Vitest + Testing Library: `units`, `SettingsPanel`, `DisplayPage`, `TapsPanel`,
`KegForm`, `KegHistory`, `AdminPage` stubs. Headless Firefox screenshots of both TV themes.

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE behind NPM; TV is Fully Kiosk on
a Fire TV Stick at 1080p

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: a setting change shows on the TV within one menu check (10 s poll, ≤ 30 s,
SC-001); the menu read adds one indexed lookup of four keys

**Constraints**: no inline styles (CSP has no `style-src`); text contrast ≥ 4.5:1 in both themes
(FR-011); the public menu must not expose units or any non-display setting (FR-004); `/api/menu`
stays backward compatible (added field only)

**Scale/Scope**: 4 settings; 2 routes; 1 new Go package (plus moved helpers); 1 new panel; 4
web components touched; 1 stylesheet reworked to custom properties

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Settings read and written through `/api/settings`, described in `api/openapi.yaml`; the panel and the TV only call the API. `/api/menu` gains `display` as an added field, nothing removed | Pass |
| II. Go + SQLite | Stdlib only; no schema change, so no migration; volumes stay litres and units are a setting, exactly as II states | Pass |
| III. React Clients | Panel and TV changes in the same React + TS build; dark stays the TV default; the TV still updates and recovers unattended (settings ride the existing poll, bad `display` falls back to defaults) | Pass |
| IV. Tests Required | Real-SQLite handler and package tests; route walk covers both new routes (401, 403 read token on PATCH, revoked, cross-site, write token and session); no migration, so the upgrade test opens a v5 database with existing data; component tests for every touched component | Pass |
| V. LAN Security | Writes need a session or `write` token with CSRF on browser writes (existing middleware); public read limited to title, theme, page interval; input validated at the boundary (length, control characters, enums, integer range), unknown fields and `null` refused; title rendered as text only | Pass |
| VI. Documented | Doc comments on the `settings` package's exported symbols, both handlers and `SettingsPanel`; doc of `DisplayPage` and `units.ts` updated | Pass |
| VII. Simplicity | No new table, migration, dependency, context provider or public route; logo and custom colors left out per owner | Pass |
| Deployment constraints | Still two containers; no new env vars; no migration, so no backup is taken on upgrade (none needed) | Pass |

Post-design re-check (after Phase 1): still passes. One visual change to the dark TV is
recorded in research R6: the pager color moves from `#6f6f78` (3.95:1) to `#8c8c95` (5.9:1) so
every TV text meets FR-011. No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/007-display-settings/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/settings.openapi.yaml
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── openapi.yaml                        # + /api/settings GET/PATCH, Menu.display; 0.6.0 -> 0.7.0
└── internal/
    ├── settings/                       # new
    │   ├── settings.go                 # Get, Set (moved from importer), over DB or Tx
    │   ├── display.go                  # Display, Public, DisplayPatch, GetDisplay, PatchDisplay
    │   └── *_test.go
    ├── importer/store.go               # GetSetting/SetSetting removed; calls settings.Get/Set
    └── httpapi/
        ├── routes.go                   # + 2 routes
        ├── settings.go                 # getSettings, updateSettings handlers
        ├── settings_test.go            # behavior, validation, upgrade from v5 data
        ├── menu.go                     # response embeds kegs.Menu + display
        ├── menu_test.go                # display present, units absent
        └── auth_walk_test.go           # fixtures for both routes

web/src/
├── api.ts                              # + Settings, MenuDisplay; Menu.display
├── testApi.ts                          # + settings() fixture; menu() gains display
├── units.ts                            # Units type; volume/pour helpers by units (R9)
├── units.test.ts
├── components/SettingsPanel.tsx        # new (R10)
├── components/SettingsPanel.test.tsx
├── components/TapsPanel.tsx            # units prop; KegForm keyed by units
├── components/KegForm.tsx              # units prop
├── components/KegHistory.tsx           # units prop
├── components/*.test.tsx               # metric cases
├── pages/AdminPage.tsx                 # units state; SettingsPanel
├── pages/AdminPage.test.tsx            # stub GET /api/settings
├── pages/DisplayPage.tsx               # title, light class, page interval, display fallback
├── pages/DisplayPage.test.tsx
└── pages/DisplayPage.css               # colors as custom properties; .display.light; .title
```

**Structure Decision**: keep the two-folder layout. All `settings` SQL lives in
`internal/settings`; `kegs` stays unaware of settings (the menu handler joins them); the TV's
theme stays in its one scoped stylesheet.

## Complexity Tracking

None.
