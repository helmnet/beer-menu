---

description: "Task list for feature 8: Admin app restyle"
---

# Tasks: Admin App Restyle

**Input**: Design documents from `specs/008-admin-app-restyle/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/admin-ui.md](./contracts/admin-ui.md),
[quickstart.md](./quickstart.md); approved mockup
https://claude.ai/artifact/4QkqjYn1Vh6vfRsvCb44LF

**Tests**: Required. Constitution IV: React components with logic get Vitest + Testing Library
tests. Tests query by role and accessible name from contracts/admin-ui.md, never by class or
DOM shape. Write each test task before its implementation task and see it fail first. Existing
tests are updated to the new navigation and dialogs, never deleted to make them pass. Layout,
real dialog focus and contrast are checked by the browser gate (T050), since jsdom cannot see
them.

**Organization**: grouped by user story (spec.md US1 to US6). Shared pieces (dependency,
test setup, preferences, base stylesheet, `KegCard`) are foundational.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US6)

## Path Conventions

- Web only: `web/` (Vite + React 18 + TypeScript 5.9). Components in `web/src/components/`,
  pages in `web/src/pages/`, helpers in `web/src/`.
- Test helpers: `web/src/testApi.ts` (`stubApi(routes)` returns received calls; fixtures
  `beer()`, `tap()`, `keg()`, `menu()`, `settings()`, `kegTypes`, `brewfatherStatus()`,
  `batch()`, `importResult()`, `testPassword()`).
- Nothing under `api/`, `web/nginx/`, `web/src/pages/DisplayPage.tsx` or
  `web/src/pages/DisplayPage.css` changes.

## Conventions for every task

- Every new React component and `prefs.ts` gets a doc comment saying what it does and any
  contract it holds; inline comments explain why only (constitution VI). Rewrite the existing
  doc comments of `AdminPage`, `TapsPanel`, `BeersPanel`, `TokensPanel`, `KegForm` and
  `BrewfatherPanel` to match their new shape.
- **No `style` prop in JSX and no `style` attribute in markup** (nginx CSP: `default-src
  'self'`, no `style-src`). The one allowed `style` attribute at runtime is `--nav-width` on the
  `.admin` root, written with `element.style.setProperty("--nav-width", "<n>px")` from a ref
  (research R8).
- **All admin CSS lives in `web/src/pages/AdminPage.css`; every selector starts with `.admin`**,
  except `body:has(> #root > .admin)` for page margin and background (research R1).
- **Royal blue `#4169e1` is an accent, never a text color** (owner decision): primary button
  fills (white text), current nav item bar and tint, selected theme radio, checked boxes, focus
  ring, keg level bar, resize handle while dragged. All text uses the neutral text and muted
  tokens (research R7).
- Tokens verbatim from research R7: light background `#f4f6fb`, cards `#ffffff`, text
  `#1a1d24`, muted `#555b6a`; dark background `#0b0c0f`, cards `#17191f`, text `#e8eaf0`, muted
  `#a3a8b5`. Borders and tints as in the mockup: light border `#dfe3ec`, dark border `#2a2e37`,
  current-nav tint `rgba(65,105,225,0.12)` light / `rgba(65,105,225,0.22)` dark, dark side
  panel `#111317`. Font: system UI stack (no web font; CSP blocks Google Fonts; research R7).
- Breakpoint 768 px (phone below). Touch targets at least 44×44 px under the phone media query.
- Keep every action listed in research R10 ("Actions present today") working (FR-018).
- Run `cd web && npm test && npm run build` after each phase; both must pass.

## Phase 1: Setup

- [X] T001 Add `lucide-react` pinned exactly `1.61.0` to `dependencies` in `web/package.json` and update `web/package-lock.json` (`cd web && npm install --save-exact lucide-react@1.61.0`); confirm the lock entry has no dependencies of its own (research R2)
- [X] T002 Create `web/src/test/setup.ts` that, when `HTMLDialogElement.prototype.showModal` is missing, adds `showModal()` (sets `open`) and `close()` (removes `open`, dispatches `close`) stubs, and register it as `test.setupFiles: ["./src/test/setup.ts"]` in `web/vite.config.ts` (research R3)

## Phase 2: Foundational (blocks all stories)

- [X] T003 [P] Write `web/src/prefs.test.ts`: theme defaults to `system`; reads back `light`/`dark`/`system`; an invalid stored value reads `system`; nav width defaults to 256; clamps to "200–400 px and to at most 50% of the window width"; a non-integer stored width reads 256; `localStorage` whose `getItem`/`setItem` throw falls back to the defaults and does not throw
- [X] T004 [P] Implement `web/src/prefs.ts`: `loadTheme()`/`saveTheme(theme)` on key `beer-menu.admin.theme` (values `light` | `dark` | `system`, default `system`), `loadNavWidth()`/`saveNavWidth(px)` on key `beer-menu.admin.nav-width` (integer CSS px, default 256), `clampNavWidth(px, windowWidth)` to 200–400 and ≤ 50% of the window; every storage access in `try`/`catch` (data-model.md)
- [X] T005 [P] Write `web/src/components/Icon.test.tsx`: render one `lucide-react` icon (`Beer`) and assert the `<svg>` has no `style` attribute and has `aria-hidden="true"` (research R2 check)
- [X] T006 [P] Create `web/src/pages/AdminPage.css` base: `.admin` root with light tokens as custom properties (`--bg`, `--surface`, `--side`, `--text`, `--muted`, `--border`, `--accent: #4169e1`, `--accent-tint`, `--track`, `--scrim`, `--nav-width: 256px`), system font stack, `body:has(> #root > .admin) { margin: 0; background: var(--bg) }`; components: `.admin .card` (surface, 1px border, 12 px radius, 16 px padding), `.admin .btn` (neutral outline, 36 px tall), `.admin .btn-primary` (accent fill, white text), `.admin .icon-btn` (32 px square, muted icon), form fields (`input`, `select`, `textarea` under `.admin`, 40 px tall, border token), `.admin .field-error`, `.admin .table-wrap { overflow-x: auto }`, visible focus ring `outline: 2px solid var(--accent)`; nothing outside `.admin` except the `body:has` rule
- [X] T007 Write `web/src/components/KegCard.test.tsx`: buttons in order Pour, Set level, Edit keg, Take off, Kick; Take off only when `state === "on_tap"`; Pour opens the inline form prefilled 16 (US) / 500 (metric) and sends litres; Set level resent only when changed; a units change restarts an open form in the new units; Edit keg shows the edit `KegForm` inside this card and Cancel closes it; Kick asks for confirmation
- [X] T008 Extract the keg view and its actions from `TapsPanel` into `web/src/components/KegCard.tsx` (props: `keg`, keg types, `units`, `onChange(method, path, body)` or equivalent callbacks, `onSignedOut`): artwork (`BeerArt`), beer name, keg type, level as an accent `.level-bar` (`<progress>` or `<meter>` element with value = remaining/size, styled via CSS, no style attribute) plus "x of y" text, and the approved button row in order **Pour** (`btn-primary`), Set level, Edit keg, Take off (only when `state === "on_tap"`), Kick; it owns the inline pour/set-level form keyed by units, and **the inline edit `KegForm` renders inside this card** (Edit keg stays inline, spec Assumptions), so Taps and On Deck both get it; `TapsPanel` uses `KegCard` with no other behavior change and `web/src/components/TapsPanel.test.tsx` still passes apart from the edit form's location

**Checkpoint**: preferences, test setup, base styles and `KegCard` ready.

## Phase 3: User Story 1 - Find any admin task from a side panel (Priority: P1) MVP

**Goal**: side panel with three groups and nine pages (icon + text), one group open, only the
current page's cards shown, Taps after sign-in, resizable width saved, version and Sign out at
the bottom; Taps styled as in the approved mockup.

**Independent test**: at laptop width sign in, open every group and page, see only that
page's cards, one group open at a time, width survives a reload, version and Sign out shown.

### Tests for User Story 1

- [X] T009 [P] [US1] Write `web/src/components/SideNav.test.tsx` (contract: Shell table): navigation named "Admin"; only Menu, Administration, Integrations group buttons plus Menu's pages show at first; group buttons have `aria-expanded`; opening Administration closes Menu; clicking a page calls `onSelect(key)` and the current page has `aria-current="page"`; separator named "Resize side panel" with `aria-valuenow/min/max`, ArrowLeft/ArrowRight change width by 16 and call `saveNavWidth`; render `SideNav` inside a `<div className="admin">`; width is written as `--nav-width` on that `.admin` ancestor via `style.setProperty` (assert `getPropertyValue`); no `style` prop in JSX, and no other element gets a `style` attribute; "Version 1.2.3" from props and "Version unknown" when missing; "Sign out" calls `onSignOut`
- [X] T010 [US1] Update `web/src/pages/AdminPage.test.tsx` for navigation: after sign-in the Taps page shows (heading "Taps") and Keg History, Beer Library, Brewfather etc. are not mounted; selecting each of the nine pages shows its heading and unmounts the previous page; `GET /api/health` stubbed with `{"version":"1.2.3"}` shows "Version 1.2.3" and a 500 shows "Version unknown"; units load from `GET /api/settings` after sign-in (metric shows litres on Taps) and a 401 from it returns to sign-in; Sign out still calls `DELETE /api/session`; drop the tests for cross-panel refresh counters ("reloads the keg history after a kick", "reloads the beer library after a keg goes on a tap", "reloads the beer library after a Brewfather import", "shows display settings between Brewfather and keg history") and replace them with "a page shown again fetches fresh data"; keep setup, sign-in, 401 and unreachable-API cases

### Implementation for User Story 1

- [X] T011 [P] [US1] Implement `web/src/components/SideNav.tsx` per contracts/admin-ui.md Shell table and data-model.md page tree (groups Menu `Beer`, Administration `Shield`, Integrations `Plug`; pages Taps `GlassWater`, On Deck `Hourglass`, Keg History `History`, Beer Library `Library`, Display Settings `Tv`, Admin Users `Users`, API Tokens `KeyRound`, Change Password `Lock`, Brewfather `FlaskConical`; icons `aria-hidden`): props `page`, `openGroup`, `onGroup`, `onSelect`, `version`, `onSignOut`, and a slot for the theme control (US3); bottom area with version text and Sign out (`LogOut` icon); resize separator using pointer capture and arrow keys, clamped with `clampNavWidth`, re-clamped on window `resize`, saved with `saveNavWidth`; `SideNav` owns the width and writes it as `--nav-width` on its nearest `.admin` ancestor (`ref.current.closest(".admin")`) via `style.setProperty`, on mount and on every change
- [X] T012 [US1] Rewrite `web/src/pages/AdminPage.tsx` as the shell (data-model.md "In-memory UI state"): root `<div className="admin theme-…">` with `SideNav` and `<main>`; state `page` (default `taps`, reset on every sign-in), `openGroup`; render only the current page's component; load `GET /api/health` version once (failure or empty → "unknown") and `GET /api/settings` units once after sign-in (401 → sign-in, other failure keeps `us`), still updated by `SettingsPanel`'s `onChange`; remove the `kegChanges`/`beerChanges` counters and `BeersPanel`'s `refresh` prop and `onKegsChanged`/`onBeersChanged` wiring (research R4); setup, sign-in, loading and error views render inside the `.admin` root without the side panel; import `./AdminPage.css`
- [X] T013 [US1] Remove the `refresh` prop from `web/src/components/BeersPanel.tsx` and its test cases in `web/src/components/BeersPanel.test.tsx`; drop `onKegsChanged` from `TapsPanel` and `onBeersChanged` from `BrewfatherPanel` if nothing else uses them, updating their tests to match
- [X] T014 [US1] Restyle Taps per the approved mockup in `web/src/components/TapsPanel.tsx`: page header with `h2` "Taps", a muted summary line, and "Add tap" (`btn-primary`, `Plus` icon) on the right; taps in a two-column card grid on laptop; each tap an `article` card named by its label with a header of `h3` "<position>. <label>" (plus "(disabled)") and icon buttons with `aria-label` "Rename" (`Pencil`, reveals the inline label form), "Move up" (`ArrowUp`), "Move down" (`ArrowDown`), "Disable"/"Enable" (`Power`), "Remove" (`Trash2`); body is `KegCard` or, for an empty tap, a dashed empty glass, "Empty tap", "Add keg" (`btn-primary`) and, while kegs wait, the "Waiting keg" select with "Put on tap"; remove the On deck section (moves to US5); update `web/src/components/TapsPanel.test.tsx` for the new names (rename now opens via the "Rename" button)
- [X] T015 [US1] Apply the shared classes to the other pages without changing behavior: `card` sections with `h2` page headings and `btn`/`btn-primary` buttons in `web/src/components/KegHistory.tsx`, `BeersPanel.tsx`, `SettingsPanel.tsx`, `AdminsPanel.tsx`, `TokensPanel.tsx`, `PasswordForm.tsx`; field, error and button classes in `KegForm.tsx` and `BeerForm.tsx`; `LoginForm.tsx` and `SetupForm.tsx` as a centered card with the "Beer Menu" title and the beer icon, using the same theme (FR-013); wide tables or lists in `.table-wrap`; headings match page labels ("Keg History", "Beer Library", "Display Settings", "Admin Users", "API Tokens", "Change Password"); update affected heading queries in their tests
- [X] T016 [P] [US1] Split `web/src/components/BrewfatherPanel.tsx` into two `card` sections in one component: "Connection" (connect form or connected state, Disconnect, Sync interval, Sync now, last sync) and "Batches" (Refresh, batch list in `.table-wrap`, Import selected, import results); update `web/src/components/BrewfatherPanel.test.tsx` heading queries
- [X] T017 [US1] Add side panel, main area, page header, card grid, tap card, `KegCard` and resize handle styles to `web/src/pages/AdminPage.css` for laptop widths (≥ 768 px): fixed-width side panel `width: var(--nav-width)`, current page item with a 3 px accent left bar and `--accent-tint` background and neutral text, accent-colored icon on the current item only via `stroke: var(--accent)` on its `svg` (never a `color:` declaration, so T041 holds), level bar fill `var(--accent)` on `var(--track)`

**Checkpoint**: navigation works at laptop width with the approved Taps look.

## Phase 4: User Story 2 - Use the admin app on a phone (Priority: P1)

**Goal**: below 768 px the side panel is a drawer behind a menu button in a top bar; nothing
scrolls sideways; 44 px targets.

**Independent test**: at 390 px open the menu, visit every page, Escape and backdrop close it,
no sideways scroll, version and Sign out reachable.

- [X] T018 [US2] Extend `web/src/components/SideNav.test.tsx`: a "Open menu" button with `aria-expanded="false"` and `aria-controls` the navigation; clicking it sets `aria-expanded="true"`, the label becomes "Close menu" and focus moves to the current page item, or to the first group button when the current page's group is collapsed; selecting a page, clicking the backdrop or pressing Escape closes it and returns focus to the menu button; Escape does nothing when closed
- [X] T019 [US2] Implement the drawer in `web/src/components/SideNav.tsx` and `web/src/pages/AdminPage.tsx`: top bar (shown only under the phone media query) with the menu button (`Menu`/`X` icons), the current page title and a slot for the page's primary action (Taps: "Add tap"); `drawerOpen` state per data-model.md; a decorative backdrop `div` (`aria-hidden="true"`) closes the drawer on click (keyboard users close it with Escape); Escape listener on the drawer only while open (no global keydown handler when closed)
- [X] T020 [US2] Add phone styles to `web/src/pages/AdminPage.css` (`@media (max-width: 767.98px)`): top bar 60 px; side panel off-canvas (300 px, hidden unless `.nav-open`), `--scrim` backdrop; resize handle hidden; single-column card grid; keg action buttons in a 3-column grid; every `button`, nav item, `input`, `select` at least 44 px tall and icon buttons 44×44; `html, body` no horizontal overflow; `.table-wrap` scrolls inside its card; the page header's primary action (Taps: "Add tap") is hidden under this query because the top bar shows it, so it appears once

**Checkpoint**: phone layout matches the approved phone mockups.

## Phase 5: User Story 3 - Pick light, dark or system theme (Priority: P2)

**Goal**: Light / Dark / System radio group in the side panel, saved per browser, System
follows the device live, TV unaffected.

**Independent test**: switch themes, reload, the choice stays; System follows the device; the
TV theme setting is unchanged.

- [X] T021 [P] [US3] Write `web/src/components/ThemeControl.test.tsx`: `radiogroup` named "Theme" with radios "Light", "Dark", "System"; the checked one matches the prop; choosing one calls `onChange` with `light` | `dark` | `system`
- [X] T022 [US3] Extend `web/src/pages/AdminPage.test.tsx`: the root has class `theme-system` with no saved theme; choosing Dark sets `theme-dark` and saves `beer-menu.admin.theme=dark`; a saved `light` applies on load and on the sign-in screen; no request to `/api/settings` is made by a theme change
- [X] T023 [P] [US3] Implement `web/src/components/ThemeControl.tsx`: a `fieldset` with a visually hidden `legend` "Theme" and three radio inputs with `Sun`, `Moon`, `Monitor` icons as a segmented control; the selected option uses the accent fill with white text (accent, not text color)
- [X] T024 [US3] Wire the theme in `web/src/pages/AdminPage.tsx`: state from `loadTheme()`, root class `theme-<choice>`, `saveTheme` on change, `ThemeControl` in the `SideNav` bottom slot; the sign-in and setup views use the same root class (FR-013)
- [X] T025 [US3] Add dark tokens to `web/src/pages/AdminPage.css`: `.admin.theme-dark` and `@media (prefers-color-scheme: dark) { .admin.theme-system }` set background `#0b0c0f`, side `#111317`, cards `#17191f`, text `#e8eaf0`, muted `#a3a8b5`, border `#2a2e37`, tint `rgba(65,105,225,0.22)`; `color-scheme: light` / `dark` to match; accent stays `#4169e1`

**Checkpoint**: both themes render; System follows the device without a reload.

## Phase 6: User Story 4 - Add and edit in modal dialogs (Priority: P2)

**Goal**: add keg, add beer, edit beer, add tap and new API token open in native `<dialog>`
modals that trap focus, close on Escape/Cancel, keep errors open, refresh on success.

**Independent test**: open each of the five dialogs; cancel one, submit one with a 400, submit
one successfully; focus returns to the opener.

### Tests for User Story 4

- [X] T026 [P] [US4] Write `web/src/components/Modal.test.tsx`: renders a `dialog` named by its title; `showModal` was called on mount; firing `cancel` calls `onClose` and prevents the default close; unmounting calls `close` and returns focus to the element focused before it opened
- [X] T027 [P] [US4] Update `web/src/components/TapsPanel.test.tsx`: "Add tap" opens dialog "Add tap" with an optional "Label" field; empty label sends `POST /api/taps` with body `{}`, a label sends `{"label": "..."}`; a 400 keeps the dialog open with the field error; success closes it and reloads taps; "Add keg" on an empty tap opens dialog "Add keg" and success closes it and reloads; Cancel closes without a request; a 401 on saving from the "Add keg" dialog calls `onSignedOut` (spec edge case: session expires while a dialog is open)
- [X] T028 [P] [US4] Update `web/src/components/BeersPanel.test.tsx`: "Add beer" opens dialog "Add beer"; "Edit" opens dialog "Edit <name>" filled in; a 400 keeps it open with field errors; success closes and reloads the list; Cancel sends nothing
- [X] T029 [P] [US4] Update `web/src/components/TokensPanel.test.tsx`: "New token" opens dialog "New API token" with Name and Scope; submit shows the secret once with "Copy" and "Close" inside the dialog; after Close the secret text is not in the document and the list shows the token; a 400 keeps the form open with the error

### Implementation for User Story 4

- [X] T030 [US4] Implement `web/src/components/Modal.tsx` per research R3 and contracts/admin-ui.md Dialogs: native `<dialog>` with `aria-labelledby` its `h2` title and a "Close" icon button (`X`); `showModal()` on mount, `close()` on unmount; `cancel` → `preventDefault()` + `onClose`; remembers `document.activeElement` on open and refocuses it after close; styles in `web/src/pages/AdminPage.css` (`.admin dialog`, `::backdrop` scrim, 480 px max width, full width minus 32 px on phones)
- [X] T031 [US4] Use `Modal` in `web/src/components/TapsPanel.tsx` for "Add keg" (wrapping `KegForm` with `tapId`) and a new "Add tap" form (optional "Label" input, 1–50 characters per `TapCreate.label`; empty sends `{}`), closing on save and reloading
- [X] T032 [US4] Use `Modal` in `web/src/components/BeersPanel.tsx` for Add beer and Edit beer around `BeerForm` (title "Add beer" / "Edit <name>")
- [X] T033 [US4] Use `Modal` in `web/src/components/TokensPanel.tsx`: a "New token" button opens "New API token" with the create form; on success the dialog shows the secret once with Copy and Close; closing clears the secret from state

**Checkpoint**: all five dialogs work; the inline forms they replaced are gone.

## Phase 7: User Story 5 - See On Deck as the TV shows it (Priority: P2)

**Goal**: On Deck lists waiting kegs, then conditioning beers, then fermenting beers without a
keg, in the TV's order, with keg actions and Add keg.

**Independent test**: with a waiting keg, a conditioning beer and a fermenting beer without a
keg, the admin On Deck order equals the TV's coming-soon order.

- [X] T034 [P] [US5] Write `web/src/components/OnDeckPanel.test.tsx` (stub `GET /api/kegs` and `GET /api/beers`): waiting kegs (current kegs with `tap_id === null`) first in API order; then `conditioning`, then `fermenting` beers with no current keg, "each by name ignoring case, then id"; a fermenting beer with a current keg is listed once, as its keg; beers with other statuses are absent; empty state text "Nothing on deck"; waiting kegs show `KegCard` actions Edit keg and Kick only (owner decision 2026-11-10); "Add keg" opens dialog "Add keg" and success reloads; a 401 calls `onSignedOut`
- [X] T035 [US5] Implement `web/src/components/OnDeckPanel.tsx` per research R5 and data-model.md "On Deck item": loads `GET /api/kegs` and `GET /api/beers` (plus `GET /api/keg-types` for labels), builds the list with `localeCompare(…, { sensitivity: "base" })` then id; renders waiting kegs with `KegCard` (Edit keg, with its inline form inside the card, and Kick only; owner decision 2026-11-10), beers as cards with name, style, ABV, status ("Conditioning"/"Fermenting") and `BeerArt`; "Add keg" (`btn-primary`) opens `KegForm` in `Modal` with no tap
- [X] T036 [US5] Route the `on-deck` page to `OnDeckPanel` in `web/src/pages/AdminPage.tsx`, passing `units` and `onSignedOut`; add an AdminPage test case that On Deck shows its heading

**Checkpoint**: On Deck matches the TV.

## Phase 8: User Story 6 - Find a beer by typing (Priority: P3)

**Goal**: the keg form's beer choice is a searchable combobox, newest first, filter only.

**Independent test**: open Add keg, see newest-first order, type part of a name, list narrows,
keyboard pick works, no create option.

- [X] T037 [P] [US6] Write `web/src/components/BeerPicker.test.tsx` (contract: Beer picker table): `combobox` named "Beer" with `listbox` "Beers"; options sorted by `created_at` newest first, ties by id descending; typing "ipa" shows only names containing it ignoring case; "No beer matches" when none and no option; ArrowDown/ArrowUp move `aria-activedescendant`, Enter picks and calls `onChange(id)`, Escape closes the list; the current value stays selected when filtered out
- [X] T038 [US6] Implement `web/src/components/BeerPicker.tsx` per research R6 (WAI-ARIA 1.2 combobox; filter "the typed text, trimmed, as a case-insensitive substring of the name (`toLocaleLowerCase`)"; sort `Date.parse(created_at)` desc then id desc); Escape while the list is open stops propagation so it does not close the dialog; styles in `web/src/pages/AdminPage.css` (search icon in the field, active option with `--accent-tint` background and neutral text)
- [X] T039 [US6] Replace the beer `<select>` in `web/src/components/KegForm.tsx` with `BeerPicker` (same `beer_id` value and field error); update `web/src/components/KegForm.test.tsx` to pick beers through the combobox

**Checkpoint**: all six stories done.

## Phase 9: Polish & Cross-Cutting

- [X] T040 [P] Add `web/src/pages/AdminStyle.test.tsx`: for each of the nine pages and each open dialog, render the signed-in `AdminPage` with stubs and assert `container.querySelectorAll("[style]")` is empty, except the `.admin` root, whose `style` may hold only `--nav-width`
- [X] T041 [P] Add a check in `web/src/pages/AdminStyle.test.tsx` that reads `web/src/pages/AdminPage.css` and fails if any selector does not start with `.admin` (other than `body:has(> #root > .admin)`), or if any `color:` declaration (not `stroke`, `fill`, `background`, `border` or `outline`) uses `var(--accent)` or `#4169e1` (accent is never a text color)
- [X] T042 [P] Add a test in `web/src/pages/DisplayPage.test.tsx` or `web/src/App.test.tsx` that `/display` renders without the `.admin` class or side panel (TV untouched)
- [X] T043 Confirm with `git diff --stat origin/main -- api web/nginx web/src/pages/DisplayPage.tsx web/src/pages/DisplayPage.css` that nothing changed there
- [X] T044 Run `cd web && npm ci && npm test && npm run lint && npm run build` and `cd api && go test -race ./...`; all pass
- [X] T045 Run `cd web && npm audit --omit=dev` and record the result; `gitleaks detect --no-git --source web/src --redact` and `--source specs --redact`
- [X] T046 Build the local stack (project notes recipe: worktree compose with `PUID=$(id -u) PGID=$(id -g)`, override with env admin credentials and `BEER_MENU_COOKIE_SECURE: "false"`, non-8080 `BEER_MENU_WEB_PORT`, scratch `BEER_MENU_DATA`) and seed the data in quickstart.md §1 with a script in the session scratchpad
- [X] T047 Write the screenshot script in the session scratchpad (not the project): `puppeteer-core@24.10.0` + `axe-core` with `/usr/bin/firefox` headless (`{browser: "firefox", executablePath: "/usr/bin/firefox"}`); sign in; for each of the nine pages and the five dialogs at 1920×1080, 1366×768 and 390×844 in light and dark (set `beer-menu.admin.theme` in `localStorage` before load) save a PNG; plus the phone drawer open and the sign-in screen; with the theme set to `system`, emulate `prefers-color-scheme` light then dark (`page.emulateMediaFeatures`) on one page and confirm the background token changes without a reload (FR-011); if Firefox under puppeteer cannot emulate it, record that and check it by hand in T051
- [X] T048 In the same script, fail on: any console error or `securitypolicyviolation` event; any axe-core `color-contrast` violation; `document.documentElement.scrollWidth > innerWidth` at 390 px; in a dialog, Escape not closing it, Tab leaving it, or focus not returning to the opener; at 390 px any visible `button`, `input`, `select`, `textarea`, nav item or `[role=option]` smaller than 44×44 CSS px (`getBoundingClientRect`, FR-028); after opening the drawer at 390 px and resizing to 1366 px, the side panel not shown as a column or the page content missing (spec edge case)
- [X] T049 Screenshot `/display` at 1920×1080 on this branch and on `origin/main` (built per the upgrade recipe, same data) and compare (pixel diff or side by side); any difference fails (SC-003)
- [X] T050 Run the gate (T047–T049) until it passes, then show the owner the screenshots (laptop and phone, both themes) against the mockup; record results in the PR
- [ ] T051 Run quickstart.md §2, §3 and §5 by hand in the browser, including every action in research R10's FR-018 list once, and time adding a keg on a phone-width window by typing part of a beer name (SC-005: under 30 seconds); record the time
- [X] T052 Stop the stack (`docker compose -p <project> down`), delete scratch data and local `ghcr.io/helmnet/beer-menu-*:latest` images

## Dependencies & Execution Order

- **Setup (T001–T002)** → **Foundational (T003–T008)** → stories.
- **US1 (T009–T017)** first: it creates the shell every other story plugs into.
- **US2** depends on US1 (`SideNav`, `AdminPage`).
- **US3** depends on US1 (theme slot in `SideNav`, root class in `AdminPage`); independent of
  US2.
- **US4** depends on Foundational and US1 (T014 restyled `TapsPanel`); `Modal` (T030) before
  T031–T033.
- **US5** depends on US4's `Modal` (Add keg dialog) and `KegCard` (T008).
- **US6** depends on Foundational and on T015 (both edit `KegForm.tsx`); T039 runs after T015.
  `KegForm` is also wrapped by US4 dialogs, but from other files.
- **T015** edits `BeersPanel.tsx` (also T013, T032), `TokensPanel.tsx` (also T033) and
  `KegForm.tsx` (also T039): run it after T013 and before T032, T033 and T039, never in
  parallel with them.
- **Shared files**: `web/src/pages/AdminPage.css` (T006, T017, T020, T025, T030, T038) and
  `web/src/pages/AdminPage.tsx` (T012, T019, T024, T036) are edited one task at a time, in task
  order, whatever story the task belongs to. Only a story's other files may run in parallel.
- **Polish** after all stories; T050 is the done-when gate.

## Parallel Opportunities

- T003, T005, T006 together; T004 with T005/T006.
- US1: T009 with T011 (test vs. implementation in different files, write test first); T016
  alongside T014 (different files). T015 is not parallel (shared files, see Dependencies).
- US3: T021 and T023 alongside each other; US3 can run in parallel with US2 once US1 is done
  (but both edit `AdminPage.tsx` and `AdminPage.css`: sequence those edits).
- US4: T026–T029 in parallel.
- US6 (T037–T038) can run in parallel with US2–US5; T039 waits for T015.
- Polish: T040–T042 in parallel.

## Implementation Strategy

1. **MVP**: Setup + Foundational + US1 + US2 (P1): the app is navigable and usable on a phone,
   light theme only.
2. Add US3 (themes), then US4 (dialogs), then US5 (On Deck), then US6 (picker), running tests
   and build after each.
3. Finish with Polish; the screenshot gate (T050) and the owner's look are the done-when. Then
   PR and propose release `v0.7.0`.
