---

description: "Task list for selective CI checks"
---

# Tasks: Run Only the CI Checks a Change Needs

**Input**: Design documents from `specs/016-cicd-job-refinement/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/changes-job.md, quickstart.md

**Tests**: Included. Constitution IV requires tests for new logic: the path classifier gets an
assert-based self-check (`.github/ci/changed-sides.test.sh`) that runs in CI. Job gating cannot be
unit-tested; it is proven by the throwaway pull requests in quickstart.md (T011).

**Organization**: Tasks are grouped by user story. The classifier and the `changes` job are
foundational: every story reads their output.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story the task belongs to (US1–US4)

## Path Conventions

CI only: `.github/ci/`, `.github/workflows/ci.yml`, `docs/development.md`,
`.specify/memory/constitution.md`. No new action or dependency (plan.md). Do not copy steps
between jobs: one rule set, one image-check loop (owner rule: no duplication).

---

## Phase 1: Setup

No setup. No new dependency or tool; `git` and `docker compose` are on `ubuntu-latest`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The amended CI rule, then the path rules and the `changes` job that every story's gating reads.

- [X] T001 Amend `.specify/memory/constitution.md` Development Workflow: replace "CI MUST build both images and run all tests on every pull request. A failing check blocks the merge." with "CI MUST run every check that a pull request's changed files can affect, and every check when the CI definition, its helper scripts, or files shared by both images (the compose file) change. A failing check blocks the merge." Bump **Version** to 1.5.0 and **Last Amended** to the commit date. Check for a sync-impact comment or template references that need the same update (Governance: dependent specs and plans). Do this first and state the reason in the pull request body (constitution Governance); the rest of this feature depends on the amended rule (analysis D1).
- [X] T002 [P] Create `.github/ci/changed-sides.test.sh` (POSIX `sh`, `set -eu`, header comment saying what it checks and that the `changes` job runs it). It runs `sh "$(dirname "$0")/changed-sides.sh"` with fixed stdin and compares the full stdout to the expected three lines; on a mismatch it prints the case name, expected and actual, and exits 1; at the end it prints `OK`. Cases (input → `api`/`web`/`services`): `web/src/App.tsx` → false/true/`web`; `api/cmd/main.go` → true/false/`api`; `api/openapi.yaml` → true/false/`api`; `.tool-versions` → false/true/`web`; `docker-compose.yaml` → true/true/`api web`; `.github/workflows/ci.yml` → true/true/`api web`; `.github/ci/changed-sides.sh` → true/true/`api web`; `api/x.go` + `web/y.ts` → true/true/`api web`; `web/a.ts` + `specs/016-x/contracts/a.yaml` → false/true/`web`; `api/README.md` alone → false/false/empty; `api/x.go` + `specs/x.go` (a move as `--no-renames` lists it) → true/false/`api`; `README.md` + `api/x.go` → true/false/`api`; `specs/001-x/contracts/openapi.yaml` → false/false/empty; `.github/workflows/release.yml` → false/false/empty; `.github/dependabot.yml` → false/false/empty; empty input → false/false/empty; the word `ALL` → true/true/`api web`. Run it now: it must fail (script missing).
- [X] T003 Create `.github/ci/changed-sides.sh` (POSIX `sh`, header comment: reads repo-relative paths on stdin, or `ALL` to force every check; prints `api=`, `web=`, `services=` lines for `$GITHUB_OUTPUT`; always exits 0; the rules below are the single source for which files trigger which checks, and a new file the builds depend on must be added here in the same change). Rules in one `case`, first match wins, per research.md R6: `*.md` → neither; `api/*` → api; `web/*|.tool-versions` → web; `.github/workflows/ci.yml|.github/ci/*|docker-compose.yaml` → both; `ALL` → both (a one-line comment notes that a root file literally named `ALL` also forces every check, which is harmless); `*` → neither. Output exactly `api=true|false`, `web=true|false`, `services=` followed by `api`, `web`, `api web` or nothing (in that order). Run `sh .github/ci/changed-sides.test.sh`: prints `OK`. Run `shellcheck` on both scripts if installed.
- [X] T004 In `.github/workflows/ci.yml`: (a) remove the workflow-level `paths-ignore: ['**/*.md']` from both `pull_request` and `push` (Markdown is now "neither" in the classifier, so `ci` always starts and no check is left pending; research.md R3); (b) add a first job `changes` on `ubuntu-latest` with `outputs: api, web, services` from a step `id: sides`; steps: `actions/checkout` at the same pinned SHA as the other jobs with `fetch-depth: 0` and `persist-credentials: false`; `run: sh .github/ci/changed-sides.test.sh`; then a step with `env: BASE: ${{ github.event.pull_request.base.sha || github.event.before }}` (never `${{ }}` inside the script text) that runs, in bash: if `BASE` is empty or all zeros, or `git cat-file -e "$BASE^{commit}"` fails, or `git diff --no-renames --name-only "$BASE" HEAD` fails (`--no-renames` so a file moved out of `api/` or `web/` is listed under its old path too; research.md R1), feed `ALL` to the classifier, else feed the diff; append the classifier output to `"$GITHUB_OUTPUT"` and also echo it to the log. Add one-line comments for why: the two-dot diff against the PR merge commit is exactly the PR's changes, and the fallback runs every check when the diff cannot be trusted (research.md R1, R2). (c) Update the file's header comment: `ci` runs on every pull request and push to `main`; the `changes` job decides which checks run (rules in `.github/ci/changed-sides.sh`); skipped checks report Skipped; never merge while any of `changes`, `api`, `web`, `openapi`, `images` or `secrets` (the check from `secrets.yml`) fails or is pending.

**Checkpoint**: `changes` job exists and prints correct flags; no other job reads them yet (all still run).

---

## Phase 3: User Story 1 - Web-only change skips API checks (Priority: P1) 🎯 MVP

**Goal**: A web-only change runs `web`, `images` (web image checked, smoke test) and skips `api`, `openapi` and the API image checks.

**Independent Test**: quickstart.md row "one file under `web/`".

- [X] T005 [US1] In `.github/workflows/ci.yml`, add `needs: changes` and `if: needs.changes.outputs.api == 'true'` to the `api` job and the `openapi` job.
- [X] T006 [US1] In `.github/workflows/ci.yml`, `images` job: add `needs: changes` and `if: needs.changes.outputs.api == 'true' || needs.changes.outputs.web == 'true'`; add job-level `env: SERVICES: ${{ needs.changes.outputs.services }}`. Keep `docker compose build` building both images (the smoke test needs both; FR-012) and update its comment to say so. In the three check steps ("default user is non-root", "refuse to run as root", "Scan both images"), change `$(docker compose config --images)` to `$(docker compose config --images $SERVICES)` (unquoted on purpose: it may hold two service names; add `# shellcheck disable=SC2086` with that reason once per step, as the existing `$opts` line does). Rename "both images" in those step names and comments to "the changed images". Leave the smoke test step and `if: failure()` logs step unchanged. Do not split the job or copy steps (research.md R4).

**Checkpoint**: US1 quickstart row passes.

---

## Phase 4: User Story 2 - API-only change skips web checks (Priority: P1)

**Goal**: An API-only change runs `api`, `openapi`, `images` (API image checked, smoke test) and skips `web` and the web image checks.

**Independent Test**: quickstart.md row "one file under `api/`".

- [X] T007 [US2] In `.github/workflows/ci.yml`, add `needs: changes` and `if: needs.changes.outputs.web == 'true'` to the `web` job. (The per-image loop for this story is T006.)

**Checkpoint**: US2 quickstart row passes.

---

## Phase 5: User Story 3 - Shared or CI changes run both sides (Priority: P2)

**Goal**: Changes to `ci.yml`, `.github/ci/*` or `docker-compose.yaml`, or to both sides, run every check.

**Independent Test**: quickstart.md rows "`docker-compose.yaml`" and "`.github/workflows/ci.yml`".

- [X] T008 [US3] Confirm `.github/ci/changed-sides.test.sh` (T002) covers every shared path and the both-sides case, and that the fallback in the `changes` job (T004) forces `ALL` for an all-zeros `BASE`: run locally `BASE=0000000000000000000000000000000000000000` through the same bash snippet (copy it to the scratchpad, do not add a second copy to the repo) and check it prints `api=true`, `web=true`, `services=api web`. Fix T003/T004 if not.

---

## Phase 6: User Story 4 - Changes that affect neither side run no build checks (Priority: P3)

**Goal**: Changes to only `specs/`, docs, tooling, `release.yml`, dependabot config or Markdown run only `changes` (and `secrets`); every other check is Skipped, none pending.

**Independent Test**: quickstart.md rows "only `specs/**/*.yaml`" and "only a `.md` file".

- [X] T009 [US4] Confirm `.github/workflows/secrets.yml` is untouched and still has no path filter (FR-009), and that no `paths`/`paths-ignore` remains in `.github/workflows/ci.yml` (FR-005, FR-008).

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T010 [P] Rewrite "Required checks" in `docs/development.md` (FR-010): `ci` runs on every pull request and push to `main`; the `changes` job lists changed files with `git diff` and sets which checks run, with a table of the path rules from research.md R6 and which checks each side runs (contracts/changes-job.md); `images` builds both images for the smoke test but checks and scans only the changed sides' images; if the diff cannot be trusted every check runs; skipped checks report Skipped and count as passing; the six required checks are `changes`, `api`, `web`, `openapi`, `images`, `secrets`, and `changes` must be required because its failure skips the others; a change that adds a file the builds depend on outside `api/` and `web/` must add it to `.github/ci/changed-sides.sh`. Remove the now-wrong text: "except changes that touch only Markdown files", "on a Markdown-only pull request only `secrets` runs", the `git diff ... | grep -v '\.md$'` manual check and the 300-file note, and the closing note about skipped workflows leaving checks pending. Keep the branch-protection paragraph, updated to six checks.
- [X] T011 Push the branch and run quickstart.md on GitHub: one throwaway pull request per row (base it on this branch so the new `ci.yml` runs), record ran/skipped jobs and total time for the web-only row versus a recent full run on `main` (SC-005), confirm no check is pending (SC-004), the moved-file row runs the API checks (U1), the failing-web-test row fails `web` (SC-006), and the `images` log names only the changed image in the check steps. Close the throwaway pull requests and delete their branches. Record results in `specs/016-cicd-job-refinement/quickstart.md`.

---

## Dependencies & Execution Order

- T001 (constitution amendment) first.
- T002 ∥ T003 start together; T003 is done when T002 passes. T004 needs T003.
- US1 (T005, T006) needs T004. US2 (T007) needs T004 and uses T006's loop.
- US3 (T008) and US4 (T009) need T004; they are checks, not new code.
- T010 can run any time after T004. T011 runs last.
- All of T004–T007 edit `ci.yml`: do them in order, not in parallel.

## Parallel Example

```text
T002 (test script)  ∥  T003 (classifier)      # different files
T010 (docs)         ∥  T005–T009 (ci.yml)     # different files
```

## Implementation Strategy

MVP is Phase 2 (amendment first) + US1: web-only changes stop running API work, the most common case. US2 is one
`if:` line on top. US3 and US4 come from the classifier for free and only need checking. Ship all
in one pull request, since the docs and constitution must change with the workflow.
