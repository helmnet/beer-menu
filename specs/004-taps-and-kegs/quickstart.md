# Quickstart: Taps and Kegs

Validation guide for feature 4. Each section proves one user story from [spec.md](./spec.md).
Endpoints and bodies follow [contracts/taps-kegs.openapi.yaml](./contracts/taps-kegs.openapi.yaml);
tables and states follow [data-model.md](./data-model.md).

## Prerequisites

- Feature 3 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26, `curl`, `jq`).
- A data folder from feature 3 with an admin and at least two beers (ids `$B1`, `$B2`), to prove
  the upgrade (SC-006). Note the newest file in `data/backups/`.
- A `write` token in `$W` and a `read` token in `$R`.

```sh
export PUID=$(id -u) PGID=$(id -g)
B=http://localhost:8080
H="Authorization: Bearer $W"
```

## 0. Automated tests

```sh
(cd api && go test -race ./...)
(cd web && npm ci && npm test)
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
```

Expected: all pass, including migration 0003 to 0004, the route-walk auth and CSRF tests for
every tap and keg route, concurrent pours, the racing second keg on one tap, unit conversions
for every keg type (SC-008), and the `x-access` contract test (research R9).

## 1. Upgrade (SC-006)

```sh
docker compose up -d --build
ls -t data/backups | head -1                                   # a new beermenu-*-v3.db
curl -s -H "Authorization: Bearer $R" $B/api/taps | jq 'map({position,label,enabled,keg})'
# [{"position":1,"label":"Tap 1","enabled":true,"keg":null}]
curl -s -H "Authorization: Bearer $R" $B/api/beers | jq 'map(.on_tap)'   # all false
```

## 2. Configure taps (User Story 1)

```sh
curl -s -H "$H" -d '{}' $B/api/taps | jq '{id,position,label}'          # Tap 2, position 2
curl -s -H "$H" -d '{"label":"Nitro"}' $B/api/taps | jq '{id,position}' # position 3
curl -s -H "$H" -X PATCH -d '{"label":"Left"}' $B/api/taps/1 | jq .label
curl -s -H "$H" -X PUT -d '{"ids":[3,1,2]}' $B/api/taps/order | jq 'map([.id,.position])'
# [[3,1],[1,2],[2,3]]
curl -s -H "$H" -X PUT -d '{"ids":[3,1]}' $B/api/taps/order | jq .fields   # ids error
curl -s -H "$H" -X PATCH -d '{"enabled":false}' $B/api/taps/2 | jq .enabled
```

In the admin app's Taps panel, move a tap up and down and rename one: the order and labels
persist after a reload.

## 3. Put kegs on taps and on deck (User Story 2)

```sh
curl -s -H "$H" -d "{\"beer_id\":$B1,\"tap_id\":1}" $B/api/kegs \
  | jq '{id,type,size_l,remaining_l,state,tapped_at}'   # corny_5_gal, 18.93, 18.93, on_tap
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" -d "{\"beer_id\":$B2,\"tap_id\":1}" $B/api/kegs
# 409 (tap_occupied)
curl -s -H "$H" -d "{\"beer_id\":$B2,\"type\":\"sanke_half\"}" $B/api/kegs \
  | jq '{id,size_l,state,tapped_at}'                    # 58.67, waiting, null
curl -s -H "$H" "$B/api/kegs?state=waiting" | jq 'map(.beer_name)'
curl -s -H "$H" $B/api/beers/$B1 | jq .on_tap          # true
curl -s -H "$H" -X PATCH -d '{"archived":true}' -o /dev/null -w '%{http_code}\n' $B/api/beers/$B2
# 409 (beer_has_keg: waiting keg)
```

Move the waiting keg onto tap 3 (`PATCH {"tap_id":3}`): its `tapped_at` is set. Take it off
(`{"tap_id":null}`) and put it back: `tapped_at` is unchanged.

In the admin app: putting a full keg on an empty tap takes under 30 seconds (SC-001). The beer
library shows beer 1 as a pint glass and the others as kegs; the half barrel on deck is drawn as
a sanke keg, the corny as a corny keg (SC-007).

## 4. Keg levels (User Story 3)

```sh
K=1
curl -s -H "$H" -d '{"amount_l":0.5}' $B/api/kegs/$K/pours | jq .remaining_l      # 18.43
curl -s -H "$H" -X PATCH -d '{"remaining_l":10}' $B/api/kegs/$K | jq .remaining_l  # 10
curl -s -H "$H" -X PATCH -d '{"remaining_l":20}' $B/api/kegs/$K | jq .fields       # remaining_l
for i in $(seq 1 10); do curl -s -H "$H" -d '{"amount_l":0.1}' $B/api/kegs/$K/pours > /dev/null & done; wait
curl -s -H "$H" $B/api/kegs/$K | jq .remaining_l                                   # 9
curl -s -o /dev/null -w '%{http_code}\n' -H "Authorization: Bearer $R" \
  -d '{"amount_l":0.5}' $B/api/kegs/$K/pours                                       # 403
```

In the admin app, a 16 fl oz pour takes two clicks (SC-002) and the level shows in gallons.

## 5. Kick and history (User Story 4)

```sh
curl -s -H "$H" -X PATCH -d '{"kicked":true}' $B/api/kegs/$K | jq '{state,kicked_at,tap_label}'
curl -s -H "$H" $B/api/taps/1 | jq .keg                       # null
curl -s -H "$H" -X PATCH -d '{"remaining_l":1}' -o /dev/null -w '%{http_code}\n' $B/api/kegs/$K  # 409
curl -s -H "$H" "$B/api/kegs?state=kicked" | jq 'map({beer_name,tap_label,kicked_at})'
curl -s -H "$H" -X DELETE -o /dev/null -w '%{http_code}\n' $B/api/beers/$B1   # 409 beer_has_kegs
```

Rename tap 1, then read history again: the kicked keg keeps the old label. In the admin app's
keg history, cancelling the delete prompt keeps the record.

## 6. Refusals (SC-004)

The route-walk tests cover every route. Spot-check: with no credential, `GET /api/taps` and
`POST /api/kegs/1/pours` give 401; a cross-site browser request (`Sec-Fetch-Site: cross-site`
with the session cookie) to `PATCH /api/taps/1` gives 403.

## 7. Scale (SC-005)

Create 20 taps and 1,000 kicked kegs with a loop like feature 3's, then time
`GET /api/taps` and `GET /api/kegs?state=kicked`: each under 1 s; the Taps panel and keg history
load in under 1 s. Reset the data folder afterwards.
