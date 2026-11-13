# Implementation Plan: Styled Confirmation Dialogs

**Branch**: `compmodder26/confirmation_dialog_styling` | **Date**: 2026-11-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/014-styled-confirm-dialogs/spec.md`

## Summary

The admin app replaces its 10 `window.confirm` calls with an app dialog built on the existing `Modal` component. The new `ConfirmDialog.tsx` holds the dialog (title, message, red `btn-danger` confirm button, plain Cancel button). It also holds a `useConfirmAction()` hook that returns `{ ask, dialog }`, the same shape as the existing identity `useConfirm`. Each call site becomes `confirm.ask({ title, message, confirmLabel }, () => run(...))` and renders `{confirm.dialog}`. Cancel gets initial focus. A source-scan test stops `window.confirm` from coming back. Details are in [research.md](./research.md) and [contracts/confirm-dialog.md](./contracts/confirm-dialog.md).

## Technical Context

**Language/Version**: TypeScript with React 19 (Vite build); CSS

**Primary Dependencies**: existing React, Vite, Vitest and Testing Library, and `lucide-react` through `Modal`. No new dependencies.

**Storage**: N/A

**Testing**: Vitest and Testing Library component tests (`--maxWorkers=2`)

**Target Platform**: admin app in modern browsers, light and dark themes, desktop and phone widths

**Project Type**: web application (Go API and React web). This feature touches `web/` only.

**Performance Goals**: N/A

**Constraints**: no `style` attributes (the CSP has no `style-src`); admin rules stay under `.admin`; reuse `Modal` and `btn-danger` and do not copy them

**Scale/Scope**: 1 new component and its test; 8 components edited (10 call sites); 9 existing test files updated. CSS changes are none expected, or at most a rule for the message paragraph.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Note |
|-----------|--------|------|
| I. API-First | Pass | UI only. No endpoint or OpenAPI change. |
| II. Go + SQLite | Pass | The backend is untouched. |
| III. React Clients | Pass | Admin app only. `/display` is unaffected. |
| IV. Tests Required | Pass | New `ConfirmDialog` tests cover focus, the 3 cancel paths, confirming once and the `window.confirm` scan. Each panel test proves cancel sends nothing and confirm sends the request. |
| V. LAN Security | Pass | No auth or input change. Messages render as escaped text. The 2FA "Confirm it is you" flow is unchanged and still follows the confirmation. |
| VI. Documented | Pass | The component and hook get doc comments stating the contract (focus, cancel paths, the action runs once). |
| VII. Simplicity | Pass | Reuses `Modal` and `btn-danger`. One small hook mirrors an existing one. No provider, library or ESLint. |

Post-design re-check: unchanged; all principles pass.

## Project Structure

### Documentation (this feature)

```text
specs/014-styled-confirm-dialogs/
├── plan.md
├── research.md
├── quickstart.md
├── contracts/
│   └── confirm-dialog.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks
```

There is no `data-model.md` because the feature has no data.

### Source Code (repository root)

```text
web/src/
├── pages/AdminPage.css            # only if the message paragraph needs spacing
└── components/
    ├── ConfirmDialog.tsx          # new: ConfirmDialog + useConfirmAction
    ├── ConfirmDialog.test.tsx     # new: behavior + no-window.confirm scan
    ├── BeersPanel.tsx             # Delete beer
    ├── KegHistory.tsx             # Delete keg
    ├── TapsPanel.tsx              # Remove tap
    ├── KegCard.tsx                # Kick keg (also used by OnDeckPanel)
    ├── AdminsPanel.tsx            # Remove admin / yourself, Clear 2FA
    ├── TokensPanel.tsx            # Revoke token
    ├── TwoFactorPanel.tsx         # Remove factor, Turn off 2FA
    ├── BrewfatherPanel.tsx        # Disconnect
    └── *.test.tsx                 # the 9 files that stub window.confirm today
```

**Structure Decision**: This keeps the existing web app layout. The new dialog sits beside `Modal` and `ConfirmIdentity` in `components/`.

## Complexity Tracking

No violations.
