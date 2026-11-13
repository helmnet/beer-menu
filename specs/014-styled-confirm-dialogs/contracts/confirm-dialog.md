# UI Contract: Confirmation Dialog

## Component interface

`web/src/components/ConfirmDialog.tsx` exports:

- `useConfirmAction()` returns `{ ask, dialog }`.
  - `ask(request: { title: string; message: string; confirmLabel: string }, action: () => void)` opens the dialog. Calling it again while a dialog is open replaces the request.
  - `dialog` is a React node, or `null` when nothing is pending. The caller renders it inside its own `.admin` subtree.
- The default export is `ConfirmDialog({ title, message, confirmLabel, onConfirm, onCancel })`, the presentational dialog the hook renders.

## Rendered structure

Built from `Modal`:

```text
<dialog class="modal" aria-labelledby=…>       role dialog, name = title
  <h2>{title}</h2>
  <p>{message}</p>
  <div class="btn-row">
    <button class="btn btn-danger">{confirmLabel}</button>
    <button class="btn">Cancel</button>         focused on open
  </div>
  <button class="icon-btn modal-close" aria-label="Close">
</dialog>
```

## Behavior

| Input | Result |
|-------|--------|
| Confirm button | The dialog closes, then `action` runs once. |
| Cancel, Close (X) or Escape | The dialog closes. `action` does not run. |
| Click outside the dialog | Nothing happens. |
| Open | Focus is on Cancel. The page behind is inert. |
| Close (any way) | Focus returns to the element that opened the dialog. |

## Call-site table (FR-004)

| Where | Title | Message (unchanged) | Confirm label |
|-------|-------|---------------------|---------------|
| Beer Library | Delete beer | `Delete ${name} permanently?` | Delete |
| Keg History | Delete keg | `Delete the ${beer} keg from history permanently?` | Delete |
| Taps | Remove tap | `Remove ${label}?` | Remove |
| Keg card | Kick keg | `Kick the ${beer} keg? It moves to keg history.` | Kick |
| Admin Users (other admin) | Remove admin | `Remove admin "${username}"?` | Remove |
| Admin Users (self) | Remove yourself | `Remove yourself? You will be signed out.` | Remove |
| Admin Users | Clear two-factor sign-in | `Clear two-factor sign-in for "${username}"? They will be signed out everywhere.` | Clear |
| API Tokens | Revoke token | `Revoke token "${name}"? Clients using it stop working at once.` | Revoke |
| Two-Factor Sign-In | Remove second factor | `Remove ${label}? Your other sessions will be signed out.` | Remove |
| Two-Factor Sign-In | Turn off 2FA | `Turn off two-factor sign-in? This removes every factor and recovery code.` | Turn off |
| Brewfather | Disconnect Brewfather | `Disconnect Brewfather? Imported beers stay, and scheduled sync stops.` | Disconnect |
