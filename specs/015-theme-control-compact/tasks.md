---

description: "Task list for the compact theme control"
---

# Tasks: Compact Theme Control in a Narrow Side Panel

**Input**: Design documents from `specs/015-theme-control-compact/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/theme-control.md, quickstart.md

**Tests**: Included. Constitution IV requires component tests and, for a bug fix, a test that fails without the fix. T002 and T005 fail before their fixes, but they prove the markup contract (word wrapper, tooltip, names), not the crowding itself: jsdom cannot lay out or run container queries. The behavior proof is the owner's manual check (T008), and the PR body must say so.

**Organization**: Tasks are grouped by user story so each story can be built and tested on its own.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story the task belongs to (US1, US2)

## Path Conventions

Web app: `web/src/components/`, `web/src/pages/`. Run vitest as `npx vitest run --maxWorkers=2` (owner's laptop).

---

## Phase 1: Setup

No setup. No new dependency, file or configuration (plan.md, Constitution VII).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Measure the switch point both stories rely on.

- [X] T001 Measure the labelled control's needed width in the running admin app (Vite dev server, `cd web && npm run dev`, or `docker compose up`), per research.md R2: at default text size, find the smallest `.admin .nav-bottom` width at which the Light, Dark and System segments in `web/src/components/ThemeControl.tsx` show icon + word on one line with at least 4 px between each word and its segment's edge and nothing touching the control's border. Check whether that fits at the default side panel width of 256 px (SC-002). If it does not, plan a reduced segment `padding`/`gap` in `web/src/pages/AdminPage.css` (`.admin .theme-control .theme-option`, now `gap: 4px; padding: 0 4px`) instead of a lower threshold. Record the measured px and its `em` value (px ÷ 16, rounded up to a 0.5 em step) in this task's notes for T003.

  - **T001 notes (measured in a browser, not computed)**: headless Firefox (`firefox --headless --screenshot`) on a static page loading the real `AdminPage.css` with the real shell markup (`.shell` > `.side-nav` > `.side-inner` > `.nav-bottom`); a script swept `--nav-width` 400 to 200 px and checked each word's box against its segment. Admin font is the `system-ui` stack, which resolved to Noto Sans here. Word widths at 0.85rem (13.6 px): Light 33.1, Dark 31.1, System 47.1 px. With today's 4 px icon-word gap the words need a 236 px `.nav-bottom`, but the default 256 px panel gives only 231 px (256 − 1 px border − 2 × 12 px padding, border-box), so SC-002 failed. Fix: icon-word `gap` 4 px → 2 px (segment `padding` stays 4 px, the required clearance); the words then need 230 px. Threshold: `@container (width < 14.4rem)` = 230.4 px. Unit is `rem`, not `em`: in a container condition `em` resolves to `.nav-bottom`'s font size (fixed 15 px from `.admin`), so it would not follow the user's default text size, while the words are `0.85rem`. Not rounded to a 0.5 step: 14.5rem (232 px) would hide the words at the default 231 px block. Re-checked after the change: words at 256 px, icons only from 255 px down, no crowded or mixed state (Noto Sans, Liberation Sans, Cantarell, Open Sans); root font 20 px goes icon-only below a 312 px panel. DejaVu Sans (wider) still crowds at panels 256–265 px.

**Checkpoint**: Threshold known in `rem` (14.4rem, 230.4 px).

---

## Phase 3: User Story 1 - Readable theme choice at any side panel width (Priority: P1) 🎯 MVP

**Goal**: The control shows icon + word when the words fit and icons only when they do not, switching all three together as the panel width or text size changes.

**Independent Test**: Drag the side panel from 400 px to 200 px and back: the control is always either all icon + word with no overlap, or all icons only, never crowded or past its border (quickstart steps 1–2, 4–6).

### Tests for User Story 1

- [X] T002 [P] [US1] In `web/src/components/ThemeControl.test.tsx`, add a test that each option's word is rendered in a `span.theme-label` inside its `label.theme-option` (Light, Dark, System, in order), and that the radios are still named Light, Dark and System (the existing first test covers names; extend it or add one, do not copy it). Must fail before T004.

### Implementation for User Story 1

- [X] T003 [US1] In `web/src/pages/AdminPage.css`: (a) add `container-type: inline-size;` to `.admin .nav-bottom`; (b) add `white-space: nowrap;` for `.admin .theme-control .theme-label`; (c) apply any padding/gap change decided in T001 to `.admin .theme-control .theme-option`; (d) add one `@container (max-width: <T001 value>em) { .admin .theme-control .theme-label { … } }` rule that visually hides the word with the same declarations as `.admin .sr-only` (line ~116: absolute position, 1 px box, -1 px margin, no padding, overflow hidden, `clip: rect(0 0 0 0)`, nowrap, no border). Never use `display: none` or `visibility: hidden` (they drop the radio's accessible name, FR-007). A container-scoped rule cannot join the `.sr-only` selector list, so this repeat is unavoidable; say so in a one-line comment pointing at `.sr-only`. Update the theme control block comment to say the words hide below the measured width (state the px), why the container is `.nav-bottom` (a container cannot query itself; R1) and why `em` (text zoom). Keep the grid, `min-height` (32 px, 44 px in the touch media query) and selected/focus rules unchanged (FR-005, FR-008, FR-009).
- [X] T004 [US1] In `web/src/components/ThemeControl.tsx`, change `<span>{o.label}</span>` to `<span className="theme-label">{o.label}</span>` and update the component doc comment: words show when the side panel's bottom block is wide enough and are visually hidden (still the radios' names) when it is not; the switch is CSS only. Run `npx vitest run --maxWorkers=2 src/components/ThemeControl.test.tsx` in `web/`: T002 passes.

**Checkpoint**: US1 works; quickstart steps 1, 2, 4, 5, 6 pass.

---

## Phase 4: User Story 2 - Icons stay identifiable without their words (Priority: P2)

**Goal**: Hover shows each option's name; assistive technology hears the same names at every width.

**Independent Test**: At 200 px, hover each icon and see Light / Dark / System; DevTools accessibility tree shows the "Theme" group with three named radios (quickstart steps 3, 7).

### Tests for User Story 2

- [X] T005 [P] [US2] In `web/src/components/ThemeControl.test.tsx`, add a test that each `label.theme-option` has a `title` equal to its option name (Light, Dark, System). Must fail before T006.

### Implementation for User Story 2

- [X] T006 [US2] In `web/src/components/ThemeControl.tsx`, add `title={o.label}` to each option's `<label>` (research.md R4: native tooltip, at all widths) and mention the tooltip in the doc comment. Run the ThemeControl tests: T005 passes.

**Checkpoint**: US1 and US2 both work.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T007 Run the full web gate in `web/`: `npx vitest run --maxWorkers=2`, then `npm run build` (it runs `tsc --noEmit` before `vite build`). All pass.
- [ ] T008 Owner runs quickstart.md manual steps 1–7 in a real browser (light, dark, system theme; 200–400 px sweep; larger default font size; phone drawer incl. 320 px window; accessibility tree) and records the result in this file. This is the behavior proof for Constitution IV; the PR body states that the switch point is verified by hand because jsdom cannot lay out.

---

## Dependencies & Execution Order

- **T001** blocks T003 (threshold value and padding decision).
- **US1**: (T002 ∥ T003) → T004. T002 is written first and fails until T004; T003 (CSS) is not covered by jsdom tests.
- **US2**: T005 → T006. Independent of US1 in behavior, but T005/T006 edit the same files as T002/T004, so run them after US1 to avoid edit conflicts.
- **Polish**: T007 after all code tasks; T008 after T007.

### Parallel Opportunities

- T002 and T003 are different files and can run in parallel once T001 is done.
- T005 can be written alongside T003 (different file), but not alongside T002 (same file).

```text
After T001:  T002 (ThemeControl.test.tsx)  ∥  T003 (AdminPage.css)
Then:        T004 (ThemeControl.tsx)
Then:        T005 (ThemeControl.test.tsx) → T006 (ThemeControl.tsx)
```

## Implementation Strategy

### MVP First (User Story 1 only)

1. T001 measure.
2. T002–T004: the crowding is gone; icons alone at narrow widths, names kept.
3. Stop and validate with quickstart steps 1, 2, 4–6.

### Incremental Delivery

1. US1 fixes the reported crowding (MVP).
2. US2 adds the hover tooltip.
3. Polish: full gate, owner's manual check.

## Notes

- Total 8 tasks, all in `web/`. No API, storage or OpenAPI change.
- One small CSS repeat (the visually-hidden declarations) is forced by `@container`; no other copies.
