import { X } from "lucide-react";
import { ReactNode, useId, useLayoutEffect, useRef } from "react";

/**
 * Modal is a native `<dialog>` opened with `showModal()` while mounted (research R3): the page
 * behind is inert, so keyboard focus stays inside, and its role is `dialog`, named by its `h2`
 * `title` (`aria-labelledby`). The caller decides whether it shows by mounting it; Modal never
 * closes itself. Escape (the `cancel` event) is prevented and turned into `onClose`, as is its
 * "Close" icon button, so the caller unmounts it. Unmounting calls `close()` and returns focus
 * to the element that was focused when it opened (normally the button that opened it).
 * It renders in place, not in a portal, so the `.admin` styles apply to it. Safe under
 * StrictMode's mount, unmount, mount: each mount opens it and each unmount closes it.
 */
export default function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useLayoutEffect(() => {
    const dialog = ref.current!;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (!dialog.open) dialog.showModal();
    return () => {
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
