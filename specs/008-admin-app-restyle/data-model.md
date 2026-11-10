# Data Model: Admin App Restyle

No server data changes (FR-029): no migration, no new setting, no API change. The database stays
at its current version. The only new state lives in the browser.

## Browser preferences (`localStorage`, per browser)

Read and written only through `web/src/prefs.ts`. Every access is wrapped in `try`/`catch`.

| Key | Values | Default | Invalid or unreadable |
| --- | --- | --- | --- |
| `beer-menu.admin.theme` | `light`, `dark`, `system` | `system` | read as `system` |
| `beer-menu.admin.nav-width` | integer CSS px | 256 | read as 256 |

- The width is clamped on read and on every window resize to 200–400 px and to at most 50% of
  the window width. The clamped value is what shows; the stored value is overwritten only when
  the owner resizes.
- Separate from the server's TV theme (`display.theme` in `/api/settings`), which this feature
  does not touch.

## In-memory UI state (`AdminPage`)

| State | Values | Initial |
| --- | --- | --- |
| `view` | loading, setup, login, signedIn(admin), error | loading (unchanged) |
| `page` | one of the nine page keys below | `taps`, and again after every sign-in |
| `openGroup` | `menu`, `administration`, `integrations`, or none | the current page's group |
| `units` | `us`, `metric` | `us`, then `GET /api/settings` after sign-in |
| `version` | string | from `GET /api/health`; "unknown" on failure or empty |
| `drawerOpen` | boolean (phone only) | false |

Rules:

- Selecting a group sets `openGroup` to it (or to none if it was open). Only one group is open.
- Selecting a page sets `page`, sets `openGroup` to its group, and closes the drawer.
- Only the current page's component is mounted.

## Page tree

| Group (icon) | Page key | Label (icon) | Cards |
| --- | --- | --- | --- |
| Menu (`Beer`) | `taps` | Taps (`GlassWater`) | Taps |
| | `on-deck` | On Deck (`Hourglass`) | On deck |
| | `keg-history` | Keg History (`History`) | Keg history |
| | `beers` | Beer Library (`Library`) | Beer library |
| | `display` | Display Settings (`Tv`) | Display settings |
| Administration (`Shield`) | `admins` | Admin Users (`Users`) | Admin Users |
| | `tokens` | API Tokens (`KeyRound`) | API tokens |
| | `password` | Change Password (`Lock`) | Change password |
| Integrations (`Plug`) | `brewfather` | Brewfather (`FlaskConical`) | Connection and sync interval; Batches |

Icon names are `lucide-react` 1.61.0 exports (all checked present in the tarball). Other icons:
`Menu` and `X` (drawer), `LogOut`, `Sun`, `Moon`, `Monitor` (theme), `Plus` (add buttons).

## On Deck item (derived, not stored)

Built in the browser from `GET /api/kegs` and `GET /api/beers` (research R5):

- **Waiting keg**: a current keg with `tap_id === null`; full `Keg` with id, so it keeps its keg
  actions.
- **Beer being made**: a current beer with status `conditioning` or `fermenting` and no current
  keg (no keg in the `GET /api/kegs` result has its `beer_id`).

Order: waiting kegs in API order (oldest first), then conditioning beers, then fermenting
beers, each by name ignoring case, then id.

## Beer picker order and filter (derived)

- Sort: `created_at` newest first (`Date.parse`), ties by id descending.
- Filter: the typed text, trimmed, as a case-insensitive substring of the name
  (`toLocaleLowerCase`). Empty text lists all.
