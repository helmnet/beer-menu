---

description: "Task list for feature 6: Brewfather import"
---

# Tasks: Brewfather Import

**Input**: Design documents from `specs/006-brewfather-import/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md),
[contracts/brewfather.openapi.yaml](./contracts/brewfather.openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks of the database; Brewfather is replaced by a fake
`importer.Provider` or an `httptest.NewTLSServer`, never the real service; every new read and
write route joins the route walk; the migration is tested from v4; React components with logic
get Vitest tests. Write each test task before its implementation task and see it fail first.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US4)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`). New packages
  `api/internal/secrets`, `api/internal/brewfather`, `api/internal/importer`; handlers in
  `api/internal/httpapi/brewfather.go`.
- Web: `web/` (Vite + React + TypeScript). New panel `web/src/components/BrewfatherPanel.tsx`.
- Test helpers: `api/internal/beers/beers_test.go` (`openDB`, `insert`, `insertKeg`),
  `api/internal/beers/validate_test.go` (`ptr`, `fieldKeys`); `api/internal/httpapi/helpers_test.go`
  (`newTestDeps`, `testClock`, `do`, `insertToken`, `wantError`, `sessionCookie`);
  `api/internal/httpapi/auth_walk_test.go` (`routeFixtures`, `walkedRoutes`);
  `api/internal/store/migrate_test.go` (`TestMigration0004FromVersion3` pattern);
  `web/src/testApi.ts` (`stubApi`, `beer()`, `tap()`, `keg()`).

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No new Go or web dependencies (plan). Crypto is stdlib `crypto/aes`, `crypto/cipher`,
  `crypto/rand`.
- **Secret hygiene (FR-005, research R14)**: never log, return, wrap into an error, or put in a
  test fixture the API key, the sealed value, the Basic auth header, the encryption key, or the
  Brewfather user ID. Never log or return a Brewfather response body. Tests use obviously fake
  keys such as `"fake-key-" + rand.Text()`; never a real Brewfather key anywhere in the repo.
- Brewfather field names (`measuredAbv`, `Brewing`, `_id`, ...) appear only in
  `api/internal/brewfather`. `importer`, `beers`, `httpapi` and `web` see only `importer.Batch`
  and importer error types (constitution II, FR-028).
- Archived Brewfather batches are never listed in bulk and never imported (spec clarification).
- No inline `style` anywhere in the web app (nginx CSP has no `style-src`). The admin app stays
  unstyled (FR-035).
- Times are INTEGER Unix seconds in SQLite and RFC 3339 in JSON. Brew dates are `YYYY-MM-DD`
  in UTC (research R12).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the API contract, the container's CA bundle and shared web types.

- [ ] T001 Merge `specs/006-brewfather-import/contracts/brewfather.openapi.yaml` into `api/openapi.yaml`: add the `brewfather` tag, the 8 operations (with path parameters inside each operation, as existing paths do), the schemas `SyncInterval`, `BrewfatherCredentials`, `BrewfatherStatus`, `LastSync`, `BatchSummary`, `ImportResult`, `BeerSync` and the responses `BrewfatherNotConfigured`, `BrewfatherNotConnected`, `BrewfatherConflict`, `BrewfatherRefused`, `BrewfatherRateLimited`, `BrewfatherUnreachable`; add `sync` (`oneOf: [BeerSync, null]`) to `Beer.properties` and to `Beer.required`; bump `info.version` to `0.6.0`. Run `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml` and fix errors (the `x-access` contract test will fail until T030 adds the routes; that is expected)
- [ ] T002 [P] In `api/Dockerfile`, add `RUN apk add --no-cache ca-certificates` to the build stage and `COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt` to the `scratch` stage, with a one-line why comment ("scratch has no CA roots; needed for HTTPS to Brewfather", research R12)
- [ ] T003 [P] In `web/src/api.ts`, add types `SyncInterval = "off" | "15m" | "1h" | "6h" | "24h"`, `BrewfatherStatus`, `LastSync`, `BatchSummary`, `ImportResult` (outcome `"imported" | "already_imported" | "archived" | "not_found" | "rate_limited" | "failed"`) and `BeerSync`, matching the contract field for field; add `sync: BeerSync | null` to `Beer`. In `web/src/testApi.ts`, give `beer()` a default `sync: null` and add fixtures `brewfatherStatus(overrides)` (default `configured: true, connected: false, reconnect_needed: false, interval: "off", sync_running: false, retry_at: null, last_sync: null`), `batch(overrides)` and `importResult(overrides)`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: storage, encryption, the provider boundary and its Brewfather implementation, the
beer-side sync helpers, and the importer service skeleton. Every story needs these.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete.

### Tests for Foundational

- [ ] T004 [P] Migration test `TestMigration0005FromVersion4` in `api/internal/store/migrate_test.go`, following `TestMigration0004FromVersion3`: build a v4 database with two manual beers (one with a kicked keg) and a tap, apply 0005, and assert: existing beers keep every value, have `synced_at` NULL and `sync_not_found`, `sync_archive_pending`, `sync_batch_archived` all 0; tables `settings`, `provider_connections`, `sync_runs`, `import_dismissed` exist with the columns in data-model.md; `provider_connections.provider` refuses anything but `'brewfather'`; two beers with the same `(source, source_ref)` (non-null `source_ref`) are refused by `beers_source_ref`, while two with NULL `source_ref` are allowed
- [ ] T005 [P] Tests in `api/internal/secrets/secrets_test.go`: `ParseKey` accepts standard base64 of exactly 32 bytes with surrounding whitespace, refuses empty, non-base64, 16, 31 and 33 bytes, and its error text never contains the input; `Seal` then `Open` round-trips with the same purpose; two `Seal`s of the same plaintext differ (random nonce) and start with `v1:`; `Open` returns `ErrCannotOpen` for a different key, a different purpose, a flipped ciphertext byte, a truncated value, a `v2:` prefix and non-base64; the sealed string never contains the plaintext
- [ ] T006 [P] Tests in `api/internal/beers/sanitize_test.go` for `Sanitize(Input) (Values, []string)` (research R6): trims; strips control characters from name, style, brewer (description keeps `\n` and `\t`); shortens name, style and brewer to 100 runes and description to 2000 runes (multi-byte input, cut on rune boundaries); rounds ABV and SRM to one decimal and IBU to a whole number; empties ABV above 25, SRM above 100, negative values, an IBU outside the existing `Normalize` range and an invalid `brew_date`; an empty or all-control name becomes `"Brewfather batch"`; the returned slice names exactly the changed JSON fields, sorted; a fuzz-style table of ugly inputs whose `Values`, turned back into an `Input`, always pass `Normalize` without error
- [ ] T007 [P] Tests in `api/internal/beers/beers_test.go`: `Beer.Sync` is nil for a manual beer and `{SyncedAt, NotFound, ArchivePending}` for one created with `CreateImported`; `CreateImported(db, source, ref, Values, batchArchived bool, now)` stores `source`, `source_ref`, `synced_at`, brewer NULL, and returns `ErrAlreadyImported` (no new row) for a second create with the same source and ref; `Delete` of an imported beer with no kegs inserts `(source, ref)` into `import_dismissed`, while `Delete` of a manual beer, or of an imported beer refused with `ErrHasKegs`, inserts nothing; `ClearDismissal(db, source, ref)` removes the row
- [ ] T008 [P] Tests in `api/internal/beers/beers_test.go` for `ApplySync(db, id, Values, batchArchived bool, now) (Beer, error)` (research R7, FR-017, FR-017a): overwrites only `abv`, `ibu`, `srm`, `brew_date`, `status`, sets `synced_at`, clears `sync_not_found`; name, style, description and brewer are untouched even when `Values` differs; batch archived with `sync_batch_archived = 0` and no current keg archives the beer and sets `sync_batch_archived = 1`, `sync_archive_pending = 0`; with a current keg (on a tap or waiting) leaves it unarchived, sets `sync_archive_pending = 1` and keeps `sync_batch_archived = 0`; after a kicked keg the next call archives it; with `sync_batch_archived = 1` and the beer unarchived by the owner, it is not archived again; batch not archived resets both flags and never unarchives an archived beer; `MarkNotFound(db, id, now)` sets `sync_not_found = 1` and changes nothing else; an unknown id gives `ErrNotFound`
- [ ] T009 [P] Tests in `api/internal/brewfather/client_test.go` against `httptest.NewTLSServer` with the server's client (research R1, R4): every request carries `Authorization: Basic base64(user:key)` and goes to `/v2/batches...`; `ListBatches` asks once per status `Planning`, `Brewing`, `Fermenting`, `Conditioning`, `Completed` with `limit=50`, never `Archived`, follows `start_after=<last _id>` until a short page, and requests the `include` fields it maps; `GetBatch` hits `/v2/batches/{id}` with the id path-escaped; mapping per data-model.md "Brewfather to common batch" (name chain `name` → `recipe.name` → `Batch <batchNo>` → `Brewfather batch`; measured ABV else `recipe.abv`; `tasteNotes` else `batchNotes`; `brewDate` ms → UTC `YYYY-MM-DD`, 0 or absent → nil; statuses Planning→planned, Brewing and Fermenting→fermenting, Conditioning→conditioning, Completed→ready, Archived→ready with `Archived: true`, unknown→planned); errors: 401 → `importer.ErrCredentialsRefused`, 403 → `importer.ErrNoBatchAccess`, 404 on `GetBatch` → `importer.ErrBatchNotFound`, 429 with `Retry-After: 120` and with an HTTP-date → `*importer.RateLimitedError` with the right `Until`, absent or junk header → 60 s, a value over an hour → capped at 1 h; 500, a handler that sleeps past the client timeout (inject a short timeout), malformed JSON and a body over 4 MiB → `importer.ErrUnreachable`; a 302 is not followed; a cancelled or expired context stops an in-flight request with `importer.ErrUnreachable`; `ListBatches` has at most 5 requests in flight and still returns every page of every status; `New` with an `http://` base URL refuses; no error string contains the key, the Basic header or the response body
- [ ] T010 [P] Tests in `api/internal/importer/store_test.go` on real SQLite (open with the store migrations): settings get/set with `brewfather.sync_interval` (missing → `"off"`, invalid value refused); save, load and delete the connection with a sealed key; a stored value sealed under another key loads as `reconnect needed` without error text containing it; sync run start/finish upsert and read back; a run row with `finished_at` NULL at service start reads as failed with message "Sync was interrupted"

### Implementation for Foundational

- [ ] T011 Write `api/internal/store/migrations/0005_brewfather.sql` exactly per data-model.md: `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at INTEGER NOT NULL)`; `provider_connections(provider TEXT PRIMARY KEY CHECK (provider IN ('brewfather')), user_id TEXT NOT NULL, api_key_sealed TEXT NOT NULL, connected_at INTEGER NOT NULL)`; `sync_runs(provider TEXT PRIMARY KEY, started_at INTEGER NOT NULL, finished_at INTEGER, ok INTEGER, message TEXT, updated INTEGER NOT NULL DEFAULT 0, imported INTEGER NOT NULL DEFAULT 0, not_found INTEGER NOT NULL DEFAULT 0)`; `import_dismissed(source TEXT NOT NULL, ref TEXT NOT NULL, dismissed_at INTEGER NOT NULL, PRIMARY KEY (source, ref))`; `ALTER TABLE beers ADD COLUMN synced_at INTEGER`, `sync_not_found INTEGER NOT NULL DEFAULT 0`, `sync_archive_pending INTEGER NOT NULL DEFAULT 0`, `sync_batch_archived INTEGER NOT NULL DEFAULT 0`; `CREATE UNIQUE INDEX beers_source_ref ON beers(source, source_ref) WHERE source_ref IS NOT NULL` (makes T004 pass)
- [ ] T012 [P] Implement `api/internal/secrets/secrets.go`: `ParseKey(string) ([]byte, error)`, `Box`, `NewBox(key []byte) (*Box, error)`, `(*Box).Seal(plain, purpose string) (string, error)` producing `"v1:" + base64(nonce ‖ ciphertext)` with AES-256-GCM, 12-byte random nonce and `purpose` as additional data, `(*Box).Open(sealed, purpose string) (string, error)`, `ErrCannotOpen`; package doc says the key never touches the database (makes T005 pass)
- [ ] T013 [P] Implement `api/internal/beers/sanitize.go` with `Sanitize`, reusing the limits and checks in `validate.go` rather than copying numbers (makes T006 pass)
- [ ] T014 Extend `api/internal/beers/beers.go`: `BeerSync` type and `Beer.Sync *BeerSync` (`json:"sync"`), set from the new columns only when `source <> 'manual'`; add the four sync columns to `columns` and `scan`; `ErrAlreadyImported`; `CreateImported`; `ApplySync` and `MarkNotFound` in one transaction each, using the same "no current keg" condition as `Update`'s archive (`kegs.kicked_at IS NULL`); `ClearDismissal`; change `Delete` to a transaction that first `INSERT OR IGNORE INTO import_dismissed SELECT source, source_ref, ? FROM beers WHERE id = ? AND source_ref IS NOT NULL AND NOT EXISTS (SELECT 1 FROM kegs WHERE beer_id = ?)` then deletes as today (research R9). Update existing `beers` and `httpapi` beer tests that compare whole JSON objects for the new `sync` field (makes T007, T008 pass)
- [ ] T015 [P] Implement `api/internal/importer/provider.go`: `Provider` interface (`Check`, `ListBatches` documented as "every batch not archived, all pages", `GetBatch`), `Factory func(userID, apiKey string) Provider`, `Batch` (`Ref string`, `Name string`, `Number *int64`, `Status string` one of the four beer statuses, `Archived bool`, `Style`, `Notes`, `BrewDate *string`, `ABV`, `IBU`, `SRM *float64`), and errors `ErrBatchNotFound`, `ErrCredentialsRefused`, `ErrNoBatchAccess`, `ErrUnreachable`, `RateLimitedError{Until time.Time}`
- [ ] T016 **Before starting, the owner runs quickstart.md section P (real-account probe) and records its results in research.md R1**; use them to choose the `include` list or `complete=true` and to confirm the page size fits the 4 MiB cap. Implement `api/internal/brewfather/client.go` and `api/internal/brewfather/mapping.go`: `Client` with `BaseURL` (default `https://api.brewfather.app/v2`) and `HTTP *http.Client` (15 s timeout, TLS 1.2 minimum, `CheckRedirect` returns an error); `New(userID, apiKey string) importer.Provider` and a `NewWith(baseURL string, hc *http.Client, userID, apiKey string) (importer.Provider, error)` for tests that refuses a non-`https` base URL; `Check` = list one batch with `limit=1`; every request uses the caller's `ctx`; `ListBatches` fetches the 5 statuses concurrently, at most 5 in flight (research R4); JSON decoded from `io.LimitReader(body, 4<<20)`; the mapping and error translation tested in T009 (makes T009 pass; depends on T015 and the section P probe)
- [ ] T017 Implement `api/internal/importer/store.go`: settings `GetSetting`/`SetSetting`; `interval` parse/validate for `off`, `15m`, `1h`, `6h`, `24h`; connection save (upsert), load (returns user ID and sealed key), delete; sync run start/finish/read; `user_id` input limits "1 to 200" characters and `api_key` "1 to 500", trimmed, no control characters (makes T010 pass with T018's status reading)
- [ ] T018 Implement `api/internal/importer/service.go` skeleton: `Service` struct (`DB`, `Box *secrets.Box` (nil = not configured), `Factory`, `Now`), `New(...)`, the rate gate (`blockedUntil`, set from `RateLimitedError`, checked before every provider call, research R4), the sync mutex, `Status(ctx) (Status, error)` returning the `BrewfatherStatus` shape (`Configured`, `Connected`, `ReconnectNeeded` when `Open` fails, `Interval`, `SyncRunning`, `RetryAt`, `LastSync`), and a private `provider(ctx)` that loads and opens the connection and returns `ErrNotConfigured`, `ErrNotConnected` or `ErrReconnectNeeded`. Never put the user ID or key in `Status` or any error
- [ ] T019 Wire it up: in `api/internal/httpapi/routes.go` add `Deps.Importer *importer.Service`; in `api/internal/httpapi/helpers_test.go` make `newTestDeps` build a Service with a test `Box` (fresh random key), a fake factory and the test clock; add the fake provider as `api/internal/importer/importertest/fake.go` (a tiny package imported only by `_test.go` files in `importer` and `httpapi`) with programmable batches, per-call errors and a call counter
- [ ] T020 Read the encryption key at startup in `api/cmd/beer-menu-api/startup.go` and `main.go`: `config.SecretKey []byte`; `loadSecretKey(getenv, readFile)` reads `BEER_MENU_SECRET_KEY_FILE` (wins) or `BEER_MENU_SECRET_KEY`, trims, `secrets.ParseKey`; set but invalid or unreadable → startup error naming the variable, never the value; neither → nil (Brewfather not configured). Build `importer.New(db, box, brewfather.New, time.Now)` and pass it in `Deps`. Update the package doc comment in `main.go` to list both variables. Tests in `api/cmd/beer-menu-api/startup_test.go`: file wins over env; missing file, non-base64 and wrong length refuse with an error that contains the variable name and not the value; neither set gives nil

**Checkpoint**: `go test -race ./...` passes; nothing user-visible yet.

---

## Phase 3: User Story 1 - Connect a Brewfather account (Priority: P1) 🎯 MVP

**Goal**: an admin connects, sees the status, and disconnects; the key is sealed and never
leaks.

**Independent Test**: with the encryption key configured and a fake provider, connect with valid
credentials, see `connected: true`, confirm the key is in no response, log line, database file
or backup; disconnect and see the row gone (spec story 1).

### Tests for User Story 1

- [ ] T021 [P] [US1] Service tests in `api/internal/importer/service_test.go`: `Connect` calls `Check` once and saves only on success; `ErrCredentialsRefused`, `ErrNoBatchAccess`, `ErrUnreachable` and `RateLimitedError` save nothing and keep an existing connection; a second `Connect` replaces the first; `Connect` with a nil `Box` gives `ErrNotConfigured` without calling the provider; `Disconnect` deletes the connection, sets nothing else, keeps imported beers, and is idempotent; `Status` after a key change reports `ReconnectNeeded` and connecting again clears it
- [ ] T022 [P] [US1] Handler tests in `api/internal/httpapi/brewfather_test.go`: `GET /api/brewfather` with session, read token and write token → 200 status (exact field set per contract, no `user_id`); `PUT /api/brewfather/connection` with a session and same-origin → 200 `connected: true`; bad body, unknown field, `user_id` over 200 or `api_key` over 500 characters or with control characters → 400 `invalid_request`; refused → 400 `credentials_refused`; no batch scope → 400 `no_batch_access`; unreachable → 502 `provider_unreachable`; rate limited → 429 `provider_rate_limited` with `Retry-After` and `retry_at`; not configured → 503 `not_configured` and `GET` shows `configured: false`; `PUT` and `DELETE /api/brewfather/connection` with a read or write token → 403, without credentials → 401, cross-site session → 403; `DELETE` → 204
- [ ] T023 [P] [US1] Leak test in `api/internal/httpapi/brewfather_leak_test.go` (FR-005, FR-037, SC-004): install a capturing `slog` handler for the test; connect with fake key `K` and user `U`; run status, a failed connect, a disconnect and reconnect; assert no response body or header, and no captured log record, contains `K`, `U` or `base64(U:K)`; then `VACUUM INTO` a backup with `store.Backup`, read the database file, its `-wal` and the backup as bytes, and assert none contains `K`
- [ ] T024 [P] [US1] Component tests in `web/src/components/BrewfatherPanel.test.tsx` for the connection part: `configured: false` shows the not-set-up message naming `BEER_MENU_SECRET_KEY_FILE` and no form; not connected shows a form with user ID and an `<input type="password">` API key; submit `PUT`s both, then clears the key field whether it succeeds or fails; error codes `credentials_refused`, `no_batch_access`, `provider_unreachable`, `provider_rate_limited` show plain messages; connected shows "Connected" and a Disconnect button that asks for confirmation then `DELETE`s; `reconnect_needed` shows "Reconnect needed" with the form; a 401 calls `onSignedOut`

### Implementation for User Story 1

- [ ] T025 [US1] Implement `Connect(ctx, userID, apiKey)` and `Disconnect(ctx)` in `api/internal/importer/service.go`: validate inputs (T017 limits), refuse with `ErrNotConfigured` when `Box` is nil, call `Factory(userID, apiKey).Check` through the rate gate, then seal with purpose `"brewfather.api_key"` and save; drop the batch list cache on both; log `"brewfather connected"` / `"brewfather disconnected"` with no attributes beyond the provider name (makes T021 pass)
- [ ] T026 [US1] Implement `getBrewfather`, `connectBrewfather`, `disconnectBrewfather` and `writeImporterError` in `api/internal/httpapi/brewfather.go`, mapping errors to the contract codes: `not_configured` 503, `not_connected` 409, `reconnect_needed` 409, `credentials_refused` 400, `no_batch_access` 400, `provider_unreachable` 502, `provider_rate_limited` 429 with `Retry-After` (whole seconds, at least 1) and `retry_at`, `sync_running` 409, `not_imported` 409; anything else falls through to `writeBeerError`. Decode bodies with `decodeJSON(w, r, &dst, maxBody)`; never log the request body; run `connectBrewfather` under the same 20 s deadline as T036 (makes T022, T023 pass with T030)
- [ ] T027 [US1] Build the connection part of `web/src/components/BrewfatherPanel.tsx` (props `onSignedOut`, `onBeersChanged`): loads `GET /api/brewfather`; the states and messages tested in T024; the key field is never prefilled and is cleared after submit (makes T024 pass)
- [ ] T028 [US1] Add `BrewfatherPanel` to `web/src/pages/AdminPage.tsx` and stub `GET /api/brewfather` in `web/src/pages/AdminPage.test.tsx`
- [ ] T029 [P] [US1] Document the encryption key in `README.md` (what it protects, `openssl rand -base64 32`, `BEER_MENU_SECRET_KEY_FILE` preferred over `BEER_MENU_SECRET_KEY` because env values show in `docker inspect`, losing it only means re-entering the Brewfather key, a restore needs the same key) and add a commented example to `docker-compose.yaml` using a compose `secrets:` entry, with no real value
- [ ] T030 [US1] Register all 8 routes in `api/internal/httpapi/routes.go` with the access in research R11 (handlers for later stories return 501 until their tasks land; every route must exist so the `x-access` contract test passes from here on). Add the `routeFixtures` entry in `api/internal/httpapi/auth_walk_test.go` for `GET /api/brewfather` (read) now; each write route's fixture is added by the task that implements its handler (T036, T043, T051), so `TestRouteFixturesMatchRoutes` and the write walk are red from here until T051 and fully green only after it. Write fixtures first connect the fake provider and, for the beer route, create an imported beer. Change the write walk to skip only its cross-site case for `GET` routes, with a comment that `http.CrossOriginProtection` never blocks safe methods (plan Complexity Tracking)

**Checkpoint**: story 1 works end to end with a fake provider; quickstart sections 1, 2 and 7.

---

## Phase 4: User Story 2 - Import batches as beers (Priority: P1)

**Goal**: list non-archived batches with import markers, import selected ones, no duplicates,
archived refused, bad values cleaned.

**Independent Test**: fake account with Planning, Fermenting, Completed, one with missing values
and one Archived; the list shows four; import three; fields match the mapping, missing values
are null, a re-import makes no second beer, the archived ID is refused (spec story 2).

### Tests for User Story 2

- [ ] T031 [P] [US2] Service tests in `api/internal/importer/importing_test.go`: `Batches(ctx, refresh)` returns the provider's batches newest brew date first (nil dates last), with `BeerID` from the database even when served from cache; a second call within 5 minutes makes no provider call, `refresh` or a call after 5 minutes (fake clock) does; connect and disconnect drop the cache; `Import(ctx, ids)` per id: `GetBatch` fresh, `Sanitize`, `CreateImported`, clears any dismissal, outcome `imported` with `adjusted` fields; already linked → `already_imported` with the existing beer and no provider call needed beyond the lookup; `Archived` → `archived`, no beer; `ErrBatchNotFound` → `not_found`; a rate limit stops the rest, which report `rate_limited`; one `failed` for bad data does not stop the others; `ErrUnreachable` or an expired context stops the rest, which report `failed` with message "Not attempted: Brewfather did not answer in time" (FR-025a); `Import` and `Batches` with the rate gate already closed return `RateLimitedError` without a provider call; results are in request order
- [ ] T032 [P] [US2] Handler tests in `api/internal/httpapi/brewfather_test.go`: `GET /api/brewfather/batches` → 200 `{batches, fetched_at}` per contract, `?refresh=true` bypasses cache; read token → 403; not connected → 409 `not_connected`; `POST /api/brewfather/imports` with `batch_ids` empty, over 50, duplicated, non-string or an id over 100 characters → 400; success → 200 with one result per id and beers whose `source` is `brewfather`, `source_ref` the batch id and `sync.synced_at` set; then `GET /api/menu` lists an imported fermenting beer with no keg on deck (spec 2.9); with the rate gate closed both routes give 429 `provider_rate_limited` with `retry_at` and the fake provider records no call; a fake provider that blocks until its context ends makes both routes answer 502 `provider_unreachable` within the 20 s deadline (inject a short deadline in tests)
- [ ] T033 [P] [US2] Component tests in `web/src/components/BrewfatherPanel.test.tsx` for the batch list: shown only when connected; rows show name, number, status and brew date; imported rows show "Imported" with the beer name and no checkbox; Refresh calls `?refresh=true`; "Import selected" posts the checked ids, shows each result's outcome and adjusted fields, and calls `onBeersChanged`; a 429 shows the retry time
- [ ] T034 [P] [US2] Component tests in `web/src/components/BeersPanel.test.tsx`: a beer with `source: "brewfather"` shows a "Brewfather" label; a manual beer does not

### Implementation for User Story 2

- [ ] T035 [US2] Implement `Batches(ctx, refresh)` (5-minute in-memory cache, research R5) and `Import(ctx, ids)` in `api/internal/importer/importing.go`, both checking the rate gate first and stopping an import at the first `ErrUnreachable` or context expiry (FR-025a) (makes T031 pass)
- [ ] T036 [US2] Implement `listBrewfatherBatches` and `importBrewfatherBatches` in `api/internal/httpapi/brewfather.go`, each running under `context.WithTimeout(r.Context(), 20*time.Second)` (a package-level duration tests can shorten); add their `routeFixtures` entries; validate `batch_ids` "minItems: 1, maxItems: 50, uniqueItems", each "minLength: 1, maxLength: 100"; log counts only (makes T032 pass)
- [ ] T037 [US2] Build the batch list in `web/src/components/BrewfatherPanel.tsx` and add the `beerChanges` counter in `web/src/pages/AdminPage.tsx` that `BrewfatherPanel.onBeersChanged` bumps and `BeersPanel` reloads on, as `kegChanges` does (makes T033 pass)
- [ ] T038 [US2] Show the "Brewfather" source label in `web/src/components/BeersPanel.tsx` (makes T034 pass)

**Checkpoint**: stories 1 and 2 work; quickstart section 3.

---

## Phase 5: User Story 3 - Re-sync an imported beer (Priority: P2)

**Goal**: one-click re-sync updates measurements, status and archive; local edits stay.

**Independent Test**: import, edit name and description locally, change the fake batch's ABV,
status and name, re-sync; ABV and status change, name and description stay (spec story 3).

### Tests for User Story 3

- [ ] T039 [P] [US3] Service tests in `api/internal/importer/importing_test.go` for `Resync(ctx, beerID)`: manual beer → `ErrNotImported`; unknown → `beers.ErrNotFound`; present batch → `ApplySync` result with `adjusted`; `ErrBatchNotFound` → beer marked not found, otherwise unchanged, no error; Archived batch with no keg → archived; with a current keg → `archive_pending`; rate limit → `RateLimitedError`, nothing written; works on an archived beer
- [ ] T040 [P] [US3] Handler tests in `api/internal/httpapi/brewfather_test.go` for `POST /api/beers/{id}/sync`: 200 `{beer, adjusted}`; manual beer → 409 `not_imported`; unknown or non-positive id → 404; read token → 403; rate limited → 429; the beer's `sync.not_found` and `sync.archive_pending` appear in `GET /api/beers/{id}`
- [ ] T041 [P] [US3] Component tests in `web/src/components/BeersPanel.test.tsx`: Brewfather beers have a Re-sync button that `POST`s `/api/beers/{id}/sync` and reloads; manual beers have none; `sync.not_found` shows "Not found in Brewfather"; `sync.archive_pending` shows "Archived in Brewfather; kick its keg to archive"; a 429 shows the retry time

### Implementation for User Story 3

- [ ] T042 [US3] Implement `Resync` and `ErrNotImported` in `api/internal/importer/importing.go` (makes T039 pass)
- [ ] T043 [US3] Implement `resyncBeer` in `api/internal/httpapi/brewfather.go`; run under the same 20 s deadline and add its `routeFixtures` entry (makes T040 pass)
- [ ] T044 [US3] Add the Re-sync button and sync notes to `web/src/components/BeersPanel.tsx` (makes T041 pass)

**Checkpoint**: quickstart sections 4 and 5.

---

## Phase 6: User Story 4 - Scheduled sync (Priority: P2)

**Goal**: an interval set in the panel runs syncs in the API process; "Sync now" runs one;
results show in the panel.

**Independent Test**: shortest interval against a fake provider; add a Fermenting batch and
change an imported batch's status; after one interval the new batch is imported and the status
changed, the panel shows the counts; turn it off and no more syncs run (spec story 4).

### Tests for User Story 4

- [ ] T045 [P] [US4] Service tests in `api/internal/importer/sync_test.go` for `Sync(ctx)` (research R10): one `ListBatches`; every non-archived imported beer whose batch is listed gets `ApplySync`; an imported beer missing from the list gets exactly one `GetBatch`: Archived → archive rules, not found → `not_found`; a beer already `not_found` and missing from the list gets no call at all (FR-021), and its flag clears if its batch is listed again; archived local beers are skipped entirely; new batches with status fermenting or conditioning (Brewing, Fermenting, Conditioning upstream), not linked and not dismissed, are imported; planned and ready batches are not; a beer deleted mid-run is skipped without error; `sync_runs` records counts and `ok`; a list failure records `ok = 0` with a plain message and changes nothing; a concurrent `Sync` gets `ErrSyncRunning`; the provider call count for 120 active batches across 5 statuses and no missing beers is at most 7 (SC-005)
- [ ] T046 [P] [US4] Scheduler tests in `api/internal/importer/scheduler_test.go` for `Tick(ctx)` with a fake clock: interval off → no sync; not connected or reconnect needed → no sync; due (`now ≥ last started_at + interval` or never ran) → sync; not due → none; rate gate closed → none until `retry_at`; a new `Service` on the same database (restart) keeps the cadence from `sync_runs`; `Run(ctx)` returns when `ctx` is cancelled
- [ ] T047 [P] [US4] Handler tests in `api/internal/httpapi/brewfather_test.go`: `PUT /api/brewfather/schedule` with a session → 200 with the new `interval`; invalid interval → 400; token → 403; cross-site → 403; `POST /api/brewfather/sync` → 202 with `sync_running: true`, then (wait on the service, not on time) status shows `last_sync`; a second call while running → 409 `sync_running`; not connected → 409
- [ ] T048 [P] [US4] Component tests in `web/src/components/BrewfatherPanel.test.tsx` (fake timers): interval `<select>` shows the current value and `PUT`s a change; "Sync now" posts, then polls `GET /api/brewfather` every 2 s while `sync_running` and stops when it is false, then shows the last sync time, counts and message and calls `onBeersChanged`; a failed last sync shows its message; polling stops on unmount

### Implementation for User Story 4

- [ ] T049 [US4] Implement `Sync`, `SetInterval` and `ErrSyncRunning` in `api/internal/importer/sync.go` and `api/internal/importer/service.go`; each beer's change in its own transaction; message ≤ 500 characters, no key or provider body (makes T045 pass)
- [ ] T050 [US4] Implement `Run(ctx)` (one-minute ticker) and `Tick(ctx)` in `api/internal/importer/scheduler.go`; start `go svc.Run(ctx)` from `run` in `api/cmd/beer-menu-api/main.go` on the shutdown context; "Sync now" goroutines use the same base context (makes T046 pass)
- [ ] T051 [US4] Implement `setBrewfatherSchedule` and `syncBrewfather` in `api/internal/httpapi/brewfather.go`; give `Deps` (or the Service) a base context for background syncs so tests can wait on completion; add their `routeFixtures` entries, after which the route walk is fully green (makes T047 pass)
- [ ] T052 [US4] Build the schedule and "Sync now" part of `web/src/components/BrewfatherPanel.tsx` (makes T048 pass)

**Checkpoint**: all stories work; quickstart section 6.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T053 [P] Run `npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml`; confirm every route in `Routes` matches an operation and `x-access`, and `TestPublicRoutes` is unchanged (no new public route)
- [ ] T054 [P] Full gates: `(cd api && go vet ./... && go test -race ./...)`, `(cd web && npm ci && npm test && npm run build)`, `cd api && ~/go/bin/govulncheck ./...`, `~/go/bin/gitleaks detect --source . --redact` and `~/go/bin/gitleaks detect --no-git --source <dir> --redact` for `api`, `web/src`, `specs`, `docs`
- [ ] T055 Build the API image and confirm it can reach Brewfather over TLS: `docker build api` then run the binary's `healthcheck` plus a one-off check that `/etc/ssl/certs/ca-certificates.crt` exists in the image (`docker create` + `docker cp`), since `scratch` has no shell
- [ ] T056 Run quickstart.md sections 1 to 8 on a local stack with the owner's real Brewfather account (owner enters the key; never written to a file in the repo). Record in research.md R1 whether the `include` list and the `status` filter behaved as documented, and fix the client if not
- [ ] T057 [P] Screenshot the admin page with the Brewfather panel in headless Firefox (memory recipe) in its not-configured, connected and batch-list states to confirm it renders without console errors (layout is unstyled by design)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none; T001 to T003 in parallel. After T001 the `x-access` contract test is red until T030 registers the routes.
- **Foundational (Phase 2)**: needs Setup. T016 also needs the owner's quickstart section P probe. T011 before T014, T017; T015 before T016, T018; T012 before T018, T020; T018 before T019; tests T004 to T010 first, in parallel.
- **US1 (Phase 3)**: needs Foundational. T030 registers every route, so do it early in this phase.
- **US2 (Phase 4)**: needs US1 (a connection).
- **US3 (Phase 5)**: needs US2 (an imported beer). Independent of US4.
- **US4 (Phase 6)**: needs US2's `Import` (auto-import) and US3's `ApplySync` path through `beers` (already foundational), so it can run alongside US3 once US2 is done.
- **Polish (Phase 7)**: needs all stories.

### User Story Dependencies

- US1 → US2 → (US3 ∥ US4).

### Within Each User Story

- Tests first, failing. Service before handler before web.
- API and web tasks of one story touch disjoint folders and can run in parallel (two coding
  agents; only the orchestrator edits tasks.md).

### Parallel Opportunities

- Phase 2 tests T004 to T010 all [P]; T012, T013, T015 in parallel.
- In each story, the Go service test, handler test and web component test tasks are [P].
- US3 and US4 in parallel after US2.

---

## Parallel Example: User Story 2

```text
Agent A (api): T031 → T035 → T032 → T036
Agent B (web): T033 → T034 → T037 → T038
```

---

## Implementation Strategy

### MVP First

1. Phases 1 and 2.
2. Phase 3 (US1): connect safely. Stop and validate quickstart sections 1, 2, 7.
3. Phase 4 (US2): import by hand. This is the first owner-visible value (beers without typing).

### Incremental Delivery

1. US1 + US2 → usable import.
2. US3 → manual re-sync and archive-follow.
3. US4 → hands-off on-deck updates.
4. Polish, real-account check (T056), PR.
