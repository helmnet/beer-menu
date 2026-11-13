# Research: Fix Passkey Prompt Inside Admin Dialogs

## R1. Root cause (confirmed)

**Decision**: The fault is in the admin app, not only in 1Password. `Modal` opens its native
`<dialog>` with `showModal()` (feature 008, research R3). While a modal dialog is open, the
browser makes every element outside it *inert*, meaning it takes no clicks, focus or keyboard
input. The 1Password extension draws its "Use passkey" / save prompt as its own elements in the
page, outside our dialog, so the browser blocks them. The prompt still paints on top of the
page. That is why it looks usable but ignores clicks, including on its own close button.

**Rationale**:
- Mobile works: the phone shows an OS sheet that is not part of the page.
- Passkey sign-in works: `LoginForm` is not in a dialog, so nothing is inert.
- Closing the dialog fixes it at once: no modal dialog, no inert page.
- Both passkey flows that run inside a `Modal` are hit: `ConfirmIdentity` (`getPasskey`) and
  `TwoFactorSetup` "Add passkey" (`createPasskey`).

**Spike result (2026-11-13, owner, desktop + 1Password)**:
- Lowering only the first open dialog failed. "Confirm it is you" opens over "New API token"
  (two modals), and the one still modal kept blocking everything.
- Lowering every open dialog (`document.querySelectorAll("dialog[open]").forEach(d => { d.close();
  d.show(); })`) made the 1Password "Use passkey" prompt clickable. R1 holds.
- The spike exposed a stacking problem: once lowered, "New API token" painted above "Confirm it
  is you". Non-modal dialogs stack by DOM order, and `TokensPanel` renders `identity.dialog`
  before its own `Modal`. See R2.

**Alternatives considered**:
- 1Password bug only: rejected. The inert rule explains every observation, and the spike proves
  it.

## R2. Fix: lower every open modal while the passkey prompt runs, in stack order

**Decision**: `web/src/components/Modal.tsx` keeps a module-level stack of open dialogs, in the
order they were opened. The layout effect pushes after `showModal()`, and its cleanup removes the
dialog. `Modal.tsx` also exports `lowerModals(): () => void`, which does three things:
- Switches every dialog on the stack, bottom first, to non-modal (`close()`, then `show()`).
- Adds class `modal-lowered` to each dialog, and `modal-lowered-top` to the last one.
- Returns `restore()`. It removes those classes and switches each dialog that is still
  connected and still on the stack back to modal, bottom first (`close()`, then
  `showModal()`), so the original top-layer order comes back. It then returns focus to the
  element that had it, if that element was not `document.body` and is still connected.

`prompt()` in `web/src/webauthn.ts` calls `const restore = lowerModals()` before `run()` and
`restore()` in a `finally` block.

**Rationale**:
- Lowering every dialog is required, because any dialog left modal blocks the prompt (spike).
- Neither DOM order nor `dialog[open]` tells which dialog is on top. Only the opening order
  does, and `Modal` is the one place that knows it. Restoring in DOM order would leave
  "New API token" above "Confirm it is you" after the check.
- One call site serves both passkey flows. `ConfirmIdentity`, `TwoFactorSetup`, `LoginForm` and
  every caller of `Modal` stay unchanged.
- Outside the prompt, dialogs behave exactly as today (FR-005).
- A non-modal dialog ignores Escape, so 1Password's own Escape handling wins and the typed
  password stays (spec edge case).
- Typed values are React state, so they survive (FR-003). The identity check is unchanged
  (FR-007).

**Trade-off (accepted)**: while the prompt is open, which lasts seconds, the page behind is not
inert. The top dialog's scrim covers it visually, but a deliberate click could reach it. Every
action back there keeps its own confirm. FR-005 asks for inertness only when no passkey check
is running.

**Alternatives considered**:
- Lower only `dialog[open]` in DOM order. This failed in the spike, and it restores the dialogs
  in the wrong order.
- Use inline `style.zIndex` per depth. That adds style attributes against the project rule (the
  CSP has no `style-src`), and two classes already cover it.
- Replace `showModal()` with `show()` plus a hand-made focus trap, always. That is more code and
  loses native inertness for every dialog.
- Add a `modal` prop on `Modal`, driven by `prompting` state in each caller. It duplicates state
  in two callers and still does not solve stacking.
- Set `inert` on the page siblings. The 1Password element can be a sibling of our root, so it
  would be blocked again.

## R3. Look while lowered

**Decision**: Two rules go under the existing dialog styles in `web/src/pages/AdminPage.css`:
- `.admin dialog.modal.modal-lowered` sets `inset: 0; z-index: 1000;`, which keeps the dialog
  centred and above the admin layout (whose highest `z-index` is 40).
- `.admin dialog.modal.modal-lowered-top` sets `z-index: 1001;` and paints the scrim as
  `box-shadow: 0 0 0 100vmax var(--scrim, rgba(11, 12, 15, 0.55))`, because `::backdrop`
  exists only for modal dialogs.

**Rationale**: the result looks the same as the modal stack (the top dialog over a scrim, with
lower dialogs dimmed under it). No `style` attributes are used.

## R4. Tests

**Decision**:
- `web/src/test/setup.ts`: jsdom has no `HTMLDialogElement.show()`. Add a stub that sets `open`,
  in its own `!show` guard beside the existing `showModal`/`close` stubs. Without it, the
  existing passkey component tests throw.
- `web/src/components/Modal.test.tsx` tests `lowerModals` with two nested `Modal`s, the second
  opened after the first:
  - Lowering: `show()` is called for both, both get `modal-lowered`, and only the second gets
    `modal-lowered-top`.
  - Restore: classes are removed, and `showModal()` is called first for the first dialog, then
    for the second.
  - A `Modal` unmounted while lowered is not reopened.
  - Focus returns to the saved element, even when the test moves focus away while lowered.
  - With no open `Modal`, `lowerModals()` and its `restore()` do nothing.

  These tests fail without the fix (constitution IV).
- `web/src/webauthn.test.ts`: `vi.mock("./components/Modal")` with a spy `lowerModals` that
  returns a spy `restore`. For `getPasskey` and `createPasskey`, check that lowering happens
  before the credential call and `restore` runs after it. Cover success, `"cancelled"`
  (`NotAllowedError`), `InvalidStateError` and any other thrown error.
- `ConfirmIdentity.test.tsx`, `TwoFactorSetup.test.tsx` and `LoginForm.test.tsx` are unchanged
  and must still pass.
- The real 1Password interaction is checked by hand (quickstart), because tests cannot load an
  extension.

Run with `npx vitest run --maxWorkers=2`.
