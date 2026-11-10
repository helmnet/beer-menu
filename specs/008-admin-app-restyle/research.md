# Research: Admin App Restyle

Feature 8 decisions. The owner skipped `/speckit-clarify` and accepted every default listed in
[checklists/requirements.md](./checklists/requirements.md). Facts about the current code were
checked on `58502d3`.

## R1. Styling: hand-written CSS with custom properties, no CSS or component library

- **Decision**: one new stylesheet, `web/src/pages/AdminPage.css`, imported by `AdminPage`.
  Every rule is scoped under the admin root class `.admin`, the way every rule in
  `DisplayPage.css` is scoped under `.display`. Colors, spacing, radii and the nav width are CSS
  custom properties on `.admin`. The only selector outside `.admin` is
  `body:has(> #root > .admin)` for the page margin and background, which cannot match on
  `/display`.
- **Why**: Vite bundles every imported stylesheet into one CSS file that loads on both `/` and
  `/display`, so scoping is the only thing keeping the two apart (spec FR-014, SC-003). The
  layout is small: a side panel, a top bar, cards, buttons, form fields, a dialog, a list box.
  About 400 lines of CSS cover it with no new dependency (constitution VII).
- **Alternatives rejected**:
  - Tailwind: needs a build plugin and new dev dependencies, and puts utility classes on every
    element of every panel. More churn than the CSS it saves.
  - MUI, Chakra, Mantine (CSS-in-JS): inject `<style>` elements at runtime, which the CSP
    (`default-src 'self'`, no `style-src`) blocks without a nonce. Heavy for nine pages.
  - Radix or Headless UI primitives: their dialog and scroll lock set `style` attributes and
    inject `<style>` tags; same CSP problem, plus several transitive packages.
  - Pico CSS (classless): its global element selectors would restyle `/display`; the scoped
    `pico.conditional` build still needs overriding for the side panel and royal blue theme.

## R2. Icons: `lucide-react` 1.61.0

- **Decision**: add `lucide-react` pinned at exactly `1.61.0` as a runtime dependency. Import
  icons by name (tree-shaken, so only the ~19 used ship).
- **Justification (constitution VII)**: the brief needs an icon on every side panel item (3
  groups, 9 pages) plus menu, close, sign out and three theme icons. Drawing and maintaining 19
  consistent line icons by hand is more code and design work than one small, widely used
  package. `lucide-react` has no dependencies of its own (checked in the 1.61.0 tarball), an ISC license, and peer-supports React
  18. Icons render as SVG with presentation attributes (`stroke`, `width`), no `style`
  attribute, so they work under the CSP like `BeerArt` does.
- **Release age**: `1.61.0` was published 2026-10-21, 20 days before this plan (2026-11-10). It
  is the newest release at least two weeks old (1.62.0 onward are newer than 2026-10-27).
- **Check during implementation**: render one icon in a test and assert the SVG has no `style`
  attribute.
- **Alternatives rejected**: hand-drawn SVG (more code, uneven look); emoji (render differently
  per device, poor screen-reader names); an icon font (needs a font file and CSS
  `::before`, harder to label).

## R3. Dialogs: native `<dialog>` with `showModal()`

- **Decision**: a small `Modal` component wraps a native `<dialog>`. It calls `showModal()` on
  mount and `close()` on unmount, labels the dialog by its heading (`aria-labelledby`), turns
  the `cancel` event (Escape) into `onClose`, and returns focus to the element that was focused
  when it opened.
- **Why**: `showModal()` gives the top layer, an inert page behind (so keyboard focus stays in
  the dialog, FR-020), Escape handling and the implicit `dialog` role with no library and no
  `style` attributes. Supported by every current browser, including Firefox used in the gate.
- **Testing**: jsdom 30.1.3 has `HTMLDialogElement` but no `showModal` or `close` (checked:
  `x.showModal is not a function`). A Vitest setup file (`web/src/test/setup.ts`, wired with
  `test.setupFiles`) adds both as stubs that toggle the `open` attribute. Component tests find
  dialogs with `getByRole("dialog", { name })` and close them by firing `cancel`. The real
  Escape key, focus trap and focus return are checked in the browser gate (quickstart §4).
- **Alternatives rejected**: a hand-rolled `div role="dialog"` with a focus trap (more code,
  easy to get wrong); a library dialog (R1).

## R4. Navigation: state in `AdminPage`, no router

- **Decision**: a constant page tree (group, page, label, icon) and two pieces of state in
  `AdminPage`: the current page (default `taps`) and the open group. Selecting a group opens it
  and closes the other; selecting a page also opens its group. Only the current page's
  component is rendered.
- **Why**: the owner accepted "reload returns to Taps, Back is not used for in-app
  navigation", so a router adds nothing. Rendering only the current page means each page
  fetches fresh data when shown.
- **Consequences found in the code**:
  - Volume units come from `SettingsPanel`'s `onChange` today. With Display Settings not mounted
    on the Taps page, units would never load. `AdminPage` therefore reads `GET /api/settings`
    once after sign-in (a 401 returns to sign-in, any other failure keeps US) and still takes
    `SettingsPanel`'s `onChange`.
  - The `kegChanges` and `beerChanges` counters exist so that panels mounted side by side
    refresh each other. With one page mounted at a time they are not needed: remove them and
    `BeersPanel`'s `refresh` prop, and keep `onKegsChanged`/`onBeersChanged` only where a page
    still needs them (none after the split).
  - The app version is read once from `GET /api/health` (public, already used by Docker). A
    failure or empty `version` shows "unknown" (spec edge case).

## R5. On Deck from credentialed lists, matching the TV order

- **Decision**: the On Deck page loads `GET /api/kegs` (current kegs) and `GET /api/beers`
  (current beers) and builds the list in the browser:
  1. waiting kegs (current kegs with `tap_id === null`), in API order, which is oldest first
     (`kegs.ListKegs`: `ORDER BY k.created_at, k.id`);
  2. beers with status `conditioning`, then `fermenting`, that have no current keg, each by name
     ignoring case, then id.
  This is the order of `GetMenu` in `api/internal/kegs/menu.go`
  (`ORDER BY b.status = 'fermenting', b.name COLLATE NOCASE, b.id`).
- **Why not `GET /api/menu`**: its `on_deck` items carry no ids, so the admin page could not
  edit, kick or put a waiting keg on tap. The credentialed lists have ids and the same data.
- **Same items as the TV**: the menu drops waiting kegs of archived beers, but a beer with a
  current keg cannot be archived (`beers.ErrHasCurrentKeg`, 409 `beer_has_keg`), and
  `GET /api/beers` without `archived` returns only current beers, so both sources give the same
  set. Name comparison uses `localeCompare` with `sensitivity: "base"`; SQLite `NOCASE` folds
  ASCII only, so names differing only in non-ASCII case may sort differently. Accepted: cosmetic.
- **Waiting-keg actions** move from `TapsPanel`'s on-deck list to this page (pour, set level,
  kick, edit keg), shared through an extracted `KegCard`. Taps keeps "Put on tap" for an empty
  tap, which needs the waiting list too (it already loads `GET /api/kegs?state=waiting`).

## R6. Searchable beer picker: hand-written ARIA combobox

- **Decision**: a `BeerPicker` component following the WAI-ARIA 1.2 combobox pattern (text
  input `role="combobox"` with `aria-expanded`, `aria-controls`, `aria-activedescendant`; a
  `role="listbox"` of `role="option"`). Arrow keys move, Enter picks, Escape closes. Typing
  filters by name, case-insensitive substring. Beers are sorted newest first by `created_at`
  (`Date.parse`, then id descending). No match shows "No beer matches"; there is no create
  option. Used by `KegForm`, the only beer chooser in the app.
- **Why**: `<datalist>` cannot show a "no match" line, filters differently per browser, maps a
  typed name back to an id badly when names repeat, and looks different on every phone. A
  library (Downshift, React Select) adds a dependency for about 120 lines.
- **Edit keg**: the keg's current beer stays selected even when the filter text hides it.

## R7. Theme

- **Decision**: the admin root carries `theme-light`, `theme-dark` or `theme-system`. Light
  tokens apply to `theme-light` and `theme-system`; dark tokens to `theme-dark` and, inside
  `@media (prefers-color-scheme: dark)`, `theme-system`. `color-scheme` is set to match so
  native controls follow. System therefore follows the device live with no script (FR-011).
- **Colors** (checked for WCAG 2.1 AA, FR-027):
  - Royal blue `#4169e1` is the one accent, the same in both themes, and is never a text color.
    It marks things, not words: primary button fill (white text on it, 4.85:1), the current
    nav item's bar and background tint, the selected theme radio, checked checkboxes, the focus
    ring, and the resize handle while dragged. As a non-text mark it needs 3:1 (WCAG 1.4.11): it
    is 4.48:1 on the light background and 3.6:1 or more on the dark background and cards.
  - All text uses the neutral text colors. Links and the current nav item stay neutral text and
    are marked by an underline or the blue bar.
  - Light: background `#f4f6fb`, cards `#ffffff`, text `#1a1d24` (16.9:1), muted text
    `#555b6a` (6.3:1 on the background).
  - Dark: background `#0b0c0f`, cards `#17191f`, text `#e8eaf0` (14.6:1), muted `#a3a8b5`
    (7.4:1).
  - Final values may move a little; the gate (quickstart §4) checks contrast on every page in
    both themes with axe-core and fails on any AA color-contrast violation.
- **Font**: the system UI font stack. The approved mockup used Inter from Google Fonts, but the
  CSP (`default-src 'self'`) blocks external fonts, and bundling a font file is not worth a new
  asset for this. Text looks slightly different from the mockup (owner told 2026-11-10).
- **Theme control**: a three-way radio group (Light, Dark, System, with icons) at the bottom of
  the side panel next to the version and Sign out. Sign-in and setup screens use the saved
  theme but show no control.

## R8. Layout, side panel and phone menu

- **Breakpoint**: 768 px. At 768 and wider the side panel is a fixed column; narrower, it is an
  off-canvas drawer opened by a menu button in a top bar.
- **Resizing (FR-007)**: a `role="separator"` handle on the panel edge with
  `aria-orientation="vertical"`, `aria-valuenow/min/max` and a label. Pointer drag
  (pointer capture) and Left/Right arrow keys (16 px steps). Width default 256 px, clamped to
  200–400 px and to at most 50% of the window, re-clamped on window resize.
- **No style attributes**: the width is applied as the custom property `--nav-width` through
  CSSOM (`element.style.setProperty`) from a ref. CSP's inline-style rule covers `style`
  attributes in markup and `setAttribute("style")`, not CSSOM property writes. No `style` prop or
  attribute appears in any component. The gate loads the app through the nginx image with the
  real CSP and fails on any CSP violation in the console.
- **Phone drawer (FR-008)**: the menu button has `aria-expanded` and `aria-controls`. Opening
  moves focus to the current page item; selecting a page, tapping the backdrop or pressing
  Escape closes it and returns focus to the menu button.
- **Touch targets (FR-028)**: buttons, nav items and fields have a minimum block size of 44 px
  under the phone media query.
- **Tables**: wide tables (keg history, tokens, batches) sit in a wrapper with
  `overflow-x: auto`, so only the card scrolls sideways (FR-009).

## R9. Saved preferences

- **Decision**: `web/src/prefs.ts` reads and writes two `localStorage` keys,
  `beer-menu.admin.theme` (`light` | `dark` | `system`) and `beer-menu.admin.nav-width`
  (integer px). Every access is in `try`/`catch`; a missing, invalid or unreadable value falls
  back to `system` and 256 (spec edge case: private mode).
- **Why**: per-browser preferences, no server state (FR-029), separate from the TV theme in
  `/api/settings`.

## R10. Dialog conversions and FR-018 check

Forms that move into `Modal` (FR-019), and what they wrap today:

| Dialog | Opened from | Today |
| --- | --- | --- |
| Add keg | Taps (empty tap), On Deck | `KegForm` inline in `TapsPanel` |
| Add beer, Edit beer | Beer Library | `BeerForm` inline in `BeersPanel` |
| Add tap | Taps | one-click `POST /api/taps {}`; the dialog adds an optional Label field (`TapCreate.label`, 1–50 chars; empty sends `{}` so the API names it "Tap N") |
| New API token | API Tokens | create form and one-time secret inline in `TokensPanel` |

Edit keg stays inline in its keg card (spec assumption).

Actions present today, each of which must remain (FR-018), from reading every panel:

- Taps: rename, enable/disable, move up/down, remove, add keg to an empty tap, put a waiting keg
  on an empty tap, add tap.
- Kegs on a tap: pour, set level, edit, take off, kick. Waiting kegs: edit, kick (owner
  decision 2026-11-10 after the screenshot review: pour and set level are dropped for waiting
  kegs; edit still sets the remaining volume).
- Keg History: list, delete.
- Beer Library: show archived, add, edit, re-sync (Brewfather beers), archive, restore, delete.
- Display Settings: title, TV theme, page interval, units.
- Admin Users: list, add, remove.
- API Tokens: list, create (name, scope), copy the secret once, revoke.
- Change Password.
- Brewfather: connect (user ID, API key), disconnect, sync interval, sync now, refresh batches,
  select and import batches, import results.
- Session: sign out; first-admin setup and sign-in screens.

## R11. Screenshot and visual gate

- **Decision**: a puppeteer-core script in the session scratchpad (not the project) drives
  `/usr/bin/firefox` headless against the local compose stack (nginx image, real CSP) with seeded
  data. It captures every page and each dialog at 1920×1080, 1366×768 and 390×844 in light and
  dark (theme set through `localStorage` before load), plus the phone drawer open. It logs
  console errors and CSP violations, runs axe-core's `color-contrast` rule on every capture,
  and screenshots `/display` at 1920×1080 on this branch and on `main` for a side-by-side
  comparison (SC-003).
- **Why**: jsdom cannot see layout (project lesson); the done-when is screenshots.
- **Not in CI**: CI has no browser; the gate runs during `/speckit-implement` verification and
  the screenshots are shown to the owner.
