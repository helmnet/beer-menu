# Quickstart: Admin App Restyle

Validation guide for feature 8. Roles, names and calls are in
[contracts/admin-ui.md](./contracts/admin-ui.md), state and storage in
[data-model.md](./data-model.md), design choices in [research.md](./research.md).

## Prerequisites

- Node 26 (`.tool-versions`), Go 1.27, Docker Compose v2.
- The local test stack recipe from the project notes: compose from the worktree with
  `PUID=$(id -u) PGID=$(id -g)`, an override with env admin credentials and
  `BEER_MENU_COOKIE_SECURE: "false"`, a non-8080 `BEER_MENU_WEB_PORT`, scratch `BEER_MENU_DATA`.
  The web container's nginx must serve the build, so the real CSP applies.
- Headless Firefox (`/usr/bin/firefox`) with `puppeteer-core@24.10.0` and `axe-core` installed in
  the session scratchpad (not the project).

## 0. Automated tests

```sh
(cd web && npm ci && npm test && npm run build)
(cd api && go test -race ./...)   # unchanged API, must still pass
git diff --stat origin/main -- api web/src/pages/DisplayPage.css web/src/pages/DisplayPage.tsx web/nginx
```

Expected: all tests pass; the diff shows no change under `api/`, `web/nginx/` or the two
display files. New and updated tests include:

- `prefs`: defaults, invalid values, `localStorage` that throws, width clamping.
- `Modal`: labelled by its heading, `cancel` calls `onClose`, focus returns to the opener.
- `BeerPicker`: newest-first order, case-insensitive filter, "No beer matches", keyboard pick,
  current beer kept when filtered out.
- `SideNav` / `AdminPage`: Taps after sign-in; one group open at a time; only the current page
  mounted; version shown and "unknown" on failure; theme radios set the root class and save;
  separator arrow keys change and save the width; Sign out; units loaded from settings.
- `OnDeckPanel`: waiting kegs, then conditioning, then fermenting beers without a keg, by name;
  empty state; Add keg dialog.
- Dialog conversions in `TapsPanel` (Add keg, Add tap with and without label), `BeersPanel`
  (Add beer, Edit), `TokensPanel` (New token, secret shown once), each with a 400 keeping the
  dialog open.
- `KegCard`: button order, Take off only on a tapped keg, pour/level forms in the current units,
  inline Edit keg form inside the card.
- `ThemeControl`: Light, Dark, System radios report the choice.
- A test that renders a `lucide-react` icon and finds no `style` attribute.
- `AdminStyle`: the rendered admin DOM for every page and dialog has no `style` attribute except
  `--nav-width` on the `.admin` root; every `AdminPage.css` selector starts with `.admin`; no
  `color:` declaration uses the accent.

## 1. Seed data

With the stack up and signed in, add: 4 taps (one empty), 2 kegs on taps, 2 waiting kegs, 1
conditioning beer and 1 fermenting beer without kegs, 1 kicked keg, about 15 beers added at
different times (names with mixed case, one containing "IPA"), 1 extra admin, 2 tokens.

## 2. Navigation and layout (laptop, 1366×768)

1. Sign in: Taps shows, Menu is open, Taps marked current.
2. Open Administration: Menu closes. Open API Tokens: only the tokens card shows.
3. Drag the panel edge wider, reload: same width. Focus the handle, press Left twice: 32 px
   narrower.
4. The side panel bottom shows "Version <x>" matching `GET /api/health`, theme radios and Sign
   out.

## 3. Phone (390×844)

1. The side panel is hidden; "Open menu" shows it; Escape and a backdrop tap close it.
2. Pick On Deck: the drawer closes and On Deck shows. No page scrolls sideways; wide tables
   scroll inside their card.
3. Version and Sign out are reachable in the drawer.

## 4. Screenshot gate (blocking)

Run the scratchpad script against the stack. It captures each of the nine pages and the five
dialogs at 1920×1080, 1366×768 and 390×844 in light and dark (theme put in `localStorage`
before load), the phone drawer open, and the sign-in screen. It must report:

- no console errors and no CSP violation;
- no axe-core `color-contrast` violation on any capture;
- `document.documentElement.scrollWidth <= innerWidth` on every phone capture;
- in Firefox, Escape closes an open dialog, Tab stays inside it, and focus returns to the
  opening button;
- `/display` at 1920×1080 on this branch and on `main` (same data) with no visible difference.

Show the screenshots to the owner; the feature is done when they match the spec.

## 5. Behavior spot checks

- Add a keg from On Deck by typing "ipa" in the beer picker: matches only beers containing
  "ipa", newest first; it saves and the list updates.
- Set the device to dark with theme System: the admin turns dark without a reload. The TV theme
  in Display Settings is unchanged.
- Run every action in research R10's FR-018 list once.
