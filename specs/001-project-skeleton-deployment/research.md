# Research: Project Skeleton and Deployment

**Feature**: [spec.md](./spec.md) | **Date**: 2026-11-08

Each decision lists what was chosen, why, and what was rejected. The constitution already fixes
Go, SQLite, React + Vite, nginx, two images, GHCR and `vX.Y.Z` tags, so research covers only the
choices left open.

## R1. HTTP router

- **Decision**: standard library `net/http.ServeMux` with Go 1.22+ method and path patterns
  (`GET /api/health`).
- **Rationale**: one endpoint now, and the stdlib mux handles methods and path values. No
  dependency.
- **Alternatives**: chi (allowed by the constitution). Rejected for now; it can be added later if
  middleware grouping gets awkward.

## R2. Go version

- **Decision**: Go 1.27 (matches the local toolchain), set in `go.mod` and the Docker build stage.
- **Rationale**: current stable; supports the mux patterns from R1.
- **Alternatives**: older Go versions. No reason to pin older.

## R3. SQLite driver and settings

- **Decision**: `modernc.org/sqlite` (pure Go, so `CGO_ENABLED=0` and a static binary). On open,
  set `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL`, `temp_store=MEMORY`.
- **Rationale**: the constitution requires a pure-Go driver in WAL mode. `busy_timeout` avoids
  `SQLITE_BUSY` errors under small concurrent load; `synchronous=NORMAL` is the standard safe
  pairing with WAL. `temp_store=MEMORY` keeps SQLite's temp files in memory, because the
  `scratch` image (R9) has no `/tmp`.
- **Alternatives**: `mattn/go-sqlite3` needs cgo and breaks the static binary.

## R4. Migration mechanism

- **Decision**: a small in-repo migrator. Migrations are SQL files named `NNNN_name.sql`, embedded
  with `embed.FS`. The schema version is SQLite's `PRAGMA user_version`. Each migration runs in
  its own transaction together with its `user_version` update, so it applies fully or not at all.
  The migrator takes an `fs.FS`, so tests can feed it extra migrations.
- **Rationale**: about 100 lines, no dependency, and the backup step must run between "detect
  pending" and "apply", which is easy to control in our own code. `user_version` lives in the
  database header, so it cannot drift from a separate table.
- **Alternatives**: `golang-migrate` or `goose`. Rejected: an extra dependency, and hooking a
  mandatory backup in before the first step is awkward.

## R5. Backup naming, retention and permissions

- **Decision**:
  - Name: `beermenu-<UTC timestamp>-v<from>.db`, timestamp format `20060102T150405.000Z`
    (millisecond precision, so two backups in one second stay unique; sorts by time).
  - Retention: match `^beermenu-\d{8}T\d{6}\.\d{3}Z-v\d+\.db$`; sort by name; delete all but the 5
    newest. Files that do not match are never touched.
  - Write with `VACUUM INTO` to a temporary name in `/data/backups/`, `chmod 0600`, then rename to
    the final name. A partial file never has the final name.
  - The API sets its umask to `0077` at startup, so every database, `-wal`, `-shm` and backup
    file is created `0600` and every folder `0700`, with no window where a file is readable by
    others.
  - Folder `/data/backups/` created with mode `0700`. The database file and its `-wal`/`-shm`
    files are set to `0600`. All files belong to the container user (the non-root UID/GID the deployer assigns).
- **Rationale**: matches FR-016 to FR-019 and the edge cases. Rename makes the backup appear
  atomically.
- **Alternatives**: second-precision timestamps (collide); a raw file copy (misses WAL writes).

## R6. When to back up

- **Decision**: back up only when `user_version > 0` and at least one migration is pending.
  `user_version == 0` is a brand-new database with no data.
- **Rationale**: matches the spec's assumption; there is nothing to protect in an empty file.

## R7. Startup checks

- **Decision**: at startup, in order: exit with an error if the process runs as UID 0 or GID 0
  (constitution v1.1.0; the deployer could set `PUID=0`); check the data folder is writable (create and remove a
  probe file) and exit with an error that names the folder if not; open the database; read
  `user_version`; exit without changes if it is greater than the newest known migration; back up
  if needed; migrate; then listen.
- **Rationale**: fails early with a clear message (edge cases 1 and 2, FR-014, FR-017).

## R8. Version reporting

- **Decision**: a package variable `version = "dev"`, set at build time with
  `-ldflags "-X main.version=X.Y.Z"`. The Dockerfile takes a `VERSION` build arg (default `dev`).
  The release workflow passes the tag without the `v`.
- **Rationale**: FR-024; the standard Go pattern.

## R9. API image and container health check

- **Decision**: multi-stage build: `golang:1.27-alpine` builds with `CGO_ENABLED=0`; the final
  stage is `scratch` with the binary and
  `USER 65532:65532` as a non-root default (R19). The binary has a `healthcheck` subcommand that calls
  `http://127.0.0.1:$PORT/api/health` and exits 0 or 1; the Dockerfile `HEALTHCHECK` runs it.
- **Rationale**: `scratch` has no shell or curl, so the binary checks itself. Smallest image.
- **Alternatives**: `distroless/static` (fine, but `scratch` is smaller and enough); alpine plus
  curl (larger, more attack surface).

## R10. Web image

- **Decision**: build stage `node:26-alpine` runs `npm ci && npm run build`. Final stage
  `nginxinc/nginx-unprivileged:alpine`, listening on 8080, `USER 65532:65532` as a non-root default (R19). The nginx
  config is a static file baked into the image at `/etc/nginx/conf.d/default.conf`:
  - `location /api/` proxies to `api:8080`, the compose service name. No `API_UPSTREAM` env and
    no `envsubst` template: the template step writes into `/etc/nginx/conf.d`, which an arbitrary
    UID cannot write to.
  - The upstream host is resolved per request (`resolver` set to Docker's DNS `127.0.0.11` and a
    variable in `proxy_pass`), so nginx starts and serves static pages while the API is down,
    returning 502 for `/api` (edge case 3).
  - `location /` uses `try_files $uri /index.html` for client routes (FR-003).
  - The base image's `nginx.conf` already puts the PID file and temp paths under `/tmp` (sticky
    and world-writable in the base image, not made so by ours), so nginx runs as any non-root UID
    with no `chown` or `chmod` in our Dockerfile.
  - `HEALTHCHECK` uses `wget -q --spider http://127.0.0.1:8080/`.
  - The final stage runs `apk upgrade --no-cache` as root before `USER 65532:65532`, because the
    upstream base image lags Alpine security fixes.
- **Rationale**: the unprivileged image is built for non-root; per-request resolution decouples
  startup order.
- **Alternatives**: plain `nginx` image (needs root for port 80 and its paths); static
  `proxy_pass http://api:8080` (nginx fails to start if the API name does not resolve yet).

## R11. Frontend routing

- **Decision**: no router library in this feature. `App` picks the page from
  `window.location.pathname` (`/display` shows the display placeholder, anything else the admin
  placeholder).
- **Rationale**: two static placeholder pages. A router is added when feature 2 needs real
  navigation (constitution VII).

## R12. Frontend tests

- **Decision**: Vitest + `@testing-library/react` + `jsdom`. One test checks that `App` renders
  the admin page at `/` and the display placeholder at `/display`.
- **Rationale**: constitution IV names this stack.

## R13. OpenAPI document and route coverage

- **Decision**: `api/openapi.yaml`, OpenAPI 3.1. CI lints it with `@redocly/cli lint` (pinned
  version, run with `npx`). A Go test compares the mux's registered routes with the document's
  paths: routes are declared in one `routes` slice in the HTTP package; the test reads the YAML
  path keys with `gopkg.in/yaml.v3` (test-only dependency) and fails on any undocumented route.
  The file is also embedded in the binary so later features can serve it; this feature does not
  serve it.
- **Rationale**: FR-027 and FR-028. A single route slice is the only way to list routes with the
  stdlib mux.
- **Alternatives**: generate OpenAPI from code (heavy tooling); regex the YAML (fragile).

## R14. CI (pull requests)

- **Decision**: `.github/workflows/ci.yml` on `pull_request` and `push` to `main`. Jobs:
  - `api`: `go vet`, `go test ./...` (includes migration, backup, restore and route-coverage
    tests against real SQLite files in temp folders).
  - `web`: `npm ci`, `npm run lint` (`tsc --noEmit`), `npm test`, `npm run build`.
  - `openapi`: redocly lint.
  - `images`: `docker compose build` (tags the images with the compose file's names), check that
    each image's configured user is `user:group` with neither part empty, `0` or `root` (a
    `scratch` image with no group part runs as GID 0), then `docker compose up -d --no-build`
    with `PUID=12345 PGID=12345` and `./data` owned by `12345:12345`, which proves the images run
    under a UID/GID they were not built for and poll the health endpoint through the web
    container (smoke test of User Story 1). Before the smoke test, the `aquasec/trivy`
    container, pinned by tag and digest, scans a `docker save` tarball of each image (no Docker
    socket) and fails on HIGH or CRITICAL findings that have a fix.
  - `secrets`: gitleaks scan (SC-007), run as the pinned `zricethezav/gitleaks` container.
    `gitleaks-action` is rejected: it needs a paid license secret for organization repos.
  Branch protection on `main` makes these required checks. It is turned on once, after the
  first CI run creates the check names, and documented in the README.
- **Rationale**: FR-025, constitution workflow rules.

## R15. Release workflow

- **Decision**: `.github/workflows/release.yml` on `push: tags: ['v*.*.*']`. A first step checks
  the tag against `^v[0-9]+\.[0-9]+\.[0-9]+$` and fails otherwise (the glob also matches things
  like `v1.2.3-rc1`). Then for each image: `docker/login-action` with `GITHUB_TOKEN`,
  `docker/metadata-action` with tags `type=semver,pattern={{version}}`,
  `type=semver,pattern={{major}}.{{minor}}` and `type=raw,value=latest`, and
  `docker/build-push-action` with `VERSION` set to the version, loaded locally (`load: true`,
  `push: false`), scanned with the same Trivy tarball step as CI, and pushed with
  `docker push --all-tags` only if the scan passes. Permissions:
  `contents: read`, `packages: write`. Platform `linux/amd64` only (TrueNAS host).
- **Rationale**: FR-022 to FR-024 and FR-026. Only built-in credentials.
- **Alternatives**: multi-arch builds (no ARM target in scope).

## R16. Restore test

- **Decision**: a Go integration test runs the documented path on files: create a database at
  v1 with data, take a backup, change the data, close, replace the file with the backup (and
  remove stale `-wal`/`-shm` files), reopen with a migration set that has v2, and check that the
  data matches the backup and the version is 2. The quickstart also walks the same steps by hand
  with Docker Compose once.
- **Rationale**: FR-021 in CI; the manual pass satisfies the brief's "test once" against real
  containers.

## R17. Example compose file

- **Decision**: `docker-compose.yaml` at the repo root. Services `api` and `web`; each has both
  `image: ghcr.io/helmnet/beer-menu-<name>:${BEER_MENU_VERSION:-latest}` and `build:` pointing at
  its folder, so `docker compose up --build` works from a checkout and `docker compose up` works
  from released images. Both services set `user: "${PUID:-65532}:${PGID:-65532}"`, so the default
  matches the images' default user and the deployer overrides it. `api` mounts
  `${BEER_MENU_DATA:-./data}:/data`, no published port. `web` publishes `${BEER_MENU_WEB_PORT:-8080}:8080`. The host-port variable is named
  `BEER_MENU_WEB_PORT` so it never clashes with the API's in-container `BEER_MENU_PORT`. No secrets.
- **Rationale**: FR-005. Production pins `X.Y.Z` in `truenas_apps`; the example default of
  `latest` is for trying it out only, and the README says so.

## R18. Dependencies added

| Dependency | Where | Why |
| --- | --- | --- |
| `modernc.org/sqlite` | API | Required pure-Go SQLite driver |
| `gopkg.in/yaml.v3` | API tests only | Read OpenAPI paths for the route-coverage test |
| `react`, `react-dom` | Web | Required UI framework |
| `vite`, `@vitejs/plugin-react`, `typescript` | Web dev | Required build tooling |
| `vitest`, `@testing-library/react`, `jsdom` | Web dev | Required component tests |
| `@redocly/cli` | CI only, via `npx` | OpenAPI lint |
| `zricethezav/gitleaks` image | CI only | Secret scan (SC-007) |
| `aquasec/trivy` image, pinned by digest | CI and release only | Image vulnerability scan |

## R19. Container user

- **Decision**: images set `USER 65532:65532` as a non-root default (the common "nonroot" ID,
  not a deployment choice). The deployer overrides it with `user:` in their compose file, for
  example `user: "${PUID}:${PGID}"` in `truenas_apps`. Nothing in the repo depends on a specific
  UID/GID. The API writes only to `/data`, which the deployer owns. The web image writes only to
  `/tmp`. CI runs the stack as `12345:12345` to prove an arbitrary UID works.
- **Rationale**: constitution v1.1.0, Deployment and Data Constraints.
- **Alternatives**: `PUID`/`PGID` env vars read by an entrypoint that starts as root and drops
  privileges (the linuxserver.io pattern). Rejected: it needs root at start, a shell and a
  privilege-drop tool, none of which the `scratch` API image has. Group 0 with group-write paths
  (the OpenShift pattern). Rejected: the constitution forbids GID 0.
