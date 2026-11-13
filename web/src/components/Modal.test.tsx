import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import Modal, { lowerModals } from "./Modal";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Modal", () => {
  it("is a dialog named by its title, opened with showModal", () => {
    const showModal = vi.spyOn(HTMLDialogElement.prototype, "showModal");
    render(
      <Modal title="Add tap" onClose={vi.fn()}>
        <p>Body</p>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog", { name: "Add tap" });
    expect(showModal).toHaveBeenCalledTimes(1);
    expect((dialog as HTMLDialogElement).open).toBe(true);
    expect(screen.getByRole("heading", { level: 2, name: "Add tap" })).toBeTruthy();
    expect(screen.getByText("Body")).toBeTruthy();
  });

  it("turns Escape (cancel) into onClose and keeps the dialog open itself", () => {
    const onClose = vi.fn();
    render(
      <Modal title="Add tap" onClose={onClose}>
        <p>Body</p>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog", { name: "Add tap" });
    const cancel = new Event("cancel", { cancelable: true });
    fireEvent(dialog, cancel);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(cancel.defaultPrevented).toBe(true);
  });

  it("calls onClose from its Close button", () => {
    const onClose = vi.fn();
    render(
      <Modal title="Add tap" onClose={onClose}>
        <p>Body</p>
      </Modal>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on unmount and returns focus to the element focused before it opened", () => {
    const close = vi.spyOn(HTMLDialogElement.prototype, "close");
    const opener = document.createElement("button");
    document.body.appendChild(opener);
    opener.focus();
    const { unmount } = render(
      <Modal title="Add tap" onClose={vi.fn()}>
        <input aria-label="Label" />
      </Modal>,
    );
    screen.getByLabelText("Label").focus();
    unmount();
    expect(close).toHaveBeenCalled();
    expect(document.activeElement).toBe(opener);
    opener.remove();
  });

  it("stays open under StrictMode's double mount", () => {
    render(
      <StrictMode>
        <Modal title="Add tap" onClose={vi.fn()}>
          <p>Body</p>
        </Modal>
      </StrictMode>,
    );
    expect((screen.getByRole("dialog", { name: "Add tap" }) as HTMLDialogElement).open).toBe(true);
  });
});

describe("lowerModals", () => {
  const firstModal = (
    <Modal title="First" onClose={vi.fn()}>
      <p>Body</p>
    </Modal>
  );

  // renderStack mounts "First", then "Second" after it, so Second is on top.
  function renderStack() {
    const view = render(<>{firstModal}</>);
    view.rerender(
      <>
        {firstModal}
        <Modal title="Second" onClose={vi.fn()}>
          <input aria-label="Password" />
        </Modal>
      </>,
    );
    const first = screen.getByRole("dialog", { name: "First" }) as HTMLDialogElement;
    const second = screen.getByRole("dialog", { name: "Second" }) as HTMLDialogElement;
    return { ...view, first, second };
  }

  it("lowers every open dialog and marks the top one", () => {
    const { first, second } = renderStack();
    const show = vi.spyOn(HTMLDialogElement.prototype, "show");
    lowerModals();
    expect(show.mock.contexts).toEqual([first, second]);
    expect(first.classList.contains("modal-lowered")).toBe(true);
    expect(second.classList.contains("modal-lowered")).toBe(true);
    expect(first.classList.contains("modal-lowered-top")).toBe(false);
    expect(second.classList.contains("modal-lowered-top")).toBe(true);
  });

  it("restores the dialogs as modal in opening order", () => {
    const { first, second } = renderStack();
    const restore = lowerModals();
    const showModal = vi.spyOn(HTMLDialogElement.prototype, "showModal");
    restore();
    for (const dialog of [first, second]) {
      expect(dialog.classList.contains("modal-lowered")).toBe(false);
      expect(dialog.classList.contains("modal-lowered-top")).toBe(false);
      expect(dialog.open).toBe(true);
    }
    expect(showModal.mock.contexts).toEqual([first, second]);
  });

  it("does not reopen a dialog unmounted while lowered", () => {
    const { first, rerender } = renderStack();
    const restore = lowerModals();
    rerender(<>{firstModal}</>);
    const showModal = vi.spyOn(HTMLDialogElement.prototype, "showModal");
    restore();
    expect(showModal.mock.contexts).toEqual([first]);
  });

  it("returns focus to the element focused when it lowered", () => {
    renderStack();
    const outside = document.createElement("button");
    document.body.appendChild(outside);
    const input = screen.getByLabelText("Password");
    input.focus();
    const restore = lowerModals();
    outside.focus();
    restore();
    expect(document.activeElement).toBe(input);
    outside.remove();
  });

  it("does nothing with no dialog open", () => {
    const show = vi.spyOn(HTMLDialogElement.prototype, "show");
    const showModal = vi.spyOn(HTMLDialogElement.prototype, "showModal");
    lowerModals()();
    expect(show).not.toHaveBeenCalled();
    expect(showModal).not.toHaveBeenCalled();
  });
});
