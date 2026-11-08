# Quickstart: Project Skeleton and Deployment

Validation guide for feature 1. Each section proves one user story from [spec.md](./spec.md).
Paths and names follow [data-model.md](./data-model.md); the health response follows
[contracts/openapi.yaml](./contracts/openapi.yaml).

## Prerequisites

- Docker with Compose v2.
- Go 1.27 and Node 22 (only for running tests outside Docker).
- A checkout of this repo.

## 1. Start the stack (User Story 1)

```sh
export PUID=$(id -u) PGID=$(id -g)   # any non-root UID/GID; never 0
mkdir -p data                         # owned by you, so by PUID:PGID
docker compose up -d --build
```

Expected:

- `docker compose ps` shows `api` and `web` as `healthy` within 30 seconds.
- `curl -s localhost:8080/api/health` returns `{"status":"ok","version":"dev","database":"ok"}`.
- `http://localhost:8080/` shows the blank admin page.
- `http://localhost:8080/display` shows the display placeholder, also after a browser refresh.
- `data/beermenu.db` exists, owned by `$PUID:$PGID`, mode `0600`. `data/backups/` is empty (new
  database, no backup).

Restart check: `docker compose restart api`, then `docker compose logs api` shows no backup and
no migration.

API-down check: `docker compose stop api`. `/` still loads; `/api/health` returns 502.
`docker compose start api` restores it.

## 2. Upgrade with backup (User Story 2)

Run the automated tests:

```sh
cd api && go test ./...
```

They cover: backup before pending migrations, no backup when nothing is pending or the database
is new, retention of 5, refusal when the backup folder is unwritable, rollback of a failed
migration, downgrade refusal, and unique names within one second.

The upgrade path needs a release with a newer migration, so it is proven by these tests, not
by hand. Never edit `user_version` by hand on real data.

## 3. Restore from a backup (User Story 3)

Automated: `go test ./... -run Restore` in `api/`.

Manual, once, against containers (the brief's "test restore once"):

1. Make sure a backup exists in `data/backups/` (run the stack once on a database at an older
   version, or copy one produced by the tests).
2. Stop the stack: `docker compose down`.
3. Replace the database and remove stale WAL files:
   ```sh
   cp data/backups/beermenu-<timestamp>-v<N>.db data/beermenu.db
   rm -f data/beermenu.db-wal data/beermenu.db-shm
   chown "$PUID:$PGID" data/beermenu.db && chmod 0600 data/beermenu.db
   ```
4. Start the stack: `docker compose up -d`.

Expected: `api` is healthy; its log shows a backup of the restored file and the migrations above
`v<N>` applied; `/api/health` returns `ok`.

The same steps are in the README restore section.

## 4. Publish a release (User Story 4)

1. Merge to `main` with green CI.
2. Tag and push: `git tag v0.1.0 && git push origin v0.1.0`.
3. Watch the `release` workflow in GitHub Actions.

Expected:

- `ghcr.io/helmnet/beer-menu-api` and `ghcr.io/helmnet/beer-menu-web` both have tags `0.1.0`,
  `0.1` and `latest`.
- `BEER_MENU_VERSION=0.1.0 docker compose up -d` (no `--build`) starts, and `/api/health`
  reports `"version":"0.1.0"`.
- Pushing `v0.1` or `test` publishes nothing.

## 5. API contract (User Story 5)

```sh
npx @redocly/cli@<pinned> lint api/openapi.yaml
cd api && go test ./... -run Routes
```

Expected: lint passes; the route-coverage test passes. Adding a route without documenting it
makes the test fail.

## 6. Secret scan (SC-007)

```sh
docker run --rm -v "$PWD:/repo" zricethezav/gitleaks:latest detect --source /repo
```

Expected: no leaks found.
