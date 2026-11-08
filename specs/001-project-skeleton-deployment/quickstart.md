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

### Manual run record

- **Date**: 2026-11-08, Docker 29.9.3, Compose 5.7.0, `PUID`/`PGID` = the invoking user.
- **Method**: this release has only migration `0001`, so the stack never backs up on its own.
  A throwaway API image was built from a copy of `api/` outside the repo with one extra
  migration (`0002_restore_check.sql`, creates an empty table) and tagged
  `beer-menu-api:restoretest`. Steps: start the normal stack on an empty `./data` (v1); stop
  it; insert `settings` row `title=before`; start with `BEER_MENU_VERSION=restoretest`, which
  made `backups/beermenu-<timestamp>-v1.db` through the real startup code and migrated to v2;
  stop; change the row to `after`; start again. Then steps 2 to 4 above, word for word, with
  `BEER_MENU_VERSION=restoretest` so the restored v1 file has a pending migration. The
  throwaway image and `./data` were deleted afterwards.
- **Result**: pass. The API log showed `database backed up ... -v1.db` and
  `migrations applied from=1 to=2`; `/api/health` returned `ok`; the row read `before` and
  `user_version` was 2; `data/beermenu.db` stayed mode `0600`.
- **Elapsed (SC-005)**: 2.7 s from `docker compose down` to `/api/health` answering `ok`,
  7.9 s until both containers were `healthy` (target: under 5 minutes).

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
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
cd api && go test ./... -run Routes
```

Expected: lint passes; the route-coverage test passes. Adding a route without documenting it
makes the test fail.

## 6. Secret scan (SC-007)

```sh
docker run --rm -v "$PWD:/repo" zricethezav/gitleaks:v8.30.1 detect --source /repo --redact
```

Expected: no leaks found. In a git worktree, `.git` is a file that points at the main
checkout's `.git` folder; also mount that folder at the same path (`-v <main>/.git:<main>/.git:ro`),
or gitleaks scans 0 commits.
