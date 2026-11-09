# Feature Specification: Authentication and API Tokens

**Feature Branch**: `compmodder26/auth`

**Created**: 2026-11-08

**Status**: Draft

**Input**: User description: "Feature 2: Authentication and API tokens, per docs/project-brief.md ("Authentication and API tokens" section and feature breakdown row 2). Scope: first-run admin setup (no default credentials; setup screen or env var), add and remove admins, login and logout with browser sessions (username/password, argon2id or bcrypt hash, HttpOnly SameSite cookie, CSRF protection on writes), API token create/list/revoke (shown once, stored hashed, Authorization: Bearer, revocable, last-used time recorded), read and write scopes enforced (menu reads remain public on LAN), login rate limiting. Carried-forward security notes: login rate limiting must not trust client X-Forwarded-For (app sits behind NPM reverse proxy); security headers required before any admin write actions."

## Context

Feature 1 delivered a running two-container app with a public health check and no users. This
feature adds the security layer every later feature relies on. Before beers, taps or the
Brewfather key exist, the app must know who its admins are, keep strangers on the LAN out of
admin actions, and let machines (scripts, later Home Assistant and the APK) call the API with
their own revocable credentials.

The credential types, hashing choices, cookie rules, token rules and "no default credentials"
rule are fixed by constitution principle V and the brief. This spec records them as constraints,
not as design decisions to revisit.

Two credentials exist:

- **Session**: a browser login for an admin, used by the admin app.
- **API token**: a long-lived secret for a machine, with a `read` or `write` scope.

Menu reads stay public on the LAN, so the TV display needs neither. Feature 2 has no menu
endpoints yet; it establishes the rule so features 3 to 7 apply it.

## Clarifications

### Session 2026-11-08

- Q: How is the first admin created? A: Both. Deployment configuration creates it at startup if
  set and no admin exists; otherwise a setup screen is offered until an admin exists.
- Q: What does the login rate limit key on? A: The username only. It never uses the client
  address or forwarding headers. (A global cap across all usernames was dropped on 2026-11-08:
  it let any LAN device block every sign-in, and added no protection for real usernames.)
- Q: How does the owner recover if every admin is locked out? A: A `reset-password <username>`
  command in the API program, run inside the container; the password is read from standard input.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create the first admin (Priority: P1)

The owner deploys a fresh install. No admin account exists and there are no default credentials.
The owner creates the first admin account, choosing a username and password. From then on, the
app requires a login for every admin action.

**Why this priority**: nothing else in this feature, or in any later feature, can be used until
an admin exists. It is also the most exposed moment: a fresh install with no admin.

**Independent Test**: start the stack against an empty data folder, create the first admin by the
supported method, and confirm a second attempt to create a "first" admin is refused.

**Acceptance Scenarios**:

1. **Given** a fresh install with no admins, **When** the owner opens the admin app, **Then** it
   shows how to create the first admin instead of a login form.
2. **Given** no admins exist, **When** the owner creates the first admin with a valid username and
   password, **Then** the account is created and the owner is signed in or sent to the login form.
3. **Given** at least one admin exists, **When** anyone tries the first-run setup again, **Then**
   it is refused and no account is created or changed.
4. **Given** no admins exist, **When** the owner submits a password that does not meet the
   password rules, **Then** setup is refused with a clear message and no account is created.
5. **Given** no admins exist and the deployment configuration supplies a first-admin username
   and password, **When** the API starts, **Then** it creates that admin before serving requests,
   and the setup screen is never offered.
6. **Given** at least one admin exists and the deployment configuration still supplies a
   first-admin username and password, **When** the API starts, **Then** it ignores them and
   changes no account.
7. **Given** no admins exist and the configured first-admin password breaks the password rules,
   **When** the API starts, **Then** it exits with a clear error and creates no account.

---

### User Story 2 - Log in and log out (Priority: P1)

An admin opens the admin app on a laptop or phone on the LAN, signs in with username and
password, works, and signs out. A signed-out browser, or one that never signed in, cannot do
anything an admin can do.

**Why this priority**: sessions are what the admin app uses for every later feature. Without
login, no admin action can be protected.

**Independent Test**: with one admin account, sign in, call a protected admin endpoint, sign out,
and call it again. The second call is refused.

**Acceptance Scenarios**:

1. **Given** an admin account, **When** the admin signs in with the right username and password,
   **Then** a session starts and the admin app shows the signed-in view.
2. **Given** a wrong username or wrong password, **When** someone tries to sign in, **Then** the
   same generic failure message is shown in both cases, so the attempt does not reveal which
   usernames exist.
3. **Given** a signed-in admin, **When** they sign out, **Then** the session ends on the server and
   the same session can no longer be used, even if its cookie was copied.
4. **Given** a signed-in admin, **When** their session has been idle longer than the idle limit or
   has passed the absolute limit, **Then** the next request is treated as signed out.
5. **Given** a signed-in admin, **When** a page from another site tries to make the browser send
   a write request to the app, **Then** the write is refused (CSRF protection).
6. **Given** a client with no session and no token, **When** it calls any non-public endpoint,
   **Then** it is refused as unauthenticated and receives no data.

---

### User Story 3 - Slow down password guessing (Priority: P1)

Someone on the LAN, or a compromised device, tries many passwords against the login. The app
slows them down enough that guessing is impractical, without trusting anything the client says
about its own address.

**Why this priority**: login is the main attack surface on a LAN app, and the constitution makes
rate limiting non-negotiable. It must be in place before the first admin write action ships.

**Independent Test**: make repeated failed logins and confirm that, after the limit, further
attempts are refused for the stated period even with the right password, and that changing the
forwarded-address header on each request does not reset the limit.

**Acceptance Scenarios**:

1. **Given** repeated failed logins, **When** the failure count passes the limit within the
   window, **Then** further login attempts are refused with a "too many attempts, try later"
   response until the window passes.
2. **Given** an attacker who sends a different forwarded-address header on every attempt, **When**
   they keep guessing, **Then** the limit still applies to them.
3. **Given** the limit was hit, **When** the window passes, **Then** login works again.
4. **Given** the owner, **When** an attacker is guessing passwords for the owner's username,
   **Then** the owner can still sign in once the window passes, and the lockout never becomes
   permanent.
---

### User Story 4 - Create, list and revoke API tokens (Priority: P2)

The owner wants a script (and later Home Assistant or the APK) to call the API. In the admin app
they create a token, give it a name and a scope, and copy the secret, which is shown only once.
Later they view the list of tokens with each one's name, scope, created time and last-used time,
and revoke one that is no longer needed or may have leaked.

**Why this priority**: tokens are how every non-browser client authenticates, but nothing in the
MVP needs them until later features add data. Sessions come first.

**Independent Test**: create a `read` token and a `write` token, use each against a protected
read and a protected write, list the tokens, revoke one, and confirm it stops working at once.

**Acceptance Scenarios**:

1. **Given** a signed-in admin, **When** they create a token with a name and a scope, **Then** the
   token secret is shown once, and no later screen or API response shows it again.
2. **Given** existing tokens, **When** an admin lists them, **Then** each shows its name, scope,
   created time, last-used time (or "never") and whether it is revoked, but never the secret.
3. **Given** a valid token, **When** a client sends it as a bearer credential, **Then** the
   request is authenticated with that token's scope and its last-used time is updated.
4. **Given** a revoked token, **When** a client uses it, **Then** the request is refused as
   unauthenticated.
5. **Given** a `read` token, **When** a client uses it for a write, **Then** the request is
   refused as forbidden.
6. **Given** a `write` token, **When** a client uses it for a read or a write in the data API,
   **Then** the request is allowed.
7. **Given** any token, **When** a client uses it to manage admins or tokens, **Then** the request
   is refused as forbidden, so a leaked token cannot create new credentials.
8. **Given** a malformed, unknown or empty bearer credential, **When** a client sends it, **Then**
   the request is refused as unauthenticated, the same way as an unknown token.

---

### User Story 5 - Add and remove admins (Priority: P2)

The household has more than one person who manages the menu. A signed-in admin adds another admin
with a username and initial password, and can later remove an admin. Any admin can change their
own password.

**Why this priority**: useful for a shared household, but one admin is enough to run the MVP.

**Independent Test**: as one admin, add a second admin, sign in as the second, remove the first,
and confirm the first can no longer sign in and their sessions end.

**Acceptance Scenarios**:

1. **Given** a signed-in admin, **When** they add an admin with a unique username and a valid
   password, **Then** the new admin can sign in.
2. **Given** a username that already exists (ignoring letter case), **When** an admin tries to add
   it, **Then** the request is refused and no account is created.
3. **Given** two or more admins, **When** an admin removes another admin, **Then** that admin can no
   longer sign in, and all of their open sessions end at once.
4. **Given** exactly one admin, **When** anyone tries to remove that admin, **Then** the request is
   refused, so the install never loses all admins.
5. **Given** a signed-in admin, **When** they change their own password with their current
   password, **Then** the new password works, the old one does not, and their other sessions end.
6. **Given** a signed-in admin, **When** they list admins, **Then** they see usernames and created
   times, never password hashes.
7. **Given** every admin is locked out, **When** an operator with access to the running API
   container runs the reset-password command for an existing username and types a new password
   at the prompt, **Then** that admin can sign in with the new password, the old one no longer
   works, and all of that admin's sessions end.
8. **Given** an operator, **When** they run the reset-password command with a new password that
   breaks the password rules, **Then** the command fails with a clear message and changes
   nothing.

---

### User Story 6 - Browser hardening (Priority: P2)

Before any admin write action exists, responses carry the standard browser security headers, so
the admin app cannot be framed by another site, content types are not guessed, and scripts load
only from the app itself.

**Why this priority**: a carried-forward review finding. It is cheap and must land in the same
feature as the first admin write action.

**Independent Test**: request the admin page, the display page and an API endpoint, and check that
each response carries the required headers.

**Acceptance Scenarios**:

1. **Given** the running stack, **When** a browser loads the admin app, **Then** the response
   forbids framing by other sites, forbids content-type guessing, sets a restrictive referrer
   policy, and limits scripts to the app's own origin.
2. **Given** the running stack, **When** a client calls any API endpoint, **Then** the response
   carries the same no-framing and no-content-sniffing protections.
3. **Given** the display page, **When** it loads in Fully Kiosk, **Then** the headers do not stop it
   from working.

### Edge Cases

- Two people submit first-run setup at the same moment: exactly one succeeds; the other is
  refused.
- An admin removes themselves while other admins exist: allowed; their sessions end and they are
  signed out.
- An admin is removed while a request of theirs is in flight: requests after the removal are
  refused.
- A token is revoked while a client is mid-use: the next request with that token is refused.
- A request carries both a session cookie and a bearer token: the bearer token decides the
  request, and a cookie alone never satisfies a token-only check. [Assumption]
- Usernames that differ only by letter case are treated as the same username.
- Leading or trailing spaces in a username are trimmed; passwords are used exactly as typed.
- Very long passwords or usernames are refused with a clear message before any hashing, so a
  huge input cannot tie up the server.
- The app restarts: sessions and tokens survive the restart; login rate-limit counters may reset.
- The clock moves backwards: a session whose sign-in time is later than the current time is
  treated as invalid, so a clock jump never extends a session.
- A token name is empty or duplicates another token's name: empty is refused; duplicates are
  allowed, because the list also shows created time and scope.
- The reset-password command is given a username that does not exist: it fails with a clear
  message, exits with an error, and creates no account.
- The reset-password command runs while the API is serving requests: the change applies to the
  next sign-in, and the admin's existing sessions stop working at once.
- The reset-password command gets no input (for example, run without an interactive terminal):
  it fails and changes nothing.
- A database restored from a backup brings back admins, sessions and tokens as they were at
  backup time, including tokens revoked since. Documented in the restore notes.

## Requirements *(mandatory)*

### Functional Requirements

**First-run setup**

- **FR-001**: The system MUST ship with no admin accounts and no default credentials.
- **FR-002**: The system MUST let the owner create the first admin only while no admin exists,
  by either method:
  - **Deployment configuration**: if a first-admin username and password are configured and no
    admin exists, the API MUST create that admin at startup, before serving requests. If an admin
    already exists, the configured values MUST be ignored. If the configured password breaks the
    password rules, the API MUST exit with a clear error.
  - **Setup screen**: otherwise, while no admin exists, the admin app MUST offer a setup screen
    that creates the first admin. The screen and its endpoint MUST stop working as soon as any
    admin exists.
- **FR-003**: Once any admin exists, first-run setup MUST be refused and MUST NOT change any
  account.
- **FR-004**: Concurrent first-run setup attempts MUST result in at most one admin being created.
- **FR-005**: The system MUST report, without authentication, only whether first-run setup is
  still needed, so the admin app can show setup or login. It MUST NOT reveal usernames.

**Accounts and passwords**

- **FR-006**: Usernames MUST be unique ignoring letter case, 1 to 64 characters after trimming
  surrounding spaces, using only ASCII letters, digits, `.`, `_` and `-`.
- **FR-007**: Passwords MUST be at least 12 characters and at most 256 characters. No other
  composition rules.
- **FR-008**: Passwords MUST be stored only as hashes made with argon2id or bcrypt. The plaintext
  MUST never be stored, logged or returned.
- **FR-009**: A signed-in admin MUST be able to add an admin with a username and an initial
  password.
- **FR-010**: A signed-in admin MUST be able to list admins (username, created time), with no
  password data.
- **FR-011**: A signed-in admin MUST be able to remove any admin, including themselves, except the
  last remaining admin. Removing an admin MUST end all of that admin's sessions at once.
- **FR-012**: A signed-in admin MUST be able to change their own password by giving the current
  one. A change MUST end the admin's other sessions.

**Account recovery**

- **FR-041**: The API program MUST provide a `reset-password <username>` command that an operator
  runs inside the API container (for example with `docker compose exec`). It MUST:
  - read the new password from standard input, with a confirmation entry, never from a
    command-line argument or environment variable;
  - apply the password rules in FR-007 and fail without changing anything if they are broken;
  - fail with an error, and create no account, if the username does not exist;
  - end all of that admin's sessions once the password changes;
  - log the reset with the username and time, never the password.
- **FR-042**: Account recovery MUST NOT be reachable over the network. No HTTP endpoint may reset
  a password without the current password.

**Sessions**

- **FR-013**: An admin MUST be able to sign in with username and password, which starts a session
  held in a cookie that page scripts cannot read and that browsers do not send on cross-site
  requests.
- **FR-014**: Failed logins MUST return the same response for an unknown username and a wrong
  password, and take about the same time.
- **FR-015**: Sessions MUST be stored on the server so they can be ended there. Sign-out MUST end
  the session on the server, not only clear the cookie.
- **FR-016**: Sessions MUST expire after 7 days without use and 30 days after sign-in, whichever
  comes first.
- **FR-017**: A new session identifier MUST be issued at every sign-in; an identifier set before
  sign-in MUST NOT be reused.
- **FR-018**: Every state-changing request from a browser MUST pass CSRF protection, including
  the public sign-in and first-admin creation requests; a cross-site request MUST be refused.

**Login rate limiting**

- **FR-019**: Failed logins MUST be limited to 5 per 15 minutes per username (compared ignoring
  letter case, including usernames that do not exist). Once over the limit, login attempts for
  that username MUST be refused until the window passes, even with the right password. The
  limit MUST NOT depend on the client's network address or on any forwarding header.
- **FR-020**: The rate limit MUST NOT be bypassable by changing client-supplied forwarding headers
  such as `X-Forwarded-For`.
- **FR-021**: The rate limit MUST never lock an account permanently.
- **FR-022**: Rate-limited responses MUST tell the client to retry later and roughly when.

**API tokens**

- **FR-023**: A signed-in admin MUST be able to create a token with a name (1 to 64 characters)
  and a scope of `read` or `write`.
- **FR-024**: The token secret MUST be shown once, in the creation response only. The system MUST
  store only a hash of it.
- **FR-025**: Token secrets MUST be random with at least 128 bits of entropy and carry a fixed,
  recognisable prefix so leaked tokens are easy to spot in logs and secret scanners.
- **FR-026**: A signed-in admin MUST be able to list tokens with name, scope, created time,
  last-used time and revoked state, and never the secret or its hash.
- **FR-027**: A signed-in admin MUST be able to revoke a token. Revocation MUST take effect on the
  next request. Revoked tokens stay in the list, marked revoked.
- **FR-028**: Clients MUST send tokens as `Authorization: Bearer <token>`. Tokens in query strings
  or cookies MUST NOT be accepted.
- **FR-029**: Each successful token use MUST update the token's last-used time. The time MAY be
  coarse (up to 1 minute) to limit database writes.

**Authorization rules**

- **FR-030**: Only menu reads and the bootstrap endpoints MUST be public: the health check,
  first-run setup status, first-admin creation (refused once an admin exists) and sign-in.
  Every other endpoint MUST require a session or a token with the right scope.
- **FR-031**: A `read` token MUST allow only read requests to data endpoints (menu, beers, taps,
  kegs, settings as later features add them). It MUST be refused on writes.
- **FR-032**: A `write` token MUST allow reads and writes to data endpoints.
- **FR-033**: Managing admins, managing tokens and changing passwords MUST require a session. No
  token scope MAY do these. [Assumption: limits damage from a leaked token.]
- **FR-034**: Unauthenticated requests MUST be refused as unauthenticated; authenticated requests
  without the needed scope MUST be refused as forbidden. Neither may return data.
- **FR-035**: Every new endpoint in later features MUST declare whether it is public, read or
  write, and default to requiring authentication if it declares nothing.

**Security headers and logging**

- **FR-036**: Responses MUST carry headers that forbid framing by other sites, forbid content-type
  sniffing and set a restrictive referrer policy. Admin app pages MUST also carry a content
  security policy that limits scripts to the app's own origin.
- **FR-037**: Authentication responses and responses with secrets (such as a new token) MUST tell
  browsers and proxies not to cache them.
- **FR-038**: Passwords, session identifiers and token secrets MUST never appear in logs.
  Sign-ins, failed sign-ins, rate-limit hits, admin adds and removals, and token creates and
  revokes MUST be logged with the username or token name and time.

**Contract and admin app**

- **FR-039**: Every endpoint in this feature MUST be in the OpenAPI document, including its
  security requirement (public, session, `read` token, `write` token).
- **FR-040**: The admin app MUST provide screens for first-run setup, sign-in, sign-out, admin management, own password change and token management.

### Key Entities

- **Admin**: a person who manages the menu. Username (unique ignoring case), password hash,
  created time.
- **Session**: a signed-in browser for one admin. Belongs to an admin; created time, last-used
  time, expiry. Ends on sign-out, expiry, password change or admin removal.
- **API token**: a machine credential. Name, scope (`read` or `write`), hash of the secret,
  created time, last-used time, revoked time. Not tied to the admin who created it, so removing
  an admin does not break their scripts.
- **Login attempt record**: recent failed logins per username, used only for rate
  limiting. May be held in memory.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a fresh install, the owner can go from first start to a signed-in admin app in
  under 2 minutes.
- **SC-002**: 100% of non-public endpoints refuse requests with no credential, with a revoked
  token, or (for writes) with a `read` token, as shown by automated tests for each rule.
- **SC-003**: After 5 failed logins in 15 minutes, 100% of further attempts in that window are
  refused, including attempts that change forwarding headers.
- **SC-004**: A revoked token or a signed-out session is refused on the very next request.
- **SC-005**: No password, session identifier or token secret is found in logs, API responses
  (other than the one-time token display) or the database in plain form, as checked by tests.
- **SC-006**: The admin app, the display page and the API all return the required security
  headers, and the display still runs in Fully Kiosk.
- **SC-007**: A cross-site write attempt against a signed-in browser fails 100% of the time.
- **SC-008**: Sign-in completes in under 1 second on the TrueNAS server for a correct password.

## Assumptions

- Any admin can do everything any other admin can. There are no roles beyond "admin" in the MVP.
- There is no password reset by email or "forgot password" flow. Recovery uses the
  `reset-password` command (FR-041), which needs Docker access on the server. Anyone with that
  access can already read the database file, so the command adds no new attack surface. The
  quickstart documents it.
- Tokens are not tied to the admin who created them and have no expiry date; revocation is the
  way to end them. Expiring tokens can be added later without breaking clients.
- Write tokens can call every data endpoint but not admin, token or password management (FR-033).
  This keeps a leaked Home Assistant token from creating new credentials.
- The idle and absolute session limits (7 and 30 days) suit a home admin used from a few devices.
- The per-username limit lets an attacker make a real admin wait out the 15-minute window. The
  owner accepted this trade-off: it never locks an account permanently and needs no proxy
  configuration.
- Rate-limit counters may live in memory and reset on restart; a restart does not help an
  attacker much, because the LAN attacker cannot restart the server.
- Menu read endpoints arrive in features 3 to 5. This feature defines the public/read/write rule
  and proves it on its own endpoints and a test-only protected route if needed.
- Multi-factor authentication, single sign-on and account lockout notifications are out of scope.
- TLS is handled by NPM. Whether the session cookie is marked HTTPS-only is a deployment
  setting, not inferred from request headers, so plain LAN HTTP still works when the owner
  chooses it. The plan picks the default.
- The restore notes from feature 1 gain a line: a restored backup brings back admins and tokens
  as they were, so tokens revoked since the backup must be revoked again.
