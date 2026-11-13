import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useConfirmAction } from "./ConfirmDialog";

afterEach(cleanup);

/** Harness opens one confirmation for `action` from its "Open" button. */
function Harness({ action }: { action: () => void }) {
  const confirm = useConfirmAction();
  return (
    <div>
      <button
        type="button"
        onClick={() =>
          confirm.ask({ title: "Delete beer", message: "Delete <b>Stout</b> permanently?", confirmLabel: "Delete" }, action)
        }
      >
        Open
      </button>
      {confirm.dialog}
    </div>
  );
}

/** open renders the harness, focuses and clicks its opener, and returns the action and opener. */
function open() {
  const action = vi.fn();
  render(<Harness action={action} />);
  const opener = screen.getByRole("button", { name: "Open" });
  opener.focus();
  fireEvent.click(opener);
  return { action, opener };
}

describe("ConfirmDialog", () => {
  it("shows the title, the message as text and red confirm and plain Cancel buttons", () => {
    open();
    const dialog = screen.getByRole("dialog", { name: "Delete beer" });
    expect(dialog.textContent).toContain("Delete <b>Stout</b> permanently?");
    expect(dialog.querySelector("b")).toBeNull();
    expect(screen.getByRole("button", { name: "Delete" }).className).toBe("btn btn-danger");
    const cancel = screen.getByRole("button", { name: "Cancel" });
    expect(cancel.classList.contains("btn")).toBe(true);
    expect(cancel.classList.contains("btn-danger")).toBe(false);
  });

  it("focuses Cancel when it opens", () => {
    open();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Cancel" }));
  });

  it("closes and runs the action once on confirm, even when clicked twice", () => {
    const { action, opener } = open();
    const confirmButton = screen.getByRole("button", { name: "Delete" });
    fireEvent.click(confirmButton);
    fireEvent.click(confirmButton);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(action).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(opener);
  });

  it("closes without the action on Cancel", () => {
    const { action, opener } = open();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(action).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(opener);
  });

  it("closes without the action on Close", () => {
    const { action, opener } = open();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(action).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(opener);
  });

  it("closes without the action on Escape", () => {
    const { action, opener } = open();
    fireEvent(screen.getByRole("dialog", { name: "Delete beer" }), new Event("cancel", { cancelable: true }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(action).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(opener);
  });

  it("is the only confirmation: no source file calls the browser's confirmation box", () => {
    const sources = import.meta.glob<string>("../**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true });
    expect(sources["./AdminsPanel.tsx"]).toContain("useConfirmAction");
    const offenders = Object.entries(sources)
      .filter(([path, text]) => !/\.test\.tsx?$/.test(path) && /window\.confirm\s*\(/.test(text))
      .map(([path]) => path);
    expect(offenders).toEqual([]);
  });
});
