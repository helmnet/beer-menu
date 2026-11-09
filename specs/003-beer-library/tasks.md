---

description: "Task list for feature 3: beer library"
---

# Tasks: Beer Library

**Input**: Design documents from `specs/003-beer-library/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/beers.openapi.yaml](./contracts/beers.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks; migration 0003 is applied to a version 2 database;
every beer write route has tests for no credential (401), `read` token (403), revoked token
(401), `write` token and session (success), and a cross-site browser request (403); React
components with logic get Vitest tests. Write each test task before its implementation task and
see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US5)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`); new package `api/internal/beers`
  holds rules, SQL and the color table with no HTTP; handlers in `api/internal/httpapi/beers.go`
- Web: `web/` (Vite + React + TypeScript), new components in `web/src/components/`
- Test helpers: `api/internal/httpapi/helpers_test.go` (`newTestDeps`, test clock), and
  `api/internal/store/migrate_test.go` (`openTemp`, `mustVersion`)

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go or web dependencies (plan).
- Error bodies are `{"error": "<code>", "message": "<text>"}`; beer validation failures add
  `"fields": {"<field>": "<message>"}` (research R4). Unknown fields, including `source` and
  `source_ref`, give `400 invalid_request` (research R3).
- Times: `INTEGER` Unix seconds in SQLite, RFC 3339 in JSON; `brew_date` is `TEXT`
  `YYYY-MM-DD`. Use `deps.Now()` for every timestamp so tests control time.
- Optional fields are always present in responses, as `null` when empty (FR-003).
- Register each route in `api/internal/httpapi/routes.go` with its `Access` in the same task as
  its handler. Its operation is already in `api/openapi.yaml` from T001.
- Artwork uses SVG presentation attributes (`fill`, `stroke`), never inline `style` strings, so
  the nginx CSP stays unchanged (research R7).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the contract and client types the rest of the feature builds against.

- [X] T001 Merge `specs/003-beer-library/contracts/beers.openapi.yaml` into `api/openapi.yaml`: add the `beers` tag, the paths `/api/beers`, `/api/beers/{id}` and `/api/srm-colors`, the `InvalidBeer` response and the schemas `HexColor`, `BeerStatus`, `ValidationError`, `Beer`, `BeerCreate`, `BeerPatch`; bump `info.version` to `0.3.0`; confirm `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` reports no errors
- [X] T002 [P] In `web/src/api.ts` add `BeerStatus = "planned" | "fermenting" | "conditioning" | "ready"` and a `Beer` interface matching the contract's `Beer` schema (nullable fields typed `T | null`); give `ApiError` an optional `fields?: Record<string, string>` read from the error body's `fields` object; extend `web/src/api.test.ts` with a case where a 400 body carries `fields`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the table, rules, color table and read endpoints every story uses.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

### Tests for Foundational

- [X] T003 [P] Migration test `TestMigration0003FromVersion2` in `api/internal/store/migrate_test.go`, modelled on `TestMigration0002FromVersion1`: build a version 2 database holding one admin and one API token, apply pending migrations, assert version 3, the admin and token rows unchanged, and that `beers` accepts a row with only `name`, `created_at`, `updated_at` (defaults `status = 'ready'`, `source = 'manual'`) and refuses `status = 'brewing'` and `source = 'x'` by `CHECK`
- [X] T004 [P] Tests in `api/internal/httpapi/json_test.go` for `decodeJSON` taking a byte limit: a body of exactly the limit decodes, one byte over gives `400 invalid_request`, and the existing 8 KiB callers still refuse a 9 KiB body; the error message names no size
- [X] T005 [P] Tests for `Optional[T]` in `api/internal/beers/optional_test.go`: decoding `{}` leaves `Set == false`; `{"x": null}` gives `Set == true, Value == nil`; `{"x": 5}` gives `Set == true, *Value == 5`; works inside a struct decoded with `DisallowUnknownFields`
- [X] T006 [P] Tests for the color table in `api/internal/beers/srm_test.go`: table has exactly 40 entries, each matching `^#[0-9a-f]{6}$`; `ColorFor(nil) == nil`; every SRM 0 to 40 in steps of 1 (0 → entry for 1, n → entry n); 4.4 → entry 4; 4.5 → entry 5; 60 and 100 → entry 40; each entry is no lighter than the one before (sum of RGB non-increasing)
- [X] T007 [P] Tests for validation in `api/internal/beers/validate_test.go`, one case per rule, each asserting the `FieldErrors` key: name `""`, `"   "`, 101 code points, a control character → `name`; name of exactly 100 emoji passes; `style` and `brewer` "trimmed, up to 100, no control characters; empty → NULL"; `description` "trimmed, up to 2000; newline and tab allowed" (2000 multi-byte code points pass, 2001 fail, `\r` and `\x00` fail); `abv` "0 to 25, at most one decimal" (−0.1, 25.1, 6.75 fail; 0, 25, 6.7 pass); `ibu` "0 to 200" integer (−1, 201 fail); `srm` "0 to 100, at most one decimal" (100.1, 4.25 fail); `brew_date` `YYYY-MM-DD` (`2026-02-30`, `2026-1-5`, `01/05/2026` fail; a future date passes); `status` one of `planned`, `fermenting`, `conditioning`, `ready`; several bad fields give several keys
- [X] T008 [P] Store tests in `api/internal/beers/beers_test.go` on a real migrated SQLite file: `Get` of a missing id returns `ErrNotFound`; `Get` returns every column with `nil` for empty optionals and `Color` derived from `SRM`; `List(db, false)` returns only rows with `archived_at IS NULL`, ordered `name COLLATE NOCASE, id` (insert "beta", "Alpha", "alpha" and check order and tie on id); `List(db, true)` returns only archived rows; 500 rows list in under 1 s (SC-006; loosened from 100 ms in quality review to avoid CI flakiness)
- [X] T009 [P] Handler tests in `api/internal/httpapi/beers_test.go` for the reads: `GET /api/beers` with no credential gives 200 and a JSON array (`[]` when empty, never `null`); `GET /api/beers/{id}` with no credential gives 200 with every contract field present, including `color` and `null` optionals; unknown id, `0`, `-1` and `abc` give `404 not_found`; responses carry `Cache-Control: no-store` and the API CSP like other routes

### Implementation for Foundational

- [X] T010 Create `api/internal/store/migrations/0003_beers.sql` with the `beers` table exactly as in `specs/003-beer-library/data-model.md`: `id INTEGER PRIMARY KEY`, `name TEXT NOT NULL`, `style TEXT`, `abv REAL`, `ibu INTEGER`, `srm REAL`, `description TEXT`, `brewer TEXT`, `brew_date TEXT`, `status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('planned','fermenting','conditioning','ready'))`, `source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','brewfather'))`, `source_ref TEXT`, `archived_at INTEGER`, `created_at INTEGER NOT NULL`, `updated_at INTEGER NOT NULL`; no indexes; update any test in `api/internal/store/` or `api/cmd/beer-menu-api/` that hard-codes the newest version 2 (depends on T003)
- [X] T011 [P] Change `decodeJSON` in `api/internal/httpapi/json.go` to `decodeJSON(w, r, dst, limit int64)`, keep `maxBody` (8 KiB) for existing callers in `setup.go`, `session.go`, `admins.go`, `tokens.go`, add `maxBeerBody = 16 << 10` (research R5), and change the error message to "Request body must be one JSON object with only the documented fields, within the size limit." (depends on T004)
- [X] T012 [P] Create `api/internal/beers/optional.go`: `Optional[T any] struct { Set bool; Value *T }` with `UnmarshalJSON` that sets `Set` and leaves `Value` nil for `null` (research R2) (depends on T005)
- [X] T013 [P] Create `api/internal/beers/srm.go`: `var SRMColors = [40]string{...}` holding the widely published SRM 1–40 sRGB hex table, lower-case `#rrggbb`, copied from a public source whose URL is cited in a comment above it, and `ColorFor(srm *float64) *string` that rounds to the nearest integer, clamps to 1..40 and looks up (research R6) (depends on T006)
- [X] T014 Create `api/internal/beers/validate.go`: `FieldErrors map[string]string` implementing `error`; `Input` struct for create (`Name string`, pointer optionals, `Status *string`) and `Patch` struct for PATCH (`Optional[...]` for every field plus `Archived Optional[bool]`), both with JSON tags matching the contract; `(Input) Normalize() (Values, error)` and `(Patch) Normalize() (...)` applying research R4 verbatim — trim text, empty optional text → nil, code-point lengths, control-character rules, `abv` "0 to 25, at most one decimal", `ibu` integer "0 to 200", `srm` "0 to 100, at most one decimal" checked as `|x·10 − round(x·10)| < 1e-9`, `brew_date` via `time.Parse("2006-01-02")`, `status` enum, default `ready` on create; PATCH refuses `null` for `name`, `status`, `archived` (depends on T007, T012)
- [X] T015 Create `api/internal/beers/beers.go`: `Beer` struct with JSON tags for every contract field (`archived_at`, `created_at`, `updated_at` as `time.Time`/`*time.Time`, `Color *string`), `ErrNotFound`, `Get(db, id) (Beer, error)` and `List(db, archived bool) ([]Beer, error)` returning a non-nil slice, ordered `name COLLATE NOCASE, id`, with `Color` from `ColorFor` (depends on T008, T010, T013)
- [X] T016 Create `api/internal/httpapi/beers.go` with `listBeers` (`GET /api/beers`, current beers only for now) and `getBeer` (`GET /api/beers/{id}`, using `pathID` and `writeNotFound`), and register both in `api/internal/httpapi/routes.go` with `AccessPublic` (depends on T001, T009, T015)

**Checkpoint**: `go test ./...` passes; the read endpoints answer and the contract test covers them.

---

## Phase 3: User Story 1 - Add and edit beers (Priority: P1) 🎯 MVP

**Goal**: admins and `write` tokens create and edit beers; the admin app lists and edits them.

**Independent Test**: sign in, add a beer with only a name, add one with every field, edit one
field of the first; both show in the admin list and in `GET /api/beers` (quickstart §2).

### Tests for User Story 1

- [X] T017 [P] [US1] Store tests in `api/internal/beers/beers_test.go` for `Create` and `Update`: create with name only gives `status = ready`, `source = manual`, `source_ref = nil`, `created_at == updated_at == now`; create with every field round-trips each value exactly, including emoji text; `Update` with one field changes only it and sets `updated_at`; `Update` with `null` clears an optional; `Update` on a missing id gives `ErrNotFound`; a validation failure changes nothing; the id is unchanged after `Update` (FR-007); two beers with the same name are both created with different ids (spec edge case)
- [X] T018 [P] [US1] Handler tests in `api/internal/httpapi/beers_test.go` for `POST /api/beers` and `PATCH /api/beers/{id}`: 201 with the `Beer` body; 200 after PATCH with only the sent field changed; `{"abv":150}` gives 400 with `fields.abv`; bodies with `source` or `source_ref` give `400 invalid_request`; an empty PATCH `{}` gives 400; a 16 KiB body is accepted and 16 KiB + 1 refused; PATCH unknown id gives 404. Auth for both routes: no credential 401, `read` token 403, revoked token 401, `write` token and session succeed, session request with `Sec-Fetch-Site: cross-site` gives 403 and creates nothing
- [X] T019 [P] [US1] Component test `web/src/components/BeerForm.test.tsx`: add mode posts the entered fields (empty inputs omitted) and calls `onSaved` with the result; edit mode sends a PATCH with only changed fields and `null` for a cleared field; a 400 with `fields` shows each message next to its field; a 401 calls `onSignedOut`
- [X] T020 [P] [US1] Component test `web/src/components/BeersPanel.test.tsx`: loads `GET /api/beers` and shows name, style, ABV and status per beer; "Add beer" opens `BeerForm` and the new beer appears after save; "Edit" opens the form with the beer's values; a 401 calls `onSignedOut`

### Implementation for User Story 1

- [X] T021 [US1] Add `Create(db, Input, now) (Beer, error)` and `Update(db, id, Patch, now) (Beer, error)` to `api/internal/beers/beers.go`, validating through T014 before any write and running each in one statement or transaction; `Update` ignores `Archived` for now (depends on T014, T015, T017)
- [X] T022 [US1] Add `createBeer` (`POST /api/beers`, 201) and `updateBeer` (`PATCH /api/beers/{id}`, 200) to `api/internal/httpapi/beers.go`, decoding with `maxBeerBody`, answering `FieldErrors` as `400 {"error":"invalid_request","message":"Some fields are invalid.","fields":{...}}`; register both in `routes.go` with `AccessWrite`; log beer id and name only (depends on T011, T018, T021)
- [X] T023 [P] [US1] Create `web/src/components/BeerForm.tsx`: add and edit form for name, style, ABV, IBU, SRM, description (textarea), brewer, brew date (`type="date"`), status (select); props `beer?: Beer`, `onSaved(beer)`, `onCancel()`, `onSignedOut()`; numeric inputs parsed to numbers, empty → omitted on add and `null` on edit when previously set; uses `apiFetch` and `errorMessage` (depends on T002, T019)
- [X] T024 [US1] Create `web/src/components/BeersPanel.tsx` listing current beers (name, style, ABV, status) with "Add beer" and per-row "Edit" opening `BeerForm`; prop `onSignedOut`; add it to the signed-in view in `web/src/pages/AdminPage.tsx` above the admins and tokens panels, passing the same `toLogin` callback as the feature 2 panels; update the fetch stubs in `web/src/pages/AdminPage.test.tsx` to answer `GET /api/beers` with `[]` so the existing tests still pass, and add one case that the signed-in view shows the beer library (depends on T020, T023)

**Checkpoint**: quickstart §2 passes.

---

## Phase 4: User Story 2 - Read beers without signing in (Priority: P1)

**Goal**: prove the public read contract and that every change refuses missing or weak
credentials.

**Independent Test**: quickstart §3.

### Tests for User Story 2

- [X] T025 [P] [US2] Table-driven test `TestBeerWritesRefuseWeakCredentials` in `api/internal/httpapi/beers_test.go` that walks `Routes` for every `/api/beers` route whose method is not GET and, for each, asserts no credential 401, `read` token 403, revoked token 401, and a cross-site session request 403, and that the beer table is unchanged afterwards; it fails if a beer write route is ever registered with weaker access (FR-020, SC-003)
- [X] T026 [P] [US2] Test in `api/internal/httpapi/beers_test.go` that `GET /api/beers` and `GET /api/beers/{id}` succeed with no credential, with a `read` token and with a session, and that a beer with `srm` 7 returns `color` equal to `beers.SRMColors[6]` and one without SRM returns `"color": null` (FR-013, SC-002)

### Implementation for User Story 2

- [X] T027 [US2] Fix anything T025 or T026 exposes in `api/internal/httpapi/beers.go` or `routes.go`; if both pass unchanged, record that in the PR description (depends on T022, T025, T026)

**Checkpoint**: quickstart §3 passes.

---

## Phase 5: User Story 3 - See each beer's artwork (Priority: P2)

**Goal**: original SRM-colored glass and keg artwork in the admin list and a live preview in the
form.

**Independent Test**: quickstart §4.

### Tests for User Story 3

- [X] T028 [P] [US3] Handler test in `api/internal/httpapi/beers_test.go`: `GET /api/srm-colors` with no credential gives 200 `{"colors": [...]}` equal to `beers.SRMColors`
- [X] T029 [P] [US3] Component test `web/src/components/BeerArt.test.tsx`: `onTap` renders the glass and no `onTap` renders the keg, whatever the beer's status (distinguish by a `data-vessel="glass"|"keg"` attribute on the root `svg`); the liquid element's `fill` equals the `color` prop; `color={null}` renders the liquid with `fill="none"`; root has `role="img"` and `aria-label` "<name>, pint glass" or "<name>, keg"; `height` prop sets the svg height; no element has a `style` attribute and nothing has an `href` to an external resource. Also test `colorForSrm` with a 40-entry table: every SRM 0 to 40 maps to the same index as the Go `ColorFor` test in T006 (0 → index 0, n → index n−1), plus 4.4 → 3, 4.5 → 4, 60 and 100 → 39, `null` → `null` (SC-004, research R6)
- [X] T030 [P] [US3] Extend `web/src/components/BeerForm.test.tsx`: the form fetches `/api/srm-colors` once; typing SRM 20 shows the preview liquid filled with `colors[19]` before saving; the preview is a keg and changing status from ready to conditioning leaves it a keg with the same fill; SRM 4.5 previews `colors[4]` and 60 previews `colors[39]`
- [X] T031 [P] [US3] Extend `web/src/components/BeersPanel.test.tsx`: each row shows a `BeerArt` using the beer's `color` from the API, drawn as a keg

### Implementation for User Story 3

- [X] T032 [US3] Add `getSrmColors` (`GET /api/srm-colors`) to `api/internal/httpapi/beers.go` returning `{"colors": SRMColors}` and register it with `AccessPublic` (depends on T013, T028)
- [X] T033 [US3] Create `web/src/components/BeerArt.tsx` (research R7): `BeerArt({onTap, color, name, height})` returning inline SVG, the glass when `onTap` and the keg otherwise (`onTap` defaults to false; feature 4 sets it) with a fixed `viewBox`; the glass is an original shaker pint (tapered body, foam band, liquid area); the keg is an original 5 gallon corny keg (cylindrical body, top and bottom rubber bands, handle cutouts; the whole body between the bands is the liquid, like a full keg — owner feedback 2026-11-09 replaced the first design's small window); liquid `fill={color ?? "none"}`; outlines in a neutral stroke that reads on light and dark backgrounds; export `colorForSrm(colors: string[], srm: number | null): string | null` with the same round-and-clamp rule as the API (depends on T029)
- [X] T034 [US3] In `web/src/components/BeerForm.tsx` fetch `/api/srm-colors` once on mount and show a live `BeerArt` preview driven by the current SRM input via `colorForSrm`, always a keg (FR-042); a failed fetch shows the preview empty, not an error (depends on T030, T032, T033)
- [X] T035 [US3] In `web/src/components/BeersPanel.tsx` show `BeerArt` (about 64 px high) in each row, from the beer's `color`, with no `onTap`, so every row is a keg (FR-040) (depends on T031, T033)

**Checkpoint**: quickstart §4 passes, including the owner's check of three beers (SC-004).

---

## Phase 6: User Story 4 - Archive and restore beers (Priority: P2)

**Goal**: archive and restore through PATCH; archived view in the API and admin app.

**Independent Test**: quickstart §5.

### Tests for User Story 4

- [X] T036 [P] [US4] Store tests in `api/internal/beers/beers_test.go`: `Update` with `Archived = true` sets `archived_at = now`; again at a later time keeps the first `archived_at`; `Archived = false` clears it; restoring a current beer is a no-op success; other fields unchanged by archive and restore; an archived beer can still be edited
- [X] T037 [P] [US4] Handler tests in `api/internal/httpapi/beers_test.go`: `PATCH {"archived":true}` hides the beer from `GET /api/beers` and shows it in `GET /api/beers?archived=true`; `GET /api/beers/{id}` still returns it with `archived_at` set; `?archived=false` equals the default; `?archived=yes` gives `400 invalid_request`; `PATCH {"archived":null}` gives 400
- [X] T038 [P] [US4] Extend `web/src/components/BeersPanel.test.tsx`: "Show archived" fetches `/api/beers?archived=true` and marks rows archived; "Archive" sends `PATCH {"archived":true}` and removes the row; "Restore" in the archived view sends `{"archived":false}` and removes the row there

### Implementation for User Story 4

- [X] T039 [US4] Handle `Archived` in `Update` in `api/internal/beers/beers.go`: `archived_at = COALESCE(archived_at, now)` when true, `NULL` when false (FR-023) (depends on T021, T036)
- [X] T040 [US4] In `listBeers` in `api/internal/httpapi/beers.go` parse the `archived` query value with `strconv.ParseBool` (absent = false; invalid → `400 invalid_request`) and pass it to `beers.List` (depends on T016, T037, T039)
- [X] T041 [US4] In `web/src/components/BeersPanel.tsx` add a "Show archived" toggle, an "Archive" button on current rows and a "Restore" button on archived rows (FR-040, FR-043) (depends on T035, T038)

**Checkpoint**: quickstart §5 passes.

---

## Phase 7: User Story 5 - Delete a beer (Priority: P3)

**Goal**: permanent delete with confirmation in the admin app.

**Independent Test**: quickstart §6.

### Tests for User Story 5

- [X] T042 [P] [US5] Store test in `api/internal/beers/beers_test.go`: `Delete` removes the row; a second `Delete` and a missing id give `ErrNotFound`; an archived beer can be deleted
- [X] T043 [P] [US5] Handler tests in `api/internal/httpapi/beers_test.go`: `DELETE /api/beers/{id}` gives 204 and a later `GET` gives 404; unknown id gives 404; confirm `TestBeerWritesRefuseWeakCredentials` (T025) now covers `DELETE` (no credential 401, `read` 403, revoked 401, cross-site 403), and that `write` token and session succeed
- [X] T044 [P] [US5] Extend `web/src/components/BeersPanel.test.tsx`: "Delete" calls `window.confirm`; cancel sends nothing and keeps the row; confirm sends `DELETE` and removes the row

### Implementation for User Story 5

- [X] T045 [US5] Add `Delete(db, id) error` to `api/internal/beers/beers.go` (depends on T042)
- [X] T046 [US5] Add `deleteBeer` (`DELETE /api/beers/{id}`, 204) to `api/internal/httpapi/beers.go` and register it with `AccessWrite` (depends on T043, T045)
- [X] T047 [US5] In `web/src/components/BeersPanel.tsx` add a "Delete" button that asks `window.confirm("Delete <name> permanently?")` first (FR-043) (depends on T041, T044)

**Checkpoint**: quickstart §6 passes.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T048 [P] Confirm `api/openapi.yaml` matches the implementation (status codes, `fields`, `archived` query, `minProperties: 1` on `BeerPatch`); copy any change back into `specs/003-beer-library/contracts/beers.openapi.yaml`; redocly lint reports no errors
- [X] T049 [P] Update `README.md`: a short "Beers" section listing the endpoints and their access (public reads, `write` for changes), the PATCH merge semantics, and that beer artwork is generated from SRM with no uploads
- [X] T050 Run `go vet ./...`, `go test ./...`, `npm run lint`, `npm test`, `npm run build` and the compose smoke test locally; all pass
- [X] T051 Run quickstart §1 to §7 against images built with `--pull never --build` from a copy of a feature 2 data folder, and record results, including the SC-001 timing, the SC-004 color check and SC-006 timings, in the PR. In the PR review, confirm the glass and keg drawings in `web/src/components/BeerArt.tsx` are original work, not traced from taplist.io or any other product (FR-036)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: depends on T001 (contract, for T016's route test). Blocks all
  stories.
- **US1 (Phase 3)**: after Foundational.
- **US2 (Phase 4)**: after US1 (needs the write routes to test refusals).
- **US3 (Phase 5)**: API part (T028, T032) after Foundational; web part after US1 (`BeerForm`,
  `BeersPanel`).
- **US4 (Phase 6)**: after US1 (`Update`, `BeersPanel`); T041 after T035.
- **US5 (Phase 7)**: API part after US2 (T043 extends T025's refusal test) and after US4's
  edits to `api/internal/httpapi/beers.go` and `beers_test.go` (same files); T047 after T041.
- **Polish (Phase 8)**: after all stories.

### User Story Dependencies

```text
Setup ─> Foundational ─> US1 ─┬─> US2 ─────────────┐
                              └─> US3 ─> US4 ──────┴─> US5
```

### Within Each User Story

- Tests first, and see them fail.
- `internal/beers` functions before `httpapi` handlers before web components.
- Register each route with its `Access` in the same task as its handler.

### Parallel Opportunities

- T001 and T002 in Setup.
- T003 to T009 (all Foundational tests) together; then T011, T012, T013 together; T014 after
  T012; T015 after T010 and T013.
- Within each story, all `[P]` test tasks together; T023 beside T021/T022.
- US2 beside US3's web tasks (different files). Stories that edit
  `api/internal/httpapi/beers.go` or `beers_test.go` (US1 to US5) run one after another.

---

## Parallel Example: User Story 1

```bash
# Tests together:
Task: "Store tests for Create and Update in api/internal/beers/beers_test.go (T017)"
Task: "Handler and auth tests for POST and PATCH in api/internal/httpapi/beers_test.go (T018)"
Task: "Component test in web/src/components/BeerForm.test.tsx (T019)"
Task: "Component test in web/src/components/BeersPanel.test.tsx (T020)"
```

---

## Implementation Strategy

### MVP First (P1 stories)

1. Phase 1 and Phase 2.
2. US1 and US2: add, edit and publicly read beers, with every refusal tested.
3. **Stop and validate**: quickstart §1 to §3.

### Incremental Delivery

1. Add US3 (artwork), then US4 (archive), then US5 (delete).
2. Polish, then one PR for the whole feature; the dev-pipeline reviews the feature as a whole.

### Notes

- Every code task goes through the dev-pipeline (coding-agent, code-quality, code-security).
- Commit after each task or logical group.
- Stop at any checkpoint to validate the story on its own.
