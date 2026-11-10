# Implementation Plan: Admin App Restyle

**Branch**: `compmodder26/feature_8` | **Date**: 2026-11-10 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/008-admin-app-restyle/spec.md`

## Summary

Web-only restyle of the admin app; no API, database or TV change. `AdminPage` becomes a shell
with a side panel (three groups, nine pages, icon + text, one group open at a time), a top bar
with a menu button below 768 px, and a main area that mounts only the current page (Taps after
sign-in). One new stylesheet, every rule scoped under `.admin`, gives light, dark and system
themes through CSS custom properties with a royal blue accent; the theme and the side panel
width are saved in `localStorage`. Five forms move into native `<dialog>` modals (add keg, add
and edit beer, add tap, new API token). A new On Deck page lists waiting kegs, then
conditioning and fermenting beers without a keg, in the TV's order, built from `GET /api/kegs`
and `GET /api/beers`. A hand-written ARIA combobox makes the keg form's beer choice searchable,
newest first. One new runtime dependency: `lucide-react` 1.61.0 for icons. A headless-Firefox
screenshot gate (laptop and phone, both themes, real CSP, axe contrast) is the done-when.
Details in [research.md](./research.md).

## Technical Context

**Language/Version**: TypeScript 5.9 with React 18.3 on Node 26 (web build). Go 1.27 API
unchanged.

**Primary Dependencies**: existing React, Vite, Vitest + Testing Library, jsdom. New:
`lucide-react` 1.61.0 (exact pin, runtime; research R2). No CSS or component library (R1).

**Storage**: browser `localStorage` only, two keys (see [data-model.md](./data-model.md)). No
server storage change.

**Testing**: Vitest + Testing Library, querying by role and accessible name
([contracts/admin-ui.md](./contracts/admin-ui.md)). A setup file stubs `showModal`/`close`,
which jsdom 30.1.3 lacks (R3). New tests: `prefs`, `Modal`, `BeerPicker`, `SideNav`,
`OnDeckPanel`, `KegCard`, `ThemeControl`, icon has no `style`, and `AdminStyle` (no `style`
attribute in the admin DOM except `--nav-width` on the root; CSS scoped to `.admin`; accent never
a text color). Updated:
`AdminPage`, `TapsPanel`, `BeersPanel`, `TokensPanel`, `KegForm`, `BrewfatherPanel` tests for
the navigation and dialogs. `go test -race ./...` must still pass untouched. Headless-Firefox
screenshot gate (quickstart §4).

**Target Platform**: current Firefox, Chrome and Safari on laptops and phones; the TV
(`/display`, Fully Kiosk on Fire TV) must be unaffected.

**Project Type**: web app (static SPA served by nginx) in front of the existing API

**Performance Goals**: no added request on page switch beyond the page's own loads; icons
tree-shaken to the ~19 used.

**Constraints**: no `style` prop in JSX and no `style` attribute in markup (nginx CSP has
`default-src 'self'` and no `style-src`). The one allowed `style` attribute at runtime is
`--nav-width` on the `.admin` root, written through CSSOM `style.setProperty` (R8), which the
CSP permits. `DisplayPage.css` loads on the admin app too and every rule in it is
scoped `.display`; every new rule is scoped `.admin`, so neither leaks into the other.
`DisplayPage.tsx`, `DisplayPage.css`, `web/nginx/` and `api/` are not modified. WCAG 2.1 AA
contrast in both themes (R7); 44 px touch targets on phones.

**Scale/Scope**: 9 pages, 5 dialogs; new files: 1 stylesheet, `prefs.ts`, `Modal`, `SideNav`,
`BeerPicker`, `OnDeckPanel`, `KegCard`, test setup; ~8 components touched.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | No capability added outside the API; every page uses existing routes. No endpoint change, so `api/openapi.yaml` is untouched | Pass |
| II. Go + SQLite | No backend change | Pass |
| III. React Clients | Same React + TS + Vite build behind nginx. The TV is untouched (no file of it edited, styles scoped, screenshot compared with `main`) | Pass |
| IV. Tests Required | Component tests for every new component with logic and every converted panel; existing tests updated, not deleted; API suite still runs. jsdom limits (layout, real dialog focus) covered by the browser gate | Pass |
| V. LAN Security | No new route, credential or stored secret. CSP unchanged and honored (no inline styles, no injected `<style>`). The token secret stays shown once (dialog). `localStorage` holds only theme and width, nothing sensitive | Pass |
| VI. Documented | Doc comments on every new component (`SideNav`, `Modal`, `BeerPicker`, `OnDeckPanel`, `KegCard`) and `prefs.ts`; `AdminPage` and `TapsPanel` docs rewritten for the new shape | Pass |
| VII. Simplicity | One dependency, justified in R2 (no transitive deps, ISC, 20 days old). No router, CSS framework, state library or dialog library. The cross-panel refresh counters are removed, since only one page mounts at a time | Pass |
| Deployment constraints | Same two containers; no env var, migration or nginx change | Pass |

Post-design re-check (after Phase 1): still passes. No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/008-admin-app-restyle/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/admin-ui.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
web/
├── package.json, package-lock.json     # + lucide-react 1.61.0 (exact)
├── vite.config.ts                      # test.setupFiles
└── src/
    ├── test/setup.ts                   # new: showModal/close stubs for jsdom (R3)
    ├── prefs.ts, prefs.test.ts         # new: theme and nav width in localStorage (R9)
    ├── pages/
    │   ├── AdminPage.tsx               # shell: page/group state, units + version load, theme class
    │   ├── AdminPage.css               # new: all admin styles under .admin (R1, R7, R8)
    │   └── AdminPage.test.tsx          # navigation, units, sign out
    └── components/
        ├── SideNav.tsx, .test.tsx      # new: tree, drawer, resize handle, theme, version, sign out
        ├── ThemeControl.tsx, .test.tsx # new: Light/Dark/System radio group (R7)
        ├── Modal.tsx, .test.tsx        # new: native <dialog> wrapper
        ├── BeerPicker.tsx, .test.tsx   # new: searchable combobox (R6)
        ├── OnDeckPanel.tsx, .test.tsx  # new: TV-order on-deck list + Add keg (R5)
        ├── KegCard.tsx, .test.tsx      # new: keg view, actions, inline edit form (from TapsPanel)
        ├── TapsPanel.tsx, .test.tsx    # on-deck list moved out; Add keg / Add tap dialogs
        ├── KegForm.tsx, .test.tsx      # BeerPicker instead of <select>
        ├── BeersPanel.tsx, .test.tsx   # Add/Edit beer dialog; refresh prop removed
        ├── TokensPanel.tsx, .test.tsx  # New token dialog
        ├── BrewfatherPanel.tsx         # two cards; onBeersChanged no longer needed by the shell
        └── *Panel.tsx, *Form.tsx       # class names for cards, fields, buttons; no logic change
```

**Structure Decision**: keep the existing `web/src/components` + `pages` layout. The shell owns
navigation and preferences; panels stay self-contained and keep their own API calls, so the
change per panel is markup classes plus the dialog wrapper where FR-019 asks.

## Complexity Tracking

None.
