# Quickstart: Beer Library

Validation guide for feature 3. Each section proves one user story from [spec.md](./spec.md).
Endpoints and bodies follow [contracts/beers.openapi.yaml](./contracts/beers.openapi.yaml);
the table and rules follow [data-model.md](./data-model.md).

## Prerequisites

- Feature 2 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26, `curl`, `jq`).
- A data folder from feature 2 with an admin, to prove the upgrade (SC-007). Note the newest
  file in `data/backups/` before starting.
- A `write` token in `$W` and a `read` token in `$R`, made in the admin app's Tokens panel.

```sh
export PUID=$(id -u) PGID=$(id -g)
B=http://localhost:8080
```

## 0. Automated tests

```sh
(cd api && go test ./...)
(cd web && npm ci && npm test)
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
```

Expected: all pass, including the 0002 to 0003 migration test, the per-route auth and CSRF
tests for every beer write, the per-route auth tests for every beer read, and the `x-access`
contract test (research R11).

## 1. Upgrade (SC-007)

```sh
docker compose up -d --build
ls -t data/backups | head -1          # a new beermenu-*-v2.db
curl -s -H "Authorization: Bearer $R" $B/api/beers   # []
```

Sign in with the feature 2 admin: it still works, and the token list is unchanged.

## 2. Add and edit beers (User Story 1)

```sh
H="Authorization: Bearer $W"
curl -s -H "$H" -d '{"name":"Mad Hops Pale Ale"}' $B/api/beers | jq '{id,status,source,abv}'
# status "ready", source "manual", abv null
curl -s -H "$H" -d '{"name":"Honey IPA","style":"American IPA","abv":6.7,"ibu":55,"srm":7,
  "brewer":"Helm'"'"'s Brews","brew_date":"2026-10-01","status":"ready"}' $B/api/beers | jq .
curl -s -H "$H" -X PATCH -d '{"abv":5.4}' $B/api/beers/1 | jq '{name,abv}'
curl -s -H "$H" -d '{"name":"X","abv":150}' $B/api/beers | jq .fields   # {"abv": "..."}
curl -s -H "$H" -d '{"name":"X","source":"brewfather"}' $B/api/beers     # 400 invalid_request
```

In the admin app, time adding a beer with name, style, ABV, SRM and status: under 1 minute
(SC-001). Add a beer with a name only, then edit it. Submitting an empty name shows an
error next to the Name field.

## 3. Reads with any credential and refusals (User Story 2, SC-002, SC-003)

```sh
curl -s -H "Authorization: Bearer $R" $B/api/beers | jq '.[] | {name,srm,color}'  # "#rrggbb"
for p in /api/beers /api/beers/1 /api/srm-colors; do
  curl -s -o /dev/null -w '%{http_code}\n' $B$p                                   # 401 each
done
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" $B/api/beers/999                 # 404
curl -s -o /dev/null -w '%{http_code}\n' -d '{"name":"x"}' $B/api/beers          # 401
curl -s -o /dev/null -w '%{http_code}\n' -H "Authorization: Bearer $R" \
  -X DELETE $B/api/beers/1                                                        # 403
curl -s -o /dev/null -w '%{http_code}\n' -H "$H" -H 'Sec-Fetch-Site: cross-site' \
  -X DELETE $B/api/beers/1                                                        # 403
```

## 4. Artwork (User Story 3, SC-004, SC-005)

```sh
curl -s -H "$H" -d '{"name":"Pale","srm":4}' $B/api/beers > /dev/null
curl -s -H "$H" -d '{"name":"Stout","srm":35,"status":"fermenting"}' $B/api/beers > /dev/null
curl -s -H "$H" $B/api/srm-colors | jq '.colors | length'      # 40
```

In the admin app's beer library every beer is a corny keg, since no beer is on a tap until
feature 4: "Pale" is a pale-gold keg, "Stout" a near-black keg, and "Mad Hops Pale Ale" (no SRM)
an empty keg. Open the edit form for "Pale", change SRM to 20 and status to conditioning: the
preview turns amber before saving and stays a keg. With the browser's network tab open, reload
the library: only `/api/beers` is requested, with no image or font request; opening the form
adds one `/api/srm-colors` request (SC-005). Check the colors of three of your own beers against
what you expect (SC-004).

## 5. Archive and restore (User Story 4)

```sh
curl -s -H "$H" -X PATCH -d '{"archived":true}' $B/api/beers/1 | jq .archived_at  # a time
curl -s -H "$H" $B/api/beers | jq 'map(.id) | index(1)'           # null: gone from default
curl -s -H "$H" "$B/api/beers?archived=true" | jq 'map(.id)'      # [1]
curl -s -H "$H" $B/api/beers/1 | jq .archived_at                  # still readable, archived
curl -s -H "$H" -X PATCH -d '{"archived":false}' $B/api/beers/1 | jq .archived_at  # null
```

In the admin app, "Show archived" lists archived beers, and Restore brings one back unchanged.

## 6. Delete (User Story 5)

In the admin app, click Delete on a beer and cancel the prompt: it stays. Delete and confirm:
it is gone, and `curl -s -o /dev/null -w '%{http_code}\n' -H "$H" $B/api/beers/<id>` gives 404.

## 7. Scale (SC-006)

```sh
for i in $(seq 1 500); do curl -s -H "$H" -d "{\"name\":\"Beer $i\",\"srm\":$((i % 40 + 1))}" \
  $B/api/beers > /dev/null; done
curl -s -o /dev/null -w '%{time_total}\n' -H "$H" $B/api/beers  # under 1 s
```

The admin library with 500 beers loads in under 1 second. Reset the data folder afterwards.
