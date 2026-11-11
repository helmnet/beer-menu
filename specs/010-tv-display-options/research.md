# Research: TV Display Options

Feature 10. Each entry: decision, rationale, alternatives considered.

## R1. Storage: more rows in `settings`, no migration

- **Decision**: 11 new keys in the existing `settings` table, read and validated by
  `internal/settings` like the feature 7 keys: `display.layout`, `display.title_align`,
  `display.title_font`, `display.text_font`, `display.show_style`, `display.show_abv`,
  `display.show_ibu`, `display.show_srm`, `display.show_description`, `display.title_color`,
  `display.text_color`. Defaults live in `settings.Default`; a missing or bad stored value reads
  as its default.
- **Rationale**: feature 7 set this pattern; an upgraded database has no rows, so it reads as
  defaults (FR-015, SC-002) with no migration and no backup on upgrade.
- **Alternatives**: one JSON blob key (loses per-field validation on read and per-key merge);
  a new table (migration for no gain).

## R2. API shape: flat fields on the existing routes

- **Decision**: `GET`/`PATCH /api/settings` gain 11 flat fields. Booleans are JSON booleans;
  enums are strings; colors are `""` (theme color) or `#rrggbb`, stored lowercase. `PATCH` keeps
  merge-patch, all-or-nothing, unknown-field and `null` refusal. `GET /api/menu`'s `display`
  gains every new field. Units stay out of `display`.
- **Rationale**: FR-002 to FR-005 ask for exactly the feature 7 behavior; `rawField` already
  handles a typed field per name, so `rawField[bool]` covers the switches.
- **Alternatives**: nested `tv: {…}` object (breaks the flat merge-patch the panel already
  sends); a separate public route (the TV already polls `/api/menu`).

## R3. Colors: `#rrggbb` only, no contrast check

- **Decision**: a color is `""` or matches `^#[0-9a-fA-F]{6}$`, normalized to lowercase. No
  contrast check (owner decision, Clarifications Q4).
- **Rationale**: `<input type="color">` produces exactly `#rrggbb`. The narrow pattern also means
  a stored value can never carry CSS syntax into the TV's style (V: validate at the boundary).
- **Alternatives**: any CSS color (`rgb()`, names): wider input, needs a CSS parser to stay safe.

## R4. Applying colors under the CSP

- **Decision**: the TV root sets custom properties through React's `style` prop
  (`--title-color`, `--tv-text`), which React writes with `style.setProperty` (CSSOM). The
  stylesheet reads them with fallbacks: `color: var(--title-color, var(--text))`; the text color
  overrides `--text`, `--muted`, `--soft` and the status color inside cards, rows, the "On deck"
  heading and the "Nothing on tap" / "Nothing on deck" lines. `--bg`, `--card`, the pager,
  offline notice and art colors are untouched (FR-019).
- **Rationale**: the CSP (`default-src 'self'`, no `style-src`) blocks `style` attributes in
  markup and `<style>` blocks, but not CSSOM writes from a same-origin script. The values are
  validated hex (R3), so nothing else can reach the style. Proven in a real browser behind the
  real nginx CSP (quickstart step 6), not just jsdom.
- **Alternatives**: a class per color (impossible for a free picker); adding
  `'unsafe-inline'` to `style-src` (weakens the CSP for every page).
- **Note**: this narrows the feature 5 "no inline styles" rule to "no style attributes in
  markup". `DisplayPage`'s doc comment is updated to say so.

## R5. Fonts: 7 `@fontsource` packages, Latin subset, served by nginx

- **Decision**: today's `system-ui` stack (key `system`, the default) plus 7 fonts from
  `@fontsource`, version 5.3.0, Latin subset, weights 400 and 700 where the font has both:

  | Key | Font | Style | License |
  | --- | --- | --- | --- |
  | `system` | system-ui, sans-serif | today's | — |
  | `inter` | Inter | sans | OFL-1.1 |
  | `lora` | Lora | serif | OFL-1.1 |
  | `roboto-slab` | Roboto Slab | slab serif | Apache-2.0 |
  | `oswald` | Oswald | condensed sans | OFL-1.1 |
  | `nunito` | Nunito | rounded sans | OFL-1.1 |
  | `bebas-neue` | Bebas Neue | display, caps (400 only) | OFL-1.1 |
  | `pacifico` | Pacifico | script (400 only) | OFL-1.1 |

  One `web/src/fonts.ts` imports the `latin-400.css` / `latin-700.css` files and maps each key
  to a `font-family` class (`font-inter`, …). Vite copies the `.woff2` files into the build, so
  nginx serves them from the app's own origin (FR-011, SC-005).
- **Rationale**: the owner asked for 6 to 8 shipped fonts. `@font-face` files download only when
  text uses that family, so the TV fetches at most two fonts. `@fontsource` pins versions, ships
  license files and gets Dependabot updates; vendoring `.woff2` by hand gives none of that. The
  packages are data only (CSS + woff2), no runtime code. Adds roughly 300–400 KB of woff2 to the
  web image, none loaded unless picked. This is the feature's new dependency, justified per VII.
- **Alternatives**: hand-vendored woff2 (no updates, license tracking by hand); Google Fonts CDN
  (breaks `'self'` CSP and offline LAN use); device fonts only (owner rejected).
- **Fallback**: every font class ends in today's stack, so a font that fails to load falls back
  to it (edge case).

## R6. One drawing for TV and preview: `TvView`

- **Decision**: split `DisplayPage` into:
  - `components/TvView.tsx`: pure drawing. Props: menu, display settings, page index. Renders
    title, the layout's page, pager, and offline notice. No timers, no fetch.
  - `tv.ts`: `displayOf` (extended, R2 fields with defaults), `pagesOf(menu, layout)` (R7) and
    a `usePager(count, pageMs)` hook with the existing turn/restart rules.
  - `pages/DisplayPage.tsx`: polling, offline count, daily reload, as today; renders `TvView`.
- **Rationale**: FR-017 and SC-008 need the preview to match the TV; one component makes drift
  impossible. Existing `DisplayPage` tests keep passing as the behavior contract.
- **Alternatives**: an `<iframe src="/display?preview=…">` (blocked by `frame-ancestors 'none'`,
  and settings would go through the URL); a second drawing for the preview (drifts).

## R7. Layouts and paging

- **Decision**: `pagesOf` builds pages per layout:
  - `grid`: today's code unchanged: taps in chunks of 8, then on deck in chunks of 8.
  - `list`: taps in chunks of 6 rows, then on deck in chunks of 6 under "On deck".
  - `columns` (side by side): page *i* holds tap chunk *i* (6) and on deck chunk *i* (6); page
    count is the longer column's; the shorter column is blank after its last row; an empty on
    deck list shows "Nothing on deck" in its column on every page.

  A row (list and columns) is a CSS grid: small art cell (fixed width, fixed height, same for
  every row so the gauge reads the same), then label, name and details on one or two lines.
  `ROWS_PER_PAGE = 6` with the `shortcut:` comment rule, tuned by the screenshots in quickstart.
- **Rationale**: Clarifications Q2 and the row-content answer. Keeping `grid` byte-for-byte the
  same is what SC-002 checks.
- **Alternatives**: fit-to-screen row count (owner rejected); separate column paging (rejected).

## R8. Scaling for the preview: size unit variables

- **Decision**: every `vh`/`vw` size in `DisplayPage.css` (25 uses) becomes
  `calc(N * var(--vh))` / `calc(N * var(--vw))`. `.display` sets `--vh: 1vh; --vw: 1vw`. The
  preview wrapper is a 16:9 box (`aspect-ratio: 16 / 9; container-type: size`) whose `.display`
  sets `--vh: 1cqh; --vw: 1cqw` and `position: absolute` instead of `fixed`.
- **Rationale**: the TV keeps `vh` exactly, so Fire TV WebView support is unchanged; only the
  admin page (a modern browser) uses container units. Scaling with `transform` would not help:
  `vh` would still be the admin window's.
- **Alternatives**: rewrite everything in `cqh` (risk on older Fire TV WebView); `transform:
  scale()` on a 1920×1080 box (vh still wrong).

## R9. Menu carries SRM and description for tap beers

- **Decision**: `MenuBeer` on taps gains `srm` and `description`, each omitted when unset. On deck beers do not get them (`omitempty` pointer fields, set only in
  the taps query). The TV shows each only when its switch is on. Spec FR-006 amended.
- **Rationale**: the preview must show a detail before it is saved; sending them only when the
  saved switch is on would leave the preview blank. Both are text the owner writes for display,
  the description is capped at 2,000 characters, and no other beer field becomes public.
- **Alternatives**: a credentialed preview route (a new route, auth walk fixtures and a second
  menu shape, for data meant for the TV anyway).

## R10. Details: SRM text and description clamp

- **Decision**: SRM shows as `SRM N`, `Math.round`. The description uses
  `-webkit-line-clamp: 2` with `overflow: hidden` inside the card's fixed text block, so it
  ends in "…" and never changes the card size. Description text is a React text node (no
  markup).
- **Rationale**: Clarifications Q1. `line-clamp` with the `-webkit-box` form is supported in
  every WebView the TV could run.

## R11. Admin form and preview

- **Decision**: `SettingsPanel` keeps its card and adds fieldsets: Layout (select), Title
  (alignment select, font select, color), Text (font select, color), Tap details (5
  checkboxes). Color is `<input type="color">` plus a Reset button that sets `""`; when `""` the
  picker shows the theme's text color and a "Theme color" note. Save sends only changed fields,
  as today. The preview sits beside the form on wide screens and above it on phones, renders
  `TvView` with the draft values (invalid draft fields fall back to the last valid value), the
  menu from one `GET /api/menu` on mount, and `usePager` plus Previous and Next buttons. If the
  menu read fails it uses a fixed sample menu from `tv.ts`.
- **Rationale**: FR-016 to FR-018. Draft values render on the next React render, well within
  1 second. Imports `fonts.ts`, so the preview loads the same fonts.
- **Alternatives**: a separate preview page (loses side-by-side editing).
