# Feature Specification: Danger Button Styling

**Feature Branch**: `compmodder26/danger_buttons_styling`

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "I want to add "danger" styling to certain buttons. They should be styled like the regular blue buttons on the site, but should be red instead (may need to alter text color so there's proper contrast). These are the following buttons I want styled: Beer library: Delete button in each keg row; Admin Users: Remove button in each row; API Tokens: Revoke button; Two-Factor Sign-In: Turn off 2FA, Remove button in each Second Factors row, New Recovery Codes; Brewfather: Disconnect"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Spot destructive actions at a glance (Priority: P1)

An admin working in the admin app sees buttons that delete, remove, revoke, disconnect or replace something shown in red, filled the same way as the blue primary buttons. Before clicking, the admin can tell these actions destroy or invalidate something, so they are less likely to click one by mistake.

**Why this priority**: This is the whole feature. Every listed button gets the same treatment, so one story covers it.

**Independent Test**: Sign in as an admin, open each listed page, and check that each listed button is red-filled with readable text, while every other button looks as before.

**Acceptance Scenarios**:

1. **Given** the Beer Library has at least one beer, **When** the admin opens the Beer Library page, **Then** the Delete button in each row is a red-filled button and the Edit, Re-sync, Archive and Restore buttons look as before.
2. **Given** there is at least one kicked keg, **When** the admin opens the Keg History page, **Then** the Delete button in each keg row is red-filled.
3. **Given** there are at least two admins, **When** the admin opens Admin Users, **Then** the Remove button in each row is red-filled.
4. **Given** at least one API token exists, **When** the admin opens API Tokens, **Then** each Revoke button is red-filled.
5. **Given** the admin has two-factor sign-in turned on, **When** they open Two-Factor Sign-In, **Then** the Turn off 2FA button, the Remove button for each second factor (authenticator app and each passkey) and the New recovery codes button are red-filled, and Rename and the other buttons look as before.
6. **Given** Brewfather is connected, **When** the admin opens the Brewfather page, **Then** the Disconnect button is red-filled.
7. **Given** any listed button, **When** the admin hovers over it, focuses it with the keyboard, or it is disabled, **Then** it gives the same hover, focus and disabled feedback as the blue primary button, in red.

---

### Edge Cases

- Light and dark theme: the red button and its text must stay readable in both themes, including the "follow the system" theme setting.
- Disabled state: a disabled danger button (for example while a request runs) must still look disabled, the same way a disabled primary button does.
- Buttons that already ask for confirmation (both Deletes, Remove, Revoke, Turn off 2FA, Disconnect, New recovery codes) keep their existing confirmation steps; only their look changes.
- Narrow screens: a red button in a table row wraps and sizes the same way the button did before.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The admin app MUST offer a "danger" button style that matches the primary (blue, filled) button in shape, size, spacing, font and border, but uses a danger red as its fill and border: the light-theme danger red (`#c0352b`), the same in both themes.
- **FR-002**: Danger button text MUST have a contrast ratio of at least 4.5:1 against the button fill, in both the light and the dark theme.
- **FR-003**: The danger button MUST show hover, keyboard focus and disabled states consistent with the primary button.
- **FR-004**: These buttons, and only these, MUST use the danger style:
  - Beer Library: the Delete button in each beer row.
  - Keg History: the Delete button in each keg row.
  - Admin Users: the Remove button in each row.
  - API Tokens: the Revoke button in each row.
  - Two-Factor Sign-In: Turn off 2FA; the Remove button for the authenticator app and for each passkey under Second factors; New recovery codes.
  - Brewfather: Disconnect.
- **FR-005**: The behavior of every restyled button (what it does, its label, its confirmation prompt) MUST NOT change.
- **FR-006**: Every other button in the admin app and the TV display MUST look exactly as before.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 9 listed button kinds (Beer Library Delete, Keg History Delete, Admin Users Remove, API Tokens Revoke, Turn off 2FA, authenticator Remove, passkey Remove, New recovery codes, Brewfather Disconnect) show the danger style, checked on every listed page.
- **SC-002**: Danger button text meets a contrast ratio of at least 4.5:1 in both themes.
- **SC-003**: No button outside the list changes its look.
- **SC-004**: Every restyled button performs the same action, with the same confirmation, as before.

## Assumptions

- "Beer library: Delete button in each keg row" covers both the Delete button in each beer row of the Beer Library page and the Delete button in each keg row of the Keg History page (confirmed by the owner).
- "Regular blue buttons" means the filled primary buttons (for example "Add beer"), not the plain outlined buttons the listed buttons use today.
- The admin app already defines a danger red for each theme (used for error text). The dark-theme red is too light for white text (3.70:1), so the button uses the light-theme red in both themes, as the blue accent does.
- Only the admin app is in scope; the TV display has no destructive buttons.
- No API or data changes; this is a visual change only.
