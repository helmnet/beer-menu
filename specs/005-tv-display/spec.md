# Feature Specification: TV Display

**Feature Branch**: `compmodder26/feature_5`

**Created**: 2026-11-09

**Status**: Draft

**Input**: User description: "Feature 5: TV display. Full-screen landscape tap list at /display for a 1080p TV in Fully Kiosk on a Fire TV Stick, readable at 3 m, dark theme by default, no interaction, no credential. Shows enabled taps in position order: beer name, style, ABV, IBU, glassware art in the beer's SRM color where the liquid fill level is the keg level gauge (add a level 0..1 prop to BeerArt; also replace the admin TapsPanel <meter> with it), plus a number beside it for readability (US gallons and/or percent; units setting is feature 7 so default US gallons) and the level in the art's aria-label (e.g. "Pale Ale, pint glass, 60% left"). An "on deck" (coming soon) section shows waiting kegs (filled, off tap) drawn with their keg shape (corny/sanke) in the beer color, and beers with status fermenting/conditioning (decide about planned). Updates without reload (polling or SSE; nginx proxy matters for SSE) and recovers unattended from network drops and server restarts for days. Adds public GET /api/menu (AccessPublic, the only public menu read) returning only display fields: enabled taps in position order with beer name, style, ABV, IBU, color, keg type/shape, size_l, remaining_l, plus on-deck items; no ids beyond what the TV needs, no archived data. Full beer reads stay credentialed. Spec must decide: whether disabled taps show, how an empty tap shows, polling vs SSE. Title/logo/theme settings are feature 7, not this one. Existing DisplayPage.tsx placeholder has an inline style= that the nginx CSP (no style-src) blocks; drop it. Depends on feature 4. Spec dir specs/005-tv-display."

## Context

Feature 4 put kegs on taps and kept their levels. This feature shows that on the TV: a
full-screen tap list that guests read from across the room, plus an "on deck" section for what
comes next. The TV runs Fully Kiosk on a Fire TV Stick with nobody at the controls, so the page
must keep itself current for days and recover on its own.

The TV has no credential. It reads one new public menu read that returns only what the TV
shows. Every other beer, tap and keg read still needs an admin session or an API token
(feature 3 and 4 rules are unchanged).

The keg level gauge is the beer artwork itself: the beer in the glass (or keg) fills to the
keg's level. The admin taps panel uses the same drawing in place of its plain level bar.

## Clarifications

### Session 2026-11-09

- Q: What number shows beside the glass on the TV (percent, US gallons, both)? A: None. The
  glass fill is the only level on the TV. The public menu gives a level fraction, not volumes;
  Home Assistant reads exact volumes with a token.
- Q: How does the TV show more taps than fit well on one screen? A: In pages that rotate.
  Each page holds up to 8 cards (4 × 2 on a 16:9 screen); more taps make more pages. On deck
  is its own page (or pages) in the rotation, not a strip under the taps. Each page shows for
  15 seconds. The interval is a fixed default here; making it a setting belongs to feature 7
  with the other display settings. No aspect-ratio setting: the page sizes itself to the
  screen.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See what is on tap (Priority: P1)

A guest looks at the TV and sees every tap in use, left to right as on the kegerator: tap label,
beer name, style, ABV, IBU, and a pint glass in the beer's color, filled to how much is left in
the keg. The fill is the only level shown; there is no number.

**Why this priority**: this is the product. Without it the TV shows nothing useful.

**Independent Test**: with three enabled taps (two with kegs, one empty) and one disabled tap,
open the display with no credential and confirm the three enabled taps show in position order
with the right beer details, glass color and level, the empty tap shows as empty, and the
disabled tap does not show.

**Acceptance Scenarios**:

1. **Given** an enabled tap with a keg of "Pale Ale" (5.2% ABV, 35 IBU) at 60% left, **When**
   the display loads with no credential, **Then** it shows the tap label, "Pale Ale", its style,
   "5.2%", "35 IBU", and a pint glass in the beer's color filled to 60%, with no level number.
2. **Given** taps at positions 1, 2 and 3, **When** the display loads, **Then** they show in
   that order, left to right then top to bottom; with more than 8 enabled taps, taps 9 onward
   continue on the next page.
3. **Given** an enabled tap with no keg, **When** the display loads, **Then** the tap shows its
   label and "Empty" with an empty glass outline, so the slots match the kegerator.
4. **Given** a disabled tap, with or without a keg, **When** the display loads, **Then** that
   tap does not show.
5. **Given** a keg with 0 L left that is not kicked, **When** the display loads, **Then** its tap
   shows the beer with an empty glass.
6. **Given** a beer with no ABV, IBU or style, **When** it is on tap, **Then** the missing values
   are left out, with no "null", "0" or blank label in their place.
7. **Given** a screen reader reads the page, **When** it reaches a glass, **Then** it hears the
   beer name, the drawing and the level, such as "Pale Ale, pint glass, 60% left".

---

### User Story 2 - The display keeps itself current (Priority: P1)

The owner changes the menu from the admin app or a script (puts on a keg, records a pour, kicks
a keg). The TV shows the change by itself. If the Wi-Fi drops or the server restarts, the TV
keeps showing the last menu it had and catches up once the server is back, with nobody touching
it.

**Why this priority**: the TV runs unattended for days (constitution III). A display that needs
a manual reload fails its purpose.

**Independent Test**: open the display, record a pour with a `write` token, and confirm the
level changes on screen within 30 seconds. Stop the API container for 5 minutes, confirm the
last menu stays on screen, start it again, make a change, and confirm the display shows it
within 30 seconds of the server being back, with no reload.

**Acceptance Scenarios**:

1. **Given** the display is open, **When** a pour is recorded, a keg is put on or taken off a
   tap, a keg is kicked, or a beer on tap is renamed, **Then** the display shows the change
   within 30 seconds without a reload.
2. **Given** the display is open, **When** the server is unreachable, **Then** the display keeps
   showing the last menu it received and keeps retrying.
3. **Given** the server has been unreachable for more than 2 minutes, **When** the display is
   still showing the last menu, **Then** a small, unobtrusive "offline" notice appears in a
   corner, and it disappears once the display reaches the server again.
4. **Given** the display was first opened while the server was down, **When** the server comes
   back, **Then** the menu appears without a reload.
5. **Given** the display has run for 7 days, **When** nothing has changed, **Then** it still
   shows the current menu and its memory use has not grown without bound.

---

### User Story 3 - See what is on deck (Priority: P2)

Guests see what comes next on an "on deck" page in the rotation: kegs that are filled and waiting off a tap,
drawn as their keg type in the beer's color, and beers still fermenting or conditioning, with
their status.

**Why this priority**: the brief's "coming soon" list. Useful, but the tap list stands alone.

**Independent Test**: create a waiting sanke keg of one beer, set one beer to fermenting and one
to conditioning, set one to planned and archive one fermenting beer, and confirm the on-deck
section shows the keg (sanke drawing, beer color), the fermenting and conditioning beers with
their status, and not the planned or archived beers.

**Acceptance Scenarios**:

1. **Given** a waiting keg of "Stout" in a corny keg, **When** the display loads, **Then** on deck
   shows "Stout" with its style and ABV and a corny keg drawn in the stout's color.
2. **Given** a waiting keg in a sanke or euro keg type, **When** the display loads, **Then** it
   draws as the sanke keg.
3. **Given** a beer with status fermenting or conditioning and no current keg, **When** the
   display loads, **Then** on deck shows its name, style and status ("Fermenting" or
   "Conditioning"), drawn as a 5 gallon corny keg in its color.
4. **Given** a beer with status planned or ready and no waiting keg, **When** the display loads,
   **Then** it does not show on deck.
5. **Given** an archived beer, whatever its status, **When** the display loads, **Then** it does
   not show anywhere on the display.
6. **Given** a beer that has a waiting keg and also has status conditioning, **When** the display
   loads, **Then** it shows once on deck, as the waiting keg.
7. **Given** nothing is on deck, **When** the display loads, **Then** there is no on-deck page
   in the rotation.
8. **Given** 3 taps and 2 on-deck items, **When** the display runs, **Then** it alternates
   between the tap page and an "On deck" page every 15 seconds.

---

### User Story 4 - Keg level as artwork in the admin app (Priority: P3)

In the admin taps panel, each keg's level shows as its beer artwork filled to the level, with
the amount left beside it, in place of the plain level bar.

**Why this priority**: consistency with the TV; the admin panel already works without it.

**Independent Test**: open the taps panel with a keg at 25% and confirm a glass filled to a
quarter, the amount in US gallons beside it, and no plain level bar.

**Acceptance Scenarios**:

1. **Given** a keg on a tap at 25% left, **When** an admin opens the taps panel, **Then** the
   keg shows a pint glass in the beer's color filled to a quarter, with the level in its
   accessible name.
2. **Given** a waiting keg, **When** the admin views it, **Then** it shows the keg drawing filled
   to its level.

---

### Edge Cases

- No taps enabled, or all enabled taps empty and nothing on deck: the display shows a calm
  "Nothing on tap" message, not an error.
- Many taps: each page holds up to 8 tap cards; 9 to 16 taps make 2 tap pages, and so on.
  Cards never shrink below the 8-per-page size and the display never scrolls.
- Many on-deck items: on-deck pages hold up to 8 items each, after the tap pages.
- One page in total (up to 8 taps and nothing on deck): no rotation and no page indicator.
- The menu changes mid-rotation so there are fewer pages: the display moves to a page that
  still exists, never a blank one.
- Rotation and polling are separate: a menu update does not restart the rotation, and the
  rotation keeps running while the server is unreachable (it rotates the last good menu).
- Long beer names and styles: they wrap to at most two lines and are then cut off with an
  ellipsis; they never push other taps off screen.
- Names with non-English characters or emoji: shown unchanged.
- Levels above size or below zero cannot happen (feature 4 rules). A keg with any beer left
  draws a visible sliver of beer (at least 2% of the glass), so it never looks empty. A glass
  draws full only when the keg is full.
- The server answers with an error (such as 500) or a body the display cannot read: treated
  like being offline; the last good menu stays on screen.
- The server is upgraded to a new version while the TV is open: the display keeps working; if
  the page itself changed, it picks up the new page the next time it reloads on its own (see
  FR-024).
- The TV clock is wrong: nothing on the display depends on the TV's clock.
- Everyone on the LAN can read the menu: it holds nothing beyond what the TV shows.

## Requirements *(mandatory)*

### Functional Requirements

**Public menu read**

- **FR-001**: The system MUST offer one public menu read that needs no credential. It MUST be
  the only public read of beer, tap or keg data; every other beer, tap and keg read keeps its
  feature 3 and 4 access rule.
- **FR-002**: The menu MUST list enabled taps in position order. Each tap MUST give its label and,
  if it has a current keg, the beer's name, style, ABV, IBU and color, the keg's drawing
  (corny or sanke) and its level as a fraction from 0 to 1. Disabled taps MUST NOT appear.
- **FR-003**: The menu MUST list on-deck items: current kegs waiting off a tap (beer name, style,
  ABV, IBU, color, keg drawing, level), then beers with status conditioning, then
  fermenting, that have no current keg (name, style, ABV, IBU, color, status).
- **FR-004**: The menu MUST NOT include archived beers, kicked kegs, beers with status planned or
  ready that have no waiting keg, keg sizes or volumes, beer descriptions, brewer, brew dates,
  source references, record identifiers, timestamps, or anything about admins and tokens.
  Clients that need volumes (such as Home Assistant) use the credentialed keg reads.
- **FR-005**: Waiting kegs on deck MUST be ordered oldest first (the order they were added);
  beers on deck MUST be ordered by name within their status.
- **FR-006**: The public menu read MUST be safe to call often from many screens: read-only, no
  side effects, no writes, and it MUST NOT be cached by the browser or proxy so a change shows
  on the next read.
- **FR-007**: The public menu read MUST be described in the repository's API document, marked
  public, with its response shape.

**Display page**

- **FR-010**: The display MUST be a full-screen, landscape page for a 1080p TV, dark by default,
  readable at 3 m: beer names at least 48 px tall on a 1080-pixel-high screen with up to 8 taps,
  and nothing smaller than 24 px.
- **FR-011**: The display MUST need no credential and no interaction: no sign-in, no buttons, no
  cursor-dependent content, no pop-ups.
- **FR-012**: Each tap with a keg MUST show its label, beer name, style, ABV, IBU (each left out
  when missing), and a pint glass drawing in the beer's color filled to the keg's level. The
  fill is the only visible level: the display MUST NOT show a percent or volume number.
- **FR-013**: Each enabled tap with no keg MUST show its label, "Empty" and an empty glass
  outline.
- **FR-014**: Each on-deck waiting keg MUST draw as its keg type (corny or sanke) in the beer's
  color, filled to its level. Each on-deck beer MUST draw as a 5 gallon corny keg in its color
  with its status.
- **FR-015**: Every beer drawing MUST include the level, when it has one, in its accessible name,
  such as "Pale Ale, pint glass, 60% left".
- **FR-016**: The display MUST NOT use inline styles, so it renders under the site's content
  security policy; the current placeholder's inline style MUST go.
- **FR-017**: The display MUST show taps and on-deck items in pages of at most 8 cards, tap
  pages first (in position order) then on-deck pages, rotating every 15 seconds when there is
  more than one page, with a small page indicator (such as "2 / 3"). Each page MUST fit the
  screen with no scrolling. With one page there is no rotation and no indicator. Timing counts
  ticks, not the TV clock.

**Live updates and recovery**

- **FR-020**: The display MUST check for menu changes at least every 15 seconds, so a change
  shows within 30 seconds, without a reload.
- **FR-021**: When a check fails (network down, server restarting, error response, unreadable
  body), the display MUST keep the last good menu on screen and try again, waiting no longer
  than 30 seconds between tries.
- **FR-022**: After 2 minutes without a good check, the display MUST show a small "offline"
  notice that does not cover the menu, and MUST remove it after the next good check.
- **FR-023**: The display MUST NOT grow its memory use over days of running: each check replaces
  the previous menu instead of adding to it.
- **FR-024**: The display MUST reload itself fully about once a day, so a server upgrade's new
  page is picked up without anyone touching the TV. It MUST reload only right after a good check,
  never while the server is unreachable, so the TV never lands on a browser error page.

**Shared artwork**

- **FR-030**: The beer artwork MUST accept a level from 0 to 1 and fill the beer to that fraction
  of the drawing's liquid area, for the pint glass and both keg drawings. With no level, it
  MUST fill fully, as today.
- **FR-031**: The admin taps panel MUST show each current keg's level with the filled artwork
  and the amount left beside it (US gallons, as today), replacing the plain level bar.

**Tests and contract**

- **FR-040**: Tests MUST show that the menu read works with no credential and returns only the
  fields in FR-002 and FR-003, that it leaves out disabled taps, archived beers, kicked kegs,
  planned and ready beers without waiting kegs, and that every other beer, tap and keg read
  still refuses a missing credential.
- **FR-041**: The route table test that lists public routes MUST include the menu read, and a
  test MUST show the menu read refuses changes (only reading is allowed).

### Key Entities

- **Menu**: a read-only view assembled from taps, kegs and beers; not stored.
  - **Menu tap**: label, and optionally a beer (name, style, ABV, IBU, color) and keg (drawing,
    level 0 to 1).
  - **On-deck keg**: beer (name, style, ABV, IBU, color) and keg (drawing, level 0 to 1).
  - **On-deck beer**: name, style, ABV, IBU, color, status (fermenting or conditioning).

No new stored data. No schema change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A guest 3 m from a 1080p TV can read every beer name and ABV, and tell a full glass
  from a half and a nearly empty one, on a menu of 8 taps, as checked by the owner on the real
  TV.
- **SC-002**: A pour, keg change or beer rename shows on the TV within 30 seconds, without
  anyone touching the TV.
- **SC-003**: After the server is down for 10 minutes and comes back, the TV shows the current
  menu within 30 seconds of the server being back, with no reload by hand.
- **SC-004**: The display runs for 7 days in Fully Kiosk on the Fire TV Stick with no manual
  reload and no crash.
- **SC-005**: The public menu read exposes no field beyond FR-002 and FR-003, and every other
  beer, tap and keg read is refused without a credential, as shown by automated tests.
- **SC-006**: The menu loads in under 1 second on the home server with 20 taps, 50 on-deck
  items and 1,000 kegs in history.
- **SC-007**: Every glass and keg drawing on the display and in the admin taps panel fills to
  within 2% of the keg's level and announces that level, as shown by automated tests.
- **SC-008**: A guest who watches the TV for one full rotation (pages × 15 s) sees every
  enabled tap and every on-deck item, as shown by automated tests.

## Assumptions

- Polling, not server-sent events. A check every 10 to 15 seconds meets the 30-second target,
  survives server restarts and dropped Wi-Fi by just trying again, and needs no proxy changes in
  nginx or NPM. Server-sent events can come later if needed.
- Disabled taps are hidden on the TV: disabled means out of service, and guests have nothing
  to read there.
- Empty enabled taps show as "Empty" so the TV matches the kegerator's slots.
- Planned beers stay off the TV: "coming soon" in the brief means fermenting or conditioning.
  Ready beers with no keg are not on deck either; a ready beer shows when someone fills a keg.
- The TV shows no volumes, so feature 7's unit setting does not affect it. Title, logo, colors
  and theme are feature 7; this feature uses a fixed dark theme and no title setting.
- The display is for one landscape 1080p TV; other screen sizes work but are not tuned. The
  page sizes cards from the screen (`vh`, `vw`), so no aspect-ratio setting is needed.
- The rotation interval is fixed at 15 seconds. Feature 7 makes it a display setting.
- The menu read is public on the LAN by design (brief, constitution V). It is reached only
  through the same web origin and proxy as the admin app.
- Fully Kiosk's own "reload on network reconnect" and "reload on error" options are helpful but
  not relied on; the page recovers by itself.
