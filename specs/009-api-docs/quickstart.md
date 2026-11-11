# Quickstart: API Docs (feature 9)

Validation guide. Contract: [contracts/openapi-docs.yaml](contracts/openapi-docs.yaml).
Decisions: [research.md](research.md).

## 1. Automated tests

```sh
cd api && go test ./...
cd web && npx vitest run --maxWorkers=2
```

Expect: contract test passes with the three new operations; `TestPublicRoutes` lists them; the
docs handler tests (below) pass; the TokensPanel test finds the link.

API tests cover (real handlers via `NewMux`, no credential):

- `GET /api/docs` 200 `text/html`, docs CSP header, references only `/api/docs/...` assets.
- Every allowlisted `/api/docs/{file}` 200 with its type; unknown names (`x.js`, `index.html`,
  `..%2fgo.mod`, `%2e%2e`) 404; `/api/docs/` 404.
- `GET /api/openapi.yaml` 200 `application/yaml`, body equals `api.OpenAPI`.
- Same responses with a session, a valid token and an invalid bearer header (FR-003).
- `POST /api/docs` and `POST /api/openapi.yaml` 405; HEAD 200.
- Every other route still sends `default-src 'none'; frame-ancestors 'none'`.

## 2. Vendored files match upstream

```sh
cd api/internal/httpapi/docs/swagger-ui && ./fetch.sh && git status --porcelain .
```

Expect: the sha512 check passes and `git status` shows no change.

## 3. Browser check (headless Firefox, no internet)

Build and run the local stack (see project memory "Local test stack"). Open
`http://localhost:<port>/api/docs` in a fresh profile with no session, wait for the operation
list, then check:

- Every tag and operation from the contract is listed; `x-access` shows on an expanded operation.
- No "Try it out", no "Authorize", no lock buttons.
- Console: 0 CSP violations, 0 errors. Network: 0 requests to another origin.
- Repeat with the host's network cut (or the container on an internal network): page renders.
- `/api/docs?url=https://example.com/x.yaml` still shows this API.

If a CSP violation appears, apply research R6's fallback and re-run.

## 4. Admin link

Sign in, open API Tokens, follow "API docs": the docs open in a new tab, the admin tab stays on
API Tokens, still signed in. Screenshot at laptop and phone widths with the existing visual gate.
