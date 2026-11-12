import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RecoveryCodes from "./RecoveryCodes";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const codes = Array.from({ length: 10 }, (_, i) => `abcd${"234567abcd"[i]}-efghi`);

describe("RecoveryCodes", () => {
  it("lists the codes once with a warning", () => {
    render(<RecoveryCodes codes={codes} onDone={vi.fn()} />);
    const items = within(screen.getByRole("list", { name: "Recovery codes" })).getAllByRole("listitem");
    expect(items.map((li) => li.textContent)).toEqual(codes);
    expect(screen.getByText(/shown only once/)).toBeTruthy();
  });

  it("copies the codes one per line", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
    render(<RecoveryCodes codes={codes} onDone={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    expect(await screen.findByText("Copied.")).toBeTruthy();
    expect(writeText).toHaveBeenCalledWith(codes.join("\n"));
  });

  it("says when copying fails", async () => {
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText: vi.fn().mockRejectedValue(new Error("no")) } });
    render(<RecoveryCodes codes={codes} onDone={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    expect(await screen.findByText("Copy failed. Select the codes and copy them by hand.")).toBeTruthy();
  });

  it("downloads the codes as a text file", async () => {
    let blob: Blob | undefined;
    const createObjectURL = vi.fn((b: Blob) => {
      blob = b;
      return "blob:codes";
    });
    const revokeObjectURL = vi.fn();
    // A subclass, so the real URL keeps no stub after the test.
    vi.stubGlobal("URL", Object.assign(class extends URL {}, { createObjectURL, revokeObjectURL }));
    let clicked: HTMLAnchorElement | undefined;
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clicked = this;
    });
    render(<RecoveryCodes codes={codes} onDone={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Download" }));
    expect(clicked?.download).toBe("beer-menu-recovery-codes.txt");
    expect(clicked?.href).toBe("blob:codes");
    expect(blob?.type).toBe("text/plain");
    expect(await blob?.text()).toBe(codes.join("\n") + "\n");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:codes");
  });

  it("enables Done only after the codes are saved", () => {
    const onDone = vi.fn();
    render(<RecoveryCodes codes={codes} onDone={onDone} />);
    const done = screen.getByRole("button", { name: "Done" }) as HTMLButtonElement;
    expect(done.disabled).toBe(true);
    fireEvent.click(done);
    expect(onDone).not.toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText("I saved these codes"));
    expect(done.disabled).toBe(false);
    fireEvent.click(done);
    expect(onDone).toHaveBeenCalled();
  });
});
