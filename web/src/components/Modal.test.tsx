import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import Modal from "./Modal";

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
