# Feature Specification: Fix Passkey Prompt Inside Admin Dialogs

**Feature Branch**: `compmodder26/passkey_verificaton_issues`

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "Fix passkey for id verification in browser (mobile works). May be a 1password issue, not sure, but when I have to verify my identity, when adding a new api token for instance, when the 1password passkey dropdown appears, I cannot click the use passkey, nor can I close the dropdown. [Image #1]. I have to either refresh the browser, or close the "Confirm it is you" dialog."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Confirm identity with a passkey from a password manager (Priority: P1)

An admin on a desktop browser starts a sensitive action (for example, creating an API token). The "Confirm it is you" dialog opens. The admin enters their password, picks the passkey proof and clicks Confirm. Their password manager (for example, the 1Password browser extension) shows its "Use passkey" prompt at the top of the page. The admin clicks "Use passkey", completes the check, and the sensitive action goes ahead.

Today the password manager's prompt shows, but the admin cannot click "Use passkey" or its close button. The only ways out are to reload the page or to close the "Confirm it is you" dialog, so a desktop admin with a password-manager passkey cannot confirm with a passkey at all. The same passkey works on mobile, where the device shows its own system prompt.

**Why this priority**: This is the reported bug. It blocks every passkey-confirmed sensitive action on desktop for admins whose passkey lives in a browser-extension password manager.

**Independent Test**: On a desktop browser with the 1Password extension and an admin whose second factor is a 1Password passkey, start creating an API token, confirm with password and passkey, and check that the token is created.

**Acceptance Scenarios**:

1. **Given** an admin with a passkey stored in a browser-extension password manager, **When** they confirm their identity with the passkey proof and the password manager shows its prompt, **Then** they can click "Use passkey" and the confirmation completes.
2. **Given** the password manager's prompt is showing, **When** the admin clicks the prompt's close button, **Then** the prompt closes, the "Confirm it is you" dialog stays open and shows "Passkey check was cancelled.", and the admin can try again or pick another proof.
3. **Given** the password manager's prompt is showing, **When** the admin chooses "Other logins" (or the equivalent option to use another device or the browser's own prompt), **Then** that option works.
4. **Given** an admin whose passkey is not in a browser extension (the browser's or operating system's own passkey prompt, or a phone), **When** they confirm with a passkey, **Then** it works as it does today.

---

### User Story 2 - Add a new passkey from a password manager (Priority: P2)

An admin on a desktop browser opens Two-Factor Sign-In and clicks "Add passkey". The add-passkey dialog opens; they name the passkey and click Add. Their password manager offers to save the new passkey. The admin can interact with that prompt and the passkey is saved and listed.

**Why this priority**: Adding a passkey also runs the device prompt from inside a dialog, so it very likely has the same fault. Without it, an admin cannot store a new passkey in a browser-extension password manager.

**Independent Test**: On a desktop browser with the 1Password extension, add a passkey from Two-Factor Sign-In and save it to 1Password; check that it appears in the Second factors list.

**Acceptance Scenarios**:

1. **Given** the add-passkey dialog is open, **When** the admin clicks Add and the password manager offers to save the passkey, **Then** they can click the save option and the new passkey appears in Second factors.
2. **Given** the password manager's save prompt is showing, **When** the admin dismisses it, **Then** the add-passkey dialog stays open and the admin can try again.

---

### Edge Cases

- Admin presses Escape while the password manager's prompt shows: the prompt's own Escape handling should win; the admin must not lose the dialog and their typed password unless the prompt is already gone.
- Admin clicks outside the password manager's prompt while it shows: the dialog behind must not close or lose what was typed.
- The passkey check times out or the password manager is locked: the dialog stays open and shows the existing "cancelled" message, so the admin can retry or pick another proof.
- Dialogs opened over other dialogs (for example "Confirm it is you" over "New API token") keep their order during and after the passkey check.
- Signing in with a passkey on the sign-in page (not inside a dialog) must keep working as it does today.
- While the passkey check runs, the dialog's own fields and buttons must still be safe: a second click on Confirm must not start a second check.
- Keyboard users: focus and screen-reader behaviour of the dialogs (focus stays in the dialog, the page behind cannot be used, Escape cancels, focus returns to the opening button on close) must stay the same when no password-manager prompt is showing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: While a passkey check runs from the "Confirm it is you" dialog, a passkey prompt that a browser-extension password manager adds to the page MUST accept clicks and keyboard input, including its "Use passkey", close and "Other logins" controls.
- **FR-002**: While a new passkey is being created from the add-passkey dialog, a password manager's save prompt MUST accept clicks and keyboard input in the same way.
- **FR-003**: Cancelling or closing the password manager's prompt MUST leave the dialog open, keep what the admin typed, and show the existing cancelled message, so the admin can retry or switch proof.
- **FR-004**: A successful passkey check from the dialog MUST complete the confirmation (or the passkey addition) the same way it does today on mobile.
- **FR-005**: When no passkey check is running, the dialogs MUST keep their current behaviour: the page behind cannot be used, keyboard focus stays in the dialog, Escape and the Close button cancel, and focus returns to the opening button on close.
- **FR-006**: Passkey sign-in on the sign-in page, and passkey use through the browser's or operating system's own prompt, MUST keep working unchanged.
- **FR-007**: The fix MUST NOT weaken the identity check: the password and the passkey proof are still both required, and the server-side checks do not change.
- **FR-008**: When dialogs are stacked, every dialog that was open when the passkey check started MUST, once the check ends, be open again in the same order, with only the top one usable.
- **FR-009**: While a passkey check runs, mouse clicks MUST NOT reach the page behind the dialogs; the top dialog and the password manager's prompt stay usable.
- **FR-010**: Closing the dialog that started a passkey check (Cancel, Close or Escape) MUST cancel that check, so the password manager's prompt goes away and nothing is confirmed or saved afterwards.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a desktop browser with the 1Password extension, an admin confirms a sensitive action (such as creating an API token) with a 1Password passkey in one attempt, with no page reload and without closing the dialog.
- **SC-002**: On the same setup, an admin adds a new passkey saved to 1Password in one attempt, and it appears in Second factors.
- **SC-003**: Closing the password manager's prompt returns the admin to a usable dialog 100% of the time, with their typed password still present.
- **SC-004**: Passkey confirmation on mobile and with the browser's own passkey prompt passes the same manual checks as before the fix (no regressions).
- **SC-005**: All existing automated dialog and two-factor tests still pass.

## Assumptions

- The fault lies in how the admin app's dialogs block the rest of the page: the password manager draws its prompt on the page itself, outside the dialog, so the dialog's "page behind is unusable" rule also blocks the prompt. Mobile works because the phone shows a system prompt that is not part of the page. This must be confirmed during planning; if the fault is solely in the password manager, the outcome is a documented workaround instead of a code change.
- The 1Password browser extension on a current desktop Chromium or Firefox browser is the reference setup; desktop Safari is expected to work but is not separately verified; other extension-based password managers (Bitwarden, Dashlane and similar) are expected to benefit from the same fix but are not separately verified.
- The add-passkey flow (User Story 2) shares the cause, because it also runs the passkey prompt from inside a dialog; if testing shows it already works, Story 2 needs only a regression check.
- No server or API change is expected; this is an admin web app fix.
- Verification of the password-manager interaction is manual, because automated tests cannot run a real browser extension.
