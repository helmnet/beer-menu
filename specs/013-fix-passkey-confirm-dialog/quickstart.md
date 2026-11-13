# Quickstart: Fix Passkey Prompt Inside Admin Dialogs

## Prerequisites

- Admin app at its `passkeys_origin` on a desktop browser with the 1Password extension, unlocked.
- An admin with two-factor turned on and a passkey stored in 1Password.

## 1. Spike: confirm the cause (done 2026-11-13)

Lowering every open dialog in DevTools
(`document.querySelectorAll("dialog[open]").forEach(d => { d.close(); d.show(); })`) made the
1Password prompt clickable. See research R1.

## 2. Automated

```sh
cd web && npx vitest run --maxWorkers=2
```

Expected: all pass, including the new `Modal.test.tsx` and `webauthn.test.ts` cases (contract in
[contracts/passkey-prompt.md](./contracts/passkey-prompt.md)).

## 3. Manual, after the fix

| # | Steps | Expected |
|---|-------|----------|
| 1 | Create an API token, confirm with password and the 1Password passkey | "Use passkey" works; token created; no reload (SC-001) |
| 2 | Same, but click the 1Password prompt's close button | Dialog stays, password still typed, "Passkey check was cancelled." (SC-003) |
| 3 | Same, press Escape in the 1Password prompt | As row 2; the dialog does not close |
| 4 | Same, choose "Other logins" | The browser's own passkey prompt (or other device) works |
| 5 | Two-Factor Sign-In, Add passkey, save to 1Password | New passkey listed (SC-002) |
| 6 | Open any dialog without a passkey check; press Tab, Escape | Focus stays in dialog; Escape closes; page behind unusable (FR-005) |
| 7 | Confirm with a passkey on a phone; sign in with a passkey | Works as before (SC-004) |
| 8 | During row 1, check the look | "Confirm it is you" stays on top of "New API token", centred, page dimmed |
| 9 | During row 1, click outside the 1Password prompt | Dialog stays open, password still typed |
| 10 | During row 1, click Confirm again | Nothing happens (Confirm disabled); only one passkey check runs |
| 11 | After row 2, check the stack | "Confirm it is you" is still on top of "New API token"; Escape closes only the top one |
| 12 | During row 1, click the dimmed page behind the dialogs | Nothing behind reacts; the 1Password prompt and the top dialog still work (FR-009) |
| 13 | During row 1 or row 5, press Cancel on the dialog | The 1Password prompt goes away; nothing is confirmed or saved (FR-010) |
