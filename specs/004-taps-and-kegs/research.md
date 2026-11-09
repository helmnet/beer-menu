# Research: Taps and Kegs

Phase 0 decisions for [plan.md](./plan.md). Each entry gives the decision, why, and what was
rejected. Requirement IDs refer to [spec.md](./spec.md).

## R1. Endpoints

- **Decision**: twelve tap and keg operations plus one keg-type table read (13 routes).

  | Method | Path | x-access | Purpose |
  | --- | --- | --- | --- |
  | GET | `/api/taps` | read | Taps in position order, each with its current keg or `null` (FR-041) |
  | POST | `/api/taps` | write | Add a tap at the end (FR-003, FR-005) |
  | GET | `/api/taps/{id}` | read | One tap with its keg |
  | PATCH | `/api/taps/{id}` | write | Rename, enable or disable |
  | DELETE | `/api/taps/{id}` | write | Remove; 409 if it holds a keg or is the last tap (FR-006) |
  | PUT | `/api/taps/order` | write | Full new order `{"ids": [...]}` (FR-004) |
  | GET | `/api/kegs` | read | Kegs by `state` (`current` default, `waiting`, `kicked`), `beer_id`, `tap_id` (FR-042) |
  | POST | `/api/kegs` | write | Create a keg, on a tap or waiting (FR-015) |
  | GET | `/api/kegs/{id}` | read | One keg |
  | PATCH | `/api/kegs/{id}` | write | Correct beer/type/size, set `remaining_l`, move (`tap_id`), kick (`kicked: true`) |
  | DELETE | `/api/kegs/{id}` | write | Delete a keg record (FR-021) |
  | POST | `/api/kegs/{id}/pours` | write | Subtract `{"amount_l": x}` (FR-016) |
  | GET | `/api/keg-types` | read | The fixed keg type table (R4) |

- **Kick and move are PATCH fields**, like feature 3's archive: `kicked: true` kicks;
  `tap_id: <id>` puts the keg on a tap; `tap_id: null` takes it off. `kicked: false` is refused
  (a kicked keg never comes back, FR-019).
- **Pours get their own POST**: a pour is relative, so two clients pouring at once both count
  (FR-016, spec scenario 3.4). PATCH `remaining_l` is the absolute "set level" (FR-017). Home
  Assistant and scales call `POST /api/kegs/{id}/pours` with a `write` token; this is the
  backward-compatible contract principle I protects.
- **`PUT /api/taps/order`** and `PATCH /api/taps/{id}` differ by method, and Go's ServeMux
  prefers the literal segment for the same method, so the patterns do not clash.
- **Rationale**: matches feature 3 (PATCH for state changes, merge-patch, reads `read`, writes
  `write`). One extra route for pours is the only way to make concurrent relative updates safe
  without client-side read-modify-write.
- **Alternatives**: `POST /api/kegs/{id}/kick` and `/move` routes (more routes, no gain);
  `POST /api/taps/{id}/keg` to tap a keg (two ways to create a keg); pour as PATCH
  `{"pour_l": x}` (mixes relative and absolute in one body; ambiguous with `remaining_l`).

## R2. Storage

- **Decision**: migration `0004_taps_kegs.sql` adds `taps` and `kegs` and inserts one tap
  `(position 1, "Tap 1", enabled)` (FR-002). See [data-model.md](./data-model.md).
  - `kegs.tap_id` stays set on kicked kegs (history filter by tap, FR-042) and is cleared when a
    keg is taken off a tap. So `?tap_id=` matches kegs on that tap and kegs kicked on it; a keg
    taken off a tap and kicked later, or whose tap was removed, has `tap_id` NULL and matches
    only by its `tap_label`, which the filter does not use. One current keg per tap is a partial unique index
    `ON kegs(tap_id) WHERE kicked_at IS NULL AND tap_id IS NOT NULL` (FR-014), so the database
    refuses a second keg even under concurrent requests.
  - `kegs.tap_id REFERENCES taps(id) ON DELETE SET NULL`: removing a tap leaves history rows.
    The tap delete code refuses first when a current keg is on it (FR-006).
  - `kegs.beer_id REFERENCES beers(id)` with the default `NO ACTION`: with `foreign_keys(ON)`
    (already in `store/open.go`) the database itself refuses deleting a beer with kegs, as a
    backstop to the 409 check (FR-031).
  - `kegs.tap_label` is a snapshot written when a keg leaves a tap (kick or take off). Responses
    show the live tap label while the keg is on a tap, else the snapshot (FR-043).
  - `tapped_at` is set the first time a keg goes on a tap: `COALESCE(tapped_at, now)`.
- **Volumes**: `REAL` litres, rounded to 0.01 in Go on input (`math.Round(x*100)/100`) and in SQL
  for pours (`MAX(0, ROUND(remaining_l - ?, 2))`). Rounding instead of refusing extra decimals
  lets clients send converted gallons as-is.
- **Positions**: `INTEGER NOT NULL`, no unique constraint, 1..N kept by code. Add uses one
  `INSERT ... SELECT COALESCE(MAX(position),0)+1`; delete and reorder run in a transaction. A
  unique constraint would need a two-phase renumber for each shift, with no gain over the
  transaction.
- **Rationale**: the database enforces the rules that concurrency can break (one keg per tap,
  beer delete with kegs); code enforces the rest with clear messages.

## R3. Transactions take the write lock up front

- **Decision**: add `&_txlock=immediate` to the DSN in `store/open.go`, so every
  `db.Begin` is `BEGIN IMMEDIATE`. `modernc.org/sqlite` supports this parameter
  (`sqlite.go`, `_txlock`).
- **Rationale**: tap delete and reorder read then write. A deferred transaction that reads then
  writes can fail with `SQLITE_BUSY` without waiting when another writer is active; an
  immediate one waits on `busy_timeout(5000)` like single statements do. Existing transactions
  (migrations, auth) all write, so they behave the same.
- **Alternatives**: `db.SetMaxOpenConns(1)` (serializes reads too); a Go mutex around tap writes
  (does not cover other processes and is easy to miss on a new path).

## R4. Keg types

- **Decision**: a fixed Go table in `internal/kegs`, served by `GET /api/keg-types` as
  `[{"type","label","size_l","shape"}]`, in this order:

  | type | label | size_l | shape |
  | --- | --- | --- | --- |
  | `corny_2_5_gal` | Corny 2.5 gal | 9.46 | corny |
  | `corny_3_gal` | Corny 3 gal | 11.36 | corny |
  | `corny_5_gal` | Corny 5 gal | 18.93 | corny |
  | `sanke_sixth` | Sixth barrel | 19.56 | sanke |
  | `sanke_quarter` | Quarter barrel | 29.34 | sanke |
  | `sanke_half` | Half barrel | 58.67 | sanke |
  | `euro_20_l` | Euro 20 L | 20.00 | sanke |
  | `euro_30_l` | Euro 30 L | 30.00 | sanke |
  | `euro_50_l` | Euro 50 L | 50.00 | sanke |

  Sizes from 1 US gal = 3.785411784 L and 1 US barrel = 31 US gal, rounded to 0.01 L. The
  column has no `CHECK`, so adding a type later needs no table rebuild; Go validates.
- **Rationale**: one definition for every client, as feature 3 did for SRM colors (feature 3
  R6). The admin form needs labels, default sizes and shapes before a keg exists.
- **Alternatives**: a TypeScript copy (two tables drift); a `keg_types` table (editable types
  are not asked for).

## R5. Validation and errors

- **Decision**:
  - Tap `label`: trimmed, 1 to 50 code points, no control characters. Omitted on create →
    `"Tap N"`.
  - Keg `size_l`: `> 0` and `<= 100`. `remaining_l`: `0` to `size_l`, default `size_l`.
    `amount_l` on a pour: `> 0` and `<= 100`. All rounded to 0.01 first; a value that rounds
    to 0 for size or pour is refused.
  - `beer_id` must name a current (not archived) beer → else field error on `beer_id`.
  - `tap_id` on create or PATCH must name an existing tap → else field error on `tap_id`.
  - Field errors: `400 invalid_request` with `fields`, as feature 3. Unknown fields refused.
  - Conflicts, `409` with distinct codes so clients can branch:
    `tap_occupied` (keg onto a tap that has one), `tap_has_keg` (remove a tap with a keg),
    `last_tap`, `keg_kicked` (any change but delete or a repeat kick on a kicked keg),
    `beer_has_keg` (archive with a current keg), `beer_has_kegs` (delete with any keg).
  - Reorder: `ids` must be a permutation of all tap ids → else `400` with field error on `ids`.
- **PATCH `type` alone** changes only the type; `size_l` and `remaining_l` stay as stored. A
  client that wants the new type's default size sends `size_l` too (the admin form does, since
  choosing a type pre-fills size). Defaults from the type apply only on create.
- **PATCH size below remaining** lowers remaining to the new size in the same statement
  (`remaining_l = MIN(remaining_l, new size)`), unless the same PATCH also sets `remaining_l`,
  which is then checked against the new size (FR-020).
- **Rationale**: follows feature 3 error shapes; the 409 codes are part of the contract later
  clients rely on.

## R6. Beers: on-tap flag and guards (FR-030 to FR-033)

- **Decision**:
  - Every beer read gains `on_tap` (boolean), computed in the beers query with
    `EXISTS (SELECT 1 FROM kegs WHERE beer_id = beers.id AND tap_id IS NOT NULL AND kicked_at IS NULL)`.
  - `beers.Update` with `archived: true` refuses with `ErrHasCurrentKeg` when the beer has a
    current keg (on a tap or waiting); the check and update share one statement's `WHERE`, and a
    0-row result is resolved to not found or conflict.
  - `beers.Delete` refuses with `ErrHasKegs` when any keg names the beer (the FK is the
    backstop).
- **Rationale**: `on_tap` in the read lets the admin library and any later client pick glass or
  keg from one rule with no extra request (FR-032). The beers package keeps all beer SQL.

## R7. Code layout

- **Decision**: new package `api/internal/kegs` (no HTTP) holding taps, kegs, keg types,
  validation and SQL: `taps.go`, `kegs.go`, `types.go`, `validate.go`. It reuses
  `beers.Optional[T]` and `beers.FieldErrors`. Handlers in `httpapi/taps.go` and
  `httpapi/kegs.go`, one `writeKegError` mapping errors to statuses.
- **Rationale**: same shape as `internal/beers`; taps and kegs share invariants (one keg per
  tap, tap delete guard), so one package keeps them in one place.
- **Alternatives**: separate `taps` and `kegs` packages (cross-imports for the guards).

## R8. Admin app

- **Decision**: no new web dependencies.
  - `TapsPanel`: taps in order; per tap label (inline rename), enabled toggle, up/down buttons
    (sending `PUT /api/taps/order`), remove (confirm), and its keg: `BeerArt` with `onTap` and
    the keg's shape, beer name, type label, level in gallons with a bar. Keg actions: "Pour"
    (amount in fl oz, prefilled 16, so two clicks, SC-002), "Set level" (gallons), "Kick"
    (confirm), "Take off", "Edit". Empty taps: "Add keg" and, when waiting kegs exist, "Put on
    tap". An "On deck" list shows waiting kegs (FR-063a), and "Add keg" without a tap.
  - `KegForm`: beer select (current beers), type select from `/api/keg-types` (prefills size
    and remaining), size and remaining in gallons, optional tap. Sends only changed fields on
    edit.
  - `KegHistory`: kicked kegs newest first with beer, tap label, type, size, tapped and kicked
    dates, and delete (confirm).
  - `units.ts`: `litresToGallons`, `gallonsToLitres`, `flOzToLitres`, formatting gallons to two
    decimals (FR-065). Inputs convert on submit; unchanged inputs are not sent, so stored values
    never drift.
  - `BeerArt` gains `kegShape?: "corny" | "sanke"` (default `"corny"`) and a new original
    `SankeKeg` drawing; `BeersPanel` and `BeerForm` pass `onTap={beer.on_tap}` (FR-033, FR-034).
  - Panels take `onSignedOut`. `AdminPage` adds `TapsPanel` and `KegHistory`; its test stubs
    gain the new GETs.
- **Rationale**: same panel pattern as features 2 and 3.
- **Alternatives**: drag-and-drop reorder (a dependency or much code for a handful of taps).

## R9. Tests

- **Decision**:
  - Go `kegs` package on real SQLite: tap add/rename/disable/reorder/remove rules, positions
    stay 1..N, last tap and occupied tap refused; keg create on a tap and waiting, defaults
    from type, rounding, remaining bounds, move on/off with `tapped_at` and `tap_label` rules,
    kick idempotence, kicked keg refuses edits, pour floor at 0, concurrent pours (goroutines,
    sum of amounts), second keg on a tap refused even when racing, delete.
  - Beers: `on_tap` true only for a current keg on a tap; archive with a current keg 409;
    delete with any keg 409.
  - Handlers: every route's success and error codes. Auth per route by walking `Routes`:
    the existing `TestBeerWritesRefuseWeakCredentials`/`TestBeerReadsNeedCredential` pattern
    generalized to every `AccessWrite`/`AccessRead` route, so new routes are covered without a
    list to maintain (no credential 401, `read` token 403 on writes, revoked 401, cross-site
    403, `write` token and session succeed).
  - Migration 0004 applied to a version 3 database holding an admin, a token and a beer: all
    survive, one tap exists.
  - Existing `x-access` contract test covers the new routes.
  - Vitest: `units.ts` round-trips for every keg type size (SC-008); `BeerArt` shape and
    `onTap`; `TapsPanel` (pour sends litres, kick confirm cancel sends nothing, reorder body,
    401 calls `onSignedOut`); `KegForm` (type prefills, only changed fields on edit);
    `KegHistory`; `BeersPanel` passes `on_tap`.
