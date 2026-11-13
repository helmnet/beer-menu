# Implementation Plan: Danger Button Styling

**Branch**: `compmodder26/danger_buttons_styling` | **Date**: 2026-11-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/012-danger-buttons/spec.md`

## Summary

Nine destructive buttons in the admin app become red, filled like the blue primary button. A
new `btn-danger` modifier class copies `btn-primary` with a new `--danger-fill` token
(`#c0352b`, same in both themes, white text at 5.55:1). The existing dark `--danger` red fails
white-text contrast, so it is not reused. Each affected panel's component test asserts the right
buttons carry the class. Details in [research.md](./research.md).

## Technical Context

**Language/Version**: TypeScript 7 with React 19 on Node 26 (web build); CSS

**Primary Dependencies**: existing React, Vite, Vitest + Testing Library. No new dependencies.

**Storage**: N/A

**Testing**: Vitest + Testing Library component tests (`--maxWorkers=2`)

**Target Platform**: Admin app in modern browsers, light and dark themes

**Project Type**: Web application (Go API + React web); this feature touches `web/` only

**Performance Goals**: N/A

**Constraints**: Text contrast at least 4.5:1 in both themes (FR-002); no `style` attributes
(CSP has no `style-src`); all admin rules stay under `.admin`

**Scale/Scope**: 1 stylesheet, 6 components, 6 test files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Note |
|-----------|--------|------|
| I. API-First | Pass | No capability added; UI look only. No endpoint or OpenAPI change. |
| II. Go + SQLite | Pass | Backend untouched. |
| III. React Clients | Pass | Admin app only; `/display` unaffected (rules scoped to `.admin`). |
| IV. Tests Required | Pass | Component tests assert the danger class on each listed button and its absence on a neighbour. |
| V. LAN Security | Pass | No auth, secret or input change. Confirm prompts unchanged. |
| VI. Documented | Pass | CSS comment states contrast and why `--danger` is not reused. |
| VII. Simplicity | Pass | One token, one class pair, class added to 9 buttons. No new component or dependency. |

Post-design re-check: unchanged, all pass.

## Project Structure

### Documentation (this feature)

```text
specs/012-danger-buttons/
├── plan.md
├── research.md
├── quickstart.md
├── contracts/
│   └── danger-button.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks
```

No `data-model.md`: the feature has no data.

### Source Code (repository root)

```text
web/src/
├── pages/
│   └── AdminPage.css            # --danger-fill token, .btn-danger and :hover rules
└── components/
    ├── BeersPanel.tsx           # Delete (beer rows)
    ├── KegHistory.tsx           # Delete (keg rows)
    ├── AdminsPanel.tsx          # Remove
    ├── TokensPanel.tsx          # Revoke
    ├── TwoFactorPanel.tsx       # Turn off 2FA, Remove (app, passkey), New recovery codes
    ├── BrewfatherPanel.tsx      # Disconnect
    └── *.test.tsx               # matching tests for the six components
```

**Structure Decision**: Existing web app layout. Change is confined to the admin stylesheet and
the six components that own the listed buttons.

## Complexity Tracking

No violations.
