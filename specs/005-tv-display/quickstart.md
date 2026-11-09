# Quickstart: TV Display

Validation guide for feature 5. Each section proves one user story from [spec.md](./spec.md).
The menu shape is in [contracts/menu.openapi.yaml](./contracts/menu.openapi.yaml) and the
selection rules in [data-model.md](./data-model.md).

## Prerequisites

- Feature 4 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26, `curl`, `jq`).
- A local stack from this worktree (`docker compose up --build`), with an admin, a `write` token
  in `$W` and a `read` token in `$R`.

```sh
export PUID=$(id -u) PGID=$(id -g)
B=http://localhost:8080
H="Authorization: Bearer $W"
```

## 0. Automated tests

```sh
(cd api && go test -race ./...)
(cd web && npm ci && npm test && npm run build)
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
```

Expected: all pass, including the menu selection tests, the exact-field test on the public
response, `TestPublicRoutes` with `GET /api/menu`, the 405 for other methods, `BeerArt` level
tests and `DisplayPage` polling tests with fake timers.

## 1. Set up a menu (US1, US3)

With the admin app or the API: three taps, the third disabled; a keg of beer A (with SRM, ABV,
IBU and style) on tap 1 at 60% left; tap 2 empty; a keg on tap 3; a waiting sanke keg of beer B;
beer C fermenting, beer D conditioning, beer E planned, beer F fermenting and archived.

```sh
curl -s $B/api/menu | jq .
```

Expected, with no credential: `taps` has tap 1 (beer A, `level` 0.6, `shape` corny) and tap 2
(`beer` and `keg` null), not tap 3. `on_deck` is B (sanke keg), then D (`conditioning`), then C
(`fermenting`); no E or F. No `id`, `size_l`, `remaining_l` or timestamp anywhere:

```sh
curl -s $B/api/menu | jq '[paths | last | strings] | unique'
```

lists only `beer, color, abv, ibu, keg, label, level, name, on_deck, shape, status, style, taps`.

## 2. Access stays closed elsewhere (FR-001)

```sh
for p in beers taps kegs keg-types srm-colors; do curl -s -o /dev/null -w "$p %{http_code}\n" $B/api/$p; done
curl -s -o /dev/null -w "%{http_code}\n" -X POST $B/api/menu
curl -sI $B/api/menu | grep -i cache-control
```

Expected: every list 401; POST 405; `Cache-Control: no-store`.

## 3. The display (US1, US3)

Open `$B/display` in a browser at 1920×1080 (full screen). Expected: dark page, taps 1 and 2
left to right, tap 1 a glass in beer A's color filled to about 60% with no number, tap 2 "Empty"
with an empty glass, and "1 / 2" in the bottom right corner. After 15 seconds the "On deck" page
shows B as a sanke keg, then D and C as corny kegs with their status, with "2 / 2"; 15 seconds
later the tap page returns. No scroll bar on either page. The browser console shows no CSP
errors.

## 4. Live updates (US2)

```sh
KEG=<tap 1 keg id>
curl -s -X POST $B/api/kegs/$KEG/pours -H "$H" -H 'Content-Type: application/json' -d '{"amount_l": 5}'
```

Expected: the glass drops within 30 seconds, no reload. Then `docker compose stop api` for
3 minutes: the menu stays, an "offline" notice appears after 2 to 4 minutes.
`docker compose start api`: the notice goes within 30 seconds of the API answering; a further
change shows without reload.

## 5. Admin taps panel (US4)

Open the admin Taps panel. Expected: each keg shows its art filled to its level with US gallons
beside it, and no plain level bar.

## 6. Many taps

Add taps until 12 are enabled, each with a keg. Expected: pages turn every 15 seconds: taps 1
to 8 (4 × 2), taps 9 to 12, then the "On deck" page, with "1 / 3" to "3 / 3" in the corner.
Every glass on a page is the same size, names are readable and nothing scrolls. Add taps 13 to
16: still 3 pages, the second now full. The 15-second interval is fixed; making it a setting
belongs to feature 7.

## 7. Real TV (SC-001, SC-004, R10)

Point Fully Kiosk on the Fire TV Stick at the deployed `/display`. Expected: renders the same as
step 3 (if blank, check the WebView version against Vite's build target, research R10); readable
from 3 m; a glass at 100%, 50% and 10% are clearly different; runs 7 days with no touch, through
at least one API restart.
