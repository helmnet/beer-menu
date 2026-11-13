# Implementation Plan: Run Only the CI Checks a Change Needs

**Branch**: `compmodder26/cicd_job_refinement` | **Date**: 2026-11-13 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/016-cicd-job-refinement/spec.md`

## Summary

CI runs every check on every non-Markdown change. A new first job, `changes`, diffs the base and
head commits and classifies the paths with one shell script into api / web / both / neither
flags, falling back to "everything" when the diff cannot be trusted. Each existing job gains
`needs: changes` and an `if:` on its side's flag, so skipped checks report Skipped, not pending.
The `images` job keeps building both images for the smoke test, but its user, root-refusal and
Trivy steps loop over only the changed sides' images. The workflow-level Markdown filter goes
away; Markdown becomes "neither". No new action or dependency. Docs and the constitution's CI
rule are updated in the same change.

## Technical Context

**Language/Version**: GitHub Actions YAML; POSIX `sh` for the classifier.

**Primary Dependencies**: existing only: `git` and `docker compose` on `ubuntu-latest`, the
already-pinned `actions/checkout`. No new action.

**Storage**: N/A.

**Testing**: `.github/ci/changed-sides.test.sh` (assert-based self-check, run in the `changes`
job and locally); end-to-end by throwaway pull requests ([quickstart.md](quickstart.md)).

**Target Platform**: GitHub-hosted `ubuntu-latest` runners.

**Project Type**: CI configuration for the existing Go API + React web repo.

**Performance Goals**: one-sided pull requests at least a third faster (SC-005); `changes` job
adds about 10 s.

**Constraints**: skipped checks must not be pending (FR-005); pinned actions,
`persist-credentials: false`, event values passed by `env` only; scanner never gets the Docker
socket (unchanged).

**Scale/Scope**: `ci.yml`, 2 new scripts under `.github/ci/`, `docs/development.md`,
`.specify/memory/constitution.md`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status |
|-----------|--------|
| I. API-First | N/A, no API change. |
| II. Go + SQLite | N/A. |
| III. React Clients | N/A. |
| IV. Tests Required | Pass: the classifier has an assert-based self-check run in CI; every check still runs, unchanged, whenever its side changes. |
| V. LAN Security | Pass: no secrets, no new third-party action, event SHAs via `env`, `persist-credentials: false`, Trivy and root checks unchanged for the changed side. |
| VI. Documented | Pass: scripts get a header comment; `docs/development.md` "Required checks" rewritten (FR-010). |
| VII. Simplicity | Pass: `git diff` + one `case`; no new dependency. |
| Development Workflow: "CI MUST build both images and run all tests on every pull request" | **Conflict**: amended in this change (Complexity Tracking). |

Post-design re-check: same result; the design adds no further deviation.

## Project Structure

### Documentation (this feature)

```text
specs/016-cicd-job-refinement/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/changes-job.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
.github/
├── ci/
│   ├── changed-sides.sh        # new: path rules, prints api/web/services
│   └── changed-sides.test.sh   # new: self-check
└── workflows/
    └── ci.yml                  # changes job; needs/if on api, web, openapi, images; per-side image loops; drop paths-ignore
docs/development.md             # "Required checks" rewritten
.specify/memory/constitution.md # Development Workflow CI rule amended, 1.4.0 -> 1.5.0
```

**Structure Decision**: CI-only change. `release.yml` and `secrets.yml` are untouched.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Constitution Development Workflow: "CI MUST build both images and run all tests on every pull request" | It is what this feature changes: running the other side's checks on a one-sided change is the cost the owner asked to remove. Amend to: "CI MUST run every check that a pull request's changed files can affect, and every check when the CI definition, its helper scripts, or files shared by both images (the compose file) change. A failing check blocks the merge." Bump to 1.5.0. | Keeping the rule means no feature. Amending only after merge would leave `main` out of line with its own constitution. |
