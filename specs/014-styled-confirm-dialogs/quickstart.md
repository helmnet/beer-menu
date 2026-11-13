# Quickstart: Styled Confirmation Dialogs

## Automated

```sh
cd web
npm run lint
npx vitest run --maxWorkers=2
```

These results are expected:

- `ConfirmDialog.test.tsx` covers the following:
  - The dialog is named by its title and shows its message.
  - Focus starts on Cancel.
  - Confirm runs the action once.
  - Cancel, Close and Escape each skip the action.
  - Focus returns to the opener.
  - No source file calls `window.confirm` (see [contract](./contracts/confirm-dialog.md)).
- Each panel test opens its confirmation as a dialog. Cancel sends no request, and confirm sends the request.

## Manual (both themes, desktop and phone width)

1. Sign in as an admin. Open API Tokens and click Revoke on a token. Expected: the app dialog "Revoke token" opens, with no browser box, the red Revoke button and the plain Cancel button. It looks like the "Add admin" dialog.
2. Press Enter straight away. Expected: the dialog closes and the token is still listed, because focus started on Cancel.
3. Click Revoke again, then click outside the dialog. Expected: nothing happens. Press Escape. Expected: the dialog closes and focus is back on the Revoke button.
4. Click Revoke, then click the red Revoke button in the dialog. Expected: the token is gone.
5. Repeat the open, cancel and confirm steps for each row of the call-site table in the [contract](./contracts/confirm-dialog.md).
6. With 2FA on, remove a passkey after the confirmation. When "Confirm it is you" is required, expected: that dialog opens next and works.
