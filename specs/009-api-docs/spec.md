# Feature Specification: API Docs

**Feature Branch**: `compmodder26/feature_9`

**Created**: 2026-11-11

**Status**: Draft

**Input**: User description: "Feature 9: API docs. The API serves Swagger UI at `/api/docs` and the OpenAPI spec at `/api/openapi.yaml`, both without authentication. Read-only: no \"Try it out\" or Authorize. Swagger UI assets are a pinned version bundled in the API image, so the page works offline and keeps a `'self'`-only CSP. The API Tokens page links to it. Depends on feature 8."

## Context

The API is the product, and its contract is an OpenAPI document kept in the repo since feature 1.
Today that document is only readable by opening the source. Anyone writing a client (a script,
Home Assistant in roadmap phase 2, the Android APK in phase 3) has to find and read the raw file.

This feature publishes the contract from the running app: a browsable documentation page and the
raw document itself, both open on the LAN without signing in. The page only documents the API.
It cannot send requests, so it gives no one a way to read or change data, and it holds no
credential.

The owner made these choices when adding the feature to the brief (2026-11-11): the docs live
under the API's own path, they are public, they are read-only, the page works with no internet
access, and the admin app's API Tokens page links to them.

Out of scope: sending requests from the docs page, a docs link in the side panel or on the TV,
versioned or historical docs, and any change to the API's existing endpoints.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse the API docs without signing in (Priority: P1)

Someone on the LAN who wants to script against the tap list opens `/api/docs` in a browser. With
no account and no token, they see every endpoint grouped by area, with its method, path,
parameters, request body, responses and the credential it needs. They expand an endpoint, read
its schema, and close the page knowing how to call it.

**Why this priority**: browsable docs are the feature. Everything else supports it.

**Independent Test**: on a fresh install with no admin signed in, open `/api/docs`, confirm it
lists every endpoint in the contract with its access level, and confirm no request needed a
credential.

**Acceptance Scenarios**:

1. **Given** a browser with no session and no token, **When** it opens `/api/docs`, **Then** the
   docs page loads and lists every endpoint in the API contract.
2. **Given** the docs page is open, **When** the reader expands an endpoint, **Then** they see its
   parameters, request body, responses and the access it needs (public, `read`, `write` or admin).
3. **Given** the docs page is open, **When** the reader looks for a way to send a request or enter
   a credential, **Then** there is none: no "Try it out" control and no Authorize control.
4. **Given** the app has no internet access, **When** the docs page loads, **Then** it renders
   fully, with every script, style and image served by the app itself.

---

### User Story 2 - Download the raw API contract (Priority: P2)

A developer generating a client, or a Home Assistant setup, fetches `/api/openapi.yaml` with no
credential and gets the exact contract the running version of the API serves.

**Why this priority**: tools need the machine-readable contract; the page alone serves people.

**Independent Test**: fetch `/api/openapi.yaml` with no credential and compare it byte for byte
with `api/openapi.yaml` for the same release.

**Acceptance Scenarios**:

1. **Given** no credential, **When** a client fetches `/api/openapi.yaml`, **Then** it receives
   the API contract, identical to the one in the repo at that release, labeled as YAML.
2. **Given** a new release changes the contract, **When** the API restarts on that release,
   **Then** both `/api/openapi.yaml` and `/api/docs` show the new contract.

---

### User Story 3 - Find the docs from the API Tokens page (Priority: P3)

The owner creates a token for a script on the API Tokens page and follows a link there to the API
docs, which open without disturbing the admin app.

**Why this priority**: the link is a convenience; the docs work by URL without it.

**Independent Test**: sign in, open API Tokens, follow the docs link, and confirm the docs open and
the admin app is still signed in on the same page.

**Acceptance Scenarios**:

1. **Given** the owner is on the API Tokens page, **When** they look near the token list, **Then**
   they see a link to the API docs.
2. **Given** the owner follows the link, **When** the docs open, **Then** they open in a new tab and
   the admin app stays on API Tokens, still signed in.

---

### Edge Cases

- A signed-in admin opens the docs: the page behaves the same as for anyone else and sends no
  request with their session beyond fetching the contract.
- The docs page is opened with a query string: it still loads and shows this API's contract.
  With a trailing slash (`/api/docs/`) it gets 404 like any other unknown API path; the admin
  link and documented URL have no slash.
- A request uses a method other than GET or HEAD on either docs URL: it is refused with 405, like
  other API routes.
- Another site tries to embed the docs page in a frame: the browser refuses.
- A path under the docs that does not exist (for example a mistyped asset) returns 404, never
  another file from the API image.
- The API's own endpoint list and the contract drift apart: the existing contract test still fails
  the build, so the docs never describe routes that do not exist.
- The docs page is opened through the reverse proxy on another hostname: it loads, because it uses
  only same-origin relative paths.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The API MUST serve a human-readable documentation page for its contract at
  `/api/docs`, reachable through the web container's existing `/api` proxy with no change there.
- **FR-002**: The API MUST serve its contract document at `/api/openapi.yaml`, byte-identical to
  the contract in the repo for that build, with a YAML content type.
- **FR-003**: Both docs URLs MUST be public: they MUST answer without a session or token, and a
  supplied credential MUST NOT change the response.
- **FR-004**: The docs page MUST be read-only: it MUST NOT offer a way to send API requests
  ("Try it out") or to enter credentials (Authorize).
- **FR-005**: The docs page MUST show, for every operation in the contract, its method, path,
  summary, parameters, request body, responses and required access level.
- **FR-006**: Every script, stylesheet, font and image the docs page uses MUST be served by the
  API itself, from a pinned version shipped in the image. The page MUST NOT load anything from
  another origin.
- **FR-007**: The docs page MUST carry a content security policy that allows only same-origin
  resources, forbids framing by other sites, and allows no inline script. Every other API
  response keeps its current policy.
- **FR-008**: The docs URLs MUST answer only GET and HEAD; other methods MUST get 405.
- **FR-009**: The docs page and contract MUST NOT include secrets, private hostnames or
  environment-specific server addresses beyond what the committed contract already holds.
- **FR-010**: The admin app's API Tokens page MUST show a link to `/api/docs` that opens in a new
  tab without passing the opener to the new page.
- **FR-011**: The contract document MUST describe the two new docs endpoints, marked public, so
  the existing contract test keeps holding every route and operation in step.
- **FR-012**: The constitution's list of public endpoints (principle V) MUST include the API
  docs. Amended to v1.3.0 in this feature's pull request.

### Key Entities

- **API contract**: the OpenAPI document in the repo, built into the API. Source of both the raw
  download and the docs page. One version per release; no history kept.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A reader with no account can find any endpoint's path, method and required access
  level from the docs page in under 1 minute.
- **SC-002**: 100% of operations in the contract appear on the docs page; the page shows 0
  controls that send requests or accept credentials.
- **SC-003**: The docs page loads fully on a host with no internet access, with 0 requests to
  another origin and 0 policy violations in the browser console.
- **SC-004**: The downloaded contract matches the repo contract for the same release in 100% of
  releases.
- **SC-005**: Every existing endpoint keeps its current access rules and responses: the full
  existing test suite passes unchanged.

## Assumptions

- "Read-only" means the page cannot call the API beyond fetching the contract. It does not hide
  write endpoints; they are documented like any other.
- Public docs are acceptable on the LAN: they expose the route map but no data, and the repo (and
  so the contract) may become public anyway.
- The docs page uses the API's existing security headers except for its own content security
  policy, which needs to allow same-origin scripts and styles.
- The docs page follows the documentation tool's own look; it is not restyled to match the admin
  app or its light and dark themes.
- No rate limit on the docs URLs, in line with the public menu read (LAN, cheap static reads).
- The bundled documentation viewer is a new dependency; its choice and version (at least two weeks
  old) are justified in the plan.
- Depends on feature 8 for the API Tokens page layout the link sits on.
