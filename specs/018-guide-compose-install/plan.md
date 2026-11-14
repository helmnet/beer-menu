# Implementation Plan: Install From a Compose File, Without Cloning

**Branch**: `compmodder26/user_guide_modifications` | **Date**: 2026-11-14 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/018-guide-compose-install/spec.md`

## Summary

Rewrite the guide's Install with Docker page so users copy a compose file from the page instead
of cloning the project, add an environment variables section after the basic steps, and add a
GitHub repository link to the Home page. The compose file is derived at build time from the
root `docker-compose.yaml` (comments and `build:` lines stripped), so there is no second copy.
A small external `copy.js` adds the one-click Copy button. Tests guard the derived file, the
script, and that every user-settable variable the API and compose file read is documented.

## Technical Context

**Language/Version**: Node 26 (guide build, as `.tool-versions`); HTML/CSS; one plain browser script

**Primary Dependencies**: none new; Node standard library only (`node:test`, `node:vm`)

**Storage**: N/A

**Testing**: `node --test guide/` (`guide/build.test.mjs`); `sh .github/ci/changed-sides.test.sh`

**Target Platform**: GitHub Pages static site; current desktop and phone browsers; local `file://` preview

**Project Type**: static documentation site inside the existing repo

**Performance Goals**: N/A (static pages)

**Constraints**: no inline script or `on*=` handlers; no hard-coded colors in `guide.css`; no sideways page scroll at 320 px; works without JavaScript

**Scale/Scope**: 2 page fragments, 1 new script, build + test changes, 1 CI rule file, 1 workflow paths list

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Note |
|-----------|--------|------|
| I. API-First | Pass | No API change. |
| II. Go + SQLite | Pass | Not touched; only read by a test for variable names. |
| III. React Clients | Pass | Admin app and display not touched. |
| IV. Tests Required | Pass | Build test covers derived compose, Install/Home content, variable coverage; `copy.js` tested in `node:vm`; CI rule changes get `changed-sides.test.sh` cases. |
| V. LAN Security | Pass | Guide tells users to keep secrets out of the compose file (`.env`, key file with 0600, `docker inspect` caveat). No secrets committed. No inline script keeps the site CSP-friendly. Copy writes only page text to the clipboard. |
| VI. Documented | Pass | `copy.js` and new build function get a doc comment; guide README updated. |
| VII. Simplicity | Pass | No new dependency; derivation reuses the root file instead of a copy; no download file, no per-page script. |
| Deployment: non-root | Pass | Steps keep PUID/PGID as the user's own IDs, now persisted in `.env`. |
| Workflow: CI runs affected checks | Pass | `changed-sides.sh`: `docker-compose.yaml` and `api/cmd/beer-menu-api/*` now also set `guide=true`. `pages.yml` paths gain `docker-compose.yaml`. |

Post-design re-check: unchanged, all pass.

## Project Structure

### Documentation (this feature)

```text
specs/018-guide-compose-install/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/guide-site.md   # delta on specs/017-user-guide/contracts/guide-site.md
└── tasks.md                  # /speckit-tasks
```

### Source Code (repository root)

```text
guide/
├── build.mjs            # + derive compose from ../docker-compose.yaml, fill {{compose}}, copy copy.js
├── build.test.mjs       # + compose, install, home, env-coverage, copy.js (vm) tests; script rule relaxed
├── README.md            # + copy.js, {{compose}}, variable coverage test
└── src/
    ├── copy.js          # new: Copy button for pre.copy
    ├── guide.css        # + .copy-button, h3 spacing (tokens only)
    └── pages/
        ├── install.html # rewritten steps + Environment variables section
        └── index.html   # + GitHub repository link
.github/
├── ci/changed-sides.sh        # docker-compose.yaml and api/cmd/beer-menu-api/* also set guide=true
├── ci/changed-sides.test.sh   # matching cases
└── workflows/pages.yml        # paths + docker-compose.yaml
```

**Structure Decision**: all changes live in the existing `guide/` site and its CI wiring. The
root `docker-compose.yaml` and the API are read, not changed.

## Complexity Tracking

No violations.
