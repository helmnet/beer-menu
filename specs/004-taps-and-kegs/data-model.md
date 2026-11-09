# Data Model: Taps and Kegs

One new migration, `api/internal/store/migrations/0004_taps_kegs.sql`, takes the schema from
version 3 to 4. The existing backup-before-migrate step runs first, and a test applies it to a
version 3 database holding an admin, a token and a beer (constitution IV).

Times are `INTEGER` Unix seconds (UTC), returned as RFC 3339. Volumes are `REAL` litres rounded
to 0.01 (research R2).

## taps

A fixed slot on the kegerator. Brief table `taps`.

| Column | Type | Rules |
| --- | --- | --- |
| `id` | `INTEGER PRIMARY KEY` | stable (FR-001) |
| `position` | `INTEGER NOT NULL` | 1..N, no gaps or repeats, kept by code (FR-003) |
| `label` | `TEXT NOT NULL` | trimmed, 1 to 50 code points, no control characters (FR-005) |
| `enabled` | `INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1))` | |
| `created_at` | `INTEGER NOT NULL` | |
| `updated_at` | `INTEGER NOT NULL` | |

The migration inserts `(position 1, label 'Tap 1', enabled 1)` at migration time (FR-002).

**Order**: `ORDER BY position`.

## kegs

One fill of one beer. Brief table `kegs`, plus `type` and `tap_label`.

| Column | Type | Rules |
| --- | --- | --- |
| `id` | `INTEGER PRIMARY KEY` | |
| `beer_id` | `INTEGER NOT NULL REFERENCES beers(id)` | a current beer when set (FR-013); blocks beer delete |
| `type` | `TEXT NOT NULL DEFAULT 'corny_5_gal'` | one of the keg types (research R4), checked in Go |
| `size_l` | `REAL NOT NULL` | `> 0`, `<= 100` |
| `remaining_l` | `REAL NOT NULL` | `0..size_l` |
| `tap_id` | `INTEGER REFERENCES taps(id) ON DELETE SET NULL` | `NULL` = waiting, or tap removed after kick |
| `tap_label` | `TEXT` | label of the last tap, written when the keg leaves a tap |
| `tapped_at` | `INTEGER` | first time on a tap; `NULL` if never tapped |
| `kicked_at` | `INTEGER` | `NULL` while current |
| `created_at` | `INTEGER NOT NULL` | |
| `updated_at` | `INTEGER NOT NULL` | every successful change, including pours |

**Indexes**:

- `CREATE UNIQUE INDEX kegs_one_per_tap ON kegs(tap_id) WHERE kicked_at IS NULL AND tap_id IS NOT NULL`
  (FR-014).
- `CREATE INDEX kegs_beer ON kegs(beer_id)` (beer guards and `on_tap`, filter by beer).

**Orders**: current and waiting `ORDER BY created_at, id`; kicked `ORDER BY kicked_at DESC, id DESC`.

**States and transitions**:

```text
                 PATCH tap_id:<empty tap>
  create ─┬─> waiting ───────────────────> on tap ──PATCH tap_id:null──> waiting
          │   (tap_id NULL)                  ▲  (tapped_at = COALESCE(tapped_at, now))
          └─(tap_id given)───────────────────┘
  waiting or on tap ──PATCH kicked:true──> kicked (kicked_at = now; tap_label snapshot)
  any state ──DELETE──> row removed
```

- Leaving a tap (take off or kick) writes `tap_label` from the tap's current label.
- Kicking keeps `tap_id` (history filter) and `remaining_l` as last recorded.
- On a kicked keg every change except delete and a repeat kick answers `409 keg_kicked`.
- Pours and `remaining_l` changes are allowed on waiting kegs too.

## Keg (API representation)

| Field | Type | Rule |
| --- | --- | --- |
| `id` | integer | |
| `beer_id` | integer | |
| `beer_name` | string | joined from `beers` |
| `beer_color` | string or `null` | `beers.ColorFor(srm)` |
| `type` | string | keg type key |
| `size_l`, `remaining_l` | number | litres, two decimals |
| `tap_id` | integer or `null` | |
| `tap_label` | string or `null` | live label while on a tap, else the snapshot |
| `state` | string | `on_tap`, `waiting` or `kicked`, derived |
| `tapped_at`, `kicked_at` | string or `null` | RFC 3339 |
| `created_at`, `updated_at` | string | RFC 3339 |

## Tap (API representation)

`id`, `position`, `label`, `enabled`, `created_at`, `updated_at`, and `keg`: the current keg on
it (Keg representation) or `null`.

## Beer (change)

Beer reads gain `on_tap` (boolean): true when a keg with `tap_id IS NOT NULL AND kicked_at IS
NULL` names the beer (research R6).

## Keg type table

Not stored. A constant list in `api/internal/kegs` (research R4): `type`, `label`, `size_l`,
`shape` (`corny` or `sanke`).
