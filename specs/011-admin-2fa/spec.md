# Feature Specification: Two-Factor Sign-In for Admins

**Feature Branch**: `compmodder26/2fa`

**Created**: 2026-11-12

**Status**: Draft

**Input**: User description: "I want to be able to enable 2fa (authenticators, passkeys, biometrics if possible) on the admin GUI (API doesn't need it). Are there pre-made libraries/components that can handle that. And is biometrics a possibility on a PWA?"

## Context

Feature 2 gave admins a username-and-password sign-in that starts a browser session. This
feature adds a second factor to that sign-in, so a leaked or guessed password alone no longer
opens the admin app.

Two kinds of second factor are in scope:

- **Authenticator app**: a 6-digit code that changes every 30 seconds, from an app such as Google
  Authenticator, 1Password or Aegis.
- **Passkey**: a credential held by the admin's device or a security key, unlocked with the
  device's own check (fingerprint, face, PIN or a key touch).

"Biometrics" is delivered through passkeys. The app never sees or stores fingerprint or face data;
the device does the check and only reports that it passed. This works in the installed admin app
(PWA) as well as in a browser tab, on devices and browsers that support passkeys.

API tokens are out of scope: machine clients (TV display, scripts, Home Assistant, the future
APK) keep using bearer tokens exactly as today, with no second factor.

## Clarifications

### Session 2026-11-12

- Q: Can a passkey replace the password (passwordless sign-in)? A: No. A passkey is only a second
  step after the password.
- Q: Is 2FA optional or required? A: Optional per admin, with an admin setting that, when on,
  requires every admin to use 2FA.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Turn on an authenticator app and use it to sign in (Priority: P1)

An admin opens the Two-Factor Sign-In page, chooses to add an authenticator app, scans the shown QR
code (or types the shown key) into their app, and confirms with a current code. They are shown a
set of one-time recovery codes to save. From then on, signing in asks for a code after the
password.

**Why this priority**: authenticator apps work on every device and browser, need no special
hardware, and close the "password alone" gap on their own.

**Independent Test**: enable an authenticator for an admin, sign out, and confirm sign-in needs
both the password and a valid current code, and that a wrong or old code is refused.

**Acceptance Scenarios**:

1. **Given** an admin with no second factor, **When** they add an authenticator and confirm it
   with a valid current code, **Then** the authenticator is saved, recovery codes are shown once,
   and their other sessions are ended.
2. **Given** an admin setting up an authenticator, **When** they confirm with a wrong code,
   **Then** nothing is saved and they can try again.
3. **Given** an admin with an authenticator, **When** they sign in with the right password,
   **Then** they are asked for a code and no session starts until a valid code is given.
4. **Given** an admin at the code step, **When** they enter a code already used for an earlier
   sign-in, **Then** it is refused.
5. **Given** an admin at the code step, **When** they enter a wrong code too many times, **Then**
   further attempts are refused for a while, just like failed passwords.

---

### User Story 2 - Add a passkey and sign in with fingerprint or face (Priority: P2)

An admin on a phone or laptop with a fingerprint reader, face unlock, or a security key adds a
passkey from the Two-Factor Sign-In page and gives it a name ("Kitchen phone"). At sign-in, instead
of typing a code, they approve with their fingerprint, face or key.

**Why this priority**: faster and phishing-resistant, and the only way to offer biometrics, but it
depends on device and browser support, so the authenticator path comes first.

**Independent Test**: on a supported device, add a passkey, sign out, and confirm sign-in completes
by approving the device prompt; confirm a removed passkey no longer works.

**Acceptance Scenarios**:

1. **Given** a signed-in admin on a device that supports passkeys, **When** they add a passkey and
   approve the device prompt, **Then** the passkey is saved under the name they gave it.
2. **Given** an admin with a passkey, **When** they sign in on that device, **Then** they can
   complete the second step by approving the device prompt.
3. **Given** an admin with more than one second factor, **When** they reach the second step,
   **Then** they can choose which one to use.
4. **Given** a device or browser without passkey support, **When** the admin opens security
   settings, **Then** the option to add a passkey says it is not available here, and other
   options still work.
5. **Given** an admin who cancels or fails the device prompt, **When** the prompt closes,
   **Then** no session starts and they can retry or pick another factor.

---

### User Story 3 - Recover when a second factor is lost (Priority: P2)

An admin loses their phone. They sign in with their password and one of their saved recovery
codes, then remove the lost device's factor and set up a new one. If they have lost their codes
too, another admin can clear their second factors; if they are the only admin, the operator can
clear them from inside the server.

**Why this priority**: without a way back in, turning on 2FA risks locking the owner out of their
own bar menu.

**Independent Test**: use a recovery code to sign in and confirm it cannot be used again; clear an
admin's factors as another admin and as the operator, and confirm the admin can then sign in with
the password alone.

**Acceptance Scenarios**:

1. **Given** an admin with 2FA and unused recovery codes, **When** they enter a recovery code at
   the second step, **Then** they are signed in and that code is spent.
2. **Given** a signed-in admin, **When** they ask for new recovery codes (after confirming their
   identity), **Then** new codes are shown once and all old codes stop working.
3. **Given** two admins, **When** one clears the other's second factors, **Then** all of that
   admin's factors and recovery codes are removed and all of their sessions end.
4. **Given** a sole admin who lost every factor and code, **When** the operator runs the server-side
   reset for that username, **Then** the admin's second factors are cleared and they can sign in
   with their password.

---

### User Story 4 - Manage own second factors (Priority: P3)

An admin sees the list of their second factors (type, name, when added, when last used), renames
or removes any of them, and can turn 2FA off entirely.

**Why this priority**: housekeeping; needed for long-term use but not for the first sign-in.

**Independent Test**: add two factors, remove one, and confirm only the other works at sign-in;
remove the last one and confirm sign-in needs only the password.

**Acceptance Scenarios**:

1. **Given** an admin with factors, **When** they open the Two-Factor Sign-In page, **Then** they see each
   factor's type, name, added time and last-used time, and never any secret.
2. **Given** an admin removing a factor or turning 2FA off, **When** they confirm, **Then** they
   must first prove it is them (current password, plus a second factor if they have one left).
3. **Given** an admin who removes their last factor, **When** they next sign in, **Then** only the
   password is asked for, and their recovery codes no longer exist.

---

### User Story 5 - Require 2FA for every admin (Priority: P3)

An admin who has 2FA turns on a setting that requires it for every admin. From then on, an admin
without a second factor who signs in with their password must set one up before they can do
anything else.

**Why this priority**: closes the gap where one admin skips 2FA, but only matters once there is
more than one admin, and it builds on stories 1 and 4.

**Independent Test**: turn the setting on, sign in as an admin with no factor, and confirm the
only thing they can do is set up a factor (or sign out); confirm admins can no longer remove their
last factor while the setting is on.

**Acceptance Scenarios**:

1. **Given** an admin without 2FA, **When** they try to turn the requirement on, **Then** it is
   refused until they have a factor themselves.
2. **Given** the requirement is on, **When** an admin without a factor gives the correct password,
   **Then** they must set up a factor and save recovery codes before any session for admin actions
   starts.
3. **Given** the requirement is on, **When** an admin tries to remove their last factor or turn
   2FA off, **Then** it is refused with a message naming the setting.
4. **Given** the requirement is on, **When** another admin or the operator clears an admin's
   factors, **Then** that admin must set up a new factor at their next sign-in.
5. **Given** the requirement is on, **When** an admin turns it off (after confirming their
   password and a second factor), **Then** admins may again choose to have no factor.

---

### Edge Cases

- Admin's device clock is off by a minute or so: codes from the step just before or after the
  current one are accepted; larger drift is refused.
- Admin abandons sign-in after the password step: the half-finished sign-in expires within a few
  minutes and grants nothing.
- Second-step attempts count against the same per-username limit as failed passwords, so the
  limit cannot be dodged by guessing codes instead.
- Admin opens the app by IP address or over plain HTTP: passkeys cannot work there; the app says
  so, and authenticator codes still work.
- Admin is removed, or their factors are cleared, while a sign-in is half finished: that sign-in
  cannot complete.
- Recovery codes all used up: the admin is warned when few remain and can make a new set.
- Sign-in failure messages never reveal whether the username exists or whether it has 2FA before
  the correct password is given.

## Requirements *(mandatory)*

### Functional Requirements

**Sign-in flow**

- **FR-001**: An admin with at least one second factor MUST give the correct password and then a
  valid second factor before any session starts. No admin action may be possible between the two
  steps.
- **FR-002**: The half-finished sign-in between the two steps MUST be held on the server, MUST be
  usable only by the browser that gave the password, MUST expire after 5 minutes, and MUST be
  usable for one completed sign-in only.
- **FR-003**: An admin with no second factor MUST sign in with password only, as today, unless
  2FA is required for all admins (FR-034).
- **FR-004**: The response to a wrong password MUST be the same whether or not the admin has 2FA.
  The second step MUST be offered only after the correct password.
- **FR-005**: Failed second-step attempts MUST count toward the existing per-username sign-in
  limit (5 failures per 15 minutes), and that limit MUST apply to the second step too.
- **FR-006**: Every step of sign-in MUST keep the existing protection against cross-site requests.
- **FR-007**: A passkey MUST be usable only as the second step after the correct password.
  Signing in with a passkey and no password is out of scope.

**Authenticator apps**

- **FR-008**: An admin MUST be able to add an authenticator by scanning a QR code or typing a key,
  and the authenticator MUST NOT be saved until the admin confirms it with a valid current code.
- **FR-009**: Codes MUST be 6 digits on a 30-second step, accepted for the current step and one
  step either side.
- **FR-010**: A code MUST NOT be accepted twice for the same admin.
- **FR-011**: Authenticator secrets MUST be encrypted at rest with the key held outside the
  database, and MUST never be shown again or returned by the API after setup.
- **FR-012**: An admin MAY have at most one authenticator app at a time.

**Passkeys**

- **FR-013**: An admin MUST be able to add one or more passkeys, at most 10, each with a name
  of 1 to 64 characters. An 11th MUST be refused with a message saying to remove one first.
- **FR-014**: Adding or using a passkey MUST require the device's own user check (biometric, PIN
  or key touch).
- **FR-015**: A passkey MUST be usable only for this app's site and only by the admin who added it.
- **FR-016**: Where passkeys are not supported (browser, device, or the app not opened over HTTPS
  by its host name), the app MUST say so and MUST NOT offer a passkey option that cannot work.

**Recovery codes**

- **FR-017**: When an admin adds their first second factor, the app MUST create 10 single-use
  recovery codes and show them once, with a way to copy or download them.
- **FR-018**: Recovery codes MUST be stored only as hashes and MUST work once each.
- **FR-019**: An admin MUST be able to replace their recovery codes; making a new set MUST make
  every old code invalid.
- **FR-020**: The app MUST show how many recovery codes remain and warn when 2 or fewer remain.

**Managing factors**

- **FR-021**: An admin MUST be able to list their own second factors with type, name, added time
  and last-used time, and never any secret.
- **FR-022**: An admin MUST be able to rename passkeys and remove any of their factors.
- **FR-023**: Adding a factor, removing a factor, replacing recovery codes or turning 2FA off MUST
  require the admin's current password; if they still have a second factor, removing a factor or
  turning 2FA off MUST also require a valid second factor.
- **FR-024**: Adding the first factor, removing a factor, and turning 2FA off MUST end the admin's
  other sessions.
- **FR-025**: Removing the last factor MUST also delete the admin's recovery codes.
- **FR-026**: 2FA MUST be optional per admin unless the "require 2FA for all admins" setting is
  on. The setting MUST be off by default.

**Requiring 2FA for all admins**

- **FR-033**: A signed-in admin MUST be able to turn the requirement on only if they have at least
  one second factor, and MUST confirm with their current password and a second factor to turn it
  on or off. Changing it MUST be possible only with an admin session, never an API token.
- **FR-034**: While the requirement is on, an admin without a factor who gives the correct password
  MUST be limited to setting up a factor (and signing out). No other admin action may be possible
  until a factor is confirmed and recovery codes are shown. This limited state MUST follow the
  same 5-minute expiry and single-browser rules as FR-002.
- **FR-035**: While the requirement is on, removing an admin's last factor or turning their 2FA off
  MUST be refused. Clearing by another admin (FR-027) or by the operator (FR-029) MUST still work;
  the affected admin then falls under FR-034 at next sign-in.
- **FR-036**: Turning the requirement on MUST NOT end existing sessions of admins without a factor;
  they fall under FR-034 at their next sign-in.

**Recovery by others**

- **FR-027**: A signed-in admin MUST be able to clear all second factors of another admin, after
  confirming their identity (FR-023: their current password, plus a second factor if they have
  one). This MUST end all of that admin's sessions. An admin MUST NOT use this to clear their own
  factors.
- **FR-028**: The admin list MUST show whether each admin has 2FA turned on.
- **FR-037**: Adding an admin or removing an admin (including yourself) MUST also need a confirmed
  identity, as in FR-027.
- **FR-029**: The API program MUST provide a server-side command, run inside the API container
  like the existing `reset-password` command, that clears all second factors and recovery codes
  of a named admin, ends their sessions, and logs the username and time. This MUST NOT be
  reachable over the network.

**Scope boundaries**

- **FR-030**: API tokens MUST keep working exactly as today, with no second factor.
- **FR-031**: Endpoints for managing second factors MUST accept only an admin session, never an
  API token.
- **FR-032**: Second-factor setup, sign-in steps and factor management MUST be documented in the
  OpenAPI document.

### Key Entities

- **Second factor**: belongs to one admin. Type (authenticator or passkey), name, added time,
  last-used time. An authenticator holds an encrypted secret and the last accepted code step; a
  passkey holds the public credential the device gave.
- **Recovery code**: belongs to one admin. Stored hashed, with a used/unused state.
- **Pending sign-in**: a short-lived, server-held record that an admin gave the correct password
  and still owes a second factor. Tied to one browser, expires after 5 minutes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can set up an authenticator app, including saving recovery codes, in under
  2 minutes.
- **SC-002**: With 2FA on, sign-in with the correct password but no valid second factor never
  starts a session (0 successes in testing of wrong, reused, expired and missing codes).
- **SC-003**: Sign-in with a passkey on a supported phone or laptop takes under 10 seconds after
  the password is entered.
- **SC-004**: An admin who loses their only device regains access in under 5 minutes using a
  recovery code, another admin, or the server-side reset.
- **SC-005**: Machine clients using API tokens see no change: 100% of existing token-based
  requests behave as before.
- **SC-006**: The installed admin app (PWA) and a normal browser tab both support every sign-in
  path that the device supports.

## Assumptions

- Admins reach the admin app through the reverse proxy over HTTPS using a host name. Passkeys need
  this; access by IP address or plain HTTP supports authenticator codes only.
- Biometrics means the device's own fingerprint or face check, reached through passkeys. There is
  no separate "biometric" factor, and no biometric data reaches the app.
- No "remember this device" option: every sign-in asks for the second factor. Sessions keep their
  current limits (1 hour idle, 30 days maximum).
- SMS and email codes are out of scope: the app sends neither.
- The existing encryption key used for third-party secrets is reused to protect authenticator
  secrets.
- The existing per-username sign-in rate limit is reused for the second step; no new limit is
  added.
- Signing in counts as proving identity for 5 minutes: right after sign-in (password, plus a
  second factor if the admin has one), the changes in FR-023, FR-027 and FR-037 need no new prompt.
  After that, they ask again.
