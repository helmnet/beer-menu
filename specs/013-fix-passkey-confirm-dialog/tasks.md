# Tasks: Fix Passkey Prompt Inside Admin Dialogs

**Input**: Design documents from `specs/013-fix-passkey-confirm-dialog/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[contracts/passkey-prompt.md](./contracts/passkey-prompt.md), [quickstart.md](./quickstart.md)

**Tests**: Included. Constitution IV requires a bug fix to ship a test that fails without the
fix. Research R4 defines the tests: `lowerModals` cases in `web/src/components/Modal.test.tsx`,
and lower/restore around each outcome in `web/src/webauthn.test.ts`. The real 1Password
interaction is checked manually (quickstart).

**Organization**: US1 (confirm identity, P1) and US2 (add passkey, P2) share one cause and one
change point, `prompt()` in `web/src/webauthn.ts`, which calls `lowerModals()` from
`web/src/components/Modal.tsx`. Those changes are foundational. Each story phase then holds its
own tests and manual check.

Run tests with `cd web && npx vitest run --maxWorkers=2`. Never use more workers: this runs on
the owner's laptop.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2)

## Phase 1: Setup (cause check, blocks everything)

- [X] T001 Owner ran the manual spike (`specs/013-fix-passkey-confirm-dialog/quickstart.md` step 1). Lowering every open dialog made the 1Password prompt clickable. It also showed that stacked dialogs need their order kept (research R1, R2).

**Checkpoint**: cause confirmed.

---

## Phase 2: Foundational (blocks US1 and US2)

- [X] T002 If `web/node_modules` is missing (fresh worktree), first run `cd web && npm ci`. Then in `web/src/test/setup.ts`, right after the existing `if (... !HTMLDialogElement.prototype.showModal) { ... }` block, add a separately guarded block: `if (typeof HTMLDialogElement !== "undefined" && !HTMLDialogElement.prototype.show) { HTMLDialogElement.prototype.show = function (this: HTMLDialogElement) { this.open = true; }; }`. jsdom has no `show()`, and without this stub T003 and T004 throw `TypeError` in the existing passkey component tests. Extend the existing jsdom comment above the `showModal` block to also name `show`. The new block needs no comment of its own.
- [X] T003 In `web/src/components/Modal.tsx`, follow [contracts/passkey-prompt.md](./contracts/passkey-prompt.md):
  - Add a module-level `const open: HTMLDialogElement[] = []`, kept in opening order.
  - In the existing `useLayoutEffect`, push `dialog` after `showModal()`. In its cleanup, remove `dialog` from `open` (splice by index, if present) before the existing `close()` and `opener?.focus()`. The existing behaviour must stay StrictMode-safe: each mount pushes once and each unmount removes once.
  - Add and export `lowerModals(): () => void`:
    - Snapshot `open`. If it is empty, return a no-op.
    - Save `document.activeElement` if it is an `HTMLElement` and not `document.body`.
    - For each dialog in the snapshot, bottom first, call `close()`, then `show()`, then add class `modal-lowered`. Give the last dialog `modal-lowered-top` as well.
    - Return `restore`. For each snapshot dialog, bottom first, it removes both classes. If the dialog is still `isConnected`, still in `open`, and still `open`, it calls `close()` then `showModal()`. It then calls `focus()` on the saved element if one was saved and it is still connected.
  - Write a doc comment for `lowerModals`. It must say that a modal dialog makes the page inert, which blocks password-manager prompts drawn in the page. It must also say why every dialog is lowered and why the opening order is kept (research R1, R2). Update the `Modal` doc comment with one sentence about the open stack. Do NOT restate the code.
  - Do NOT change `Modal` props or the markup it renders.
- [X] T004 In `web/src/webauthn.ts`, import `{ lowerModals }` from `./components/Modal`. In `prompt()`, call `const restore = lowerModals();` before `run()`. Wrap the existing `try`/`catch` in `try`/`finally`, with `restore()` in the `finally`. Keep the `NotAllowedError` / `InvalidStateError` mapping and the public signatures unchanged. In the doc comments of `getPasskey` and `createPasskey`, or in a comment on `prompt()`, add one line saying that open dialogs are lowered while the device prompt runs, so password-manager prompts can be used. Do NOT change `ConfirmIdentity.tsx`, `TwoFactorSetup.tsx`, `LoginForm.tsx` or any other `Modal` caller.
- [X] T005 [P] In `web/src/pages/AdminPage.css`, right after the `.admin dialog.modal::backdrop` rule, add:
  - `.admin dialog.modal.modal-lowered { inset: 0; z-index: 1000; }`
  - `.admin dialog.modal.modal-lowered-top { z-index: 1001; box-shadow: 0 0 0 100vmax var(--scrim, rgba(11, 12, 15, 0.55)); }`

  Put a one-line why-comment before them: this is the passkey-prompt state, `::backdrop` exists only for modal dialogs, and the top class keeps the newest dialog above the others (research R3). Do NOT use `style` attributes.

**Checkpoint**: both passkey flows lower every open dialog while the device prompt runs, and restore the stack afterwards.

---

## Phase 3: User Story 1 - Confirm identity with a passkey from a password manager (Priority: P1) 🎯 MVP

**Goal**: The 1Password prompt in "Confirm it is you" (shown over "New API token") accepts clicks. "Use passkey" completes the confirm. Closing the prompt leaves both dialogs open, in the same order, with the typed password still there.

**Independent Test**: The `lowerModals` and `getPasskey` tests pass, and they fail with T003 and T004 reverted. Then run quickstart manual rows 1-4 and 6-11.

### Tests for User Story 1

Write these first and confirm they fail without T003 and T004 (revert them locally if they are already done). They need T002.

- [X] T006 [US1] In `web/src/components/Modal.test.tsx`, add a `describe("lowerModals")` block.
  - **Setup:**
    - Render two `Modal`s, with the second mounted after the first. For example, render the first, then `rerender` with both, or use a wrapper with state.
    - Spy on `HTMLDialogElement.prototype.show` and `showModal`. Use the file's existing `afterEach` / `vi.restoreAllMocks`.
  - **Assert:**
    - (a) After `lowerModals()`, `show` was called once per dialog. Both dialogs have `modal-lowered`, and only the second has `modal-lowered-top`.
    - (b) After `restore()`, neither dialog has either class, and the `showModal` calls after lowering are on the first dialog and then the second (check `mock.contexts` order).
- [X] T007 [US1] In `web/src/components/Modal.test.tsx`, in the same block, add these tests:
  - (c) Unmount the second `Modal` between `lowerModals()` and `restore()`. `restore()` does not call `showModal` on it.
  - (d) Focus an `<input>` inside the top `Modal`, call `lowerModals()`, then move focus to a `<button>` outside both dialogs. After `restore()`, the `<input>` has focus.
  - (e) With no `Modal` mounted, `lowerModals()` and its `restore()` call neither `show` nor `showModal`.
- [X] T008 [US1] In `web/src/webauthn.test.ts`, add `vi.mock("./components/Modal", () => ({ lowerModals: vi.fn() }))`. In a new `describe("lowered dialogs during the prompt")`, have `lowerModals` return a spy `restore`. Using the existing `stubCredentials` and `request` fixtures, assert for `getPasskey`:
  - `lowerModals` was called before `navigator.credentials.get`, and `restore` was not yet called at that time. Record this inside the `get` stub.
  - `restore` is called exactly once after the call settles, in each of three cases: a credential result (returns `toJSON()`), `NotAllowedError` (returns `"cancelled"`), and any other error (the error passes on).

### Manual check for User Story 1

- [ ] T009 [US1] Owner runs `specs/013-fix-passkey-confirm-dialog/quickstart.md` manual rows 1, 2, 3, 4, 6, 7, 8, 9, 10 and 11 on a desktop browser with 1Password, and row 7 also on a phone. Record pass or fail for each row in the PR description.

**Checkpoint**: US1 delivered. SC-001, SC-003 and SC-004 are checked.

---

## Phase 4: User Story 2 - Add a new passkey from a password manager (Priority: P2)

**Goal**: The 1Password save prompt in "Add passkey" accepts clicks, and the new passkey is listed.

**Independent Test**: The `createPasskey` test passes. Then run quickstart manual row 5.

- [X] T010 [US2] In `web/src/webauthn.test.ts`, inside the T008 `describe` block, add a test: `createPasskey` calls `lowerModals` before `navigator.credentials.create` and `restore` once after it settles. Include a `create` rejecting with `InvalidStateError`, where the "already has a passkey" error must still throw.
- [ ] T011 [US2] Owner runs `specs/013-fix-passkey-confirm-dialog/quickstart.md` manual row 5 (Two-Factor Sign-In, Add passkey, save to 1Password). Record pass or fail in the PR description.

**Checkpoint**: US2 delivered. SC-002 is checked.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T012 Run the full web suite and confirm it passes (SC-005): `cd web && npx vitest run --maxWorkers=2`. This includes the unchanged `web/src/components/ConfirmIdentity.test.tsx`, `TwoFactorSetup.test.tsx`, `LoginForm.test.tsx` and `TokensPanel.test.tsx`.
- [X] T013 [P] Run `cd web && npm run lint` (the `tsc --noEmit` typecheck). Fix any errors in `web/src/components/Modal.tsx`, `web/src/webauthn.ts`, the two test files and `web/src/test/setup.ts`.

---

## Dependencies & Execution Order

- Order: T001 (done), then T002, then T003, then T004, then the story phases, then T012 and T013.
- T005 [P] can run alongside T002 to T004, since it is a different file.
- T006 and T007 both edit `Modal.test.tsx`, and T008 and T010 both edit `webauthn.test.ts`. Run each pair in order. The two pairs can run in parallel with each other. All four tasks need T002.
- US2 (T010 to T011) depends only on Phase 2. It does not depend on US1.
- The manual checks T009 and T011 need a build with T003 to T005. Both can be done in one session.

## Parallel Example

```text
After T001: T002 → T003 → T004 alongside T005 (AdminPage.css).
Tests: T006 → T007 (Modal.test.tsx) alongside T008 → T010 (webauthn.test.ts).
After Phase 2: T013 typecheck alongside manual checks T009 + T011.
```

## Implementation Strategy

- **MVP**: T002 to T009. US1 is the reported bug. The same shared change in T003 and T004 also fixes US2.
- **Then**: T010 to T011 prove the add-passkey flow, and T012 to T013 close out the work.
