# Research: API Docs (feature 9)

All Technical Context unknowns are resolved below. Each item: decision, rationale, alternatives.

## R1. Docs viewer and version

- **Decision**: Swagger UI from the npm package `swagger-ui-dist` **5.34.0** (Apache-2.0,
  released 2026-10-19). npm integrity of the tarball:
  `sha512-H6tAIjr3XqE31PbkZl0OFya1ziRLDfTwh3J6OHdumPtPCLfjTnyLeagl/55kFNZcTOz9a9QI9WP0FXldwiGJag==`.
- **Rationale**: the owner chose Swagger UI. 5.34.1 (2026-10-30) is only 12 days old on
  2026-11-11, under the project's two-week release-age rule; 5.34.0 is the newest release that
  passes. `swagger-ui-dist` is the prebuilt static bundle, so the API needs no JS build step.
- **Alternatives**: 5.34.1 (too new); Redoc (read-only by design, but not the owner's choice and
  needs the same vendoring); the `swagger-ui` npm package (needs a bundler).

## R2. How the files reach the API image

- **Decision**: vendor only the files the page needs into
  `api/internal/httpapi/docs/swagger-ui/`: `swagger-ui-bundle.js` (1.68 MB),
  `swagger-ui.css` (188 KB), `LICENSE`, `NOTICE`, `swagger-ui-bundle.js.LICENSE.txt`. Embed
  them with `//go:embed`, next to our own `index.html` and `init.js`. A short
  `docs/swagger-ui/fetch.sh` downloads the pinned tarball, checks the npm sha512 above, and copies
  exactly those files, so a reviewer can reproduce the vendored bytes and an upgrade is one
  version bump. Files are copied byte-identical (no edits), so the script's output diffs clean.
- **Rationale**: `go:embed` needs the files at compile time, and CI runs `go test` without
  Docker, so they must be in the repo. Vendoring keeps the image offline-capable (FR-006) and the
  binary single and static (principle II). Binary grows by about 1.9 MB.
- **Alternatives**: download in the Dockerfile (CI tests and local `go build` would fail without
  the files); `github.com/swaggest/swgui` Go module (Dependabot could track it, but it lags
  upstream, brings extra Go dependencies and serves its own HTML with an inline script that the
  CSP forbids); serve from the web container (owner chose the API).
- **Trade-off recorded**: Dependabot cannot see a vendored bundle. Upgrades are manual via
  `fetch.sh`. Not a standalone security fix path: Swagger UI here renders only our own spec.

## R3. Page and asset routes

- **Decision**: three public `GET` routes in `Routes`:
  - `GET /api/docs`: our `index.html`.
  - `GET /api/docs/{file}`: one embedded asset by exact name from a fixed allowlist (the files
    in R2 plus `init.js`); anything else 404. `{file}` never matches an empty segment, so
    `/api/docs/` gets the mux's own 404 (checked on Go 1.27); no redirect route is added.
  - `GET /api/openapi.yaml`: `api.OpenAPI` bytes as `application/yaml`.

  The page references assets by absolute path (`/api/docs/swagger-ui.css`), so it works through
  any proxy hostname. The mux unescapes the path before matching: `/api/docs/..%2fgo.mod` reaches
  the handler as `file="../go.mod"`, and the allowlist lookup turns it into 404. Go's `ServeMux` answers HEAD for
  GET patterns and 405 with `Allow` for other methods (FR-008), as for every other route.
- **Rationale**: one route table stays the single source (contract test, `TestPublicRoutes`). A
  fixed allowlist cannot serve other embedded or disk files (spec edge case "never another
  file"). `http.FileServerFS` was rejected: it lists directories and redirects `index.html`.
- **Alternatives**: a subtree pattern `/api/docs/` outside `Routes` (escapes the contract test).

## R4. Content types

- **Decision**: set types explicitly: `text/html; charset=utf-8`, `text/javascript;
  charset=utf-8`, `text/css; charset=utf-8`, `text/plain; charset=utf-8` (license files),
  `application/yaml` (RFC 9512).
- **Rationale**: the API image is `scratch`, with no `/etc/mime.types`, and every response has
  `nosniff`, so a wrong type would block the script or stylesheet.

## R5. Read-only Swagger UI (FR-004, FR-005)

- **Decision**: our `init.js` (no inline script) calls `SwaggerUIBundle` with:
  - `url: "/api/openapi.yaml"`, `dom_id: "#swagger-ui"`, `layout: "BaseLayout"` (no top bar
    with a URL box, so no standalone preset file is needed).
  - `supportedSubmitMethods: []`: removes "Try it out" on every operation.
  - a plugin that wraps `authorizeBtn` and `authorizeOperationBtn` to render nothing: removes
    the Authorize button and the per-operation lock buttons that open the credential dialog.
  - `showExtensions: true`: shows each operation's `x-access` (public, read, write, admin).
  - `queryConfigEnabled: false` (explicit, though the default): `?url=` / `?configUrl=` cannot
    load another document.
  - `validatorUrl: null`: no request to the online validator.
  - `deepLinking: true`, `docExpansion: "list"`.
- **Rationale**: these are Swagger UI's documented options; nothing is patched in the bundle.
- **Check**: the quickstart confirms no "Try it out", no Authorize and no lock buttons on screen.

## R6. Content security policy for the docs page (FR-007)

- **Decision**: only the `/api/docs` page and its assets get
  `default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:;
  connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`, set by the
  docs handler (it runs after `withSecurity`, so `Header().Set` replaces the API default).
  Every other response keeps `default-src 'none'; frame-ancestors 'none'`. All other
  `withSecurity` headers (nosniff, `X-Frame-Options: DENY`, no-referrer, no-store) stay.
- **Rationale**: Swagger UI's CSS uses `data:` SVG images; React sets inline styles through the
  CSSOM, which CSP does not block. The bundle's one `new Function("return this")` sits behind
  `this||`, so it does not run in a browser.
- **Fallback** (only if the quickstart's console check shows violations): add
  `'unsafe-inline'` to `style-src` alone and record why. Never `'unsafe-eval'` or inline script.

## R7. Admin link (FR-010)

- **Decision**: `TokensPanel` gets a plain link, "API docs", near the token list:
  `<a href="/api/docs" target="_blank" rel="noopener noreferrer">`. Styled with existing
  `AdminPage.css` rules (neutral text color: blue is accent only, `AdminStyle.test.tsx`).
- **Rationale**: smallest change; the admin and docs share an origin, so a relative URL works in
  dev (Vite proxies `/api`) and production.

## R8. Secrets scanning of the vendored bundle

- **Finding**: `gitleaks detect --no-git` with the repo config reports no leaks in the vendored
  files; its default allowlist skips `swagger-ui-bundle.js` itself (0 bytes scanned). The bundle
  is a published upstream artifact verified by its npm sha512, so this is acceptable.

## R9. Contract and version

- **Decision**: document the three routes in `api/openapi.yaml` under a new `docs` tag, each with
  `security: []` and `x-access: public`, and bump `info.version` (0.7.0 to 0.8.0). Responses:
  `/api/docs` 200 `text/html`; `/api/docs/{file}` 200 (`text/javascript`, `text/css`,
  `text/plain`), 404; `/api/openapi.yaml` 200 `application/yaml`.
- **Rationale**: constitution I (every endpoint change updates the contract) and the existing
  two-way contract test.
