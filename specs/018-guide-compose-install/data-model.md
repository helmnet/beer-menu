# Data Model: Install From a Compose File, Without Cloning

No stored data. The "entities" are build inputs and page content.

## Guide compose file (derived, not stored)

- **Source**: root `docker-compose.yaml`.
- **Derivation** (`build.mjs`): remove lines whose first non-space character is `#`; remove
  `build:` lines; collapse runs of blank lines to one; trim.
- **Rules** (tested): contains `services:`, `api:` and `web:`, an `image: ghcr.io/helmnet/…`
  line for each, no `#`, no `build:`.
- **Used by**: `{{compose}}` in `guide/src/pages/install.html`, HTML-escaped.

## Environment variable entry (page content)

| Field | Rule |
|-------|------|
| Name(s) | Exact variable name in `<code>`; every name from R4's source scan appears |
| Kind | compose setting (`.env`) or app setting (`environment:` under `api:`) |
| Purpose | One or two plain sentences |
| Default | Stated explicitly, or "not set" with what that means |
| Secret | If yes: a `warning` block; never a value in `docker-compose.yaml` |

The variables and defaults are the FR-006 table in spec.md.

## Copy block (page content)

- Markup: `<pre class="copy"><code>…</code></pre>`.
- `copy.js` wraps each block in `<div class="copy-block">` and adds
  `<button type="button" class="copy-button">Copy</button>` inside it, after the `pre`, shown in
  the top-right corner (on hover or focus where hover exists; always on touch screens).
- States: `Copy` → `Copied` (2 s, then back) or `Copy failed` (text selected).
