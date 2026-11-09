# Feature Specification: Taps and Kegs

**Feature Branch**: `compmodder26/feature_4`

**Created**: 2026-11-09

**Status**: Draft

**Input**: User description: "Feature 4: Taps and kegs. Depends on feature 3 (beer library). Done when: configure taps, put a keg on a tap, update and kick kegs, keg history. Taps: configure the number of taps (at least 1, no maximum), their order (position) and labels, and enabled/disabled. Assign a beer to a tap by putting a keg of it on the tap, or mark a tap empty. Taps hold kegs only; bottles and cans come after the MVP. Data sketch: taps (id, position, label, enabled) — fixed slots on the kegerator. Kegs: kegs (id, tap_id, beer_id, size_l, remaining_l, tapped_at, kicked_at). One active keg per tap; history kept. Keg volume lives on kegs in litres (constitution II), never on beers; display units are a later setting (feature 7). Keg type (5 gallon corny by default) is recorded on the keg for artwork. Open question: should tap_id be nullable so a filled keg can wait off-tap ("on deck")? Keg levels: record keg size and volume remaining per keg. Update the level by hand from the admin app, either by a pour amount or a direct remaining value. Kick a keg (marks it empty/finished, frees the tap). Keg history lists past kegs with tapped/kicked times. Level updates must be reachable with a write-scoped API token (Home Assistant, future keg scales that update automatically after each pour). Access: all tap and keg routes are read (GET) / write (changes) access per the route table; nothing new is public (the public menu read is feature 5). Admin app gets a taps/kegs panel. The existing BeerArt component's onTap prop must be set to true when the beer has an active keg on a tap (admin beer library and beer form preview). BeerArt may need a keg-type prop."

## Context

Feature 3 delivered the beer library. This feature puts those beers on the kegerator: the owner
configures the taps, puts a keg of a beer on a tap, keeps its level current, and kicks it when it
runs dry. Kicked kegs stay on record as keg history. Feature 5 shows taps and their keg levels on
the TV; feature 7 adds the volume unit setting.

Volumes are stored in litres (constitution II). Keg levels must be changeable with a `write` API
token, because Home Assistant and later keg scales will update them after each pour.

The access rules follow feature 3:

- Reading taps and kegs needs an admin session or any API token (`read` or `write`). The public
  menu read comes in feature 5.
- Every change needs an admin session or a `write` API token.
- Browser writes refuse cross-site requests.

## Clarifications

### Session 2026-11-09

- Q: Can a keg exist without a tap, filled and waiting? A: Yes. A keg can be filled and wait off
  a tap ("on deck"), be put on a tap later, and be taken off a tap without kicking it. A keg is
  current until kicked, whether or not it is on a tap.
- Q: Which keg types, and do they get their own artwork? A: Corny kegs (2.5, 3 and 5 gallon),
  sanke kegs (sixth, quarter and half barrel) and euro kegs (20, 30 and 50 L), each with a
  standard default size. There are two drawings: the existing corny keg for corny types, and a
  new sanke-style keg for sanke and euro types.
- Q: Which units does the admin app use until feature 7 adds the unit setting? A: US gallons for
  sizes and remaining volume, and US fluid ounces for pours. The API stays in litres; the admin
  app converts. Feature 7 makes the unit switchable.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure taps (Priority: P1)

The owner signs in and sets up the taps to match the kegerator: how many there are, their order
from left to right, and a label for each (such as "Tap 1" or "Left"). They can add a tap, remove
an empty one, reorder them, rename them, and disable a tap that is out of service.

**Why this priority**: kegs go on taps, so taps must exist first.

**Independent Test**: on a fresh upgrade, confirm one tap exists. Add two more, rename one,
move the last to the front, disable one, and confirm the list in the admin app and a read with a
`read` token show that order, those labels and that state.

**Acceptance Scenarios**:

1. **Given** an install upgraded from feature 3, **When** an admin opens the taps panel, **Then**
   exactly one tap exists, labelled "Tap 1", enabled.
2. **Given** taps exist, **When** an admin adds a tap, **Then** it is placed last, enabled, with a
   default label "Tap N" where N is its position.
3. **Given** three taps, **When** an admin moves the third to the first place, **Then** the order
   is saved and the positions stay 1, 2, 3 with no gaps.
4. **Given** one tap left, **When** an admin tries to remove it, **Then** it is refused: at least
   one tap must exist.
5. **Given** a tap with a keg on it, **When** an admin tries to remove the tap, **Then** it is
   refused until the keg is kicked or taken off.
6. **Given** a script holding a `write` token, **When** it adds, renames or reorders taps through
   the API, **Then** the change succeeds as it would for a signed-in admin.

---

### User Story 2 - Put a keg on a tap (Priority: P1)

The owner fills a keg with a beer from the library and connects it to a tap. In the admin app
they choose the tap, the beer and the keg type; size and remaining volume default to a full keg
of that type. A keg can also be filled ahead of time and left waiting off the taps, then put on a
tap when one frees up. The tap now shows that beer, and the beer library shows it as a pint glass instead
of a keg.

**Why this priority**: a tap list is taps with beers on them. This is the core of the feature
and of the menu in feature 5.

**Independent Test**: with a beer and an empty tap, put a 5 gallon corny keg of the beer on the
tap. Confirm the tap read shows the beer and a full level, the keg's tapped time is now, and the
beer appears as a pint glass in the admin library and edit form.

**Acceptance Scenarios**:

1. **Given** an empty tap and a current beer, **When** an admin puts a keg of the beer on the
   tap with default values, **Then** a keg is created with type "5 gallon corny", size 18.93 L
   (5.00 gal), remaining equal to size, and tapped time now.
2. **Given** a tap that already has a keg, **When** an admin tries to put another keg on it,
   **Then** it is refused: one keg per tap.
3. **Given** an archived beer, **When** an admin tries to put a keg of it on a tap, **Then** it
   is refused with a field error on the beer.
4. **Given** a beer with a keg on a tap, **When** the beer is shown in the admin library or its
   edit form, **Then** it is drawn as a pint glass in its color; other beers stay kegs.
5. **Given** a keg on a tap, **When** a client with a `read` token reads the taps, **Then** each
   tap includes its keg, if any, with the beer's name and color, keg type, size, remaining
   volume and tapped time.
6. **Given** a disabled tap, **When** an admin puts a keg on it, **Then** it succeeds; the tap
   stays disabled.
7. **Given** a current beer, **When** an admin adds a keg of it with no tap, **Then** the keg is
   saved as waiting (on deck), with no tapped time, and the beer stays drawn as a keg.
8. **Given** a waiting keg and an empty tap, **When** an admin puts the keg on the tap, **Then**
   the tap shows it and the keg's tapped time is recorded, if it had none.
9. **Given** a keg on a tap, **When** an admin takes it off without kicking, **Then** the tap is
   empty, the keg is waiting with its level and tapped time kept.
10. **Given** a half barrel keg, **When** its beer is shown with the keg, **Then** it is drawn as
    a sanke-style keg; a corny keg is drawn as the corny keg.

---

### User Story 3 - Update the keg level (Priority: P1)

As beer is poured, the owner records it: either "I poured 16 oz" or "about 2 gallons left". A
script or a future keg scale does the same with a `write` token after each pour.

**Why this priority**: the level gauge is the main thing the TV shows for each tap besides the
beer itself, and automated updates are a stated goal of the API.

**Independent Test**: on a full 18.93 L keg, record a pour of 0.5 L and confirm 18.43 L remains;
set the remaining volume to 10 L directly and confirm 10 L; do both with a `write` token; confirm
a `read` token is refused.

**Acceptance Scenarios**:

1. **Given** a keg with 18.93 L remaining, **When** a pour of 0.5 L is recorded, **Then**
   18.43 L remains.
2. **Given** a keg with 0.3 L remaining, **When** a pour of 0.5 L is recorded, **Then** 0 L
   remains; the keg is not kicked on its own.
3. **Given** a keg of size 18.93 L, **When** the remaining volume is set to 20 L or to a negative
   value, **Then** it is refused with a field error and nothing changes.
4. **Given** two pours recorded at the same moment by two clients, **When** both complete,
   **Then** the remaining volume reflects both pours.
5. **Given** a `write` token, **When** it records a pour or sets the remaining volume, **Then**
   it succeeds. **Given** a `read` token or no credential, **Then** it is refused and nothing
   changes.
6. **Given** the admin app, **When** the owner records a 16 fl oz pour on a full 5 gallon keg,
   **Then** it shows 4.88 gal remaining (stored as 18.46 L).

---

### User Story 4 - Kick a keg and see keg history (Priority: P2)

The keg runs dry. The owner kicks it: the tap becomes empty and the keg moves to history with
its kicked time. The owner can look back at past kegs: which beer, which tap, when it was tapped
and kicked.

**Why this priority**: kicking frees the tap for the next keg, but a tap can also be cleared by
removing the keg record. History is useful, not essential, for the first menu.

**Independent Test**: kick a keg, confirm the tap is empty and the keg is in history with tapped
and kicked times, then put a new keg on the same tap.

**Acceptance Scenarios**:

1. **Given** a keg on a tap, **When** an admin kicks it, **Then** its kicked time is recorded,
   the tap is empty, and its remaining volume is kept as last recorded. A waiting keg can be
   kicked the same way (for example, a dumped batch).
2. **Given** kicked kegs, **When** an admin opens keg history, **Then** they are listed newest
   kicked first, with beer, last tap, keg type, size, tapped time and kicked time.
3. **Given** a kicked keg, **When** an admin tries to record a pour or change its level, **Then**
   it is refused: history is read-only except for deleting a record.
4. **Given** a keg entered by mistake, **When** an admin deletes it after confirming, **Then** it
   is removed for good, and its tap is empty if it was on one.
5. **Given** a beer, **When** a client reads its keg history, **Then** only that beer's kegs are
   listed.

---

### Edge Cases

- Removing a tap that has kicked kegs in history: allowed. The history keeps the tap's label as
  it was.
- A keg taken off a tap and put back, on the same or another tap: its first tapped time is kept,
  and its last tap is the one it is on.
- A waiting keg that was never on a tap is kicked: history shows no tap and no tapped time.
- Taking a waiting keg off a tap: no effect and not an error.
- Archiving a beer that has a current keg (on a tap or waiting): refused; kick or delete the keg
  first.
- Deleting a beer that has any keg, current or in history: refused; archive it instead, so
  history stays whole.
- Recording a pour of zero or a negative amount: refused with a field error.
- Kicking a keg that is already kicked: no effect and not an error.
- A keg with remaining volume 0 that is not kicked: stays on its tap, shown empty, until kicked.
- Changing a current keg's beer, type or size to fix a mistake: allowed. A size below the
  remaining volume lowers the remaining volume to the new size.
- Volumes with many decimals: stored to the nearest 0.01 L. The admin app shows gallons to two
  decimals; saving without editing a volume must not change the stored value, because a
  gallon shown to two decimals can be up to about 0.02 L off the stored litres.
- Reordering with a list that misses a tap, repeats one or names one that does not exist:
  refused, and the order is unchanged.
- Tap labels with non-English characters or emoji: saved and shown back unchanged.
- An identifier for a tap or keg that does not exist: "not found", and no change.

## Requirements *(mandatory)*

### Functional Requirements

**Taps**

- **FR-001**: The system MUST store taps with a position, a label and an enabled flag. Each tap
  MUST have a stable identifier that does not change when it is renamed, moved or disabled.
- **FR-002**: At least one tap MUST always exist. The upgrade that adds taps MUST create one tap,
  labelled "Tap 1", enabled. There is no maximum.
- **FR-003**: Positions MUST run 1 to N with no gaps or repeats. Adding a tap MUST place it last;
  removing a tap MUST close the gap.
- **FR-004**: Reordering MUST take the complete new order of all taps and MUST refuse an order
  that misses, repeats or invents a tap.
- **FR-005**: A label MUST be 1 to 50 characters after trimming. A new tap without a label MUST
  get "Tap N", N being its position when added. Labels need not be unique.
- **FR-006**: A tap MUST NOT be removed while a keg is on it, or when it is the last tap.
- **FR-007**: Disabling a tap MUST keep its keg, if any. Whether disabled taps show on the TV is
  for feature 5.

**Kegs**

- **FR-010**: The system MUST store kegs with a beer, a keg type, a size, a remaining volume, an
  optional tap, the label of the last tap it was on, a tapped time and a kicked time. Volumes
  MUST be stored in litres, to the nearest 0.01 L.
- **FR-011**: Keg type MUST be one of this fixed list, each with its default size: corny 2.5 gal
  (9.46 L), corny 3 gal (11.36 L), corny 5 gal (18.93 L), sanke sixth barrel (19.56 L), sanke
  quarter barrel (29.34 L), sanke half barrel (58.67 L), euro 20 L, euro 30 L, euro 50 L. The
  default type MUST be corny 5 gal.
- **FR-012**: Size MUST be greater than 0 and at most 100 L. Remaining MUST be from 0 to size.
  A new keg's remaining MUST default to its size.
- **FR-013**: A keg MUST be of a beer that exists and is not archived.
- **FR-014**: A tap MUST hold at most one current (not kicked) keg. Putting a keg on a tap that
  has one MUST be refused.
- **FR-015**: A keg MAY be created with or without a tap. A keg without a tap is waiting (on
  deck). Putting a keg on a tap for the first time MUST record the tapped time; later moves keep
  it.
- **FR-015a**: A current keg MUST be movable onto an empty tap and off its tap without kicking.
  Every time a keg goes on a tap, its last tap label MUST be updated.
- **FR-016**: Recording a pour MUST subtract the amount from the remaining volume, stopping at 0.
  The amount MUST be greater than 0. Concurrent pours MUST all count.
- **FR-017**: Setting the remaining volume directly MUST be allowed from 0 to size.
- **FR-018**: Kicking a current keg, on a tap or waiting, MUST record the kicked time and free its
  tap. Kicking a kicked keg MUST succeed without change. Nothing kicks a keg on its own.
- **FR-019**: A kicked keg MUST NOT be edited, poured from or put back on a tap. It MAY be
  deleted.
- **FR-020**: A current keg's beer, type and size MAY be corrected. Changing the type alone
  MUST NOT change the size. If a new size is below the remaining volume, the remaining volume
  MUST become the new size.
- **FR-021**: Deleting a keg MUST remove it permanently and MUST ask for confirmation in the admin
  app.

**Beers (changes to feature 3 behavior)**

- **FR-030**: Archiving a beer with a current keg, on a tap or waiting, MUST be refused with a
  clear message.
- **FR-031**: Deleting a beer that has any keg, current or kicked, MUST be refused with a clear
  message suggesting archive.
- **FR-032**: Beer reads MUST say whether the beer currently has a keg on a tap, so every client
  can choose glass or keg artwork from one rule.
- **FR-033**: The admin beer library and the beer edit form preview MUST draw a beer as a pint
  glass when it has a keg on a tap, and as a keg otherwise.
- **FR-034**: Keg artwork MUST come in two original drawings: the existing corny keg for corny
  types, and a sanke-style keg for sanke and euro types. Where a keg is shown, it MUST use its
  type's drawing; a beer shown without a specific keg uses the corny drawing.

**Reading**

- **FR-040**: Listing taps, listing kegs and reading one tap or keg MUST need an admin session or
  any API token. No credential, or an invalid or revoked token, MUST be refused with 401.
- **FR-041**: The tap list MUST be in position order and include each tap's current keg, if any,
  with its beer's identifier, name and display color.
- **FR-042**: The keg list MUST show current kegs (on a tap and waiting) by default and kicked
  kegs (history) on request, newest kicked first. It MUST be filterable by beer and by tap, and
  MUST let a client list only waiting kegs. The tap filter matches the tap a keg is on, or was
  kicked on while that tap still exists; a keg taken off a tap before being kicked, or whose
  tap was removed, keeps its tap label but does not match.
- **FR-043**: A kicked keg in history MUST keep the label of the last tap it was on, even if the
  tap is later renamed or removed.

**Changing (protected)**

- **FR-050**: Every tap and keg change (add, edit, reorder, remove taps; create, edit, move, pour,
  set level, kick, delete kegs) MUST require an admin session or a `write` API token. A `read` token
  or no credential MUST be refused.
- **FR-051**: Every browser change MUST refuse cross-site requests, tested for each change
  operation.
- **FR-052**: Invalid input MUST be refused with an error that names each invalid field, and MUST
  change nothing. Conflicts (a tap already holding a keg, removing the last tap) MUST be refused
  with a clear message and change nothing.
- **FR-053**: Edits MUST change only the fields the client sends.

**Admin app**

- **FR-060**: The admin app MUST show a taps panel for signed-in admins: each tap in order, its
  label, enabled state, and its keg's beer artwork, beer name, keg type and level.
- **FR-061**: The panel MUST let the admin add, rename, reorder, enable or disable, and remove
  taps.
- **FR-062**: The panel MUST let the admin put a keg on an empty tap, or add a waiting keg,
  choosing the beer from the current library and the keg type, with size and remaining prefilled
  from the type.
- **FR-063**: The panel MUST let the admin record a pour or set the remaining volume, kick the
  keg, correct the keg, take it off its tap, and put a waiting keg on an empty tap.
- **FR-063a**: The admin app MUST list waiting kegs (on deck) with beer artwork in the keg's
  drawing, beer name, type and level.
- **FR-064**: The admin app MUST show keg history with beer, tap, type, size, tapped time and
  kicked time.
- **FR-065**: Until feature 7, the admin app MUST show and take keg sizes and remaining volume in
  US gallons (two decimals) and pours in US fluid ounces, converting to and from litres for the
  API. The API MUST only use litres.
- **FR-066**: If the session ends while the admin is working, the admin app MUST return to the
  sign-in screen, as earlier panels do.

**Contract and data**

- **FR-070**: Every tap and keg endpoint MUST be described in the repository's API document with
  its access level, covered by the existing access check. Pour and level updates are endpoints
  later clients depend on and MUST stay backward compatible (constitution I).
- **FR-071**: The schema change MUST be a new versioned migration, tested by applying it to a
  database at the previous version, after the existing backup-before-migrate step.

### Key Entities

- **Tap**: a fixed slot on the kegerator. Attributes: identifier, position, label, enabled.
  Holds at most one current keg.
- **Keg**: one fill of one beer. Attributes: identifier, beer, keg type, size (L), remaining (L),
  tap (optional, current kegs only), last tap label, tapped time (first time on a tap), kicked
  time (empty while current), created and updated times. A current keg is on a tap or waiting;
  kicked kegs are history.
- **Keg type**: a fixed list of keg kinds, each with a default size and one of two drawings
  (corny or sanke). Not editable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An admin can put a full keg of a beer on an empty tap, or move a waiting keg onto
  one, in under 30 seconds from the admin app, on a laptop or a phone.
- **SC-002**: An admin can record a pour in two taps or clicks from the taps panel.
- **SC-003**: A script with a `write` token can record a pour with a single request, and the new
  level is visible to every reader at once.
- **SC-004**: 100% of tap and keg change operations are refused with no credential, with a
  `read` token, with a revoked token, and from a cross-site browser request, as shown by
  automated tests.
- **SC-005**: With 20 taps and 1,000 kegs in history, the taps panel and keg history each load
  in under 1 second on the home server.
- **SC-006**: Upgrading an install from feature 3 keeps all beers, admins and tokens, adds one
  tap and no kegs, with a database backup taken before the upgrade.
- **SC-007**: Every beer with a keg on a tap shows as a pint glass in the admin library, and
  every other beer as a keg, as shown by automated tests. Corny types draw as the corny keg and
  sanke and euro types as the sanke keg.
- **SC-008**: A keg saved from the admin app without editing its size or level keeps its stored
  litres exactly, and a value entered in gallons is stored within 0.005 gal of what was typed,
  for every keg type's default size, as shown by automated tests.

## Assumptions

- Waiting kegs are real kegs, so feature 5 can show them on deck alongside beers that are still
  fermenting or conditioning. Feature 5 decides what the TV shows; this feature only stores and
  lists waiting kegs.
- Default keg sizes are nominal: 1 US gallon = 3.785411784 L, 1 US barrel = 31 US gallons,
  1 US fluid ounce = 29.5735295625 mL.
- The sanke drawing is original, like the corny one, and is used for euro kegs too; their shapes
  are close enough at menu size.
- No pour log: a pour changes the remaining volume and is not stored as its own record. Pour
  history and consumption stats are out of scope.
- No automatic kick at 0 L: scales drift, so only a person or client kicks.
- No paging on tap or keg lists in the MVP; a home bar has tens of taps and hundreds of kegs.
- Keg level changes are not pushed to clients; the display polls or uses server events in
  feature 5.
- Bottles and cans are out of scope until after the MVP.
