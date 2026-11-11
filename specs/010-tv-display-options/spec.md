# Feature Specification: TV Display Options

**Feature Branch**: `compmodder26/display_options_additions`

**Created**: 2026-11-11

**Status**: Draft

**Input**: User description: "Feature 10: TV display options. Depends on feature 9. New TV display settings, set from the admin app's Display Settings page, each defaulting to what the TV shows today so an upgrade changes nothing. Layout: current layout (default), vertical list (taps one per row, top to bottom), or side by side (on tap and on deck as two columns, each listed top to bottom). Title alignment: left, centered, right. Title font. Font for the tap and on deck text. Tap details: show or hide each of style, ABV, IBU, SRM and description. The TV picks up changes with no reload and no credential (public display settings, as in feature 7). Brief: docs/project-brief.md. Today the TV tap card shows name, style, ABV and IBU; SRM shows only as glass color; description is not shown. Open questions for clarify: which fonts are offered and how they are delivered given the self-only CSP and no external downloads; whether detail toggles also apply to on deck cards; how long descriptions are handled at 3 m readability; how paging works in vertical list and side by side layouts; whether SRM shows as a number. Added later: a preview inside the Display Settings editor showing what the changes will look like, and text color for the title and for the tap and on deck text, chosen with a color picker."

## Context

Feature 5 put the tap list on the TV as a grid of cards: taps first, then on deck on its own
pages, at most 8 cards a page. Feature 7 let the owner set the TV's title, theme and page
interval from the admin app. Everything else about the TV's look is fixed: one layout, the
title on the left, one system font, the theme's own text colors, and each tap card shows the
beer's name, style, ABV and IBU.

This feature adds more TV settings to the same Display Settings page: the layout, how the title
is aligned, the title font, the font of the tap and on deck text, which beer details a tap
shows, and the text colors of the title and of the tap and on deck text, picked with a color
picker. A preview on the same page shows how the TV will look before the owner saves. Every new
setting defaults to what the TV shows today, so an upgrade changes nothing on
screen until the owner picks something. The TV picks the settings up the same way it picks up
the feature 7 settings: on its next menu refresh, with no reload and no credential.

Out of scope: background, accent or gauge colors, logos or uploaded fonts, per-TV settings, and any change to the
admin app's own look.

## Clarifications

### Session 2026-11-11

- Q: How does a long description fit on a tap card? A: Cut to 2 lines, ending in "…".
- Q: How do the list layouts page? A: A fixed number of rows a page; in side by side both
  columns turn pages together, and an empty on deck column shows "Nothing on deck".
- Q: Which fonts are offered? A: Today's font plus 6 to 8 free fonts shipped with the app.
- Q: What happens with a low-contrast text color? A: The save is allowed with no warning; the
  preview lets the owner judge it.
- Q: How does the preview page? A: It turns pages by itself at the page interval, and has
  previous and next buttons.
- Q: Does the public menu carry SRM and description only while their switches are on? A: No.
  The menu always carries them for tap beers (planning, 2026-11-11): the preview must show a
  detail before it is saved, and both are text the owner writes for display.
- Q: What does a row hold in the list layouts? A: A small glass or keg picture at the left, then
  the label, name and details.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Choose which beer details a tap shows (Priority: P1)

The owner opens Display Settings, turns on description and SRM, turns off IBU, and saves. On its
next refresh the TV shows each tap's style, ABV, SRM and description, and no IBU. Nobody touches
the TV.

**Why this priority**: it changes what guests read about each beer, works in today's layout, and
needs no new fonts or layouts.

**Independent Test**: on a fresh install, confirm tap cards show name, style, ABV and IBU as
before; toggle each detail from the admin app and confirm the TV adds or drops only that detail
on its next refresh.

**Acceptance Scenarios**:

1. **Given** a fresh or upgraded install, **When** the TV loads, **Then** each tap shows name,
   style, ABV and IBU, and no SRM number or description, as before this feature.
2. **Given** the owner turns description on and saves, **When** the TV next refreshes, **Then**
   each tap whose beer has a description shows it, with no reload.
3. **Given** the owner turns SRM on, **When** the TV next refreshes, **Then** each tap whose beer
   has an SRM shows it as a number (for example "SRM 6"); the glass keeps its color either way.
4. **Given** the owner turns style, ABV and IBU off, **When** the TV next refreshes, **Then** tap
   cards show the beer name and artwork with none of those details.
5. **Given** a detail is on but a beer has no value for it, **When** the TV shows that tap,
   **Then** that detail is left out for that beer, with no placeholder.
6. **Given** a description longer than 2 lines on the card, **When** the TV shows it, **Then**
   it is cut to 2 lines ending in "…", stays readable at 3 m, and never pushes other taps off
   the page or changes the card's size.
7. **Given** an empty tap, **When** the TV shows it, **Then** it shows "Empty" whatever the
   detail settings are.

---

### User Story 2 - Choose the TV layout (Priority: P2)

The owner picks a layout: the current grid, a vertical list, or on tap and on deck side by side.
The TV switches to it on its next refresh.

**Why this priority**: the biggest visual change, but the TV is fully usable in today's grid.

**Independent Test**: switch between the three layouts with taps and on deck items present, and
confirm the TV shows each one on its next refresh, keeps every tap and on deck item reachable
through its pages, and goes back to today's look when grid is picked again.

**Acceptance Scenarios**:

1. **Given** a fresh or upgraded install, **When** the TV loads, **Then** it uses the grid layout
   exactly as before this feature.
2. **Given** the owner picks vertical list, **When** the TV next refreshes, **Then** taps show one
   per row, top to bottom, in tap order, and on deck items follow under the "On deck" heading the
   same way.
3. **Given** vertical list or side by side, **When** the TV shows a row, **Then** it has a small
   glass (tap) or keg (on deck) picture at the left, filled as in grid, then the label, name and
   details.
4. **Given** the owner picks side by side, **When** the TV next refreshes, **Then** taps fill one
   column and on deck fills the other, each listed top to bottom.
5. **Given** side by side and an empty on deck list, **When** the TV shows it, **Then** the on
   deck column shows "Nothing on deck" and the tap column is unchanged.
6. **Given** vertical list or side by side, **When** there are more items than rows on a page,
   **Then** each page holds a fixed number of rows; in side by side both columns turn pages
   together, the page count is the longer column's, and the shorter column shows blank space
   after its last item.
7. **Given** any layout and more items than fit on one page, **When** the TV rotates, **Then**
   every item is shown within one rotation, with the page indicator and the page interval
   working as today.

---

### User Story 3 - Align the title and choose fonts (Priority: P3)

The owner sets the title to centered, picks a title font and a different font for the tap and
on deck text, and saves. The TV shows the new alignment and fonts on its next refresh.

**Why this priority**: cosmetic; the TV is readable without it.

**Independent Test**: change alignment and both fonts, confirm the TV applies each on its next
refresh with no reload and no request to any other server, and that the defaults bring back
today's look.

**Acceptance Scenarios**:

1. **Given** a fresh or upgraded install, **When** the TV loads, **Then** the title is
   left-aligned and all text uses today's font.
2. **Given** the owner sets alignment to centered or right, **When** the TV next refreshes,
   **Then** the title is centered or right-aligned on every page.
3. **Given** the owner picks a title font from the list (today's font plus 6 to 8 fonts that
   ship with the app), **When** the TV next refreshes, **Then** the title uses it.
4. **Given** the owner picks a text font, **When** the TV next refreshes, **Then** tap and on
   deck text (labels, names, details, status, headings) use it, and the title keeps its own font.
5. **Given** any font choice, **When** the TV shows it, **Then** the TV loads nothing from
   outside the app's own server.
6. **Given** no title is set, **When** alignment or title font changes, **Then** the TV still
   shows no title row.

---

### User Story 4 - Preview changes before saving (Priority: P2)

While editing Display Settings, the owner sees a scaled-down preview of the TV beside the form.
Each change shows in the preview at once, before saving. Saving sends the changes to the TV;
leaving without saving changes nothing on the TV.

**Why this priority**: with layouts, fonts, colors and details, the owner needs to see a choice
before putting it on the TV in front of guests.

**Independent Test**: change each setting in the form without saving, confirm the preview
updates at once and the TV does not; save and confirm the TV then matches the preview.

**Acceptance Scenarios**:

1. **Given** the owner opens Display Settings, **When** the page loads, **Then** the preview
   shows the TV as it looks now, with the current menu.
2. **Given** the owner changes any TV setting without saving, **When** they look at the preview,
   **Then** it shows the change within 1 second, and the TV is unchanged.
3. **Given** the preview has more than one page, **When** the owner watches it, **Then** it
   turns pages by itself at the page interval, like the TV, and previous and next buttons let
   the owner step through the pages.
4. **Given** the owner saves, **When** the TV next refreshes, **Then** the TV matches the
   preview.
5. **Given** the owner leaves the page without saving, **When** the TV next refreshes, **Then**
   it is unchanged.
6. **Given** a phone-width screen, **When** the owner opens Display Settings, **Then** the
   preview still fits the screen width, keeping the TV's 16:9 shape.
7. **Given** a form value is invalid, **When** the preview updates, **Then** it keeps showing
   the last valid value for that field.

---

### User Story 5 - Choose text colors (Priority: P3)

The owner picks a color for the title and a color for the tap and on deck text, each with a
color picker, and saves. The TV uses them on its next refresh. A reset brings back the theme's
own colors.

**Why this priority**: cosmetic; the theme colors already read well.

**Independent Test**: pick both colors, confirm the preview and then the TV use them; reset
each and confirm the theme colors come back.

**Acceptance Scenarios**:

1. **Given** a fresh or upgraded install, **When** the TV loads, **Then** text uses the theme's
   own colors, as before this feature.
2. **Given** the owner picks a title color and saves, **When** the TV next refreshes, **Then**
   the title uses it on every page.
3. **Given** the owner picks a text color and saves, **When** the TV next refreshes, **Then**
   tap and on deck text (labels, names, details, status, headings) use it.
4. **Given** the owner picks a color with low contrast against the theme background, **When**
   they save, **Then** the save is accepted with no warning; the preview shows how it reads.
5. **Given** a color is set, **When** the owner switches the theme between dark and light,
   **Then** the chosen color stays (one color for both themes) unless reset.
6. **Given** the owner resets a color, **When** they save, **Then** that text goes back to the
   theme's own color.

---

### Edge Cases

- The owner or a script sends a layout, alignment or font that is not in the allowed list: the
  save is refused, names the field, and nothing changes.
- A font fails to load on the TV: the TV falls back to today's font and stays readable.
- The settings cannot be read when the TV first loads: the TV uses every default and still shows
  the menu if it can; a later failed refresh keeps the last good settings.
- The layout changes in the middle of a page: the TV redraws in the new layout on that refresh
  and keeps showing every item within one rotation.
- A description holds markup or script text: it shows as plain text, never as markup.
- Every detail is turned off: tap cards show the name and artwork only.
- A color is sent that is not a valid color value: the save is refused and names the field.
- The menu cannot be read when the preview loads: the preview shows sample taps and on deck
  items so the owner can still judge the look.
- A database from the previous version upgrades: every new setting reads as its default and the
  TV looks unchanged.
- A `read` token tries to change a new setting: refused; it can read them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST store these new display settings, each with a default matching the
  TV before this feature: layout (grid, vertical list, side by side; default grid), title
  alignment (left, centered, right; default left), title font (default: today's font), text
  font (default: today's font), and a show or hide switch for each tap detail: style (shown),
  ABV (shown), IBU (shown), SRM (hidden), description (hidden), title color and text color
  (default for each: none, meaning the theme's own color).
- **FR-002**: The new settings MUST be read and changed the same way as the feature 7 settings:
  an admin or a `write` token can change any of them in one save that changes only what it
  names; an admin or any valid token can read them all.
- **FR-003**: Every change MUST be validated before it is stored: each choice MUST be one of its
  allowed values, each switch MUST be on or off, and each color MUST be a valid color value or
  none. An invalid save MUST change nothing and name
  each invalid field.
- **FR-004**: Writes MUST follow the existing rules: no credential refused, `read` token refused,
  revoked token refused, cross-site browser request refused.
- **FR-005**: The TV MUST read the new settings without a credential, alongside the feature 7
  settings. The public read MUST NOT expose units or any credential, secret or integration
  setting.
- **FR-006**: The TV MUST receive each tap beer's SRM number and description without a
  credential, so the preview can show a detail before it is saved; no other beer field becomes
  public. On deck beers stay as they are.
- **FR-007**: The admin app's Display Settings page MUST show the new settings with their current
  values, save them with the existing settings, and show each validation error beside its field.
- **FR-008**: The TV MUST apply changed settings on its next menu refresh, with no reload, and
  MUST keep its last good settings while the server cannot be reached.
- **FR-009**: The TV MUST lay out taps and on deck items in the chosen layout, keep tap order,
  and show every item within one rotation; the page indicator and page interval MUST work in
  every layout.
- **FR-010**: The TV MUST align the title as set, on every page, and show no title row when the
  title is empty.
- **FR-011**: The TV MUST use the chosen title font for the title and the chosen text font for all
  other TV text. Every font MUST come from the app's own server, and every font choice MUST keep
  text readable at 3 m and keep both themes' 4.5:1 contrast with the theme's own colors.
  Colors the owner picks are not contrast-checked; the preview is how the owner judges them.
- **FR-012**: Each tap MUST show each detail that is switched on and has a value, and MUST leave
  out each detail that is switched off or has no value. SRM shows as a number; the glass color is
  unchanged by the SRM switch.
- **FR-013**: The detail switches MUST apply to tap cards only; on deck cards keep showing name,
  style, ABV and status as today.
- **FR-014**: The new settings and any new public menu fields MUST be described in the API
  contract.
- **FR-015**: Upgrading a database from the previous version MUST keep all data and leave every
  new setting at its default.
- **FR-016**: The Display Settings page MUST show a preview of the TV, in the TV's 16:9 shape,
  that reflects every unsaved TV setting within 1 second of a change and uses the current menu.
  The preview MUST NOT change what the TV shows; only a save does.
- **FR-017**: The preview MUST look the same as the TV would with the same settings and menu,
  apart from scale.
- **FR-018**: The admin app MUST offer a color picker for the title color and the text color,
  each with a way to reset to the theme's own color.
- **FR-019**: The TV MUST use the title color for the title, and the text color for tap and on
  deck text: labels, names, details, status, the "On deck" heading and the "Nothing on tap" and
  "Nothing on deck" lines. Each uses the theme's own color when not set. The page indicator, the
  offline notice, the background, the cards and the artwork stay set by the theme.

### Key Entities

- **Display settings** (extended from feature 7): adds layout, title alignment, title font, text
  font, five tap detail switches, title color and text color. One set for the whole install.
- **Public display config** (extended from feature 7): the TV's subset, now title, theme, page
  interval and every new setting in this feature. Units stay admin-only.
- **Preview**: a scaled copy of the TV view inside Display Settings, drawn from the unsaved form
  values and the current menu. Holds no data of its own.
- **Font choice**: one entry in a fixed list the app offers; the same list serves the title font
  and the text font.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After the owner saves a new setting, the TV shows the change within one menu
  refresh (at most 30 seconds), with nobody touching the TV.
- **SC-002**: A fresh install and an upgraded install look the same on the TV as before this
  feature until a new setting is changed; checked by screenshot comparison.
- **SC-003**: In every layout, with up to 24 taps and 12 on deck items, every item appears within
  one rotation and no text is cut off mid-line except a description cut by design.
- **SC-004**: Every font and theme combination, with the theme's own colors, passes a 4.5:1 text
  contrast check and is readable at 3 m on a 1080p TV.
- **SC-005**: The TV makes no request to any server other than the app's own, shown by an
  automated test or the page's content policy.
- **SC-006**: No new setting can be changed without an admin session or a `write` token, and the
  public read returns only the TV's settings and fields; both shown by automated tests.
- **SC-007**: The owner can change layout, alignment, fonts, colors and details from the admin
  app in under 1 minute.
- **SC-008**: Every unsaved change shows in the preview within 1 second, and a screenshot of the
  preview matches a screenshot of the TV with the same settings, apart from scale.

## Assumptions

- The current layout is named "grid" in settings.
- The title is left-aligned today, so left is the alignment default.
- Detail switches apply to tap cards only, as the brief names them "tap details"; on deck cards
  keep today's details.
- SRM shows as a number with the label "SRM", rounded to a whole number; the glass color stays.
- Fonts ship with the app and are served by its own server; no font is downloaded from another
  site and there are no uploaded fonts. The list is today's font plus 6 to 8 free fonts covering
  serif, sans, condensed, rounded and script styles; the plan picks them.
- The list layouts hold 6 rows a page; the plan may tune this for 3 m readability.
- In vertical list, on deck items follow the taps on their own pages under the "On deck"
  heading, as in grid.
- Today's font stays in the list and is the default for both font settings.
- Settings ride on the TV's existing menu refresh, as in feature 7; no separate push.
- Tap order, the "On deck" heading, the "Empty" tap and the on deck status work as today in
  every layout.
- One title color and one text color apply to both themes; there are no per-theme colors.
- The preview uses the same drawing as the TV, so the two cannot drift apart.
- The admin app's Display Settings page keeps the feature 8 look; this feature adds fields and a
  preview.
