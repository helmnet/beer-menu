# Feature Specification: Styled Confirmation Dialogs

**Feature Branch**: `compmodder26/confirmation_dialog_styling`

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "Confirmation dialogs currently use the default browser confirmation. [Screenshot: the browser's own 'taplist… says / Revoke token "tets"? Clients using it stop working at once. / Cancel / OK' box] They are completely unstyled and detract from the continuity of the app. I want confirmations to be styled just like other modals. Example: [Screenshot: the app's 'Add admin' modal, with a title, a close (X) button top right, and Save / Cancel buttons]"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Confirm a destructive action in an app-styled dialog (Priority: P1)

An admin clicks a button that deletes, removes, revokes, kicks or disconnects something. Instead of the browser's own grey confirmation box (which shows the site's address as its heading and generic "OK" / "Cancel" buttons), the app opens its own dialog. It looks like the other admin dialogs, such as "Add admin": same panel, same title style, same close (X) button in the top right, and the same button styles. The dialog shows the same question the browser box showed. The admin confirms to go ahead, or cancels to leave everything as it was.

**Why this priority**: This is the whole feature. Every confirmation gets the same treatment, so one story covers it.

**Independent Test**: Sign in as an admin, trigger each listed confirmation, and check that an app-styled dialog appears (no browser box), that confirming performs the action, and that cancelling does nothing.

**Acceptance Scenarios**:

1. **Given** an API token exists, **When** the admin clicks Revoke, **Then** an app-styled dialog opens with the message `Revoke token "<name>"? Clients using it stop working at once.`, and no browser confirmation box appears.
2. **Given** the confirmation dialog is open, **When** the admin clicks the confirm button, **Then** the dialog closes and the action runs exactly as it did after "OK" in the browser box.
3. **Given** the confirmation dialog is open, **When** the admin clicks Cancel, clicks the close (X) button, or presses Escape, **Then** the dialog closes and nothing changes.
4. **Given** any of the confirmations listed in FR-004, **When** the admin triggers it, **Then** it opens as an app-styled dialog with the same message text as before.
5. **Given** the confirmation dialog opens, **When** the admin uses only the keyboard, **Then** focus is inside the dialog, Tab moves between its buttons, Enter or Space activates the focused button, and after the dialog closes focus returns to the button that opened it.
6. **Given** a two-factor action whose confirmation is accepted, **When** the server then asks the admin to confirm their identity, **Then** the existing "Confirm it is you" dialog opens and works as before.

---

### Edge Cases

- Cancel is the safe default: when the dialog opens, keyboard focus is on Cancel, so pressing Enter straight away does not perform the destructive action.
- Clicking or tapping outside the dialog does nothing (it neither confirms nor cancels), the same as the existing admin dialogs.
- Double click on the confirm button: the action runs once.
- Light and dark theme, including "follow the system": the dialog stays readable in both, like the other admin dialogs.
- Narrow screens: the dialog fits the screen width and its buttons wrap rather than overflow, like the other admin dialogs.
- A confirmation opened while another admin dialog is already open stacks on top of it and returns to it when closed. No current confirmation opens from inside a dialog; this is covered by the existing dialog stacking behavior.
- Names in messages (beer, keg, token, admin, passkey) are shown as plain text, never interpreted as formatting or markup.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The admin app MUST show every confirmation in its own dialog, styled the same as its other dialogs (panel, title, close button, spacing, fonts and theme colours), instead of the browser's built-in confirmation box.
- **FR-002**: Each confirmation dialog MUST show a short title naming the action (for example "Revoke token"), the same message text the browser box showed, a confirm button and a Cancel button.
- **FR-003**: The confirm button MUST use the red danger button style from feature 12 and be labelled with the action's verb (for example "Revoke", "Delete", "Remove", "Kick", "Disconnect", "Turn off", "Clear"). The Cancel button MUST use the plain button style used by Cancel in other admin dialogs.
- **FR-004**: These confirmations, and every other use of the browser's confirmation box in the admin app, MUST use the new dialog:
  - Beer Library: Delete a beer.
  - Keg History: Delete a keg from history.
  - Taps: Remove a tap.
  - Keg card: Kick a keg.
  - Admin Users: Remove an admin (including removing yourself), and Clear two-factor sign-in for an admin.
  - API Tokens: Revoke a token.
  - Two-Factor Sign-In: Remove a second factor (authenticator app or passkey), and Turn off 2FA.
  - Brewfather: Disconnect.
- **FR-005**: Confirming MUST perform exactly the action the browser box's "OK" performed before. Cancelling (Cancel button, close button or Escape) MUST perform no action and change no data.
- **FR-006**: The dialog MUST be fully usable by keyboard and screen reader: it is announced as a dialog named by its title, focus stays inside it while open, initial focus is on Cancel, and focus returns to the opening button when it closes.
- **FR-007**: After this feature, the admin app MUST NOT call the browser's built-in confirmation box anywhere.
- **FR-008**: No button outside the confirmation dialogs, and no action's behavior, MUST change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 10 confirmations listed in FR-004 open the app-styled dialog (Remove admin has two messages, one for removing yourself); none shows the browser's confirmation box.
- **SC-002**: For each of the 10 confirmations, confirming performs the action and cancelling (each of the 3 ways) changes nothing.
- **SC-003**: An admin can complete or cancel every confirmation using only the keyboard.
- **SC-004**: Side by side with the "Add admin" dialog, the confirmation dialog shows the same panel, title style, close button and button styles in both themes.

## Assumptions

- "Styled just like other modals" means reuse of the admin app's existing dialog look and behavior (the one "Add admin" uses), not a new design.
- The confirm button is red (feature 12's danger style) because every current confirmation guards a destructive action; this matches the red buttons that open most of them.
- Message text stays as it is today; only the container changes. Titles and confirm-button verbs are new short labels chosen per action.
- "New recovery codes" has no browser confirmation today and is out of scope; it keeps its current flow.
- The TV display has no confirmations and is out of scope.
- No API or data changes; this is a front-end change only.
