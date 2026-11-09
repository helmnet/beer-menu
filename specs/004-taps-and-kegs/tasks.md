---

description: "Task list for feature 4: taps and kegs"
---

# Tasks: Taps and Kegs

**Input**: Design documents from `specs/004-taps-and-kegs/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/taps-kegs.openapi.yaml](./contracts/taps-kegs.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks; migration 0004 is applied to a version 3 database;
every write route has tests for no credential (401), `read` token (403), revoked token (401),
`write` token and session (success), and a cross-site browser request (403); every read route
for no credential and revoked token (401); React components with logic get Vitest tests. Write
each test task before its implementation task and see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US4)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`); new package `api/internal/kegs`
  holds taps, kegs, keg types, rules and SQL with no HTTP; handlers in
  `api/internal/httpapi/taps.go` and `api/internal/httpapi/kegs.go`
- Web: `web/` (Vite + React + TypeScript), new components in `web/src/components/`
- Test helpers: `api/internal/httpapi/helpers_test.go` (`newTestDeps`, `do`, `insertToken`,
  `wantError`), `api/internal/httpapi/beers_test.go` (`newCreds`, `insertBeer`,
  `decodeObject`), `api/internal/store/migrate_test.go` (`openTemp`, `mustVersion`)

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go or web dependencies (plan).
- Reuse `beers.Optional[T]` for PATCH fields and `beers.FieldErrors` for field errors. Field
  errors answer `400 {"error":"invalid_request","message":...,"fields":{...}}`; unknown fields
  give `400 invalid_request` through `decodeJSON(w, r, dst, maxBody)` (8 KiB).
- Conflicts answer `409` with these exact `error` codes (research R5): `tap_occupied`,
  `tap_has_keg`, `last_tap`, `keg_kicked`, `beer_has_keg`, `beer_has_kegs`.
- Volumes: litres, rounded to 0.01 with `math.Round(x*100)/100` in Go before validation and
  storage (research R2). Times: `INTEGER` Unix seconds in SQLite, RFC 3339 in JSON, from
  `deps.Now()`.
- Register each route in `api/internal/httpapi/routes.go` with its `Access` in the same task as
  its handler, and add its entry to the route-walk fixture table (T009) in the same task. Its
  operation is already in `api/openapi.yaml` from T001.
- Artwork uses SVG presentation attributes (`fill`, `stroke`), never inline `style` strings, so
  the nginx CSP stays unchanged.
- Web panels take `onSignedOut` and call it on any 401 (`isUnauthenticated`), like `BeersPanel`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the contract and client types the rest of the feature builds against.

- [X] T001 Merge `specs/004-taps-and-kegs/contracts/taps-kegs.openapi.yaml` into `api/openapi.yaml`: add the `taps` and `kegs` tags; the paths `/api/taps`, `/api/taps/order`, `/api/taps/{id}`, `/api/kegs`, `/api/kegs/{id}`, `/api/kegs/{id}/pours`, `/api/keg-types`; the `Conflict` response; the schemas `Tap`, `TapCreate`, `TapPatch`, `TapOrder`, `KegTypeKey`, `KegType`, `Keg`, `KegCreate`, `KegPatch`, `Pour`; add `on_tap: {type: boolean}` to `Beer.properties` and to `Beer.required` (Beer has `additionalProperties: false`; do not add the fragment's `BeerOnTap` helper schema); add `"409": $ref Conflict` to `PATCH /api/beers/{id}` and `DELETE /api/beers/{id}`; bump `info.version` to `0.4.0`; confirm `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` reports no errors
- [X] T002 [P] In `web/src/api.ts` add `on_tap: boolean` to `Beer`, and add `KegTypeKey`, `KegType` (`type`, `label`, `size_l`, `shape: "corny" | "sanke"`), `KegState = "on_tap" | "waiting" | "kicked"`, `Keg` and `Tap` interfaces matching the contract (nullable fields typed `T | null`); update every test fixture building a `Beer` (`grep -rn "source_ref" web/src`) to include `on_tap: false`
- [X] T003 [P] Create `web/src/units.ts` with `LITRES_PER_GALLON = 3.785411784`, `LITRES_PER_FL_OZ = 0.0295735295625`, `litresToGallons(l)`, `gallonsToLitres(g)` and `flOzToLitres(oz)` (results rounded to 0.01 L), and `formatGallons(l)` returning the gallons to two decimals with " gal"; tests in `web/src/units.test.ts`: for every keg type default size in research R4 (9.46, 11.36, 18.93, 19.56, 29.34, 58.67, 20, 30, 50 L), `gallonsToLitres(Number(litresToGallons(l).toFixed(2)))` is within 0.02 L of `l` (two-decimal gallons are up to 0.005 gal, about 0.019 L, off), and `litresToGallons(gallonsToLitres(g))` is within 0.005 gal of `g` for g in 0.01, 2.5, 5.17, 7.93, 15.5; `formatGallons(18.93) === "5.00 gal"`; `flOzToLitres(16) === 0.47` (SC-008)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the tables, transaction mode, keg type table, shared validation and the
generalized auth walk every story uses.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

### Tests for Foundational

- [X] T004 [P] Migration test `TestMigration0004FromVersion3` in `api/internal/store/migrate_test.go`, modelled on `TestMigration0003FromVersion2`: build a version 3 database holding one admin, one API token and one beer; apply pending migrations; assert version 4; admin, token and beer unchanged; exactly one row in `taps` with `position = 1`, `label = 'Tap 1'`, `enabled = 1`; `kegs` accepts a row with `beer_id`, `size_l`, `remaining_l`, `created_at`, `updated_at` (default `type = 'corny_5_gal'`); a second keg with the same `tap_id` and `kicked_at IS NULL` is refused by index `kegs_one_per_tap`, while two kicked kegs on one tap and two waiting kegs (`tap_id NULL`) are accepted; deleting the beer that a keg names fails (foreign key); deleting a tap sets its kicked kegs' `tap_id` to `NULL`
- [X] T005 [P] Test in `api/internal/store/open_test.go` that a transaction from `db.Begin()` on a database from `Open` holds the write lock at once: while it is open and has only read, a second connection's `BEGIN IMMEDIATE` (with `busy_timeout` 0 via a separate `sql.Open` without `_pragma=busy_timeout`) fails with `SQLITE_BUSY` (research R3)
- [X] T006 [P] Tests in `api/internal/kegs/types_test.go`: `Types` has exactly the nine entries of research R4 in that order, with `type`, `label`, `size_l` and `shape` exactly as in the table (`corny_2_5_gal` 9.46 corny … `euro_50_l` 50.00 sanke); `TypeByKey("corny_5_gal")` found; `TypeByKey("keg")` not found; `DefaultType == "corny_5_gal"`
- [X] T007 [P] Tests in `api/internal/kegs/validate_test.go` for the shared rules: `roundL(18.927) == 18.93`, `roundL(0.004) == 0`; tap label "trimmed, 1 to 50 code points, no control characters" (`""`, `"   "`, 51 code points, `"a\x00"` fail; 50 emoji pass); keg `size_l` "`> 0`, `<= 100`" after rounding (0, 0.004, 100.01, −1 fail; 0.01 and 100 pass); `remaining_l` "`0..size_l`"; pour `amount_l` "`> 0` and `<= 100`" after rounding; unknown keg `type` fails on `type`; several bad fields give several `FieldErrors` keys
- [X] T008 [P] Route-walk auth tests: in `api/internal/httpapi/auth_walk_test.go` replace `TestBeerWritesRefuseWeakCredentials`, `TestBeerReadsNeedCredential` and `TestBeerReadsAnyCredential` (move them out of `beers_test.go`) with `TestWritesRefuseWeakCredentials`, `TestReadsNeedCredential` and `TestReadsAnyCredential`, which walk every `Routes` entry with `AccessWrite` or `AccessRead`. Each looks up a fixture `func(t, deps) (path, body string)` in a table `routeFixtures` keyed by `"<METHOD> <pattern>"` that builds the rows the request needs (for example a beer, a tap, a keg) and returns a concrete path and a valid body; a route with no fixture fails the test, so new routes cannot skip it. For writes: no credential 401, `read` token 403, revoked token 401, cross-site session request (`Sec-Fetch-Site: cross-site`) 403, and `write` token and session each succeed (2xx) on fresh fixtures. For reads: no credential and revoked token 401; `read` token, `write` token and session 200. Seed the table with the six existing beer and srm-color routes so the tests pass before any new route exists

### Implementation for Foundational

- [X] T009 Delete the three moved tests from `api/internal/httpapi/beers_test.go`, keeping any helper `api/internal/httpapi/auth_walk_test.go` now uses (`newCreds`, `insertBeer`); `go test ./internal/httpapi/` passes with at least the beer-route coverage the old tests had (depends on T008)
- [X] T010 Create `api/internal/store/migrations/0004_taps_kegs.sql` exactly as `specs/004-taps-and-kegs/data-model.md`: `taps` (`id INTEGER PRIMARY KEY`, `position INTEGER NOT NULL`, `label TEXT NOT NULL`, `enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1))`, `created_at INTEGER NOT NULL`, `updated_at INTEGER NOT NULL`); `kegs` (`id INTEGER PRIMARY KEY`, `beer_id INTEGER NOT NULL REFERENCES beers(id)`, `type TEXT NOT NULL DEFAULT 'corny_5_gal'`, `size_l REAL NOT NULL`, `remaining_l REAL NOT NULL`, `tap_id INTEGER REFERENCES taps(id) ON DELETE SET NULL`, `tap_label TEXT`, `tapped_at INTEGER`, `kicked_at INTEGER`, `created_at INTEGER NOT NULL`, `updated_at INTEGER NOT NULL`); `CREATE UNIQUE INDEX kegs_one_per_tap ON kegs(tap_id) WHERE kicked_at IS NULL AND tap_id IS NOT NULL`; `CREATE INDEX kegs_beer ON kegs(beer_id)`; `INSERT INTO taps (position, label, enabled, created_at, updated_at) VALUES (1, 'Tap 1', 1, unixepoch(), unixepoch())`; update any test in `api/internal/store/` or `api/cmd/beer-menu-api/` that hard-codes newest version 3 (depends on T004)
- [X] T011 [P] In `api/internal/store/open.go` append `&_txlock=immediate` to `pragmas` with a one-line why comment (research R3) (depends on T005)
- [X] T012 [P] Create `api/internal/kegs/types.go`: `KegType` struct (`Type`, `Label`, `SizeL`, `Shape` with JSON tags `type`, `label`, `size_l`, `shape`), `var Types = []KegType{...}` with the nine rows of research R4 and a comment citing 1 US gal = 3.785411784 L and 1 US barrel = 31 US gal, `DefaultType = "corny_5_gal"`, `TypeByKey(key string) (KegType, bool)` (depends on T006)
- [X] T013 Create `api/internal/kegs/validate.go`: `roundL(x float64) float64`; `normalizeLabel(s string) (string, error-message)` "trimmed, 1 to 50 code points, no control characters"; checks for `size_l` "`> 0`, `<= 100`", `remaining_l` "`0..size_l`", `amount_l` "`> 0` and `<= 100`", all after `roundL`; keg `type` via `TypeByKey`; error values `ErrNotFound`, `ErrTapOccupied`, `ErrTapHasKeg`, `ErrLastTap`, `ErrKegKicked`, each with a doc comment naming its 409 code; returns `beers.FieldErrors` for field problems (depends on T007, T012)
- [X] T014 Add `GET /api/keg-types` (`AccessRead`): handler `listKegTypes` in `api/internal/httpapi/kegs.go` returning `kegs.Types` as a JSON array; register in `api/internal/httpapi/routes.go`; add its `routeFixtures` entry; handler test in `api/internal/httpapi/kegs_test.go` asserting nine entries with `type`, `label`, `size_l`, `shape` (depends on T001, T009, T012)

**Checkpoint**: `go test -race ./...` passes; migration 4 applies; the auth walk covers every route.

---

## Phase 3: User Story 1 - Configure taps (Priority: P1) 🎯 MVP

**Goal**: the owner adds, renames, reorders, enables or disables and removes taps.

**Independent Test**: on a fresh upgrade one tap "Tap 1" exists; add two, rename one, move the
last to the front, disable one; the admin app and a `read` token read show that order, labels
and state (spec US1).

### Tests for User Story 1

- [X] T015 [P] [US1] Store tests in `api/internal/kegs/taps_test.go` on a real migrated SQLite file: `ListTaps` in `position` order with `Keg == nil` when empty; `CreateTap` with no label places the tap last with label `"Tap N"` (N = new position) and `enabled = true`; with a label it trims and keeps it; `UpdateTap` renames and toggles `enabled`, changes only the fields set, keeps the tap's current keg when disabling (insert a keg row with `tap_id` set and `kicked_at NULL`, disable, the keg row is unchanged), sets `updated_at`, and returns `ErrNotFound` for a missing id; an empty patch gives `beers.ErrEmptyPatch`-style error (reuse `beers.ErrEmptyPatch`); `ReorderTaps` with a permutation sets positions 1..N in that order; a list that misses, repeats or invents an id gives a `FieldErrors` key `ids` and changes nothing; `DeleteTap` of the only tap gives `ErrLastTap`; of a tap with a current keg (insert a keg row with `tap_id` set and `kicked_at NULL`) gives `ErrTapHasKeg`; otherwise removes it and positions after it close up (1, 2, 3 → delete 2 → 1, 2); a tap whose only kegs are kicked can be removed and those kegs keep `tap_label`; positions stay 1..N after 20 random add/delete/reorder operations
- [X] T016 [P] [US1] Handler tests in `api/internal/httpapi/taps_test.go`: `GET /api/taps` returns the array (never `null`); `POST /api/taps` 201 with `{}` and with `{"label":"Nitro"}`; `{"label":""}` gives 400 with `fields.label`; unknown field `{"position":3}` gives 400 `invalid_request`; `GET /api/taps/{id}` 200 and 404 for unknown, `0`, `abc`; `PATCH /api/taps/{id}` `{"enabled":false}` 200; `{}` 400; `PUT /api/taps/order` 200 returning the new order, 400 with `fields.ids` for a bad list; `DELETE /api/taps/{id}` 204, 409 `last_tap`, 409 `tap_has_keg`, 404

### Implementation for User Story 1

- [X] T017 [US1] Create `api/internal/kegs/taps.go`: `Tap` struct (`id`, `position`, `label`, `enabled`, `keg *Keg` with JSON tag `keg` marshalling `null` when nil, `created_at`, `updated_at`); `TapInput` (`Label *string`, `Enabled *bool`) and `TapPatch` (`Label`, `Enabled` as `beers.Optional`, both refusing `null`); `ListTaps(db)`, `GetTap(db, id)`, `CreateTap(db, in, now)` using one `INSERT ... SELECT COALESCE(MAX(position),0)+1` and label `'Tap ' || that position` when none given, `UpdateTap(db, id, p, now)`, `ReorderTaps(db, ids, now)` and `DeleteTap(db, id)` in a transaction (`db.Begin`, immediate per T011) that checks last tap and current keg, deletes, then `UPDATE taps SET position = position - 1 WHERE position > ?`. Leave `Keg` nil in this story (a stub `Keg` struct with `ID` is enough to compile; US2 fills it) (depends on T010, T013, T015)
- [X] T018 [US1] Create `api/internal/httpapi/taps.go`: handlers `listTaps`, `getTap`, `createTap`, `updateTap`, `reorderTaps` (body `{"ids":[...]}`), `deleteTap`, each with a doc comment naming method, path, access and status codes; `writeKegError` (in `api/internal/httpapi/kegs.go`) mapping `beers.FieldErrors` → 400 with `fields`, empty patch → 400, `kegs.ErrNotFound` → 404, `ErrTapOccupied` → 409 `tap_occupied`, `ErrTapHasKeg` → 409 `tap_has_keg`, `ErrLastTap` → 409 `last_tap`, `ErrKegKicked` → 409 `keg_kicked`, else 500; `slog.InfoContext` on each change like `beers.go`; register `GET /api/taps`, `GET /api/taps/{id}` with `AccessRead` and `POST /api/taps`, `PATCH /api/taps/{id}`, `PUT /api/taps/order`, `DELETE /api/taps/{id}` with `AccessWrite` in `routes.go`; add each `routeFixtures` entry (delete fixture creates a second tap first) (depends on T009, T016, T017)
- [X] T019 [P] [US1] Component tests in `web/src/components/TapsPanel.test.tsx` (stub `fetch` like `BeersPanel.test.tsx`): lists taps in order with labels and a disabled marker; "Add tap" sends `POST /api/taps` `{}` and reloads; renaming sends `PATCH {"label":...}`; "Move up" on the second tap sends `PUT /api/taps/order` with the first two ids swapped; the first tap has no "Move up" and the last no "Move down"; "Disable" sends `{"enabled":false}`; "Remove" with `window.confirm` false sends nothing, with true sends `DELETE`; a 409 shows the server message; a 401 calls `onSignedOut`
- [X] T020 [US1] Create `web/src/components/TapsPanel.tsx`: lists `GET /api/taps`; per tap: position, label with inline rename (save on submit), enabled toggle, Move up / Move down buttons sending the full reordered id list to `PUT /api/taps/order`, Remove with `window.confirm`; "Add tap" button; errors shown with `errorMessage`; 401 calls `onSignedOut`. Leave a placeholder region per tap for the keg (filled in US2). Add `TapsPanel` to `web/src/pages/AdminPage.tsx` beside `BeersPanel`, and stub `GET /api/taps` in `web/src/pages/AdminPage.test.tsx` (depends on T002, T019)

**Checkpoint**: taps are configurable through the API and admin app; quickstart §2 passes.

---

## Phase 4: User Story 2 - Put a keg on a tap (Priority: P1)

**Goal**: the owner puts a keg of a beer on a tap or on deck, moves it on and off taps, and the
beer shows as a glass while on a tap.

**Independent Test**: put a default keg of a beer on an empty tap; the tap read shows the beer
with a full 18.93 L level and `tapped_at` now; the beer reads `on_tap: true` and draws as a pint
glass; a half barrel on deck draws as a sanke keg (spec US2).

### Tests for User Story 2

- [X] T021 [P] [US2] Store tests in `api/internal/kegs/kegs_test.go`: `CreateKeg` with only `beer_id` gives type `corny_5_gal`, `size_l` 18.93, `remaining_l` = size, `tap_id` nil, `state` `waiting`, `tapped_at` nil; with `type: sanke_half` size 58.67; explicit `size_l` 19.999 stored as 20.00; `remaining_l` defaults to the (explicit) size and above size gives `fields.remaining_l`; with `tap_id` gives `state` `on_tap`, `tapped_at` = now, `tap_label` = the tap's label; a missing tap gives `fields.tap_id`; a missing or archived beer gives `fields.beer_id`; a second keg on an occupied tap gives `ErrTapOccupied`; two goroutines creating a keg on the same empty tap: exactly one succeeds and the other gets `ErrTapOccupied` (unique index violation mapped, not a 500); `UpdateKeg` `tap_id: <empty tap>` on a waiting keg sets `tapped_at` = now if nil, keeps it if set; `tap_id: null` takes it off, snapshots `tap_label`, `state` `waiting`; `tap_id` null on a waiting keg is a no-op; moving onto an occupied tap gives `ErrTapOccupied`; changing `beer_id`, `type`, `size_l` works; a PATCH of `type` alone (`corny_5_gal` → `sanke_half`) keeps `size_l` 18.93; a `size_l` below remaining lowers `remaining_l` to it; a keg can be created on, and moved onto, a disabled tap; `GetKeg`/`ListKegs(state=current)` return `beer_name`, `beer_color` (via `beers.ColorFor`), live `tap_label` while on a tap (rename the tap and re-read); `ListKegs` filters `waiting`, `beer_id`, `tap_id` (`tap_id` does not match a keg taken off that tap); `ListTaps` now embeds each tap's current keg; `DeleteKeg` removes it and its tap reads `keg: null`
- [X] T022 [P] [US2] Beer tests in `api/internal/beers/beers_test.go`: `on_tap` is false with no kegs, false for a waiting keg, true for a current keg on a tap, false after it is taken off; `Update` with `archived: true` on a beer with a current keg (on a tap or waiting) returns `ErrHasCurrentKeg` and changes nothing; with only kicked kegs it archives; `Delete` of a beer with any keg (current or kicked) returns `ErrHasKegs` and keeps the beer; handler tests in `api/internal/httpapi/beers_test.go`: these answer 409 `beer_has_keg` and `beer_has_kegs`, and `GET /api/beers` includes `on_tap`
- [X] T023 [P] [US2] Handler tests in `api/internal/httpapi/kegs_test.go`: `POST /api/kegs` 201 waiting and on a tap; 400 `fields.beer_id`/`fields.tap_id`/`fields.type`; unknown field 400; 409 `tap_occupied`; `GET /api/kegs` default current, `?state=waiting`, `?beer_id=`, `?tap_id=`; `?state=x` and `?beer_id=abc` give 400 `invalid_request`; `GET /api/kegs/{id}` 200/404; `PATCH` move on/off 200 and 409 `tap_occupied`; `DELETE` 204/404; every Keg response has all contract fields including `state`; `GET /api/taps` embeds the keg

### Implementation for User Story 2

- [X] T024 [US2] Create `api/internal/kegs/kegs.go`: `Keg` struct with every contract field (`state` derived: `kicked` if `kicked_at` set, else `on_tap` if `tap_id` set, else `waiting`); one SELECT joining `beers` (name, srm → `beers.ColorFor`) and `LEFT JOIN taps` (`COALESCE(CASE WHEN kicked_at IS NULL THEN taps.label END, kegs.tap_label)` as `tap_label`); `KegInput` (`BeerID`, `Type *string`, `SizeL *float64`, `RemainingL *float64`, `TapID beers.Optional[int64]`) and `KegPatch` (`beer_id`, `type`, `size_l`, `remaining_l`, `tap_id` nullable, `kicked` as `beers.Optional`; `kicked` refuses `null` and `false` with `fields.kicked`); `GetKeg`, `ListKegs(db, filter)` with `state` (`current` default: `kicked_at IS NULL ORDER BY created_at, id`; `waiting`: also `tap_id IS NULL`; `kicked`: `kicked_at IS NOT NULL ORDER BY kicked_at DESC, id DESC`) and optional `beer_id`, `tap_id`, returning a non-nil slice; `CreateKeg(db, in, now)` and `UpdateKeg(db, id, p, now)` in an immediate transaction: validate beer is current (`archived_at IS NULL`) and tap exists, apply defaults from `TypeByKey`, `tapped_at = COALESCE(tapped_at, now)` when going on a tap, `tap_label` = the tap's label when leaving a tap, `remaining_l = MIN(remaining_l, new size)` unless `remaining_l` is also set (then checked against the new size); map a `kegs_one_per_tap` unique violation to `ErrTapOccupied`; any change to a kicked keg returns `ErrKegKicked` (kick handling comes in US4); `DeleteKeg(db, id)`. Fill `Tap.Keg` in `ListTaps`/`GetTap` from current kegs (one query for all taps, no N+1) (depends on T017, T021)
- [X] T025 [US2] In `api/internal/beers/beers.go` add `OnTap bool` (`json:"on_tap"`) computed in `columns` with `EXISTS (SELECT 1 FROM kegs WHERE kegs.beer_id = beers.id AND kegs.tap_id IS NOT NULL AND kegs.kicked_at IS NULL)`; add `ErrHasCurrentKeg` and `ErrHasKegs`; in `Update` refuse archiving when a current keg names the beer (check in the `WHERE` of the same statement, then resolve a 0-row result to `ErrNotFound` or `ErrHasCurrentKeg`); in `Delete` refuse when any keg names it (`ErrHasKegs`; the foreign key is the backstop); in `api/internal/httpapi/beers.go` map them in `writeBeerError` to 409 `beer_has_keg` and `beer_has_kegs` and update the doc comments of `updateBeer` and `deleteBeer` (depends on T010, T022)
- [X] T026 [US2] In `api/internal/httpapi/kegs.go` add handlers `listKegs` (parse `state`, `beer_id`, `tap_id`; bad values 400 `invalid_request`), `getKeg`, `createKeg` (201), `updateKeg`, `deleteKeg` (204), each with a doc comment; register `GET /api/kegs`, `GET /api/kegs/{id}` with `AccessRead` and `POST /api/kegs`, `PATCH /api/kegs/{id}`, `DELETE /api/kegs/{id}` with `AccessWrite`; add `routeFixtures` entries (depends on T018, T023, T024)
- [X] T027 [P] [US2] Tests in `web/src/components/BeerArt.test.tsx`: `kegShape="sanke"` draws `data-vessel="keg"` with `data-keg-shape="sanke"` and its liquid filled with `color`; default and `"corny"` give `data-keg-shape="corny"`; `onTap` still draws the glass whatever `kegShape`; null color draws empty; `aria-label` unchanged ("<name>, keg" / "<name>, pint glass"); no element has a `style` attribute. In `web/src/components/BeersPanel.test.tsx` a beer with `on_tap: true` renders `data-vessel="glass"` and others `keg`; in `web/src/components/BeerForm.test.tsx` the edit preview of an `on_tap` beer is a glass
- [X] T028 [US2] In `web/src/components/BeerArt.tsx` add prop `kegShape?: "corny" | "sanke"` (default `"corny"`) and an original `SankeKeg` drawing (straight-sided steel keg with top and bottom chimes with hand-hold cutouts, two rolling rings, central spear fitting; whole body is the liquid), same `viewBox` scale as `Keg`, presentation attributes only, `data-keg-shape` on the root `svg`; update the component doc comment. In `web/src/components/BeersPanel.tsx` pass `onTap={beer.on_tap}`; in `web/src/components/BeerForm.tsx` pass `onTap={beer?.on_tap ?? false}` to the preview, and update both doc comments that say no caller sets `onTap` (depends on T002, T027)
- [X] T029 [P] [US2] Component tests in `web/src/components/KegForm.test.tsx`: loads `/api/keg-types` and current beers; choosing a type prefills size and remaining in gallons (`Half barrel` → 15.50 gal); submitting a new keg on tap 2 sends `POST /api/kegs` with `beer_id`, `type`, `tap_id: 2` and `size_l`/`remaining_l` only when the user changed them from the prefilled values (converted with `gallonsToLitres`); an edit sends only changed fields, and opening an edit of a 30 L keg and saving without touching size or remaining sends neither `size_l` nor `remaining_l` (SC-008); server `fields` errors show next to their inputs; 401 calls `onSignedOut`
- [X] T030 [US2] Create `web/src/components/KegForm.tsx` per research R8: beer select (current beers from `GET /api/beers`), type select from `GET /api/keg-types` (default `corny_5_gal`), size and remaining in gallons via `web/src/units.ts`, optional tap prop; add and edit modes; field errors from `ApiError.fields` (depends on T002, T003, T029)
- [X] T031 [P] [US2] Extend `web/src/components/TapsPanel.test.tsx`: a tap with a keg shows `BeerArt` as a glass with the keg's `beer_color`, the beer name, the type label and `formatGallons(remaining_l)`; an empty tap shows "Add keg" (opens `KegForm` with that tap) and, when waiting kegs exist, "Put on tap" with a select that sends `PATCH /api/kegs/{id}` `{"tap_id": <tap>}`; "Take off" sends `{"tap_id": null}`; the On deck section lists `GET /api/kegs?state=waiting` with `BeerArt` in the keg's shape (`sanke` for `sanke_half`) and an "Add keg" without a tap
- [X] T032 [US2] Extend `web/src/components/TapsPanel.tsx` per T031: fill the per-tap keg region, add Put on tap / Take off / Edit keg (opens `KegForm`), an "On deck" list of waiting kegs with Add keg; shape from the `/api/keg-types` table loaded once; stub `GET /api/kegs?state=waiting` and `GET /api/keg-types` in `web/src/pages/AdminPage.test.tsx` (depends on T020, T028, T030, T031)

**Checkpoint**: quickstart §3 passes; beers on a tap draw as glasses.

---

## Phase 5: User Story 3 - Update the keg level (Priority: P1)

**Goal**: pours and direct level sets from the admin app and from a `write` token.

**Independent Test**: on a full 18.93 L keg, a 0.5 L pour leaves 18.43 L; setting 10 L gives
10 L; a `read` token is refused; ten concurrent 0.1 L pours all count (spec US3).

### Tests for User Story 3

- [X] T033 [P] [US3] Store tests in `api/internal/kegs/kegs_test.go`: `Pour(db, id, 0.5, now)` on 18.93 gives 18.43 and sets `updated_at`; 0.5 on 0.3 gives 0 and the keg is not kicked; amount 0, −1, 0.004 and 100.01 give `fields.amount_l`; a missing keg gives `ErrNotFound`; a waiting keg can be poured; 50 goroutines pouring 0.1 L each on a 18.93 L keg leave exactly 13.93 L (run under `-race`); `UpdateKeg` `remaining_l: 10` sets 10, `20` on an 18.93 keg and `-1` give `fields.remaining_l` and change nothing
- [X] T034 [P] [US3] Handler tests in `api/internal/httpapi/kegs_test.go`: `POST /api/kegs/{id}/pours` `{"amount_l":0.5}` 200 with the Keg; `{}` and `{"amount_l":0}` 400 `fields.amount_l`; unknown field 400; 404 unknown keg; `PATCH {"remaining_l":10}` 200

### Implementation for User Story 3

- [X] T035 [US3] In `api/internal/kegs/kegs.go` add `Pour(db, id, amount, now)`: validate and round `amount`, then one statement `UPDATE kegs SET remaining_l = MAX(0, ROUND(remaining_l - ?, 2)), updated_at = ? WHERE id = ? AND kicked_at IS NULL RETURNING ...`, resolving 0 rows to `ErrNotFound` or `ErrKegKicked`; make sure `UpdateKeg` sets `remaining_l` per research R5 (depends on T024, T033)
- [X] T036 [US3] In `api/internal/httpapi/kegs.go` add `pourKeg` handling `POST /api/kegs/{id}/pours` (body `{"amount_l": number}`, required), doc comment noting it is the stable contract for Home Assistant and keg scales; register with `AccessWrite`; add its `routeFixtures` entry (depends on T026, T034, T035)
- [X] T037 [P] [US3] Extend `web/src/components/TapsPanel.test.tsx`: "Pour" opens an input prefilled with 16 (fl oz) and submitting sends `POST /api/kegs/{id}/pours` `{"amount_l": 0.47}` — two clicks from the panel (SC-002) — and shows the new level; "Set level" takes gallons and sends `PATCH {"remaining_l": gallonsToLitres(x)}`; a level bar's width follows `remaining_l / size_l`
- [X] T038 [US3] Extend `web/src/components/TapsPanel.tsx` per T037: Pour (fl oz, default 16) and Set level (gallons) on tap kegs and on-deck kegs, and a level bar drawn with SVG presentation attributes or a `<meter>`/`<progress>` element, no inline `style` (depends on T032, T037)

**Checkpoint**: quickstart §4 passes.

---

## Phase 6: User Story 4 - Kick a keg and see keg history (Priority: P2)

**Goal**: kick frees the tap and moves the keg to history; history lists kicked kegs.

**Independent Test**: kick a keg; its tap is empty; history shows it with tapped and kicked
times; a new keg goes on the same tap (spec US4).

### Tests for User Story 4

- [X] T039 [P] [US4] Store tests in `api/internal/kegs/kegs_test.go`: `UpdateKeg` `{"kicked": true}` on a keg on a tap sets `kicked_at` = now, keeps `remaining_l` and `tap_id`, snapshots `tap_label`, `state` `kicked`, and the tap reads `keg: nil` and accepts a new keg; on a waiting keg also works (history shows no tap and no `tapped_at` if never tapped); a repeat kick returns the keg unchanged (`kicked_at` kept); on a kicked keg any other field, or `kicked` together with another field, gives `ErrKegKicked`; `Pour` on a kicked keg gives `ErrKegKicked`; `ListKegs(state=kicked)` orders newest kicked first; renaming or deleting the tap afterwards keeps the history `tap_label`; `ListKegs(state=kicked, tap_id=T)` returns a keg kicked on tap T, and no longer returns it after tap T is removed; `DeleteKeg` of a kicked keg works
- [X] T040 [P] [US4] Handler tests in `api/internal/httpapi/kegs_test.go`: `PATCH {"kicked":true}` 200; repeat 200; `PATCH {"remaining_l":1}` and `POST .../pours` on a kicked keg 409 `keg_kicked`; `{"kicked":false}` 400 `fields.kicked`; `GET /api/kegs?state=kicked` order
- [X] T041 [P] [US4] Component tests: in `web/src/components/TapsPanel.test.tsx` "Kick" with `window.confirm` false sends nothing, with true sends `PATCH {"kicked":true}` and reloads; in `web/src/components/KegHistory.test.tsx` the list from `GET /api/kegs?state=kicked` shows beer name, tap label, type label, size in gallons, tapped and kicked dates (an em dash when `tapped_at` is null); Delete with confirm false sends nothing, true sends `DELETE /api/kegs/{id}`; 401 calls `onSignedOut`

### Implementation for User Story 4

- [X] T042 [US4] In `api/internal/kegs/kegs.go` handle `kicked: true` in `UpdateKeg`: set `kicked_at = now` and `tap_label` from the current tap's label, keep `tap_id`; repeat kick on a kicked keg is a no-op returning the keg; anything else on a kicked keg returns `ErrKegKicked` (depends on T024, T039)
- [X] T043 [US4] Make `api/internal/httpapi/kegs_test.go` T040 pass (handlers from T026/T036 already route; adjust `writeKegError` only if needed) (depends on T040, T042)
- [X] T044 [US4] Add Kick (with `window.confirm`) to `web/src/components/TapsPanel.tsx`, create `web/src/components/KegHistory.tsx` per T041, and add `KegHistory` to `web/src/pages/AdminPage.tsx` with its `GET /api/kegs?state=kicked` stub in `web/src/pages/AdminPage.test.tsx` (depends on T038, T041)

**Checkpoint**: quickstart §5 passes; all four stories work.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T045 [P] Confirm `api/openapi.yaml` matches the implementation (status codes, 409 codes, query parameters, `on_tap`); copy any change back into `specs/004-taps-and-kegs/contracts/taps-kegs.openapi.yaml`; redocly lint reports no errors
- [X] T046 [P] Update `README.md`: a short "Taps and kegs" section listing the endpoints and their access, that volumes are litres in the API, that `POST /api/kegs/{id}/pours` is the endpoint for Home Assistant and keg scales with a `write` token, and the 409 codes
- [X] T047 [P] Scale test in `api/internal/kegs/kegs_test.go`: 20 taps and 1,000 kicked kegs; `ListTaps` and `ListKegs(state=kicked)` each under 1 s (SC-005)
- [X] T048 Run `go vet ./...`, `go test -race ./...`, `npm run lint`, `npm test`, `npm run build`, `govulncheck ./...` (in `api/`) and `gitleaks detect --source . --redact`; all pass
- [ ] T049 Run quickstart §1 to §7 against images built with `--pull never --build` from a copy of a feature 3 data folder, and record results, including SC-001 and SC-002 timings and SC-005, in the PR. In the PR review, confirm the sanke drawing in `web/src/components/BeerArt.tsx` is original work

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: T014 depends on T001. Blocks all stories.
- **US1 (Phase 3)**: after Foundational.
- **US2 (Phase 4)**: after US1 (kegs go on taps; `TapsPanel` exists).
- **US3 (Phase 5)**: after US2 (pours need kegs).
- **US4 (Phase 6)**: after US2; its web task T044 after US3's T038 (same file).
- **Polish (Phase 7)**: after all stories.

### User Story Dependencies

```text
Setup ─> Foundational ─> US1 ─> US2 ─┬─> US3 ─┐
                                     └─> US4 ─┴─> Polish   (US4 web after US3 web)
```

### Within Each User Story

- Tests first, and see them fail.
- `internal/kegs` and `internal/beers` before `httpapi` handlers before web components.
- Register each route with its `Access` and its `routeFixtures` entry in the same task as its
  handler.

### Parallel Opportunities

- T001, T002, T003 in Setup.
- T004 to T008 (Foundational tests) together; then T010, T011, T012 together; T013 after T012.
- Per story, all `[P]` test tasks together (Go and web tests touch different files).
- In US2, the web track (T027 to T032) beside the API track (T024 to T026) once T021 to T023
  are written.
- US3 API (T033 to T036) beside US4 API (T039 to T043) only if one person owns
  `api/internal/kegs/kegs.go`; they edit the same file, so run them one after another by
  default.

---

## Parallel Example: User Story 2

```bash
# Tests together:
Task: "T021 store tests in api/internal/kegs/kegs_test.go"
Task: "T022 beer tests in api/internal/beers/beers_test.go"
Task: "T023 handler tests in api/internal/httpapi/kegs_test.go"
Task: "T027 BeerArt/BeersPanel/BeerForm tests"
Task: "T029 KegForm tests"

# Then API and web tracks side by side:
Task: "T024 kegs.go" -> "T026 kegs handlers"
Task: "T028 BeerArt sanke + onTap wiring" -> "T030 KegForm" -> "T032 TapsPanel keg region"
```

---

## Implementation Strategy

### MVP First

1. Setup and Foundational.
2. US1 (taps), US2 (kegs on taps), US3 (levels): together they are the tap list feature 5
   needs. Stop and validate quickstart §1 to §4.
3. US4 (kick and history) completes the brief's "done when".

### Incremental Delivery

Each story ends at a checkpoint where `go test -race ./...` and `npm test` pass and the matching
quickstart section works. Run every story through the dev-pipeline (implementation, quality,
security, verification) as one feature branch and one PR.
