# User guide (maintainer notes)

The end-user guide for the admin app, published at <https://helmnet.github.io/beer-menu/> by
`.github/workflows/pages.yml` on every push to `main` that changes `guide/`,
`web/src/pages/AdminPage.css` or `docker-compose.yaml`. This file is for maintainers and is not
published.

## Layout

- `build.mjs`: builds the site into `guide/_site/` (git-ignored). It holds the only navigation
  list (`NAV`) and reads the color tokens from the admin app. It fills `{{compose}}` in a
  fragment with the root `docker-compose.yaml`, comments and `build:` lines stripped.
- `build.test.mjs`: builds the site and checks it against
  [the site contract](../specs/017-user-guide/contracts/guide-site.md) and
  [its changes](../specs/018-guide-compose-install/contracts/guide-site.md).
- `seed.sh`: fills a fresh, throwaway stack with invented sample data for screenshots.
- `src/layout.html`: the page shell shared by every page (head, nav, main).
- `src/guide.css`: the guide's styles.
- `src/copy.js`: adds a Copy button after each `<pre class="copy">`; the only script. A fragment
  that needs it loads it with `<script src="copy.js" defer></script>`.
- `src/pages/<slug>.html`: one body fragment per page.
- `src/images/<slug>[-<state>].png`: the screenshots.

The build uses only the Node standard library (Node 26, as in `.tool-versions`); there is
nothing to install.

## Build, test and preview

```sh
node guide/build.mjs
node --test guide/
```

Then open `guide/_site/index.html` in a browser. All links are relative, so it works from the
file system with no server.

## Add a page

1. Write the body in `src/pages/<slug>.html`: a `<p>` saying what the page is for, a `<figure>`
   with an `<img alt>` and `<figcaption>` per screenshot, one `<h2>` per task followed by an
   `<ol>` of steps, and a `<div class="warning">` wherever an action cannot be undone or a
   secret is shown once.
2. Add `{ slug, title, group }` to `NAV` in `build.mjs`, in side-panel order. The build fails if
   a fragment has no `NAV` entry or an entry has no fragment.

Write for end users: plain words, no code (only the Install with Docker page shows commands).
The Install with Docker page also documents the environment variables; the test fails until
every variable the API startup code (`api/cmd/beer-menu-api/`) or `docker-compose.yaml` reads is
named there.
Copy button, field and heading names exactly from the admin app's components in
`web/src/components/`.

## Colors

Every color comes from the admin app's dark theme: `build.mjs` copies the custom properties of
the `.admin` and `.admin.theme-dark` blocks in `web/src/pages/AdminPage.css` into each page.
`guide.css` uses them only through `var(--…)`; never write a color value in it (the test fails
on one). If the guide needs another token, add it to `REQUIRED_TOKENS` in `build.mjs`.

## Retake screenshots

Only invented sample data from `seed.sh` may appear. Never take screenshots of a real instance:
no real names, hostnames, IP addresses, tokens, recovery codes or QR codes.

1. Start a throwaway stack with an empty data folder. For the two-factor setup screenshot and
   the Brewfather page, give it a throwaway encryption key, and for the Add passkey button,
   `BEER_MENU_PUBLIC_URL=http://localhost:8080` (see the commented `api` settings in
   `docker-compose.yaml`). Bind it to this computer only (the sample password is public) and give
   it its own Compose project, so it never replaces a Beer Menu already running from this folder.

   ```sh
   export PUID=$(id -u) PGID=$(id -g) BEER_MENU_DATA=$(mktemp -d)
   export BEER_MENU_WEB_PORT=127.0.0.1:8080 COMPOSE_PROJECT_NAME=beer-menu-guide
   docker compose up -d
   ```

2. Take `getting-started-setup.png` now, before seeding, while the app shows the first-run form.
3. Run `sh guide/seed.sh`. It refuses to run once setup is done. It prints the sample admin's
   username and password.
4. In a desktop browser with a 1280×800 viewport, open `http://localhost:8080/`, take
   `getting-started-sign-in.png`, sign in, and choose **Dark** at the bottom of the side panel.
5. Take each screenshot below as a PNG of the viewport, about 300 KB or less, into
   `src/images/`:

   | File | State |
   |------|-------|
   | `getting-started-setup.png` | first-run form, empty |
   | `getting-started-sign-in.png` | sign-in form, empty |
   | `tv-display.png` | `/display` with the sample taps |
   | `taps.png` | Taps: three taps with kegs, one empty |
   | `on-deck.png` | On Deck: the two waiting kegs |
   | `keg-history.png` | Keg History: the kicked kegs |
   | `beers.png` | Beer Library list |
   | `beers-form.png` | Edit dialog open on Porch Light Pale Ale |
   | `display.png` | Display Settings |
   | `admins.png` | Admin Users: the two sample admins |
   | `tokens.png` | API Tokens: only "Kitchen tablet" |
   | `tokens-new.png` | New API token dialog showing a newly created token |
   | `password.png` | Change Password, empty |
   | `two-factor.png` | Two-Factor Sign-In with no factors |
   | `two-factor-setup.png` | Add authenticator app dialog with its QR code |
   | `brewfather.png` | Brewfather, not connected (never enter a real key) |

6. Open every image and check it shows only sample data.
7. Tear the stack down and delete its data, which also makes the shown token and QR code
   worthless:

   ```sh
   docker compose down
   rm -rf "$BEER_MENU_DATA"
   ```

## Keeping the guide current

A pull request that changes an admin page should update that page's guide fragment and
screenshot in the same pull request. The `guide` CI job runs when `guide/`, `AdminPage.css`,
`docker-compose.yaml`, `api/cmd/beer-menu-api/` or the CI files change; changing only this
README does not run it.

## One-time owner steps

- Settings > Pages > Source: **GitHub Actions** (the deploy fails until this is set).
- Add `guide` to the required status checks of the `main` ruleset.
