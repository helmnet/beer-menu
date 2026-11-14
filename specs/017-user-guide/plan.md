# Implementation Plan: Admin App User Guide

**Branch**: `compmodder26/user_guide` | **Date**: 2026-11-14 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/017-user-guide/spec.md`

## Summary

An end-user guide for the admin app, published to GitHub Pages at
`https://helmnet.github.io/beer-menu/`. It has 13 static HTML pages: home, Install with Docker,
Getting started, and one page per admin side-panel page (10). Each page has dark-mode screenshots
taken from a disposable local stack filled with sample data.

The guide source lives in a new top-level `guide/` folder. Each page body is a hand-written HTML
fragment. A dependency-free Node script (`guide/build.mjs`) wraps every fragment in one shared
layout and one shared navigation list, so neither is copied 13 times. It also reads the color
tokens straight from the admin app's stylesheet (`web/src/pages/AdminPage.css`), so the guide
cannot drift from the app's dark mode. A Node test checks the built site: every nav page exists,
there are no broken links or images, every screenshot has alt text, and every admin page has a
screenshot.

CI runs that test when `guide/` or the token source changes. A new Pages workflow publishes the
built site on every push to `main` that touches those same paths. The guide needs no JavaScript.

## Technical Context

**Language/Version**: HTML5 and CSS for the guide. Node 26.13 (`.tool-versions`) ES modules for the
build and test scripts. POSIX sh plus `curl` for the sample-data seed script; JSON fields it needs are read with `sed`, so no `jq`.

**Primary Dependencies**: none new. The scripts use only the Node standard library (`node:fs`,
`node:path`, `node:test`, `node:assert`). The workflow uses the official Pages actions
(`actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`), pinned
by commit SHA like the existing workflows.

**Storage**: files only: `guide/src/` (source) and `guide/_site/` (build output, gitignored).

**Testing**: `node --test guide/` covers the build output (contract in
[contracts/guide-site.md](contracts/guide-site.md)) and token extraction. The
`changed-sides.test.sh` cases are extended for the new `guide` output. Wording and screenshot
content are checked by hand against the running app ([quickstart.md](quickstart.md)).

**Target Platform**: GitHub Pages (static hosting). Evergreen desktop and phone browsers, from
320 px wide, with JavaScript off.

**Project Type**: static documentation site inside the existing web-application repo.

**Performance Goals**: pages are plain HTML and CSS. Screenshots are PNGs at 1280×800, kept under
about 300 KB each.

**Constraints**: no JavaScript; dark style only; no new dependency; no secrets or real data in
screenshots; the app's button and label wording is copied exactly.

**Scale/Scope**: 13 pages, about 15 screenshots, 1 build script, 1 test file, 1 seed script,
1 new workflow, 1 new CI job, and an updated change-rules script.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. API-First | Documentation only. No capability is added and no endpoint changes. | Pass (N/A) |
| II. Go + SQLite | No backend change. | Pass (N/A) |
| III. React Clients | The guide is documentation, not an app client, so plain HTML is allowed (research R1). The admin app and the TV display are untouched. | Pass |
| IV. Tests Required | `guide/build.test.mjs` tests the build and the site contract, and runs in a new CI `guide` job. `changed-sides.test.sh` gets cases for the new rule. | Pass |
| V. LAN Security | Screenshots come from a throwaway local stack with invented data (research R5). No hostnames, IPs or secrets are committed. The Pages workflow has the least permissions it needs (`pages: write` and `id-token: write`, deploy job only) and pins actions by SHA. | Pass |
| VI. Documented | Header comments on the scripts and workflow say what they do and their contracts. Maintainer steps are in `guide/README.md`. | Pass |
| VII. Simplicity | No static-site generator, no npm package, no JavaScript in the guide. One ~100-line build script replaces 13 copies of the layout. Screenshots are taken by hand from seeded data, not with a browser-automation dependency (research R4). Publishing on GitHub Pages hosts docs, not the app, so the "no public cloud hosting" rule is not affected. | Pass |
| Deployment / Workflow | The CI change rules in `.github/ci/changed-sides.sh` gain a `guide` output, and the token source file triggers it too. | Pass |

Post-design re-check (after Phase 1): unchanged, all pass. No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/017-user-guide/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output: page list and screenshot rules
├── quickstart.md        # Phase 1 output: validation guide
├── contracts/
│   └── guide-site.md    # Phase 1 output: built-site contract the test checks
└── tasks.md             # Phase 2 output (/speckit-tasks, not created here)
```

### Source Code (repository root)

```text
guide/
├── README.md               # maintainer steps: build, preview, seed, retake screenshots
├── build.mjs               # layout + nav + tokens + fragments -> guide/_site/
├── build.test.mjs          # node:test: tokens extracted, site contract holds
├── seed.sh                 # fills a fresh local stack with sample data via the API
└── src/
    ├── layout.html         # shared page shell: head, side nav slot, main slot
    ├── guide.css           # layout and components; colors only via var(--token)
    ├── pages/              # one body fragment per page (13), e.g. taps.html
    └── images/             # screenshots, e.g. taps.png
.github/
├── ci/changed-sides.sh     # + guide output (guide/*, web/src/pages/AdminPage.css)
├── ci/changed-sides.test.sh
└── workflows/
    ├── ci.yml              # + guide job: node --test guide/
    └── pages.yml           # new: build and deploy guide/_site to GitHub Pages on main
.gitignore                  # + guide/_site/
README.md                   # + link to the published guide
```

**Structure Decision**: a top-level `guide/` folder that sits beside `api/` and `web/`. `docs/`
stays developer documentation. The guide has its own folder so that the CI change rules and the
Pages workflow path filter can each match it with one pattern.

## Complexity Tracking

No constitution violations.

## Manual steps for the repository owner

1. Before the first deploy, set **Settings → Pages → Source** to **GitHub Actions**. The deploy
   job fails until it is set.
2. Add `guide` to the required status checks of the `main` branch ruleset, next to the existing
   checks, so a failing guide check blocks merging.

Only the repository owner can change these settings.
