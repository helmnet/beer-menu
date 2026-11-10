# Quickstart: Display Settings

Validation guide for feature 7. Routes and shapes are in
[contracts/settings.openapi.yaml](./contracts/settings.openapi.yaml), storage and rules in
[data-model.md](./data-model.md), design choices in [research.md](./research.md).

## Prerequisites

- Feature 5 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26, `curl`, `jq`).
- The local test stack recipe from the project notes: compose from the worktree with
  `PUID=$(id -u) PGID=$(id -g)`, an override with env admin credentials and
  `BEER_MENU_COOKIE_SECURE: "false"`, a non-8080 `BEER_MENU_WEB_PORT`, scratch `BEER_MENU_DATA`.
- Headless Firefox + `puppeteer-core` in the session scratchpad for screenshots.
- A `write` token and a `read` token made from the admin app (Tokens panel).

## 0. Automated tests

```sh
(cd api && go test -race ./...)
(cd web && npm ci && npm test && npm run build)
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
```

Expected: all pass, including:

- `settings`: defaults on an empty table; bad stored values read as defaults; each validation
  rule (title 60/61 code points, control character, spaces-only to `""`; theme; units;
  page_seconds 4/5/300/301 and `15.5`); a patch with one bad field writes nothing; a patch
  touches only its keys; `Get`/`Set` still serve `brewfather.sync_interval`.
- `httpapi`: `GET /api/settings` and `PATCH /api/settings` behavior; route walk fixtures for both
  (no credential 401, read token 403 on PATCH, revoked 401, cross-site 403, write token and
  session succeed; GET with any credential); `GET /api/menu` has `display` with no `units`;
  `TestPublicRoutes` unchanged; contract test matches routes and `x-access` both ways.
- Upgrade: a database at version 5 with feature 6 data and no `display.*` rows serves default
  settings and the same menu plus `display` defaults.
- Web: `units` (US and metric conversions and round trip of 18.93 L); `SettingsPanel` (load,
  save only changed fields, field errors, 401); `DisplayPage` (title shown/absent, `light`
  class, page interval from `display`, bad or missing `display` falls back to defaults, a new
  interval applies); `TapsPanel`, `KegForm`, `KegHistory` in metric; `AdminPage` stubs.

## 1. Defaults (US1 scenarios 1-2, SC-003)

Start the stack on a fresh data dir, create the admin, add two taps with kegs.

```sh
curl -s localhost:$PORT/api/menu | jq .display
# {"title":"","theme":"dark","page_seconds":15}
curl -s -H "Authorization: Bearer $READ" localhost:$PORT/api/settings | jq
# {"title":"","theme":"dark","units":"us","page_seconds":15}
```

Screenshot `/display` at 1920×1080: same as v0.5.0 (no title row, dark).

## 2. Title and interval (US1)

```sh
curl -s -X PATCH -H "Authorization: Bearer $WRITE" -H 'Content-Type: application/json' \
  -d '{"title":"Helm Taproom","page_seconds":20}' localhost:$PORT/api/settings | jq
```

Expected: `200`, both values back. Within 30 s, with `/display` already open and not reloaded,
the title shows at the top. With more than 8 taps (or on-deck items), pages turn every 20 s.

Refusals (each `400` with `fields`, settings unchanged):
`{"page_seconds":4}`, `{"page_seconds":301}`, `{"page_seconds":15.5}`, a 61-character title,
`{"theme":"blue"}`, `{"units":"imperial"}`, `{"title":null}`, `{}`, `{"logo":"x"}`.

Access: the same PATCH with `$READ` is `403`; with no credential `401`.

Clear: `{"title":"   "}` stores `""`; the TV drops the title row and the cards take the space.

## 3. Theme (US2)

PATCH `{"theme":"light"}`. Within 30 s the open TV page turns light. Screenshot both themes:
text readable, glasses visible on the cards, pager and status readable. Open the admin app:
unchanged. PATCH `{"theme":"dark"}` brings dark back.

## 4. Units (US3)

In the admin app Settings panel pick Metric and save. Without reloading:

- Taps panel: levels like `12.50 L of 18.93 L`; the level form says "Pour (mL)" prefilled 500
  and "Remaining (L)".
- Add a keg: a corny 5 gal type prefills `18.93`. Keg history shows litres.
- Pour 500 mL: remaining drops by 0.50 L (check with `GET /api/kegs/{id}` as litres).
- Switch back to US: the same keg shows `5.00 gal` and `GET /api/kegs/{id}` `size_l` is still
  `18.93`.
- The TV looks the same in both units.

## 5. TV resilience (edge cases)

Stop the API container with the TV open: the last title, theme and menu stay on screen. Start
the API, change the title, and confirm the TV shows it within 30 s, with nobody touching it.

## 6. Upgrade from v0.5.0

Run the v0.5.0 compose on a scratch data dir with taps, kegs and a Brewfather schedule (if a
connection is available, otherwise just taps and kegs). Stop it, start this branch's compose on
the same data dir. Expected: no migration runs (database stays at version 5, no new backup),
settings read as defaults, the TV looks the same, and the Brewfather schedule is unchanged.
