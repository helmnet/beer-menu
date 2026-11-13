# Research: Styled Confirmation Dialogs

## R1. Where the dialog comes from

- **Decision**: A new `ConfirmDialog` built on the existing `Modal` component (`web/src/components/Modal.tsx`).
- **Rationale**: `Modal` already gives the "Add admin" look and the required behavior: native `<dialog>` opened with `showModal()` (inert page, focus kept inside), `role="dialog"` named by its `h2` title, Escape and the close (X) button call `onClose`, a click outside does nothing, focus returns to the opener on unmount, and stacking over other open modals (FR-001, FR-006, spec edge cases).
- **Alternatives considered**: a third-party dialog library (new dependency, different look; rejected by Principle VII); a separate `<dialog>` with its own styling (duplicates `Modal`).

## R2. How call sites ask

- **Decision**: A hook `useConfirmAction()` in `web/src/components/ConfirmDialog.tsx` returns `{ ask, dialog }`. `ask({ title, message, confirmLabel }, action)` shows the dialog. Confirm unmounts the dialog, then calls `action`. Cancel, close (X) and Escape unmount it and drop `action`. The caller renders `{dialog}`.
- **Rationale**: Same shape as the existing `useConfirm` in `ConfirmIdentity.tsx` (`identity.ask(action)` and `{identity.dialog}`), so the panels read consistently. Each call site changes from `if (!window.confirm(msg)) return; run(...)` to `confirm.ask({...}, () => run(...))` with no async rewrite.
- **Alternatives considered**:
  - A promise API (`await confirm(...)`): it makes 10 sync handlers async, and an unresolved promise leaks when the panel unmounts mid-dialog.
  - A global provider and context at the admin root: it adds a provider and a portal or root mount for no gain, because every caller already renders its own dialogs.
- **Naming**: `useConfirm` is taken by the identity hook, so the new hook is `useConfirmAction`.

## R3. Initial focus on Cancel

- **Decision**: `ConfirmDialog` focuses its Cancel button in a `useEffect` through a ref.
- **Rationale**: `showModal()` focuses the first focusable element, which is the confirm button in the standard row order (primary first, as "Add admin" has Save then Cancel). Child layout effects (`Modal`'s `showModal()`) run before the parent's passive effects, so the `useEffect` runs last and wins. `Modal` reads the opener in its layout effect, before this moves focus, so focus still returns to the opening button.
- **Alternatives considered**:
  - The `autoFocus` prop: React 19 calls `focus()` at commit instead of writing the attribute, so `showModal()` then moves focus to the first focusable element.
  - Putting Cancel first in the row: this breaks consistency with the other dialogs.

## R4. Double activation

- **Decision**: Confirm clears the pending request before it runs the action. No extra guard is needed.
- **Rationale**: React flushes discrete events such as clicks synchronously. After the first click, the dialog is unmounted before a second click can reach it. Each action's own `busy` handling stays as it is.

## R5. Message safety

- **Decision**: Render title and message as React text children.
- **Rationale**: React escapes text, so names (beer, keg, token, admin, passkey) never become markup (spec edge case). The CSP is unaffected. No `style` attributes are used.

## R6. Keeping `window.confirm` out (FR-007)

- **Decision**: One test in `ConfirmDialog.test.tsx` loads every non-test `src/**/*.{ts,tsx}` file as raw text with Vite's `import.meta.glob(..., { query: "?raw", eager: true })`. It asserts that no file contains `confirm(` called on `window`.
- **Rationale**: The project has no ESLint (`lint` is `tsc --noEmit`). Adding ESLint for one rule is heavier than a 10-line test.
- **Alternatives considered**: Adding ESLint with `no-restricted-globals` (new dependency and config); a review checklist only (not enforced).

## R7. Existing tests

- **Decision**: The 9 test files that stub `window.confirm` (`AdminsPanel`, `TokensPanel`, `BeersPanel`, `BrewfatherPanel`, `KegCard`, `KegHistory`, `TapsPanel`, `TwoFactorPanel`, `OnDeckPanel`) now open the dialog. Each test clicks Cancel and expects no request. It then reopens the dialog, clicks the confirm button and expects the request.
- **Rationale**: These tests keep the existing coverage for "only after confirmation" and also prove FR-005 through the real dialog.
- **Shared helper**: Many tests click a button inside a named dialog. If a short `within(getByRole("dialog", { name })).getByRole("button", { name })` call repeats often, extract one helper into the existing test utilities. Do not copy it into each file.
