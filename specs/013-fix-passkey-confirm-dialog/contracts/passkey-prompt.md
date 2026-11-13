# Contract: passkey prompt and open dialogs

## `web/src/components/Modal.tsx`

- The default export `Modal` keeps its current props and behaviour. In addition, each mounted
  `Modal` records its dialog on a module-level stack in opening order: it pushes after
  `showModal()`, and its unmount cleanup removes the dialog.
- New export: `lowerModals(): () => void`

| When | Then |
|------|------|
| `lowerModals()` is called while N ≥ 1 `Modal`s are open | Each dialog on the stack, bottom first, runs `close()` then `show()` and gets class `modal-lowered`. The last (top) dialog also gets `modal-lowered-top`. |
| `lowerModals()` is called with no `Modal` open | Nothing changes. The returned `restore()` does nothing. |
| `restore()` is called | Each lowered dialog loses both classes. Each one still connected and still on the stack runs `close()` then `showModal()`, bottom first, so the original stacking returns. Focus then goes back to the element focused when `lowerModals()` ran, if it was not `document.body` and is still connected. |
| A `Modal` unmounts while lowered | It is not reopened by `restore()`. |

## `web/src/webauthn.ts`

Public signatures are unchanged:

- `getPasskey(options): Promise<PasskeyReply>`
- `createPasskey(options): Promise<PasskeyReply>`

The shared `prompt()` calls `lowerModals()` before the browser's credential call and `restore()`
in a `finally` block. It does this on success, on `"cancelled"`, and on every error. Return values
and error mapping (`NotAllowedError` to `"cancelled"`, `InvalidStateError` to the "already has a
passkey" error) are unchanged.

## Styling

- `.admin dialog.modal.modal-lowered`: centred, above the admin layout.
- `.admin dialog.modal.modal-lowered-top`: above the other lowered dialogs, with the scrim over
  everything else.

Together these look the same as the modal stack.
