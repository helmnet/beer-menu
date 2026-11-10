# Data Model: Brewfather Import

Migration `api/internal/store/migrations/0005_brewfather.sql` (schema v4 to v5). Decisions are
in [research.md](./research.md) R7 to R9.

## New tables

### `settings`

General key-value settings, shared with feature 7.

| Column | Type | Rules |
| --- | --- | --- |
| `key` | TEXT PRIMARY KEY | Dotted name, such as `brewfather.sync_interval` |
| `value` | TEXT NOT NULL | Validated by the owner of the key, not by the table |
| `updated_at` | INTEGER NOT NULL | Unix seconds |

Keys this feature uses: `brewfather.sync_interval` = `off` \| `15m` \| `1h` \| `6h` \| `24h`.
A missing row means `off`.

### `provider_connections`

One row per connected provider; no row means not connected.

| Column | Type | Rules |
| --- | --- | --- |
| `provider` | TEXT PRIMARY KEY | `CHECK (provider IN ('brewfather'))` |
| `user_id` | TEXT NOT NULL | 1 to 200 characters, no control characters; never returned by the API |
| `api_key_sealed` | TEXT NOT NULL | `v1:` + base64(nonce ‖ AES-256-GCM ciphertext), additional data `brewfather.api_key`; never returned |
| `connected_at` | INTEGER NOT NULL | Unix seconds |

Input limits on connect: `user_id` 1 to 200, `api_key` 1 to 500 characters, both trimmed, no
control characters. Both are checked with Brewfather before the row is written.

### `sync_runs`

The last sync per provider (upserted).

| Column | Type | Rules |
| --- | --- | --- |
| `provider` | TEXT PRIMARY KEY | |
| `started_at` | INTEGER NOT NULL | Scheduler cadence reads this |
| `finished_at` | INTEGER | NULL while running or if the process stopped mid-run |
| `ok` | INTEGER | 1, 0, or NULL while running |
| `message` | TEXT | Plain words, ≤ 500 characters; never a key or raw provider body |
| `updated` | INTEGER NOT NULL DEFAULT 0 | Beers re-synced |
| `imported` | INTEGER NOT NULL DEFAULT 0 | Batches auto-imported |
| `not_found` | INTEGER NOT NULL DEFAULT 0 | Beers whose batch was missing |

A row with `finished_at` NULL at startup is a run cut short by a restart; status reports it as
failed ("Sync was interrupted").

### `import_dismissed`

| Column | Type | Rules |
| --- | --- | --- |
| `source` | TEXT NOT NULL | Beer source, such as `brewfather` |
| `ref` | TEXT NOT NULL | Provider batch ID |
| `dismissed_at` | INTEGER NOT NULL | Unix seconds |

Primary key `(source, ref)`. Written when an imported beer is deleted; removed when that batch
is imported by hand.

## Changed table: `beers`

| Column | Type | Rules |
| --- | --- | --- |
| `synced_at` | INTEGER | Last successful sync of this beer; NULL for manual beers |
| `sync_not_found` | INTEGER NOT NULL DEFAULT 0 | 1 while the provider reports the batch missing |
| `sync_archive_pending` | INTEGER NOT NULL DEFAULT 0 | 1 while an archive from the provider waits on a current keg |
| `sync_batch_archived` | INTEGER NOT NULL DEFAULT 0 | 1 once the provider's archive was applied |

New index: `CREATE UNIQUE INDEX beers_source_ref ON beers(source, source_ref) WHERE source_ref
IS NOT NULL` (one beer per batch, FR-015). Existing rows are manual with NULL `source_ref`, so
the index builds on any v4 database.

Imported beer at creation: `source = 'brewfather'`, `source_ref = <Brewfather batch _id>` (the
permanent link used by re-sync, sync, the batch list's import markers and dismissals),
`synced_at = now`, brewer NULL. Archived batches are never imported.

## API shape change: `Beer.sync`

`null` when `source = 'manual'`, else:

```json
{ "synced_at": "2026-11-09T18:00:00Z", "not_found": false, "archive_pending": false }
```

`sync_batch_archived` stays internal.

## Common batch (in memory only)

`importer.Batch`: `Ref`, `Name`, `Number`, `Status` (planned \| fermenting \| conditioning \|
ready), `Archived`, `Style`, `ABV`, `IBU`, `SRM`, `BrewDate` (`YYYY-MM-DD`, UTC), `Notes`. Built
by the provider; cleaned by `beers.Sanitize` before any write.

### Brewfather to common batch (in `brewfather` only)

| Batch field | Common field |
| --- | --- |
| `_id` | `Ref` |
| `name` (blank or the default `Batch`, trimmed and any case, counts as absent), else `recipe.name`, else `Batch <batchNo>`, else `Brewfather batch` | `Name` |
| `batchNo` | `Number` |
| `status`: Planning → planned; Brewing, Fermenting → fermenting; Conditioning → conditioning; Completed, Archived → ready | `Status` |
| `status == Archived` | `Archived` |
| `recipe.style.name` | `Style` |
| `measuredAbv`, else `recipe.abv` | `ABV` |
| `estimatedIbu` | `IBU` |
| `estimatedColor` | `SRM` |
| `brewDate` (ms, > 0) → UTC date | `BrewDate` |
| `tasteNotes`, else `batchNotes` (non-blank) | `Notes` → description |

An unknown status string maps to planned (kept off the TV) and is not auto-imported.

## State: imported beer across syncs

```text
batch in active list ──► ApplySync: ABV, IBU, SRM, brew date, status; not_found = 0
batch not in active list ──► GetBatch(ref):
    Archived ──► ApplySync, then the archive rules below
    not found ──► not_found = 1, nothing else changes
batch Archived, sync_batch_archived = 0:
    no current keg ──► archived_at = now, sync_batch_archived = 1, archive_pending = 0
    current keg    ──► archive_pending = 1 (retry next sync)
batch Archived, sync_batch_archived = 1 ──► no archive change
batch not Archived ──► sync_batch_archived = 0, archive_pending = 0 (never unarchives)
beer archived locally ──► skipped by sync; hand re-sync still allowed
```

## Status object (API, not stored as one row)

`configured`, `connected`, `reconnect_needed`, `interval`, `sync_running`, `retry_at`
(rate gate, or null), `last_sync` (`started_at`, `finished_at`, `ok`, `message`, `updated`,
`imported`, `not_found`, or null). See [contracts/brewfather.openapi.yaml](./contracts/brewfather.openapi.yaml).
