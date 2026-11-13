# Development

How to work on beer-menu. To run it, see the [README](../README.md); for the API, see
[api.md](api.md).

## Layout

- `api/`: Go API (`cmd/beer-menu-api`), SQLite store and migrations (`internal/store`), HTTP
  handlers (`internal/httpapi`) and the API contract `openapi.yaml`.
- `web/`: React + TypeScript app built with Vite, and the nginx config of the web image.
- `docker-compose.yaml`: example compose file that starts both containers.
- `specs/`: feature specifications and plans.
- `docs/`: this guide, the [API overview](api.md) and the [project brief](project-brief.md).

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

## API contract

`api/openapi.yaml` is the API contract. Every change to an endpoint updates it in the same
change. CI lints it with Redocly, and the route-coverage test
(`go test ./internal/httpapi/... -run Routes` in `api/`) fails if a route in
`internal/httpapi/routes.go` is not documented in it. Each operation's `x-access` says who may
call it (`public`, `read`, `write` or `admin`), and the same test checks it against the route's
declared access.

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
Once both images are published, the workflow creates a GitHub Release for the tag with notes
generated from the changes since the previous tag. If only that job fails, re-run it, or
create the release by hand: `gh release create vX.Y.Z --verify-tag --generate-notes`.

Production pins an exact `X.Y.Z` in the `truenas_apps` repo and never uses `latest`.

## Required checks

The `ci` workflow runs on every pull request and every push to `main`. Its first job, `changes`,
lists the changed files with `git diff --no-renames --name-only -z` against the base commit and sets
which checks run. The rules live in `.github/ci/changed-sides.sh`, one `case` where the first match
wins:

| Changed path | Side |
|--------------|------|
| `*.md` | neither (Markdown never adds checks) |
| `api/*` | api |
| `web/*`, `.tool-versions` | web |
| `.github/workflows/ci.yml`, `.github/ci/*`, `docker-compose.yaml` | both |
| anything else (`specs/`, other workflows, dependabot config) | neither |

A pull request runs the checks for every side its files touch:

| Check | What it does | Runs when |
|-------|--------------|-----------|
| `changes` | runs the classifier self-check, then sets the sides | always |
| `api` | `go vet` and `go test` in `api/` | api side changed |
| `openapi` | Redocly lint of `api/openapi.yaml` | api side changed |
| `web` | `npm ci`, lint, tests and build in `web/` | web side changed |
| `images` | builds both images; checks the changed sides' images run as a non-root user and refuse to run as root, and scans them with Trivy for HIGH or CRITICAL vulnerabilities that have a fix; runs a smoke test of the stack as UID/GID `12345:12345` | either side changed |

`images` always builds both images, because the smoke test starts the whole stack, but checks and
scans only the changed sides' images. If the diff cannot be trusted (no base commit, for example
on the first push or after a force push, or `git diff` fails), every check runs.

The `secrets` workflow runs on its own on every pull request and every push to `main`:

- `secrets`: gitleaks scan of the full git history.

A check skipped by `changes` reports Skipped and counts as passing. Never merge a pull request while
any of the six checks (`changes`, `api`, `web`, `openapi`, `images`, `secrets`) is failing or
pending. `changes` must be one of them: if it fails, every check that depends on it is skipped and
would look green. A pull request that changes `.github/workflows/ci.yml` or `.github/ci/` must
show `api`, `web`, `openapi` and `images` as run, not Skipped: it runs its own copy of the rules,
so Skipped there means the rules were changed to skip checks. A change that adds a file the builds
depend on outside `api/` and `web/` must add it to `.github/ci/changed-sides.sh` in the same change.

The repo is private on a free GitHub org plan, which has no branch protection, so this rule is
followed by hand. When the repo goes public, turn on branch protection for `main` requiring all six
checks and up-to-date branches.
