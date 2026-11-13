# Tasks: Danger Button Styling

**Input**: Design documents from `specs/012-danger-buttons/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[contracts/danger-button.md](./contracts/danger-button.md), [quickstart.md](./quickstart.md)

**Tests**: Included. Constitution IV requires component tests, and research R4 defines them:
each listed button has class `btn-danger`; one neighbouring non-danger button does not.

**Organization**: The spec has one user story (US1, P1). Every listed button gets the same
treatment, so all button work sits in that one phase.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1)

## Phase 1: Setup

None. No new dependencies, tooling or config.

---

## Phase 2: Foundational (blocks US1)

- [X] T001 Add the danger button style in `web/src/pages/AdminPage.css`:
  - In the `.admin { ... }` token block (after `--danger: #c0352b;`), add `--danger-fill: #c0352b;`. Do NOT add it to `.admin.theme-dark` or the `@media (prefers-color-scheme: dark) .admin.theme-system` block: one value in both themes, like `--accent` (research R1).
  - Right after the `.admin .btn-primary:hover` rule, add `.admin .btn-danger` (`background: var(--danger-fill); border-color: var(--danger-fill); color: #ffffff;`) and `.admin .btn-danger:hover` (`border-color: var(--danger-fill); filter: brightness(1.08);`), mirroring `.btn-primary` exactly.
  - Precede the new rules with a one-line why-comment, e.g. `/* Danger: one red in both themes; white on #c0352b is 5.55:1. Dark --danger (#e5534b) is for red text and gives white only 3.70:1. */`. Add `white on --danger-fill 5.55:1` to the contrast notes in the file header comment.
  - Do not touch the disabled or `:focus-visible` rules; they already apply via `.btn` (research R2, R3).

**Checkpoint**: `btn-danger` exists; nothing uses it yet, so the app looks unchanged.

---

## Phase 3: User Story 1 - Spot destructive actions at a glance (Priority: P1) 🎯 MVP

**Goal**: The 9 buttons in [contracts/danger-button.md](./contracts/danger-button.md) are red-filled; nothing else changes.

**Independent Test**: Component tests assert the class on each listed button and its absence on a neighbour; visual check per [quickstart.md](./quickstart.md) in light, dark and system-dark themes.

### Tests for User Story 1

Write each test first and confirm it fails before its matching implementation task. Use the existing render/stub setup in each file; select buttons by role and name (e.g. `screen.getAllByRole("button", { name: "Delete" })`) and assert with `toHaveClass("btn-danger")` / `not.toHaveClass("btn-danger")`.

- [X] T002 [P] [US1] In `web/src/components/BeersPanel.test.tsx`, add a test: every row's Delete button has `btn-danger`; Edit, Archive and (for a Brewfather beer, reusing the "Brewfather beers" setup) Re-sync do not.
- [X] T003 [P] [US1] In `web/src/components/KegHistory.test.tsx`, add a test: every keg row's Delete button has `btn-danger`.
- [X] T004 [P] [US1] In `web/src/components/AdminsPanel.test.tsx`, add a test: every row's Remove button has `btn-danger`.
- [X] T005 [P] [US1] In `web/src/components/TokensPanel.test.tsx`, add a test: every Revoke button has `btn-danger`.
- [X] T006 [P] [US1] In `web/src/components/TwoFactorPanel.test.tsx`, add a test with 2FA on, an authenticator app and at least one passkey: Turn off 2FA, the authenticator Remove, each passkey Remove and New recovery codes have `btn-danger`; Rename does not.
- [X] T007 [P] [US1] In `web/src/components/BrewfatherPanel.test.tsx`, add a test: when connected, Disconnect has `btn-danger`.

### Implementation for User Story 1

Change `className="btn"` to `className="btn btn-danger"` on exactly these buttons. Do not change labels, handlers, confirm prompts or any other button (FR-005, FR-006).

- [X] T008 [P] [US1] `web/src/components/BeersPanel.tsx`: the Delete button in each beer row (`onClick={() => remove(b)}`, ~line 169).
- [X] T009 [P] [US1] `web/src/components/KegHistory.tsx`: the Delete button in each keg row (`onClick={() => remove(k)}`, ~line 73).
- [X] T010 [P] [US1] `web/src/components/AdminsPanel.tsx`: the Remove button in each row (`onClick={() => remove(a)}`, ~line 132).
- [X] T011 [P] [US1] `web/src/components/TokensPanel.tsx`: the Revoke button (`onClick={() => revoke(t)}`, ~line 137).
- [X] T012 [P] [US1] `web/src/components/TwoFactorPanel.tsx`: Turn off 2FA (`onClick={turnOff}`, ~line 153); authenticator Remove (`remove("the authenticator app", ...)`, ~line 166); passkey Remove (multi-line button with `remove(\`the passkey ...\`)`, button opens ~line 199); New recovery codes (`onClick={newCodes}`, ~line 229).
- [X] T013 [P] [US1] `web/src/components/BrewfatherPanel.tsx`: the Disconnect button (`onClick={disconnect}`, ~line 225).

**Checkpoint**: T002–T007 pass; US1 is complete.

---

## Phase 4: Polish & Cross-Cutting Concerns

- [X] T014 Confirm `btn-danger` appears on exactly the 9 buttons: `grep -rn "btn-danger" web/src --include=*.tsx` should list only the lines changed in T008–T013 (plus tests).
- [X] T015 Run `cd web && npx vitest run --maxWorkers=2 && npx tsc --noEmit` (`tsc --noEmit` is also the project's `lint` script); all pass.
- [ ] T016 Do the visual check in [quickstart.md](./quickstart.md) in Light, Dark and System (device dark) themes, at desktop and at phone width (narrow-screen edge case: red buttons wrap and size as before). In Dark, confirm in dev tools that a danger button's computed background is `rgb(192, 53, 43)` (`#c0352b`) and text `rgb(255, 255, 255)` (SC-002). Record the result in this file.

---

## Dependencies & Execution Order

- T001 first (the class must exist for the look; tests only check the class name, so T002–T007 can be written alongside it).
- T002–T007: all [P], different files.
- T008–T013: all [P], different files; each depends on its matching test (T002→T008, T003→T009, T004→T010, T005→T011, T006→T012, T007→T013).
- T014–T016 after all of the above.

## Parallel Example: User Story 1

```text
# Tests, all at once:
T002 BeersPanel.test.tsx   T003 KegHistory.test.tsx   T004 AdminsPanel.test.tsx
T005 TokensPanel.test.tsx  T006 TwoFactorPanel.test.tsx T007 BrewfatherPanel.test.tsx

# Then implementation, all at once:
T008 BeersPanel.tsx  T009 KegHistory.tsx  T010 AdminsPanel.tsx
T011 TokensPanel.tsx T012 TwoFactorPanel.tsx T013 BrewfatherPanel.tsx
```

## Implementation Strategy

US1 is the whole feature and the MVP: T001, then T002–T013, then T014–T016. One small PR.
