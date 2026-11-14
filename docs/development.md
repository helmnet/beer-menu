# Development

How to work on beer-menu. To run it, see the [README](../README.md); for the API, see
[api.md](api.md).

## Layout

- `api/`: Go API (`cmd/beer-menu-api`), SQLite store and migrations (`internal/store`), HTTP
  handlers (`internal/httpapi`) and the API contract `openapi.yaml`.
- `web/`: React + TypeScript app built with Vite, and the nginx config of the web image.
- `docker-compose.yaml`: example compose file that starts both containers.
- `scripts/`: repo maintenance scripts (third-party notices, below).
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

## Third-party notices

`api/THIRD_PARTY_NOTICES` and `web/THIRD_PARTY_NOTICES` hold the license texts of everything
each image ships: Go modules, the Go standard library and the vendored Swagger UI in the API,
and the runtime npm packages bundled into the web app. Both images copy them, with `LICENSE`,
to `/usr/share/doc/beer-menu/`. `scripts/third-party-notices.sh` writes them and also copies
the root `LICENSE` into `api/` and `web/`, the Docker build contexts. It needs Go and
`npm ci` in `web/`. It fails if a package has no license file or license id; a new Go module
needs its id added to `go_license`. Some distro Go packages drop `LICENSE` from `GOROOT`; the
script then switches to the upstream Go of the same version by itself (`GOTOOLCHAIN`), which
Go downloads once.

Turn on the repo's git hooks once per clone (worktrees share the setting):

```sh
git config core.hooksPath .githooks
```

Then `.githooks/pre-commit` runs the script whenever a staged file can change the notices and
stages the result. The files that count are the `notices` rules in
`.github/ci/changed-sides.sh` (Go modules, `web/package.json`, `web/package-lock.json`, Swagger
UI, `LICENSE`, the script and its outputs). If the script fails, the commit is blocked;
`git commit --no-verify` skips the hook, and the `notices` CI check then catches stale notices.
The hook reads the working tree, so stage all of a dependency change before committing. Without
the hook, run `sh scripts/third-party-notices.sh` from the repo root and commit the result.

### Dependabot and third-party notices

Dependabot cannot run the script, so on a Dependabot pull request the `notices` check fails and
the `notices-push` job in `ci.yml` commits the regenerated notices to the pull request branch.
It pushes with a GitHub App token: a push with the workflow's own token would not start CI on the
new commit, and the required checks would stay pending. Runs started by Dependabot can read only
Dependabot secrets. Without this setup, `notices-push` fails and the notices must be updated by
hand (check out the branch, run the script, push). One-time setup by a repo admin:

1. Create a GitHub App owned by the `helmnet` org (Settings, Developer settings, GitHub Apps):
   no webhook, repository permission Contents: Read and write, nothing else, installable only on
   this account.
2. Note its Client ID, generate a private key, and install the app on this repo only.
3. In the repo, Settings, Secrets and variables, Dependabot, add `NOTICES_APP_CLIENT_ID` (the
   Client ID) and `NOTICES_APP_PRIVATE_KEY` (the whole `.pem` file).

`notices-push` runs only when the actor and pull request author are `dependabot[bot]` and the
branch is a `dependabot/` branch in this repo. It
applies only the patch from the token-free `notices` job, refuses (`.github/ci/notices-patch.sh`)
a patch that touches any other file, deletes a file, or makes one a symlink or executable, and
runs no dependency code. Its commit comes from the app, so CI runs again and
`notices-push` does not. After a Dependabot rebase or recreate drops that commit, the next run
adds it again. A push fails, harmlessly, if the branch moved meanwhile; the newer run handles it.

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

A separate `notices` flag is set by `api/go.mod`, `api/go.sum`,
`api/internal/httpapi/docs/swagger-ui/*`, `web/package.json`, `web/package-lock.json`,
`LICENSE`, `scripts/third-party-notices.sh`, the four generated files, `.github/workflows/ci.yml`
and `.github/ci/*` (Markdown never sets it). The pre-commit hook uses the same flag.

A pull request runs the checks for every side its files touch:

| Check | What it does | Runs when |
|-------|--------------|-----------|
| `changes` | runs the classifier self-check, then sets the sides | always |
| `api` | `go vet` and `go test` in `api/` | api side changed |
| `openapi` | Redocly lint of `api/openapi.yaml` | api side changed |
| `web` | `npm ci`, lint, tests and build in `web/` | web side changed |
| `notices` | runs `scripts/third-party-notices.sh` on the pull request head (not the merge commit) and fails if its output differs from the committed files | `notices` flag set |
| `images` | builds both images; checks the changed sides' images run as a non-root user and refuse to run as root, and scans them with Trivy for HIGH or CRITICAL vulnerabilities that have a fix; runs a smoke test of the stack as UID/GID `12345:12345` | either side changed |

`images` always builds both images, because the smoke test starts the whole stack, but checks and
scans only the changed sides' images. If the diff cannot be trusted (no base commit, for example
on the first push or after a force push, or `git diff` fails), every check runs.

The `secrets` workflow runs on its own on every pull request and every push to `main`:

- `secrets`: gitleaks scan of the full git history.

A check skipped by `changes` reports Skipped and counts as passing. Never merge a pull request while
any of the seven checks (`changes`, `api`, `web`, `openapi`, `notices`, `images`, `secrets`) is
failing or pending; `notices-push` is not required. `changes` must be one of them: if it fails,
every check that depends on it is skipped and would look green. A pull request that changes
`.github/workflows/ci.yml` or `.github/ci/` must show `api`, `web`, `openapi`, `notices` and
`images` as run, not Skipped: it runs its own copy of the rules, so Skipped there means the rules were changed to skip checks. A change that adds a file the builds
depend on outside `api/` and `web/` must add it to `.github/ci/changed-sides.sh` in the same change.

The repo is private on a free GitHub org plan, which has no branch protection, so this rule is
followed by hand. When the repo goes public, turn on branch protection for `main` requiring all seven
checks and up-to-date branches.
