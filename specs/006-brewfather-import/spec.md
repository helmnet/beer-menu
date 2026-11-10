# Feature Specification: Brewfather Import

**Feature Branch**: `compmodder26/feature_6`

**Created**: 2026-11-09

**Status**: Draft

**Input**: User description: "Feature 6: Brewfather import. Depends on feature 3 (beer library). Spec dir specs/006-brewfather-import. Brief source: docs/project-brief.md "Integrations" > "Brewfather (MVP)" and "Provider interface", "Coming soon", security bullets, and row 6 of "Spec-kit feature breakdown". Admin connects a Brewfather account (user ID + API key entered in the admin app), lists batches, imports selected ones as beers (name, style, ABV, IBU, color as SRM, brew date, notes into description, batch status), re-syncs an imported beer on demand (e.g. final ABV after fermentation), and sets a scheduled sync from the admin app. Pull-only. Respect Brewfather rate limits and cache responses; the brief's API details (v2 API, Basic auth with user ID and key, ~500 calls/hour) are from memory and MUST be verified against current Brewfather docs in research. Integrations sit behind a Go provider interface (e.g. ListBatches, GetBatch returning a common batch type); the core never sees provider-specific fields (constitution II). Security (constitution V): the Brewfather key is encrypted at rest with an encryption key held outside the DB (env var or Docker secret file), never returned by the API, never logged; backups hold only the encrypted key, so a restore without the encryption key loses the connection; imported data is validated like any other input. Existing model: beers already have source ('manual'|'brewfather') and source_ref columns (migration 0003); beer statuses are planned, fermenting, conditioning, ready, and Brewfather statuses (Planning, Brewing, Fermenting, Conditioning, Completed, Archived) must be mapped onto them. Feature 5's TV on-deck list shows fermenting/conditioning beers with no keg, so imported/re-synced status changes what the TV shows ("status feeds coming soon"). Spec must decide: where the encryption key comes from and behaviour when missing or wrong; which fields re-sync overwrites vs keeps local edits; duplicate import of the same batch; what happens to an imported beer whose batch disappears; scheduled sync interval and that the scheduler runs inside the API process (no extra container); outbound HTTP timeouts, rate-limit handling and caching; access level of each route (connect/disconnect, list batches, import, re-sync, schedule) per the route table conventions; settings storage via new migration 0005 in a shape feature 7's display settings can share; tests use a fake provider / httptest server and never a real key. Admin UI: a Brewfather panel in the existing admin page (admin app styling is out of scope; a separate feature will restyle the admin app later)."

## Context

The owner plans and tracks brews in Brewfather. Today every beer is typed into the admin app by
hand. This feature lets the owner connect their Brewfather account, pick batches to import as
beers, and keep those beers current: final ABV after fermentation, and a status that moves from
fermenting to conditioning to ready. Because the TV's on-deck page (feature 5) shows fermenting
and conditioning beers, a connected account keeps "coming soon" up to date with no typing.

Import is pull-only: this app never writes to Brewfather. Brewfather is the first of possibly
several sources, so the import sits behind a provider boundary; the rest of the app sees only a
common batch shape, never Brewfather-specific fields.

Brewfather facts this spec relies on, checked against the current Brewfather API docs
(docs.brewfather.app/api, 2026-11-09): the v2 API uses HTTP Basic auth with the account's user
ID and an API key; keys carry scopes and this feature needs only `batches.read`; each key may
make 500 calls per hour, and going over returns "429 Too Many Requests" with a `Retry-After`
header; list calls return at most 50 batches per page. Batch status is one of Planning,
Brewing, Fermenting, Conditioning, Completed, Archived. Batch color is given in SRM and the brew
date as a Unix time in milliseconds.

## Clarifications

### Session 2026-11-09

- Q: Which fields does a re-sync overwrite? A: Measurements only: ABV, IBU, SRM, brew date and
  status. Name, style, description and brewer are copied once at import and then belong to the
  owner; re-sync never changes them.
- Q: What does the scheduled sync do? A: It re-syncs every imported beer and also imports new
  batches that are Brewing, Fermenting or Conditioning, so the on-deck page fills itself.
- Q: What happens when a batch is archived in Brewfather? A: The next sync archives the local
  beer too. It archives once, when the batch moves to Archived; a beer the owner unarchives
  locally is not archived again. A beer with a current keg cannot be archived (feature 4), so
  the archive waits, with a note in the beer list, until the keg is gone.
- Q: Are archived Brewfather batches imported? A: No. The owner has hundreds of archived
  batches, so the batch list and every sync read only batches that are not archived, and an
  archived batch cannot be imported. Completed maps to ready. Archiving a batch in Brewfather
  still archives a beer already imported from it: sync looks up each imported beer missing from
  the active list on its own to tell "archived" from "deleted".
- Q: How is a beer tied to its batch? A: By the batch's permanent Brewfather ID, stored as the
  beer's source reference (feature 3 already has the column). Renaming either side does not
  break the link.
- Q: What is the encryption key for, given the owner only enters a user ID and API key? A: It
  is a server setting the deployer adds once (like the cookie setting), not something typed in
  the admin app. It encrypts the stored Brewfather key so a copy of the database or a backup
  does not reveal it (constitution V). Losing it only means entering the Brewfather key again.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect a Brewfather account (Priority: P1)

The owner opens the Brewfather panel in the admin app, enters their Brewfather user ID and an
API key, and saves. The app checks the credentials with Brewfather before keeping them. The
panel then shows "Connected". The key is never shown again, by the panel or by any API
response. The owner can disconnect at any time, which forgets the credentials.

**Why this priority**: nothing else in this feature works without a connection, and it carries
the feature's security weight (a third-party secret at rest).

**Independent Test**: with the encryption key configured and a fake Brewfather server, connect
with valid credentials and confirm the status reads connected, the stored key is not readable
in the database file or any backup, and no API response or log line contains it. Disconnect and
confirm the credentials are gone.

**Acceptance Scenarios**:

1. **Given** the encryption key is configured and no account is connected, **When** an admin
   submits a valid user ID and API key, **Then** the app confirms them with Brewfather, stores
   them, and the status shows connected with no part of the key in the response.
2. **Given** an admin submits a wrong user ID or key, or a key without batch read permission,
   **When** Brewfather refuses it, **Then** nothing is stored and the admin sees a message
   saying the credentials were refused (or lack batch access), without echoing the key.
3. **Given** Brewfather cannot be reached when connecting, **When** the check times out,
   **Then** nothing is stored and the admin sees "Brewfather could not be reached; try again".
4. **Given** an account is connected, **When** an admin disconnects, **Then** the stored
   credentials are deleted, scheduled sync stops, and imported beers stay in the library
   unchanged. The chosen interval is kept, so scheduled sync resumes after a later connect.
5. **Given** an account is connected, **When** an admin connects again with new credentials,
   **Then** the new ones replace the old ones.
6. **Given** the encryption key is not configured, **When** an admin opens the Brewfather panel,
   **Then** it explains that Brewfather import is not set up on the server and names the setting
   the deployer must add; connecting is refused, and every other part of the app works.
7. **Given** a database restored on a server whose encryption key differs from the one that
   stored the credentials, **When** the app reads the connection, **Then** the status shows
   "Reconnect needed", syncing stops, and the admin can connect again; the app does not crash
   and does not log the stored value.

---

### User Story 2 - Import batches as beers (Priority: P1)

The owner opens the batch list in the Brewfather panel and sees their batches with name, batch
number, status and brew date, newest first. Batches already imported are marked and link to
their beer. The owner picks one or more batches and imports them. Each becomes a beer with
source "Brewfather": name, style, ABV, IBU, color, brew date, description from the batch notes,
and a status mapped from the batch status.

**Why this priority**: importing is the point of the integration; it saves typing every beer.

**Independent Test**: with a fake Brewfather account of four batches (Planning, Fermenting,
Completed and one with missing values), import three and confirm each beer's fields match the
batches by the mapping rules, the missing values are empty rather than zero, and importing the
same batch again does not create a second beer.

**Acceptance Scenarios**:

1. **Given** a connected account, **When** the admin opens the batch list, **Then** every
   batch that is not archived in Brewfather shows with name, batch number, status and brew
   date, newest brew date first, and batches already imported are marked as such. Archived
   batches are not listed.
2. **Given** a Fermenting batch "Hazy IPA" with style "New England IPA", measured ABV 6.4,
   estimated IBU 45, estimated color 5 SRM and brew date 2026-10-30, **When** the admin imports
   it, **Then** a beer "Hazy IPA" is created with those values, status fermenting, source
   "Brewfather", linked to that batch.
3. **Given** a batch with no measured ABV yet, **When** it is imported, **Then** the beer's ABV
   is the recipe's estimated ABV.
4. **Given** a batch with tasting notes, **When** it is imported, **Then** the beer's description
   is the tasting notes; with no tasting notes it is the general batch notes; with neither it is
   empty.
5. **Given** a batch that is already imported, **When** the admin imports it again, **Then** no
   second beer is created and the admin is told it is already imported, with a link to the beer.
6. **Given** the admin selects several batches, **When** they import them, **Then** each is
   imported or reported on its own; one failure does not stop the others, and the result lists
   which were imported and which were not, with the reason.
7. **Given** a batch whose values break the beer library's rules (such as ABV 40, a name of 300
   characters, or control characters in notes), **When** it is imported, **Then** the beer is
   still created, with each bad value cleaned or left empty by the rules in FR-014, and the
   import result notes which fields were adjusted.
8. **Given** a batch that is archived in Brewfather, **When** a client asks to import it by
   its ID, **Then** no beer is created and the result says archived batches are not imported.
9. **Given** a Fermenting or Conditioning batch is imported and has no keg, **When** the TV
   display refreshes, **Then** the beer shows on the on-deck page with its status.

---

### User Story 3 - Re-sync an imported beer (Priority: P2)

After fermentation the owner records final gravity in Brewfather, and the measured ABV changes.
In the admin app the owner re-syncs that beer, and its ABV, IBU, color, brew date and status
update. Any name, style, description or brewer the owner set locally stays as it is.

**Why this priority**: the brief's example (final ABV after fermentation); without it imported
beers go stale. The schedule (story 4) covers the same need without a click.

**Independent Test**: import a batch, edit the beer's name and description locally, change the
fake batch's ABV, status and name, re-sync, and confirm ABV and status changed while the local
name and description stayed.

**Acceptance Scenarios**:

1. **Given** an imported beer whose batch's measured ABV went from empty to 6.1, **When** the
   admin re-syncs it, **Then** the beer's ABV becomes 6.1.
2. **Given** an imported beer renamed locally, **When** it is re-synced, **Then** the local
   name, style, description and brewer are unchanged, even if they differ from Brewfather.
3. **Given** the batch status moved from Fermenting to Conditioning, **When** the beer is
   re-synced, **Then** the beer's status becomes conditioning, even if the owner had set a
   different status locally.
4. **Given** the batch was deleted in Brewfather or belongs to an account no longer connected,
   **When** the beer is re-synced, **Then** the beer is left unchanged and marked "Not found in
   Brewfather"; it is not deleted or archived.
5. **Given** a manually created beer, **When** a re-sync is requested for it, **Then** it is
   refused because the beer has no Brewfather batch.
6. **Given** Brewfather says the rate limit is used up, **When** a re-sync is requested,
   **Then** nothing changes and the admin is told when Brewfather will accept calls again.
7. **Given** an imported beer with no current keg whose batch moved to Archived, **When** it is
   re-synced, **Then** the beer is archived and leaves the TV and the default beer list.
8. **Given** an imported beer with a current keg whose batch moved to Archived, **When** it is
   re-synced, **Then** its other fields update, it stays unarchived, and the beer list shows
   "Archived in Brewfather; kick its keg to archive". Once the keg is kicked or deleted, the
   next sync archives it.
9. **Given** a beer archived by sync that the owner unarchives locally, **When** later syncs run
   while the batch is still Archived, **Then** the beer is not archived again.

---

### User Story 4 - Scheduled sync (Priority: P2)

The owner picks a sync interval in the Brewfather panel. On that interval the app, by itself,
re-syncs every imported beer and imports new batches that are Brewing, Fermenting or
Conditioning. The panel shows when the last sync ran, what it did and whether it failed. The
owner can also run the same sync at once with "Sync now".

**Why this priority**: keeps the TV's on-deck page current with no clicks; builds on stories
2 and 3.

**Independent Test**: set the interval to the shortest option against a fake Brewfather server,
add a new Fermenting batch and change an imported batch's status, wait one interval, and
confirm the new batch is imported, the status changed, and the panel shows the sync time and
counts. Turn the schedule off and confirm no further syncs run.

**Acceptance Scenarios**:

1. **Given** the schedule is off (the default), **When** time passes, **Then** no automatic
   sync calls Brewfather.
2. **Given** the schedule is set to an interval, **When** the interval passes, **Then** a sync
   runs inside the API server, with no other container or outside scheduler.
3. **Given** a sync runs, **When** it finds a Brewing, Fermenting or Conditioning batch not yet
   imported, **Then** it imports it as in story 2; Planning, Completed and Archived batches are
   not auto-imported.
4. **Given** the owner deleted a beer that came from Brewfather, **When** later syncs run,
   **Then** that batch is not imported again automatically; the owner can still import it by
   hand from the batch list.
5. **Given** a sync runs, **When** it finishes, **Then** the panel shows its time, how many
   beers were updated and imported, and any failure in plain words.
6. **Given** a sync is running, **When** another sync is requested by hand or by the schedule,
   **Then** the second does not start, and a hand request is told a sync is already running.
7. **Given** the API server restarts, **When** it comes back, **Then** the schedule carries on
   with the saved interval, without anyone setting it again.
8. **Given** Brewfather is down or rate limited during a scheduled sync, **When** the sync fails,
   **Then** nothing is half-changed for any one beer, the failure is shown in the panel, and the
   next sync runs no earlier than Brewfather's stated retry time.
9. **Given** an imported beer is archived locally, **When** a sync runs, **Then** that beer is not
   updated and its batch is not imported again.

---

### Edge Cases

- **Many batches**: archived batches are never read in bulk, so hundreds of them cost nothing.
  More than 50 batches in one status are read in pages; the batch list and the sync see every
  batch that is not archived.
- **Brewfather slow or unreachable**: each call to Brewfather gives up after 15 seconds; the
  admin sees a "could not reach Brewfather" message and nothing is changed.
- **Rate limit**: when Brewfather answers 429, the app makes no more calls to it until the
  `Retry-After` time has passed, for hand and scheduled actions alike.
- **Repeated list views**: opening the batch list again within 5 minutes shows the saved copy
  without calling Brewfather; the panel has a "Refresh" action that fetches a fresh copy.
  Import and re-sync always read the batch fresh.
- **Batch with no name, or with Brewfather's default name "Batch"**: the beer is named after
  the recipe; with no recipe name, "Batch <number>"; with no number either, "Brewfather batch".
  A batch the owner renamed in Brewfather keeps its name.
- **Fractional IBU**: rounded to the nearest whole number. **ABV and SRM with more decimals**:
  rounded to one decimal place.
- **Brew date**: converted from Brewfather's millisecond time to a calendar date in UTC (the
  server image has no time zone data); a missing or zero brew date leaves the beer's brew date
  empty.
- **Beer on tap gets re-synced**: allowed. A status change (such as Completed to ready) does not
  touch the keg or tap. A fermenting beer that is also on a tap shows on the TV as on tap, as in
  feature 5.
- **Batch unarchived in Brewfather**: the local beer stays archived (archived beers are not
  synced); the owner unarchives it by hand.
- **Owner edits a re-synced field locally**: allowed; the next re-sync overwrites ABV, IBU, SRM,
  brew date and status with Brewfather's values.
- **Two beers claim one batch**: impossible; each batch links to at most one beer.
- **Beer deleted while a sync runs**: the sync skips it without error.
- **Account switched**: after connecting a different account, beers from the old account's
  batches show "Not found in Brewfather" on their next sync and are otherwise left alone.
- **Encryption key set but malformed** (wrong length or not decodable): the API refuses to start
  and prints a clear configuration error that does not include the value.
- **Brewfather returns unexpected data** (missing fields, wrong types, oversized text): the batch
  is treated as untrusted input; bad fields are left empty or trimmed by FR-014, and a response
  that cannot be read at all fails that one call with a clear message.

## Requirements *(mandatory)*

### Functional Requirements

**Connection and secrets**

- **FR-001**: Admins MUST be able to connect one Brewfather account by entering a user ID and an
  API key. Only one account is connected at a time.
- **FR-002**: Before storing credentials, the system MUST confirm them with one Brewfather call
  that needs batch read access, and MUST store nothing if that call fails.
- **FR-003**: The API key MUST be stored only in encrypted form, encrypted with authenticated
  encryption using a key that is not in the database.
- **FR-004**: The encryption key MUST come from the deployer, as an environment variable or as a
  file path pointing to a Docker secret. If both are set, the file wins. The repo's example
  compose and docs MUST show how to generate and supply it, with no real value.
- **FR-005**: The API MUST never return the API key, in whole or in part, in any response, and
  MUST never write it, the encryption key or the Basic auth header to logs or error messages.
- **FR-006**: When no encryption key is configured, the API MUST start and serve everything
  else; the Brewfather status MUST say it is not configured, and connecting MUST be refused.
- **FR-007**: When the stored key cannot be decrypted (different encryption key, corrupted
  value), the status MUST report "reconnect needed", syncing MUST stop, and connecting again
  MUST replace the stored value.
- **FR-008**: Admins MUST be able to disconnect, which deletes the stored credentials and stops
  scheduled sync. Imported beers MUST stay. The sync interval MUST be kept, and scheduled sync
  MUST resume once an account is connected again.
- **FR-009**: The connection status MUST report: whether import is configured on the server,
  whether an account is connected, whether a reconnect is needed, the sync interval, and the
  last sync's time, outcome and counts. It MUST NOT include the user ID or any part of the key.

**Batches and import**

- **FR-010**: The system MUST list every batch in the connected account that is not archived in
  Brewfather, reading all pages, with name, batch number, status, brew date and whether it is
  already imported (and as which beer). Archived batches MUST NOT be read in bulk or listed.
- **FR-011**: Admins and write-scope clients MUST be able to import one or more batches by
  Brewfather batch ID. Each import MUST read the batch fresh and create one beer with source
  "Brewfather" and a link to the batch (its Brewfather batch ID as the beer's source
  reference). A batch that is archived in Brewfather MUST NOT be imported; its result MUST say
  so.
- **FR-012**: The system MUST map batch fields onto a beer as follows: name from the batch name
  (a blank name or Brewfather's default name "Batch", in any case, counts as no name; else
  recipe name, else "Batch <number>", else "Brewfather batch"); style from the recipe
  style name; ABV from measured ABV, else the recipe's estimated ABV; IBU from the batch's
  estimated IBU; SRM from the batch's estimated color; brew date from the batch brew date;
  description from tasting notes, else general batch notes; brewer left empty.
- **FR-013**: The system MUST map batch status to beer status: Planning to planned; Brewing,
  Fermenting to fermenting; Conditioning to conditioning; Completed, Archived to ready.
  Archived also archives the beer, by FR-017a.
- **FR-014**: Imported values MUST pass the beer library's existing rules. A value that breaks
  them MUST be cleaned where a clean form exists (trim whitespace, strip control characters,
  shorten text to the field's limit, round numbers) and otherwise left empty (for example ABV
  above 25 or SRM above 100). An unusable name falls back as in FR-012. The import result MUST
  list each adjusted field by name.
- **FR-015**: A batch MUST link to at most one beer. Importing an already-imported batch MUST NOT
  create a beer and MUST report the existing beer.
- **FR-016**: A multi-batch import MUST report a result per batch; one batch failing MUST NOT
  stop or undo the others.

**Re-sync and schedule**

- **FR-017**: Admins and write-scope clients MUST be able to re-sync one imported beer. Re-sync
  MUST read its batch fresh and overwrite only ABV, IBU, SRM, brew date and status, using the
  rules in FR-012 to FR-014. It MUST NOT change name, style, description or brewer, and MUST
  change archive state only by FR-017a.
- **FR-017a**: When a re-sync (by hand or in a sync) sees the batch status change to Archived
  since the beer's last sync, it MUST archive the beer. If the beer has a current keg, the
  archive MUST wait, the beer MUST show "archive pending" with the reason, and each later sync
  MUST try again until it succeeds. A beer the owner unarchives after that MUST NOT be archived
  again while the batch stays Archived. Sync MUST never unarchive a beer.
- **FR-018**: Re-sync of a beer whose batch Brewfather reports as missing MUST leave the beer
  unchanged and mark it "not found in Brewfather" until a later sync finds the batch again.
- **FR-019**: Re-sync MUST be refused for a beer that did not come from Brewfather.
- **FR-020**: Admins MUST be able to set the sync interval to off, every 15 minutes, every hour,
  every 6 hours or every 24 hours. The default is off. The setting MUST survive restarts.
- **FR-021**: A sync (scheduled or "Sync now") MUST read the batches that are not archived,
  re-sync every imported, non-archived beer whose batch is among them, and import every
  Brewing, Fermenting or Conditioning batch that is not yet linked to a beer and not dismissed
  (FR-022). For each imported, non-archived beer whose batch is not among them, the sync MUST
  read that one batch: if Brewfather has it as Archived, apply FR-017 and FR-017a; if
  Brewfather does not have it, apply FR-018. A beer already marked "not found in Brewfather"
  MUST NOT be read on its own by a sync (only a hand re-sync checks it again), so deleted
  batches or a switched account do not cost calls on every sync. It is cleared if its batch
  shows up in the list again.
- **FR-022**: Deleting a beer that came from Brewfather MUST record its batch as dismissed, so
  sync does not import it again. Importing that batch by hand MUST clear the dismissal.
- **FR-023**: The scheduler MUST run inside the API server process. Only one sync may run at a
  time; a hand request during a sync MUST be refused with a "sync already running" error.
- **FR-024**: Each beer's update in a sync MUST be all or nothing. A failed sync MUST record its
  time and a plain-words reason for the status, without the key or Brewfather's raw response.

**Calls to Brewfather**

- **FR-025**: Every call to Brewfather MUST use HTTPS, give up after 15 seconds, and read at most
  a bounded response size, so a slow or hostile server cannot hang or exhaust the API.
- **FR-025a**: Every request that waits on Brewfather while the admin waits (connect, batch list,
  import, re-sync) MUST answer within 20 seconds in total. When that time runs out, or when
  Brewfather cannot be reached during a multi-batch import, the remaining batches MUST NOT be
  tried and MUST each report "not attempted". Background syncs are not bound by this limit.
- **FR-026**: On a 429 answer, the system MUST stop calling Brewfather until the `Retry-After`
  time (or 60 seconds if absent or unreadable) and MUST tell a hand request when to retry. A
  hand request made while the limit is already in force MUST be refused at once with the
  retry time, without calling Brewfather.
- **FR-027**: A whole sync MUST read the batches that are not archived through paged list calls,
  not one call per beer, plus one call per imported beer whose batch is missing from that list,
  so that a sync of an account with up to 200 batches that are not archived uses at most 10
  calls, however many batches are archived, and beers already not found add no calls. The batch list view MUST be cached
  for 5 minutes, with a refresh action that bypasses the cache.
- **FR-028**: The rest of the app MUST see Brewfather only through a provider boundary that
  returns a common batch shape (ID, name, batch number, status as one of the four beer
  statuses, style, ABV, IBU, SRM, brew date, notes). No Brewfather field names MUST leak past it.

**Access**

- **FR-029**: Connect, disconnect and setting the sync interval MUST need an admin session (not
  an API token), since they handle the third-party secret and account choice.
- **FR-030**: Listing batches, importing, re-syncing a beer and "Sync now" MUST need an admin
  session or a write-scope token. Read-scope tokens MUST be refused, because each call spends
  the owner's Brewfather quota.
- **FR-031**: Reading the connection status MUST need an admin session or any token (read or
  write).
- **FR-032**: Every new route MUST follow the existing authentication and CSRF rules and appear in
  the API description with its access level.

**Admin app**

- **FR-033**: The admin page MUST have a Brewfather panel with: the connection state and a
  connect form (user ID, and API key as a password field that is cleared after submit);
  disconnect; the sync interval; "Sync now" with the last sync result; and the batch list with
  import selection, imported markers and refresh.
- **FR-034**: The beer list MUST show which beers came from Brewfather, a re-sync action for
  them, and "Not found in Brewfather" where it applies. After an import or sync, the beer list
  MUST refresh.
- **FR-035**: Admin app styling is out of scope; the panel uses the same unstyled markup as the
  existing panels.

**Storage and tests**

- **FR-036**: Connection details, the sync interval, the last sync result and dismissed batches
  MUST be stored through one schema migration. Settings MUST use a general key-value settings
  store that feature 7's display settings can reuse.
- **FR-037**: Tests MUST use a fake Brewfather server or fake provider and MUST NOT contain or
  need a real Brewfather key. A test MUST show that the stored key is not present in plain form
  in the database file or a backup.

### Key Entities

- **Settings**: general key-value settings for the app. Holds the Brewfather sync interval and,
  later, feature 7's display settings.
- **Provider connection**: one per provider (only Brewfather now): the user ID and encrypted API
  key, when connected, and the last sync's time, outcome, reason and counts.
- **Common batch**: the provider-neutral view of a batch: provider batch ID, name, batch number,
  mapped status, style, ABV, IBU, SRM, brew date and notes. Never stored as is.
- **Beer (existing)**: gains a sync state for imported beers (last synced time, last seen
  batch status, "not found in Brewfather", "archive pending"). Its existing source ("brewfather") and source reference (the batch ID) link it
  to its batch.
- **Dismissed batch**: a provider batch ID whose beer the owner deleted, so sync skips it.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An owner can connect Brewfather and import a batch as a beer in under 2 minutes,
  with no field typed except the user ID and key.
- **SC-002**: With a sync interval set, a batch that moves to Fermenting in Brewfather shows on
  the TV's on-deck page within one interval plus 1 minute, with no action in the admin app.
- **SC-003**: After a re-sync, 100% of imported beers have Brewfather's current ABV, IBU, color,
  brew date and status, and 100% of locally edited names, styles, descriptions and brewers are
  kept.
- **SC-004**: No API response, log line, database file or backup contains the Brewfather key in
  plain form, as checked by tests.
- **SC-005**: A full sync of an account with 200 batches that are not archived (and any number
  that are) makes no more than 10 calls to Brewfather, and the 15-minute schedule uses under 10% of the hourly allowance on such an
  account.
- **SC-006**: With Brewfather down or rate limited, every other admin and TV function keeps
  working, and admin actions that need Brewfather fail within 20 seconds with a clear message.
- **SC-007**: Importing the same batch any number of times, by hand or by sync, never makes more
  than one beer.

## Assumptions

- Single owner, single Brewfather account. Multiple accounts or providers at once are out of
  scope; the provider boundary keeps a second source possible later (constitution VII).
- The owner creates a Brewfather API key with the `batches.read` scope. Recipes and inventory are
  not read.
- Pull-only: the app never writes to Brewfather.
- The encryption key is 32 random bytes supplied base64-encoded; the docs give a one-line
  command to generate it. Losing it means reconnecting Brewfather, nothing more: imported beers
  are ordinary beers.
- Batch notes (the timestamped log) are not imported; only tasting notes or general batch notes.
- Brewing maps to fermenting because wort is in the fermenter by the end of brew day; planned
  batches stay off the TV, as in feature 5.
- Archived batches are never listed or imported (owner decision; the owner has hundreds). An
  archived batch can still be read one at a time, to follow an imported beer into archive.
- Archived in Brewfather archives the local beer (owner decision). Because feature 4 refuses to
  archive a beer with a current keg, a batch archived while its keg is still on tap waits until
  the keg is kicked.
- Deleting a beer keeps its existing rules (refused while it has kegs, feature 4).
- The batch list cache lives in memory and is lost on restart; that only costs one extra call.
- Admin app styling is deferred to a later feature.
