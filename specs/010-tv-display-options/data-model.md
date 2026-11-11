# Data Model: TV Display Options

No schema change. The database stays at version 5. New settings are rows in the existing
`settings (key, value, updated_at)` table (research R1). A missing row or a stored value that
fails validation reads as its default.

## Display settings (extends feature 7)

| JSON field | Settings key | Type | Allowed | Default | Public (`/api/menu` `display`) |
| --- | --- | --- | --- | --- | --- |
| `title` | `display.title` | string | feature 7 | `""` | yes |
| `theme` | `display.theme` | string | feature 7 | `dark` | yes |
| `units` | `display.units` | string | feature 7 | `us` | **no** |
| `page_seconds` | `display.page_seconds` | integer | feature 7 | `15` | yes |
| `layout` | `display.layout` | string | `grid`, `list`, `columns` | `grid` | yes |
| `title_align` | `display.title_align` | string | `left`, `center`, `right` | `left` | yes |
| `title_font` | `display.title_font` | string | font key (below) | `system` | yes |
| `text_font` | `display.text_font` | string | font key (below) | `system` | yes |
| `show_style` | `display.show_style` | boolean | `true`, `false` | `true` | yes |
| `show_abv` | `display.show_abv` | boolean | `true`, `false` | `true` | yes |
| `show_ibu` | `display.show_ibu` | boolean | `true`, `false` | `true` | yes |
| `show_srm` | `display.show_srm` | boolean | `true`, `false` | `false` | yes |
| `show_description` | `display.show_description` | boolean | `true`, `false` | `false` | yes |
| `title_color` | `display.title_color` | string | `""` or `#rrggbb` | `""` | yes |
| `text_color` | `display.text_color` | string | `""` or `#rrggbb` | `""` | yes |

Stored forms: booleans as `"1"` / `"0"`; colors lowercase.

Font keys: `system`, `inter`, `lora`, `roboto-slab`, `oswald`, `nunito`, `bebas-neue`,
`pacifico` (research R5).

### Validation (PATCH, all-or-nothing)

- Enum fields: one of their allowed strings, else `Must be one of: …`.
- Switches: a JSON boolean, else `Must be true or false.`
- Colors: a string that is `""` or `#` plus 6 hex digits (any case, stored lowercase), else
  `Must be a color like #1a2b3c, or empty for the theme color.`
- `null`, a wrong JSON type and unknown fields: refused as in feature 7. Messages never echo
  the value.

## Public menu beer (extends feature 5)

Tap beers (`taps[].beer`) gain two fields. On deck beers do not (research R9).

| Field | Type | Source |
| --- | --- | --- |
| `srm` | number, omitted when unset | `beers.srm` |
| `description` | string, omitted when unset | `beers.description` (max 2,000 characters) |

The TV shows `srm` as `SRM N` (rounded) and the description clamped to 2 lines, each only
when its switch is on and the value is present.

## Client-side, not stored

- **Page** (`tv.ts`): `grid` page = taps or on deck (≤ 8); `list` page = taps or on deck (≤ 6
  rows); `columns` page = up to 6 taps and up to 6 on deck items side by side.
- **Preview draft** (`SettingsPanel`): the form's unsaved values; invalid fields fall back to
  the last valid value for drawing.
