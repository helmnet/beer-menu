---

description: "Task list for 018: install from a compose file, without cloning"
---

# Tasks: Install From a Compose File, Without Cloning

**Input**: Design documents from `specs/018-guide-compose-install/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/guide-site.md, quickstart.md

**Tests**: Required by the constitution (IV). Each story writes its tests in `guide/build.test.mjs` first and sees them fail before the implementation.

**Organization**: Tasks are grouped by user story. Most tasks in a story touch different files, but every story adds tests to `guide/build.test.mjs`, so test tasks across stories run one after another.

**Run tests with**: `node --test guide/` (Node standard library only; no install). CI rules: `sh .github/ci/changed-sides.test.sh`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

No setup: the guide build has no dependencies to install and every folder exists.

---

## Phase 2: Foundational (CI wiring)

**Purpose**: The new guide tests read `docker-compose.yaml` and `api/cmd/beer-menu-api/*.go`, so the guide check and publishing must run when those change (research R4).

- [X] T001 In `.github/ci/changed-sides.test.sh`, add `check` cases (same style as the existing "guide build" case at about line 53): `docker-compose.yaml` → api=true web=true guide=true services="api web" notices=false; `api/cmd/beer-menu-api/main.go` → api=true web=false guide=true services="api" notices=false; `api/cmd/beer-menu-api/startup_test.go` → same as main.go; `api/internal/store/store.go` → api=true guide=false (unchanged rule). Run `sh .github/ci/changed-sides.test.sh` and see the new cases fail.
- [X] T002 In `.github/ci/changed-sides.sh`: change the `ci/images/* | docker-compose.yaml)` rule so `docker-compose.yaml` also sets `guide=true` (split it into its own case `docker-compose.yaml) api=true web=true guide=true ;;` with a one-line why comment: "The guide derives its Install page compose file from it."); add `api/cmd/beer-menu-api/*) api=true guide=true ;;` above the `api/*)` rule with a comment: "The guide test checks every variable the API reads is documented." Update the header comment's description of `guide=true`. Run `sh .github/ci/changed-sides.test.sh`: all pass.
- [X] T003 [P] In `.github/workflows/pages.yml`, add `"docker-compose.yaml"` to `on.push.paths`, and extend the header comment ("…changes it, its color source (web/src/pages/AdminPage.css) or the compose file it derives the Install page from…").

**Checkpoint**: CI runs the guide check for compose and API startup changes.

---

## Phase 3: User Story 1 - Start Beer Menu from a copied compose file (Priority: P1) 🎯 MVP

**Goal**: The Install page shows a copyable, image-only compose file derived from the root `docker-compose.yaml`, and steps that need no clone or download.

**Independent Test**: quickstart.md steps 1–4, 6, 7 on a machine with Docker and no checkout.

### Tests for User Story 1 (write first, see them fail)

- [X] T004 [US1] In `guide/build.test.mjs`, import `deriveCompose` from `./build.mjs` and add test "derived compose file is image-only": `const yaml = deriveCompose(readFileSync(join(root, "docker-compose.yaml"), "utf8"))`; assert it matches `/^services:$/m`, `/^  api:$/m`, `/^  web:$/m`, `/image: ghcr\.io\/helmnet\/beer-menu-api:/`, `/image: ghcr\.io\/helmnet\/beer-menu-web:/`; does not match `/#/` nor `/^\s*build:/m` nor `/\n\n\n/`; and has no leading or trailing whitespace. Also a unit case: `deriveCompose("# c\nservices:\n  a:\n    build: ./a\n    # x\n    image: i\n\n\n\n  b:\n    image: j\n# end\n")` equals `"services:\n  a:\n    image: i\n\n  b:\n    image: j"`.
- [X] T005 [US1] In `guide/build.test.mjs`, replace test "no script and no inline event handlers" with: every page's `<script` tags, collected with `/<script\b[^>]*>[\s\S]*?<\/script>/gi`, each equal exactly `<script src="copy.js" defer></script>`, and no `<script` remains after removing them; inline `on*=` check unchanged. Add test "install page shows the derived compose file and no clone steps": the `install.html` page main has `/<pre class="copy"><code>([\s\S]*?)<\/code><\/pre>/`; one of its matches, unescaped (`&lt;`→`<`, `&gt;`→`>`, `&quot;`→`"`, `&amp;`→`&`, in that order with `&amp;` last), equals `deriveCompose(...)` of the root file; the page contains `<script src="copy.js" defer></script>`; and does not match `/git clone/i`, `/Download ZIP/i`, `/beer-menu-main/`.
- [X] T006 [US1] In `guide/build.test.mjs`, add test "copy.js adds a Copy button that copies the code text" using `node:vm` (`import vm from "node:vm"`) and plain-object fakes, no new dependency: a fake `code` `{ textContent: "services:\n  api:" }`; a fake `pre` with `querySelector: () => code` and `before(el)` that records the inserted wrapper; a fake `document` with `querySelectorAll: (s) => (s === "pre.copy" ? [pre] : [])` and `createElement: (tag) => ({ tag, children: [], listeners: {}, append(...els) { this.children.push(...els); }, addEventListener(t, f) { this.listeners[t] = f; } })`; `getSelection: () => ({ selectAllChildren: (n) => (selected = n) })`; `setTimeout` that records the callback. Run `readFileSync(join(root, "guide", "src", "copy.js"), "utf8")` in `vm.createContext({ document, navigator, getSelection, setTimeout })`. Assert one `div` wrapper with `className === "copy-block"` was inserted, holding the `pre` then one button with `type === "button"`, `className === "copy-button"`, `textContent === "Copy"`. Case 1: `navigator.clipboard.writeText` resolves and records its argument; after `await button.listeners.click()` (have the handler return its promise) the argument equals `code.textContent`, the label is `"Copied"`, and running the recorded timeout callback sets it back to `"Copy"`. Case 2: a fresh context whose `writeText` rejects: label is `"Copy failed"` and `selected === code`. Case 3: `navigator = {}` (no clipboard API, plain HTTP): same result as case 2. Also assert `copy.js` is in the built `_site`.

### Implementation for User Story 1

- [X] T007 [US1] In `guide/build.mjs`: add exported `deriveCompose(text)` with a doc comment (data-model.md "Derivation": "remove lines whose first non-space character is `#`; remove `build:` lines; collapse runs of blank lines to one; trim"); in `build()`, read `join(root, "docker-compose.yaml")` once and fill each fragment's content with `.replace("{{compose}}", () => escape(deriveCompose(compose)))` (a function, so `$&`, `` $` ``, `$'` or `$n` in the compose text are never read as replacement patterns) before the layout fill; copy `join(src, "copy.js")` to `join(out, "copy.js")` next to `guide.css`. Update the file's header comment: inputs gain the root `docker-compose.yaml` and `guide/src/copy.js`, output gains `copy.js`. Keep the existing `escape` function (it already handles `& < > "`).
- [X] T008 [P] [US1] Create `guide/src/copy.js`: a classic script (no `import`/`export`, works from `file://`), with a top comment stating its contract (contracts/guide-site.md "copy.js"). For each `document.querySelectorAll("pre.copy")`: create a `button`, set `type = "button"`, `className = "copy-button"`, `textContent = "Copy"`; create a `div` with `className = "copy-block"`, insert it with `pre.before(wrapper)`, then `wrapper.append(pre, button)` (the button sits outside the scrolling `pre`); on `click`, return `Promise.resolve().then(() => navigator.clipboard.writeText(code.textContent))` (so a missing `navigator.clipboard` also rejects), where `code = pre.querySelector("code")`; on success set `"Copied"` and `setTimeout(() => (button.textContent = "Copy"), 2000)`; on failure set `"Copy failed"` and `getSelection().selectAllChildren(code)` so the user can copy by hand. Wrap the whole script in a block or IIFE so it adds no globals. No inline styles.
- [X] T009 [P] [US1] In `guide/src/guide.css`, add styles using tokens only (no color literals; the existing test fails on one): `.copy-block { position: relative }`; `.copy-block pre { padding-right: 72px }`; `.copy-button` absolute at `top: 6px; right: 6px`, `padding: 2px 8px`, `font-size: 13px`, `color: var(--text)`, `font-family: inherit`, `cursor: pointer`, with the surface, border and radius from the shared boxed rule (`summary, figure, .warning, pre, .copy-button`); `:hover` joins the `nav a:hover` rule (`background: var(--accent-tint)`); inside `@media (hover: hover)` the button has `opacity: 0` and `.copy-block:hover`/`.copy-block:focus-within` show it; add `.copy-button:focus-visible` to the existing `a:focus-visible, summary:focus-visible` outline rule.
- [X] T010 [US1] Rewrite the "Start Beer Menu for the first time" part of `guide/src/pages/install.html` (keep the intro paragraph, but say users need no copy of the project). Every example is `<pre class="copy"><code>` so each has a Copy button. Notation in this task and T012: `\n` inside a `<pre class="copy"><code>` example means a real line break in the HTML, except inside the `printf` format string `'PUID=%s\nPGID=%s\n'`, where the backslash-n characters are typed literally. Keep one `<h2>Start Beer Menu for the first time</h2>` and one `<ol>`: (1) make a folder for Beer Menu and its data folder, and open a terminal there: `<pre class="copy"><code>mkdir -p ~/beer-menu/data\ncd ~/beer-menu</code></pre>`; (2) copy the compose file with the **Copy** button and save it in that folder as `docker-compose.yaml`, followed by `<pre class="copy"><code>{{compose}}</code></pre>` and `<script src="copy.js" defer></script>`; (3) tell Beer Menu to run as your own user so it can write the data folder (research R5; it refuses to run as root): `<pre class="copy"><code>(set -C &amp;&amp; printf 'PUID=%s\nPGID=%s\n' "$(id -u)" "$(id -g)" &gt; .env)\nchmod 600 .env</code></pre>` and say the `.env` file keeps this for every later start, an existing `.env` is left unchanged, and it is readable only by you, because it may later hold a password; (4) `docker compose up -d`, which downloads the images itself (no separate `docker compose pull` on first start); (5) admin app at **http://localhost:8080/**; (6) TV display at **http://localhost:8080/display**. Keep the closing paragraphs (runs in background, keep the "data" folder safe, link to Getting started) and add one sentence pointing to the Environment variables section below for changing the port or other settings. No `git clone`, "Download ZIP", "beer-menu-main" or project page link.

**Checkpoint**: `node --test guide/` passes; quickstart.md steps 1–4, 6, 7 work.

---

## Phase 4: User Story 2 - Look up an environment variable (Priority: P2)

**Goal**: An Environment variables section after the basic steps documents every user-settable variable (spec FR-006 to FR-008).

**Independent Test**: quickstart.md step 5; the coverage test passes.

### Tests for User Story 2 (write first, see them fail)

- [X] T011 [US2] In `guide/build.test.mjs`, add test "install page documents every variable the API and compose file read": collect names from `"(BEER_MENU_[A-Z_]+)"` string literals in every `api/cmd/beer-menu-api/*.go` file not ending in `_test.go`, plus `/\$\{([A-Z_]+)/g` in the root `docker-compose.yaml`; drop `BEER_MENU_DATA_DIR` and `BEER_MENU_PORT` with a comment saying why ("internal to the containers; changing them breaks the data mount or the web proxy"); assert the set is not empty and each name appears in `install.html` main as `<code>NAME</code>` (allow it inside a longer `<code>` too: match `new RegExp("<code>[^<]*\\b" + name + "\\b")`). Also assert the install main has `<h2>Environment variables</h2>` and that it comes after the first `<ol>` and after the compose file's `<pre class="copy">`.

### Implementation for User Story 2

- [X] T012 [US2] Append to `guide/src/pages/install.html` an `<h2>Environment variables</h2>` section (research R6), using `<h3>` subheadings, `<p>`, `<pre class="copy"><code>` examples (every example gets the Copy button), and `<div class="warning"><p>…</p></div>` for secrets. Use facts only from README.md, the comments in the root `docker-compose.yaml` and `api/cmd/beer-menu-api/startup.go`. Content:
  - Intro: Beer Menu runs without any of these; each is optional.
  - `<h3>How to set a variable</h3>`: two kinds. Compose settings (`PUID`, `PGID`, `BEER_MENU_DATA`, `BEER_MENU_WEB_PORT`, `BEER_MENU_VERSION`): one `NAME=value` line each in the `.env` file next to `docker-compose.yaml`. App settings (every other `BEER_MENU_` name): add an `environment:` block under `api:` in `docker-compose.yaml`, e.g. `<pre class="copy"><code>  api:\n    environment:\n      BEER_MENU_COOKIE_SECURE: "false"</code></pre>`; secret values go in `.env` and are referenced as `${NAME}`. A variable set in the shell (`export NAME=value`) overrides the same name in `.env`, but lasts only for that terminal, so `.env` is the place to keep settings. After any change run `docker compose up -d`; Compose restarts what changed. Run every `docker compose` command from the Beer Menu folder (`cd ~/beer-menu`), where `docker-compose.yaml` and `.env` are.
  - `<h3>Run as your user: PUID and PGID</h3>`: user and group IDs Beer Menu runs as; must own the data folder; never 0 (it refuses to start as root); default 65532; the install steps already set them to yours.
  - `<h3>Data folder: BEER_MENU_DATA</h3>`: host folder for the database and its backups; default `./data`; must exist and be owned by PUID:PGID.
  - `<h3>Web port: BEER_MENU_WEB_PORT</h3>`: port for the admin app and TV display; default 8080; use another if 8080 is taken; the addresses above change with it.
  - `<h3>Version: BEER_MENU_VERSION</h3>`: which release to run, e.g. `1.0.0`; default `latest`; pinning a version means updates happen only when you change it; update with `docker compose pull` then `docker compose up -d`.
  - `<h3>First admin: BEER_MENU_ADMIN_USERNAME and BEER_MENU_ADMIN_PASSWORD</h3>`: creates the first admin at startup only while no admin exists; set both or neither; password 12 to 256 characters; not set means you create the first admin in the browser (link Getting started). Example: `.env` line `BEER_MENU_ADMIN_PASSWORD='CHANGE-ME'` (too short on purpose, so an unedited paste stops startup; tell users to replace it and keep the single quotes) and `environment:` entries `BEER_MENU_ADMIN_USERNAME: owner` and `BEER_MENU_ADMIN_PASSWORD: ${BEER_MENU_ADMIN_PASSWORD}`. Warning: never write the password in `docker-compose.yaml`; keep `.env` readable only by you (`chmod 600 .env`, as the install steps did); remove both after the first start.
  - `<h3>Plain HTTP on your network: BEER_MENU_COOKIE_SECURE</h3>`: default `true`; sign-in cookies then need HTTPS or `localhost`; set `"false"` only to open the admin app over plain HTTP by LAN address.
  - `<h3>Encryption key: BEER_MENU_SECRET_KEY_FILE or BEER_MENU_SECRET_KEY</h3>`: encrypts the stored Brewfather API key and authenticator-app secrets; not set means Brewfather import and authenticator apps are off. Steps: `<pre class="copy"><code>(umask 077 &amp;&amp; set -C &amp;&amp; openssl rand -base64 32 &gt; beer-menu-secret-key)</code></pre>` (private from creation; `set -C` keeps an existing key; file owned by PUID:PGID), then a `class="copy"` example adding to `api:` `environment: BEER_MENU_SECRET_KEY_FILE: /run/secrets/beer_menu_secret_key` and `secrets: - beer_menu_secret_key`, plus a top-level `secrets: beer_menu_secret_key: file: ./beer-menu-secret-key`. `BEER_MENU_SECRET_KEY` holds the key itself instead; the file wins if both are set; the file is safer because environment values show in `docker inspect`. Warning: keep a copy of the key outside the data folder; losing or changing it disconnects Brewfather (enter the API key again) and stops every authenticator app's codes (those admins sign in with a passkey or a recovery code and add the authenticator again); a restored backup needs the same key.
  - `<h3>Passkeys: BEER_MENU_PUBLIC_URL</h3>`: the address admins open the app at, exactly as the browser shows it (scheme, host name and port if any); needed for passkeys; not set means passkeys are off; must be `https://<host name>[:port]` or `http://localhost[:port]`, with no path; a bad value stops Beer Menu at startup.
- [X] T013 [P] [US2] In `guide/src/guide.css`, add `main h3 { margin: 24px 0 6px; font-size: 1.1rem; line-height: 1.25; }` after the `main h2` rule.

**Checkpoint**: `node --test guide/` passes; quickstart.md step 5 works.

---

## Phase 5: User Story 3 - Find the project from the Home page (Priority: P3)

**Goal**: Home page links the GitHub repository (FR-009).

**Independent Test**: quickstart.md step 8.

- [X] T014 [US3] In `guide/build.test.mjs`, add test "home page links the GitHub repository": `index.html` main contains `<a href="https://github.com/helmnet/beer-menu">` exactly once (spec SC-004 "exactly one way"), and that link appears before the first `<figure>`. See it fail.
- [X] T015 [US3] In `guide/src/pages/index.html`, after the first paragraph, add `<p>The source code, releases and issue tracker are in the <a href="https://github.com/helmnet/beer-menu">Beer Menu GitHub repository</a>.</p>` (research R7).

**Checkpoint**: `node --test guide/` passes.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T016 [P] Update `guide/README.md`: Layout list gains `src/copy.js` (Copy button for `<pre class="copy">`, the only script; loaded by a fragment with `<script src="copy.js" defer></script>`) and notes that `build.mjs` fills `{{compose}}` from the root `docker-compose.yaml` (comments and `build:` lines stripped); `build.test.mjs` line points to both contracts (017 and 018); "Write for end users" note: the Install page also documents environment variables, and the test fails until every variable the API startup code or compose file reads is listed there.
- [X] T017 [P] Update the header comment of `guide/build.test.mjs` to reference `specs/017-user-guide/contracts/guide-site.md` and `specs/018-guide-compose-install/contracts/guide-site.md`.
- [X] T018 Confirm no other file still points users to cloning for the guide flow: `grep -rn -i "clone\|beer-menu-main\|Download ZIP" guide/src` returns nothing.
- [X] T019 Run `node --test guide/` and `sh .github/ci/changed-sides.test.sh` (both must pass), then walk through `specs/018-guide-compose-install/quickstart.md` by hand and record any step that fails.

---

## Dependencies & Execution Order

- Phase 2 (T001–T003) before the stories: the new tests read files whose changes CI must route to the guide check.
- US1 (T004–T010) first: T007 (`deriveCompose`, `{{compose}}`) is needed by T005's test and T011's placement check.
- US2 (T011–T013) depends on US1's `install.html` (it appends to that page).
- US3 (T014–T015) independent of US1/US2 except that test tasks share `guide/build.test.mjs`.
- Polish after all stories.

Within each story: tests first (fail), then implementation, then `node --test guide/`.

## Parallel Opportunities

- T003 with T001–T002 (different files).
- US1: T008 (`copy.js`) and T009 (`guide.css`) in parallel with T007 (`build.mjs`); T010 after T007.
- US2: T013 (`guide.css`) in parallel with T012 (`install.html`).
- US3: T015 can be written while US2 is in progress (different file); its test T014 waits for the `build.test.mjs` edits before it.
- Polish: T016 and T017 in parallel.

```text
# US1 after tests T004–T006:
T007 guide/build.mjs   |  T008 guide/src/copy.js  |  T009 guide/src/guide.css
then T010 guide/src/pages/install.html
```

## Implementation Strategy

1. Phase 2, then US1 → run tests and quickstart steps 1–4, 6, 7. This is the MVP: users install without cloning.
2. US2 → environment variables section.
3. US3 → Home link.
4. Polish and full quickstart.
