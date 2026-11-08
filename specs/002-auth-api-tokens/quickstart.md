# Quickstart: Authentication and API Tokens

Validation guide for feature 2. Each section proves one user story from [spec.md](./spec.md).
Endpoints and bodies follow [contracts/openapi.yaml](./contracts/openapi.yaml); tables and
access levels follow [data-model.md](./data-model.md).

## Prerequisites

- Feature 1 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26).
- `curl` and `jq`.
- A fresh data folder: `rm -rf data && mkdir data`.

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

Expected: all pass. The Go suite covers every constitution IV auth rule: unauthenticated
access, `read` versus `write`, revoked tokens, CSRF on session writes, rate limiting, plus the
0001 to 0002 migration and the route `x-access` check.

## 1. First admin (User Story 1)

**Setup screen path**:

```sh
docker compose up -d --build
curl -s $B/api/setup                       # {"needed":true}
```

Open `http://localhost:8080/`: the setup form shows. Create `owner` with a 12+ character
password. You land signed in.

```sh
curl -s $B/api/setup                       # {"needed":false}
curl -s -o /dev/null -w '%{http_code}\n' -X POST $B/api/setup \
  -H 'Content-Type: application/json' -d '{"username":"x","password":"abcdefghijkl"}'   # 409
```

**Env var path** (fresh data folder): add to the `api` service

```yaml
    environment:
      BEER_MENU_ADMIN_USERNAME: owner
      BEER_MENU_ADMIN_PASSWORD: ${BEER_MENU_ADMIN_PASSWORD}
```

start the stack, and confirm `GET /api/setup` returns `{"needed":false}` and the log shows
`first admin created` with no password. Restart with a different password in the env var: the
old password still works and the log says the setting was ignored. With a fresh folder and an
11-character password, the API exits with an error and `data/` holds no admin.

## 2. Sign in and out (User Story 2)

```sh
curl -s -c jar -X POST $B/api/session -H 'Content-Type: application/json' \
  -d '{"username":"owner","password":"<password>"}'          # 200 {"id":1,"username":"owner",...}
grep bm_session jar                                           # HttpOnly; no plain password
curl -s -b jar $B/api/session                                 # 200
cp jar stolen
curl -s -b jar -X DELETE -o /dev/null -w '%{http_code}\n' $B/api/session   # 204
curl -s -b stolen -o /dev/null -w '%{http_code}\n' $B/api/session         # 401
curl -s -o /dev/null -w '%{http_code}\n' $B/api/admins                    # 401
```

Wrong username and wrong password both return `401` with the same body.

**CSRF**: sign in again, then

```sh
curl -s -b jar -X DELETE -H 'Sec-Fetch-Site: cross-site' -o /dev/null -w '%{http_code}\n' $B/api/session  # 403
curl -s -b jar -X DELETE -H 'Origin: http://evil.example' -o /dev/null -w '%{http_code}\n' $B/api/session # 403
```

## 3. Rate limiting (User Story 3)

```sh
for i in 1 2 3 4 5 6; do
  curl -s -o /dev/null -w '%{http_code} ' -X POST $B/api/session \
    -H "X-Forwarded-For: spoofed-client-$i" -H 'Content-Type: application/json' \
    -d '{"username":"owner","password":"wrong-password-x"}'
done; echo                                       # 401 401 401 401 401 429
```

Then the right password still gets `429` with a `Retry-After` header. After the window (15
minutes, or a restart in this manual check) it works again. Failures for `owner` do not limit
another username.

## 4. API tokens (User Story 4)

Sign in (cookie in `jar`), then:

```sh
R=$(curl -s -b jar -X POST $B/api/tokens -H 'Content-Type: application/json' \
  -d '{"name":"script","scope":"read"}' | jq -r .secret)
W=$(curl -s -b jar -X POST $B/api/tokens -H 'Content-Type: application/json' \
  -d '{"name":"ha","scope":"write"}' | jq -r .secret)
curl -s -b jar $B/api/tokens | jq                   # names, scopes, times; no secret field
curl -s -H "Authorization: Bearer $W" -o /dev/null -w '%{http_code}\n' $B/api/tokens  # 403
curl -s -H "Authorization: Bearer bmt_bad" -o /dev/null -w '%{http_code}\n' $B/api/session  # 401
```

Feature 2 has no data endpoints, so `read` versus `write` on data routes is proven by the
automated guard tests (section 0). In the admin app, create a token, confirm the secret shows
once, close the dialog, and confirm it cannot be shown again. Revoke it: it stays listed as
revoked, and a request with it returns `401`.

## 5. Admins and passwords (User Story 5)

In the admin app as `owner`: add `second`, sign in as `second` in another browser, change
`second`'s password (the first browser's session for `second`, if any, ends), remove `owner`,
and confirm `owner` can no longer sign in. Try to remove `second`, now the only admin: refused
with "The last admin cannot be removed."

**Recovery** (every admin locked out):

```sh
docker compose exec -it api /beer-menu-api reset-password second
```

Type the new password twice (not echoed). Sign in with it; the old password fails. Running it
for an unknown username exits 1 and changes nothing. Running it without `-it` refuses because
stdin is not a terminal.

## 6. Security headers (User Story 6)

```sh
curl -sI $B/ | grep -iE 'content-security|x-frame|x-content-type|referrer'
curl -sI $B/display | grep -iE 'content-security|x-frame'
curl -sI $B/api/setup | grep -iE 'x-frame|x-content-type|cache-control'
```

Expected: each header present once. The display page still loads in a browser (and in Fully
Kiosk) with no CSP errors in the console.

## 7. Behind NPM (production check)

On TrueNAS, behind NPM over HTTPS: sign in, add and remove a token, sign out. All succeed, so
the Origin/Host check passes through both proxies and the `Secure` cookie is sent. If the owner
uses plain HTTP by LAN IP instead, set `BEER_MENU_COOKIE_SECURE=false`.

Time a correct sign-in from a LAN client:

```sh
curl -s -o /dev/null -w '%{time_total}\n' -X POST https://<host>/api/session \
  -H 'Content-Type: application/json' -d '{"username":"owner","password":"<password>"}'
```

Expected: under 1 s (SC-008).
