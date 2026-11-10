# Feature Specification: Display Settings

**Feature Branch**: `compmodder26/feature_7`

**Created**: 2026-11-10

**Status**: Draft

**Input**: User description: "Feature 7: Display settings. Title, logo, theme, units and the TV page rotation interval (default 15 s) editable from the admin app. Depends on feature 5 (TV display). Brief: dark theme by default; basic theme settings (title, logo, colors) from the admin app; settings table (key, value) holds display title, theme, units. The TV must read the settings it needs without a credential (public). No aspect-ratio or cards-per-page setting (owner decision). Open questions for clarify: logo source given \"no image uploads\" decision; theme scope (dark/light vs custom colors, TV only vs admin); which units switch and where; how public display config reaches the TV."

## Context

Feature 5 put the tap list on the TV with fixed choices: no title, a built-in dark look, and
pages that rotate every 15 seconds. The admin app shows volumes in US gallons and fluid ounces
only. This feature makes those choices settings the owner changes from the admin app, and the
TV picks them up by itself, with no credential and nobody touching it.

Like every other capability, the settings are reachable through the API, so a script or a later
client can read and change them with a token. The TV only needs the settings that change what
it shows, and reads them without a credential, alongside the public menu.

Out of scope (owner decisions): a logo, an aspect-ratio setting and a cards-per-page setting.
The admin app's own look is restyled in a later feature, not this one.

## Clarifications

### Session 2026-11-10

- Q: How does the owner set a logo, given no image uploads are planned? A: No logo. The brief's
  logo is dropped; the title names the tap list. The no-uploads decision stands.
- Q: What theme choices exist, and where do they apply? A: Dark (default) or light, on the TV
  only. No custom colors. The admin app keeps its look until the later restyle feature.
- Q: Which volume units, and where? A: US (gallons, fluid ounces; default) or metric (litres,
  millilitres), in the admin app only. The TV keeps showing no volume numbers (feature 5).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Name the tap list and change how often the TV turns the page (Priority: P1)

The owner opens the admin app, types a title such as "Helm Taproom", sets the page rotation to
20 seconds, and saves. Within one menu refresh the TV shows the title at the top of every page
and holds each page for 20 seconds. Nobody touches the TV.

**Why this priority**: the title and rotation are the settings the brief names first and the
TV already has fixed values for; they deliver value with no new artwork or colors.

**Independent Test**: with a fresh install, open the display (title absent, 15 s pages), set a
title and a 20 s interval from the admin app, then confirm the TV shows the title and the new
interval without a reload, and that the settings read back the same after an API restart.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the owner opens the settings, **Then** the title is empty
   and the rotation interval is 15 seconds.
2. **Given** a fresh install, **When** the TV loads, **Then** it looks as it did before this
   feature: no title, dark look, 15 s pages.
3. **Given** the owner saves the title "Helm Taproom", **When** the TV next refreshes the menu,
   **Then** every page shows "Helm Taproom" at the top, with no reload.
4. **Given** the owner saves a 20 s interval, **When** the TV next refreshes, **Then** each page
   then shows for 20 s.
5. **Given** the owner enters an interval outside the allowed range, or a title longer than the
   allowed length, **When** they save, **Then** the save is refused, the field shows why, and
   the stored values do not change.
6. **Given** the owner clears the title, **When** they save, **Then** the TV shows no title row
   and the tap cards keep their full space.
7. **Given** only one page of taps and no on-deck page, **When** the interval changes, **Then**
   the TV still shows the one page with no visible flicker.

---

### User Story 2 - Choose the TV's look (Priority: P2)

The owner picks dark or light for the TV in the admin app and saves. The TV switches look on its
next refresh. Dark stays the default. The admin app's look does not change.

**Why this priority**: the brief asks for theme settings, but the TV is fully usable in the
default dark look, so this ranks after the title and rotation.

**Independent Test**: change the theme from the admin app, then confirm the TV changes look
without a reload, the text stays readable at 3 m, and a reset brings back the default dark
look.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the TV loads, **Then** it uses the dark theme.
2. **Given** the owner picks light and saves, **When** the TV next refreshes, **Then** the TV
   uses the light theme, with no reload; picking dark brings the dark theme back the same way.
3. **Given** either theme, **When** the TV shows it, **Then** text keeps at least a 4.5:1
   contrast with its background, and beer artwork stays visible against the background.
4. **Given** the owner (or a script) sends a theme other than dark or light, **When** it is
   saved, **Then** the save is refused and nothing changes.
5. **Given** the theme is light, **When** the owner uses the admin app, **Then** the admin app
   looks the same as before.

---

### User Story 3 - Choose the units for volumes (Priority: P2)

The owner picks US or metric units, and every place in the admin app that shows or takes a
volume uses them. The TV is unchanged: it shows no volume numbers.

**Why this priority**: the constitution says display units are a setting; today the admin app is
fixed to US units.

**Independent Test**: switch units, then confirm keg sizes, levels and pour amounts show and are
entered in the new units, and that stored amounts are unchanged when switching back.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the owner views a keg, **Then** volumes show in US gallons
   and pours are entered in US fluid ounces, as today.
2. **Given** the owner switches to metric and saves, **When** they view or enter a keg size or
   level, **Then** it is in litres, and pours are entered in millilitres.
3. **Given** a keg of 18.93 L, **When** the owner switches units and back, **Then** the stored
   amount is still 18.93 L; switching units never changes stored data.
4. **Given** units are metric, **When** the TV loads, **Then** it looks the same as with US units.

---

### Edge Cases

- The TV is offline when a setting changes: it keeps its last settings and menu, and applies the
  new ones on the first refresh after it reconnects.
- The settings cannot be read when the TV first loads: the TV uses the defaults (no title, dark,
  15 s) and still shows the menu if it can.
- Two admins save settings at the same time: the last save wins per setting; no setting is left
  half-written.
- A title with only spaces is stored as empty; leading and trailing spaces are trimmed.
- A title with markup or script text shows as plain text, never as markup.
- The rotation changes in the middle of a page: the new interval applies from the next page turn
  at the latest.
- A database at the previous version upgrades with no settings rows for this feature: every
  setting reads as its default.
- A read-only token tries to change a setting: refused; it can read them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST store these display settings: title, theme (dark or light), volume
  units (US or metric) and TV page rotation interval, each with a default: empty title, dark,
  US, 15 seconds.
- **FR-002**: An admin (session) or a `write` token MUST be able to change any of the settings in
  one save; a save MUST change only the settings it names.
- **FR-003**: An admin or any valid token (`read` or `write`) MUST be able to read all settings,
  each returned as its stored value or its default.
- **FR-004**: The settings the TV needs (title, theme, rotation interval) MUST be readable
  without a credential. No other setting may be exposed publicly, and the public read MUST NOT
  expose any credential, secret or integration setting.
- **FR-005**: Every change MUST be validated before it is stored: the title is plain text of at
  most 60 characters after trimming; the rotation interval is a whole number of seconds from 5
  to 300; theme and units are one of the allowed values. An invalid save MUST change nothing and
  name each invalid field.
- **FR-006**: Writes MUST follow the existing rules: no credential refused, `read` token refused,
  revoked token refused, cross-site browser request refused.
- **FR-007**: The admin app MUST have a settings panel that shows the current values, lets the
  owner change and save them, and shows each validation error beside its field.
- **FR-008**: The TV MUST apply changed settings on its next menu refresh, with no reload and no
  credential, and MUST keep its last good settings while the server cannot be reached.
- **FR-009**: The TV MUST show the title, when set, at the top of every page, readable at 3 m,
  and MUST show no title row when the title is empty.
- **FR-010**: The TV MUST hold each page for the rotation interval setting.
- **FR-011**: The TV MUST use the theme setting; both themes MUST keep text contrast of at least
  4.5:1. The theme MUST NOT change the admin app.
- **FR-012**: Every volume shown or entered in the admin app MUST use the units setting: US shows
  sizes and levels in US gallons and takes pours in US fluid ounces; metric shows litres and
  takes pours in millilitres. Stored volumes stay in litres and never change when units change.
  The TV shows no volume numbers in either setting.
- **FR-013**: The settings read and write MUST be described in the API contract.
- **FR-014**: Upgrading a database from the previous version MUST keep all data and leave every
  new setting at its default.

### Key Entities

- **Display settings**: one set for the whole install (single tenant). Title, theme, volume
  units, rotation interval. Each has a default used when it was never set. Stored as named
  settings beside the existing Brewfather sync interval, which this feature does not expose.
- **Public display config**: the subset of display settings the TV reads without a credential:
  title, theme, rotation interval.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After the owner saves a setting, the TV shows the change within one menu refresh
  (at most 30 seconds), with nobody touching the TV.
- **SC-002**: The owner can change the title and rotation interval from the admin app in under 1
  minute.
- **SC-003**: A fresh install and an upgraded install look the same on the TV as before this
  feature until a setting is changed.
- **SC-004**: Both themes pass a 4.5:1 text contrast check, and the title is readable at
  3 m on a 1080p TV.
- **SC-005**: No setting can be changed without an admin session or a `write` token, and the
  public read returns only the TV's settings; both are shown by automated tests.
- **SC-006**: Switching units back and forth leaves every stored volume unchanged.

## Assumptions

- The TV already refreshes the menu on a short interval (feature 5); settings ride on that same
  refresh, so no separate push is needed.
- Title limit of 60 characters and rotation range of 5 to 300 seconds are reasonable defaults
  for a TV header and a page turn; the owner can change them in clarify.
- Settings are global to the install; there are no per-TV or per-user settings.
- The Brewfather sync interval stays where feature 6 put it and is not shown in this panel.
- ABV and IBU have no units choice; SRM color stays as is.
- The admin app's own restyle is a later feature; the settings panel follows the admin app's
  current unstyled look.
- No new display settings beyond those listed (no aspect ratio, no cards per page).
