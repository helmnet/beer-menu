# Research: Admin App User Guide

## R1. How to build 13 pages that share one layout

- **Decision**: write each page body as a hand-written HTML fragment in `guide/src/pages/`. Then
  `guide/build.mjs`, which uses only the Node standard library, puts each fragment into
  `guide/src/layout.html`. It fills in the page title, the shared navigation (built from one
  list in the script, with the current page marked by `aria-current="page"`), and the color
  tokens. The output goes to `guide/_site/`.
- **Rationale**: copying the side navigation into 13 files would let them drift apart, and the
  repo owner does not want copied code. A short script with no dependencies is less to maintain
  than a site generator. Node 26 is already the repo's toolchain (`.tool-versions`, web CI).
  The output is plain HTML, so it works with JavaScript off (FR-015).
- **Alternatives considered**:
  - Plain HTML with the nav copied into every page: rejected because of the copies.
  - Jekyll (GitHub Pages' built-in generator): previewing the site locally would need Ruby, it
    adds a templating language, and it cannot read the tokens from the app's CSS (R2).
  - Markdown with a converter (for example `marked`): adds a dependency, and the pages need
    figures and warning boxes that Markdown does not express well.
  - Loading the nav with JavaScript: fails FR-015.

## R2. How to match the admin app's dark mode without copying its colors

- **Decision**: `build.mjs` reads `web/src/pages/AdminPage.css`. It takes the custom properties
  from the `.admin { … }` block, then overrides them with those from the first
  `.admin.theme-dark { … }` block, and writes the result into the page head as
  `:root { … }`. `guide/guide.css` uses colors only through `var(--…)`. The build fails with a
  clear error if either block or any required token (`--bg --surface --side --text --muted --border --accent --accent-tint --danger --radius`) is missing.
- **Rationale**: the app's stylesheet stays the only source for the colors (FR-013). If the app's
  dark palette changes, the next build of the guide follows it. The CI rule runs the guide test
  when `AdminPage.css` changes, so a refactor that breaks the extraction fails the PR, not the
  publish.
- **Alternatives considered**: copying the hex values into `guide.css` (copies that drift);
  moving the tokens into a shared CSS file that both import (a refactor of the app's
  stylesheet, which this feature has no other reason to touch, and the app's system-theme media
  query would still repeat the values).
- **Dark only**: the guide sets `color-scheme: dark` and has no light tokens or theme switch,
  per the spec.

## R3. How to publish

- **Decision**: add a new workflow, `.github/workflows/pages.yml`. It runs on `push` to `main` with
  a path filter of `guide/**`, `web/src/pages/AdminPage.css` and the workflow itself, and on
  `workflow_dispatch`. A build job runs `node guide/build.mjs` and the guide test, then
  `actions/upload-pages-artifact` uploads `guide/_site`. A deploy job uses
  `actions/deploy-pages` with `pages: write` and `id-token: write`, and only that job gets
  those permissions. A `concurrency: pages` group keeps two deploys from overlapping. Actions
  are pinned by commit SHA with a version comment, like `ci.yml` and `release.yml`.
- **Rationale**: publishing through GitHub Actions needs no `gh-pages` branch and puts no built
  files in git. Each merge publishes within minutes (SC-006). The path filter skips deploys
  for unrelated merges.
- **Alternatives considered**: a `gh-pages` branch pushed by the workflow (more history noise
  and needs `contents: write`); serving `/docs` from `main` (it would publish the developer
  docs and the raw source, and needs a build step anyway).
- **Owner step**: set Settings → Pages → Source to "GitHub Actions" before the first deploy.

## R4. How to take screenshots

- **Decision**: take them by hand. Start a fresh local stack with `docker compose up` and an
  empty data folder, then run `guide/seed.sh`. Use a desktop browser window sized to
  1280×800 CSS px with the admin app's theme set to Dark, and save one PNG per state to
  `guide/src/images/<page>.png`. `guide/README.md` lists every screenshot and the state to
  show.
- **Rationale**: about 15 images that change rarely do not justify adding Playwright, which
  downloads browsers and is a new dependency (constitution VII). The seed script makes the
  data repeatable, and that is the part that matters for keeping images consistent
  (US4 scenario 2).
- **Alternatives considered**: Playwright or Puppeteer scripted capture. These could come later
  if the screenshots become a burden to retake.

## R5. Sample data and secrets in screenshots

- **Decision**: `guide/seed.sh` takes a base URL (default `http://localhost:8080`). It creates the
  first admin (`POST /api/setup`) with invented names. It signs in with a cookie jar, creates a
  write token, and with it adds invented beers, taps, kegs (on tap, on deck and kicked) and
  display settings. It refuses to run if setup is already done, so it can never write to a
  real instance. Screenshots of pages that show secrets (a new API token, recovery codes, the
  authenticator QR code) come from this throwaway stack, which is deleted afterwards, so those
  values are never valid anywhere (FR-008). The Brewfather page is shown not connected.
- **Rationale**: sample data that can be rebuilt from scratch is safe to publish. The setup
  check stops anyone from seeding a production instance by mistake.
- **Recent sign-in**: `POST /api/admins` and `POST /api/tokens` need a session signed in or
  confirmed within the confirm window (`requireConfirmed` in
  `api/internal/httpapi/twofactor.go`). The session that `POST /api/setup` starts counts, so the
  script makes those two calls first, right after setup, and needs no `POST /api/2fa/confirm`.
- **Note**: sign-in and setup refuse cross-site browser requests (by checking `Sec-Fetch-Site` or
  `Origin`). `curl` sends neither header, so the script works without extra headers.

## R6. Layout at phone width without JavaScript

- **Decision**: at widths of 800 px and up, the nav is a fixed left side panel styled like the
  app's (`--side` background, group headings, the current page highlighted with
  `--accent-tint`). Below 800 px, the nav moves to the top of the page inside a
  `<details><summary>Menu</summary>` element, which opens and closes with no script.
  Screenshots use `max-width: 100%; height: auto` and sit inside rounded `--surface` panels.
- **Rationale**: this meets FR-014 and SC-004 with no sideways scrolling, and meets FR-015.

## R7. Guide wording

- **Decision**: page text uses the names the app shows, read from the side panel
  (`web/src/components/SideNav.tsx`) and the page components (for example `TapsPanel.tsx` and
  `OnDeckPanel.tsx`) when the pages are written. Every name is checked against the running
  app when the screenshots are taken (quickstart step 5).
- **Rationale**: FR-011 requires exact matches, and the app's own source and screen are the only
  sources.

## R8. CI

- **Decision**: add a `guide=` output to `.github/ci/changed-sides.sh`. It is true for `guide/*`,
  `web/src/pages/AdminPage.css`, and every path that already forces all checks (the CI
  definition and its helpers). Add cases to `changed-sides.test.sh`. Add a `guide` job to
  `ci.yml` that runs `node --test guide/` with Node 26. Add `guide` to the required checks in
  `docs/development.md`. The existing `*.md` skip keeps applying, so changing only
  `guide/README.md` does not run the job.
- **Rationale**: the constitution requires CI to run every check that a PR's changed files can
  affect.
