# Implementation Plan: API Docs

**Branch**: `compmodder26/feature_9` | **Date**: 2026-11-11 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/009-api-docs/spec.md`

## Summary

Serve read-only Swagger UI at `/api/docs` and the embedded contract at `/api/openapi.yaml`,
both public, from the Go API. Swagger UI 5.34.0 is vendored (byte-identical, sha512-checked by a
fetch script) and embedded in the binary, so the page works offline under a same-origin-only CSP.
Read-only through Swagger UI's own options (no submit methods, Authorize buttons wrapped to
nothing). The admin API Tokens page links to the docs. The constitution is amended to v1.3.0 in
this PR to allow the public docs.

## Technical Context

**Language/Version**: Go 1.27 (API); TypeScript 5 + React 18 (admin link)

**Primary Dependencies**: standard library `net/http`, `embed`; new vendored static asset
`swagger-ui-dist` 5.34.0 (research R1, R2). No new Go or npm dependency.

**Storage**: N/A (no database change)

**Testing**: `go test` with `httptest` against `NewMux`; Vitest + Testing Library for
`TokensPanel`; headless Firefox check for CSP and read-only UI (quickstart)

**Target Platform**: `beer-menu-api` scratch image on TrueNAS, behind nginx and NPM on the LAN

**Project Type**: web service + web app (existing `api/` and `web/`)

**Performance Goals**: docs page interactive in under 3 s on the LAN (about 1.9 MB of assets,
served from memory)

**Constraints**: no request to another origin; `script-src 'self'` with no inline script; no
change to any existing route's behavior or headers; binary grows by about 1.9 MB

**Scale/Scope**: 3 new public routes, 1 handler file, 1 link, about 40 documented operations shown

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Status |
| --- | --- | --- |
| I. API-First | Contract gains the 3 docs operations (`docs` tag), `info.version` 0.8.0; contract test keeps routes and operations in step | Pass |
| II. Go + SQLite | Standard library only; still one static binary; no schema change | Pass |
| III. React Clients | Admin change is one link in `TokensPanel`; `/display` untouched | Pass |
| IV. Tests Required | Handler tests on `NewMux` (types, CSP, allowlist and path traversal, 404/405, credentials ignored); `TestPublicRoutes` updated; `TokensPanel` component test; no migration | Pass |
| V. LAN Security | Docs are public only after the v1.3.0 amendment in this PR (spec FR-012); docs send no API requests beyond the contract and accept no credentials; strict same-origin CSP; fixed allowlist; no secrets in the contract | Pass (with amendment) |
| VI. Documented | Doc comments on the handlers and the embed; vendored dir has a README with version, checksum and upgrade steps | Pass |
| VII. Simplicity | Vendored files over a Go module or build step (R2); 5 upstream files, not the full package; no gzip, no cache headers, no theming | Pass |
| Release age ≥ 2 weeks | 5.34.0 released 2026-10-19 (23 days) | Pass |

Post-design re-check (after Phase 1): unchanged, all pass. No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/009-api-docs/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/openapi-docs.yaml
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
.specify/memory/constitution.md       # v1.3.0: principle V lists the API docs (done)
api/
├── openapi.yaml                      # + docs tag, 3 operations, version 0.8.0
└── internal/httpapi/
    ├── routes.go                     # + 3 AccessPublic routes
    ├── routes_test.go                # TestPublicRoutes want list
    ├── docs.go                       # embed + getDocs, getDocsAsset, getOpenAPI, docs CSP
    ├── docs_test.go
    └── docs/
        ├── index.html                # ours: links /api/docs/swagger-ui.css, bundle, init.js
        ├── init.js                   # ours: read-only SwaggerUIBundle config (research R5)
        └── swagger-ui/
            ├── README.md             # version, npm sha512, upgrade steps, license note
            ├── fetch.sh              # downloads pinned tarball, verifies, copies 5 files
            ├── swagger-ui-bundle.js  # vendored, byte-identical
            ├── swagger-ui.css
            ├── swagger-ui-bundle.js.LICENSE.txt
            ├── LICENSE
            └── NOTICE
web/src/components/
├── TokensPanel.tsx                   # + "API docs" link (new tab, noopener noreferrer)
└── TokensPanel.test.tsx              # link test
docs/project-brief.md                 # unchanged (feature 9 row already merged)
```

**Structure Decision**: everything lives in the existing `httpapi` package beside the route
table; `go:embed` names the files explicitly so `README.md` and `fetch.sh` are not served.
Assets are under `docs/` inside the package because `go:embed` cannot reach parent directories.

## Complexity Tracking

None.
