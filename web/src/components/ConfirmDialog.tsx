import { useEffect, useRef, useState } from "react";
import Modal from "./Modal";

/** ConfirmRequest is what a confirmation shows: the dialog title, the question and the confirm button's label. */
interface ConfirmRequest {
  title: string;
  message: string;
  confirmLabel: string;
}

/**
 * ConfirmDialog asks the admin to confirm a destructive action, in place of the browser's
 * confirmation box. It is a Modal titled `title` that shows `message` as text, a red confirm
 * button labelled `confirmLabel` (`onConfirm`) and a plain Cancel button (`onCancel`). Focus
 * starts on Cancel, so Enter does not confirm by accident. Cancel, Close and Escape call
 * `onCancel`; clicking outside does nothing.
 */
export default function ConfirmDialog({
  title,
  message,
  confirmLabel,
  onConfirm,
  onCancel,
}: ConfirmRequest & { onConfirm: () => void; onCancel: () => void }) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  // showModal focuses the first button (confirm); this runs after Modal's layout effect, so Cancel wins.
  useEffect(() => cancelRef.current?.focus(), []);

  return (
    <Modal title={title} onClose={onCancel}>
      <p>{message}</p>
      <div className="btn-row">
        <button type="button" className="btn btn-danger" onClick={onConfirm}>
          {confirmLabel}
        </button>
        <button type="button" className="btn" ref={cancelRef} onClick={onCancel}>
          Cancel
        </button>
      </div>
    </Modal>
  );
}

/**
 * useConfirmAction asks before a destructive action (named apart from `useConfirm` in
 * ConfirmIdentity.tsx, which asks for the password instead): `ask(request, action)` shows
 * `dialog`, a ConfirmDialog for `request`; asking again while it is open replaces the request.
 * Cancel, Close or Escape closes it and drops `action`; confirm closes it,
 * then runs `action` once. The caller renders `dialog` inside its `.admin` subtree.
 */
export function useConfirmAction() {
  const [pending, setPending] = useState<(ConfirmRequest & { action: () => void }) | null>(null);

  function ask(request: ConfirmRequest, action: () => void) {
    setPending({ ...request, action });
  }

  const dialog = pending && (
    <ConfirmDialog
      title={pending.title}
      message={pending.message}
      confirmLabel={pending.confirmLabel}
      onConfirm={() => {
        setPending(null);
        pending.action();
      }}
      onCancel={() => setPending(null)}
    />
  );
  return { ask, dialog };
}
