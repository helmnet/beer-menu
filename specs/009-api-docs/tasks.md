---

description: "Task list for feature 9: API docs"
---

# Tasks: API Docs

**Input**: Design documents from `specs/009-api-docs/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/openapi-docs.yaml](./contracts/openapi-docs.yaml),
[quickstart.md](./quickstart.md)

**Tests**: Required. Constitution IV makes tests non-negotiable. Go handler tests go through
`NewMux` with `httptest` (helpers in `api/internal/httpapi/helpers_test.go`); no database change,
so no migration test. React components with logic get Vitest tests; run Vitest with
`--maxWorkers=2`. Write each test task before its implementation task and see it fail first.

**Organization**: grouped by user story. Serving the contract (`GET /api/openapi.yaml`) is
foundational: the docs page (US1) loads it, and it alone delivers US2's download.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1, US2, US3 from spec.md

## Path Conventions

- API: `api/` (Go module `github.com/helmnet/beer-menu/api`); handlers in `api/internal/httpapi/`
- Web: `web/src/`

---

## Phase 1: Setup

**Purpose**: vendor Swagger UI and record the constitution amendment.

- [x] T001 Amend constitution principle V to list the read-only API docs as public, version 1.2.0 to 1.3.0, Last Amended 2026-11-11, in `.specify/memory/constitution.md` (done during `/speckit-specify`)
- [X] T002 Create `api/internal/httpapi/docs/swagger-ui/fetch.sh` (POSIX sh, `set -eu`, executable): download `https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-5.34.0.tgz` into a fresh `mktemp -d` directory, verify it against npm integrity `sha512-H6tAIjr3XqE31PbkZl0OFya1ziRLDfTwh3J6OHdumPtPCLfjTnyLeagl/55kFNZcTOz9a9QI9WP0FXldwiGJag==` (base64 of `openssl dgst -sha512 -binary`), exit non-zero on mismatch, extract, and copy exactly `swagger-ui-bundle.js`, `swagger-ui.css`, `swagger-ui-bundle.js.LICENSE.txt`, `LICENSE`, `NOTICE` byte-identical into the script's own directory; remove the temp dir on exit (research R1, R2)
- [X] T003 Run `api/internal/httpapi/docs/swagger-ui/fetch.sh` and commit its five output files unchanged
- [X] T004 [P] Write `api/internal/httpapi/docs/swagger-ui/README.md`: upstream package and version (`swagger-ui-dist` 5.34.0, Apache-2.0, released 2026-10-19), the npm sha512, why it is vendored (offline, `go:embed`, CI without Docker), that Dependabot cannot see it, and upgrade steps (bump version and sha512 in `fetch.sh`, run it, check the release is at least two weeks old, re-run quickstart §3)

---

## Phase 2: Foundational (serve the contract)

**Purpose**: `GET /api/openapi.yaml`, needed by the docs page and complete for US2.

- [X] T005 Write failing tests in `api/internal/httpapi/docs_test.go`: with no credential, `GET /api/openapi.yaml` returns 200, `Content-Type: application/yaml`, body equal to `api.OpenAPI`; the same 200 and body with a signed-in session, with a valid `read` token, and with an invalid `Authorization: Bearer bmt_invalid` header (FR-003; public routes skip guard); `HEAD` returns 200; `POST` returns 405; the response keeps `X-Content-Type-Options: nosniff` and the API default CSP `default-src 'none'; frame-ancestors 'none'`
- [X] T006 Add the `/api/openapi.yaml` operation and the `docs` tag from `specs/009-api-docs/contracts/openapi-docs.yaml` to `api/openapi.yaml` (`security: []`, `x-access: public`), and bump `info.version` from 0.7.0 to 0.8.0
- [X] T007 Create `api/internal/httpapi/docs.go` with `getOpenAPI(Deps) http.Handler` writing `api.OpenAPI` with `Content-Type: application/yaml` (set explicitly: the scratch image has no mime table, research R4); doc comment per constitution VI
- [X] T008 Register `{Method: http.MethodGet, Path: "/api/openapi.yaml", Access: AccessPublic, Handler: getOpenAPI}` in `Routes` in `api/internal/httpapi/routes.go` directly after the `GET /api/menu` entry, and update `TestPublicRoutes` in `api/internal/httpapi/routes_test.go`: `want` becomes `GET /api/health,GET /api/setup,POST /api/setup,POST /api/session,GET /api/menu,GET /api/openapi.yaml` and add the API docs to its doc comment's list (constitution v1.3.0)

**Checkpoint**: `cd api && go test ./...` passes; `curl -s localhost:<port>/api/openapi.yaml` returns the contract.

---

## Phase 3: User Story 1 - Browse the API docs without signing in (Priority: P1) 🎯 MVP

**Goal**: read-only Swagger UI at `/api/docs`, all assets same-origin, strict CSP.

**Independent Test**: with no session, open `/api/docs`; every operation is listed with its `x-access`; no "Try it out", Authorize or lock buttons; 0 CSP violations; 0 requests to another origin.

### Tests for User Story 1

- [X] T009 [US1] Add failing tests to `api/internal/httpapi/docs_test.go`:
  - `GET /api/docs` with no credential: 200, `Content-Type: text/html; charset=utf-8`, `Content-Security-Policy` exactly `default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `nosniff`; body references `/api/docs/swagger-ui.css`, `/api/docs/swagger-ui-bundle.js`, `/api/docs/init.js` and contains no `<script>` without `src` and no `http://` or `https://` URL
  - each allowlisted `/api/docs/{file}` returns 200 with the docs CSP and its type: `init.js` and `swagger-ui-bundle.js` `text/javascript; charset=utf-8`, `swagger-ui.css` `text/css; charset=utf-8`, `LICENSE`, `NOTICE`, `swagger-ui-bundle.js.LICENSE.txt` `text/plain; charset=utf-8`; body equals the embedded file
  - `/api/docs/x.js`, `/api/docs/index.html`, `/api/docs/README.md`, `/api/docs/fetch.sh`, `/api/docs/..%2fgo.mod` and `/api/docs/%2e%2e` return 404 (the mux unescapes `%2f`, so the handler sees `../go.mod`; the allowlist must refuse it)
  - `/api/docs/` returns 404 (the mux never matches `{file}` to an empty segment; no redirect)
  - `/api/docs` gives the same response with a session, a `read` token and an invalid bearer header; `POST /api/docs` and `POST /api/docs/init.js` return 405; `HEAD /api/docs` returns 200
  - `GET /api/health` and `GET /api/menu` still send `default-src 'none'; frame-ancestors 'none'` (docs CSP does not leak)
  - `init.js` contains `supportedSubmitMethods: []`, `url: "/api/openapi.yaml"`, `validatorUrl: null`, `queryConfigEnabled: false`, `showExtensions: true` and wraps `authorizeBtn` and `authorizeOperationBtn` (guards read-only config against edits)

### Implementation for User Story 1

- [X] T010 [P] [US1] Create `api/internal/httpapi/docs/index.html`: `<!doctype html>`, `lang="en"`, charset and viewport metas, `<title>Beer Menu API</title>`, `<link rel="stylesheet" href="/api/docs/swagger-ui.css">`, `<div id="swagger-ui"></div>`, then `<script src="/api/docs/swagger-ui-bundle.js"></script>` and `<script src="/api/docs/init.js"></script>`; no inline script or style, no favicon link (research R3, R6)
- [X] T011 [P] [US1] Create `api/internal/httpapi/docs/init.js` (plain ES2017, no inline config): `window.ui = SwaggerUIBundle({ url: "/api/openapi.yaml", dom_id: "#swagger-ui", layout: "BaseLayout", presets: [SwaggerUIBundle.presets.apis], plugins: [readOnly], supportedSubmitMethods: [], showExtensions: true, queryConfigEnabled: false, validatorUrl: null, deepLinking: true, docExpansion: "list" })` where `readOnly` is `() => ({ wrapComponents: { authorizeBtn: () => () => null, authorizeOperationBtn: () => () => null } })`; a header comment says why each read-only option is set (research R5)
- [X] T012 [US1] Add the `/api/docs` and `/api/docs/{file}` operations from `specs/009-api-docs/contracts/openapi-docs.yaml` to `api/openapi.yaml` (`security: []`, `x-access: public`, `file` enum of the six allowlisted names)
- [X] T013 [US1] In `api/internal/httpapi/docs.go`: `//go:embed` the explicit files `docs/index.html docs/init.js docs/swagger-ui/swagger-ui-bundle.js docs/swagger-ui/swagger-ui.css docs/swagger-ui/swagger-ui-bundle.js.LICENSE.txt docs/swagger-ui/LICENSE docs/swagger-ui/NOTICE` (never the directory, so `README.md` and `fetch.sh` are not embedded); a fixed map from served name to embedded path and content type; a `docsCSP` constant with the policy from T009; `getDocs` serves `index.html`; `getDocsAsset` reads `r.PathValue("file")` and looks it up in the allowlist only (never joins it into a path): unknown 404 via `writeError(w, http.StatusNotFound, "not_found", ...)`, known writes bytes with its type; both set `Content-Security-Policy` to `docsCSP` with `Header().Set` (replaces the `withSecurity` default, research R6); doc comments on each
- [X] T014 [US1] Register `GET /api/docs` then `GET /api/docs/{file}` as `AccessPublic` in `Routes` in `api/internal/httpapi/routes.go`, between `GET /api/menu` and `GET /api/openapi.yaml`, and set `want` in `TestPublicRoutes` in `api/internal/httpapi/routes_test.go` to `GET /api/health,GET /api/setup,POST /api/setup,POST /api/session,GET /api/menu,GET /api/docs,GET /api/docs/{file},GET /api/openapi.yaml`

**Checkpoint**: `cd api && go test ./...` passes; quickstart §3 browser check passes (T020).

---

## Phase 4: User Story 2 - Download the raw API contract (Priority: P2)

**Goal**: the served contract is exactly the repo's `api/openapi.yaml` for the release. Served by Phase 2.

**Independent Test**: fetch `/api/openapi.yaml` with no credential and diff it with `api/openapi.yaml`.

- [X] T015 [US2] Add a test to `api/internal/httpapi/docs_test.go` that reads `../../openapi.yaml` from disk with `os.ReadFile` and asserts `GET /api/openapi.yaml` returns those exact bytes (catches a stale or altered embed, FR-002, SC-004)

**Checkpoint**: US2 acceptance scenarios 1 and 2 hold (scenario 2 by construction: the file is embedded at build).

---

## Phase 5: User Story 3 - Find the docs from the API Tokens page (Priority: P3)

**Goal**: an "API docs" link on API Tokens opening `/api/docs` in a new tab.

**Independent Test**: sign in, open API Tokens, follow "API docs": a new tab shows the docs; the admin tab stays on API Tokens, signed in.

- [X] T016 [US3] Add a failing test to `web/src/components/TokensPanel.test.tsx`: after tokens load, `getByRole("link", { name: "API docs" })` has `href="/api/docs"`, `target="_blank"` and `rel` containing both `noopener` and `noreferrer` (FR-010)
- [X] T017 [US3] Add the link to `web/src/components/TokensPanel.tsx` near the token list, inside the existing card (for example under the table as a short line "Calling the API? See the API docs."), using existing `AdminPage.css` classes or a new rule under `.admin` in `web/src/pages/AdminPage.css` with neutral text color (royal blue is accent only; `AdminStyle.test.tsx` must still pass); no `style` prop; update the component's doc comment to mention the link
- [X] T018 [US3] Run `cd web && npx vitest run --maxWorkers=2 src/components/TokensPanel.test.tsx src/pages/AdminStyle.test.tsx src/pages/AdminPage.test.tsx` and `npx tsc --noEmit`; all pass

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T019 Re-run `api/internal/httpapi/docs/swagger-ui/fetch.sh` and confirm `git status --porcelain api/internal/httpapi/docs/swagger-ui` is empty (quickstart §2)
- [X] T020 Browser check per quickstart §3 in headless Firefox (puppeteer-core in the session scratchpad, local stack built from this branch with `BEER_MENU_VERSION=gate-feature9`): no session; all 46 operations listed (43 existing + 3 new; Swagger UI draws 47 blocks because `POST /api/beers/{id}/sync` has two tags); `x-access` visible on an expanded operation; no "Try it out", "Authorize" or lock buttons; console has 0 CSP violations and 0 errors; 0 requests to another origin; `/api/docs?url=https://example.com/x.yaml` still shows this API. If a `style-src` violation appears, add `'unsafe-inline'` to `style-src` only in `docsCSP` and the T009 test, record the reason in `specs/009-api-docs/research.md` R6, and re-run; never add `'unsafe-eval'` or inline script
- [X] T021 Screenshot the API Tokens page with the new link at laptop (1280×800) and phone (390×844) widths in light and dark themes; check the link is reachable and readable, and the docs open in a new tab
- [X] T022 Run `cd api && go vet ./... && go test ./...`, `cd web && npx vitest run --maxWorkers=2`, `cd api && ~/go/bin/govulncheck ./...`, and `~/go/bin/gitleaks detect --no-git --source <dir> --redact` on `api`, `web/src`, `specs`, `docs`; all pass
- [X] T023 Check every new exported or handler symbol in `api/internal/httpapi/docs.go` and the changed `TokensPanel` have doc comments (constitution VI), and that the docs CSP and allowlist each carry one why-comment

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (T002 → T003; T004 any time) blocks T013 (embeds the vendored files).
- Phase 2 blocks Phase 3: the docs page loads `/api/openapi.yaml`.
- Phase 4 (T015) needs Phase 2 only.
- Phase 5 (US3) is independent of Phases 1–4 in code; the link only works end to end once US1 is done.
- Phase 6 after all stories.

### Within Each Story

- Tests first (T005, T009, T015, T016), see them fail, then implement.
- T006 and T008 land together (contract test needs both route and operation); same for T012 and T014.
- `api/internal/httpapi/docs_test.go`, `docs.go`, `routes.go` and `api/openapi.yaml` are each touched by several tasks: run those tasks in order, not in parallel.

### Parallel Opportunities

- T004 with T002/T003.
- T010 and T011 (separate new files).
- Phase 5 (web) in parallel with Phases 2–4 (api): disjoint folders.

## Parallel Example: User Story 1

```text
Task: "T010 [P] [US1] Create api/internal/httpapi/docs/index.html"
Task: "T011 [P] [US1] Create api/internal/httpapi/docs/init.js"
```

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1 and Phase 2.
2. Phase 3 (US1). Stop and run T020: the docs page is the feature.

### Incremental Delivery

1. Add T015 (US2 byte-for-byte check).
2. Add US3 link (T016–T018).
3. Polish (T019–T023), then PR.

## Notes

- Do not edit the vendored files by hand; only `fetch.sh` writes them.
- No new Go or npm dependency. The vendored bundle is the only new third-party code.
- Every other route's headers and access stay unchanged (SC-005).
