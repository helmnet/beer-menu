# Tasks: Styled Confirmation Dialogs

**Input**: Design documents from `specs/014-styled-confirm-dialogs/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [contracts/confirm-dialog.md](./contracts/confirm-dialog.md), [quickstart.md](./quickstart.md)

**Tests**: These are included because Constitution IV requires component tests for components with logic. Research R6 and R7 define them.

**Organization**: The spec has one user story (US1, P1). Every confirmation gets the same treatment, so all call-site work is in that one phase.

**Running tests**: Always use `npx vitest run --maxWorkers=2` from `web/`. Run only the touched files while iterating, for example `npx vitest run --maxWorkers=2 src/components/ConfirmDialog.test.tsx`.

**No duplication**: Every call site uses `useConfirmAction` and does not build its own dialog. Test helpers that repeat across files go in `web/src/testApi.ts` (T002) and are not copied per file.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: The task can run in parallel with other tasks (different files, no dependencies).
- **[Story]**: The user story the task belongs to (US1).

## Phase 1: Setup

There is nothing to set up: no new dependencies, tooling or config.

---

## Phase 2: Foundational (blocks US1)

- [X] T001 Write `web/src/components/ConfirmDialog.test.tsx` first, and check that it fails because the module is missing. Render a small harness component that calls `useConfirmAction()`. The harness has an opener button whose click calls `ask({ title: "Delete beer", message: 'Delete <b>Stout</b> permanently?', confirmLabel: "Delete" }, action)` with `action = vi.fn()`, and it renders `{dialog}`. Write these tests:
  - (a) After the opener is clicked, `getByRole("dialog", { name: "Delete beer" })` exists. It shows the message as literal text, including `<b>`, with no `b` element. Its confirm button "Delete" has class `btn btn-danger`. Its "Cancel" button has class `btn` and not `btn-danger`.
  - (b) `document.activeElement` is the Cancel button.
  - (c) Clicking "Delete" removes the dialog and calls `action` exactly once. A second click fired straight after the first (two `fireEvent.click` calls in a row) still leaves `action` called exactly once (spec edge case, research R4).
  - (d) Clicking "Cancel", clicking "Close", and firing `cancel` on the dialog element (Escape; see how `Modal.test.tsx` does this) each remove the dialog and leave `action` uncalled. Write one test per path.
  - (e) After the dialog closes by any path, focus is back on the opener button.
  - (f) Scan for `window.confirm`: collect `import.meta.glob("../**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true })`, drop keys ending in `.test.ts` or `.test.tsx`, and assert that no file content matches `/window\.confirm\s*\(/`. The scan reads raw text, comments included, so no source file may write `window.confirm(` anywhere, even in a comment; comments say "the browser's confirmation box" instead. Test files are excluded only because they may name the global (FR-007). This test fails until T013 is done, which is expected.
  - jsdom note: if `HTMLDialogElement.prototype.showModal` or `close` is missing, reuse whatever `web/src/test/setup.ts` or `Modal.test.tsx` already does. Do not add a new polyfill.
- [X] T002 [P] Add shared test helpers in `web/src/testApi.ts`: `inDialog(title: string)` returns `within(screen.getByRole("dialog", { name: title }))`, and `clickInDialog(title, button)` clicks a named button in it, each with a one-line doc comment. Import `fireEvent`, `screen` and `within` from `@testing-library/react` (already a dev dependency; `testApi.ts` is test-only code). T004 to T011 use them to click "Cancel" or the confirm label; the older per-file `within(screen.getByRole("dialog", { name }))` copies use them too.
- [X] T003 Create `web/src/components/ConfirmDialog.tsx` per [contracts/confirm-dialog.md](./contracts/confirm-dialog.md) and make T001 (a) to (e) pass:
  - Default export `ConfirmDialog({ title, message, confirmLabel, onConfirm, onCancel })` renders `<Modal title={title} onClose={onCancel}>`. Inside it are `<p>{message}</p>` and `<div className="btn-row">` with `<button type="button" className="btn btn-danger" onClick={onConfirm}>{confirmLabel}</button>` followed by `<button type="button" className="btn" ref={cancelRef} onClick={onCancel}>Cancel</button>`.
  - Add `useEffect(() => cancelRef.current?.focus(), [])`, with a one-line why-comment: `showModal` focuses the first button (the confirm), and this runs after `Modal`'s layout effect (research R3). Do not use `autoFocus`.
  - Named export `useConfirmAction()` holds `useState<{ title; message; confirmLabel; action } | null>`. `ask(request, action)` sets it. `dialog` is `pending && <ConfirmDialog ... onConfirm={() => { setPending(null); pending.action(); }} onCancel={() => setPending(null)} />`. It returns `{ ask, dialog }`.
  - Add doc comments on both exports stating the contract: initial focus on Cancel; Cancel, Close and Escape drop the action; confirm closes the dialog, then runs the action once; the caller renders `dialog` inside `.admin`. Mirror the style of `useConfirm` in `ConfirmIdentity.tsx`, and mention that name difference.
  - Use no `style` attributes and no new CSS unless the visual check in T015 shows the `<p>` needs spacing. In that case, add one rule under `.admin dialog.modal` in `web/src/pages/AdminPage.css`.

**Checkpoint**: T001 (a) to (e) pass. Test (f) still fails because 10 `window.confirm` calls remain.

---

## Phase 3: User Story 1 - Confirm a destructive action in an app-styled dialog (Priority: P1) 🎯 MVP

**Goal**: All 10 confirmations in the call-site table of [contracts/confirm-dialog.md](./contracts/confirm-dialog.md) open the app dialog, and none uses `window.confirm`.

**Independent Test**: Each panel test opens its confirmation as a dialog. Cancel sends no request, and the confirm button sends it. After that, the manual check in [quickstart.md](./quickstart.md).

**Call-site pattern** (every task below): add `import { useConfirmAction } from "./ConfirmDialog";` and `const confirm = useConfirmAction();` in the component body. Do not name the variable `identity`, which is taken in some panels. Replace `if (!window.confirm(msg)) return; run(...)` with `confirm.ask({ title, message: msg, confirmLabel }, () => run(...))`. For `if (window.confirm(msg)) change(...)`, use `confirm.ask({...}, () => change(...))`. Render `{confirm.dialog}` next to the component's other dialogs, or as the last child of its root element. Keep each message string exactly as it is today. Take the titles and labels from the contract table.

**Test pattern** (every task below): remove `vi.stubGlobal("confirm", ...)` and any `confirm` mock assertions. Click the opening button and assert that `inDialog(<title>)` shows the old message text. Click "Cancel" and assert that no request was sent, using the file's existing `stubApi` calls array. Click the opening button again, click the confirm label, and assert the request (and reload) the old test asserted. Where a file has several confirm stubs (AdminsPanel 6, TwoFactorPanel 5, TapsPanel 3), convert each one. Do not delete tests. Keep the existing test names, adjusted if they mention "confirm".

### Tests and implementation for User Story 1

Each pair below shares one component, so do the test edit and the source edit together. Different pairs touch different files and can run in parallel.

- [X] T004 [P] [US1] Tokens: `web/src/components/TokensPanel.tsx`, `revoke` (~line 83): title "Revoke token", label "Revoke". Update `web/src/components/TokensPanel.test.tsx` ("revokes after confirmation"). Also check that the existing "confirm identity" tests still pass unchanged.
- [X] T005 [P] [US1] Beers: `web/src/components/BeersPanel.tsx`, `remove` (~line 79): title "Delete beer", label "Delete". Update `web/src/components/BeersPanel.test.tsx`.
- [X] T006 [P] [US1] Keg History: `web/src/components/KegHistory.tsx`, `remove` (~line 39): title "Delete keg", label "Delete". Update `web/src/components/KegHistory.test.tsx`.
- [X] T007 [P] [US1] Taps: `web/src/components/TapsPanel.tsx`, `remove` (~line 97): title "Remove tap", label "Remove". Update the Remove-tap tests in `web/src/components/TapsPanel.test.tsx` (~lines 207, 231). The kick test there (~line 425) goes through KegCard; update it in T008.
- [X] T008 [US1] Kick (after T007, which also edits `TapsPanel.test.tsx`): `web/src/components/KegCard.tsx`, `kick` (~line 65): title "Kick keg", label "Kick". `{confirm.dialog}` renders inside KegCard's root. Update `web/src/components/KegCard.test.tsx`, the kick test in `web/src/components/TapsPanel.test.tsx` (~line 425, message "Kick the Stout keg? It moves to keg history."), and `web/src/components/OnDeckPanel.test.tsx`.
- [X] T009 [P] [US1] Admins: `web/src/components/AdminsPanel.tsx`:
  - In `remove` (~line 91), for another admin use title "Remove admin", label "Remove", and the message `Remove admin "${a.username}"?`. For yourself use title "Remove yourself", label "Remove", and the message "Remove yourself? You will be signed out.".
  - In `clearTwoFactor` (~line 100), use title "Clear two-factor sign-in" and label "Clear".
  - Render `{confirm.dialog}` beside `{identity.dialog}` (~line 140).
  - Update all 6 confirm stubs in `web/src/components/AdminsPanel.test.tsx`. Convert the existing `confirm_required` tests for Remove (`DELETE /api/admins/2`, ~line 265) and Clear (`DELETE /api/admins/2/2fa`, ~line 333) so each confirms in the new dialog first, then shows "Confirm it is you" (US1 scenario 6). Do not add duplicates.
- [X] T010 [P] [US1] Two-factor: `web/src/components/TwoFactorPanel.tsx`:
  - In `remove` (~line 77), use title "Remove second factor" and label "Remove".
  - In `turnOff` (~line 85), use title "Turn off 2FA" and label "Turn off".
  - Update the doc comment line that says "each after the browser asks to be sure" (~line 26) to say an app confirmation dialog asks instead.
  - Update all 5 confirm stubs in `web/src/components/TwoFactorPanel.test.tsx`. Make sure a test where a confirmed passkey Remove gets `confirm_required` and then shows "Confirm it is you" exists or is added.
- [X] T011 [P] [US1] Brewfather: `web/src/components/BrewfatherPanel.tsx`, `disconnect` (~line 149): title "Disconnect Brewfather", label "Disconnect". Update `web/src/components/BrewfatherPanel.test.tsx`.
- [X] T012 [US1] Run `grep -rn "window.confirm" web/src`. Expected: no matches outside test files, comments included (T001 (f) scans raw text). Fix any remaining match, and remove any leftover stale `stubGlobal("confirm"` in tests.
- [X] T013 [US1] Run `npx vitest run --maxWorkers=2 src/components/ConfirmDialog.test.tsx` from `web/`. Test (f) now passes.

**Checkpoint**: US1 is complete. All 10 confirmations use the app dialog.

---

## Phase 4: Polish & Cross-Cutting Concerns

- [X] T014 From `web/`, run `npm run lint` and `npx vitest run --maxWorkers=2`. Both must pass.
- [ ] T015 Do the manual check in [quickstart.md](./quickstart.md) in light, dark and system themes, at desktop and phone width. Compare the dialog side by side with "Add admin". Add the CSS rule from T003 only if the message spacing differs from the other dialogs.
- [X] T016 [P] Check doc comments (Constitution VI): every component whose confirm flow changed still describes it correctly. For example, KegCard's header (~line 17) says "after a confirm prompt"; keep or reword it to say "after a confirmation dialog".

---

## Dependencies & Execution Order

- Phase 2: T001 first (the failing tests), then T003. T002 can run in parallel with T001 and T003.
- Phase 3: T004 to T011 depend on T002 and T003. They are independent of each other, except that T008 runs after T007 because both edit `TapsPanel.test.tsx`. T012 and T013 come after all of them.
- Phase 4: after Phase 3.

### Parallel example (US1)

```text
After T003: run T004, T005, T006, T009, T010, T011 together; run T007 then T008 (shared TapsPanel.test.tsx).
```

## Implementation Strategy

- **MVP**: Phase 2 plus T004 (Tokens, the case in the user's screenshot) proves the dialog end to end.
- **Incremental**: convert the remaining call sites one per task. Each leaves the suite green except T001 (f), which turns green at T013.
- **Commit**: one commit for the feature, or one per phase.
