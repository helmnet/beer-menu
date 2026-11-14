# Contract: Built Guide Site

`node guide/build.mjs` writes `guide/_site/`. `guide/build.test.mjs` runs the build and checks
every rule below. The Pages workflow publishes `guide/_site/` as it is.

## Build

- Inputs: `guide/src/layout.html`, `guide/src/guide.css`, `guide/src/pages/*.html`,
  `guide/src/images/*`, `web/src/pages/AdminPage.css`.
- Output: one `<slug>.html` per nav entry (home page `index.html`), `guide.css`, and `images/`.
- The build exits non-zero, naming the problem, when:
  - a nav entry has no fragment, or a fragment has no nav entry;
  - the `.admin` or `.admin.theme-dark` block, or a required token, is missing from
    `AdminPage.css`.

## Every page

- `<html lang="en">`, `<meta charset>`, `<meta name="viewport">`, a `<title>` ending in
  " – Beer Menu User Guide".
- `:root` declares the tokens `--bg --surface --side --text --muted --border --accent --accent-tint --danger --radius`, with the dark values from `AdminPage.css`.
- Exactly one `<h1>`. Headings do not skip a level.
- The nav lists every page, in data-model order, under its group heading. Exactly one link has
  `aria-current="page"`, and it is the page itself.
- Every local `href` and `src` resolves to a file in `_site/`. External links use `https:`.
- Every `<img>` has a non-empty `alt`.
- No link text is "click here", "here" or "this page" (FR-019).
- No `<script>` elements and no inline `on*=` attributes.

## Admin and Getting started pages

- At least one `<figure>` with an `<img>` from `images/`.
- At least one `<h2>` followed by an `<ol>`.

## Rules checked by hand, not by the test

- Wording matches the app (FR-011).
- Screenshots show only seed data (FR-008).
- The layout has no sideways scroll at 320 px (SC-004).
