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

Public signatures gain an optional `signal` (FR-010); callers without one, like `LoginForm`, are unchanged:

- `getPasskey(options, signal?: AbortSignal): Promise<PasskeyReply>`
- `createPasskey(options, signal?: AbortSignal): Promise<PasskeyReply>`

`signal` is passed to `navigator.credentials.get/create({ publicKey, signal })`.

The shared `prompt()` calls `lowerModals()` before the browser's credential call and `restore()`
in a `finally` block. It does this on success, on `"cancelled"`, and on every error. Return values
and error mapping (`NotAllowedError` to `"cancelled"`, `InvalidStateError` to the "already has a
passkey" error) are unchanged; an aborted prompt (`AbortError`) also maps to `"cancelled"`.
`prompt()` stops waiting when `signal` aborts, even if a password manager never settles the call,
so `restore()` still runs; a signal already aborted returns `"cancelled"` without lowering or prompting.

New export: `usePromptAbort(): { signal(): AbortSignal; abort(): void }`, a React hook for a
dialog that starts a prompt. `signal()` makes a new `AbortController` and returns its signal;
`abort()` aborts the latest one; unmounting the component also aborts it.

- `ConfirmIdentity` passes `signal()` to `getPasskey`, taken before the options fetch so a close
  during that fetch aborts too; unmounting (Cancel, Close, Escape) aborts.
- `TwoFactorSetup` passes `signal()` to `createPasskey`; `cancel()` and unmounting abort. After an
  abort it sends no `POST /api/2fa/passkeys` and shows no error, even if the prompt answered.

## Styling

- `.admin dialog.modal.modal-lowered`: centred, above the admin layout.
- `.admin dialog.modal.modal-lowered-top`: above the other lowered dialogs, with the scrim over
  everything else, and clickable.
- `.admin:has(dialog.modal-lowered)`: `pointer-events: none`, so clicks reach only the top dialog
  and the password manager's prompt, which sits outside `.admin` (FR-009).

Together these look the same as the modal stack.
