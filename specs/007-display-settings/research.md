# Research: Display Settings

All spec questions were answered by the owner on 2026-11-10 (logo dropped; dark/light on the TV
only; US or metric in the admin app only). The items below are design choices for the plan.

## R1. Where settings live

- **Decision**: reuse the existing `settings` table (`key`, `value`, `updated_at`), one row per
  setting under `display.title`, `display.theme`, `display.units`, `display.page_seconds`. A
  missing row reads as its default. No migration.
- **Rationale**: the table already exists (0001, `updated_at` from 0005) and is generic. Defaults
  in code mean a fresh or upgraded database needs no seed rows (FR-014), and no schema change
  means no backup-and-migrate step on upgrade.
- **Alternatives**: a `display_settings` single-row table (needs migration 0006 and a seed row
  for nothing gained); one JSON blob row (one bad write loses all four; harder to validate per
  field).

## R2. Shared settings package

- **Decision**: new package `api/internal/settings` owns all SQL on `settings`: `Get` and `Set`
  move here from `importer` (importer calls them; its own key `brewfather.sync_interval` is
  unchanged), plus the display settings: `Display`, `GetDisplay`, `PatchDisplay`. `Get`/`Set`
  take a small interface satisfied by both `*sql.DB` and `*sql.Tx` so a patch writes all its
  keys in one transaction.
- **Rationale**: the project-status note asked to move rather than duplicate; keeps SQL in store
  packages (multi-tenant retrofit stays mechanical).
- **Alternatives**: keep `GetSetting` in `importer` and import it from httpapi (a display concern
  depending on the Brewfather importer); duplicate the two functions (two copies of the upsert).

## R3. Read and write routes

- **Decision**: `GET /api/settings` (`read`) returns all four settings, each its stored value or
  default. `PATCH /api/settings` (`write`) takes a merge-patch subset; the four keys are
  non-nullable, so `null` is refused per field; unknown fields refused; an empty object is
  `400` like other empty patches; the whole patch is validated before any write and written in
  one transaction (FR-002, FR-005). Returns `200` with all four. Body limit `maxBody` (8 KiB).
- **Rationale**: FR-002/FR-003 say any `write` token or session may change and any credential may
  read; this matches beers, taps and kegs (`write`), not Brewfather's admin-only schedule (which
  spends third-party quota; this does not).
- **Alternatives**: `PUT` with all four required (scripts must read before writing one value);
  per-setting routes (four routes, four fixtures, no gain).

## R4. How the TV gets its settings

- **Decision**: add a `display` object to the public `GET /api/menu`: `{title, theme,
  page_seconds}`. Units are not included (FR-004, the TV shows no volumes). The handler joins
  `kegs.GetMenu` and `settings.GetDisplay` in an `httpapi` response struct that embeds
  `kegs.Menu`, so `kegs` stays free of settings.
- **Rationale**: the TV already polls `/api/menu` every 10 s, so a change shows within one check
  (SC-001, ≤ 30 s) with no new request, no new public route and no change to
  `TestPublicRoutes`. Adding a field is backward compatible (constitution I).
- **Alternatives**: a new public `GET /api/display` (second public route, second poll chain, its
  own offline handling); exposing `GET /api/settings` publicly (leaks units and any later
  setting).

## R5. Validation

- **Decision**:
  - `title`: string, trimmed; 0 to 60 code points; no control characters (same rule as tap
    labels, `normalizeLabel`, without the required check). Empty or spaces-only stores `""`.
  - `theme`: `"dark"` or `"light"`.
  - `units`: `"us"` or `"metric"`.
  - `page_seconds`: JSON integer 5 to 300 (a fraction such as `15.5` is refused).
  - Errors: `400 invalid_request` with `fields` keyed by JSON name (`beers.FieldErrors`), as
    beers and kegs do. Messages never echo the value.
- **Stored values that fail validation** (hand-edited database) read as the default, as
  `importer.loadInterval` does for the sync interval. No error to the TV.
- **Rationale**: FR-005 and the owner-approved defaults (60 chars, 5 to 300 s).

## R6. TV theme without inline styles

- **Decision**: `DisplayPage.css` moves its colors into custom properties on `.display` (dark
  values) and `.display.light` overrides them. `DisplayPage` adds class `light` to `<main>` when
  the theme is light. No `style` attribute (CSP has no `style-src`).
- **Palette** (WCAG contrast computed during planning):

  | Role | Dark (bg `#0b0b0d`, card `#17171b`) | Light (bg `#ecebe6`, card `#ffffff`) |
  | --- | --- | --- |
  | text | `#f2f2f2` (16.0:1 on card) | `#141416` (15.4:1 on bg) |
  | label, heading, nothing | `#a9a9b0` (7.7:1) | `#55555d` (6.2:1) |
  | style, facts | `#d2d2d8` (11.9:1) | `#33333a` (10.5:1) |
  | status | `#e0b04a` (8.9:1) | `#7a4f00` (6.0:1) |
  | pager | `#8c8c95` (5.9:1) | `#5f5f68` (5.3:1) |
  | offline | `#fff` on `#8b1e1e` (9.1:1) | same |

  The dark pager today is `#6f6f78` (3.95:1), below the 4.5:1 that FR-011 now requires; it
  becomes `#8c8c95`. Every other dark value is unchanged, so the dark TV looks the same
  (SC-003). BeerArt's outline `#8a8a8a` is 3.45:1 on the white card, above the 3:1 for
  graphics; the art sits on the card, so no BeerArt change.
- **Alternatives**: `prefers-color-scheme` (the TV cannot be told); a second stylesheet (two
  sheets to keep in step).

## R7. Title on the TV

- **Decision**: when the title is non-empty, an `<h1 className="title">` sits at the top of every
  page, one line, ellipsis on overflow, `5vh` bold (between the `4vh` on-deck heading and the
  `5.2vh` beer name, so it reads as the page name). Empty title renders no
  element, so the cards keep their space (US1 scenario 6). Plain text through React, never
  markup.
- **Rationale**: readable at 3 m (FR-009); the cards are a `flex: 1` row, so a `flex: none` title
  row takes only its own height.

## R8. Rotation interval on the TV

- **Decision**: `PAGE_MS` becomes `pageSeconds * 1000` from the last good menu's `display`
  (default 15 when absent or invalid). The page-turn effect depends on `[rotating, pageMs]`, so a
  new interval restarts the timer: the current page then holds for the new interval and later
  turns follow it ("at the latest the next turn", edge case). A menu update with the same
  interval does not restart it.
- **Validation on the TV**: `isMenu` stays strict for taps and on-deck. `display` is checked
  separately and field by field: a missing, malformed or out-of-range `display` value falls back
  to its default (no title, dark, 15 s) instead of failing the whole check, so an older API
  (during an upgrade) still draws the menu.

## R9. Units in the admin app

- **Decision**: `web/src/units.ts` gains `type Units = "us" | "metric"` and a two-row table:

  | Units | Volume (size, level) | Volume input | Pour input | Pour prefill |
  | --- | --- | --- | --- | --- |
  | us | `"5.00 gal"` | US gal, 2 decimals | US fl oz | 16 |
  | metric | `"18.93 L"` | litres, 2 decimals | mL | 500 |

  Functions take `units`: `volumeText(l, units)` (input text), `formatVolume(l, units)` (text with
  unit), `volumeToLitres(n, units)`, `pourToLitres(n, units)`, plus labels. The gallon-specific
  exports go once no caller remains. Conversion to litres still rounds to 0.01 (the API keeps 2
  decimals), so a metric value round-trips exactly.
- **Wiring**: `AdminPage` holds `units` (default `"us"`). `SettingsPanel` loads
  `GET /api/settings` and reports the loaded and each saved value through `onChange`, so the page
  makes one settings read. `units` is passed to `TapsPanel` (and its `KegForm`) and `KegHistory`.
  `KegForm` is keyed by units, so a form open during a units change restarts in the new units
  rather than converting old text with the new factor.
- **500 mL pour prefill**: a common metric serving; the US prefill stays 16 fl oz.
- **Alternatives**: React context (the codebase passes props; two consumers); each panel fetching
  settings itself (three reads per page load; the project already notes repeated keg-type reads).

## R10. Settings panel

- **Decision**: `SettingsPanel` (admin, unstyled like the rest): title text input, theme select
  (Dark, Light), units select (US, Metric), page seconds number input (min 5, max 300, step 1).
  Save sends only changed fields; with no change it does nothing. Field errors show beside their
  field; a 401 calls `onSignedOut`. Placed after Brewfather, before Keg history.

## R11. Dependencies and versions

- **Decision**: no new Go or web dependency. `api/openapi.yaml` `info.version` 0.6.0 → 0.7.0.
  Proposed release `v0.6.0`.
