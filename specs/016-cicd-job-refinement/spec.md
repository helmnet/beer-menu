# Feature Specification: Run Only the CI Checks a Change Needs

**Feature Branch**: `compmodder26/cicd_job_refinement`

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "We shouldn't always need to run every CICD job/step. For instance, if only code in "/web" has changed, we shouldn't have to run any steps against the api, including image checking. Likewise for the api. I think that we need to make distinctions on when cicd jobs/steps run between the 2 (api and web). If code changes or cicd workflows change that pertain to to them, then, and only then, should workflow jobs/steps run."

## Context

Today every pull request and every push to `main` that changes any non-Markdown file runs all of
the CI checks: the API checks (vet and tests), the web checks (install, lint, tests, build), the
OpenAPI lint, and the image checks (build both images, non-root user checks, refuse-to-run-as-root
checks, vulnerability scan of both images, and a smoke test of the whole stack). A change that
touches only the web app still builds, checks and scans the API image, and the reverse. A change
that touches only planning files (for example an OpenAPI contract under `specs/`) still runs
everything.

The secret scan runs on its own on every change and is out of scope: it keeps running on every
change. The release workflow (publishing images on a version tag) is also out of scope.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Web-only change skips API checks (Priority: P1)

The owner opens a pull request that changes only files in the web app. CI runs the web checks
and the web image checks, and runs none of the API checks, the OpenAPI lint, or the API image
checks (build, user checks, root refusal, vulnerability scan).

**Why this priority**: web-only changes are the most common kind of change (the recent features
11 to 15 were mostly web work), and the API image build and scan are the slowest steps.

**Independent Test**: open a pull request that changes one file under `web/` and confirm in the
run summary that only the web checks and web image checks ran, and that every API check shows as
skipped, not pending.

**Acceptance Scenarios**:

1. **Given** a pull request that changes only files under `web/`, **When** CI runs, **Then** the
   web checks, web image checks and the whole-stack smoke test run and pass or fail on their own
   merits.
2. **Given** the same pull request, **When** CI runs, **Then** no API check, OpenAPI lint, or API
   image check runs (the API image is built only to start the smoke test), and each of them reports a finished, non-failing state (not pending).

---

### User Story 2 - API-only change skips web checks (Priority: P1)

The owner opens a pull request that changes only files in the API. CI runs the API checks, the
OpenAPI lint, and the API image checks, and runs none of the web checks or web image checks.

**Why this priority**: the mirror of story 1, and equally needed; the user asked for both.

**Independent Test**: open a pull request that changes one file under `api/` and confirm that only
the API checks, OpenAPI lint and API image checks ran, and every web check shows as skipped.

**Acceptance Scenarios**:

1. **Given** a pull request that changes only files under `api/`, **When** CI runs, **Then** the
   API checks, the OpenAPI lint, the API image checks and the whole-stack smoke test run.
2. **Given** the same pull request, **When** CI runs, **Then** no web check or web image
   check runs (the web image is built only to start the smoke test), and each reports a
   finished, non-failing state.

---

### User Story 3 - Shared or CI changes run both sides (Priority: P2)

The owner changes something both sides depend on: the CI workflow definition itself, or the
compose file that defines how both images build and run together. CI runs every check for both
sides, as it does today.

**Why this priority**: a change to the checks themselves must prove the checks still work for
both sides; skipping here would let a broken pipeline merge unnoticed.

**Independent Test**: open a pull request that changes only the CI workflow file and confirm that
every API and web check ran.

**Acceptance Scenarios**:

1. **Given** a pull request that changes the CI workflow definition, **When** CI runs, **Then**
   all API and web checks run.
2. **Given** a pull request that changes the compose file, **When** CI runs, **Then** all API and
   web checks run.
3. **Given** a pull request that changes files under both `api/` and `web/`, **When** CI runs,
   **Then** all API and web checks run.

---

### User Story 4 - Changes that affect neither side run no build checks (Priority: P3)

The owner changes only files that affect neither the API nor the web app: planning files under
`specs/`, docs, Spec Kit or Claude tooling files, the dependency-update config, the release
workflow, the secret-scan config, or `.gitignore`. CI runs no API or web checks, as it already
does for Markdown-only changes. The secret scan still runs.

**Why this priority**: useful savings, but these changes are less frequent than code changes.

**Independent Test**: open a pull request that changes only a contract file under `specs/` and
confirm that no API or web check ran and every one of them reports a finished, non-failing state.

**Acceptance Scenarios**:

1. **Given** a pull request that changes only files that affect neither side, **When** CI runs,
   **Then** no API or web check runs and none is left pending.
2. **Given** the same pull request, **When** the secret scan runs, **Then** it runs as it does
   today.

---

### Edge Cases

- A change mixes a web file with a file that affects neither side: only the web checks run.
- A change mixes Markdown with API files: the API checks run (Markdown never adds or removes
  checks).
- A pull request changes a very large number of files: the way CI lists changed files MUST have
  no file-count limit; if it cannot list them all, the fallback below applies.
- The list of changed files cannot be worked out (for example the first push of a new branch, or
  a forced push that rewrote history): CI MUST run every check.
- A push to `main` merges a pull request: the same rules apply, judged on the files that push
  changed.
- A new file is added outside `api/` and `web/` that the builds start to depend on (for example a
  shared config file): it affects neither side until the path rules are updated, so the same
  change MUST add it to the rules (see FR-010).
- The smoke test needs both images running together, so a one-sided change still needs the other
  image to exist: the other image is built only to start the stack (see FR-012).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: CI MUST sort each change's files into these groups: API files (everything under
  `api/`), web files (everything under `web/`, plus the tool-versions file, which pins only the
  web toolchain), shared files (the CI workflow definition, the CI helper scripts it runs, and the
  compose file), and files that
  affect neither side (everything else that is not Markdown).
- **FR-002**: CI MUST run the API checks (vet, tests), the OpenAPI lint, and the API image checks
  (build, non-root user check, refuse-to-run-as-root check, vulnerability scan) only when the
  change includes an API file or a shared file.
- **FR-003**: CI MUST run the web checks (install, lint, tests, build) and the web image checks
  (build, non-root user check, refuse-to-run-as-root check, vulnerability scan) only when the
  change includes a web file or a shared file.
- **FR-004**: The image checks MUST run per image, so the user checks, root-refusal checks and
  vulnerability scan of one image never run because only the other side changed.
- **FR-005**: Every check that is skipped MUST end in a finished, non-failing state, never
  pending, so that when branch protection is turned on it can require every check without
  blocking one-sided or no-code pull requests.
- **FR-006**: Every check that runs MUST apply the same pass and fail rules as today to every job
  and image it covers, with the same security limits (pinned actions and scanner image, no stored
  credentials, scanner kept away from the Docker socket). Only which images a check covers may
  change (FR-004).
- **FR-007**: When CI cannot reliably list every changed file, it MUST run every check.
- **FR-008**: Changes that touch only Markdown files MUST keep today's behavior: no API or web
  checks run, and the secret scan runs.
- **FR-009**: The secret scan MUST keep running on every pull request and every push to `main`,
  whatever files changed.
- **FR-010**: The project's required-checks documentation MUST describe which files trigger which
  checks, the fall-back-to-everything rule, and that a change adding a new file the builds depend
  on must update the path rules in the same change.
- **FR-011**: The rules for which files trigger which checks MUST be defined in one place, not
  repeated per check.
- **FR-012**: The whole-stack smoke test (health, display page, security headers) MUST run
  whenever any API, web or shared file changes. When only one side changed, the other side's
  image MUST be built only so the stack can start: its user checks, root-refusal checks and
  vulnerability scan MUST NOT run.

### Key Entities

- **Change**: the set of files a pull request or push to `main` adds, modifies, deletes or
  renames.
- **File group**: API, web, shared, or neither; decides which checks a file triggers.
- **Check**: a named CI job or step group whose result is reported on the pull request (API,
  web, OpenAPI lint, API image, web image, smoke test, secret scan).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a web-only pull request, zero API checks, OpenAPI lint, or API image user,
  root-refusal or vulnerability checks run.
- **SC-002**: On an API-only pull request, zero web checks, or web image user, root-refusal or
  vulnerability checks run.
- **SC-003**: On a pull request that changes the CI workflow definition or the compose file,
  100% of the API and web checks run.
- **SC-004**: On every pull request, zero checks are left pending once CI finishes.
- **SC-005**: The total CI time of a one-sided pull request drops by at least a third compared
  with the same change on today's pipeline.
- **SC-006**: No check that runs is weaker than today: a deliberately broken API or web change
  still fails the same check it fails today.

## Assumptions

- The project has one API (`api/`) and one web app (`web/`) with no code shared between them; the
  web build does not read the API's OpenAPI document, and neither image copies files from the
  other side.
- The OpenAPI lint belongs to the API side, because the document lives in `api/`.
- The tool-versions file pins only the Node version used by the web side, so it is a web file.
- A change to the CI workflow definition runs every check, even when the edit touches only one
  side's job, because the shared parts (triggers, path rules) can break both sides.
- The release workflow, the secret-scan workflow, the dependency-update config and code owners
  file do not change what the CI checks test, so changing them alone runs no API or web checks.
- Branch protection is still off (private repo on a free plan); FR-005 prepares for turning it on
  when the repo goes public.
- Vulnerability scans run only when an image's side changes. New vulnerabilities in an unchanged
  image are found on its next change or at release time, as today for Markdown-only changes; a
  scheduled scan is out of scope.
