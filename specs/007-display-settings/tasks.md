---

description: "Task list for feature 7: Display settings"
---

# Tasks: Display Settings

**Input**: Design documents from `specs/007-display-settings/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md),
[contracts/settings.openapi.yaml](./contracts/settings.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks of the database; both new routes join the route
walk; there is no migration, so the upgrade case is a test on a version-5 database with
existing data; React components with logic get Vitest tests. Write each test task before its
implementation task and see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own. The settings API and the Settings panel are foundational because every story reads or
writes through them.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US3)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`). New package
  `api/internal/settings`; handlers in `api/internal/httpapi/settings.go`.
- Web: `web/` (Vite + React + TypeScript). New panel `web/src/components/SettingsPanel.tsx`.
- Test helpers: `api/internal/beers/beers_test.go` (`openDB`), `api/internal/importer/store_test.go`
  (opens a DB with the store migrations); `api/internal/httpapi/helpers_test.go` (`newTestDeps`,
  `testClock`, `do`, `insertToken`, `wantError`, `sessionCookie`);
  `api/internal/httpapi/auth_walk_test.go` (`routeFixtures`, `walkedRoutes`);
  `api/internal/httpapi/routes_test.go` (contract `x-access` check, `TestPublicRoutes`);
  `web/src/testApi.ts` (`stubApi`, `beer()`, `tap()`, `keg()`, `menu()`, `kegTypes`).

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go or web dependencies (plan). No migration; the database stays at version 5.
- Setting keys and rules, verbatim from data-model.md:
  - `display.title` / `title`: string, "trimmed, 0 to 60 code points, no control characters",
    default `""`.
  - `display.theme` / `theme`: "`dark`, `light`", default `dark`.
  - `display.units` / `units`: "`us`, `metric`", default `us`. Never public.
  - `display.page_seconds` / `page_seconds`: integer "5 to 300", default `15`, stored as decimal
    text.
  - "a missing row, or a stored value that fails validation, reads as the default".
- Validation errors are `400 {"error":"invalid_request","message":...,"fields":{...}}` keyed by
  JSON name (`beers.FieldErrors`); messages never echo the value.
- No inline `style` anywhere in the web app (nginx CSP has no `style-src`). TV rules stay scoped
  under `.display` in `web/src/pages/DisplayPage.css`. The admin app stays unstyled; the theme
  never changes it.
- Volumes stay litres in the API and database; units only change how the admin app shows and
  takes them.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the API contract and shared web types.

- [X] T001 Merge `specs/007-display-settings/contracts/settings.openapi.yaml` into `api/openapi.yaml`: add the `settings` tag; `/api/settings` with `getSettings` (`x-access: read`) and `updateSettings` (`x-access: write`, `400`/`401`/`403` responses as in the delta); schemas `Theme`, `Units`, `Title`, `PageSeconds`, `Settings`, `SettingsPatch`, `MenuDisplay`; add `display` (`$ref: MenuDisplay`) to `Menu.properties` and `Menu.required`; add `display: { title: Helm Taproom, theme: dark, page_seconds: 15 }` to the `getMenu` example and one sentence to its description ("`display` holds the title, theme and page interval the TV uses; units are not public"); bump `info.version` to `0.7.0`. Run `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` and fix errors (the `x-access` contract test fails until T009 adds the routes; expected)
- [X] T002 [P] In `web/src/api.ts`, add `Theme = "dark" | "light"`, `Units = "us" | "metric"`, `Settings = { title: string; theme: Theme; units: Units; page_seconds: number }`, `MenuDisplay = { title: string; theme: Theme; page_seconds: number }`, and `display?: MenuDisplay` on `Menu` (optional: the TV must cope with an API that predates it, research R8). In `web/src/testApi.ts`, add `settings(overrides)` (default `{ title: "", theme: "dark", units: "us", page_seconds: 15 }`) and give `menu()` a default `display: { title: "", theme: "dark", page_seconds: 15 }`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the `settings` package, both routes, `display` in the public menu, and the Settings
panel. Every story needs these.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

### Tests for Foundational

- [X] T003 [P] Tests in `api/internal/settings/settings_test.go` on real SQLite (open with the store migrations, as `importer/store_test.go` does): `Get` of a missing key returns `("", false, nil)`; `Set` then `Get` returns the value; a second `Set` replaces it and updates `updated_at`; `Set` inside a `*sql.Tx` that is rolled back stores nothing
- [X] T004 [P] Tests in `api/internal/settings/display_test.go`: `GetDisplay` on an empty table returns `{Title: "", Theme: "dark", Units: "us", PageSeconds: 15}`; stored bad values (`display.theme = "blue"`, `display.units = "imperial"`, `display.page_seconds` of `"4"`, `"301"`, `"15.5"`, `"x"`, a 61-code-point title, a title with `\x07`) each read as that field's default while the other fields keep their stored values; `PatchDisplay` accepts title of exactly 60 code points (multi-byte, e.g. `"é"` × 60), trims `"  Helm  "` to `"Helm"`, stores `"   "` as `""`, accepts `page_seconds` 5 and 300; refuses with `beers.FieldErrors` naming the field: 61 code points, a control character, theme `"blue"`, units `"imperial"`, page_seconds 4, 301; a patch with one valid and one invalid field writes nothing (read back unchanged); a patch naming only `title` leaves the other three rows absent; returns all four current values; `PublicOf(Display)` (or `Display.Public()`) drops `Units`; `GetDisplay` never reads or returns `brewfather.sync_interval`
- [X] T005 [P] Upgrade test `TestDisplayDefaultsOnVersion5Data` in `api/internal/settings/display_test.go`: open a database with the store migrations (version 5), insert a beer, a tap, a keg and `brewfather.sync_interval = "1h"`, then assert `GetDisplay` returns all defaults, `store.Version` is still 5, and the sync interval row is unchanged (FR-014; there is no migration to test)
- [X] T006 [P] Handler tests in `api/internal/httpapi/settings_test.go`: `GET /api/settings` with a session, a read token and a write token returns `200` and the four defaults; `PATCH /api/settings` with a session (and CSRF headers as other session writes) and with a write token returns `200` with all four, and a later GET shows them; refusals each `400 invalid_request` with `fields` and nothing stored: `{"page_seconds":4}`, `{"page_seconds":301}`, `{"page_seconds":15.5}`, `{"page_seconds":"20"}`, 61-code-point title, `{"theme":"blue"}`, `{"units":"imperial"}`, `{"title":null}`; `{}` is `400` (empty patch); `{"logo":"x"}` is `400` (unknown field); a body over 8 KiB is refused; the 400 message and `fields` values never contain the submitted title
- [X] T007 [P] Add `routeFixtures` entries in `api/internal/httpapi/auth_walk_test.go` for `GET /api/settings` (read) and `PATCH /api/settings` (write, body `{"title":"Walk"}`), so `TestWritesRefuseWeakCredentials`, `TestReadsNeedCredential`, `TestReadsAnyCredential` and `TestRouteFixturesMatchRoutes` cover them
- [X] T008 [P] Extend `api/internal/httpapi/menu_test.go`: `GET /api/menu` with no credential has `display` equal to `{"title":"","theme":"dark","page_seconds":15}` on a fresh DB, and the patched values after a `PATCH /api/settings`; `display` has exactly those three keys (no `units`, no `brewfather.sync_interval`); `taps` and `on_deck` are unchanged: update the exact-JSON `want` string in the existing menu test (`menu_test.go:41-42`) to end with `"display":{"title":"","theme":"dark","page_seconds":15}`, changing nothing else in it; with the `settings` table dropped from the test DB (`DROP TABLE settings`), `GET /api/menu` still returns `200` with the taps, on-deck list and default `display` (spec edge case "settings cannot be read"); `TestPublicRoutes` in `routes_test.go` stays unchanged

### Implementation for Foundational

- [X] T009 Create `api/internal/settings/settings.go`: package doc ("all SQL on the settings table"); a small interface satisfied by `*sql.DB` and `*sql.Tx` (`QueryRow`, `Exec`); `Get(q, key) (string, bool, error)` and `Set(q, key, value string, now time.Time) error`, moved from `api/internal/importer/store.go` (same SQL). In `importer/store.go` delete `GetSetting`/`SetSetting` and call `settings.Get`/`settings.Set`; update any importer test that called the old names. Create `api/internal/settings/display.go`: key constants, `Display` (`Title`, `Theme`, `Units` string; `PageSeconds` int; JSON `title`, `theme`, `units`, `page_seconds`), `Public` (title, theme, page_seconds), `DisplayPatch` (pointer fields), `GetDisplay(db) (Display, error)` (defaults and bad-value fallback per data-model.md "Rules"), `PatchDisplay(db, DisplayPatch, now) (Display, error)` (validate all → `beers.FieldErrors` or `beers.ErrEmptyPatch`; else one `db.Begin` transaction writing each named key with the same `now`), and the public subset. Title rule reuses the tap-label checks (trim, `utf8.RuneCountInString` ≤ 60, `unicode.IsControl`) without the required check (makes T003, T004, T005 pass)
- [X] T010 Add handlers in `api/internal/httpapi/settings.go`: `getSettings` and `updateSettings` with doc comments. `updateSettings` decodes with `decodeJSON(w, r, &raw, maxBody)` into a struct whose fields can tell absent from `null` (e.g. `json.RawMessage` per field, or `beers.Optional[T]`), refuses `null` per field as a field error, unknown fields via `decodeJSON`, a non-integer `page_seconds` (`15.5`, `"20"`) as a `page_seconds` field error, then calls `settings.PatchDisplay`; errors go through `writeBeerError` (`FieldErrors` → 400 with fields, `ErrEmptyPatch` → 400). Register in `api/internal/httpapi/routes.go`: `GET /api/settings` `AccessRead`, `PATCH /api/settings` `AccessWrite` (makes T006, T007 pass and the contract test green)
- [X] T011 Change `getMenu` in `api/internal/httpapi/menu.go` to respond with a struct embedding `kegs.Menu` plus `Display settings.Public \`json:"display"\``, filled from `settings.GetDisplay`; if `GetDisplay` returns an error, log it (`slog`, as other handlers do) and serve the default display with the menu rather than failing the TV's only read (a `GetMenu` error still returns 500 as today); update its doc comment (title, theme and page interval are public; units are not; a settings read error falls back to defaults). `kegs` does not import `settings` (makes T008 pass)
- [X] T012 [P] Test `web/src/components/SettingsPanel.test.tsx`: on mount it loads `GET /api/settings` and shows title, theme (Dark/Light select), units (US/Metric select) and page seconds (number input, min 5, max 300, step 1) with the loaded values, and calls `onChange` with the loaded settings; Save sends `PATCH /api/settings` with only the changed fields and calls `onChange` with the reply; Save with nothing changed sends nothing; a `400` with `fields` shows each message beside its field and keeps the typed values; a `401` on load or save calls `onSignedOut`; other errors show `errorMessage`
- [X] T013 Implement `web/src/components/SettingsPanel.tsx` (props `onSignedOut`, `onChange(s: Settings)`), unstyled like the other panels, with a doc comment (makes T012 pass). In `web/src/pages/AdminPage.tsx` add `SettingsPanel` after `BrewfatherPanel`, before `KegHistory`, and update the component doc comment; in `web/src/pages/AdminPage.test.tsx` stub `GET /api/settings` with `settings()` wherever the signed-in view renders

**Checkpoint**: `go test -race ./...` and `npm test` pass; settings can be read and written from
the API and the panel; the TV and admin volumes look the same as before.

---

## Phase 3: User Story 1 - Name the tap list and change how often the TV turns the page (Priority: P1) 🎯 MVP

**Goal**: the TV shows the title (when set) and turns pages at `page_seconds`, picked up on its
next menu check with no reload.

**Independent Test**: quickstart.md sections 1 and 2.

### Tests for User Story 1

- [X] T014 [P] [US1] Tests in `web/src/pages/DisplayPage.test.tsx` (fake timers, `stubApi` `menu()`): with `display.title = "Helm Taproom"` a heading with that text shows on tap pages and on on-deck pages; with `""` no title element renders; a title containing `<b>x</b>` renders as literal text; with 9+ taps and `page_seconds: 20` the page does not turn at 15 s and turns at 20 s; a later check that changes `page_seconds` from 20 to 5 makes the next turn come 5 s after that check lands, and later turns every 5 s; a check with the same `page_seconds` does not reset the page timer; a menu with no `display`, `display: null`, `display.title` not a string, `page_seconds` of `4`, `301`, `15.5` or `"20"` each still draws the menu and falls back field by field (no title, 15 s); a malformed `display` never counts as a failed check (no "offline" after 12 such checks); after a good check with title "Helm Taproom" and `page_seconds: 20`, failed checks (network error, error status, timeout) keep the title on screen and keep turning pages every 20 s (FR-008, spec edge case "TV is offline")

### Implementation for User Story 1

- [X] T015 [US1] In `web/src/pages/DisplayPage.tsx`: add a `displayOf(menu)` helper that returns `{ title, theme, pageMs }` from `menu.display`, checking each field on its own (title a string, theme `"light"` else dark, `page_seconds` an integer 5..300 else 15) and never failing `isMenu` for `display`; replace `PAGE_MS` with `pageMs` from the last good menu; make the page-turn effect depend on `[rotating, pageMs]`; render `<h1 className="title">{title}</h1>` above the cards on every page when the title is non-empty; update the component doc comment (title row, page interval from the menu, fallback). In `web/src/pages/DisplayPage.css` add `.display .title` (`flex: none`, `5vh`, bold, one line, `white-space: nowrap; overflow: hidden; text-overflow: ellipsis`) (makes T014 pass)
- [X] T016 [US1] Screenshot `/display` in headless Firefox at 1920×1080 (memory recipe) with no title and with "Helm Taproom", with 4 taps and with 9 taps (`taps-s` and `taps-m`): the title row does not push cards off screen or clip a glass; no console errors

**Checkpoint**: US1 works on its own: quickstart sections 1 and 2 pass.

---

## Phase 4: User Story 2 - Choose the TV's look (Priority: P2)

**Goal**: the TV uses the dark or light theme from the setting; the admin app never changes.

**Independent Test**: quickstart.md section 3.

### Tests for User Story 2

- [X] T017 [P] [US2] Tests in `web/src/pages/DisplayPage.test.tsx`: `display.theme = "light"` puts class `light` on the `<main className="display ...">`; `"dark"`, a missing theme and an unknown value such as `"blue"` do not; a later check switching to light adds the class with no reload; the theme never adds a `style` attribute anywhere in the page

### Implementation for User Story 2

- [X] T018 [US2] In `web/src/pages/DisplayPage.css`, move every color into custom properties on `.display` with the dark values from research R6 (`--bg #0b0b0d`, `--card #17171b`, `--text #f2f2f2`, `--muted #a9a9b0` for label/heading/nothing, `--soft #d2d2d8` for style/facts, `--accent #e0b04a` for status, `--dim #8c8c95` for the pager, changed from `#6f6f78` (3.95:1) to meet FR-011), and a `.display.light` rule overriding them (`--bg #ecebe6`, `--card #ffffff`, `--text #141416`, `--muted #55555d`, `--soft #33333a`, `--accent #7a4f00`, `--dim #5f5f68`); the offline badge (`#fff` on `#8b1e1e`) stays the same in both; the title uses `--text`. Add a comment block with the contrast ratios from research R6. In `web/src/pages/DisplayPage.tsx` add `light` to the `<main>` class when `displayOf(menu).theme === "light"` (makes T017 pass)
- [X] T019 [US2] Screenshot `/display` in headless Firefox at 1920×1080 in both themes with taps (empty, partly full, full), on-deck kegs and a fermenting beer, the pager and the offline notice: text readable, glass and keg outlines visible on the cards, the dark screenshot matches v0.5.0 apart from the pager color; screenshot the admin page with theme light to confirm it is unchanged

**Checkpoint**: US2 works on its own: quickstart section 3 passes.

---

## Phase 5: User Story 3 - Choose the units for volumes (Priority: P2)

**Goal**: every volume the admin app shows or takes uses the units setting; stored litres never
change; the TV is unchanged.

**Independent Test**: quickstart.md section 4.

### Tests for User Story 3

- [X] T020 [P] [US3] Tests in `web/src/units.test.ts` per research R9: `formatVolume(18.93, "us")` is `"5.00 gal"`, `formatVolume(18.93, "metric")` is `"18.93 L"`; `volumeText` gives the input text (`"5.00"`, `"18.93"`); `volumeToLitres(5, "us")` is `18.93`, `volumeToLitres(18.93, "metric")` is `18.93`, `volumeToLitres(18.934, "metric")` is `18.93`; `pourToLitres(16, "us")` is `0.47`, `pourToLitres(500, "metric")` is `0.5`; labels: volume unit `"gal"`/`"L"`, pour unit `"fl oz"`/`"mL"`, pour prefill `16`/`500`; round trip: `volumeToLitres(Number(volumeText(18.93, u)), u)` is `18.93` for `"metric"`, and switching `"us"` → `"metric"` → `"us"` never changes a litre value; `inputToLitres` still returns non-numeric text unchanged
- [X] T021 [P] [US3] Tests for metric in `web/src/components/TapsPanel.test.tsx`, `web/src/components/KegForm.test.tsx` and `web/src/components/KegHistory.test.tsx`: with `units="metric"` the taps panel shows `"12.50 L of 18.93 L"`, the level form labels "Pour (mL)" prefilled `500` and "Remaining (L)", a 500 mL pour sends `amount_l: 0.5`, a remaining of `10` sends `remaining_l: 10`; `KegForm` labels "Size (L)" and "Remaining (L)", prefills a corny type as `18.93`, and sends litres; `KegHistory` shows sizes in litres; with `units="us"` (and with no prop where a default applies) every existing US assertion still passes; in `TapsPanel`, changing `units` while a keg form is open remounts the form in the new units
- [X] T022 [P] [US3] Test in `web/src/pages/AdminPage.test.tsx`: when `GET /api/settings` returns `units: "metric"` the taps panel shows litres; before settings load (or if the read fails with a non-401 error) it shows US units

### Implementation for User Story 3

- [X] T023 [US3] In `web/src/units.ts`, add the units table and functions from research R9 (`formatVolume`, `volumeText`, `volumeToLitres`, `pourToLitres`, `volumeUnit`, `pourUnit`, `pourPrefill`), still rounding to litres with `roundL`; replace the "US units until feature 7" header comment; remove `gallons`, `formatGallons`, `gallonsToLitres`, `flOzToLitres`, `litresToGallons` once T024 leaves no caller (keep `LITRES_PER_GALLON`, `LITRES_PER_FL_OZ`, `inputToLitres`) (makes T020 pass)
- [X] T024 [US3] Add a `units: Units` prop to `web/src/components/TapsPanel.tsx`, `web/src/components/KegForm.tsx` and `web/src/components/KegHistory.tsx` and replace every gallon or fl oz call and label with the units functions; in `TapsPanel` pass `units` to `KegForm` and render it with `key={units}`; update each component's doc comment (volumes in the units setting). In `web/src/pages/AdminPage.tsx` hold `units` state (default `"us"`), set it from `SettingsPanel`'s `onChange`, and pass it to `TapsPanel` and `KegHistory` (makes T021, T022 pass)

**Checkpoint**: US3 works on its own: quickstart section 4 passes.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T025 [P] Run `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml`; confirm every route in `Routes` matches an operation and `x-access`, and `TestPublicRoutes` is unchanged (no new public route)
- [X] T026 [P] Update `README.md` where it lists features or API routes: display settings (title, TV theme, units, page interval) and the two `/api/settings` routes; note the TV picks settings up from `/api/menu`
- [X] T027 [P] Full gates: `(cd api && go vet ./... && go test -race ./...)`, `(cd web && npm ci && npm test && npm run build)`, `cd api && ~/go/bin/govulncheck ./...`, `~/go/bin/gitleaks detect --source . --redact` and `~/go/bin/gitleaks detect --no-git --source <dir> --redact` for `api`, `web/src`, `specs`, `docs`
- [X] T028 Run quickstart.md sections 1 to 6 on a local stack, including the upgrade from v0.5.0 (section 6: no migration and no new backup taken, Brewfather schedule kept)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none; T001 and T002 in parallel. After T001 the `x-access` contract test is red until T010 registers the routes.
- **Foundational (Phase 2)**: needs Setup. Tests T003 to T008 and T012 first, in parallel. T009 before T010 and T011. T013 needs T002.
- **US1 (Phase 3)**: needs T011 (menu `display`) and T002.
- **US2 (Phase 4)**: needs T015 (`displayOf`); T018 and T015 both edit `DisplayPage.tsx` and `DisplayPage.css`, so run US2 after US1.
- **US3 (Phase 5)**: needs T013 (`SettingsPanel.onChange`). Independent of US1 and US2 (different files except `AdminPage.tsx`, touched only by T013 and T024).
- **Polish (Phase 6)**: needs all stories.

### User Story Dependencies

- US1 → US2 (same TV files). US3 ∥ (US1 → US2).

### Within Each User Story

- Tests first, failing. API before web where both exist.
- API and web tasks touch disjoint folders and can run in parallel (two coding agents; only the
  orchestrator edits tasks.md).

### Parallel Opportunities

- Phase 2: API tests T003 to T008 and web test T012 all [P]; API (T009 to T011) and web (T013)
  implementation in parallel.
- After Phase 2: one web agent on the TV (US1 then US2) and another on units (US3).

---

## Parallel Example: after Setup

```text
Agent A (api): T003 T004 T005 T006 T007 T008 → T009 → T010 → T011
Agent B (web): T012 → T013 → T014 → T015 → T017 → T018
Agent C (web): T020 T021 T022 → T023 → T024   (after T013)
```

---

## Implementation Strategy

### MVP First

1. Phases 1 and 2.
2. Phase 3 (US1): title and page interval on the TV. Stop and validate quickstart sections 1
   and 2.

### Incremental Delivery

1. US1 → named TV with a chosen pace.
2. US2 → light theme.
3. US3 → metric admin.
4. Polish, local stack and upgrade check (T028), PR, propose `v0.6.0`.
