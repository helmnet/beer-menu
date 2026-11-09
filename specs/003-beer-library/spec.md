# Feature Specification: Beer Library

**Feature Branch**: `compmodder26/feature_3`

**Created**: 2026-11-09

**Status**: Draft

**Input** (label image upload and public reads superseded, see Clarifications): User description: "Feature 3: Beer library. Depends on feature 2 (auth and API tokens). Admins can create, edit, archive, unarchive and delete beers in the API and the admin app. Each beer has: name (required), style, ABV, IBU, SRM (color), description, brewer, brew date, optional label image, status (planned / fermenting / conditioning / ready), source (manual / brewfather) and source_ref (link to the Brewfather batch, so feature 6 can re-sync it later; this feature only creates manual beers but the fields must exist). Archived beers are hidden from the default list but kept; delete is permanent. Label image upload: admin uploads an image file for a beer, with its own size limit and content-type check (images only), stored in the app's data directory and served by this app (same origin, since the display CSP is img-src 'self'). Beer list and beer detail GET routes are public menu reads per constitution v1.2.0 principle V; every write needs an admin session or a write-scope API token, so Home Assistant and scripts can manage beers. The TV display (features 5 and 7) will later show beer color and label art, so SRM and image must be readable from the public API. Brief source: docs/project-brief.md, "Functional scope (MVP)" > "Beer library", the beers row in "Data model sketch", and row 3 of "Spec-kit feature breakdown"."

## Context

Features 1 and 2 delivered a running app with admins, sessions and API tokens, but no menu data.
This feature adds the first menu data: the beer library. Every later menu feature builds on it.
Feature 4 puts beers on taps. Feature 5 shows them on the TV. Feature 6 imports and re-syncs
beers from Brewfather.

Beers have no uploaded images. Each beer is pictured by generated artwork filled with the beer's
color, taken from its SRM: a glass for beers on a tap, and a keg for every other beer. Taps
arrive in feature 4, so in this feature every beer is a keg. This feature builds that artwork and shows it in the admin app. Feature 5 reuses
it on the TV.

The access rules are fixed by constitution principle V, feature 2 and the owner's decision of
2026-11-09:

- Reading beers needs an admin session or any API token (`read` or `write`). The full library is
  not a menu read. Only what the TV shows is public, and that endpoint comes in feature 5.
- Every change to beers needs an admin session or a `write` API token.
- Browser writes refuse cross-site requests.

## Clarifications

### Session 2026-11-09

- Q: Should beers have uploaded label images? A: No. There are no uploads. Each beer is shown as
  generated artwork filled with its SRM color: a keg for coming-soon beers and a glass for beers
  on tap. The owner's reference is taplist.io's tap list and on-deck views; the artwork here must
  be original drawings, not copies.
- Q: Is a beer whose status is ready drawn as a glass? A: No. A beer is a keg until it is
  actually on a tap; only beers on a tap show the pint glass. Taps arrive in feature 4, so in
  this feature every beer is a keg, whatever its status. The glass drawing is still built here
  for feature 4 to use.
- Q: Are the beer list, a single beer and the color table public menu reads? A: No (owner
  decision). The full library is not a menu read: it holds planned and archived beers the TV
  never shows. These reads need an admin session or any API token. Feature 5 adds a public
  `GET /api/menu` that returns only the on-tap and coming-soon beers with the fields the display
  shows, including color.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Add and edit beers (Priority: P1)

The owner signs in to the admin app and adds a beer they have brewed. They enter its name and any
of its details: style, ABV, IBU, color (SRM), description, brewer, brew date and status. Later
they correct a detail, such as the final ABV after fermentation. The beer list in the admin app
shows every current beer.

**Why this priority**: the library is empty until beers can be added. Taps, the display and the
coming-soon list all need beers to exist first.

**Independent Test**: sign in, add a beer with only a name, add a second with every field, edit
one field of the first, and confirm both beers and the edit appear in the admin list and in a
read with a `read` token.

**Acceptance Scenarios**:

1. **Given** a signed-in admin, **When** they add a beer with a name only, **Then** the beer is
   saved with status "ready" and source "manual", and appears in the list.
2. **Given** a signed-in admin, **When** they add a beer with every field filled in, **Then**
   each value is saved and shown back exactly as entered.
3. **Given** an existing beer, **When** an admin changes its ABV and saves, **Then** the new ABV
   is shown and no other field changes.
4. **Given** the add form, **When** an admin submits an empty name or an out-of-range value
   (such as ABV 150), **Then** the beer is not saved and the form says which field is wrong.
5. **Given** a script holding a `write` token, **When** it creates or edits a beer through the
   API, **Then** the change succeeds as it would for a signed-in admin.

---

### User Story 2 - Read beers with any credential (Priority: P1)

A script or device holding a `read` token, such as Home Assistant, reads the beer list and a
single beer's details. A signed-in admin and a `write` token can read them too. The response
includes the beer's SRM and the color it maps to. A client with no credential is refused: the
full library is not public.

**Why this priority**: scripts and the admin app need the full library, and principle I requires
the read contract to stay stable. The TV's public menu endpoint comes in feature 5.

**Independent Test**: with beers in the library, request the list and one beer with a `read`
token, and confirm every field, including SRM and its color, comes back. Request them with no
credential and confirm 401. Then attempt a create, edit, archive and delete with no credential
and with a `read` token, and confirm all are refused.

**Acceptance Scenarios**:

1. **Given** beers in the library, **When** a client with a `read` token requests the beer list,
   **Then** it receives every beer that is not archived, with all their fields.
2. **Given** a beer with an SRM, **When** a client with a `read` token reads that beer, **Then**
   the response includes the SRM and the display color it maps to.
3. **Given** no credential, or a revoked token, **When** a client reads the beer list, a beer or
   the color table, **Then** it is refused with 401.
4. **Given** no credential, or a `read` token, **When** a client tries any beer change, **Then**
   it is refused and nothing changes.
5. **Given** a beer that does not exist, **When** a client reads it, **Then** it receives a
   clear "not found" answer.

---

### User Story 3 - See each beer's artwork (Priority: P2)

The owner looks at the beer library and sees each beer drawn in its own color. A beer is a keg
until it is on a tap; taps arrive in feature 4, so here every beer is a keg. While editing a
beer, the artwork updates as they change the SRM, so they see the color the TV will show.

**Why this priority**: the artwork makes the menu look finished and lets the owner check colors,
but beers are usable on the menu without it.

**Independent Test**: add a pale beer (SRM 4) that is ready and a dark beer (SRM 35) that is
fermenting, and confirm the first shows as a pale-filled keg and the second as a dark-filled
keg. Edit the SRM in the form and confirm the preview color changes before saving.

**Acceptance Scenarios**:

1. **Given** a beer with status ready, **When** it is shown in the admin app, **Then** it is
   drawn as a keg filled with the color for its SRM, because it is not on a tap.
2. **Given** a beer with status planned, fermenting or conditioning, **When** it is shown in the
   admin app, **Then** it is drawn as a keg whose middle is filled with the color for its SRM.
3. **Given** a beer with no SRM, **When** it is shown, **Then** its glass or keg is drawn empty,
   not filled with a guessed color.
4. **Given** the edit form, **When** the admin changes the SRM, **Then** the artwork preview
   color updates at once, before saving. Changing the status does not change the vessel.
5. **Given** two beers with SRM 4 and SRM 35, **When** both are shown, **Then** the first is
   visibly pale gold and the second visibly near-black.

---

### User Story 4 - Archive and restore beers (Priority: P2)

The owner has a beer they will not brew again but wants to keep on record. They archive it. It
leaves the default beer list, but they can still see archived beers on request and restore one
if they brew it again.

**Why this priority**: the library grows with every batch. Archiving keeps lists short without
losing history, but it matters only once there are many beers.

**Independent Test**: archive a beer, confirm it is gone from the default list and present in the
archived view, then restore it and confirm it is back in the default list unchanged.

**Acceptance Scenarios**:

1. **Given** a current beer, **When** an admin archives it, **Then** it no longer appears in the
   default list, for admins or token holders.
2. **Given** archived beers, **When** an admin asks to see archived beers, **Then** they are
   listed, each marked as archived.
3. **Given** an archived beer, **When** an admin restores it, **Then** it appears in the default
   list again with all its details unchanged.
4. **Given** an archived beer, **When** a client reads it by its identifier, **Then** it is
   returned and marked as archived.

---

### User Story 5 - Delete a beer (Priority: P3)

The owner added a beer by mistake, or a duplicate. They delete it, after confirming, and it is
gone for good.

**Why this priority**: archiving covers most cases. Delete is for mistakes.

**Independent Test**: delete a beer and confirm it no longer exists, and that cancelling the
confirmation deletes nothing.

**Acceptance Scenarios**:

1. **Given** a beer, **When** an admin deletes it and confirms, **Then** the beer is removed and
   reading it returns "not found".
2. **Given** the delete prompt in the admin app, **When** the admin cancels, **Then** nothing is
   deleted.

---

### Edge Cases

- Two beers with the same name: allowed. Brewers often re-brew a recipe; each beer has its own
  identifier.
- An SRM above the darkest color in the color table (such as 60): drawn with the darkest
  color.
- An SRM with decimals (such as 4.5): rounded to the nearest whole value, so 4.4 shows the color
  for 4 and 4.5 the color for 5 (research R6).
- A request to set or change a beer's source or source reference from a client: refused as an
  unknown field, and nothing changes (research R3).
- Archiving an archived beer, or restoring a current one: has no effect and is not an error.
- Reading, editing or deleting a beer identifier that does not exist: "not found", and no change.
- Empty optional fields: stored as "no value", not as zero. A beer with no ABV shows no ABV, not
  0%.
- A brew date in the future: allowed, for planned beers.
- Text fields with non-English characters or emoji: saved and shown back unchanged.

## Requirements *(mandatory)*

### Functional Requirements

**Beer data**

- **FR-001**: The system MUST store beers with these fields: name, style, ABV, IBU, SRM,
  description, brewer, brew date, status, source, source reference, and archived time. Beers
  have no stored image.
- **FR-002**: Name MUST be required, 1 to 100 characters after trimming surrounding whitespace.
- **FR-003**: Every other descriptive field MUST be optional. An empty optional field MUST be
  stored and returned as "no value", distinct from zero or an empty string.
- **FR-004**: Values MUST be validated: ABV 0 to 25 (percent, at most one decimal place), IBU
  a whole number 0 to 200, SRM 0 to 100 (at most one decimal place), style and brewer up to 100 characters, description up to
  2000 characters, brew date a valid calendar date.
- **FR-005**: Status MUST be one of: planned, fermenting, conditioning, ready. A new beer
  defaults to ready.
- **FR-006**: Source MUST be one of: manual, brewfather. Beers created in this feature MUST
  have source manual and no source reference. Clients MUST NOT be able to set or change source or
  source reference in this feature; feature 6 sets them on import.
- **FR-007**: Each beer MUST have a stable identifier that does not change when it is edited,
  archived or restored.

**Reading (any credential)**

- **FR-010**: Listing beers, reading one beer and reading the color table MUST need an admin
  session or any API token (`read` or `write`). No credential, or an invalid or revoked token,
  MUST be refused with 401.
- **FR-011**: The list MUST exclude archived beers by default, and MUST include them only when a
  client asks for archived beers.
- **FR-012**: Reading one beer by its identifier MUST return it whether or not it is archived,
  and MUST say whether it is archived.
- **FR-013**: Every read MUST include all beer fields. When the beer has an SRM, the read MUST
  also include the display color it maps to, so every client (admin app, display, later the APK)
  draws the same color from one mapping.
- **FR-014**: The list MUST be in a stable, predictable order: by name, ignoring the case of
  the letters A to Z, then by identifier. Accented letters sort by their code, which is enough
  for a home library.

**Changing (protected)**

- **FR-020**: Creating, editing, archiving, restoring and deleting beers MUST require an admin
  session or a `write` API token. A `read`
  token or no credential MUST be refused.
- **FR-021**: Every browser change MUST refuse cross-site requests, and this MUST be tested for
  each change operation.
- **FR-022**: Editing MUST change only the fields the client sends and MUST leave others as they
  were. A client MUST be able to clear an optional field.
- **FR-023**: Archiving MUST record when the beer was archived. Restoring MUST clear it.
  Archiving an archived beer or restoring a current one MUST succeed without change.
- **FR-024**: Deleting MUST remove the beer permanently.
- **FR-025**: Invalid input MUST be refused with an error that names each invalid field, and
  MUST change nothing.

**Beer artwork**

- **FR-030**: The system MUST map SRM to a display color using one standard SRM color table,
  defined once and used by every client. An SRM above the table's range MUST use its darkest
  color.
- **FR-031**: A beer MUST be pictured as a keg unless it is on a tap, and as a glass when on a
  tap. Taps arrive in feature 4, so in this feature every beer is a keg.
- **FR-032**: The glass and keg MUST be generated drawings whose liquid area is filled with the
  beer's display color. A beer with no SRM MUST be drawn empty.
- **FR-033**: The keg drawing MUST be a 5 gallon corny keg. Other keg types are out of scope for
  this feature (see Assumptions).
- **FR-034**: The artwork MUST be drawn by the app itself, with no external image or font
  request, so it works under the display's same-origin rules and offline on the LAN.
- **FR-035**: The artwork MUST be a reusable part of the web app, sized to fit both the admin list
  and the TV display, so feature 5 reuses it unchanged.
- **FR-036**: The artwork MUST be original drawings. It MUST NOT copy taplist.io's or any other
  product's images.
- **FR-037**: The artwork MUST have a text alternative naming the beer and its vessel, for screen
  readers.

**Admin app**

- **FR-040**: The admin app MUST show a beer library for signed-in admins: a list of current
  beers with name, style, ABV, status and artwork, and a way to show archived beers.
- **FR-041**: The admin app MUST provide forms to add and edit a beer, showing field errors next
  to the field.
- **FR-042**: The add and edit forms MUST show a live artwork preview that follows the SRM field
  before saving.
- **FR-043**: The admin app MUST let the admin archive, restore and delete a beer. Delete MUST
  ask for confirmation first.
- **FR-044**: If the session ends while the admin is working, the admin app MUST return to the
  sign-in screen, as feature 2's panels do.

**Contract and data**

- **FR-050**: Every beer endpoint MUST be described in the repository's API document, including
  its access level, and the existing check that access levels match MUST cover them.
- **FR-051**: The schema change MUST be a new versioned migration, tested by applying it to a
  database at the previous version. The existing backup-before-migrate step MUST run before it.

### Key Entities

- **Beer**: a beer in the owner's library. Attributes: identifier, name, style, ABV, IBU, SRM,
  description, brewer, brew date, status (planned, fermenting, conditioning, ready), source
  (manual, brewfather), source reference (the Brewfather batch it came from, if any), archived
  time (empty when current), created and updated times. Later features link kegs (feature 4) and
  Brewfather batches (feature 6) to it.
- **SRM color table**: the fixed mapping from SRM value to display color. Not stored per beer and
  not editable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can add a beer with name, style, ABV, SRM and status in under 1 minute
  from the admin app, on a laptop or a phone.
- **SC-002**: A client with a `read` token can read the full beer list, including each beer's
  SRM and display color; with no credential it is refused.
- **SC-003**: 100% of beer change operations are refused with no credential, with a `read`
  token, and from a cross-site browser request, as shown by automated tests.
- **SC-004**: For every SRM from 0 to 40 in steps of 1, the artwork's fill matches the color table,
  as shown by automated tests, and the owner judges the colors right for at least 3 of their own
  beers.
- **SC-005**: Beer artwork loads no image or font: the drawings are part of the app. The beer
  list draws each beer from the color in its read, with no extra request; only the add/edit
  form reads the color table once, for its live preview.
- **SC-006**: With 500 beers in the library, the admin list and the list read with a token each
  load in under 1 second on the home server.
- **SC-007**: Upgrading an install from feature 2 keeps all admins and tokens, and the beer
  library starts empty and usable, with a database backup taken before the upgrade.

## Assumptions

- This feature only creates manual beers. Brewfather import, re-sync and status following come
  in feature 6, which sets source and source reference.
- Taps and kegs arrive in feature 4. Feature 4 decides what happens when a beer that is on a tap
  is archived or deleted. Until then, nothing blocks either action.
- The TV display reads no beer endpoint from this feature. Feature 5 adds a public
  `GET /api/menu` with only the on-tap and coming-soon beers and the fields the display shows,
  including color.
- The display (feature 5) decides where each beer appears (on tap or coming soon). The vessel
  depends on whether the beer is on a tap, not on its status. Feature 4 supplies that; until
  then every beer is drawn as a keg.
- The brief asks for keg artwork that shows the type of keg, with a 5 gallon corny keg by
  default. Keg type is a property of a keg, which arrives in feature 4. This feature draws only
  the corny keg; feature 4 adds other keg shapes if it adds keg types.
- One glass shape (a pint glass) is used for every beer. Per-style glassware is out of scope.
- The SRM color table is the common published SRM-to-RGB table; the plan names its source.
- Value ranges (ABV 0–25, IBU 0–200, SRM 0–100) cover real home-brewed beers, including strong
  ones, and reject typos. They can widen later without breaking clients.
- Stored text is returned as plain text. Clients must show it as text, not as markup.
- Description length must fit within the existing request body cap together with the other
  fields; the plan confirms or raises that cap for beer requests only.
- No search or paging in the MVP list. A home library stays in the hundreds of beers.
- With no uploads, the database file stays the only data to back up.
