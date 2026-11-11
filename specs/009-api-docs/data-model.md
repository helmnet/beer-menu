# Data Model: API Docs (feature 9)

No database change. No migration. No new settings.

## Served documents (read-only, built into the API binary)

| Document | Source | Changes when |
| --- | --- | --- |
| API contract | `api/openapi.yaml`, embedded as `api.OpenAPI` | A release changes the contract |
| Docs page | `api/internal/httpapi/docs/index.html` (ours) | This feature only |
| Docs config | `api/internal/httpapi/docs/init.js` (ours) | This feature only |
| Viewer assets | `api/internal/httpapi/docs/swagger-ui/*`, vendored `swagger-ui-dist` 5.34.0 | Manual upgrade via `fetch.sh` |

Rules:

- The served contract is byte-identical to `api/openapi.yaml` in the same build (FR-002).
- Only the allowlisted asset names are served (research R3); there is no lookup by arbitrary path.
- Nothing served depends on the database, the environment or the request's credentials (FR-003,
  FR-009).
