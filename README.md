# beer-menu

A self-hosted digital tap list for a home bar: a React admin app manages the beers and taps,
and a TV shows the current menu in a browser. It runs as two containers: a Go REST API that
owns a SQLite database, and nginx serving the React app and proxying `/api` to the API. Every
client uses the same documented REST API.

This is an early skeleton: the admin page and the `/display` tap list are placeholders, and
the only endpoint is `GET /api/health`.

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

## API

`api/openapi.yaml` is the API contract. Every change to an endpoint updates it in the same
change. CI lints it with Redocly, and the route-coverage test
(`go test ./internal/httpapi/... -run Routes` in `api/`) fails if a route in
`internal/httpapi/routes.go` is not documented in it.

`GET /api/health` is public and reports the app version and whether the database answers.

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

The `ci` workflow runs on every pull request and every push to `main`:

- `api`: `go vet` and `go test` in `api/`.
- `web`: `npm ci`, lint, tests and build in `web/`.
- `openapi`: Redocly lint of `api/openapi.yaml`.
- `images`: builds both images, checks their default user is non-root and that both refuse to
  run as root, scans both with Trivy for HIGH or CRITICAL vulnerabilities that have a fix, and
  runs a smoke test of the stack as UID/GID `12345:12345`.
- `secrets`: gitleaks scan of the full git history.

Never merge a pull request while any of the five checks (`api`, `web`, `openapi`, `images`,
`secrets`) is failing or pending. The repo is private on a free GitHub org plan, which has no
branch protection, so this rule is followed by hand. When the repo goes public, turn on branch
protection for `main` requiring all five checks and up-to-date branches.
