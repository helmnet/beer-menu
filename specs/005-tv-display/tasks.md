---

description: "Task list for feature 5: TV display"
---

# Tasks: TV Display

**Input**: Design documents from `specs/005-tv-display/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/menu.openapi.yaml](./contracts/menu.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks; the public route list is pinned; every other beer,
tap and keg read keeps its credential tests (existing route walk); React components with logic
get Vitest tests. Write each test task before its implementation task and see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US4)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`). The menu query goes in
  `api/internal/kegs/menu.go` (no HTTP); the handler in `api/internal/httpapi/menu.go`.
- Web: `web/` (Vite + React + TypeScript). The page is `web/src/pages/DisplayPage.tsx` with
  `web/src/pages/DisplayPage.css`.
- Test helpers: `api/internal/kegs/*_test.go` (`openDB`, `exec`, `insertBeer`, `insertKeg`,
  `newTap`, `mustCreate`, `ptr`); `api/internal/httpapi/helpers_test.go` (`newTestDeps`, `do`,
  `insertToken`, `wantError`, `sessionCookie`); `api/internal/httpapi/beers_test.go`
  (`newCreds`, `insertBeer`, `decodeObject`); `web/src/testApi.ts` (`stubApi`, `beer()`,
  `tap()`, `keg()`, `kegTypes`).

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go or web dependencies (plan).
- No inline `style` anywhere on the display or in `BeerArt`: classes from
  `DisplayPage.css`, and SVG presentation attributes (`fill`, `stroke`) only, so the nginx CSP
  (no `style-src`) stays unchanged (FR-016).
- The menu response uses its own structs (`kegs.Menu`, `MenuTap`, `OnDeckItem`, `MenuBeer`,
  `MenuKeg`), never the credentialed `Tap`, `Keg` or `beers.Beer` types (research R3), so a
  field added to those can never leak into the public read.
- Nothing on the display reads the TV clock (`Date.now()`, `new Date()`); timing counts poll
  ticks (research R2).
- The display shows no level number. The level appears only as the art's fill and in its
  `aria-label` (spec clarification 2026-11-09).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the API contract and shared web types.

- [X] T001 [P] Merge `specs/005-tv-display/contracts/menu.openapi.yaml` into `api/openapi.yaml`: add the `menu` tag, `GET /api/menu` (`operationId: getMenu`, `security: []`, `x-access: public`), and schemas `Menu`, `MenuTap`, `OnDeckItem`, `MenuBeer`, `MenuKeg` under `components/schemas`, reusing `HexColor`; bump `info.version` to `0.5.0`; `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` reports no errors
- [X] T002 [P] Add types `Menu { taps: MenuTap[]; on_deck: OnDeckItem[] }`, `MenuTap { label: string; beer: MenuBeer | null; keg: MenuKeg | null }`, `OnDeckItem { beer: MenuBeer; keg: MenuKeg | null; status: "fermenting" | "conditioning" | null }`, `MenuBeer { name: string; style: string | null; abv: number | null; ibu: number | null; color: string | null }`, `MenuKeg { shape: "corny" | "sanke"; level: number }` to `web/src/api.ts`; add an optional fourth argument `signal?: AbortSignal` to `apiFetch` passed through to `fetch` (existing callers unchanged); add a `menu(overrides)` fixture builder to `web/src/testApi.ts` returning `{ taps: [], on_deck: [] }` merged with overrides

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the `BeerArt` level prop, used by the TV taps (US1), on deck (US3) and the admin
panel (US4).

**⚠️ CRITICAL**: US1, US3 and US4 web tasks need this phase.

### Tests for Foundational

- [X] T003 [P] Level tests in `web/src/components/BeerArt.test.tsx`: with no `level`, output is unchanged from today (glass liquid path `M11.3 22 L16.8 90 L43.2 90 L48.7 22 Z`, keg liquid rects full, `aria-label` "<name>, pint glass" / "<name>, keg"); with `level` 0.5 the glass `data-part="liquid"` spans y 56 to 90 (top at `90 - 0.5 × 68`), the corny and sanke filled liquid rect has half the body height sitting on the body bottom (corny body y 20..86, sanke y 19..83), and the full-size body outline is still drawn; `level` 0 draws no liquid fill and no foam; `level` 0.001 fills 2% (sliver); `level` 1 fills fully; `aria-label` is "Pale Ale, pint glass, 60% left" at 0.6, "…, 1% left" at 0.001, "…, 99% left" at 0.996, "…, 100% left" at 1, "…, 0% left" at 0, and "…, keg, 50% left" for a keg; no element has a `style` attribute

### Implementation for Foundational

- [X] T004 Add `level?: number` (0..1) to `BeerArt` in `web/src/components/BeerArt.tsx` (research R5): `fill = level > 0 ? Math.max(level, 0.02) : 0`, clamped to 1; glass liquid is the trapezoid from y 90 up to `90 - fill × 68` with side x values interpolated along the walls (left wall from (11.3,22) to (16.8,90), right from (48.7,22) to (43.2,90)); foam band sits directly above the liquid top when `fill > 0` and is absent when 0; corny and sanke keep their body rect as an unfilled outline and add a filled `data-part="liquid"` rect of `fill × body height` on the body bottom; computed geometry only, no `clipPath`, no ids, no `style`; `aria-label` gains `, N% left` when `level` is given, N = `Math.round(level × 100)` raised to 1 when level > 0 and lowered to 99 when level < 1; with no `level` the drawing and label are exactly as before; update the component doc comment; T003 passes

**Checkpoint**: `npm test` passes; existing `BeerArt`, `BeersPanel`, `BeerForm` and `TapsPanel` tests unchanged and green.

---

## Phase 3: User Story 1 - See what is on tap (Priority: P1) 🎯 MVP

**Goal**: with no credential, `/display` shows enabled taps in position order with beer name,
style, ABV, IBU and a glass filled to the keg level; empty taps show "Empty"; disabled taps do
not show.

**Independent Test**: quickstart §1 (taps part), §2 and §3 (taps part): three enabled taps (two
with kegs, one empty) and one disabled tap; `curl $B/api/menu` with no credential and the
`/display` page show exactly the enabled taps in order.

### Tests for User Story 1

- [X] T005 [P] [US1] Menu tap tests in `api/internal/kegs/menu_test.go`: `GetMenu(db)` on a fresh DB returns the seeded "Tap 1" with `beer` and `keg` nil and `on_deck` an empty (non-nil) slice; enabled taps come in position order after a reorder; a disabled tap with a keg is absent; a tap's kicked keg is not shown (tap shows empty); a keg of 40 L with 10 L left gives `level` 0.25 and `shape` "corny" for `corny_5_gal`, "sanke" for a sanke and a euro type; `color` equals `beers.ColorFor(srm)` and is nil without SRM; nil `style`, `abv`, `ibu` stay nil; a current keg whose beer is archived by direct SQL (bypassing the feature 4 guard) gives that tap `beer` and `keg` both nil; the JSON of a `Menu` has exactly the keys in `specs/005-tv-display/data-model.md` (marshal and compare key sets at every level)
- [X] T006 [P] [US1] Handler tests in `api/internal/httpapi/menu_test.go`: `GET /api/menu` with no credential returns 200 and `Cache-Control: no-store`; with a session, a `read` token and a `write` token it returns the same body; the body (via `decodeObject`) has only keys `taps` and `on_deck`, each tap only `label`, `beer`, `keg`, each beer only `name`, `style`, `abv`, `ibu`, `color`, each keg only `shape`, `level` (no `id`, `size_l`, `remaining_l`, timestamps); `POST`, `PUT`, `PATCH` and `DELETE /api/menu` return 405 and change no data; `GET /api/beers`, `/api/taps`, `/api/kegs`, `/api/keg-types`, `/api/srm-colors` with no credential still return 401
- [X] T007 [P] [US1] Update `TestPublicRoutes` in `api/internal/httpapi/routes_test.go`: `want` becomes `GET /api/health,GET /api/setup,POST /api/setup,POST /api/session,GET /api/menu` (route table order); update its doc comment (feature 5 has added the menu read)
- [X] T008 [P] [US1] Display tests in `web/src/pages/DisplayPage.test.tsx` with `stubApi` for `GET /api/menu`: renders each tap's label, beer name, style, "5.2%" ABV and "35 IBU" in menu order; a tap with `beer` null shows its label and "Empty" and an empty glass (`BeerArt` with `onTap`, `color` null); missing style, ABV or IBU render nothing (no "null", no "0", no empty label); the glass `aria-label` is "Pale Ale, pint glass, 60% left" for level 0.6; no text matching `/\d+\s*%\s*left|gal/` is visible outside the SVG labels; no taps and no on deck shows "Nothing on tap"; the root `main` has class `taps-s` for 1–4 taps, `taps-m` for 5–8, `taps-l` for 9–12, `taps-xl` for 13+; no element in the page has a `style` attribute; the menu is requested with no credential header; rendering `AdminPage` after importing `DisplayPage` leaves `document.body` with no class and no display styles (the scoping rule is a review item; jsdom does not apply CSS)

### Implementation for User Story 1

- [X] T009 [US1] Create `api/internal/kegs/menu.go` (research R3, R4, R7): types `Menu { Taps []MenuTap "taps"; OnDeck []OnDeckItem "on_deck" }`, `MenuTap { Label string; Beer *MenuBeer; Keg *MenuKeg }`, `OnDeckItem { Beer MenuBeer; Keg *MenuKeg; Status *string }`, `MenuBeer { Name string; Style *string; ABV *float64; IBU *float64; Color *string }`, `MenuKeg { Shape string; Level float64 }` with JSON tags exactly as `data-model.md` (`label`, `beer`, `keg`, `status`, `name`, `style`, `abv`, `ibu`, `color`, `shape`, `level`; nullable fields marshal as `null`, never omitted); `func GetMenu(db *sql.DB) (Menu, error)` runs one `SELECT` for taps: `FROM taps t LEFT JOIN kegs k ON k.tap_id = t.id AND k.kicked_at IS NULL LEFT JOIN beers b ON b.id = k.beer_id AND b.archived_at IS NULL WHERE t.enabled = 1 ORDER BY t.position, t.id`; `level = remaining_l / size_l` clamped to [0,1], not rounded; `shape` from `TypeByKey(type).Shape`; `color` from `beers.ColorFor(srm)`; build `Keg` and `Beer` only when both the keg row and the beer row are present, otherwise both are nil (the tap shows empty), so `keg` is null exactly when `beer` is null even if an archived beer ever had a current keg; `OnDeck` is an empty non-nil slice in this story; T005 passes
- [X] T010 [US1] Create `getMenu` in `api/internal/httpapi/menu.go` (doc comment: public, display fields only, compatibility contract) writing `kegs.GetMenu(deps.DB)` with `writeJSON` 200, 500 via the existing internal-error path on failure; register `{Method: http.MethodGet, Path: "/api/menu", Access: AccessPublic, Handler: getMenu}` in `api/internal/httpapi/routes.go` after `POST /api/session`; no `routeFixtures` entry (public routes are not walked); T006, T007 and the `x-access` contract test pass
- [X] T011 [P] [US1] Create `web/src/pages/DisplayPage.css` (research R6): every rule scoped under `.display` (no `body`, `html` or bare element selectors, because `App.tsx` imports `DisplayPage` statically and its CSS loads on the admin page too); `.display` is `position: fixed; inset: 0; overflow: hidden` with its own near-black background and light text, so it covers the page without touching `body`; a tap grid with classes `taps-s` (1 row), `taps-m` (4 columns × 2 rows), `taps-l` (4 × 3), `taps-xl` (6 columns, smaller text); sizes in `vh` so beer names are about 5.2vh (≈56 px at 1080) for `taps-s`/`taps-m` and no text below 2.3vh (≈24 px) up to `taps-l`; beer name and style clamp at two lines (`display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden`); a bottom on-deck strip (`.on-deck`) of up to 9 equal-width cells (8 items plus "+N more"); an `.offline` corner notice that does not cover the menu; a centered `.nothing` message
- [X] T012 [US1] Rewrite `web/src/pages/DisplayPage.tsx`: import `./DisplayPage.css`; drop the inline `style`; load the menu once with `apiFetch<Menu>("GET", "/api/menu")` (polling comes in US2); render `<main className="display taps-…">` with each tap as a card: label, `BeerArt` (`onTap`, `color`, `name`, `level` from `keg.level`, `height` sized for the TV), beer name, then style, `abv` as "N%" and `ibu` as "N IBU" each only when not null; an empty tap shows label, "Empty" and `BeerArt` with `onTap` and `color` null; no taps and no on deck shows "Nothing on tap"; no level number; update the component doc comment; T008 passes

**Checkpoint**: quickstart §1 (taps), §2 and §3 (taps) work; `go test -race ./...` and `npm test` pass.

---

## Phase 4: User Story 2 - The display keeps itself current (Priority: P1)

**Goal**: the display polls, keeps the last good menu through failures, shows an offline notice
after 12 failed checks, and reloads itself after about a day, only after a good check.

**Independent Test**: quickstart §4: a pour shows within 30 s; stopping the API keeps the menu
and shows the notice; starting it clears the notice and later changes show, no reload.

### Tests for User Story 2

- [X] T013 [P] [US2] Polling tests in `web/src/pages/DisplayPage.test.tsx` with `vi.useFakeTimers()` and a stubbed `fetch` whose replies change per call: a second menu replaces the first after advancing 10 s; the next request is not sent before the previous one settles (no overlap with a slow reply); a failed check (network error, 500, a body without `taps`/`on_deck` arrays, a request still pending after advancing 8 s is aborted and counts as a failure, the fetch stub rejecting on its `signal`'s `abort` event) keeps the last good menu on screen and retries 10 s later; a page opened while the server is down shows no menu, then the menu after the first good reply; 11 failed checks in a row show no "offline" notice, the 12th shows it, the next good check removes it; `reload` (passed as a prop) is not called before 8,640 ticks, is not called on a failed check after that, and is called once on the first good check after 8,640 ticks; unmounting stops all timers (no request after unmount); the test never mocks `Date`

### Implementation for User Story 2

- [X] T014 [US2] Add polling to `web/src/pages/DisplayPage.tsx` (research R1, R2, R9): `DisplayPage({ reload = () => window.location.reload() })`; a `setTimeout` chain, 10 s after each check settles; each check calls `apiFetch<Menu>("GET", "/api/menu", undefined, ctrl.signal)` with an `AbortController` aborted by `setTimeout(..., TIMEOUT_MS)`, the timer cleared when the check settles and on unmount (no client cache option: the server's `Cache-Control: no-store` covers it, research R8); a reply without `taps` and `on_deck` arrays counts as a failure; on success replace the menu state (never append), reset the failure count, and call `reload` when the tick count is ≥ 8,640; on failure keep the last menu and increment the failure count; render a small `<div className="offline" role="status">offline</div>` when failures ≥ 12; clear the timer on unmount; constants `POLL_MS = 10_000`, `TIMEOUT_MS = 8_000`, `OFFLINE_AFTER = 12`, `RELOAD_AFTER = 8_640` with a one-line why comment each; no `Date` use; T013 passes

**Checkpoint**: quickstart §4 works against the local stack.

---

## Phase 5: User Story 3 - See what is on deck (Priority: P2)

**Goal**: waiting kegs (drawn as their keg shape, filled to level), then conditioning, then
fermenting beers with no current keg (drawn as a 5 gallon corny keg with their status), in a
bottom strip hidden when empty.

**Independent Test**: quickstart §1 (on-deck part) and §3 (on-deck part).

### Tests for User Story 3

- [X] T015 [P] [US3] On-deck tests in `api/internal/kegs/menu_test.go`: waiting kegs (`tap_id` NULL, not kicked) appear oldest first (`created_at`, then id) with `keg.shape`, `keg.level` and `status` nil; then beers with status `conditioning`, then `fermenting`, each ordered by name case-insensitively then id, with `keg` nil and `status` set; a beer with status `planned` or `ready` and no waiting keg is absent; an archived fermenting beer is absent; a conditioning beer with a waiting keg shows once, as the keg; a fermenting beer with a keg on a tap (enabled or disabled) is absent from on deck; a keg on a disabled tap is not on deck; a kicked keg is not on deck; the JSON key-set check from T005 covers `on_deck` items (`beer`, `keg`, `status`)
- [X] T016 [P] [US3] On-deck tests in `web/src/pages/DisplayPage.test.tsx`: a waiting corny keg renders `BeerArt` without `onTap` with `data-keg-shape="corny"`, its color, name, style and ABV, `aria-label` with "% left"; a sanke keg renders `data-keg-shape="sanke"`; a fermenting beer renders a corny keg in its color with "Fermenting", a conditioning beer "Conditioning"; items keep menu order; with 11 on-deck items, 8 render in order followed by "+3 more"; with exactly 8, no "+N more"; an empty `on_deck` renders no on-deck section; on deck with no taps does not show "Nothing on tap"

### Implementation for User Story 3

- [X] T017 [US3] Fill `OnDeck` in `kegs.GetMenu` in `api/internal/kegs/menu.go` (research R7): a second `SELECT` for waiting kegs `WHERE k.tap_id IS NULL AND k.kicked_at IS NULL AND b.archived_at IS NULL ORDER BY k.created_at, k.id`; a third for beers `WHERE b.archived_at IS NULL AND b.status IN ('conditioning','fermenting') AND NOT EXISTS (SELECT 1 FROM kegs k WHERE k.beer_id = b.id AND k.kicked_at IS NULL) ORDER BY b.status = 'fermenting', b.name COLLATE NOCASE, b.id`; keg items have `Status` nil, beer items `Keg` nil and `Status` set; T015 passes
- [X] T018 [US3] Add the on-deck strip to `web/src/pages/DisplayPage.tsx`: `<section className="on-deck" aria-label="On deck">` only when `on_deck` is non-empty; render the first `ON_DECK_MAX = 8` items (one-line why: readable at 3 m), then `<div className="more">+N more</div>` when more exist; each item `BeerArt` (keg: `kegShape` and `level` from `keg`; beer: default corny, full, no level), name, style, ABV when present, and "Fermenting"/"Conditioning" for beer items; "Nothing on tap" only when both lists are empty; T016 passes

**Checkpoint**: quickstart §1 and §3 fully work.

---

## Phase 6: User Story 4 - Keg level as artwork in the admin app (Priority: P3)

**Goal**: the admin Taps panel shows each current keg's level as its filled artwork with US
gallons beside it, and no `<meter>`.

**Independent Test**: quickstart §5.

### Tests for User Story 4

- [X] T019 [P] [US4] Update `web/src/components/TapsPanel.test.tsx`: replace the `getByRole("meter")` assertions (around line 315) with: a keg on a tap at 25% renders a pint glass whose `aria-label` ends "25% left" and the US gallons text as today; a waiting keg renders its keg shape with its level in the label; no element with role `meter` remains

### Implementation for User Story 4

- [X] T020 [US4] In `web/src/components/TapsPanel.tsx`, remove the `<meter>` and pass `level={k.remaining_l / k.size_l}` to the keg's existing `BeerArt` (on a tap and waiting); keep the gallons text (`formatGallons`); update the component doc comment that mentions the `<meter>` bar; T019 passes

**Checkpoint**: all stories work; `npm test` passes.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T021 [P] Confirm `api/openapi.yaml` matches the implementation (field names, nullability, 405s not listed); copy any change back into `specs/005-tv-display/contracts/menu.openapi.yaml`; redocly lint reports no errors
- [X] T022 [P] Update `README.md`: a short "TV display" section: open `/display` in Fully Kiosk, no credential; `GET /api/menu` is the only public read, with display fields only and the level as a fraction; Home Assistant and scripts use the credentialed keg reads for volumes; the display polls every 10 s and reloads itself about daily
- [X] T023 [P] Scale test in `api/internal/kegs/menu_test.go`: 20 taps with kegs, 50 on-deck items, 1,000 kicked kegs; `GetMenu` under 1 s (SC-006)
- [X] T024 Run `go vet ./...`, `go test -race ./...`, `npm run lint`, `npm test`, `npm run build`, `govulncheck ./...` (in `api/`), `gitleaks detect --source . --redact`, and `gitleaks detect --no-git --source <dir> --redact` on `api`, `web/src`, `specs`, `docs`; all pass
- [X] T025 Run quickstart §1 to §6 against images built from this worktree with `--pull never --build`, then confirm in the built page that the browser console shows no CSP errors on `/display`; record results in the PR. Quickstart §7 (real TV, 3 m, 7 days) is the owner's check after release

---

## Phase 7b: Page rotation (clarification 2026-11-09)

**Goal**: taps and on deck show in rotating pages of at most 8 cards, 15 s each, on deck as its
own page(s); supersedes the single-screen layout of T011/T012 and the on-deck strip and "+N
more" cap of T016/T018 (research R12, spec FR-017, SC-008).

**Independent Test**: with 12 taps and 3 on-deck items the TV shows tap page 1 (8 cards), tap
page 2 (4 cards), then the "On deck" page, 15 s each, with "N / 3" in a corner, every glass the
same size, no scrolling.

- [X] T026 [US1] Rotation tests in `web/src/pages/DisplayPage.test.tsx` (fake timers, see them fail first): 12 taps and 3 on-deck items make 3 pages, page 1 shows taps 1–8 only, after 15 s taps 9–12, after 30 s an "On deck" heading and the 3 items as kegs, after 45 s page 1 again; indicator text "1 / 3", "2 / 3", "3 / 3"; 3 taps and no on deck: one page, no indicator, no rotation timer (`vi.getTimerCount()` counts only the poll timer); 3 taps and 2 on-deck items: 2 pages; 20 on-deck items and 2 taps: 1 tap page and 3 on-deck pages of 8, 8, 4 (no "+N more"); on page 3 of 3 a menu update that leaves 1 page shows page 1, not a blank page; a menu update does not reset the current page; rotation continues while checks fail; page class `taps-s` for 1–4 cards and `taps-m` for 5–8; unmount clears the rotation timer; replace or drop the old tests for `taps-l`, `taps-xl`, the on-deck strip and "+N more"
- [X] T027 [US1] Implement rotation in `web/src/pages/DisplayPage.tsx` (research R12): build pages (tap chunks of 8, then on-deck chunks of 8); `PAGE_MS = 15_000` and `CARDS_PER_PAGE = 8` with one-line why comments and the `shortcut:` comment from R12 on `CARDS_PER_PAGE`; a separate `setTimeout` chain for rotation, running only when there is more than one page; current page kept across menu updates and reset to the first page when it runs past the last (a turn or a smaller menu), never a blank page; indicator `<div className="pager">N / M</div>` only with more than one page; on-deck pages have an `<h2>On deck</h2>` heading and use the same card grid with keg art (`kegShape`, `level`; beers being made as a full corny keg with their status); remove `ON_DECK_MAX`, the `.on-deck` strip and "+N more"; keep the doc comment accurate; no `Date`; T026 passes
- [X] T028 [P] [US1] Update `web/src/pages/DisplayPage.css`: drop `taps-l`, `taps-xl` and the on-deck strip and `.more` rules; give on-deck pages the card grid (same art share as tap cards); add `.pager` (small, corner, does not cover cards) and the "On deck" heading; with no on-deck strip, tap pages use the full height; keep all rules under `.display`, `vh` units, no inline styles
- [X] T029 [P] Update `README.md` "TV display" section and `specs/005-tv-display/quickstart.md` §3 and §6: pages of 8 rotating every 15 s, on deck as its own page, interval setting in feature 7
- [X] T030 Re-run the headless Firefox screenshots (scratch stack, 12 taps, 3 on deck, plus 3- and 7-tap views) for every page of the rotation; confirm equal glass sizes, no scrolling, no console errors

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none. T001 and T002 in parallel.
- **Foundational (Phase 2)**: T003 then T004. Blocks web tasks of US1, US3, US4.
- **US1 (Phase 3)**: API (T005–T007, T009, T010) after T001; web (T008, T011, T012) after T002
  and T004.
- **US2 (Phase 4)**: after US1 (same page file).
- **US3 (Phase 5)**: API after T009 (same file); web after US2's T014 (same page file).
- **US4 (Phase 6)**: after Foundational only.
- **Polish (Phase 7)**: after all stories.

### User Story Dependencies

```text
Setup ─> Foundational ─┬─> US1 ─> US2 ─> US3 ─┐
                       └─> US4 ───────────────┴─> Polish
```

### Within Each User Story

- Tests first, and see them fail.
- `internal/kegs` before `httpapi` before the page.

### Parallel Opportunities

- T001 and T002.
- In US1, the API track (T005–T007, T009, T010) beside the web track (T008, T011, T012).
- US4 (T019, T020) beside US1–US3 (it touches only `TapsPanel`).
- US3's API tasks (T015, T017) beside US2's web tasks (T013, T014).

---

## Parallel Example: User Story 1

```bash
# Tests together:
Task: "T005 menu tap tests in api/internal/kegs/menu_test.go"
Task: "T006 handler tests in api/internal/httpapi/menu_test.go"
Task: "T007 TestPublicRoutes in api/internal/httpapi/routes_test.go"
Task: "T008 display tests in web/src/pages/DisplayPage.test.tsx"

# Then API and web tracks side by side:
Task: "T009 kegs/menu.go" -> "T010 getMenu + route"
Task: "T011 DisplayPage.css" + "T012 DisplayPage.tsx"
```

---

## Implementation Strategy

### MVP First

1. Setup and Foundational.
2. US1 and US2: a live tap list that survives outages is the brief's MVP display. Stop and
   validate quickstart §1 to §4.
3. US3 adds coming soon; US4 brings the art to the admin panel.

### Incremental Delivery

Each story ends at a checkpoint where `go test -race ./...` and `npm test` pass and the matching
quickstart section works. Run every story through the dev-pipeline (implementation, quality,
security, verification) as one feature branch and one PR. Two coding agents fit: one API (all
`api/` tasks), one web (all `web/` tasks); only one edits `tasks.md`.
