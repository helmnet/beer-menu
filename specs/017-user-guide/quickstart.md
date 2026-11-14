# Quickstart: Validate the User Guide

## Prerequisites

- Node 26 (`.tool-versions`), Docker with Compose, `curl`, a desktop browser.

## 1. Build and test

```sh
node guide/build.mjs
node --test guide/
```

Expected: the build writes `guide/_site/` and all tests pass (see
[contracts/guide-site.md](contracts/guide-site.md)).

## 2. Preview

Open `guide/_site/index.html` in a browser. The guide uses only relative links, so it works
from the file system with no server.

- Each nav link opens its page, the current page is marked, and every screenshot loads.
- Turn JavaScript off and reload: nothing changes (FR-015).
- Narrow the window to 320 px: there is no sideways scroll, and Menu opens the nav (SC-004).

## 3. Compare with the app

```sh
export PUID=$(id -u) PGID=$(id -g) BEER_MENU_DATA=$(mktemp -d)
docker compose up -d
sh guide/seed.sh            # refuses if setup is already done
```

Open `http://localhost:8080/`, sign in with the seed admin printed by the script, and set the
theme to Dark.

- The guide's background, panels, side panel, text and accent match the app (US3).
- On each admin page, every button, field and label named in the guide matches the app (FR-011).
- Following the Taps page puts a keg on a tap (SC-003).

## 4. Install with Docker page

On a machine with Docker, follow the Install with Docker page from an empty folder. Expected:
the admin app and the TV display open.

## 5. Retake screenshots (when needed)

Follow `guide/README.md` with the stack from step 3. Check that every image shows only seed
data: no real names, hostnames or IPs, and no token or code from anywhere but this throwaway
stack.

## 6. Tear down

```sh
docker compose down
rm -rf "$BEER_MENU_DATA"
```

## 7. Publish (after merge)

After the PR merges to `main`, the `pages` workflow deploys. Expected: the change shows at
`https://helmnet.github.io/beer-menu/` within 15 minutes (SC-006).
