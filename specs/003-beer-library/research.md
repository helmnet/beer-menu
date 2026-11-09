# Research: Beer Library

Phase 0 decisions for [plan.md](./plan.md). Each entry gives the decision, why, and what was
rejected. Requirement IDs refer to [spec.md](./spec.md).

## R1. Endpoints

- **Decision**: five beer operations plus one color-table read.

  | Method | Path | x-access | Purpose |
  | --- | --- | --- | --- |
  | GET | `/api/beers` | read | List current beers; `?archived=true` lists only archived ones (FR-010, FR-011) |
  | POST | `/api/beers` | write | Create a beer (201) |
  | GET | `/api/beers/{id}` | read | Read one beer, archived or not (FR-012) |
  | PATCH | `/api/beers/{id}` | write | Edit fields, archive and restore (FR-022, FR-023) |
  | DELETE | `/api/beers/{id}` | write | Delete permanently (204) |
  | GET | `/api/srm-colors` | read | The SRM color table (R6) |

- **Archive and restore** are `PATCH {"archived": true|false}`, not separate routes. Archiving
  sets `archived_at` to now only if it is empty; restoring clears it. Both are idempotent
  (FR-023).
- **Rationale**: the full library is not a menu read (owner decision 2026-11-09): it holds
  planned and archived beers the TV never shows. GETs use `AccessRead`, so a session, a `read`
  token or a `write` token reads them and no credential gets 401. Feature 5 adds a public
  `GET /api/menu` with only what the TV shows. Writes use `AccessWrite`, so a `write` token
  (Home Assistant, scripts) works like a session and a `read` token gets 403. A field on PATCH
  is one fewer route than `POST .../archive` + `POST .../restore`, and is easy to explain in
  the contract.
- **Alternatives**: `PUT` full replace (clients must resend every field, and clearing versus
  keeping is ambiguous for older clients after fields are added); `?archived=all` (no caller
  needs current and archived together).

## R2. Partial updates (FR-022)

- **Decision**: PATCH uses JSON merge-patch semantics (RFC 7396) without the media type: a field
  that is absent is kept, `null` clears an optional field, a value sets it. In Go each PATCH
  field is a small generic `Optional[T]` with `Set bool` and `Value *T`, whose `UnmarshalJSON`
  marks it set (the decoder calls `UnmarshalJSON` for an explicit `null` on non-pointer types).
  `name`, `status` and `archived` refuse `null`.
- **Rationale**: the stdlib decoder cannot tell absent from `null` with plain pointers. A
  twenty-line type keeps `DisallowUnknownFields` and typed validation.
- **Alternatives**: decoding into `map[string]json.RawMessage` (loses typed decoding and the
  unknown-field check); a JSON Patch library (new dependency).

## R3. Source fields from clients (spec edge case)

- **Decision**: refused. `source` and `source_ref` are not in the create or PATCH request
  schemas, so `DisallowUnknownFields` answers `400 invalid_request`. Responses always include
  them.
- **Rationale**: silent ignoring hides client bugs; refusing needs no code.

## R4. Validation (FR-002 to FR-005, FR-025)

- **Decision**:
  - Text is trimmed; an empty optional text field becomes `null`. Lengths count Unicode code
    points: name 1 to 100, style and brewer up to 100, description up to 2000. Name, style and
    brewer refuse control characters; description allows newline and tab only.
  - `abv`: number 0 to 25 with at most one decimal place. `srm`: number 0 to 100 with at most
    one decimal place. `ibu`: integer 0 to 200. One decimal is checked as
    `|x·10 − round(x·10)| < 1e-9`.
  - `brew_date`: `YYYY-MM-DD`, parsed with `time.Parse("2006-01-02")`. Future dates allowed.
  - `status`: one of `planned`, `fermenting`, `conditioning`, `ready`; default `ready`.
  - A validation failure answers `400 invalid_request` with an extra `fields` object mapping
    each bad field to a message, and changes nothing. The base `{"error","message"}` shape is
    unchanged for existing clients.
- **Rationale**: integer IBU matches how menus show it; one decimal fits ABV and SRM readings.
  The `fields` map lets the form show errors by field (FR-041).
- **Alternatives**: decimal IBU (feature 6 rounds Brewfather's value instead); first-error-only
  responses (forms then fix one field per round trip).

## R5. Request body size

- **Decision**: `decodeJSON` takes the limit as an argument. Beer writes allow 16 KiB; existing
  routes keep 8 KiB. The error message no longer names a size.
- **Rationale**: a 2000-code-point description can need about 12 KB as JSON in the worst case
  (control-character escapes are 6 bytes each), plus the other fields. 16 KiB covers it with
  room; raising every route's cap is not needed.
- **Alternatives**: one global 16 KiB cap (loosens auth routes for no reason).

## R6. SRM color table (FR-013, FR-030)

- **Decision**: the API owns one table of 40 sRGB hex colors for SRM 1 to 40, the table
  published widely in brewing tools. The implementing task copies it from a public source and
  cites the URL in a comment above the table. Mapping: round SRM to the nearest integer, clamp
  to 1..40, look up. So SRM 0 shows the palest color and SRM 60 the darkest (spec edge cases).
  - Every beer read includes `color` (`"#rrggbb"`, or `null` when `srm` is `null`).
  - `GET /api/srm-colors` (session or any token) returns the 40 colors, so the admin form's
    live preview (FR-042) uses the same table before the beer is saved.
- **Rationale**: the web and API images build from separate contexts (`./api`, `./web`), so a
  shared file is not possible without changing the release pipeline. One table in the API plus
  a read keeps a single definition and serves every later client (principle I).
- **Accepted duplication**: the table is defined once, in Go. The round-and-clamp rule is
  written twice, as `ColorFor` in Go and `colorForSrm` in TypeScript (for the form preview).
  Both are tested over SRM 0 to 40 plus 4.4, 4.5, 60 and 100 against the same expectations, so
  drift fails a test.
- **Alternatives**: a copy of the table in TypeScript (two definitions drift); an approximation
  formula (no "standard table" to cite, and the owner compares against taplist.io colors);
  interpolating decimals (not visible at display size).

## R7. Artwork (FR-031 to FR-037)

- **Decision**: one React component file `BeerArt.tsx` exporting `BeerArt`, which draws inline
  SVG: a shaker pint glass for a beer on a tap, a 5 gallon corny keg otherwise. Props: `onTap`
  (optional, default false), `color` (`string | null`), `name` (for the text alternative),
  `height`. Status does not pick the vessel (owner decision 2026-11-09). Taps arrive in feature
  4, which passes `onTap`; until then no caller sets it, so every beer is a keg. The SVG
  scales by `viewBox`, so the admin list (about 64 px) and the TV (feature 5) use the same
  component.
  - The liquid area takes `fill={color}`; with `color === null` it gets no fill (outline only).
  - Colors are set with SVG presentation attributes (`fill`, `stroke`), never inline `style`
    text, so nginx's `default-src 'self'` CSP needs no change.
  - `role="img"` and `aria-label` such as "Honey IPA, pint glass" (FR-037).
  - Shapes are drawn from scratch in a few paths (glass: tapered body, foam band; keg: body,
    top and bottom rubber bands, handle cutouts, with the whole body as the liquid). Nothing is
    traced from taplist.io or other products (FR-036).
- **Rationale**: inline SVG makes no network request (SC-005, FR-034), scales sharply on a 1080p
  TV, and is testable by reading the `fill` attribute.
- **Alternatives**: static image files recolored by CSS filters (inexact colors); canvas
  (harder to test, not accessible); an icon library (new dependency, not original).

## R8. Storage

- **Decision**: migration `0003_beers.sql` adds one `beers` table (see
  [data-model.md](./data-model.md)). Times are `INTEGER` Unix seconds like feature 2;
  `brew_date` is `TEXT` `YYYY-MM-DD` because it is a calendar date without a time zone. Enum
  columns carry `CHECK` constraints. No extra index: 500 rows sort in memory well inside SC-006.
- **Sort order**: `ORDER BY name COLLATE NOCASE, id`. SQLite's `NOCASE` folds only A to Z,
  which FR-014 accepts; Unicode folding would need sorting in Go for no real gain here.
- **Rationale**: matches existing conventions; nothing speculative for features 4 or 6 (they
  add their own columns and indexes).

## R9. Code layout

- **Decision**: a new `api/internal/beers` package holds the model, validation, SQL and the
  color table, with no HTTP, like `internal/auth`. `internal/httpapi/beers.go` holds the
  handlers and adds rows to `Routes`.
- **Rationale**: keeps `httpapi` thin and lets feature 6's importer reuse the same create and
  update code.

## R10. Admin app

- **Decision**: no new web dependencies and no router. New components:
  - `BeersPanel` — list of current beers (artwork, name, style, ABV, status), "Show archived"
    toggle, add button, and per-beer edit, archive or restore, and delete with
    `window.confirm` (FR-040, FR-043).
  - `BeerForm` — add and edit form with field errors from the `fields` map and a live `BeerArt`
    preview fed by the color table from `/api/srm-colors` (FR-041, FR-042). The edit form sends
    only changed fields; clearing an input sends `null`.
  - `BeerArt` — R7.
  - `BeersPanel` takes `onSignedOut` like the feature 2 panels (FR-044).
- **Rationale**: follows feature 2's panel pattern in `AdminPage`.
- **Alternatives**: a modal library or form library (new dependencies for one form).

## R11. Tests

- **Decision**:
  - Go, real SQLite: create, read, list order, archived filter, PATCH keep/set/clear, archive
    and restore idempotence, delete, 404s, every validation rule, unknown fields (`source`)
    refused, 16 KiB limit, color mapping (SRM 0, 1, 4.4, 4.5, 40, 60, `null`).
  - Auth per write route: no credential 401, `read` token 403, revoked token 401, `write` token
    and session succeed, cross-site `Sec-Fetch-Site` 403 (constitution IV). Per read route:
    no credential and revoked token 401; `read` token, `write` token and session succeed. Both
    tests walk `Routes`, so a new beer route is covered and a read can't silently turn public.
  - Migration 0003 applied to a version 2 database holding an admin and a token: both survive.
  - The existing `x-access` contract test covers the new routes.
  - Vitest: `BeerArt` (vessel by `onTap`, fill per color, empty without SRM, label), `BeerForm`
    (validation display, PATCH body has only changed fields and `null` for cleared ones,
    preview follows input), `BeersPanel` (archived toggle, confirm cancel deletes nothing, 401
    calls `onSignedOut`).
