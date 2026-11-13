# Implementation Plan: Fix Passkey Prompt Inside Admin Dialogs

**Branch**: `compmodder26/passkey_verificaton_issues` | **Date**: 2026-11-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/013-fix-passkey-confirm-dialog/spec.md`

## Summary

`Modal` opens its `<dialog>` with `showModal()`, which makes everything outside it inert,
including the prompt the 1Password extension draws in the page, so it cannot be clicked or
closed. Confirmed by the owner's spike, which also showed dialogs can be stacked ("Confirm it is
you" over "New API token") and must all be lowered. Fix: `Modal.tsx` tracks open dialogs in
opening order and exports `lowerModals()`, which makes them all non-modal (keeping their visual
stack with two CSS classes) and returns a `restore()` that re-opens them as modal in the original
order. `prompt()` in `web/src/webauthn.ts`, shared by `getPasskey` and `createPasskey`, wraps the
browser call with it. Details in [research.md](./research.md).

## Technical Context

**Language/Version**: TypeScript with React 19 (web build); CSS

**Primary Dependencies**: existing React, Vite, Vitest + Testing Library. No new dependencies.

**Storage**: N/A

**Testing**: Vitest unit tests for `webauthn.ts` (`--maxWorkers=2`); manual check with the
1Password extension

**Target Platform**: Admin app in current desktop Chromium and Firefox, and mobile browsers
(desktop Safari expected to work, not separately verified)

**Project Type**: Web application (Go API + React web); this feature touches `web/` only

**Performance Goals**: N/A

**Constraints**: No `style` attributes (CSP has no `style-src`); admin rules stay under `.admin`;
dialog behaviour unchanged outside a passkey check (FR-005)

**Scale/Scope**: 2 modules, 2 stylesheet rules, 2 test files, 1 test-setup stub

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Note |
|-----------|--------|------|
| I. API-First | Pass | No capability added; client fix only. No endpoint or OpenAPI change. |
| II. Go + SQLite | Pass | Backend untouched. |
| III. React Clients | Pass | Admin app only; `/display` has no dialogs or passkeys. |
| IV. Tests Required | Pass | New `Modal.test.tsx` and `webauthn.test.ts` cases fail without the fix; extension interaction checked manually (quickstart). |
| V. LAN Security | Pass | Password and passkey are both still required; server checks unchanged. The page behind is briefly not inert during the prompt (research R2 trade-off); every action there keeps its own confirm. |
| VI. Documented | Pass | `lowerModals` and `prompt()` doc comments state the switch and why. |
| VII. Simplicity | Pass | One new export on `Modal`, one call site; no new option, component or dependency. |

Post-design re-check: unchanged, all pass.

## Project Structure

### Documentation (this feature)

```text
specs/013-fix-passkey-confirm-dialog/
├── plan.md
├── research.md
├── quickstart.md
├── contracts/
│   └── passkey-prompt.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks
```

No `data-model.md`: the feature has no data.

### Source Code (repository root)

```text
web/src/
├── webauthn.ts             # prompt(): lowerModals() around the credential call
├── webauthn.test.ts        # lower/restore around each outcome
├── components/
│   ├── Modal.tsx           # open-dialog stack, lowerModals()
│   └── Modal.test.tsx      # lowerModals cases
├── test/
│   └── setup.ts            # jsdom stub for HTMLDialogElement.show()
└── pages/
    └── AdminPage.css       # .modal-lowered, .modal-lowered-top
```

**Structure Decision**: Existing web app layout. `TwoFactorSetup`, `LoginForm` and the other
`Modal` callers do not change; their existing tests must still pass. Added in security review:
`ConfirmIdentity.tsx` ignores a passkey or confirm reply that arrives after the dialog closed
(the prompt is now clickable after Cancel), with tests in `ConfirmIdentity.test.tsx`.

## Complexity Tracking

No violations.
