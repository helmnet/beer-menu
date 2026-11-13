# Implementation Plan: Compact Theme Control in a Narrow Side Panel

**Branch**: `compmodder26/theme_mode_display_crowding` | **Date**: 2026-11-13 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/015-theme-control-compact/spec.md`

## Summary

The admin side panel's theme control (Light, Dark, System) crowds and overflows when the panel is
narrow. The fix is CSS only plus one attribute: the side panel's bottom block becomes a size
container, and a container query hides the three words (visually, so they stay the radios'
accessible names) when the block is narrower than the width the labelled control needs. The
threshold is in `rem`, so a larger default font size switches to icons sooner. Each option gets a `title` with its
name for the hover tooltip. No API, storage or dependency change.

## Technical Context

**Language/Version**: TypeScript 7.0 / React 19.3, CSS (existing web app)

**Primary Dependencies**: existing only: React, `lucide-react` icons. No new dependency.

**Storage**: N/A. The theme preference and its storage are untouched.

**Testing**: Vitest + Testing Library (`web/src/components/ThemeControl.test.tsx`); run with
`--maxWorkers=2`. Visual switch point checked by hand (quickstart), since jsdom does not lay out
or evaluate container queries.

**Target Platform**: evergreen browsers (Chrome, Firefox, Safari on desktop and phone). CSS size
container queries are supported in all of them and already used in `web/src/pages/DisplayPage.css`.

**Project Type**: web application (Go API + React web); this feature touches only `web/`.

**Performance Goals**: no layout work in JavaScript; the browser re-evaluates the query on resize.

**Constraints**: side panel width 200–400 px (`web/src/prefs.ts`); phone drawer 300 px / 85vw;
tap targets stay 32 px (44 px on touch, existing media query).

**Scale/Scope**: 1 component (`ThemeControl.tsx`), 1 stylesheet (`AdminPage.css`), 1 test file.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check | Result |
|-----------|-------|--------|
| I. API-First | Look-only change; no capability added, no endpoint touched. | Pass |
| II. Go + SQLite | No backend change. | Pass (N/A) |
| III. React Clients | Change stays in the React admin app; TV display untouched (FR-010). | Pass |
| IV. Tests Required | `ThemeControl` has logic-free markup but gets component tests for the new contract: each option has `title`, accessible names unchanged, words in a hideable element. Switch point verified by hand per quickstart (jsdom cannot lay out). | Pass |
| V. LAN Security | No auth, data or input change. | Pass (N/A) |
| VI. Documented | `ThemeControl` doc comment and the CSS block comment updated to describe the compact form and why the words are visually hidden, not removed. | Pass |
| VII. Simplicity | Pure CSS container query, no ResizeObserver, no JS measuring, no new dependency, no option or setting. | Pass |

Post-design re-check (after Phase 1): unchanged, all pass. No Complexity Tracking entries.

## Project Structure

### Documentation (this feature)

```text
specs/015-theme-control-compact/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (no data changes)
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── theme-control.md # Phase 1 output: UI contract
└── tasks.md             # Phase 2 output (/speckit-tasks, not created here)
```

### Source Code (repository root)

```text
web/src/
├── components/
│   ├── ThemeControl.tsx        # add title per option; class on the word span
│   └── ThemeControl.test.tsx   # tests for title and unchanged names
└── pages/
    └── AdminPage.css           # .nav-bottom container; @container rule hiding words
```

**Structure Decision**: web-only change inside the existing `web/src` layout. `SideNav.tsx` needs
no change: it already renders the theme control in its bottom block (`.nav-bottom`).

## Complexity Tracking

No constitution violations.
