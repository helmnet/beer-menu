# Feature Specification: Project Skeleton and Deployment

**Feature Branch**: `compmodder26/feature_1`

**Created**: 2026-11-08

**Status**: Draft (amended 2026-11-08 for constitution v1.1.0: the container UID/GID is set by
the deployer, not fixed at `568:568` as the input below says)

**Input**: User description: "Feature 1: Project skeleton and deployment (see docs/project-brief.md). Go REST API and React + TypeScript (Vite) admin app, each built into its own Docker image (beer-menu-api, beer-menu-web). The web image is nginx serving the React build and proxying /api to the API container (one origin). Pushing a semver tag vX.Y.Z by hand triggers GitHub Actions to build and publish both images to ghcr.io/helmnet tagged X.Y.Z, X.Y and latest. `docker compose up` serves a health endpoint and a blank admin page (with a placeholder /display route). SQLite (pure-Go driver, WAL mode) on a bind-mounted volume (/data, run as 568:568) with versioned migrations run at startup; before applying pending migrations the API makes a VACUUM INTO backup to /data/backups/beermenu-<timestamp>-v<from>.db, keeps the last 5, and refuses to migrate (exits with error) if the backup fails. Test the documented restore path once. An OpenAPI document lives in the repo from this feature onward. No secrets, credentials or private hostnames committed. Deployment compose file in truenas_apps is out of scope for this repo but an example docker-compose.yaml is provided."

## Context

This is the first feature after the constitution. It delivers no menu functionality. It proves
that the two-container app builds, starts with one command, stores data safely on the home
server, upgrades its data without losing it, and ships through a repeatable release. Every later
feature (authentication, beers, taps, display, Brewfather import) builds on this skeleton.

The technology choices named here (two containers, SQLite file, versioned migrations, image
registry, release tags) are fixed by the project constitution and brief. This spec records them
as constraints, not as design decisions to revisit.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Start the app with one command (Priority: P1)

The owner copies the example compose file, points it at a data folder on the server, and starts
the stack with one command. They open the app's address in a browser and see a blank admin page.
A health check confirms the API is up and the database is reachable.

**Why this priority**: nothing else can be built or tested until the stack starts and serves
both the web app and the API from one address.

**Independent Test**: on a clean machine with only a container runtime, run the example compose
file against an empty data folder. Open the root page, the `/display` page and the health
endpoint through the web container's address.

**Acceptance Scenarios**:

1. **Given** an empty data folder owned by the container user, **When** the owner starts the stack
   with the example compose file, **Then** both containers start, the database file is created
   in the data folder, and all migrations are applied.
2. **Given** the stack is running, **When** the owner opens the root page in a browser, **Then**
   a blank admin page loads with the app name.
3. **Given** the stack is running, **When** the owner opens `/display`, **Then** a placeholder
   display page loads (no 404), so deep links and browser refreshes work on client routes.
4. **Given** the stack is running, **When** a client requests the health endpoint under `/api`
   through the web container, **Then** it receives a success response that reports the app
   version and that the database is reachable.
5. **Given** the stack is running, **When** the owner restarts it, **Then** existing data is
   kept and no migration or backup runs.

---

### User Story 2 - Upgrade data safely (Priority: P1)

When a new release adds a schema change, the API backs up the database before it changes
anything. If the backup cannot be made, the API refuses to start rather than risk the data.

**Why this priority**: the constitution makes "no migration without a backup" non-negotiable,
and it must work before any real data exists.

**Independent Test**: start the API against a database at an older schema version, confirm a
backup file appears and the schema is upgraded. Repeat with the backup folder unwritable and
confirm the API exits with an error and the database is unchanged.

**Acceptance Scenarios**:

1. **Given** a database at schema version N and a release that has migrations above N, **When**
   the API starts, **Then** it writes a consistent copy named
   `beermenu-<timestamp>-v<N>.db` to the backups folder, then applies the pending migrations.
2. **Given** a database with no pending migrations, **When** the API starts, **Then** no backup
   is made.
3. **Given** a brand-new empty database, **When** the API starts, **Then** it applies all
   migrations without making a backup, because there is no data to protect.
4. **Given** 5 backups already exist, **When** a new backup is made, **Then** the oldest is
   removed so exactly 5 remain.
5. **Given** the backup cannot be written (folder unwritable or disk full), **When** the API
   starts with pending migrations, **Then** it applies no migration, logs a clear error, and
   exits with a non-zero status.
6. **Given** a migration fails part-way, **When** the API starts, **Then** that migration's
   changes are rolled back, the schema stays at the last good version, and the API exits with an
   error. The backup taken before it remains.

---

### User Story 3 - Restore from a backup (Priority: P2)

The owner can recover from a bad upgrade by following the documented restore steps: stop the
stack, replace the database file with a backup, start the stack. The API re-applies any
migrations newer than the backup.

**Why this priority**: a backup is only useful if restore is known to work. The brief requires
this path to be tested once in feature 1.

**Independent Test**: create a database, make a backup, change the data, follow the restore
steps, confirm the data matches the backup and the schema is current.

**Acceptance Scenarios**:

1. **Given** a running stack and a backup file, **When** the owner follows the documented
   restore steps, **Then** the stack starts, the data matches the backup, and the schema is at
   the current version.
2. **Given** the restore test, **When** it runs in CI, **Then** it passes as an automated test.

---

### User Story 4 - Publish a release (Priority: P2)

The owner pushes a semver tag `vX.Y.Z` by hand. The release pipeline builds both images and
publishes them to the project's image registry tagged `X.Y.Z`, `X.Y` and `latest`.

**Why this priority**: production deployment on TrueNAS pins an exact released image. Without
releases, there is nothing to deploy.

**Independent Test**: push a test tag such as `v0.1.0`; confirm both images appear in the
registry with all three tags, and that the images start with the example compose file.

**Acceptance Scenarios**:

1. **Given** a commit on the main branch, **When** the owner pushes tag `v0.1.0`, **Then** both
   images are published with tags `0.1.0`, `0.1` and `latest`.
2. **Given** a tag that is not semver (for example `test` or `v1.2`), **When** it is pushed,
   **Then** no images are published.
3. **Given** a pushed release tag, **When** a client calls the health endpoint on the released
   API, **Then** it reports version `X.Y.Z`.
4. **Given** a pull request, **When** CI runs, **Then** both images are built and all tests run,
   and a failure blocks the merge. Nothing is published.

---

### User Story 5 - API contract from day one (Priority: P3)

A developer of a future client (Android APK, Home Assistant) can read an OpenAPI document in the
repo that describes every endpoint the API serves. In this feature that is the health endpoint.

**Why this priority**: the constitution requires the contract from feature 1, but it has one
endpoint, so its value now is the convention, not the content.

**Independent Test**: validate the OpenAPI document with a standard linter and check that every
route the API serves is listed in it.

**Acceptance Scenarios**:

1. **Given** the repo, **When** the OpenAPI document is validated, **Then** it is valid and
   describes the health endpoint and its response.
2. **Given** the API's registered routes, **When** CI compares them to the document, **Then**
   every route is documented.

---

### Edge Cases

- The data folder is not writable by the container user: the API exits with an error that names the
  folder, instead of failing later.
- The database file is from a newer release than the running API (downgrade): the API refuses
  to start and does not modify the file.
- The API container starts before or restarts while the web container is up: the web container
  keeps serving the static pages and returns a gateway error for `/api` until the API is back.
- The backups folder does not exist yet: the API creates it with the same owner and permissions
  as the database.
- Two backups made within the same second: names stay unique, so neither overwrites the other.
- A file in the backups folder does not match the backup name pattern: retention ignores it and
  never deletes it.

## Requirements *(mandatory)*

### Functional Requirements

**Packaging and startup**

- **FR-001**: The project MUST build two container images: an API image and a web image.
- **FR-002**: The web image MUST serve the admin app's static build and forward every request
  under `/api` to the API container, so the browser sees one origin.
- **FR-003**: The web image MUST return the admin app for any non-`/api` path that is not a
  static file, so client routes such as `/display` load on refresh.
- **FR-004**: The admin app MUST show a blank admin page at `/` and a placeholder page at
  `/display`.
- **FR-005**: The repo MUST include an example compose file that starts both containers with one
  command, mounts a host data folder at `/data`, and contains no secrets, credentials, private
  hostnames or IP addresses.
- **FR-006**: Both images MUST set a non-root default user and MUST run under any non-root
  UID/GID the deployer assigns (never UID 0 or GID 0), without the image making any path
  world-writable. The repo MUST NOT hardcode a deployment UID/GID. The API MUST refuse to start,
  with an error naming the UID/GID, when it runs as UID 0 or GID 0.
- **FR-007**: All runtime configuration (for example the data folder path and listen port) MUST
  come from environment variables with safe defaults. No configuration value may require a
  secret in this feature.

**Health**

- **FR-008**: The API MUST expose a public health endpoint under `/api` that returns success
  only when the database is reachable, and reports the running app version.
- **FR-009**: The health endpoint MUST return a failure status when the database is not
  reachable.
- **FR-010**: The API image MUST declare a container health check that uses the health endpoint.

**Data and migrations**

- **FR-011**: The API MUST store its data in a single database file in the data folder, with
  write-ahead logging enabled.
- **FR-012**: The API MUST record the current schema version in the database and apply pending
  versioned migrations, in order, at startup before it serves requests.
- **FR-013**: Each migration MUST apply fully or not at all.
- **FR-014**: The API MUST refuse to start, without changing the file, when the database's
  schema version is newer than the newest migration it knows.
- **FR-015**: Feature 1 MUST ship at least one migration (an initial schema baseline) so the
  migration and backup path is exercised.

**Backups**

- **FR-016**: When migrations are pending on a database that already has a schema version, the
  API MUST first write a consistent copy of the database to `/data/backups/` named
  `beermenu-<timestamp>-v<from>.db`, where `<from>` is the current schema version.
- **FR-017**: If the backup cannot be written, the API MUST NOT apply any migration and MUST
  exit with a non-zero status and a clear error message.
- **FR-018**: After a successful backup, the API MUST keep only the 5 newest backup files that
  match the backup name pattern and delete older ones.
- **FR-019**: Backup files MUST have the same owner and permissions as the database file, and
  the backups folder MUST NOT be readable by other users.

**Restore**

- **FR-020**: The repo MUST document the restore steps: stop the stack, replace the database
  file with a backup, start the stack.
- **FR-021**: An automated test MUST prove the restore path: data matches the backup and newer
  migrations are re-applied.

**Release and CI**

- **FR-022**: Pushing a tag that matches `vX.Y.Z` MUST build both images and publish them to the
  project's registry under the `helmnet` namespace, tagged `X.Y.Z`, `X.Y` and `latest`.
- **FR-023**: Tags that do not match `vX.Y.Z` MUST NOT publish images.
- **FR-024**: Released images MUST carry the release version so the health endpoint reports it.
  Local builds MUST report a development version.
- **FR-025**: Every pull request MUST build both images and run all tests. A failure MUST block
  the merge: by branch protection once the repo is public, and until then by the owner never
  merging a pull request with a failing or pending check (private repos on a free GitHub org
  plan have no branch protection). Pull requests MUST NOT publish images.
- **FR-026**: Publishing MUST use only the CI system's built-in credentials. No registry
  credentials may be stored in the repo.

**API contract**

- **FR-027**: The repo MUST contain an OpenAPI document that describes every API endpoint,
  starting with the health endpoint.
- **FR-028**: CI MUST validate the OpenAPI document and fail if a route the API serves is
  missing from it.

### Key Entities

- **Schema version**: the number of the last migration applied to the database. Used to decide
  whether migrations are pending, to name backups, and to refuse downgrades.
- **Migration**: an ordered, numbered change to the database schema, applied once.
- **Backup file**: a consistent copy of the database taken before migrating, named with its
  timestamp and the schema version it was taken from.
- **Release**: a semver version `X.Y.Z` that tags both published images and is reported by the
  health endpoint.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a machine with only a container runtime, the owner goes from a fresh checkout
  to a working admin page and a passing health check in under 5 minutes, using one start
  command.
- **SC-002**: After a restart, 100% of stored data is still present.
- **SC-003**: In 100% of tested upgrade runs with pending migrations, a backup exists before the
  first schema change. In 100% of tested backup failures, the schema is unchanged.
- **SC-004**: The backups folder never holds more than 5 app-made backups.
- **SC-005**: Following the documented restore steps returns the data to the backup's state in
  under 5 minutes, with no manual schema work.
- **SC-006**: A release tag push results in both images published with all three tags within
  20 minutes, with no manual steps after the tag push.
- **SC-007**: A secret scan of the repo finds no secrets, credentials, private hostnames or IP
  addresses.
- **SC-008**: Each image starts and passes its health check in under 30 seconds on the target
  server.

## Assumptions

- The production compose file lives in the separate `truenas_apps` repo and is out of scope.
  This repo ships an example compose file only.
- The reverse proxy (NPM) and TLS are out of scope. The web container listens on plain HTTP on
  the LAN.
- The health endpoint is public, like menu reads, and returns no sensitive data (version and
  database status only).
- The admin page has no login in this feature. Authentication arrives in feature 2, so this
  feature exposes no data and no write endpoints.
- A brand-new database needs no backup before its first migrations, because it holds no data.
- Backup timestamps use UTC, so names sort in time order.
- The backups folder is `/data/backups/` inside the data volume, so TrueNAS snapshots cover it
  too.
- The admin app's look is a placeholder. Design and theming arrive with later features.
- Dependabot bump PRs and the self-hosted runner deploy in `truenas_apps` are out of scope here.
