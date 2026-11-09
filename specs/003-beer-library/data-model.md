# Data Model: Beer Library

One new migration, `api/internal/store/migrations/0003_beers.sql`, takes the schema from version
2 to 3. The API backs up a version 2 database before applying it (existing backup-before-migrate
step), and a test applies it to a version 2 database holding an admin and a token (constitution
IV).

Times are stored as `INTEGER` Unix seconds (UTC) and returned as RFC 3339 strings, as in
feature 2.

## beers

A beer in the owner's library. Brief table `beers`, without `image_path` (spec Clarifications).

| Column | Type | Rules |
| --- | --- | --- |
| `id` | `INTEGER PRIMARY KEY` | stable (FR-007) |
| `name` | `TEXT NOT NULL` | trimmed, 1 to 100 code points, no control characters |
| `style` | `TEXT` | trimmed, up to 100, no control characters; empty → `NULL` |
| `abv` | `REAL` | 0 to 25, at most one decimal |
| `ibu` | `INTEGER` | 0 to 200 |
| `srm` | `REAL` | 0 to 100, at most one decimal |
| `description` | `TEXT` | trimmed, up to 2000; newline and tab allowed |
| `brewer` | `TEXT` | as `style` |
| `brew_date` | `TEXT` | `YYYY-MM-DD`, any valid date |
| `status` | `TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('planned','fermenting','conditioning','ready'))` | |
| `source` | `TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','brewfather'))` | not client-settable in this feature |
| `source_ref` | `TEXT` | `NULL` for manual beers; set by feature 6 |
| `archived_at` | `INTEGER` | `NULL` when current |
| `created_at` | `INTEGER NOT NULL` | |
| `updated_at` | `INTEGER NOT NULL` | set on every successful PATCH |

Validation lives in Go (research R4). The `CHECK`s guard only the enums.

**List order**: `ORDER BY name COLLATE NOCASE, id` (FR-014). Default filter
`archived_at IS NULL`; `?archived=true` filter `archived_at IS NOT NULL`.

**Lifecycle**:

```text
create ──> current ──PATCH archived:true──> archived (archived_at = now)
              ▲                                  │
              └──────PATCH archived:false────────┘ (archived_at = NULL)
current or archived ──DELETE──> row removed
```

Editing is allowed in both states. Archiving an archived beer keeps its `archived_at`.

## Beer (API representation)

Every column above, plus a derived field:

| Field | Type | Rule |
| --- | --- | --- |
| `color` | string or `null` | `"#rrggbb"` from the SRM color table; `null` when `srm` is `null` (research R6) |

Optional fields are always present in responses, as `null` when empty (FR-003).

## SRM color table

Not stored. A constant in `api/internal/beers` of 40 `"#rrggbb"` strings for SRM 1 to 40, with
its public source cited in a comment. Lookup: `round(srm)` clamped to 1..40.

## Vessel (web only)

Derived from whether the beer is on a tap: on a tap → pint glass; otherwise → corny keg,
whatever its status (FR-031). Taps arrive in feature 4, so in this feature every beer is a keg.
Not stored and not in the API.
