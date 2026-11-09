# Research: TV Display

Decisions for feature 5. Each has the decision, why, and what else was considered.

## R1. Live updates: polling every 10 seconds

- **Decision**: the display calls `GET /api/menu` every 10 seconds with a `setTimeout` chain (the
  next call is scheduled only after the previous one settles). Each call has an 8-second abort
  timeout. A failure waits the same 10 seconds and tries again.
- **Why**: a 10-second interval meets FR-020 (a change shows within 30 s) and FR-021 (retry at
  most every 30 s) with room to spare. A failed call costs nothing special: server restarts, Wi-Fi
  drops and 502s from nginx while the API is down are all just "try again". No nginx or NPM
  change. With one TV, 8,640 small reads a day is negligible for SQLite.
- **Alternatives**: server-sent events: updates in under a second, but nginx needs
  `proxy_buffering off` and longer read timeouts, NPM needs the same, reconnect logic is still
  needed for restarts, and a long-lived connection is one more thing to hold open on a Fire TV
  Stick for days. Rejected under constitution VII; can be added later without breaking the menu
  read. `setInterval`: can stack calls when the server is slow; rejected. Exponential backoff:
  would push retries past 30 s; not needed for one client.

## R2. Offline notice and daily reload without the TV clock

- **Decision**: the display counts ticks, not time. It shows the offline notice after 12
  failed checks in a row (12 × 10 s = 2 minutes, FR-022) and clears it on the next good check. It
  reloads the page (`window.location.reload()`) on the first good check after 8,640 ticks
  (about 24 hours, FR-024). The reload is passed in as a prop with that default, so tests swap
  it.
- **Why**: a tick is at most 18 s (8 s timeout plus 10 s wait), so the notice appears after 2
  to 3.6 minutes offline, within FR-022's "after 2 minutes", and retries stay under 30 s. The
  spec says nothing may depend on the TV clock, and Fire TV clocks drift or jump
  after network loss. Ticks are monotonic. Reloading only after a good check means the server
  is up, so the browser fetches the new page instead of landing on an error page. A reload
  revalidates `index.html` with the server, and Vite asset names are content-hashed, so a server
  upgrade is picked up with no nginx cache change.
- **Alternatives**: compare `version` from `/api/health` and reload on change: faster pickup of
  upgrades, but one more request per tick and one more moving part; the daily reload already
  meets FR-024. `Date.now()` arithmetic: depends on the TV clock; rejected.

## R3. Menu query lives in the `kegs` package

- **Decision**: add `kegs.GetMenu(db) (Menu, error)` in `api/internal/kegs/menu.go`, with its own
  response structs, built from three plain `SELECT`s with no transaction (taps with current keg
  and beer; waiting kegs with beer; on-deck beers).
  The handler `getMenu` in `httpapi/menu.go` writes it as JSON.
- **Why**: the package already owns taps, kegs and the keg type table (`TypeByKey` gives the
  shape) and imports `beers` for `ColorFor`. Separate structs, not reused `Tap`/`Keg`/`Beer`, make
  it impossible to leak a field by adding it to a credentialed type later (FR-004). Three
  queries are simpler to read than one UNION; a menu made of slightly different instants is
  harmless because the next check 10 s later corrects it. SQLite WAL readers never block writers.
- **Alternatives**: reuse `ListTaps` and `ListKegs` and filter in Go: drags in ids, sizes and
  timestamps that must then be stripped, and `ListTaps` returns disabled taps. A new `menu`
  package: one more package for one function; rejected.

## R4. Level as a fraction, computed on the server

- **Decision**: the menu gives `level = remaining_l / size_l`, clamped to 0..1, not rounded.
  Sizes and remaining volumes are not in the menu (clarification 2026-11-09).
- **Why**: the TV only needs the fill. A fraction exposes less than litres and needs no unit
  conversion on the client. Not rounding keeps "any beer left" distinguishable from 0. Size is
  always greater than 0 (feature 4 FR-012), so the division is safe; the clamp guards against
  float noise.
- **Alternatives**: send `size_l` and `remaining_l` as first planned: more data than the TV shows;
  rejected by the owner.

## R5. `BeerArt` level prop: computed liquid shape, no clip path

- **Decision**: `BeerArt` gains `level?: number` (0..1). With no level, nothing changes. With a
  level, the liquid shape is computed: the glass liquid is the trapezoid from the base (y 90) up
  to `90 - fill × 68`, with the side walls interpolated along the taper; each keg's liquid rect
  keeps its outline (drawn unfilled at full size) and a filled rect of `fill × height` sits on
  its bottom. `fill = level > 0 ? max(level, 0.02) : 0`, so any beer left shows a sliver. The
  glass foam band rides on top of the liquid when `fill > 0` and is not drawn when empty. The
  `aria-label` becomes `"<name>, <vessel>, N% left"`, where N is `Math.round(level × 100)`
  but at least 1 when level > 0 and at most 99 when level < 1.
- **Why**: plain computed geometry has no ids, so many SVGs on one page cannot collide (a
  `clipPath` would need `useId`). Presentation attributes only, so the CSP stays as is.
- **Alternatives**: `clipPath` with `useId`: equally small, but id collisions and clip support
  on old WebViews are one more risk. CSS `transform: scaleY`: needs style; rejected.

## R6. Display layout: grid sized by count class, one stylesheet

- **Decision**: add `web/src/pages/DisplayPage.css`, imported by `DisplayPage.tsx`; it is the
  web app's first stylesheet. Vite emits it as a separate file linked from `index.html`, which
  the CSP allows (`default-src 'self'` covers styles). The page sets a class from the number of
  taps: `taps-s` (1 to 4: one row), `taps-m` (5 to 8: 4 columns, 2 rows), `taps-l` (9 to 12: 4
  columns, 3 rows), `taps-xl` (13+: 6 columns, rows as needed, smaller text). Sizes use `vh` so
  a 1080-pixel-high screen gets beer names of about 56 px for `taps-s` and `taps-m` (FR-010),
  and never less than 24 px for any text up to `taps-l`. Names and styles clamp at two lines
  with `-webkit-line-clamp`. On deck is a single bottom strip of equal-width items, hidden when
  empty, showing at most 8 items then "+N more". The stylesheet loads with the whole bundle, so
  every selector sits under `.display`.
- **Why**: classes keep inline styles out (FR-016). `vh` sizing makes the TV and a laptop
  preview look the same. Four buckets cover a home kegerator.
- **Alternatives**: CSS custom properties set inline: needs `style=`; rejected. A CSS framework:
  new dependency for one page; rejected.

## R7. Disabled, empty and on-deck rules in SQL

- **Decision**:
  - Taps: `WHERE enabled = 1 ORDER BY position`, left join the current keg
    (`kicked_at IS NULL`) and its beer.
  - Waiting kegs: `tap_id IS NULL AND kicked_at IS NULL`, ordered by `created_at, id`.
  - On-deck beers: `archived_at IS NULL AND status IN ('conditioning','fermenting')` and no
    current keg (`NOT EXISTS` a keg with `kicked_at IS NULL`), ordered conditioning first, then
    by name (`COLLATE NOCASE`), then id.
  - A keg on a disabled tap is in neither list (it is not waiting).
- **Why**: matches spec FR-002 to FR-005. Kegs cannot be of archived beers (feature 4 FR-030),
  so the keg queries need no archive filter, but the tap and waiting-keg queries add
  `b.archived_at IS NULL` anyway as a cheap guard for FR-004.

## R8. Caching and method handling

- **Decision**: no change. `withSecurity` already sets `Cache-Control: no-store` on every `/api`
  response except health (FR-006). The mux pattern `GET /api/menu` answers other methods with
  405, which a test pins (FR-041). The display relies on that header; `apiFetch` gets no cache option.
- **Why**: `no-store` binds the proxy and the WebView cache alike; one place to set it.

## R9. Fetch helper on the display

- **Decision**: the display uses `apiFetch` from `web/src/api.ts` for the menu call, with an
  `AbortSignal` added as an optional argument (an `AbortController` with a `setTimeout`, not
  `AbortSignal.timeout`, so fake timers in tests drive it and older WebViews support it), and treats any
  thrown error, including a body that is not a menu (missing `taps` or `on_deck` arrays), as a
  failed check.
- **Why**: reuse; one place that parses error bodies.
- **Alternatives**: raw `fetch` in the page: duplicates error handling.

## R10. Fire TV WebView support

- **Decision**: no build target change. Vite 8's default target is Baseline widely available
  (Chromium 107+). Fully Kiosk uses Android System WebView, updated through the Amazon
  Appstore on current Fire OS. Quickstart step 7 checks the real TV; if its WebView is older,
  set `build.target` in `vite.config.ts` then (a config change, not a dependency).
- **Why**: avoid a guess; verify on the device.

## R11. Public endpoint risk

- **Decision**: no rate limit on `GET /api/menu`.
- **Why**: LAN only behind NPM; three indexed reads over tens of rows; same exposure class as
  `/api/health`. The response has no ids, timestamps, sizes or text beyond what the TV shows, so
  scraping it learns nothing a guest cannot read off the TV.

## R12. Page rotation (clarification 2026-11-09)

- **Decision**: the display builds a list of pages from the menu: tap cards in chunks of 8,
  then on-deck items in chunks of 8 under an "On deck" heading. With more than one page, a
  separate `setTimeout` chain advances the page every `PAGE_MS = 15_000` (ticks, not the TV
  clock) and shows "N / M" in a corner. The page index survives menu updates and goes back
  to page 1 when it is past the last page (an update that leaves fewer pages); index mod page
  count was dropped because a page count change then jumps to another page that still exists. Polling and rotation are independent
  timers. Layout classes are `taps-s` (1 to 4 cards on the page) and `taps-m` (5 to 8);
  `taps-l` and `taps-xl` go. The bottom on-deck strip and `ON_DECK_MAX` "+N more" go: on-deck
  items use the same card grid on their own page, drawn as kegs.
- **Why**: 12 cards on 16:9 makes each glass too small to read as a gauge from 3 m (seen in
  headless Firefox at 1920×1080). 8 per page keeps the 4 × 2 layout, the largest that fits
  most kegerators, and a full cycle stays short (16 taps plus on deck = 3 pages = 45 s).
- **Alternatives**: measure the screen and fit as many as possible: more code for one TV;
  rejected for now (`shortcut: fixed 8 per page, tuned for 16:9; measure if other screens
  matter`). An aspect-ratio or cards-per-page setting: the browser already knows the screen;
  rejected. An interval setting: feature 7 (owner decision).
