# Research: Brewfather Import

Decisions for feature 6. Each has the decision, why, and what else was considered.

## R1. Brewfather API facts (verified 2026-11-09)

Checked against docs.brewfather.app/api and docs.brewfather.app/api/types.md:

- Base URL `https://api.brewfather.app/v2`, HTTPS only. v1 is deprecated.
- HTTP Basic auth: `base64(userid:apikey)` in the `Authorization` header. Keys have scopes;
  this feature needs only `batches.read`.
- 500 calls per hour per key; over that, `429 Too Many Requests` with `Retry-After`. Limits
  "can change without notice".
- `GET /v2/batches`: all statuses by default; `limit` default 10, max 50; `start_after` = last
  `_id` of the previous page; `order_by` default `_id`; `include` takes a comma list of extra
  fields (nested paths such as `recipe.mash` allowed) when `complete=false`.
- `GET /v2/batches/:id`: one batch; same `include`.
- Batch fields used: `_id`, `name`, `batchNo`, `status` (Planning, Brewing, Fermenting,
  Conditioning, Completed, Archived), `brewDate` (Unix ms), `measuredAbv` (nullable),
  `estimatedIbu`, `estimatedColor` (SRM), `batchNotes`, `tasteNotes`, `recipe.name`,
  `recipe.style.name`, `recipe.abv`. No batch-level style; colors are SRM, not EBC.
- `status` filters the list by one status. Not documented: whether it takes several values in
  one call. The client makes one list call per active status (Planning, Brewing, Fermenting,
  Conditioning, Completed), paged, and never lists Archived; the quickstart checks the filter on
  a real account.
- Not documented: the exact 404 body for an unknown batch, and whether `include` can return
  every field above in one list call. The client treats any 404 as "batch not found", and the
  quickstart has a manual step against a real account (owner only, key never committed) to
  confirm the include list returns all fields. If it does not, the client switches to
  `complete=true` for list pages (one call per 50 batches either way).
- **Probe results (owner's real account, quickstart section P, 2026-11-10)**:
  - Count per status with `include`: Planning 1, Brewing 0, Fermenting 0, Conditioning 0,
    Completed 2. The account has hundreds of archived batches, so `status` does filter.
  - Every mapped field appears in the `include` response (paths of one Completed batch):
    `_id`, `name`, `batchNo`, `status`, `brewDate`, `measuredAbv`, `estimatedIbu`,
    `estimatedColor`, `batchNotes`, `tasteNotes`, `recipe.name`, `recipe.style.name`,
    `recipe.abv`. (`brewer` also came back; not used.)
  - A 50-batch `complete=true` page (no status filter) is 2,141,256 bytes, about 2 MiB, or
    about 43 KB per batch.
  - **Decision for T016**: list pages use `include` with the field list above, not
    `complete=true`. The responses are much smaller and every field is present. `limit=50`
    stays, and the 4 MiB read cap is safe for `include` pages.
- **Quickstart sections 1 to 7 on the owner's real account (T056, 2026-11-10)**: the `include`
  list and the `status` filter behaved as documented. Only the 3 non-archived batches were
  listed, and an archived batch's ID imported as `archived`. One finding: Brewfather names
  every new batch `Batch`, so the batch name hid the beer name. The name chain now treats a
  blank or default `Batch` name as none and falls back to `recipe.name` (FR-012). Batch names
  can carry trailing spaces; `Sanitize` trims them on import, and the batch list shows them as
  sent. Every other step passed:
  - connect and refuse;
  - the key absent from logs, database and WAL;
  - import and `already_imported`;
  - re-sync keeping local edits;
  - deferred archive while a keg is current;
  - scheduled auto-import keeping its cadence across a restart;
  - a deleted auto-import not coming back;
  - `reconnect_needed` after a key change;
  - startup refused on a malformed key;
  - `configured: false` with no key.

## R2. Encryption at rest: AES-256-GCM from the standard library

- **Decision**: a new package `api/internal/secrets` with a `Box` holding a 32-byte key.
  `Seal(plain, purpose)` returns `"v1:" + base64(nonce ‖ ciphertext)` using AES-256-GCM with a
  random 12-byte nonce and `purpose` (`"brewfather.api_key"`) as additional data, so a sealed
  value cannot be moved to another column and still open. `Open` returns `ErrCannotOpen` for a
  wrong key, tampered value or unknown version, never the cause in detail.
- **Key source**: `BEER_MENU_SECRET_KEY_FILE` (path to a file, for Docker secrets) wins over
  `BEER_MENU_SECRET_KEY`. Value: standard base64 of exactly 32 bytes, surrounding whitespace
  trimmed. Generate with `openssl rand -base64 32`. Neither set: `Box` is nil and Brewfather
  reports `configured: false` (FR-006). Set but unreadable, not base64 or not 32 bytes: the API
  refuses to start with an error that names the variable, never the value.
- **Why**: `crypto/aes` + `crypto/cipher` are stdlib, authenticated and standard; no new
  dependency (constitution VII). The version prefix leaves room for key rotation later without
  a migration.
- **Alternatives**: `golang.org/x/crypto/nacl/secretbox` (already a dependency): fine, but
  GCM needs no extra import path. Deriving the key from a passphrase (scrypt/argon2): adds a
  parameter set and salt for no gain over a random key. Generating the key into `/data`: it
  would sit beside the database and its backups, defeating the point. Rejected.

## R3. Package layout and provider boundary

- **Decision**:
  - `api/internal/importer`: provider-neutral core. `Provider` interface, common `Batch`,
    `Service` (connect, status, list with cache, import, re-sync, sync, scheduler tick), rate
    gate, and the connection, settings and dismissal SQL.
  - `api/internal/brewfather`: the only code that knows Brewfather's URLs, JSON and statuses.
    It implements `importer.Provider` and maps to `importer.Batch`.
  - `api/internal/secrets`: R2.
  - `api/internal/beers`: gains `Sanitize` (R6), `ApplySync` (R7) and dismissal on delete
    (R9); its JSON gains `sync`.
- **Interface**:

  ```go
  type Provider interface {
      Check(ctx context.Context) error                       // credentials work for batch reads
      ListBatches(ctx context.Context) ([]Batch, error)       // every batch not archived, all pages
      GetBatch(ctx context.Context, id string) (Batch, error) // ErrBatchNotFound
  }
  type Factory func(userID, apiKey string) Provider
  ```

  Errors that cross the boundary are importer types only: `ErrBatchNotFound`,
  `ErrCredentialsRefused` (401), `ErrNoBatchAccess` (403), `*RateLimitedError{Until}` (429),
  `ErrUnreachable` (network, timeout, 5xx, unreadable body).
- **Batch**: `Ref` (provider batch ID), `Name`, `Number *int64`, `Status` (one of the four beer
  statuses), `Archived bool`, `Style`, `ABV`, `IBU`, `SRM *float64`, `BrewDate *string`
  (`YYYY-MM-DD`), `Notes *string`. The provider applies FR-012's fallbacks (name chain, measured
  ABV else recipe ABV, tasting notes else batch notes) and FR-013's status map, so the core
  never sees `measuredAbv` or `Brewing`.
- **Why**: constitution II. `Factory` lets `httpapi` tests inject a fake provider; brewfather
  client tests use an `httptest.NewTLSServer`.
- **Alternatives**: one `integrations` package with Brewfather inside: blurs the boundary the
  constitution asks for. A generic plugin registry: speculative (VII); `Deps` holds one
  `*importer.Service` built with the Brewfather factory.

## R4. Calls to Brewfather: timeouts, size, rate limit

- **Decision**: one `http.Client` with `Timeout: 15s`, TLS 1.2 minimum, default proxy
  handling. The base URL must be `https://` (tests pass the TLS test server's URL and client).
  Each response body is read through `io.LimitReader` at 4 MiB; over that, or unreadable JSON,
  is `ErrUnreachable` with message "Brewfather sent an unreadable response". Bodies are never
  logged or put in errors.
- **Rate gate**: the `Service` keeps `blockedUntil`. A 429 sets it from `Retry-After` (seconds
  or HTTP date), defaulting to 60 s when absent or unreadable and capped at 1 hour. While
  blocked, every call (hand or scheduled) fails at once with `RateLimitedError`, mapped to HTTP
  429 `provider_rate_limited` with `Retry-After` and `retry_at`. The gate is in memory: a
  restart forgets it, costing at most one more 429.
- **Budget**: a sync makes one list call per active status, plus one per extra page of 50, so
  5 calls for most accounts however many batches are archived; plus one `GetBatch` per imported
  beer missing from that list that is not already marked not found (normally 0 or 1). Import uses one `GetBatch` per batch (max 50
  per request); hand re-sync one call. At 200 active batches the 15-minute schedule uses about
  24 of 500 calls per hour (SC-005).
- **Deadline for waiting requests (FR-025a)**: connect, batch list, import and re-sync handlers
  run their provider work under `context.WithTimeout(r.Context(), 20*time.Second)`; the client
  passes the context to every request. Import stops at the first `ErrUnreachable` or deadline
  and gives every untried batch outcome `failed` with message "Not attempted: Brewfather did not
  answer in time". `ListBatches` fetches the 5 statuses concurrently (at most 5 in flight), so a
  normal list takes one round trip, and a slow server hits the 20 s deadline instead of 5 × 15 s.
  A deadline is reported as `provider_unreachable` (502). Background syncs use the server's
  shutdown context with no overall deadline.
- **Rate gate on entry**: every hand route checks the gate before any work; when closed, it
  answers 429 `provider_rate_limited` with `retry_at` at once, including the import route.
- **Auth header safety**: Go drops `Authorization` on redirects to another host; the client
  also refuses any redirect (`CheckRedirect` returns an error) since the API never needs one.
- **Alternatives**: retries with backoff: hides failures and spends quota; the schedule is the
  retry. A token-bucket limiter: Brewfather's 429 is authoritative and our budget is tiny.

## R5. Batch list cache

- **Decision**: the `Service` keeps the last `ListBatches` result and its time; reads within 5
  minutes reuse it unless `refresh=true`. Connect, disconnect and a sync replace or drop it.
  Import and re-sync always call `GetBatch`. The cache is in memory only.
- **Why**: FR-027; the admin page may reopen the list often. The imported markers are computed
  from the database on every read, so a cached list never shows stale import state.

## R6. Cleaning imported values: `beers.Sanitize`

- **Decision**: `beers.Sanitize(in Input) (Values, []string)` returns valid `Values` and the
  JSON names of fields it changed. Per field: trim; strip control characters (description keeps
  newlines and tabs, as `Normalize` allows); shorten to the field's limit by runes (name 100,
  style 100, description 2000, brewer 100); round ABV and SRM to one decimal and IBU to whole;
  then a value still out of range (ABV > 25, SRM > 100, IBU outside the existing range, bad
  date) becomes nil. An empty name becomes "Brewfather batch". The result always passes
  `Normalize` (a test asserts this by round trip).
- **Archived on import**: `Import` calls `GetBatch`; a batch with `Archived` set gets outcome
  `archived` and no beer.
- **Why**: FR-014 says imported data is cleaned, not refused, and constitution V says it is
  validated. Living in `beers` reuses the same limits and rules, so they cannot drift.
- **Alternatives**: refuse a batch with any bad field: the owner cannot fix Brewfather data
  from here, so import would just fail. Clean in the provider: the provider would need the beer
  library's rules.

## R7. Sync state: columns on `beers`, archive on transition

- **Decision**: migration 0005 adds to `beers`: `synced_at INTEGER`, `sync_not_found INTEGER
  NOT NULL DEFAULT 0`, `sync_archive_pending INTEGER NOT NULL DEFAULT 0`, `sync_batch_archived
  INTEGER NOT NULL DEFAULT 0`, plus a unique index on `(source, source_ref)` where `source_ref`
  is not null (FR-015). `beers.ApplySync(db, id, Values, archived bool, now)` updates ABV, IBU,
  SRM, brew date, status, `synced_at`, clears `sync_not_found`, and handles FR-017a in one
  transaction:
  - batch archived and `sync_batch_archived = 0`: try to archive with the existing
    no-current-keg guard. Success sets `sync_batch_archived = 1`, `sync_archive_pending = 0`.
    A current keg sets `sync_archive_pending = 1` and leaves `sync_batch_archived = 0`, so the
    next sync tries again.
  - batch archived and `sync_batch_archived = 1`: nothing (the owner may have unarchived).
  - batch not archived: `sync_batch_archived = 0`, `sync_archive_pending = 0`. Never unarchives.
- **API**: `Beer` gains `sync` — `null` for manual beers, else `{synced_at, not_found,
  archive_pending}`. Added field only; existing clients unaffected (principle I).
- **Why**: one table, one `RETURNING`, no join in `columns`. A separate `beer_sync` table would
  need a join or subqueries in every beer read for three flags.

## R8. Settings, connection and dismissals storage

- **Decision** (migration `0005_brewfather.sql`):
  - `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at INTEGER NOT NULL)`: general
    key-value store; feature 7 adds display keys. This feature writes
    `brewfather.sync_interval` (`off`, `15m`, `1h`, `6h`, `24h`; absent means `off`).
  - `provider_connections(provider TEXT PRIMARY KEY CHECK (provider IN ('brewfather')), user_id
    TEXT NOT NULL, api_key_sealed TEXT NOT NULL, connected_at INTEGER NOT NULL)`.
  - `sync_runs(provider TEXT PRIMARY KEY, started_at, finished_at, ok INTEGER, message TEXT,
    updated INTEGER, imported INTEGER, not_found INTEGER)`: last sync only (one row per
    provider, upserted). Kept apart from the connection so a reconnect does not lose it and the
    scheduler reads its "last run" after a restart.
  - `import_dismissed(source TEXT NOT NULL, ref TEXT NOT NULL, dismissed_at INTEGER NOT NULL,
    PRIMARY KEY (source, ref))`.
- **Why**: FR-036; key-value settings match the brief's data model sketch. The sealed key is
  stored only in `provider_connections`.

## R9. Deleting an imported beer records a dismissal

- **Decision**: `beers.Delete` runs in a transaction: `INSERT OR IGNORE INTO import_dismissed
  SELECT source, source_ref, ? FROM beers WHERE id = ? AND source_ref IS NOT NULL AND NOT EXISTS
  (kegs)`, then the existing delete. A hand import deletes the matching dismissal (FR-022).
- **Why**: the dismissal must be written exactly when the delete succeeds; doing it in the
  importer would race the delete.

## R10. Sync run and scheduler

- **Decision**: `Service.Sync(ctx)` takes a `sync.Mutex` with `TryLock`; busy gives
  `ErrSyncRunning` (HTTP 409 `sync_running`). Steps: one `ListBatches` (active statuses only);
  for each non-archived brewfather beer, `ApplySync` if its batch is present (each in its own
  transaction, FR-024); else `GetBatch` for that beer alone: an Archived batch goes through
  `ApplySync` (archive on transition), `ErrBatchNotFound` sets `sync_not_found = 1`. Beers
  already `sync_not_found = 1` and missing from the list are skipped without a call (FR-021);
  a hand re-sync still checks them, and `ApplySync` clears the flag if the batch returns. Then import
  each batch with status fermenting or conditioning
  (Brewing, Fermenting, Conditioning upstream), not archived, not linked, not dismissed. Errors
  from the list call fail the run; a failure on one beer is counted and named in the message
  without stopping the rest. The run row records counts and a plain message.
- **"Sync now"**: `POST /api/brewfather/sync` starts the run in a goroutine on a context tied
  to server shutdown and answers 202; the panel polls status (`sync_running`) every 2 s until it
  ends. A sync can take 5 or more calls × 15 s, longer than nginx's 60 s proxy timeout, so a
  synchronous answer is unsafe.
- **Scheduler**: `Service.Run(ctx)` started from `main.run`, ticking every minute. A tick syncs
  when the interval is not off, an account is connected and decrypts, the rate gate is open,
  and `now ≥ last started_at + interval` (or never ran). Because the last run time is in
  `sync_runs`, restarts keep the cadence (story 4.7). Tests call `Tick(ctx)` with a fake clock.
- **Alternatives**: `time.Ticker` per interval: restarts reset the cadence and interval changes
  need a restart of the ticker. cron libraries: new dependency.

## R11. Routes and access

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| GET | `/api/brewfather` | read | Status (FR-009, FR-031) |
| PUT | `/api/brewfather/connection` | admin | Connect or replace (FR-001, FR-002) |
| DELETE | `/api/brewfather/connection` | admin | Disconnect (FR-008) |
| PUT | `/api/brewfather/schedule` | admin | Set interval (FR-020) |
| GET | `/api/brewfather/batches` | write | List batches, `?refresh=true` (FR-010) |
| POST | `/api/brewfather/imports` | write | Import up to 50 batch IDs (FR-011) |
| POST | `/api/brewfather/sync` | write | Sync now, 202 (FR-021) |
| POST | `/api/beers/{id}/sync` | write | Re-sync one beer (FR-017) |

- `GET /api/brewfather/batches` is the first GET with write access: listing spends the owner's
  quota (FR-030). `guard` already supports it. The route walk's cross-site check applies to
  unsafe methods only (`http.CrossOriginProtection` lets GET through), so the write walk skips
  the cross-site case for GET routes; the 401/403 cases still run.
- Connection, schedule and status routes return the status object; nothing returns `user_id`
  or the key (FR-009). Error codes: `not_configured` 503, `not_connected` 409,
  `reconnect_needed` 409, `credentials_refused` 400, `no_batch_access` 400,
  `provider_unreachable` 502, `provider_rate_limited` 429, `sync_running` 409, `not_imported`
  409, plus the existing `invalid_request`, `not_found`.

## R12. Container: CA certificates and time zone

- **Decision**: the API image is `scratch`, which has no CA bundle, so HTTPS to Brewfather would
  fail. The build stage runs `apk add --no-cache ca-certificates` and the final stage copies
  `/etc/ssl/certs/ca-certificates.crt`. Brew dates convert in UTC (no tzdata in the image); the
  spec's edge case says so.
- **Why**: smallest fix; keeps `scratch`. `golang.org/x/crypto/x509roots/fallback` would embed
  roots but is a separate module (new dependency) and its roots go stale with the binary.

## R13. Web: Brewfather panel

- **Decision**: `web/src/components/BrewfatherPanel.tsx` with sections for status, connect form
  (password input, cleared after submit, never prefilled), disconnect (confirm), interval
  `<select>`, "Sync now" with polling while `sync_running`, and the batch list (checkboxes,
  imported marker linking to the beer name, Refresh, Import selected, per-batch results).
  `BeersPanel` shows "Brewfather" for imported beers, a Re-sync button, and "Not found in
  Brewfather" / "Archived in Brewfather; kick its keg to archive". `AdminPage` passes a
  `beerChanges` counter bump after import or sync so `BeersPanel` reloads, as `kegChanges` does.
  Unstyled like the other panels (FR-035).

## R14. Logging and secrets hygiene

- **Decision**: log events with `provider`, counts and error codes only. Never log `user_id`,
  the key, the sealed value, request bodies of the connect route, Brewfather responses or URLs
  with query strings. Tests: a slog handler capturing every record during connect, import and
  sync asserts the fake key and its base64 Basic form never appear; a test reads the database
  file and a `VACUUM INTO` backup as bytes and asserts the key is absent (FR-037).
