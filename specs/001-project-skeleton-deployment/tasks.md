---

description: "Task list for feature 1: project skeleton and deployment"
---

# Tasks: Project Skeleton and Deployment

**Input**: Design documents from `specs/001-project-skeleton-deployment/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi.yaml](./contracts/openapi.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable: Go tests run against real
SQLite files in `t.TempDir()`, never mocks; every migration is applied to a database at the
previous version; React components with logic get Vitest tests.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its
own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an unfinished task)
- **[Story]**: the user story the task belongs to (US1 to US5)

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`)
- Web: `web/` (Vite + React + TypeScript)
- Repo root: `docker-compose.yaml`, `.github/workflows/`, `README.md`

## Conventions for every task

- Every exported Go symbol, every HTTP handler and every React component gets a doc comment
  saying what it does and any contract it holds. Inline comments explain why, never what
  (constitution VI).
- No secrets, credentials, private hostnames or IP addresses in any file (constitution V).
- Do not add dependencies beyond research R18.

---

## Phase 1: Setup

**Purpose**: create both projects and shared repo files.

- [X] T001 Create the Go module in `api/go.mod` with `module github.com/helmnet/beer-menu/api` and `go 1.27`; add `modernc.org/sqlite` with `go get`, producing `api/go.sum`
- [X] T002 [P] Scaffold the web app in `web/`: `web/package.json` (scripts `dev`, `build` = `tsc --noEmit && vite build`, `lint` = `tsc --noEmit`, `test` = `vitest run`), `web/tsconfig.json` (strict), `web/index.html`, `web/src/main.tsx`; dependencies `react`, `react-dom`; dev dependencies `vite`, `@vitejs/plugin-react`, `typescript`, `@types/react`, `@types/react-dom`, `vitest`, `@testing-library/react`, `jsdom`; commit `web/package-lock.json`
- [X] T003 [P] Configure Vite and Vitest in `web/vite.config.ts`: React plugin, `test.environment = "jsdom"`, dev-server proxy of `/api` to `http://localhost:8080`
- [X] T004 [P] Extend `.gitignore` with `data/`, `web/node_modules/`, `web/dist/`, `api/bin/`, `*.db`, `*.db-wal`, `*.db-shm` (keep existing entries)
- [X] T005 [P] Add `api/.dockerignore` and `web/.dockerignore` excluding `node_modules`, `dist`, `bin`, `*.db*`, `.git`

**Checkpoint**: `go build ./...` in `api/` and `npm ci && npm run build` in `web/` both succeed.

---

## Phase 2: Foundational (blocks all user stories)

**Purpose**: open the database and run migrations. Every story needs a migrated database.

- [X] T006 Create migration `api/internal/store/migrations/0001_baseline.sql`: `CREATE TABLE settings (key TEXT PRIMARY KEY NOT NULL, value TEXT NOT NULL);` with no inserted rows (data-model "Migration 0001")
- [X] T007 Implement `Open(dataDir string) (*sql.DB, error)` in `api/internal/store/open.go`: database path `<dataDir>/beermenu.db`; pragmas `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL`, `temp_store=MEMORY` (research R3; the `scratch` image has no `/tmp`); after open, `chmod 0600` the database file and any existing `-wal`/`-shm` files (covers a database restored from elsewhere; new files get `0600` from the umask set in T016)
- [X] T008 Implement `CheckWritable(dir string) error` in `api/internal/store/open.go`: create and remove a probe file; the error message names the folder (spec edge case 1)
- [X] T009 Implement the migrator in `api/internal/store/migrate.go` (research R4, data-model "Migration"): embed `migrations/*.sql` with `embed.FS`; `LoadMigrations(fsys fs.FS) ([]Migration, error)` parses names `NNNN_snake_name.sql` and rejects bad names, duplicates and gaps (must be numbered `1..max`); `Version(db) (int, error)` reads `PRAGMA user_version`; `Pending(current int, ms []Migration) []Migration`; `Apply(db, ms []Migration) error` runs each migration in its own transaction together with `PRAGMA user_version = NNNN` and stops at the first failure, leaving that migration rolled back; `ErrNewerDatabase` returned by a `CheckVersion(current, newest int) error` helper when `current > newest`
- [X] T010 [P] Write migrator tests in `api/internal/store/migrate_test.go` using real SQLite files in `t.TempDir()` and a `fstest.MapFS` of test migrations: fresh database goes from 0 to newest; each embedded migration applied to a database at the previous version (constitution IV); bad name, duplicate and gap are rejected; a failing second migration leaves `user_version = 1` and no partial changes from migration 2; `CheckVersion` returns `ErrNewerDatabase` when the database is newer
- [X] T011 [P] Write `Open`/`CheckWritable` tests in `api/internal/store/open_test.go`: WAL mode is on; database file mode is `0600`; `CheckWritable` on a `0500` folder returns an error containing the folder path (skip when running as root)

**Checkpoint**: `go test ./internal/store/...` passes.

---

## Phase 3: User Story 1 - Start the app with one command (Priority: P1) 🎯 MVP

**Goal**: `docker compose up` serves a blank admin page at `/`, a placeholder at `/display`, and
`GET /api/health` through one origin, with the database created and migrated in `/data`.

**Independent Test**: quickstart section 1 on an empty `./data` folder owned by `$PUID:$PGID`.

### Tests for User Story 1

- [X] T012 [P] [US1] Write the health handler test in `api/internal/httpapi/health_test.go` against a real migrated SQLite database: `GET /api/health` returns 200 with JSON exactly `{"status":"ok","version":"<v>","database":"ok"}`; after `db.Close()` it returns 503 with `{"status":"error","version":"<v>","database":"unreachable"}`; `Content-Type: application/json`
- [X] T013 [P] [US1] Write `web/src/App.test.tsx`: with `window.history.pushState` to `/`, `App` renders the admin page heading "Beer Menu"; at `/display` it renders the display placeholder

### Implementation for User Story 1

- [X] T014 [US1] Implement the route table in `api/internal/httpapi/routes.go`: an exported `Routes` slice of `{Method, Path string; Handler func(deps) http.Handler}` entries (the single source of routes, research R13) and `NewMux(deps Deps) *http.ServeMux` registering each as `"<METHOD> <path>"`; `Deps` holds `*sql.DB` and `Version string`
- [X] T015 [US1] Implement `GET /api/health` in `api/internal/httpapi/health.go` per `contracts/openapi.yaml`: `PingContext` plus `SELECT 1` with a 2 s timeout; 200 `{"status":"ok",...,"database":"ok"}` or 503 `{"status":"error",...,"database":"unreachable"}`; returns no other data
- [X] T016 [US1] Implement `api/cmd/beer-menu-api/main.go`: first call `syscall.Umask(0o077)` so every database, `-wal`, `-shm` and backup file is created `0600` and every folder `0700` (research R5); `var version = "dev"` (set by `-ldflags "-X main.version=..."`, research R8); env `BEER_MENU_DATA_DIR` (default `/data`) and `BEER_MENU_PORT` (default `8080`); refuse to run as root: `checkNotRoot(uid, gid int, groups []int) error` returns an error when `uid == 0 || gid == 0` or `groups` contains 0, and `main` calls it with `os.Geteuid()`, `os.Getegid()` and `os.Getgroups()` right after the umask and exits 1 with a message naming the UID, GID and groups (FR-006); cover it with a table test (`0:0`, `0:1000`, `1000:0`, `1000:1000` with groups `[0 1000]` fail; `1000:1000` with no groups or groups `[1000]` passes) in `api/cmd/beer-menu-api/startup_test.go`; put the startup sequence in `func startup(ctx context.Context, cfg config, migrationsFS fs.FS, now func() time.Time) (*sql.DB, error)` in `api/cmd/beer-menu-api/startup.go` so tests can call it, in data-model "Startup state flow" order: `CheckWritable`, `Open`, `LoadMigrations`, `Version`, `CheckVersion` (error, file unchanged, if newer), `Apply` pending; `main` calls it, exits 1 on error, then listens; log each step with `log/slog`; graceful shutdown on SIGTERM/SIGINT closing the database; `healthcheck` subcommand that GETs `http://127.0.0.1:$BEER_MENU_PORT/api/health` with a 3 s timeout and exits 0 on 200, else 1 (research R9)
- [X] T017 [P] [US1] Copy `specs/001-project-skeleton-deployment/contracts/openapi.yaml` to `api/openapi.yaml` and embed it via `api/openapi.go` (`//go:embed openapi.yaml` into exported `OpenAPI []byte`, package `api`); not served in this feature
- [X] T018 [P] [US1] Create `web/src/pages/AdminPage.tsx` (heading "Beer Menu", one line "Admin coming soon") and `web/src/pages/DisplayPage.tsx` (full-screen dark placeholder, text "Tap list coming soon")
- [X] T019 [US1] Implement `web/src/App.tsx`: render `DisplayPage` when `window.location.pathname` is `/display` or starts with `/display/`, else `AdminPage` (no router library, research R11); render it from `web/src/main.tsx`
- [X] T020 [P] [US1] Write `api/Dockerfile` (research R9): stage `golang:1.27-alpine`, `ARG VERSION=dev`, `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/beer-menu-api ./cmd/beer-menu-api`; final stage `scratch` with the binary only (CA certificates come with the first outbound TLS call, feature 6); `USER 65532:65532` (non-root default only; must run under any non-root UID/GID, research R19); `EXPOSE 8080`; `HEALTHCHECK --interval=10s --timeout=3s --start-period=10s CMD ["/beer-menu-api","healthcheck"]`; `ENTRYPOINT ["/beer-menu-api"]`
- [X] T021 [P] [US1] Write the static `web/nginx/default.conf` (research R10; no template, no env substitution): `listen 8080`; `resolver 127.0.0.11 valid=10s`; `location /api/ { set $api http://api:8080; proxy_pass $api; }` with `Host`, `X-Forwarded-For`, `X-Forwarded-Proto` headers; `location / { try_files $uri /index.html; }`; `server_tokens off` (the base image's `nginx.conf` already keeps PID and temp paths in `/tmp`)
- [X] T022 [US1] Write `web/Dockerfile` (research R10): stage `node:26-alpine` runs `npm ci && npm run build`; final stage `nginxinc/nginx-unprivileged:alpine`; copy `dist/` to `/usr/share/nginx/html` and `nginx/default.conf` to `/etc/nginx/conf.d/default.conf`; no `chown`, no `chmod`, nothing made world-writable (constitution v1.1.0); `USER root` and `RUN apk upgrade --no-cache` (the base image lags Alpine security fixes), then `USER 65532:65532` (non-root default only, research R19); `EXPOSE 8080`; `HEALTHCHECK CMD wget -q --spider http://127.0.0.1:8080/ || exit 1`
- [X] T023 [US1] Write `docker-compose.yaml` at repo root (research R17): service `api` with `image: ghcr.io/helmnet/beer-menu-api:${BEER_MENU_VERSION:-latest}`, `build: ./api`, `user: "${PUID:-65532}:${PGID:-65532}"`, volume `${BEER_MENU_DATA:-./data}:/data`, no published port, `restart: unless-stopped`; service `web` with `image: ghcr.io/helmnet/beer-menu-web:${BEER_MENU_VERSION:-latest}`, `build: ./web`, `user: "${PUID:-65532}:${PGID:-65532}"`, ports `${BEER_MENU_WEB_PORT:-8080}:8080` (host port; distinct from the API's in-container `BEER_MENU_PORT`), `restart: unless-stopped`; a header comment saying production pins an exact `X.Y.Z` in `truenas_apps` and never uses `latest`, and that the deployer sets `PUID`/`PGID` to any non-root UID/GID (never 0) that owns the data folder
- [X] T024 [US1] Run quickstart section 1 by hand: both containers `healthy` within 30 s, `/`, `/display` (also after refresh) and `/api/health` behave as listed; `data/beermenu.db` is owned by `$PUID:$PGID` with mode `0600`; restart keeps data with no migration; with `api` stopped, `/` loads and `/api/health` returns 502. Fix any defect found in the files above

**Checkpoint**: User Story 1 works end to end; this is the MVP.

---

## Phase 4: User Story 2 - Upgrade data safely (Priority: P1)

**Goal**: no migration runs without a backup; keep 5 backups; refuse to migrate if the backup
fails.

**Independent Test**: `go test ./internal/store/... ./cmd/...` in `api/` covers every acceptance
scenario of US2.

### Tests for User Story 2

- [X] T025 [P] [US2] Write backup tests in `api/internal/store/backup_test.go` with real SQLite files: backup file name matches `^beermenu-\d{8}T\d{6}\.\d{3}Z-v\d+\.db$` and contains `-v<from>`; the copy opens and holds the same rows; file mode `0600`, folder mode `0700`; folder is created when missing; two backups in the same second get different names (inject the clock); with 5 existing matching files, a new backup leaves exactly 5 (oldest removed); a non-matching file such as `notes.txt` or `beermenu-manual.db` is never deleted; an unwritable backups folder returns an error and leaves no file with a final name
- [X] T026 [P] [US2] Write startup-sequence tests in `api/cmd/beer-menu-api/startup_test.go`: database at v1 with v2 pending makes one backup named `-v1` and ends at v2; no pending migrations makes no backup; a brand-new database (v0) migrates with no backup; unwritable backups folder returns an error and `user_version` stays 1; a failing v2 migration returns an error, `user_version` stays 1, and the backup remains; a database newer than the newest migration returns `ErrNewerDatabase` and the file's bytes are unchanged

### Implementation for User Story 2

- [X] T027 [US2] Implement `Backup(db *sql.DB, dataDir string, fromVersion int, now time.Time) (string, error)` in `api/internal/store/backup.go` (research R5, data-model "Backup file"): create `<dataDir>/backups` with mode `0700` if missing; `VACUUM INTO` a temporary name in that folder; `chmod 0600`; rename to `beermenu-<now.UTC() formatted 20060102T150405.000Z>-v<from>.db`; on any error remove the temporary file and return the error
- [X] T028 [US2] Implement `PruneBackups(dir string, keep int) error` in `api/internal/store/backup.go`: list files matching `^beermenu-\d{8}T\d{6}\.\d{3}Z-v\d+\.db$`, sort by name, delete all but the newest `keep`; never touch non-matching files
- [X] T029 [US2] In `startup(...)` in `api/cmd/beer-menu-api/startup.go`, add the backup step from data-model "Startup state flow": when `0 < current < newest`, call `Backup` then `PruneBackups(dir, 5)` before `Apply`; if `Backup` fails, return an error without applying anything; `main.go` logs the error and exits 1

**Checkpoint**: US1 still passes; all US2 tests pass.

---

## Phase 5: User Story 3 - Restore from a backup (Priority: P2)

**Goal**: documented restore steps that are proven by a test.

**Independent Test**: `go test ./... -run Restore` in `api/`, plus quickstart section 3 once by
hand.

- [X] T030 [P] [US3] Write `TestRestore` in `api/cmd/beer-menu-api/restore_test.go` (research R16): start at v1 with a `settings` row `title=before`; back up; change the row to `after`; close; copy the backup over `beermenu.db` and delete `beermenu.db-wal` and `beermenu.db-shm`; run `startup` with a migration set that adds v2; assert the row is `before` and `user_version` is 2
- [X] T031 [US3] Write the "Restore from a backup" section in `README.md`: stop with `docker compose down`; copy the chosen `data/backups/beermenu-<timestamp>-v<N>.db` over `data/beermenu.db`; delete `data/beermenu.db-wal` and `data/beermenu.db-shm`; set the owner to the container user (`$PUID:$PGID`) and mode `0600`; `docker compose up -d`; the API re-applies newer migrations. Note that backups hold password and token hashes and must keep `0600`
- [X] T032 [US3] Run quickstart section 3 once by hand against containers and record the date and result in `specs/001-project-skeleton-deployment/quickstart.md` under section 3

**Checkpoint**: restore proven automatically and once by hand.

---

## Phase 6: User Story 4 - Publish a release (Priority: P2)

**Goal**: pull requests are built and tested; `vX.Y.Z` tags publish both images to GHCR.

**Independent Test**: open a PR and see all checks run; push `v0.1.0` and see both images with
tags `0.1.0`, `0.1`, `latest`.

- [X] T033 [P] [US4] Write `.github/workflows/ci.yml` (research R14), triggers `pull_request` and `push` to `main`, `permissions: contents: read`; jobs: `api` (`actions/setup-go` with `go-version-file: api/go.mod`; `go vet ./...`; `go test ./...` in `api/`), `web` (`actions/setup-node` Node 26 with npm cache; `npm ci`, `npm run lint`, `npm test`, `npm run build` in `web/`), `openapi` (`npx --yes @redocly/cli@<exact version> lint api/openapi.yaml`), `images` (`docker compose build`, which tags both images with the compose file's image names so the next step never pulls from GHCR; for each image, read `docker image inspect -f '{{.Config.User}}'` and fail unless it has the form `user:group` with neither part empty, `0` or `root` (an image with no explicit group runs as GID 0 in `scratch`); `docker save` each image to `$RUNNER_TEMP/image.tar` and scan it with the `aquasec/trivy:<exact version>@sha256:<digest>` container (`image --input /image.tar --scanners vuln --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1`); then `mkdir data && sudo chown 12345:12345 data`, `PUID=12345 PGID=12345 docker compose up -d --no-build` (proves an arbitrary UID/GID works), poll `curl -fsS localhost:8080/api/health` for up to 60 s, also `curl -fsS localhost:8080/display`; `docker compose logs` on failure), `secrets` (checkout with `fetch-depth: 0`, then run the pinned `zricethezav/gitleaks:<exact version>` container with `detect --source /repo --redact`; not `gitleaks-action`, which needs a paid license secret for organization repos). Pin every action to a full commit SHA with a version comment
- [X] T034 [P] [US4] Write `.github/workflows/release.yml` (research R15): trigger `push: tags: ['v*.*.*']`; `permissions: contents: read, packages: write`; first step fails unless `GITHUB_REF_NAME` matches `^v[0-9]+\.[0-9]+\.[0-9]+$`; matrix over `api` and `web` with `fail-fast: false` (each image publishes independently); `docker/login-action` to `ghcr.io` with `${{ github.actor }}` and `${{ secrets.GITHUB_TOKEN }}`; `docker/metadata-action` with image `ghcr.io/helmnet/beer-menu-${{ matrix.image }}` and tags `type=semver,pattern={{version}}`, `type=semver,pattern={{major}}.{{minor}}`, `type=raw,value=latest`; `docker/build-push-action` with `context: ./${{ matrix.image }}`, `platforms: linux/amd64`, `push: false`, `load: true`, `build-args: VERSION=${{ steps.meta.outputs.version }}`; then the same Trivy tarball scan as CI, then `docker push --all-tags` only if the scan passes; actions pinned to SHAs
- [X] T035 [US4] Write the "Releases" and "Required checks" sections in `README.md`: how to tag (`git tag vX.Y.Z && git push origin vX.Y.Z`), the three published tags, that production pins `X.Y.Z` in `truenas_apps`, and that a PR is never merged while any of the `api`, `web`, `openapi`, `images` and `secrets` checks fails (branch protection once the repo is public)
- [ ] T036 [US4] Deferred until the repo goes public (private repos on a free GitHub org plan have no branch protection; until then the owner never merges a red PR, per FR-025 and README "Required checks"). Then turn on branch protection for `main` requiring the `api`, `web`, `openapi`, `images` and `secrets` checks (strict, branch up to date), with `gh api -X PUT repos/helmnet/beer-menu/branches/main/protection` or the GitHub UI; needs the owner's admin rights (FR-025); `ci` skips Markdown-only changes, so their required checks stay pending and would block those PRs unless handled then (e.g. require only `secrets`, or add a no-op job)
- [X] T037 [US4] After merge, push `v0.1.0` and run quickstart section 4: both images in GHCR with `0.1.0`, `0.1`, `latest`; `BEER_MENU_VERSION=0.1.0 docker compose up -d --pull always --no-build` reports `"version":"0.1.0"` from `/api/health`. Requires the owner to push the tag by hand

**Checkpoint**: a PR shows all five checks and cannot merge while one fails; a tag publishes both images.

---

## Phase 7: User Story 5 - API contract from day one (Priority: P3)

**Goal**: the OpenAPI document lists every route, enforced by a test.

**Independent Test**: `go test ./internal/httpapi/... -run Routes` passes, and fails when a route
is added to `Routes` without a matching path in `api/openapi.yaml`.

- [X] T038 [US5] Add `gopkg.in/yaml.v3` as a test-only import and write `TestRoutesDocumented` in `api/internal/httpapi/routes_test.go`: parse `api.OpenAPI`, collect each `paths` key and its lowercase method keys, and fail for every `Routes` entry whose method and path are not documented
- [X] T039 [US5] Add the "API" section to `README.md`: `api/openapi.yaml` is the contract; every endpoint change updates it in the same change (constitution I); CI lints it and checks route coverage

**Checkpoint**: contract enforced in CI.

---

## Phase 8: Polish & Cross-Cutting

- [X] T040 [P] Write the top of `README.md`: what the project is, layout (`api/`, `web/`), local development (`go run ./cmd/beer-menu-api` with `BEER_MENU_DATA_DIR=./data`, `npm run dev`), and running with `docker compose up -d --build`; no private hostnames or IPs
- [X] T041 [P] Pin `@redocly/cli` and the `zricethezav/gitleaks` image to exact versions released at least two weeks ago in `.github/workflows/ci.yml` and replace `<pinned>` in `specs/001-project-skeleton-deployment/quickstart.md` section 5
- [X] T042 Review every exported Go symbol, handler and React component for a doc comment, and remove comments that restate code (constitution VI) across `api/` and `web/src/`
- [X] T043 Run the full quickstart (sections 1 to 6) and confirm SC-001 to SC-008; note any criterion not met in the PR description

---

## Dependencies & Execution Order

### Phase dependencies

- Setup (Phase 1): none.
- Foundational (Phase 2): after Setup. Blocks every story.
- US1 (Phase 3): after Foundational.
- US2 (Phase 4): after Foundational; T029 edits the startup code from T016, so do it after US1.
- US3 (Phase 5): after US2 (uses `Backup` and `startup`).
- US4 (Phase 6): after US1 (CI builds images and runs the compose smoke test). T037 needs the
  PR merged; T036 waits until the repo goes public.
- US5 (Phase 7): after T014 and T017.
- Polish (Phase 8): after all stories.

### Within each story

Tests first and failing, then implementation, then the manual check.

### Parallel opportunities

- Phase 1: T002, T003, T004, T005 together after T001.
- Phase 2: T010 and T011 together after T009.
- US1: T012 and T013 together; T017, T018, T020, T021 together.
- US2: T025 and T026 together.
- US4: T033 and T034 together.
- US5 can run alongside US2 once T014 and T017 are done.

## Parallel Example: User Story 1

```text
Task: "T012 [US1] Health handler test in api/internal/httpapi/health_test.go"
Task: "T013 [US1] App test in web/src/App.test.tsx"

Task: "T017 [US1] Copy contract to api/openapi.yaml and embed it"
Task: "T018 [US1] AdminPage and DisplayPage in web/src/pages/"
Task: "T020 [US1] api/Dockerfile"
Task: "T021 [US1] web/nginx/default.conf"
```

## Implementation Strategy

### MVP first

1. Phases 1 and 2.
2. Phase 3 (US1). Stop and validate with quickstart section 1.

### Incremental delivery

1. US1: the stack runs.
2. US2: upgrades are safe. Required before any real data.
3. US3: restore proven.
4. US4: CI on PRs and releases on tags.
5. US5: contract enforced.
6. Polish, then open the PR. Tag `v0.1.0` after merge.
