# Beer Menu Constitution

## Core Principles

### I. API-First

The Go REST API is the product. The admin app, the TV display, and future clients (Android APK,
Home Assistant, scripts) are all clients of the same API.

- Every feature MUST be reachable through the REST API. No capability may exist only in a UI.
- The API MUST be JSON over HTTP and described by an OpenAPI document kept in the repo from
  feature 1 onward.
- Every change to an endpoint MUST update the OpenAPI document in the same change.
- Clients MUST NOT read or write the database directly. Only the API container touches SQLite.
- Endpoints that later clients depend on (menu reads, keg level updates) MUST stay backward
  compatible within a major version.

**Rationale**: one documented contract lets the APK, Home Assistant and keg scales plug in later
without API rework.

### II. Go + SQLite Backend

- The API MUST be written in Go, using the standard `net/http` router or a light router such as
  chi. Heavy frameworks are not allowed.
- The API MUST build as a single static binary.
- Storage MUST be SQLite through a pure-Go driver (such as `modernc.org/sqlite`) in WAL mode.
  No separate database service.
- Schema changes MUST go through versioned migrations run at API startup.
- Volumes MUST be stored in litres. Display units are a setting, not a storage format.
- Integrations MUST sit behind a provider interface. The core MUST NOT see provider-specific
  fields.

**Rationale**: a small image and low memory fit the NAS; one database file keeps backup simple.

### III. React Clients

- The admin app and the `/display` route MUST be React + TypeScript, built with Vite.
- The build MUST be served as static files by an nginx container that proxies `/api`, so the
  browser sees one origin.
- The display MUST be readable at 3 m or more on a 1080p landscape TV, use a dark theme by
  default, and update without a manual reload.
- The display MUST run unattended for days and recover from network drops and server restarts
  without anyone touching the TV.

**Rationale**: one origin avoids CORS and cookie problems; the display has no operator.

### IV. Tests Required (NON-NEGOTIABLE)

- Every feature MUST ship with automated tests for its behavior. A feature is not done until its
  tests pass in CI.
- API handlers MUST have tests that run against a real SQLite database, not mocks of it.
- Authentication and authorization MUST have tests for each rule: unauthenticated access, `read`
  versus `write` scope, revoked tokens, CSRF on every browser write (including sign-in and
  first-admin creation), and login rate limiting.
- Every migration MUST be tested by applying it to a database at the previous version.
- React components with logic MUST have component tests (for example Vitest and Testing Library).
- A bug fix MUST include a test that fails without the fix.

**Rationale**: the app runs unattended on a home server; regressions are found by tests, not
guests.

### V. LAN Security Baseline (NON-NEGOTIABLE)

Security is a top priority, and the repo may go public.

- No secrets, credentials, private hostnames or IP addresses may be committed. Configuration
  comes from environment variables or Docker secrets; deployment secrets use repo secrets named
  `BEER_MENU_<VAR>`.
- There are no default credentials. The first admin is created on first run.
- Passwords MUST be hashed with argon2id or bcrypt.
- Admin sessions MUST use HttpOnly, SameSite cookies, with CSRF protection on every write.
- API tokens MUST be shown once, stored hashed, sent as `Authorization: Bearer <token>`,
  revocable, and record a last-used time. Each token MUST have a `read` or `write` scope.
- Only these endpoints are public: menu reads, the health check, the first-run setup status,
  first-admin creation (refused once an admin exists), sign-in, and the read-only API docs (the
  docs page and the OpenAPI document; the page MUST NOT offer a way to send API requests, beyond
  loading the OpenAPI document, and MUST NOT accept credentials).
  Sign-in includes its later steps (second factor, and cancelling an unfinished sign-in); those
  steps MUST act only on a short-lived, single-use pending sign-in that a correct password
  created, held server-side and carried in an HttpOnly, SameSite cookie.
  Every other endpoint MUST
  require a session or a token with the right scope, with one exception: the routes that set up
  a second factor MAY also accept a pending sign-in of an admin who must set one up before
  signing in, and no other route may accept it. Public writes (first-admin creation and
  sign-in, including its later steps) MUST also refuse cross-site browser requests.
- Login MUST be rate limited.
- Third-party secrets (such as the Brewfather API key) MUST be encrypted at rest, with the key
  held outside the database, and MUST never be returned by the API.
- Input at every trust boundary (request bodies, uploads, imported data) MUST be validated.
- TLS is out of scope for the app. It runs behind the NPM reverse proxy on the LAN.

**Rationale**: a home network is not a trust boundary once the code is public or a device is
compromised.

### VI. Documented, Not Verbose

- Every exported Go symbol, every API handler and every React component MUST have a doc comment
  that says what it does and any contract it holds.
- Inline comments MUST explain why: a constraint, trade-off or non-obvious reason. They MUST NOT
  restate what the code does.
- Code MUST be readable without comments where possible: clear names over explanations.

**Rationale**: future clients and a possible public repo need clear contracts; noise comments rot.

### VII. Simplicity and Scope

- Build the smallest thing that meets the current feature's spec. No speculative options,
  abstractions or configuration.
- New dependencies MUST be justified in the feature plan. Prefer the standard library and
  dependencies already in use.
- Never in scope: prices, ratings, ordering, payments, multi-tenant or public cloud hosting.
- Deferred until after the MVP: the Android APK, Home Assistant integration, and data sources
  beyond Brewfather. Specs MUST NOT build these early, but MUST NOT block them.

**Rationale**: a home project stays maintainable only if it stays small.

## Deployment and Data Constraints

- The app runs as two containers, `beer-menu-api` and `beer-menu-web`, started with a single
  `docker compose up`.
- Images are built by GitHub Actions and pushed to `ghcr.io/helmnet` only when a semver tag
  `vX.Y.Z` is pushed by hand. Images are tagged `X.Y.Z`, `X.Y` and `latest`.
- Production deployment pins an exact `X.Y.Z` tag in `truenas_apps`. It never runs `latest`.
- Containers MUST run as a non-root user and group, never UID 0 or GID 0. Images MUST set a
  non-root default user and MUST work under any non-root UID/GID the deployer assigns (for
  example `user:` in the deployer's compose file). The repo MUST NOT assume a specific UID/GID,
  and images MUST NOT make paths world-writable to support an arbitrary UID. The SQLite file
  lives on a bind mount owned by that user.
- Before running pending migrations, the API MUST copy the database with `VACUUM INTO` to
  `/data/backups/`, keeping the last 5 copies. If the copy fails, the API MUST NOT migrate and
  MUST exit with an error.
- Backups MUST have the same owner and permissions as the database, because they hold password
  hashes, token hashes and encrypted secrets.
- The restore path (stop, replace the file, start, re-run newer migrations) MUST be tested once
  in feature 1, before real data exists.

## Development Workflow

- Work follows Spec Kit: this constitution, then one `/speckit-specify` per feature, in the order
  of the feature breakdown in `docs/project-brief.md`.
- Each feature MUST end in something testable, as stated in its "done when" criteria.
- A roadmap phase starts only when the phase before it runs on TrueNAS.
- Each feature is developed on its own branch and merged to `main` by pull request.
- CI MUST run every check that a pull request's changed files can affect, and every check when
  the CI definition, its helper scripts, or files shared by both images (the compose file) change.
  A failing check blocks the merge.
- Every plan MUST include a Constitution Check against these principles. Any deviation MUST be
  recorded in the plan's Complexity Tracking table with the reason and the simpler option that
  was rejected.

## Governance

- This constitution supersedes other project practices. Where a spec, plan or task conflicts
  with it, the constitution wins until it is amended.
- Amendments are made by pull request that edits this file, states the reason, and bumps the
  version. Dependent specs and plans that the change affects MUST be updated or listed as
  follow-ups in the same pull request.
- Versioning follows semantic versioning:
  - MAJOR: a principle is removed or redefined in an incompatible way.
  - MINOR: a principle or section is added, or guidance is materially expanded.
  - PATCH: clarifications and wording fixes with no change in meaning.
- Every pull request review MUST check compliance with these principles, including the
  NON-NEGOTIABLE ones (IV and V). `docs/project-brief.md` is the guidance file for product scope.

**Version**: 1.5.0 | **Ratified**: 2026-11-08 | **Last Amended**: 2026-11-13
