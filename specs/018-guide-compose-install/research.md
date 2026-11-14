# Research: Install From a Compose File, Without Cloning

## R1. Where the guide's compose file comes from

- **Decision**: `guide/build.mjs` derives the compose file from the repo's root `docker-compose.yaml`
  at build time: drop full-line comments and `build:` keys, collapse repeated blank lines, trim.
  The Install page fragment holds a `{{compose}}` placeholder that the build fills with the
  HTML-escaped result.
- **Rationale**: one source, no copy to drift (FR-003 "same defaults"; owner rule: no
  duplication). The root file's kept lines are already the barebones file: two services, images,
  `user:`, data mount, port, `restart:`. Everything optional in it is commented out today.
- **Alternatives considered**:
  - Hand-written copy in the fragment plus a test comparing it to the root file: two copies,
    and the test would need the same stripping logic anyway.
  - A separate `guide/src/docker-compose.yaml`: same drift problem.
  - Publishing the file for download (`curl -O`): not asked; copy-and-save meets FR-005.

## R2. One-click copy, given the "no script" rule

- **Decision**: one classic script file, `guide/src/copy.js`, loaded with
  `<script src="copy.js" defer></script>` only by the Install page fragment. It finds every
  `<pre class="copy">`, adds a "Copy" button after it, and on click writes the `<code>` text
  with `navigator.clipboard.writeText`. Success: label "Copied" for 2 s. Failure: label
  "Copy failed", and the code text is selected so Ctrl+C / long-press works.
- **Rationale**: the 017 contract bans `<script>` and inline `on*=` handlers to keep the site
  static and CSP-friendly. An external file with no inline code keeps that intent; the button is
  created by the script, so without scripts there is no dead button and the text stays
  selectable (spec edge cases). A classic script (not a module) still runs when the built site is
  opened from the file system, which the guide README promises.
- **Alternatives considered**:
  - `type="module"`: blocked on `file://` in Chromium, breaks local preview.
  - Script in `layout.html` for every page: loads code on 12 pages that have nothing to copy.
  - `document.execCommand("copy")` fallback: deprecated; selecting the text is enough.

## R3. Testing the copy script without a browser

- **Decision**: `guide/build.test.mjs` runs `copy.js` in `node:vm` with a minimal fake
  `document` / `navigator` / `window` and checks: one button per `pre.copy`; click writes the exact
  code text; resolved write shows "Copied"; rejected write shows "Copy failed" and selects the
  text. Node standard library only, as the guide build is.
- **Rationale**: Constitution IV requires automated tests for behavior; this needs no new
  dependency.
- **Alternatives considered**: jsdom or Playwright: new dependencies for ~20 lines of script
  (Constitution VII).

## R4. Keeping the environment variable list complete (SC-003)

- **Decision**: a test collects variable names from the sources and requires each in the
  Install page:
  - `"BEER_MENU_[A-Z_]+"` string literals in `api/cmd/beer-menu-api/*.go` (non-test),
  - `${NAME` references in `docker-compose.yaml`,
  - minus `BEER_MENU_DATA_DIR` and `BEER_MENU_PORT` (container-internal; spec assumption).
- **Rationale**: a new setting in the API or compose file fails the guide check until the
  guide documents it.
- **CI**: the guide check must now run when those files change. `.github/ci/changed-sides.sh`:
  `docker-compose.yaml` adds `guide=true`; a new rule `api/cmd/beer-menu-api/*` sets
  `api=true guide=true`. `changed-sides.test.sh` gets matching cases. `pages.yml` adds
  `docker-compose.yaml` to its paths, so a compose change republishes the derived file.
- **Alternatives considered**: a hard-coded list in the test: drifts silently.

## R5. How users keep PUID and PGID

- **Decision**: the steps write them to a `.env` file next to the compose file
  (`(set -C && printf 'PUID=%s\nPGID=%s\n' "$(id -u)" "$(id -g)" > .env)`, then `chmod 600 .env`)
  instead of `export`. `set -C` leaves an existing `.env` unchanged. The file is made private from the start because the variables section may later
  put the first admin password in it.
- **Rationale**: Compose reads `.env` on every run. With `export`, a later `docker compose up`
  from a new terminal falls back to 65532, which cannot write a data folder the user created.
  The same file is where the environment section says to put the other compose-level settings.

## R6. Shape of the environment variables section

- **Decision**: `<h2>Environment variables</h2>` after the basic steps, then:
  - `<h3>How to set a variable</h3>`: two kinds, each with a short copyable example:
    - compose settings (`PUID`, `PGID`, `BEER_MENU_DATA`, `BEER_MENU_WEB_PORT`,
      `BEER_MENU_VERSION`): a line in `.env`;
    - app settings (all other `BEER_MENU_*`): an `environment:` block under `api:` in
      `docker-compose.yaml`, with secret values taken from `.env` as `${NAME}`.
    - then `docker compose up -d` to apply.
  - one `<h3>` per topic: Run as your user (`PUID`, `PGID`), Data folder, Web port, Version,
    First admin, Plain HTTP on your network (`BEER_MENU_COOKIE_SECURE`), Encryption key
    (`BEER_MENU_SECRET_KEY_FILE` / `BEER_MENU_SECRET_KEY`, with `secrets:` example, `openssl`
    command made private by `umask 077` and kept by `set -C`, keep a copy), Passkeys (`BEER_MENU_PUBLIC_URL`, `https://` or
    `http://localhost` only).
  - each entry states purpose, default, optional, and how to set it; secret entries use the
    existing `<div class="warning">`.
- **Rationale**: headings and paragraphs fit phone width; a 3-column table would scroll
  sideways (FR-010). Facts come from README.md, the root compose comments and `startup.go`.

## R7. GitHub link on Home

- **Decision**: a sentence right after the intro paragraph of `index.html`:
  "The source code, releases and issue tracker are on the
  `<a href="https://github.com/helmnet/beer-menu">Beer Menu GitHub repository</a>`."
- **Rationale**: above the first figure, so visible without scrolling (SC-004); link text names
  its target (017 FR-019 test).
