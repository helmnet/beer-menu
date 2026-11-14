# Contract: Built Guide Site (changes from 017)

Extends [specs/017-user-guide/contracts/guide-site.md](../../017-user-guide/contracts/guide-site.md).
Every 017 rule stands except where replaced below. `guide/build.test.mjs` checks every rule here.

## Build

- New inputs: root `docker-compose.yaml`, `guide/src/copy.js`.
- Output adds `copy.js`.
- Fragments may contain `{{compose}}`; the build replaces it with the derived compose file
  (data-model.md), HTML-escaped.

## Every page (replaces the 017 "no script" rule)

- No inline script: the only `<script>` allowed is exactly `<script src="copy.js" defer></script>`.
- No inline `on*=` attributes (unchanged), also when split by `/` or a closing quote.
- No `<base>`, `<iframe>`, `<frame>`, `<object>`, `<embed>` or `srcdoc`.
- Every `href` and `src`, in any quoting, is `https:`, `mailto:`, `#…` or a built file.

## Install with Docker page

- Has a `<pre class="copy"><code>` (any of them) whose text (after unescaping) equals the derived compose file.
- Loads `copy.js`.
- Every `<pre>` is `<pre class="copy">`.
- Contains no `git clone`, no "Download ZIP" and no "beer-menu-main".
- Has an `<h2>` named "Environment variables" after the basic steps' `<ol>`.
- Names every variable from the source scan (research R4) inside `<code>`.

## Home page

- Has a link to `https://github.com/helmnet/beer-menu`.

## copy.js

- Wraps each `pre.copy` in a `div.copy-block` and adds one `button[type=button]` labeled
  "Copy" inside that wrapper, after the `pre`. The button sits in the box's top-right corner;
  where the device can hover it shows on hover or keyboard focus, on touch screens always.
- Click writes the block's code text to the clipboard, exactly.
- On success the label is "Copied", then returns to "Copy". On failure the label is
  "Copy failed" and the code text is selected.

## Checked by hand

- The copied compose file starts Beer Menu with no edits (quickstart.md).
- No sideways page scroll at 320 px on the Install page.
