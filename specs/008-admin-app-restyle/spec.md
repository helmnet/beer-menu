# Feature Specification: Admin App Restyle

**Feature Branch**: `compmodder26/feature_8`

**Created**: 2026-11-10

**Status**: Draft

**Input**: User description: "Feature 8: Admin app restyle. Source of truth: docs/project-brief.md section \"Admin app (feature 8, after the MVP)\" and breakdown row 8. Full visual and layout overhaul of the admin app so it feels modern and works as well on a phone at the kegerator as on a laptop. Responsive layout (phone + laptop). Theme choice light / dark / system (default system), saved per browser, separate from the TV theme. Royal blue accent in both themes; dark theme uses black/dark grey backgrounds. Side navigation panel: always visible and resizable on laptop (width saved per browser), behind a hamburger menu on phone. Items show icon + text; only top-level items at first; selecting a top-level item opens its submenu and collapses any other. Tree: Menu (Taps [default page after sign-in], On Deck [waiting kegs plus fermenting and conditioning beers, matching the TV], Keg History, Beer Library, Display Settings [TV title, TV theme, page interval, units]), Administration (Admin Users, API Tokens, Change Password), Integrations (Brewfather: connection + sync interval card, batches card). Bottom of panel: app version (from GET /api/health) and Sign out. Selecting an item shows only its cards. Forms open in modal dialogs: add keg (from Taps and On Deck), add and edit beer, add tap, new API token (owner may move some back inline later). Beer pickers searchable: typing filters existing beers only (no create), listed newest first by date added (manual or Brewfather import). Component/CSS library chosen in the plan under constitution dependency rules. Web-only change; TV display (/display) must look unchanged. Done when screenshots at laptop and phone widths, both themes, match this."

## Context

Features 1 to 7 built the admin app as one long, unstyled page: every panel (taps, on deck, keg
history, beer library, display settings, admins, API tokens, password, Brewfather) is stacked
in a single column with browser-default look. It works, but it is slow to scan on a laptop and
hard to use on a phone standing at the kegerator.

This feature gives the admin app a modern look and a navigation structure. Everything the admin
app can do today it can still do afterwards; this feature changes where things are and how they
look, and adds two small conveniences the owner asked for: an On Deck page that matches what
the TV shows, and searchable beer pickers.

Nothing changes for the TV display (`/display`) or for API clients. The data the admin app
needs is already available from the API.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Find any admin task from a side panel (Priority: P1)

The owner signs in on a laptop and lands on the Taps page. A side panel on the left lists three
groups: Menu, Administration and Integrations, each with an icon and a name. The owner selects
Administration; it opens to show Admin Users, API Tokens and Change Password, and the Menu group
closes. The owner selects API Tokens and the main area shows only the API tokens card. The
owner drags the panel's edge to make it wider; after a reload the panel keeps that width. At
the bottom of the panel the owner sees the app version and a Sign out button.

**Why this priority**: navigation is the frame every other part of the restyle sits in. Without
it the app is still one long page.

**Independent Test**: sign in at laptop width, open each group and each page in turn, and
confirm each page shows only its own cards, only one group is open at a time, the panel width
survives a reload, and the version and Sign out are visible.

**Acceptance Scenarios**:

1. **Given** the owner has just signed in, **When** the admin app loads, **Then** the Taps page
   is shown and the Menu group is open with Taps marked as the current page.
2. **Given** the side panel, **When** it first shows, **Then** only the three group items (Menu,
   Administration, Integrations) and the open group's pages are listed, each with an icon and
   text.
3. **Given** the Menu group is open, **When** the owner selects Administration, **Then**
   Administration opens and Menu closes.
4. **Given** any page is selected, **When** the main area renders, **Then** it shows only that
   page's cards: Taps (taps), On Deck (on-deck list), Keg History, Beer Library, Display
   Settings (TV title, TV theme, page interval, units), Admin Users, API Tokens, Change
   Password, Brewfather (a connection and sync interval card, and a batches card).
5. **Given** a laptop-width window, **When** the owner drags the panel edge, **Then** the panel
   resizes within sensible limits, and after a reload in the same browser it keeps that width.
6. **Given** any page, **When** the owner looks at the bottom of the side panel, **Then** the
   app version reported by the server and a Sign out button are shown; Sign out ends the
   session and returns to the sign-in screen.

---

### User Story 2 - Use the admin app on a phone (Priority: P1)

The owner stands at the kegerator with a phone, opens the admin app and signs in. The page fits
the screen with no sideways scrolling. The side panel is hidden behind a menu button in the top
bar. The owner taps the menu button, picks On Deck, the menu closes, and the On Deck page fills
the screen. The owner can still reach Sign out and see the version from the menu.

**Why this priority**: the brief's main reason for the restyle is phone use at the kegerator.

**Independent Test**: at phone width, sign in, open the menu, visit every page, confirm no page
scrolls sideways, every control can be tapped, and Sign out and the version are reachable from
the menu.

**Acceptance Scenarios**:

1. **Given** a phone-width screen, **When** the admin app loads, **Then** the side panel is
   hidden and a menu button is shown in a top bar.
2. **Given** the menu is open, **When** the owner selects a page, **Then** the menu closes and
   that page is shown.
3. **Given** the menu is open, **When** the owner taps outside it or presses Escape, **Then** it
   closes without changing the page.
4. **Given** any page at phone width, **When** it renders, **Then** nothing scrolls sideways
   except wide tables, which scroll inside their own card.
5. **Given** a phone-width screen, **When** the menu is open, **Then** the version and Sign out
   are reachable in it.

---

### User Story 3 - Pick light, dark or system theme (Priority: P2)

The owner prefers a dark admin app at night. They pick "Dark" from a theme control in the side
panel, and the admin app turns black and dark grey with royal blue accents. On another device
they leave the default, "System", and the app follows that device's light or dark setting. The
TV keeps its own theme, set under Display Settings.

**Why this priority**: the brief asks for it, and it shapes every visual decision, but the app
is usable before it exists.

**Independent Test**: switch between Light, Dark and System, reload, and confirm the choice is
kept per browser, that System follows the device setting, and that the TV theme setting does
not change.

**Acceptance Scenarios**:

1. **Given** a browser that never chose a theme, **When** the admin app loads, **Then** it uses
   System, following the device's light or dark preference.
2. **Given** the owner picks Dark, **When** the app re-renders and after a reload, **Then** it
   shows black or dark grey backgrounds with a royal blue accent.
3. **Given** the owner picks Light, **Then** it shows light backgrounds with the same royal blue
   accent.
4. **Given** System is chosen, **When** the device switches between light and dark, **Then** the
   admin app follows without a reload.
5. **Given** any admin theme choice, **When** the TV display loads, **Then** it uses the TV
   theme from Display Settings, unaffected.
6. **Given** the sign-in and first-admin setup screens, **When** they show, **Then** they use the
   same theme and styling as the rest of the admin app.

---

### User Story 4 - Add and edit in modal dialogs (Priority: P2)

On the Taps page the owner selects "Add keg". A dialog opens over the page with the keg form.
The owner fills it and saves; the dialog closes and the new keg appears. The same happens for
adding a keg from On Deck, adding and editing a beer in the Beer Library, adding a tap, and
creating a new API token, where the dialog shows the new token once before closing.

**Why this priority**: forms inline in cards make long pages on a phone; dialogs keep the list
visible and focus on one task. The owner may move some back inline after trying it.

**Independent Test**: open each of the five dialogs, cancel one, submit one with a validation
error, and submit one successfully; confirm focus, error display and list refresh.

**Acceptance Scenarios**:

1. **Given** the Taps or On Deck page, **When** the owner selects Add keg, **Then** a dialog
   opens with the keg form.
2. **Given** the Beer Library, **When** the owner selects Add beer or Edit on a beer, **Then** a
   dialog opens with the beer form, filled in for Edit.
3. **Given** the Taps page, **When** the owner selects Add tap, **Then** a dialog opens with the
   tap form.
4. **Given** the API Tokens page, **When** the owner selects New token and submits, **Then** the
   dialog shows the new token once with a way to copy it, and the token is not shown again after
   the dialog closes.
5. **Given** an open dialog, **When** the owner presses Escape or selects Cancel, **Then** it
   closes without saving, and focus returns to the button that opened it.
6. **Given** an open dialog, **When** the server rejects the input, **Then** the dialog stays
   open and shows the errors next to the fields, as the inline forms do today.
7. **Given** a successful save, **When** the dialog closes, **Then** the page's list shows the
   change without a manual reload.

---

### User Story 5 - See On Deck as the TV shows it (Priority: P2)

The owner opens On Deck and sees the same "coming soon" list the TV shows: kegs waiting to go
on tap, then beers conditioning, then beers fermenting that have no keg yet. From this page the
owner can add a keg.

**Why this priority**: the owner treats "on deck" and the TV's "coming soon" as the same thing;
today the admin app shows only waiting kegs, so the two disagree.

**Independent Test**: with a waiting keg, a conditioning beer and a fermenting beer without a
keg, compare the admin On Deck page with the TV's coming-soon list: same items, same order.

**Acceptance Scenarios**:

1. **Given** waiting kegs, conditioning beers and fermenting beers without a keg, **When** the
   owner opens On Deck, **Then** the page lists them in the same order as the TV.
2. **Given** a fermenting beer that already has a keg, **When** On Deck renders, **Then** it is
   listed once, as its keg, as on the TV.
3. **Given** nothing is on deck, **When** the page renders, **Then** it says so plainly.
4. **Given** On Deck, **When** the owner selects Add keg, **Then** the add keg dialog opens.

---

### User Story 6 - Find a beer by typing (Priority: P3)

The owner adds a keg and needs to pick the beer. The library has dozens of beers. The owner
types "ip" into the beer picker and the list narrows to beers whose name contains "ip". The
newest beers, whether typed in by hand or imported from Brewfather, are at the top.

**Why this priority**: a convenience that grows in value as the library grows.

**Independent Test**: with several beers added at different times, open a beer picker, confirm
newest-first order, type part of a name, confirm the list filters, and confirm no option creates
a new beer.

**Acceptance Scenarios**:

1. **Given** a beer picker, **When** it opens, **Then** beers are listed newest first by date
   added.
2. **Given** a beer picker, **When** the owner types text, **Then** only beers whose name
   contains it (ignoring case) are listed.
3. **Given** typed text that matches no beer, **When** the list renders, **Then** it says no
   beer matches and offers no way to create one.
4. **Given** a beer picker, **When** used by keyboard only, **Then** the owner can type, move
   through matches and choose one.

---

### Edge Cases

- The version request fails or the server reports no version: the panel shows the rest of its
  content and an "unknown" version, without an error banner.
- The saved panel width is larger than the current window (window made smaller, or the saved
  value is from a wider screen): the panel is clamped so the main area stays usable.
- The browser blocks local storage (private mode): theme and panel width fall back to System and
  the default width for that visit; nothing breaks.
- The session expires while a dialog is open: saving returns the owner to sign-in, as the inline
  forms do today.
- The window is resized across the phone/laptop boundary while the menu is open: the layout
  switches cleanly and no page content is lost.
- A beer picker in a form that edits an existing keg whose beer is not in the filtered list: the
  current beer stays selected.
- A dialog contains unsaved changes and the owner presses Escape: it closes without saving, as
  Cancel does (no unsaved-changes prompt).
- The TV display is opened in the same browser where the admin theme is set: the TV is
  unaffected.

## Requirements *(mandatory)*

### Functional Requirements

**Layout and navigation**

- **FR-001**: The admin app MUST show a side panel with three top-level items, each with an icon
  and text: Menu, Administration, Integrations.
- **FR-002**: Menu MUST contain Taps, On Deck, Keg History, Beer Library and Display Settings;
  Administration MUST contain Admin Users, API Tokens and Change Password; Integrations MUST
  contain Brewfather. Each page item MUST show an icon and text.
- **FR-003**: Only top-level items MUST show at first, plus the group of the current page.
  Opening a group MUST close any other open group.
- **FR-004**: Selecting a page MUST show only that page's cards in the main area and mark the
  page as current in the side panel.
- **FR-005**: Taps MUST be the page shown after sign-in.
- **FR-006**: The bottom of the side panel MUST show the app version reported by the server's
  health check and a Sign out button.
- **FR-007**: At laptop width the side panel MUST always be visible and resizable by dragging its
  edge, between a minimum and maximum width, and the width MUST be saved per browser.
- **FR-008**: At phone width the side panel MUST be hidden behind a menu button in a top bar; it
  MUST close when a page is selected, when the owner taps outside it, or on Escape.
- **FR-009**: Pages MUST NOT scroll sideways at phone width, except wide tables inside their own
  card.

**Theme and look**

- **FR-010**: The admin app MUST offer Light, Dark and System themes, default System, chosen from
  a control in the side panel and saved per browser.
- **FR-011**: System MUST follow the device's light or dark preference, including changes while
  the app is open.
- **FR-012**: Both themes MUST use a royal blue accent; the dark theme MUST use black or dark grey
  backgrounds.
- **FR-013**: The sign-in and first-admin setup screens MUST use the same theme and styling.
- **FR-014**: The admin theme MUST NOT change the TV display, and the TV display MUST look the
  same as before this feature.

**Pages**

- **FR-015**: Display Settings MUST show the TV title, TV theme, page interval and units, as
  today.
- **FR-016**: Brewfather MUST show two cards: connection and sync interval; and batches.
- **FR-017**: On Deck MUST list waiting kegs, then conditioning beers, then fermenting beers
  without a keg, the same items in the same order as the TV's coming-soon list.
- **FR-018**: Every action the admin app offers today MUST still be available after the restyle:
  taps (add, rename, enable or disable, move up or down, remove, add a keg or put a waiting keg
  on an empty tap); kegs on a tap (pour, set level, kick, take off, edit); waiting kegs (edit,
  kick; owner decision 2026-11-10: pour and set level dropped for kegs not on a tap, since edit
  still sets the remaining volume); keg history (delete); beers
  (show archived, add, edit, re-sync, archive, restore, delete); display settings; admin users
  (add, remove); API tokens (create, copy once, revoke); change password; Brewfather (connect,
  disconnect, sync interval, sync now, refresh batches, import selected); sign out.

**Dialogs**

- **FR-019**: These forms MUST open in modal dialogs: add keg (from Taps and from On Deck), add
  beer, edit beer, add tap, new API token. All other forms stay in their cards.
- **FR-020**: A dialog MUST have a visible title, trap keyboard focus while open, close on Escape
  or Cancel without saving, and return focus to the control that opened it.
- **FR-021**: A dialog MUST stay open and show field errors when the server rejects the input,
  and MUST close and refresh the page's list on success.
- **FR-022**: The new API token dialog MUST show the token once, with a copy control, and MUST NOT
  show it again after closing.

**Beer pickers**

- **FR-023**: Every control for choosing a beer MUST be searchable: typing filters existing beers
  by name, ignoring case.
- **FR-024**: Beer pickers MUST list beers newest first by date added, whether entered by hand or
  imported from Brewfather.
- **FR-025**: A beer picker MUST NOT offer to create a beer.

**Accessibility and quality**

- **FR-026**: The side panel, menu button, dialogs and beer pickers MUST be fully usable by
  keyboard, with a visible focus indicator.
- **FR-027**: Text and controls MUST meet WCAG 2.1 AA contrast in both themes.
- **FR-028**: Touch targets at phone width MUST be at least 44 by 44 CSS pixels.
- **FR-029**: This feature MUST NOT change the API or stored data.

### Key Entities

- **Admin theme preference**: Light, Dark or System; stored in the owner's browser only; default
  System. Separate from the TV theme setting stored on the server.
- **Side panel width**: a width stored in the owner's browser only; clamped to a minimum and
  maximum.
- **Navigation page**: one of the nine pages above, each with a group, an icon, a name and the
  cards it shows.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From any page, the owner reaches any other page in at most 2 selections on a laptop
  and at most 3 taps on a phone (menu, group, page).
- **SC-002**: Screenshots of every page at laptop width (1920 by 1080 and 1366 by 768) and phone
  width (390 by 844), in both light and dark themes, show the layout described here with no
  sideways scrolling and no overlapping or cut-off controls.
- **SC-003**: The TV display renders the same before and after this feature in a side-by-side
  screenshot comparison.
- **SC-004**: All text and controls pass WCAG 2.1 AA contrast in both themes.
- **SC-005**: The owner can add a keg, choosing a beer by typing part of its name, in under 30
  seconds on a phone.
- **SC-006**: On Deck lists the same items in the same order as the TV in every test case.
- **SC-007**: Every admin action available before this feature still works after it, shown by
  the existing behavior tests passing after they are updated to the new navigation and dialogs.

## Assumptions

- Phone width means narrower than about 768 CSS pixels; laptop width means 768 and wider. The
  exact breakpoint is a plan decision.
- The theme control sits in the side panel near the version and Sign out, so it is reachable on
  a phone. Its exact form (buttons or a menu) is a plan decision.
- The current page is not kept in the address bar: a reload returns to Taps, and the browser
  Back button is not used for in-app navigation.
- The beer picker shows all matching beers regardless of status, as the current keg form does.
- "Date added" is the beer's creation time on the server, which both manual entry and Brewfather
  import set. No API change is needed; sorting happens in the admin app.
- The On Deck data comes from what the API already offers (the public menu's on-deck list or the
  beer and keg lists); no API change is needed.
- Icons are simple line icons; no custom artwork or image uploads (the no-uploads decision
  stands).
- Edit keg, set level, rename tap, admin add, password, display settings and Brewfather forms stay
  inline in their cards.
- The component or CSS library, and any icon set, are chosen in the plan under the constitution's
  dependency rules (justified, release at least two weeks old). Styles must work under the
  current content security policy and must not leak between the admin app and the TV display.
- The owner reviews the dialogs after trying them and may move some back inline in a later change.
