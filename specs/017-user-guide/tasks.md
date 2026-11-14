---

description: "Task list for the admin app user guide"
---

# Tasks: Admin App User Guide

**Input**: Design documents from `specs/017-user-guide/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/guide-site.md, quickstart.md

**Tests**: Included. Constitution IV requires automated tests. `guide/build.test.mjs` checks the built-site contract (`contracts/guide-site.md`), and `.github/ci/changed-sides.test.sh` gets cases for the new `guide` rule. Wording, screenshot content and the 320 px layout are checked by hand (quickstart), and the PR body must say so.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story the task belongs to (US1–US4)

## Path Conventions

- Guide source: `guide/src/` (layout, CSS, `pages/<slug>.html` fragments, `images/<slug>[-<state>].png`).
- Build output: `guide/_site/` (gitignored).
- Build: `node guide/build.mjs`. Test: `node --test guide/`. Node 26 (`.tool-versions`), standard library only. No npm package.
- No copies: the layout, the navigation and the color tokens each have exactly one source (owner preference; research R1, R2). Never paste nav markup or hex colors into a fragment or into `guide.css`.
- Admin page wording comes from `web/src/components/SideNav.tsx` (page labels) and the page's panel component (`TapsPanel.tsx`, `OnDeckPanel.tsx`, `KegHistory.tsx`, `BeersPanel.tsx`, `BeerForm.tsx`, `SettingsPanel.tsx`, `AdminsPanel.tsx`, `TokensPanel.tsx`, `PasswordForm.tsx`, `TwoFactorPanel.tsx`, `TwoFactorSetup.tsx`, `RecoveryCodes.tsx`, `BrewfatherPanel.tsx`, `SetupForm.tsx`, `LoginForm.tsx`, `KegForm.tsx`, `KegCard.tsx`, all in `web/src/components/`). Copy button, field and heading text exactly (FR-011).
- Screenshots: 1280×800 CSS px browser viewport, admin theme set to **Dark**, data only from `guide/seed.sh` on a throwaway stack (FR-008), PNG about 300 KB or less. Any browser screenshot tool works.

---

## Phase 1: Setup

**Purpose**: Create the folders and ignore the build output.

- [X] T001 Add `guide/_site/` to `.gitignore`, and create the empty folders `guide/src/pages/` and `guide/src/images/` (each with its first real file in later tasks; add no `.gitkeep` if a later task in the same change fills it).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The build, the shared layout, base styles, the contract test and the sample data that every story's pages and screenshots need.

**⚠️ CRITICAL**: No page work starts until this phase is complete.

- [X] T002 [P] Create `guide/src/layout.html`, the shared page shell with placeholders the build fills: `{{title}}`, `{{tokens}}` (a `<style>:root{…}</style>` block), `{{nav}}`, `{{content}}`. Contract (contracts/guide-site.md): `<!doctype html>`, `<html lang="en">`, `<meta charset="utf-8">`, `<meta name="viewport" content="width=device-width, initial-scale=1">`, `<title>{{title}} – Beer Menu User Guide</title>`, `<link rel="stylesheet" href="guide.css">`, then a skip link to `#main`, a `<nav aria-label="Guide">` holding `{{nav}}`, and `<main id="main">` holding `<h1>{{title}}</h1>{{content}}`. No `<script>`. All links are relative so the site works from `file://` and under `/beer-menu/` on Pages.
- [X] T003 [P] Create `guide/src/guide.css` with the base styles. Every color comes from `var(--bg|--surface|--side|--text|--muted|--border|--accent|--accent-tint|--danger)`, and the corner radius comes from `var(--radius)`. Never write a hex color. Set `color-scheme: dark`, a `body` background of `var(--bg)` and text of `var(--text)`, and the admin app's font stack (`system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`, 15px/1.45; see `.admin` in `web/src/pages/AdminPage.css`). Make `main` content a max ~760px readable column. `figure` is a `--surface` panel with a `1px solid var(--border)` border and `var(--radius)` corners. `img { max-width: 100%; height: auto; display: block }`. `figcaption` uses `--muted`. `.warning` is a panel with a left border in `--danger`. Links use `--text`, underlined, with an `--accent` focus ring (research R7 of feature 008: accent marks things, never words). Side-nav and phone layout come in US3 (T032). Here, keep the nav stacked above `main` so the pages are usable.
- [X] T004 Create `guide/build.mjs`, a Node ES module that uses only `node:fs`, `node:path` and `node:url`, with a header comment stating its inputs, outputs and failure rules (contracts/guide-site.md, Build). It must:
  (a) Hold the only navigation list: an array of `{ slug, title, group }`. For now it has only `{ slug: "index", title: "Home", group: "Start" }`, and later stories add entries in data-model.md order. Group values: `"Start"`, `"Menu"`, `"Administration"`, `"Integrations"`.
  (b) Read the tokens from `web/src/pages/AdminPage.css`. Take the custom properties (`--name: value;`) from the first block whose selector is exactly `.admin`, then override them with those from the first block whose selector is exactly `.admin.theme-dark`. Fail with a message naming what is missing if either block, or any of `--bg --surface --side --text --muted --border --accent --accent-tint --danger --radius`, is absent. Emit `:root{color-scheme:dark;--bg:…;…}`. Export the required-token list so `build.test.mjs` uses the same one.
  (c) For each nav entry, read `guide/src/pages/<slug>.html`, fill the layout, and write `guide/_site/<slug>.html`. Fail if a nav entry has no fragment, or if a `.html` file in `src/pages/` has no nav entry.
  (d) Render the nav as one `<h2>` per group, followed by a `<ul>` of links (`<a href="<slug>.html">`). The current page's link gets `aria-current="page"`. Escape titles for HTML.
  (e) Empty `guide/_site/` first, then copy `guide/src/guide.css` to `_site/guide.css` and `guide/src/images/` to `_site/images/`.
  (f) Export a `build({ root })` function that returns the output folder, and run it when the file is run directly, so the test can call it.
  Keep it short (about 100 lines). Add no options and no config.
- [X] T005 Create `guide/src/pages/index.html`, a minimal home fragment: one `<p>` saying what Beer Menu is (a tap list for a home bar, shown on a TV and managed in a web admin app). The page list comes from the nav, so do not repeat it here. US1 expands this fragment (T019).
- [X] T006 Create `guide/build.test.mjs` (`node:test`, `node:assert/strict`, standard library only), with a header comment linking `specs/017-user-guide/contracts/guide-site.md`. Call `build()` once in a `before` hook, then parse each `_site/*.html` with regular expressions (no HTML-parser dependency). Test:
  (1) Every nav entry has an output file and every output file is in the nav.
  (2) Each page has `lang="en"`, charset, viewport and a `<title>` ending in ` – Beer Menu User Guide`.
  (3) Each page's `:root` declares the same required tokens as T004 (b) (import the list from `build.mjs`, do not copy it), and `--bg` equals the `.admin.theme-dark` value read from `AdminPage.css` in the test itself.
  (4) Exactly one `<h1>`, and no heading level is skipped.
  (5) Each page's nav lists every page in order, and exactly one link has `aria-current="page"`, pointing to that page.
  (6) Every local `href`/`src` (not starting with `https:`, `#` or `mailto:`) resolves to a file in `_site/`, and every external link starts with `https:`.
  (7) Every `<img>` has a non-empty `alt`, and no link text is "click here", "here" or "this page" (FR-019).
  (8) There is no `<script` and no ` on[a-z]+=` attribute.
  (9) Every page whose group is not `Start`, plus `getting-started`, has a `<figure>` with an `<img src="images/…">` and an `<h2>` followed (anywhere later) by an `<ol>`.
  (10) Token extraction fails with a clear error when given CSS without `.admin.theme-dark`. Export or call the extraction function on a string for this; do not touch the real file.
  Run `node --test guide/`: all pass.
- [X] T007 [P] Create `guide/seed.sh` (POSIX sh and `curl` only; read the few JSON fields needed, such as ids, with `sed`, not `jq`), with a header comment saying it fills a **fresh, throwaway** local stack with invented sample data for screenshots (research R5). Use `BASE=${1:-http://localhost:8080}` and a temporary cookie jar removed on exit (`trap`). Steps:
  (a) `GET $BASE/api/setup`. If `needed` is not `true`, print "setup already done; seed.sh only runs on a fresh stack" and exit 1.
  (b) `POST /api/setup` `{"username":"brewer","password":"<fixed invented password>"}`. This signs in and stores the session cookie.
  (c) Immediately after setup, while the setup session still counts as a recent sign-in (`requireConfirmed` in `api/internal/httpapi/twofactor.go`), create a second admin `POST /api/admins` (for the Admin Users screenshot) and an API token `POST /api/tokens` `{"name":"Kitchen tablet","scope":"read"}`. No `POST /api/2fa/confirm` is needed. Then create 6–8 invented beers `POST /api/beers` with name, style, abv, ibu, srm, description (fictional names only, e.g. "Porch Light Pale Ale"); 4 taps, using the default and adding taps via `POST /api/taps`; kegs `POST /api/kegs` on 3 of the 4 taps (leave one tap empty), 2 waiting kegs (no `tap_id`), and 2 kegs that are then kicked (`PATCH /api/kegs/{id}` `{"kicked": true}`), with some `remaining_l` lowered so level bars vary.
  Read request shapes from `api/openapi.yaml`. `curl` sends no `Origin` or `Sec-Fetch-Site`, so the cross-site check passes. Use `curl -fsS` and stop on the first failure (`set -eu`). Finally, print the admin username and password and the URLs for the admin app and `/display`.
  Verify by running it against a fresh stack (quickstart step 3) and then running it again: the second run must exit 1 without writing anything.

**Checkpoint**: `node guide/build.mjs && node --test guide/` passes with the home page only, and `guide/seed.sh` fills a fresh stack.

---

## Phase 3: User Story 1 - Learn how to run the taps from the guide (Priority: P1) 🎯 MVP

**Goal**: A home page plus one page for each Menu-group admin page, each with a screenshot and numbered task steps.

**Independent Test**: Build, open `guide/_site/index.html`, follow each Menu link, and check for a screenshot plus steps on each page. A new user can put a keg on a tap using only the Taps page (SC-003).

### Screenshots for User Story 1

Before T008–T013, start a throwaway stack and seed it (quickstart step 3), sign in, and set the theme to Dark.

- [X] T008 [P] [US1] Capture `guide/src/images/taps.png`: the Taps page with 3 taps holding kegs and 1 empty tap.
- [X] T009 [P] [US1] Capture `guide/src/images/on-deck.png`: the On Deck page listing the 2 waiting kegs.
- [X] T010 [P] [US1] Capture `guide/src/images/keg-history.png`: the Keg History page listing the kicked kegs.
- [X] T011 [P] [US1] Capture `guide/src/images/beers.png` (Beer Library list) and `guide/src/images/beers-form.png` (the add/edit beer form open on a sample beer).
- [X] T012 [P] [US1] Capture `guide/src/images/display.png`: the Display Settings page.
- [X] T013 [P] [US1] Capture `guide/src/images/tv-display.png`: `http://localhost:8080/display` showing the seeded taps (used on the home page now and Getting started in US2).

### Pages for User Story 1

Each fragment follows the data-model.md structure: a `<p>` saying what the page is for; `<figure><img src="images/<file>" alt="<what it shows>"><figcaption>…</figcaption></figure>`; one `<h2>` per task, each followed by an `<ol>` of steps; and a `<div class="warning">` wherever an action cannot be undone (FR-012). Write plain words with no code or developer terms (FR-010), and use the app's exact wording from the panel component (FR-011).

- [X] T014 [P] [US1] Write `guide/src/pages/taps.html` from `TapsPanel.tsx`, `KegCard.tsx` and `KegForm.tsx`. Cover every task the page offers, in the order a user does them: what a tap card shows, putting a keg on an empty tap, recording a pour or setting how much is left, moving or taking a keg off, kicking a keg (with a warning if it cannot be undone), adding, renaming, reordering, disabling and removing taps (with a warning: only a tap with no keg can be removed, and at least one must remain).
- [X] T015 [P] [US1] Write `guide/src/pages/on-deck.html` from `OnDeckPanel.tsx`: what On Deck is (kegs waiting off-tap), adding a waiting keg, putting one on a tap, editing and deleting one (with a warning).
- [X] T016 [P] [US1] Write `guide/src/pages/keg-history.html` from `KegHistory.tsx`: what appears there (kicked kegs), how to read it, and any actions the page offers.
- [X] T017 [P] [US1] Write `guide/src/pages/beers.html` from `BeersPanel.tsx`, `BeerForm.tsx` and `BeerPicker.tsx`: adding a beer (each form field explained), editing, archiving and restoring, and deleting (with a warning that it is permanent). Note that beers imported from Brewfather are managed on the Brewfather page.
- [X] T018 [P] [US1] Write `guide/src/pages/display.html` from `SettingsPanel.tsx`: what each display setting changes on the TV, and how to save.
- [X] T019 [US1] Expand `guide/src/pages/index.html`: introduce Beer Menu for end users (what the TV shows, what the admin app is for), include the `tv-display.png` figure, and add a short "Where to start" paragraph that names the Taps page as the everyday page. Link to pages only by their `.html` slug.
- [X] T020 [US1] In `guide/build.mjs`, add nav entries after `index`, in this order and with these exact titles: `taps` "Taps", `on-deck` "On Deck", `keg-history` "Keg History", `beers` "Beer Library", `display` "Display Settings", all with group `"Menu"`. Run `node guide/build.mjs && node --test guide/`: all pass.

**Checkpoint**: US1 is a usable MVP guide (quickstart steps 1–2 for the Menu pages).

---

## Phase 4: User Story 2 - Cover the rest of the admin app (Priority: P2)

**Goal**: Pages for Install with Docker, Getting started, the four Administration pages and Brewfather.

**Independent Test**: Every page listed in data-model.md exists with its screenshot(s) and steps. Following Install with Docker on a machine with Docker starts the app.

### Screenshots for User Story 2

Use the seeded stack from US1. For the setup and sign-in screenshots, use a second, empty throwaway stack (or sign out first for sign-in).

- [X] T021 [P] [US2] Capture `guide/src/images/getting-started-setup.png` (the first-run setup form on an empty stack, before submitting) and `guide/src/images/getting-started-sign-in.png` (the sign-in form, empty fields).
- [X] T022 [P] [US2] Capture `guide/src/images/admins.png`: Admin Users with the two seeded admins.
- [X] T023 [P] [US2] Capture `guide/src/images/tokens.png` (token list with "Kitchen tablet") and `guide/src/images/tokens-new.png` (the dialog that shows a newly created token once). Those values come from this throwaway stack only (FR-008).
- [X] T024 [P] [US2] Capture `guide/src/images/password.png`: the Change Password form, empty.
- [X] T025 [P] [US2] Capture `guide/src/images/two-factor.png` (page with no factors set up) and `guide/src/images/two-factor-setup.png` (the authenticator setup step with its QR code, from the throwaway stack only).
- [X] T026 [P] [US2] Capture `guide/src/images/brewfather.png`: the Brewfather page, not connected. Never enter a real API key.

### Pages for User Story 2

Same fragment structure and wording rules as US1.

- [X] T027 [P] [US2] Write `guide/src/pages/install.html` (FR-004a, first start only; no screenshot is required). It assumes Docker with Compose is installed. Steps: get the files (`git clone https://github.com/helmnet/beer-menu.git` or download the repo), go into the folder, choose and create a data folder, then start with the same commands as `README.md` "Run with Docker Compose" (`export PUID=$(id -u) PGID=$(id -g)`, `mkdir -p data`, `docker compose pull`, `docker compose up -d`). Then open `http://localhost:8080/` for the admin app and `http://localhost:8080/display` for the TV, and go on to Getting started. Put commands in `<pre><code>`, the one place code is allowed (FR-010). Do NOT cover upgrades, other variables, ports or backup restore (FR-004a). Add a `pre` style to `guide/src/guide.css` (`--surface` background, `--border`, scroll inside the block only, `overflow-x: auto`, so the page never scrolls sideways).
- [X] T028 [P] [US2] Write `guide/src/pages/getting-started.html` from `SetupForm.tsx` and `LoginForm.tsx`: creating the first admin (only on a new install), signing in, the second-factor step if one is set up (authenticator code, recovery code or passkey), and opening the TV display at `/display` on the TV's browser. Include the setup, sign-in and `tv-display.png` figures.
- [X] T029 [P] [US2] Write `guide/src/pages/admins.html` (`AdminsPanel.tsx`), `guide/src/pages/tokens.html` (`TokensPanel.tsx`), `guide/src/pages/password.html` (`PasswordForm.tsx`), `guide/src/pages/two-factor.html` (`TwoFactorPanel.tsx`, `TwoFactorSetup.tsx`, `RecoveryCodes.tsx`, `ConfirmIdentity.tsx`) and `guide/src/pages/brewfather.html` (`BrewfatherPanel.tsx`). Each needs these warnings (FR-012):
  - tokens: shown only once, and revoking cannot be undone (US2 scenario 3);
  - two-factor: save the recovery codes, which are shown once (US2 scenario 2);
  - admins: removing an admin cannot be undone;
  - pages that ask you to confirm your identity first say so.
  The API Tokens page may say what a token is for (e.g. letting Home Assistant or a script read the tap list) but shows no requests (FR-010).
- [X] T030 [US2] In `guide/build.mjs`, add nav entries with these exact slugs, titles and groups:
  - after `index`, in group `"Start"`: `install` "Install with Docker", `getting-started` "Getting started";
  - after `display`, in group `"Administration"`: `admins` "Admin Users", `tokens` "API Tokens", `password` "Change Password", `two-factor` "Two-Factor Sign-In";
  - in group `"Integrations"`: `brewfather` "Brewfather".
  The final order is data-model.md's (13 entries). Run `node guide/build.mjs && node --test guide/`: all pass.

**Checkpoint**: All 13 pages exist and the contract test passes (SC-001, SC-005).

---

## Phase 5: User Story 3 - The guide looks like the app (Priority: P3)

**Goal**: The app's dark-mode look (side panel, panels, accent) on desktop, and a phone layout with no sideways scroll and no JavaScript.

**Independent Test**: Compare the guide with the app in Dark (quickstart step 3). At 320 px, there is no horizontal scroll and Menu opens the nav (quickstart step 2).

- [X] T031 [US3] In `guide/src/layout.html` and `guide/build.mjs`, wrap `{{nav}}` in `<details class="nav-toggle"><summary>Menu</summary>…</details>` (no `open`; T032 shows it on desktop) inside the `<nav>`. The build emits the same markup for every page.
- [X] T032 [US3] In `guide/src/guide.css`, add the app-like layout (research R6):
  - Base (phone first): the nav sits above `main`. `summary` "Menu" is a full-width, button-like row (`--surface` background, `--border`, `var(--radius)`), and the `<details>` starts closed because the markup has no `open`.
  - At `min-width: 800px`: a fixed left side panel (`--side` background, `1px solid var(--border)` right border, width 256px like the app's default `--nav-width`). `main` is offset by that width. Hide `summary`, and show the nav contents with no JavaScript: `details.nav-toggle::details-content { content-visibility: visible; display: block; }`. A browser without `::details-content` still shows the Menu toggle on desktop, which is acceptable; say so in a one-line comment.
  - Group `<h2>`s are small and uppercase in `--muted`. Nav links are rounded rows. The current page (`[aria-current="page"]`) has an `--accent-tint` background and an `--accent` left marker, and its text stays `--text`.
  - Add `overflow-wrap: anywhere` on `main`, use no fixed widths below 800px, and let `pre` scroll inside itself only.
  Check by hand at 320, 800 and 1280 px in a browser, with JavaScript off.
- [X] T033 [US3] Compare side by side with the seeded app in Dark: background, panels, side panel, text, muted text, borders, accent and corner radius match (US3 scenario 1). Fix any differences in `guide/src/guide.css` only through the tokens. If a needed token is missing from the extraction list, add it in `guide/build.mjs` and to test (3) in `guide/build.test.mjs`.

**Checkpoint**: US3 acceptance scenarios pass (quickstart steps 2–3).

---

## Phase 6: User Story 4 - Keep the guide current (Priority: P3)

**Goal**: CI checks the guide, a merge to `main` publishes it, and maintainers know how to update it.

**Independent Test**: A PR touching `guide/` runs the `guide` job. After merge, the `pages` workflow deploys, and the change appears at `https://helmnet.github.io/beer-menu/` (SC-006).

- [X] T034 [P] [US4] In `.github/ci/changed-sides.sh`, add a `guide` output: `guide=false` at the start; `guide/*` sets `guide=true` (after the `*.md` case, so `guide/README.md` alone does not trigger it); `web/src/pages/AdminPage.css` sets `web=true guide=true`, placed before the `web/*` case; the `.github/workflows/ci.yml | .github/ci/*` case and the `ALL` case also set `guide=true`; and `.github/workflows/pages.yml` sets `guide=true`. Print `guide=` in the final `printf`. Update the header comment to list `guide=`. Keep the `case` as the single source of truth.
- [X] T035 [P] [US4] In `.github/ci/changed-sides.test.sh`, add cases in the file's existing style: `guide/build.mjs` gives guide=true and others false; `guide/README.md` gives guide=false; `web/src/pages/AdminPage.css` gives web=true and guide=true; `ALL` and `.github/ci/x` give guide=true; `api/x.go` gives guide=false. Update every existing expected-output line for the new `guide=` line. Run `sh .github/ci/changed-sides.test.sh`: it passes.
- [X] T036 [US4] In `.github/workflows/ci.yml`, add `guide: ${{ steps.sides.outputs.guide }}` to the `changes` job outputs. Add a `guide` job: `needs: changes`, `if: needs.changes.outputs.guide == 'true'`, `runs-on: ubuntu-latest`, `permissions: contents: read`. Steps: `actions/checkout` (same SHA and `persist-credentials: false` as the `web` job), `actions/setup-node` (same SHA, `node-version: 26`, no npm cache), then `run: node --test guide/`. Update the header comment's list of checks never to merge with failing or pending.
- [X] T037 [US4] Create `.github/workflows/pages.yml`, with a header comment on what it does and the owner steps (Settings → Pages → Source: GitHub Actions; add `guide` to the `main` ruleset's required status checks):
  - Triggers: `on: push: branches: [main], paths: ['guide/**', 'web/src/pages/AdminPage.css', '.github/workflows/pages.yml']`, plus `workflow_dispatch`.
  - Top-level `permissions: contents: read`, and `concurrency: { group: pages, cancel-in-progress: false }`.
  - Job `build`: checkout (same SHA as ci.yml, `persist-credentials: false`), setup-node 26, `node --test guide/` (this builds too), then `actions/upload-pages-artifact` with `path: guide/_site`.
  - Job `deploy`: `needs: build`, with `permissions: pages: write, id-token: write` on this job only, `environment: { name: github-pages, url: ${{ steps.deployment.outputs.page_url }} }`, and steps `actions/configure-pages` and `actions/deploy-pages` (`id: deployment`).
  - Pin every action by full commit SHA with a `# vX.Y.Z` comment. Look up the latest release SHAs with `gh api repos/actions/<name>/git/ref/tags/<tag>` (dereference annotated tags). Never use a floating tag.
- [X] T038 [P] [US4] Create `guide/README.md` (maintainer notes, not published). It covers:
  - What the guide is and the folder layout.
  - How to build, test and preview (`node guide/build.mjs`, `node --test guide/`, open `guide/_site/index.html`).
  - Adding a page: write the fragment and add the nav entry in `build.mjs`.
  - Where colors come from: `AdminPage.css`, never hex in `guide.css`.
  - Retaking screenshots: the throwaway stack plus `guide/seed.sh`, 1280×800, theme Dark, the full file list with each one's state from data-model.md, and the rule that only seed data may appear (FR-008, FR-017).
  - That a change to an admin page should update its guide page and screenshot in the same PR.
  - The one-time owner steps: the Pages source, and `guide` as a required status check.
- [X] T039 [P] [US4] In `README.md`, add a short "User guide" link to `https://helmnet.github.io/beer-menu/` near the top, after the badges and intro (FR-018).
- [X] T040 [P] [US4] In `docs/development.md` "Required checks", add `guide` to the required checks list and state what triggers it (from T034). Also mention `guide/README.md` in the Layout section.

**Checkpoint**: CI runs the guide check on guide PRs, and the publish workflow is ready (it deploys after merge, once the owner has set the Pages source).

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T041 Wording pass: open each admin page in the seeded app in Dark and check every button, field and heading named in each `guide/src/pages/*.html` against the screen (FR-011). Also check that every link's text names its target (FR-019). Fix the guide, never the app.
- [X] T042 Privacy pass: open every `guide/src/images/*.png` and confirm it shows only seed data: no real names, hostnames, IPs, or tokens or codes from any non-throwaway instance (FR-008, SC-007). Shrink any PNG over ~300 KB (e.g. `oxipng` or `pngquant` if installed, otherwise retake), without adding a repo dependency.
- [X] T043 Run quickstart.md steps 1–4 and 6 end to end, then tear down the throwaway stacks and delete their data folders.
- [X] T044 [P] In `docs/project-brief.md` "Spec-kit feature breakdown", add row `| 17 | User guide | 16 | A static end-user guide for the admin app on GitHub Pages: one page per admin page with dark-mode screenshots from sample data, a short Install with Docker page and a Getting started page; published on merge to main |`, and add after the table that the guide follows feature 16 and runs before roadmap phase 2 (constitution Development Workflow).
- [X] T045 Final checks: `node guide/build.mjs && node --test guide/`, `sh .github/ci/changed-sides.test.sh`, and `git status`, which must not list `guide/_site/`. In the PR body, list the manual checks (wording, privacy, 320 px, JavaScript off) and the owner's one-time steps: Pages source set to GitHub Actions, and `guide` added to the `main` ruleset's required status checks.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: after Setup. T002, T003 and T007 run in parallel. T004 needs T002. T005 is a fragment the build needs. T006 needs T004 and T005.
- **US1 (Phase 3)**: after Foundational. The screenshots (T008–T013) need T007's seeded stack.
- **US2 (Phase 4)**: after Foundational. It does not depend on US1's pages, but T028 reuses T013's image, and T030 edits the same nav list as T020, so do them in sequence.
- **US3 (Phase 5)**: after Foundational. It is best done after US1 and US2 so the layout is checked with all pages.
- **US4 (Phase 6)**: after Foundational. T034 is before T035 and T036, and T036 is before T037 (same SHAs). T038–T040 run in parallel.
- **Polish (Phase 7)**: after all stories.

### Within Each Story

- Screenshots come before the pages that embed them. The nav entries (T020, T030) come last, because the build fails if a nav entry has no fragment.

### Parallel Opportunities

- Phase 2: T002, T003 and T007.
- US1: T008–T013 (screenshots), then T014–T018 (fragments).
- US2: T021–T026, then T027–T029.
- US4: T034 with T038–T040.

## Parallel Example: User Story 1

```text
Screenshots: T008 taps.png | T009 on-deck.png | T010 keg-history.png | T011 beers*.png | T012 display.png | T013 tv-display.png
Then pages:  T014 taps.html | T015 on-deck.html | T016 keg-history.html | T017 beers.html | T018 display.html
Then:        T019 index.html → T020 nav entries + test
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 + Phase 2: the build, layout, test and seed work.
2. Phase 3: home page plus the five Menu pages.
3. **Stop and validate**: build, test, preview locally.

### Incremental Delivery

1. US1 is the Menu guide (MVP).
2. US2 adds the remaining pages, so the guide is complete.
3. US3 adds the app look and the phone layout.
4. US4 adds CI and publishing, so the guide goes live on merge.

All four stories ship in one PR. The order above is the build order, not separate releases.

## Notes

- [P] tasks touch different files and have no dependency on unfinished tasks.
- Commit after each phase.
- Screenshots and wording are manual work. Run them with a real browser against the throwaway stack, never against a real instance.
