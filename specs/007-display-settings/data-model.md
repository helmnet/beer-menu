# Data Model: Display Settings

No schema change and no migration (research R1). The database stays at version 5.

## Storage: `settings` table (exists since 0001; `updated_at` since 0005)

| Column | Type | Notes |
| --- | --- | --- |
| `key` | TEXT PRIMARY KEY | |
| `value` | TEXT NOT NULL | Validated by the caller, not the table |
| `updated_at` | INTEGER NOT NULL DEFAULT 0 | Unix seconds of the last write |

## Display settings (new keys)

| Key | JSON field | Type | Allowed | Default | Public on `/api/menu` |
| --- | --- | --- | --- | --- | --- |
| `display.title` | `title` | string | trimmed, 0 to 60 code points, no control characters | `""` | yes |
| `display.theme` | `theme` | string | `dark`, `light` | `dark` | yes |
| `display.units` | `units` | string | `us`, `metric` | `us` | no |
| `display.page_seconds` | `page_seconds` | integer | 5 to 300 | `15` | yes |

`page_seconds` is stored as its decimal text (`"20"`).

Existing key, untouched: `brewfather.sync_interval` (feature 6), not part of these settings and
never returned by `/api/settings` or `/api/menu`.

## Rules

- **Read**: a missing row, or a stored value that fails validation, reads as the default. Reads
  never fail because of a bad value.
- **Write** (`PatchDisplay`): validate every field in the patch first; if any fails, write
  nothing and return the field errors. Otherwise write each named key in one transaction with
  the same `updated_at`, then return all four current values. Keys not in the patch are not
  touched. Concurrent patches: last write wins per key (edge case in spec).
- **Units**: the setting changes only how the admin app shows and takes volumes. Stored volumes
  (`kegs.size_l`, `kegs.remaining_l`, pour amounts) stay litres rounded to 0.01.

## Go types (package `settings`)

- `Display{Title, Theme, Units string; PageSeconds int}`: all four, JSON names above.
- `Public{Title, Theme string; PageSeconds int}`: the subset in `GET /api/menu` as `display`.
- `DisplayPatch`: each field optional; `null` is refused per field.

## Web types (`web/src/api.ts`)

- `Settings = { title: string; theme: "dark" | "light"; units: "us" | "metric"; page_seconds: number }`
- `MenuDisplay = { title: string; theme: "dark" | "light"; page_seconds: number }`; `Menu` gains
  `display` (the TV treats it as untrusted and falls back per field, research R8).
