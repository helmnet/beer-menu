import { X } from "lucide-react";
import { ReactNode, useId, useLayoutEffect, useRef } from "react";

// Mounted Modal dialogs, in opening order (the last one is on top).
const open: HTMLDialogElement[] = [];

/**
 * lowerModals makes every open Modal non-modal while a passkey prompt runs, and returns restore
 * to make them modal again. A modal dialog makes the rest of the page inert, which blocks
 * password-manager prompts drawn in the page (such as 1Password's "Use passkey"). Every open
 * dialog is lowered, because any one left modal still blocks the prompt (research R1, R2).
 * Restore reopens them in opening order, because only that order puts the newest dialog back on
 * top; the DOM order does not. CSS classes keep the lowered stack looking the same (research R3).
 * Restore also returns focus to the element that had it, and skips dialogs unmounted meanwhile.
 */
export function lowerModals(): () => void {
  const lowered = [...open];
  if (lowered.length === 0) return () => {};
  const active = document.activeElement;
  const focused = active instanceof HTMLElement && active !== document.body ? active : null;
  lowered.forEach((dialog, i) => {
    dialog.close();
    dialog.show();
    dialog.classList.add("modal-lowered");
    if (i === lowered.length - 1) dialog.classList.add("modal-lowered-top");
  });
  return () => {
    for (const dialog of lowered) {
      dialog.classList.remove("modal-lowered", "modal-lowered-top");
      if (dialog.isConnected && open.includes(dialog) && dialog.open) {
        dialog.close();
        dialog.showModal();
      }
    }
    if (focused?.isConnected) focused.focus();
  };
}

/**
 * Modal is a native `<dialog>` opened with `showModal()` while mounted (research R3): the page
 * behind is inert, so keyboard focus stays inside, and its role is `dialog`, named by its `h2`
 * `title` (`aria-labelledby`). The caller decides whether it shows by mounting it; Modal never
 * closes itself. Escape (the `cancel` event) is prevented and turned into `onClose`, as is its
 * "Close" icon button, so the caller unmounts it. Unmounting calls `close()` and returns focus
 * to the element that was focused when it opened (normally the button that opened it).
 * It renders in place, not in a portal, so the `.admin` styles apply to it. Safe under
 * StrictMode's mount, unmount, mount: each mount opens it and each unmount closes it.
 * Open dialogs are also kept on a module-level stack in opening order, for lowerModals.
 */
export default function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useLayoutEffect(() => {
    const dialog = ref.current!;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (!dialog.open) dialog.showModal();
    open.push(dialog);
    return () => {
      const i = open.indexOf(dialog);
      if (i >= 0) open.splice(i, 1);
      if (dialog.open) dialog.close();
      opener?.focus();
    };
  }, []);

  return (
    <dialog
      ref={ref}
      className="modal"
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      <h2 id={titleId}>{title}</h2>
      {children}
      {/* Last in the DOM so showModal focuses the first field, not this; CSS puts it top right. */}
      <button type="button" className="icon-btn modal-close" aria-label="Close" title="Close" onClick={onClose}>
        <X aria-hidden="true" />
      </button>
    </dialog>
  );
}
