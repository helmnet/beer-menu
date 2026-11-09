# beer-menu

A self-hosted digital tap list for a home bar: a React admin app manages the beers and taps,
and a TV shows the current menu in a browser. It runs as two containers: a Go REST API that
owns a SQLite database, and nginx serving the React app and proxying `/api` to the API. Every
client uses the same documented REST API.

This is an early version: the admin app handles sign-in, admins, API tokens and the beer
library, and the `/display` tap list is a placeholder.

## Layout

- `api/`: Go API (`cmd/beer-menu-api`), SQLite store and migrations (`internal/store`), HTTP
  handlers (`internal/httpapi`) and the API contract `openapi.yaml`.
- `web/`: React + TypeScript app built with Vite, and the nginx config of the web image.
- `docker-compose.yaml`: example compose file that starts both containers.
- `specs/`: feature specifications and plans.

## Local development

Go 1.27 and Node 26. Run the API on port 8080 with its database in `api/data/`:

```sh
cd api
mkdir -p data
BEER_MENU_DATA_DIR=./data go run ./cmd/beer-menu-api
```

In another terminal, run the web app with hot reload. Its dev server proxies `/api` to
`localhost:8080`:

```sh
cd web
npm ci
npm run dev
```

Tests: `go test ./...` in `api/`; `npm test` in `web/`.

## Run with Docker Compose

```sh
export PUID=$(id -u) PGID=$(id -g)
mkdir -p data
docker compose up -d --build
```

Open `http://localhost:8080/` for the admin app and `http://localhost:8080/display` for the
tap list. Both containers run as `PUID:PGID`, which can be any non-root UID and GID (never 0)
that owns the data folder. Both containers refuse to start as UID 0 or GID 0, or with root as
a supplementary group.

| Variable | Default | Meaning |
| --- | --- | --- |
| `PUID`, `PGID` | `65532` | UID and GID both containers run as |
| `BEER_MENU_DATA` | `./data` | Host folder mounted as the API's `/data` |
| `BEER_MENU_WEB_PORT` | `8080` | Host port of the web container |
| `BEER_MENU_VERSION` | `latest` | Image tag; production pins an exact `X.Y.Z` |

The database is `data/beermenu.db`; backups go to `data/backups/`.

## Admins and sign-in

**First admin.** A fresh install has no admin and no default password. Either open
`http://localhost:8080/` and create the first admin on the setup screen, or set both of these
on the `api` service before the first start (see the commented lines in
`docker-compose.yaml`):

| Variable | Meaning |
| --- | --- |
| `BEER_MENU_ADMIN_USERNAME` | Username of the first admin |
| `BEER_MENU_ADMIN_PASSWORD` | Its password, 12 to 256 characters; pass it from a secret, never commit it |

They are used only while no admin exists and are ignored after that. Setting only one, or a
value that breaks the rules, stops the API with an error. Remove them once the admin exists.

**Cookie `Secure` flag.** `BEER_MENU_COOKIE_SECURE` (default `true`) marks the session cookie
`Secure`, so browsers send it only over HTTPS and to `http://localhost`. Set it to `false`
only when you reach the app over plain HTTP by LAN address or hostname; otherwise sign-in
appears to work but the next request is not signed in. Behind a TLS proxy, leave it on.

**Locked out.** If every admin has lost their password, reset one from the server:

```sh
docker compose exec -it api /beer-menu-api reset-password <username>
```

It asks for the new password twice without echoing it, refuses unless run in a terminal
(`-it`), and ends all of that admin's sessions. There is no way to reset a password over HTTP.

**API tokens.** Admins create tokens in the admin app with scope `read` or `write`. Clients
send them as `Authorization: Bearer <token>`. A token's secret is shown once, when it is
created; only its hash is stored. Every secret starts with `bmt_`, so secret scanners can
find leaked ones (`.gitleaks.toml` has a rule for them). Revoke a leaked token in the admin
app.

## API

`api/openapi.yaml` is the API contract. Every change to an endpoint updates it in the same
change. CI lints it with Redocly, and the route-coverage test
(`go test ./internal/httpapi/... -run Routes` in `api/`) fails if a route in
`internal/httpapi/routes.go` is not documented in it.

`GET /api/health` is public and reports the app version and whether the database answers.
Each operation's `x-access` says who may call it (`public`, `read`, `write` or `admin`), and
the same test checks it against the route's declared access.

## Beers

The beer library is managed through these endpoints (details in `api/openapi.yaml`):

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/beers` | read | Current beers by name; `?archived=true` lists only archived ones |
| `GET` | `/api/beers/{id}` | read | One beer, archived or not |
| `POST` | `/api/beers` | write | Add a beer |
| `PATCH` | `/api/beers/{id}` | write | Edit, archive (`{"archived": true}`) or restore (`{"archived": false}`) |
| `DELETE` | `/api/beers/{id}` | write | Delete a beer permanently |
| `GET` | `/api/srm-colors` | read | The SRM color table |

Reads need an admin session or any token (`read` or `write`); with no credential they get 401.
The full library is not public: feature 5 adds a public menu endpoint with only what the TV
shows. Changes need an admin session or a `write` token; a `read` token gets 403. `PATCH`
merges: fields left out are kept, `null` clears an optional field, and a value sets it.
`source` and `source_ref` cannot be set by clients.

Beers have no uploaded images. Each beer's artwork is drawn by the web app as inline SVG, a
corny keg filled with the `color` the API derives from the beer's SRM. Only a beer on a tap is
drawn as a pint glass; taps arrive in feature 4, so for now every beer is a keg.

## Restore from a backup

Before it applies pending migrations, the API copies the database to
`data/backups/beermenu-<timestamp>-v<N>.db`, where `<N>` is the schema version it was taken
from. It keeps the 5 newest copies. To go back to one of them:

1. Stop the stack:

   ```sh
   docker compose down
   ```

2. Copy the chosen backup over the database and delete the stale WAL files:

   ```sh
   cp data/backups/beermenu-<timestamp>-v<N>.db data/beermenu.db
   rm -f data/beermenu.db-wal data/beermenu.db-shm
   ```

3. Give the file to the container user and make it readable only by that user:

   ```sh
   chown "$PUID:$PGID" data/beermenu.db
   chmod 0600 data/beermenu.db
   ```

4. Start the stack:

   ```sh
   docker compose up -d
   ```

The API backs up the restored file and re-applies any migrations newer than `v<N>`.

A restore brings back admins, passwords and tokens as they were when the backup was taken.
Tokens revoked since the backup must be revoked again, and admins removed or passwords
changed since then must be removed or changed again.

Backups hold password and token hashes. Keep them, like the database, at mode `0600` and
owned by the container user.

## Releases

Releases are cut by hand from `main`. Push a `vX.Y.Z` tag:

```sh
git tag vX.Y.Z && git push origin vX.Y.Z
```

The `release` workflow builds `ghcr.io/helmnet/beer-menu-api` and
`ghcr.io/helmnet/beer-menu-web`, scans each with Trivy, and publishes it only if the scan
passes, with three tags: `X.Y.Z`, `X.Y` and `latest`. Tags that do not match `vX.Y.Z` exactly
(for example `v1.2.3-rc1`) publish nothing.
Each image is published on its own: if one image fails its scan, the other is still published.
Re-run the failed job once a fix is available, or release the next `X.Y.Z`.
The image reports `X.Y.Z` as `version` in `/api/health`.

Production pins an exact `X.Y.Z` in the `truenas_apps` repo and never uses `latest`.

## Required checks

The `ci` workflow runs on every pull request and every push to `main`, except changes that touch
only Markdown files:

- `api`: `go vet` and `go test` in `api/`.
- `web`: `npm ci`, lint, tests and build in `web/`.
- `openapi`: Redocly lint of `api/openapi.yaml`.
- `images`: builds both images, checks their default user is non-root and that both refuse to
  run as root, scans both with Trivy for HIGH or CRITICAL vulnerabilities that have a fix, and
  runs a smoke test of the stack as UID/GID `12345:12345`.

The `secrets` workflow runs on its own on every pull request and every push to `main`, Markdown-only
changes included:

- `secrets`: gitleaks scan of the full git history.

Never merge a pull request while any of the five checks (`api`, `web`, `openapi`, `images`,
`secrets`) is failing or pending; on a Markdown-only pull request only `secrets` runs, so it is
the only check to wait for. If `ci` did not run, confirm that the pull request really changes only
Markdown before merging: `git diff --no-renames --name-only origin/main...<branch> | grep -v '\.md$'`
must print nothing. GitHub's path filter reads only the first 300 changed files, so it is not proof
on its own. The repo is private on a free GitHub org plan, which has no
branch protection, so this rule is followed by hand. When the repo goes public, turn on branch
protection for `main` requiring all five checks and up-to-date branches. A skipped workflow leaves
its required checks pending, so Markdown-only pull requests would then block unless that is handled
at that time (for example, require only `secrets`, or add a no-op job).
