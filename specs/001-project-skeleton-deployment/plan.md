# Implementation Plan: Project Skeleton and Deployment

**Branch**: `compmodder26/feature_1` | **Date**: 2026-11-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-project-skeleton-deployment/spec.md`

## Summary

Stand up the two-container app with no menu features: a Go API with a public health endpoint,
SQLite in WAL mode, embedded versioned migrations, and a mandatory `VACUUM INTO` backup before
any pending migration; and a React + Vite admin app served by unprivileged nginx that proxies
`/api`. An example `docker-compose.yaml` starts both. GitHub Actions runs tests, image builds, a
compose smoke test, OpenAPI lint and a secret scan on every pull request, and publishes both
images to `ghcr.io/helmnet` on a hand-pushed `vX.Y.Z` tag. Details are in
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 with React 18 on Node 26 (web build)

**Primary Dependencies**: `net/http` stdlib mux, `modernc.org/sqlite`; React, Vite; nginx
(`nginxinc/nginx-unprivileged:alpine`)

**Storage**: SQLite file `/data/beermenu.db`, WAL mode, bind mount; backups in `/data/backups/`

**Testing**: `go test` against real SQLite files in temp folders; Vitest + Testing Library;
redocly lint; compose smoke test in CI; gitleaks

**Target Platform**: Linux `amd64` containers on TrueNAS SCALE (Docker Compose)

**Project Type**: web service (API) + web app (static SPA), two images

**Performance Goals**: each container healthy within 30 s of start (SC-008); release published
within 20 min of tag push (SC-006)

**Constraints**: non-root, any UID/GID the deployer assigns (no hardcoded UID/GID); static API binary; no secrets in repo; one origin;
no migration without a backup

**Scale/Scope**: one household, a handful of clients; one endpoint and two placeholder pages in
this feature

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Health endpoint in `api/openapi.yaml`; CI fails on undocumented routes; only the API opens SQLite | Pass |
| II. Go + SQLite | stdlib mux; static binary (`CGO_ENABLED=0`); `modernc.org/sqlite` in WAL; versioned migrations at startup; provider interface not needed yet | Pass |
| III. React Clients | React + TS + Vite; nginx serves build and proxies `/api`; `/display` route exists (dark theme and live updates arrive in feature 5) | Pass |
| IV. Tests Required | Go tests on real SQLite for migrations (each applied to previous version), backup, retention, restore, health handler; Vitest for `App`; CI blocks merge | Pass |
| V. LAN Security | No secrets or hostnames in repo (gitleaks in CI); non-root containers; health is public and returns no sensitive data; backups `0600` in a `0700` folder; GHCR via `GITHUB_TOKEN` only | Pass |
| VI. Documented | Doc comments on exported Go symbols, the handler and React components; enforced in review | Pass |
| VII. Simplicity | No router lib, no migration lib, no chi; deps listed and justified in research R18 | Pass |
| Deployment constraints | Two images, GHCR on `vX.Y.Z` with `X.Y.Z`/`X.Y`/`latest`; non-root default user, runs under any non-root UID/GID, no world-writable paths; pre-migration backup keeps 5, exits on failure; restore tested | Pass |

Post-design re-check (after Phase 1): still passes. The only new dependency beyond the
constitution's named stack is `gopkg.in/yaml.v3` in tests, justified in R13.

## Project Structure

### Documentation (this feature)

```text
specs/001-project-skeleton-deployment/
├── plan.md              # This file
├── research.md          # Phase 0 decisions
├── data-model.md        # Schema version, migrations, backups
├── quickstart.md        # Validation guide
├── contracts/
│   └── openapi.yaml     # Health endpoint contract (copied to api/openapi.yaml)
├── checklists/
│   └── requirements.md
└── tasks.md             # Created by /speckit-tasks
```

### Source Code (repository root)

```text
api/
├── go.mod                         # module github.com/helmnet/beer-menu/api
├── Dockerfile                     # golang:1.27-alpine -> scratch, USER 65532:65532, HEALTHCHECK
├── openapi.yaml                   # API contract (embedded)
├── cmd/beer-menu-api/
│   └── main.go                    # config from env, startup sequence, `healthcheck` subcommand
└── internal/
    ├── store/
    │   ├── open.go                # open SQLite with pragmas, file modes
    │   ├── migrate.go             # embedded migrations, user_version, per-migration tx
    │   ├── backup.go              # VACUUM INTO, naming, retention of 5
    │   ├── migrations/
    │   │   └── 0001_baseline.sql  # settings table
    │   └── *_test.go              # migrate, backup, retention, restore, downgrade tests
    └── httpapi/
        ├── routes.go              # single routes slice + mux
        ├── health.go              # GET /api/health
        └── *_test.go              # handler test, route-coverage test vs openapi.yaml

web/
├── package.json
├── vite.config.ts                 # also Vitest config
├── tsconfig.json
├── index.html
├── Dockerfile                     # node:26-alpine build -> nginx-unprivileged, USER 65532:65532
├── nginx/
│   └── default.conf               # static: /api proxy to api:8080, SPA fallback
└── src/
    ├── main.tsx
    ├── App.tsx                    # picks AdminPage or DisplayPage from pathname
    ├── pages/AdminPage.tsx
    ├── pages/DisplayPage.tsx
    └── App.test.tsx

docker-compose.yaml                # example: api + web, image + build, ./data bind mount
.github/workflows/
├── ci.yml                         # PR + main: api, web, openapi, images + smoke, gitleaks
└── release.yml                    # tag vX.Y.Z: validate tag, build, push to GHCR
README.md                          # run, release, restore steps, required checks
```

**Structure Decision**: two top-level folders, `api/` and `web/`, one per image, each with its
own Dockerfile and build context. Repo root holds the example compose file, workflows and
README. This mirrors the two-container architecture and keeps each image's build context small.

## Complexity Tracking

No constitution violations. Table intentionally empty.
