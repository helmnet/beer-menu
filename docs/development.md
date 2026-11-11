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
