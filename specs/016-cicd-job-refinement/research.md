# Research: Run Only the CI Checks a Change Needs

## R1. How to list the changed files

**Decision**: a new first job, `changes`, checks out the full history and runs
`git diff --no-renames --name-only -z <base> <head> | tr '\0' '\n'` under `set -o pipefail`,
where `<base>` is `github.event.pull_request.base.sha` on a pull request and `github.event.before` on a push to `main`, and `<head>` is the checked-out
commit (`HEAD`). Both SHAs reach the script through `env`, never by `${{ }}` interpolation into
the script text.

**Rationale**: `git` is already on the runner, so no new action or dependency (constitution VII).
On a pull request the checkout is GitHub's merge commit of the branch into the base tip, and
`base.sha` is that base tip, so the two-dot diff is exactly the pull request's changes. `git diff`
has no file-count limit, so the 300-file limit of GitHub's `paths` filters does not apply.
`--no-renames` matters: with rename detection on, a file moved out of `api/` would be listed only
under its new path and the API checks would be skipped. `-z` matters too: without it git quotes
non-ASCII, tab and newline names (`"api/caf\303\251.go"`), which match no rule and skip checks;
after `tr` a newline in a name only splits it, and its first part keeps its real directory.
`pipefail` makes a failed `git diff` fail the pipeline, so it reaches the R2 fallback instead of
being hidden by `tr`'s exit 0. If `base.sha` is behind the base tip when
the event fires, the diff also lists files already on `main`; that only runs extra checks, never
fewer.

**Alternatives considered**:
- `dorny/paths-filter`: a third-party action to pin and audit for what is one `git diff` plus a
  `case`; rejected (VII, and fewer third-party actions near the token).
- Workflow-level `paths:` filters in separate workflows per side: a skipped workflow leaves its
  required checks pending (the problem `docs/development.md` already notes), cannot express
  "shared files run both", and has the 300-file limit. Rejected.
- `tj-actions/changed-files`: compromised in March 2025 (supply-chain attack). Rejected.

## R2. Fall back to running everything

**Decision**: the classifier sets every side to `true` when it cannot trust the diff: the base
SHA is empty or all zeros (first push of a branch, which only happens on `main` here), the base
commit is not in the clone (`git cat-file -e` fails, for example after a force push to `main`),
or `git diff` exits non-zero.

**Rationale**: FR-007. Failing open to "run everything" costs time, never coverage.

**Alternatives considered**: failing the `changes` job instead. Rejected: a failed `changes` job
skips every dependent job, which is the outcome we must avoid.

## R3. How skipped checks report

**Decision**: each check job keeps its name and gains `needs: changes` and a job-level
`if: needs.changes.outputs.<side> == 'true'`. The workflow-level `paths-ignore: ['**/*.md']` is
removed, so `ci` always runs and Markdown-only changes are handled by the classifier (Markdown
counts as "neither").

**Rationale**: a job skipped by its `if` reports as "Skipped", which GitHub branch protection
counts as passing; a workflow that never started leaves its checks pending (FR-005). The cost is
one short `changes` job on Markdown-only pull requests.

**Risk and mitigation**: if `changes` itself fails, its dependents are skipped and would look
green. So `changes` joins the required checks, and the classifier never fails on a bad diff
(R2): only a broken checkout can fail it.

## R4. Image checks per side, one smoke test

**Decision**: keep the single `images` job (its check name stays). It runs when either side
changed. It still runs `docker compose build` (both images, so the stack can start), but the user
checks, root-refusal checks and Trivy scan loop over only the changed sides' images:
`docker compose config --images $SERVICES`, where `SERVICES` is the `changes` output (for example
`web`, or `api web`). That step fails the job if the list is empty, since an empty `IMAGES`
would let the three loops pass without checking anything. The smoke test then runs as today.

**Rationale**: FR-004 and FR-012 with a few-word diff to existing steps and no copied steps
(owner rule: no duplication). Verified locally: `docker compose config --images web` prints only
the web image.

**Alternatives considered**:
- Separate `image-api`, `image-web` and `smoke` jobs: copies the check steps or needs a reusable
  workflow, and builds each image twice. Rejected.
- A dynamic matrix from the `changes` output: an empty matrix fails the job, and the smoke test
  still needs both images in one job. Rejected.

## R5. Where the path rules live and how they are tested

**Decision**: one script, `.github/ci/changed-sides.sh`, reads file names on stdin and prints
`api=true|false`, `web=true|false` and `services=<list>` for `$GITHUB_OUTPUT`. The rules are one
`case` statement (FR-011). A small self-check, `.github/ci/changed-sides.test.sh`, feeds fixed
file lists and asserts the output; the `changes` job runs it before classifying, which takes
under a second.

**Rationale**: constitution IV (tests for new logic) without a test framework; keeps the rules in
one readable place outside the YAML. The script and its test sit under `.github/ci/`, which is a
shared path, so changing the rules runs every check.

## R6. Path rules

| Pattern (first match wins) | Group |
|----------------------------|-------|
| `*.md` | neither (Markdown never adds checks) |
| `api/*` | api |
| `web/*`, `.tool-versions` | web |
| `.github/workflows/ci.yml`, `.github/ci/*`, `docker-compose.yaml` | both |
| anything else | neither |

`.tool-versions` holds only `nodejs`. `api/openapi.yaml` is under `api/`, so the OpenAPI lint
follows the api side.

## R7. Constitution conflict

The Development Workflow section says "CI MUST build both images and run all tests on every pull
request." This feature changes that rule, so the same pull request amends it (see plan,
Complexity Tracking): CI MUST run every check a pull request's changed files can affect, and
every check when shared files or the CI definition change. Version 1.4.0 to 1.5.0 (MINOR:
guidance materially changed, no principle removed).
