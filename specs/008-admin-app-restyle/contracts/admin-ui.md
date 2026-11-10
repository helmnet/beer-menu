# UI Contract: Admin App

No API contract changes: `api/openapi.yaml` is untouched. This file fixes the accessible names,
roles and API calls that component tests and the screenshot gate rely on. Tests query by role
and accessible name only.

## API calls the shell and new components make (all existing routes)

| Call | Who | When |
| --- | --- | --- |
| `GET /api/health` | side panel | once after sign-in; `version` shown, "unknown" on failure or empty |
| `GET /api/settings` | `AdminPage` | once after sign-in, for `units`; 401 returns to sign-in |
| `GET /api/kegs` | On Deck | on mount and after each change |
| `GET /api/beers` | On Deck, `KegForm` picker | on mount |
| `POST /api/taps` | Add tap dialog | body `{}` when the label is empty, else `{"label": ...}` |

Every other call stays as the panels make it today.

## Shell

| Element | Role and name |
| --- | --- |
| Side panel | `navigation`, name "Admin" |
| Group item | `button`, name = group label, `aria-expanded` true/false, `aria-controls` its list |
| Page item | `button`, name = page label; the current one has `aria-current="page"` |
| Resize handle (laptop) | `separator`, name "Resize side panel", `aria-orientation="vertical"`, `aria-valuenow/min/max`, focusable; Left/Right change width by 16 px |
| Menu button (phone) | `button`, name "Open menu" / "Close menu", `aria-expanded`, `aria-controls` the side panel |
| Theme control | `radiogroup`, name "Theme"; radios "Light", "Dark", "System" |
| Version | text "Version <v>" |
| Sign out | `button`, name "Sign out" |
| Main area | `main`; first heading is the page label (`h2`) |

Root element: `div.admin.theme-<light|dark|system>`, rendered by `AdminPage`; `SideNav` sets
`--nav-width` on it (its nearest `.admin` ancestor) through CSSOM only.

## Dialogs

Each is a native `<dialog>` opened with `showModal()`, so its role is `dialog` and its name is
its heading.

| Dialog name | Opened by (button name) | Page |
| --- | --- | --- |
| "Add keg" | "Add keg" | Taps (on an empty tap), On Deck |
| "Add beer" | "Add beer" | Beer Library |
| "Edit <beer name>" | "Edit" on a beer | Beer Library |
| "Add tap" | "Add tap" | Taps |
| "New API token" | "New token" | API Tokens |

Behavior: Escape (`cancel` event) or "Cancel" closes without saving and returns focus to the
opening button; a server validation error keeps it open with field errors; success closes it
and reloads the page's list. The token dialog, after a successful create, shows the secret with
"Copy" and "Close"; closing it never shows the secret again.

## Beer picker (inside "Add keg" and keg edit forms)

| Element | Role and name |
| --- | --- |
| Input | `combobox`, name "Beer", `aria-expanded`, `aria-controls` the list, `aria-activedescendant` |
| List | `listbox`, name "Beers" |
| Option | `option`, name = beer name, `aria-selected` on the chosen one |
| No match | text "No beer matches" (no option) |

Keys: Down/Up move, Enter picks, Escape closes the list (not the dialog, when the list is open).

## Styling rules

- No `style` prop in JSX and no `style` attribute in markup. The one allowed `style` attribute
  at runtime is `--nav-width` on the `.admin` root, written via `element.style.setProperty`.
- All admin CSS selectors start with `.admin`, except `body:has(> #root > .admin)`.
- `DisplayPage.css` and `DisplayPage.tsx` are not changed.
