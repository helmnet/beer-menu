# Implementation Plan: TV Display Options

**Branch**: `compmodder26/display_options_additions` | **Date**: 2026-11-11 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/010-tv-display-options/spec.md`

## Summary

Eleven new display settings (layout, title alignment, title and text fonts, five tap detail
switches, title and text colors), stored as rows in the existing `settings` table with defaults
in code, so no migration. They ride the existing `GET`/`PATCH /api/settings` routes and the
public menu's `display` object; tap beers in the menu also gain `srm` and `description`. The TV
drawing moves out of `DisplayPage` into a pure `TvView` component that both the TV and a new
live preview in Display Settings render, so the two cannot drift. Two new layouts (list,
columns) page at 6 rows. Seven fonts ship with the app from `@fontsource` packages and load from
the app's own origin. Colors are validated `#rrggbb` applied as CSS custom properties through
CSSOM, which the CSP allows. TV sizes switch from raw `vh`/`vw` to unit variables so the preview
can scale with container units while the TV keeps `vh`. Details in [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 7 with React 19 on Node 26 (web build)

**Primary Dependencies**: existing `net/http`, `modernc.org/sqlite`; React, Vite, Vitest +
Testing Library. **New**: 7 `@fontsource/*` 5.3.0 packages (CSS + woff2 only, no runtime
code), justified in research R5.

**Storage**: SQLite, existing `settings` table; no schema change, database stays at version 5
([data-model.md](./data-model.md))

**Testing**: `go test -race` on real SQLite: `settings` unit tests for each new field (default,
bad stored value, validation messages, boolean and color storage, all-or-nothing); `httpapi`
handler tests (types, `null`, unknown fields); menu tests (`display` fields, no `units`, `srm`
and `description` on tap beers only); an upgrade test on a v5 database with no new rows;
contract test and Redocly lint. Vitest + Testing Library (`--maxWorkers=2`): `tv.ts`
(`displayOf` fallbacks, `pagesOf` per layout), `TvView` (layouts, details, clamp class, fonts,
colors, alignment), `DisplayPage` (existing tests unchanged), `SettingsPanel` (new fields,
changed-only save, preview follows draft, sample menu fallback, Previous/Next). Headless Firefox
screenshots through the nginx CSP (quickstart 2, 5, 6, 7).

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE behind NPM; TV is Fully Kiosk on
a Fire TV Stick at 1080p; admin on laptop and phone browsers

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: setting change on the TV within one menu check (≤ 30 s, SC-001); preview
updates on the next render (< 1 s, SC-008); the TV downloads at most 2 font files

**Constraints**: CSP `default-src 'self'` with no `style-src` (no style attributes in markup, no
external fonts); `grid` layout and defaults must be pixel-identical to today (SC-002); `/api/menu`
stays backward compatible (added fields only); theme-color text keeps 4.5:1 (FR-011)

**Scale/Scope**: 11 settings; 0 new routes; 2 new menu fields; 3 new web modules (`TvView`,
`tv.ts`, `fonts.ts`); 2 reworked (`DisplayPage`, `SettingsPanel`); 1 stylesheet extended

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | All settings read and written through `/api/settings`, described in `api/openapi.yaml` (0.9.0); TV and preview only call the API. Menu changes are additions | Pass |
| II. Go + SQLite | Stdlib only; no schema change, no migration | Pass |
| III. React Clients | Same React + TS build; dark stays the default; TV still recovers unattended (bad `display` fields fall back to defaults, as feature 7) | Pass |
| IV. Tests Required | Real-SQLite package and handler tests for every field; upgrade test; component tests for every new and touched component; real-browser CSP check | Pass |
| V. LAN Security | No new route; writes keep session or `write` token with CSRF; enums, booleans and a strict `#rrggbb` pattern validated at the boundary, so no CSS reaches the page; description rendered as text; public menu gains only display text the owner writes; fonts self-hosted, CSP unchanged | Pass |
| VI. Documented | Doc comments on new exported Go symbols, `TvView`, `tv.ts` and `fonts.ts` exports; `DisplayPage` and `SettingsPanel` docs updated, including the narrowed inline-style rule (R4) | Pass |
| VII. Simplicity | No new table, route or migration. One new dependency family (`@fontsource`, data only), justified in R5. Preview reuses the TV drawing instead of a second one | Pass (dependency justified) |
| Deployment constraints | Still two containers; no new env vars; no migration | Pass |

Post-design re-check (after Phase 1): still passes. Two recorded deviations from earlier
features' choices, both in research: the public menu now carries `srm` and `description` for tap
beers (R9, spec FR-006 amended), and the "no inline styles" convention narrows to "no style
attributes in markup" (R4). No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/010-tv-display-options/
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
├── openapi.yaml                        # schemas per contracts/; 0.8.0 -> 0.9.0
└── internal/
    ├── settings/
    │   ├── display.go                  # 11 fields in Display, Public, DisplayPatch, Default; validators
    │   └── display_test.go
    ├── kegs/
    │   ├── menu.go                     # MenuBeer.SRM, .Description (omitempty), set in the taps query only
    │   └── menu_test.go
    └── httpapi/
        ├── settings.go                 # 11 raw fields in the PATCH body
        ├── settings_test.go            # types, null, upgrade from v5
        └── menu_test.go                # display fields, no units; srm/description on taps only

web/
├── package.json                        # + 7 @fontsource packages
└── src/
    ├── api.ts                          # Settings, MenuDisplay, MenuBeer fields; Layout, FontKey types
    ├── testApi.ts                      # fixtures gain the new fields
    ├── fonts.ts                        # new: imports font CSS; FONTS list (key, label, class)
    ├── tv.ts                           # new: displayOf, pagesOf, usePager, SAMPLE_MENU
    ├── tv.test.ts
    ├── fonts.test.ts
    ├── components/TvView.tsx           # new: the TV drawing for one page
    ├── components/TvView.test.tsx
    ├── components/SettingsPanel.tsx    # new fields, color reset, preview with pager
    ├── components/SettingsPanel.test.tsx
    ├── pages/DisplayPage.tsx           # polling only; renders TvView
    ├── pages/DisplayPage.test.tsx      # unchanged behavior
    └── pages/DisplayPage.css           # --vh/--vw units; list and columns rows; align, font,
                                        # color, clamp classes; .tv-preview box
```

**Structure Decision**: keep the two-folder layout. Settings SQL stays in `internal/settings`;
`kegs` stays unaware of settings (the menu always carries tap `srm` and `description`). The TV
drawing becomes one component shared by the TV page and the preview; the TV stylesheet stays the
one scoped sheet.

## Complexity Tracking

None.
