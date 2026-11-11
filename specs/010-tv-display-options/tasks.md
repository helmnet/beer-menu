---
description: "Task list for feature 10: TV display options"
---

# Tasks: TV Display Options

**Input**: Design documents from `specs/010-tv-display-options/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md),
[contracts/settings.openapi.yaml](./contracts/settings.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks of the database; there is no migration, so the upgrade
case is a test on a version-5 database with no new rows; React components with logic get Vitest
tests. Run Vitest with `--maxWorkers=2`. Write each test task before its implementation task and
see it fail first.

**Organization**: the settings API for all 11 fields, the `TvView` split and the size unit
variables are foundational: every story reads the settings and draws through `TvView`. Stories
run in priority order: US1 (P1), US2 (P2), US4 (P2), US3 (P3), US5 (P3). US4 comes before US3
and US5 so fonts and colors are built with the preview in place.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US5)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`). Settings in
  `api/internal/settings/display.go`; PATCH body in `api/internal/httpapi/settings.go`; menu in
  `api/internal/kegs/menu.go`.
- Web: `web/` (Vite + React + TypeScript). New `web/src/tv.ts`, `web/src/fonts.ts`,
  `web/src/components/TvView.tsx`.
- Test helpers: `api/internal/httpapi/helpers_test.go` (`newTestDeps`, `testClock`, `do`,
  `insertToken`, `wantError`, `sessionCookie`); `api/internal/httpapi/routes_test.go` (contract
  `x-access` check); `web/src/testApi.ts` (`stubApi`, `beer()`, `tap()`, `keg()`, `menu()`).

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go dependencies. The only new web dependencies are the 7 `@fontsource` packages in
  T001. No migration; the database stays at version 5.
- No `style` attribute in markup and no `<style>` element; runtime values reach CSS only as
  custom properties through React's `style` prop (CSSOM), research R4.
- `grid` with default settings must stay pixel-identical to today (SC-002): do not change the
  grid rules' computed values.
- Setting fields, verbatim from data-model.md (JSON field / key / allowed / default):
  - `layout` / `display.layout`: "`grid`, `list`, `columns`", default `grid`.
  - `title_align` / `display.title_align`: "`left`, `center`, `right`", default `left`.
  - `title_font`, `text_font` / `display.title_font`, `display.text_font`: font key "`system`,
    `inter`, `lora`, `roboto-slab`, `oswald`, `nunito`, `bebas-neue`, `pacifico`", default
    `system`.
  - `show_style`, `show_abv`, `show_ibu` / `display.show_*`: boolean, default `true`.
  - `show_srm`, `show_description` / `display.show_*`: boolean, default `false`.
  - `title_color`, `text_color` / `display.*_color`: "`""` or `#rrggbb`", default `""`;
    stored lowercase.
  - Stored forms: "booleans as `"1"` / `"0"`; colors lowercase."
  - Messages: enums "`Must be one of: …`" (listing the allowed values); switches "`Must be true or
    false.`"; colors "`Must be a color like #1a2b3c, or empty for the theme color.`" Messages
    never echo the value.
  - Every new field is public in the menu's `display`; `units` stays out.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the font packages and the contract the rest builds on.

- [X] T001 Add `@fontsource/inter`, `@fontsource/lora`, `@fontsource/roboto-slab`, `@fontsource/oswald`, `@fontsource/nunito`, `@fontsource/bebas-neue`, `@fontsource/pacifico`, each pinned to exact version `5.3.0`, to `dependencies` in `web/package.json`; run `npm install` in `web/` to update `web/package-lock.json`
- [X] T002 [P] Update `api/openapi.yaml` per `specs/010-tv-display-options/contracts/settings.openapi.yaml`: add `Layout`, `TitleAlign`, `FontKey`, `TextColor`; extend `Settings`, `SettingsPatch`, `MenuDisplay` with the 11 fields; add optional `srm` and `description` to `MenuBeer`; bump `info.version` 0.8.0 to 0.9.0

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: all 11 settings stored, validated and served; the menu carries tap `srm` and
`description`; the TV drawing split into `TvView` with size unit variables.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

### Tests for the foundation (write first, see them fail)

- [X] T003 [P] Extend `api/internal/settings/display_test.go`: each new field reads its default with no row; a bad stored value (e.g. `display.layout`=`"tiles"`, `display.show_srm`=`"yes"`, `display.title_color`=`"red"`) reads as its default; PATCH accepts every allowed value; refuses each invalid value with the verbatim message and writes nothing (all-or-nothing with a valid field in the same patch); `#FFAA00` is stored and returned as `#ffaa00`; `""` resets a color; booleans store as `"1"`/`"0"`; `Public()` has every new field and no units
- [X] T004 [P] Extend `api/internal/httpapi/settings_test.go`: GET returns all 15 fields; PATCH with a non-boolean `show_ibu` (`"true"`, `1`), `null` for any new field, or a non-string `layout` gives 400 with that field in `fields`; an unknown field gives 400; a `read` token sending `{"show_ibu":false}` gets 403 and a request with no credential gets 401, with nothing stored; a v5 database with feature 7 rows and no new rows reads every new field at its default and keeps the feature 7 values
- [X] T005 [P] Extend `api/internal/kegs/menu_test.go`: a tap beer with SRM and description carries both; a tap beer without them omits both keys; on deck beers (waiting keg and fermenting/conditioning) never have either key, even when the beer has them
- [X] T006 [P] Extend `api/internal/httpapi/menu_test.go`: `display` has all 14 TV fields at their defaults on a fresh DB and reflects saved values; `units` is absent
- [X] T007 [P] Create `web/src/tv.test.ts`: `displayOf` returns every default (`grid`, `left`, `system`, `system`, style/ABV/IBU on, SRM/description off, colors `""`) for a missing `display`, and falls back per field for a malformed value (unknown layout, unknown font, `show_abv: "no"`, color `"red"`, color `"#abc"`), keeping the other fields
- [X] T008 [P] Create `web/src/components/TvView.test.tsx`: with `grid` and default settings it renders the same title, tap cards, on deck cards, "Nothing on tap", pager and offline notice as `DisplayPage` did (copy, do not move, the drawing assertions from `web/src/pages/DisplayPage.test.tsx` that are about one page's markup; the `DisplayPage` tests stay as they are)

### Implementation for the foundation

- [X] T009 Extend `api/internal/settings/display.go`: add the 11 fields to `Display`, `Public`, `DisplayPatch` (`*bool` for switches) and `Default`; read them in `GetDisplay` with per-field fallback; validate and write them in `PatchDisplay` with the verbatim messages; colors match `^#[0-9a-fA-F]{6}$` or `""` and are lowercased; booleans stored `"1"`/`"0"`. Keep the existing four fields unchanged
- [X] T010 Extend `updateSettings` in `api/internal/httpapi/settings.go`: 11 more `json.RawMessage` fields decoded with `rawField[string]` / `rawField[bool]` (message "`Must be true or false.`" for a switch that is not a JSON boolean); update the handler doc comments from "four" to all settings
- [X] T011 Extend `api/internal/kegs/menu.go`: `MenuBeer` gains `SRM *float64 \`json:"srm,omitempty"\`` and `Description *string \`json:"description,omitempty"\``; only the taps query selects `b.description` and sets both, so on deck beers always omit them and tap beers omit each when unset; update the `Menu`/`MenuBeer` doc comments
- [X] T012 Update `web/src/api.ts` and `web/src/testApi.ts`: `Layout`, `TitleAlign`, `FontKey` types; `Settings` and `MenuDisplay` gain the 11 fields; `MenuBeer` gains optional `srm?: number` and `description?: string`; fixtures default every new field
- [X] T013 Create `web/src/tv.ts`: move `displayOf` from `web/src/pages/DisplayPage.tsx` and extend it to return every TV setting with per-field fallback; move `chunks`, `CARDS_PER_PAGE`, the `Page` type and the page-building code into `pagesOf(menu, layout)` (only `grid` for now, today's exact behavior); move the page timer into `usePager(count, pageMs)` returning `[page, setPage]` with today's restart and wrap rules; doc comments on each export
- [X] T014 Create `web/src/components/TvView.tsx` from the render body of `web/src/pages/DisplayPage.tsx`: props `menu`, `display` (from `displayOf`), `pages`, `page`, `failures`, `offlineAfter`; no timers, no fetch; doc comment states it is the one drawing shared by the TV and the preview
- [X] T015 Rework `web/src/pages/DisplayPage.tsx` to keep only polling, the offline count and the daily reload, and render `TvView` with `pagesOf` and `usePager`; update its doc comment; all existing `web/src/pages/DisplayPage.test.tsx` tests must pass unchanged
- [X] T016 In `web/src/pages/DisplayPage.css` replace every `Nvh` with `calc(N * var(--vh))` and every `Nvw` with `calc(N * var(--vw))` (25 uses), set `--vh: 1vh; --vw: 1vw` on `.display`, and update the header comment (research R8); computed sizes on the TV stay identical

**Checkpoint**: settings API serves all 15 fields, the TV draws through `TvView`, and the TV
looks exactly as before. Run `go test -race ./...` in `api/` and `npx vitest run --maxWorkers=2`
in `web/`.

---

## Phase 3: User Story 1 - Choose which beer details a tap shows (Priority: P1) 🎯 MVP

**Goal**: the five detail switches control what each tap card shows.

**Independent Test**: on a fresh install tap cards show name, style, ABV and IBU; toggling each
switch in Display Settings adds or drops only that detail on the TV's next refresh.

### Tests for User Story 1

- [X] T017 [P] [US1] Extend `web/src/components/TvView.test.tsx`: each switch off hides its detail on tap cards; `show_srm` shows "SRM 6" for `srm: 5.6` (rounded) and nothing when absent; `show_description` shows the text in an element with class `description` and nothing when absent; all switches off leaves name and art; an empty tap shows "Empty" whatever the switches; on deck cards show name, style, ABV and status whatever the switches (FR-013); description markup text renders as text
- [X] T018 [P] [US1] Extend `web/src/components/SettingsPanel.test.tsx`: five checkboxes (Style, ABV, IBU, SRM, Description) show the loaded values; Save sends only the changed switches as JSON booleans

### Implementation for User Story 1

- [X] T019 [US1] In `web/src/components/TvView.tsx` make tap cards honor `show_style`, `show_abv`, `show_ibu`, `show_srm` (`SRM N`, `Math.round`) and `show_description`, each left out when null or absent; on deck cards keep today's details
- [X] T020 [US1] In `web/src/pages/DisplayPage.css` add `.display .description` clamped to 2 lines (`display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden`) inside the fixed text block, and an `.srm` style matching `.facts`; the card size must not change (research R10)
- [X] T021 [US1] In `web/src/components/SettingsPanel.tsx` add a "Tap details" fieldset with the five checkboxes; widen the `Values` model and the changed-only diff in `submit` to send booleans; update the doc comment

**Checkpoint**: US1 works on its own on the TV.

---

## Phase 4: User Story 2 - Choose the TV layout (Priority: P2)

**Goal**: grid, vertical list and side by side layouts, each paging at a fixed row count.

**Independent Test**: switch layouts with 9+ taps and 2+ on deck items; every item appears
within one rotation; grid matches today.

### Tests for User Story 2

- [X] T022 [P] [US2] Extend `web/src/tv.test.ts`: `pagesOf` for `grid` is unchanged (8 per page, taps then on deck); `list` gives tap pages of 6 then on deck pages of 6; `columns` gives `max(ceil(taps/6), ceil(onDeck/6))` pages each holding tap chunk i and on deck chunk i; empty lists give no pages except `columns` with taps and no on deck; with 24 taps and 12 on deck items every item appears on exactly one page in each layout (`grid` 3 + 2, `list` 4 + 2, `columns` 4 pages) (SC-003)
- [X] T023 [P] [US2] Extend `web/src/components/TvView.test.tsx`: `list` renders rows (art, label, name, details) in tap order and an "On deck" heading on on deck pages; `columns` renders a taps column and an on deck column on one page, "Nothing on deck" when on deck is empty, and blank space (no placeholder rows) after the shorter column's last item
- [X] T024 [P] [US2] Extend `web/src/components/SettingsPanel.test.tsx`: Layout select (Grid, Vertical list, Side by side) shows the loaded value and Save sends `layout`

### Implementation for User Story 2

- [X] T025 [US2] In `web/src/tv.ts` extend `pagesOf` with `list` and `columns` per research R7; `ROWS_PER_PAGE = 6` with a `shortcut:` comment (fixed for 16:9 at 3 m; tune from screenshots)
- [X] T026 [US2] In `web/src/components/TvView.tsx` render the `list` and `columns` pages: each row is the small art (same size on every row, so the gauge reads the same), then label, name and the US1 details; class on `.display` per layout (`layout-list`, `layout-columns`); `grid` markup unchanged
- [X] T027 [US2] In `web/src/pages/DisplayPage.css` add `.display.layout-list` and `.display.layout-columns` rules: 6 equal row tracks, fixed art cell, one- or two-line text, two equal columns with their headings for `columns`; all sizes via `var(--vh)`/`var(--vw)`
- [X] T028 [US2] In `web/src/components/SettingsPanel.tsx` add the Layout select

**Checkpoint**: US1 and US2 work on the TV.

---

## Phase 5: User Story 4 - Preview changes before saving (Priority: P2)

**Goal**: a 16:9 preview in Display Settings that follows the unsaved form.

**Independent Test**: change settings without saving; preview updates at once, the TV does not;
after Save the TV matches the preview.

### Tests for User Story 4

- [X] T029 [P] [US4] Extend `web/src/components/SettingsPanel.test.tsx`: the preview (region labelled "TV preview") renders the current settings and the menu from `GET /api/menu`; changing a field without saving changes the preview at once and sends no PATCH; an invalid draft `page_seconds` keeps the last valid interval; a failed menu read shows the sample menu; Previous and Next step pages and wrap; the preview turns pages by itself at the draft interval (fake timers); on the last `grid` page, switching the draft layout to one with fewer pages goes back to page 1 and still shows every item within one rotation
- [X] T030 [P] [US4] Extend `web/src/tv.test.ts`: `SAMPLE_MENU` has at least 3 taps (one with SRM and a long description, one empty) and 2 on deck items, and passes the menu shape check

### Implementation for User Story 4

- [X] T031 [US4] In `web/src/tv.ts` add `SAMPLE_MENU` and export the menu shape check (`isMenu`) moved from `web/src/pages/DisplayPage.tsx`
- [X] T032 [US4] In `web/src/components/SettingsPanel.tsx` add the preview: one `GET /api/menu` on mount (sample on failure or a body `isMenu` rejects), `TvView` drawn from the draft values merged over the last valid ones, `usePager` at the draft interval, Previous and Next buttons, wrapper `<div className="tv-preview" role="region" aria-label="TV preview">`; beside the form on wide screens, above it on narrow ones; update the doc comment
- [X] T033 [US4] In `web/src/pages/DisplayPage.css` add `.tv-preview` (`aspect-ratio: 16 / 9; container-type: size; position: relative; width: 100%`) and `.tv-preview .display { position: absolute; --vh: 1cqh; --vw: 1cqw; }`; and update the `DisplayPage.css` header comment to say `.tv-preview` is the one rule outside `.display`; in `web/src/pages/AdminPage.css` add the rule placing the preview beside the form on wide screens and above it on narrow ones

**Checkpoint**: US1, US2 and US4 work; every later setting shows in the preview for free.

---

## Phase 6: User Story 3 - Align the title and choose fonts (Priority: P3)

**Goal**: title alignment and two font pickers from 8 shipped fonts.

**Independent Test**: change alignment and both fonts; the TV applies them with no request to any
other server; defaults bring back today's look.

### Tests for User Story 3

- [X] T034 [P] [US3] Create `web/src/fonts.test.ts`: `FONTS` has exactly the 8 keys in data-model order, each with a label and a class name; `system` is first
- [X] T035 [P] [US3] Extend `web/src/components/TvView.test.tsx`: `title_align` sets class `align-left|center|right` on the title; `title_font` sets the font class on the title only; `text_font` sets the font class on the rest and not the title; no title row when the title is empty whatever the alignment and font
- [X] T036 [P] [US3] Extend `web/src/components/SettingsPanel.test.tsx`: Title alignment, Title font and Text font selects (labels from `FONTS`) show loaded values and Save sends only changed fields

### Implementation for User Story 3

- [X] T037 [US3] Create `web/src/fonts.ts`: import `@fontsource/<name>/latin-400.css` and, where the package has it, `latin-700.css` for the 7 fonts (Bebas Neue and Pacifico 400 only); export `FONTS` (`key`, `label`, `className`) per research R5; doc comment notes fonts download only when used
- [X] T038 [US3] In `web/src/pages/DisplayPage.css` add `.font-<key>` classes, each `font-family: "<Font>", system-ui, sans-serif` so a failed font falls back to today's stack, `.font-system` = today's stack, and `.title.align-left|center|right` with `text-align`
- [X] T039 [US3] In `web/src/components/TvView.tsx` import `web/src/fonts.ts`, apply the title alignment and font classes to the title and the text font class to `.display`, with the title overriding it
- [X] T040 [US3] In `web/src/components/SettingsPanel.tsx` add a "Title" fieldset (alignment, font) and the Text font select

**Checkpoint**: US3 works on the TV and in the preview.

---

## Phase 7: User Story 5 - Choose text colors (Priority: P3)

**Goal**: title and text colors from a color picker, with reset to the theme color.

**Independent Test**: pick both colors; the preview then the TV use them; reset brings back the
theme colors; nothing is contrast-checked.

### Tests for User Story 5

- [X] T041 [P] [US5] Extend `web/src/components/TvView.test.tsx`: a `title_color` sets `--title-color` on `.display` (`element.style.getPropertyValue("--title-color")`) and `text_color` sets `--tv-text`; `""` sets neither. Whether the CSP allows these is checked only in a real browser by T048, not here
- [X] T042 [P] [US5] Extend `web/src/components/SettingsPanel.test.tsx`: Title color and Text color pickers (`input[type=color]`) show the stored color, or the theme's text color with a "Theme color" note when `""`; Reset sets `""`; Save sends the color lowercase; a low-contrast color saves with no warning

### Implementation for User Story 5

- [X] T043 [US5] In `web/src/components/TvView.tsx` pass `--title-color` and `--tv-text` through React's `style` prop on `.display` only when set; doc comment notes CSSOM is allowed by the CSP and values are validated `#rrggbb` (research R3, R4)
- [X] T044 [US5] In `web/src/pages/DisplayPage.css` make `.title` use `var(--title-color, var(--text))`, and inside cards, rows, the "On deck" heading and the "Nothing on tap" / "Nothing on deck" lines override `--text`, `--muted`, `--soft` and the status color with `var(--tv-text, …)` fallbacks; leave `--bg`, `--card`, art, pager and offline notice colors alone (FR-019)
- [X] T045 [US5] In `web/src/components/SettingsPanel.tsx` add the two color pickers with Reset buttons to the Title and Text fieldsets

**Checkpoint**: all five stories work.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T046 [P] Run the contract check and Redocly lint (`api/internal/httpapi/routes_test.go`, `npx @redocly/cli lint api/openapi.yaml`) and fix any drift between `api/openapi.yaml` and the handlers
- [X] T047 [P] Update `docs/` user and API docs for the new Display Settings fields and the menu's `srm`/`description` (the API docs split in commit 868d547)
- [ ] T048 Run quickstart steps 2 (defaults pixel-identical in both themes), 3 (API curl checks), 4 (the TV shows each saved change within 30 s with no reload, SC-001), 5 (preview vs TV screenshot, phone width) and 6 (headless Firefox through the nginx CSP: no violations, only the app's origin, only the 2 chosen font files) from `specs/010-tv-display-options/quickstart.md`
- [ ] T049 Run quickstart step 7 (every layout × font × theme with theme colors, 4.5:1 and 3 m) and tune `ROWS_PER_PAGE` in `web/src/tv.ts` if rows are unreadable; record the result in `specs/010-tv-display-options/research.md` R7
- [X] T050 Full gate: `go test -race ./...` in `api/`; `npx vitest run --maxWorkers=2`, `npm run lint`, `npm run build` in `web/`; `docker compose build`

---

## Dependencies & Execution Order

### Phase dependencies

- Setup (Phase 1): none. T001 and T002 can run together.
- Foundational (Phase 2): after Setup. Blocks every story.
- US1 (Phase 3): after Foundational.
- US2 (Phase 4): after Foundational. Independent of US1 except shared files (`TvView.tsx`,
  `DisplayPage.css`, `SettingsPanel.tsx`), so run after US1 to avoid conflicts.
- US4 (Phase 5): after Foundational; best after US1 and US2 so the preview shows them.
- US3 (Phase 6) and US5 (Phase 7): after US4, so they are checked in the preview.
- Polish (Phase 8): after every story.

### Within each phase

- Tests first and failing, then implementation.
- Go: T009 before T010 (handler uses `DisplayPatch`); T011 independent.
- Web: T012 before T013; T013 before T014; T014 before T015.

### Parallel opportunities

- Phase 1: T001 ∥ T002.
- Phase 2 tests: T003 ∥ T004 ∥ T005 ∥ T006 ∥ T007 ∥ T008. Go implementation (T009–T011) ∥ web
  implementation (T012–T016).
- Each story's test tasks are [P] with each other (different test files).
- Phase 8: T046 ∥ T047.

### Parallel example: Phase 2

```text
Go track:  T003, T004, T005, T006 -> T009 -> T010 ; T011
Web track: T007, T008 -> T012 -> T013 -> T014 -> T015 ; T016
```

## Implementation Strategy

### MVP first

1. Phase 1 and Phase 2: the TV looks unchanged and the API serves every setting.
2. Phase 3 (US1): detail switches. Stop and validate on the TV. This is a shippable MVP.

### Incremental delivery

3. US2 layouts, then US4 preview, then US3 fonts and alignment, then US5 colors, validating
   each on the TV and in the preview before the next.
4. Polish: docs, real-browser CSP check, readability pass, full gate.
