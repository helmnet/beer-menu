# Quickstart: TV Display Options

Validation guide for feature 10. Schemas: [contracts/settings.openapi.yaml](./contracts/settings.openapi.yaml).
Fields and defaults: [data-model.md](./data-model.md).

## Prerequisites

- Docker Compose stack from feature 1, or `go run ./api/cmd/...` plus `npm run dev` in `web/`.
- An admin account and a `write` token (`$TOKEN`) and a `read` token (`$RTOKEN`).
- At least 9 enabled taps with beers (some with SRM and a long description), and at least 2 on
  deck items, so every layout pages.

## 1. Automated checks

```sh
cd api && go test -race ./...
cd web && npx vitest run --maxWorkers=2 && npm run lint && npm run build
npx @redocly/cli lint api/openapi.yaml
```

Expected: all pass. Coverage that matters: settings defaults and validation for each new field,
upgrade from a v5 database with no new rows, route walk for `/api/settings` unchanged, menu
`display` has every new field and no `units`, tap beers carry `srm` and `description`, on deck
beers do not; `TvView` per layout and detail switch; `SettingsPanel` sends only changed fields
and the preview follows the draft.

## 2. Defaults change nothing (SC-002)

1. Take screenshots of `/display` at 1920×1080 in both themes before upgrading.
2. Upgrade and take them again with no settings changed.
3. Expected: pixel-identical, apart from the beer data.

## 3. API behavior

```sh
curl -s localhost:8080/api/settings -H "Authorization: Bearer $RTOKEN"
# all 15 fields; new ones at their defaults

curl -s -X PATCH localhost:8080/api/settings -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"layout":"columns","show_srm":true,"title_color":"#FFAA00"}'
# 200; title_color comes back "#ffaa00"

curl -s -X PATCH localhost:8080/api/settings -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"layout":"grid","title_color":"red"}'
# 400 with fields.title_color; layout NOT changed (all-or-nothing)

curl -s -X PATCH localhost:8080/api/settings -H "Authorization: Bearer $RTOKEN" \
  -H 'Content-Type: application/json' -d '{"show_ibu":false}'
# 403

curl -s localhost:8080/api/menu | jq '.display, .taps[0].beer, .on_deck[0].beer'
# display: every TV field, no units; tap beer has srm and description; on deck beer has neither
```

## 4. TV picks up changes (SC-001)

With `/display` open, change each setting from Display Settings and save. Expected: the TV shows
it within 30 s, with no reload. Check each layout pages through every tap and on deck item, the
columns layout shows "Nothing on deck" with no on deck items, and descriptions stop at 2 lines
with "…".

## 5. Preview (SC-008)

1. Open Display Settings. Change layout, fonts, colors and switches without saving.
2. Expected: the preview changes at once; `/display` on the TV does not.
3. Use Previous and Next; leave it and watch it turn pages by itself.
4. Save. Screenshot the preview and the TV; expected: same picture apart from scale.
5. Repeat at phone width (390 px): the preview fits the screen width in a 16:9 box.

## 6. CSP and fonts in a real browser (SC-005)

1. Through the nginx container (real CSP header), open `/display` in headless Firefox with a
   title color, text color, title font `pacifico` and text font `oswald` set.
2. Expected: no CSP violations in the console; the colors and fonts render; every request in
   the network log is to the app's own origin; only the two chosen font files load.

## 7. Readability (SC-004)

Screenshot each layout with every font, in both themes, with theme colors. Expected: text
passes 4.5:1 (theme colors only) and stays readable from 3 m on the 1080p TV.
