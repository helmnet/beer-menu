# Data Model: TV Display

No schema change and no migration. The menu is a read-only view built from `taps`, `kegs` and
`beers` (feature 3 and 4 tables). Rules: [research.md](./research.md) R3, R4, R7.

## Menu (response of `GET /api/menu`)

| Field | Type | Notes |
| --- | --- | --- |
| `taps` | MenuTap[] | Enabled taps only, position order. May be empty. |
| `on_deck` | OnDeckItem[] | Waiting kegs (oldest first), then conditioning beers, then fermenting beers (each by name). May be empty. |

### MenuTap

| Field | Type | Notes |
| --- | --- | --- |
| `label` | string | Tap label, 1 to 50 characters. |
| `beer` | MenuBeer or null | Null when the tap has no current keg. |
| `keg` | MenuKeg or null | Null exactly when `beer` is null. |

### OnDeckItem

| Field | Type | Notes |
| --- | --- | --- |
| `beer` | MenuBeer | Always present. |
| `keg` | MenuKeg or null | Present for a waiting keg; null for a beer still in the fermenter. |
| `status` | `"fermenting"`, `"conditioning"` or null | Set exactly when `keg` is null. |

### MenuBeer

| Field | Type | Source |
| --- | --- | --- |
| `name` | string | `beers.name` |
| `style` | string or null | `beers.style` |
| `abv` | number or null | `beers.abv` (percent) |
| `ibu` | number or null | `beers.ibu` |
| `color` | `#rrggbb` or null | `beers.ColorFor(beers.srm)`; null without SRM |

### MenuKeg

| Field | Type | Source |
| --- | --- | --- |
| `shape` | `"corny"` or `"sanke"` | `kegs.TypeByKey(kegs.type).Shape` |
| `level` | number, 0 to 1 | `remaining_l / size_l`, clamped, not rounded |

## Selection rules

- A tap shows when `taps.enabled = 1`. Its keg is the one with `tap_id = taps.id AND kicked_at IS
  NULL` (at most one, feature 4 index `kegs_one_per_tap`).
- A waiting keg: `tap_id IS NULL AND kicked_at IS NULL`. A keg on a disabled tap is not waiting
  and does not show.
- An on-deck beer: `archived_at IS NULL`, status `conditioning` or `fermenting`, and no keg with
  `kicked_at IS NULL` (on any tap, enabled or not, or waiting).
- Never in the menu: ids, timestamps, `size_l`, `remaining_l`, keg type keys, tap position or
  enabled flag, beer description, brewer, brew date, source, source ref, archived beers, kicked
  kegs, planned or ready beers without a waiting keg.

## Web types

`web/src/api.ts` gains `Menu`, `MenuTap`, `OnDeckItem`, `MenuBeer`, `MenuKeg` mirroring the
tables above. `BeerArt` gains `level?: number` (R5).
