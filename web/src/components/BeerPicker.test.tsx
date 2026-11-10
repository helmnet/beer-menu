import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { beer } from "../testApi";
import BeerPicker from "./BeerPicker";

afterEach(cleanup);

// Added in this order: Old IPA first, then Stout, then two beers at the same time (ids 3 and 4).
const beers = [
  beer({ id: 1, name: "Old IPA", created_at: "2026-10-01T12:00:00Z" }),
  beer({ id: 2, name: "Stout", created_at: "2026-10-05T12:00:00Z" }),
  beer({ id: 3, name: "Session ipa", created_at: "2026-11-01T12:00:00Z" }),
  beer({ id: 4, name: "Pils", created_at: "2026-11-01T12:00:00Z" }),
];

const input = () => screen.getByRole("combobox", { name: "Beer" });
const options = () => within(screen.getByRole("listbox", { name: "Beers" })).queryAllByRole("option").map((o) => o.textContent);
const key = (k: string) => fireEvent.keyDown(input(), { key: k });

/** Controlled renders the picker holding its own value, as KegForm does. */
function Controlled({ initial = null, onChange }: { initial?: number | null; onChange: (id: number) => void }) {
  const [value, setValue] = useState<number | null>(initial);
  return (
    <BeerPicker
      beers={beers}
      value={value}
      onChange={(id) => {
        setValue(id);
        onChange(id);
      }}
    />
  );
}

describe("BeerPicker", () => {
  it("is a combobox named Beer controlling a listbox named Beers, closed at first", () => {
    render(<Controlled onChange={vi.fn()} />);
    expect(input().getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("listbox", { name: "Beers" })).toBeNull();
    key("ArrowDown");
    const list = screen.getByRole("listbox", { name: "Beers" });
    expect(input().getAttribute("aria-expanded")).toBe("true");
    expect(input().getAttribute("aria-controls")).toBe(list.id);
  });

  it("lists beers newest first, ties by id descending", () => {
    render(<Controlled onChange={vi.fn()} />);
    fireEvent.click(input());
    expect(options()).toEqual(["Pils", "Session ipa", "Stout", "Old IPA"]);
  });

  it("filters by name ignoring case, trimmed", () => {
    render(<Controlled onChange={vi.fn()} />);
    fireEvent.change(input(), { target: { value: " ipa " } });
    expect(options()).toEqual(["Session ipa", "Old IPA"]);
    expect(screen.queryByText("No beer matches")).toBeNull();
  });

  it("says No beer matches and offers no option (and no create) when nothing matches", () => {
    render(<Controlled onChange={vi.fn()} />);
    fireEvent.change(input(), { target: { value: "porter" } });
    expect(screen.getByText("No beer matches")).toBeTruthy();
    expect(screen.queryAllByRole("option")).toHaveLength(0);
  });

  it("moves the active option with the arrow keys and picks it with Enter", () => {
    const onChange = vi.fn();
    render(<Controlled onChange={onChange} />);
    key("ArrowDown");
    const opts = screen.getAllByRole("option");
    expect(input().getAttribute("aria-activedescendant")).toBe(opts[0].id);
    key("ArrowDown");
    key("ArrowDown");
    expect(input().getAttribute("aria-activedescendant")).toBe(opts[2].id);
    key("ArrowUp");
    expect(input().getAttribute("aria-activedescendant")).toBe(opts[1].id);
    key("Enter");
    expect(onChange).toHaveBeenCalledWith(3);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect((input() as HTMLInputElement).value).toBe("Session ipa");
  });

  it("picks an option by click", () => {
    const onChange = vi.fn();
    render(<Controlled onChange={onChange} />);
    fireEvent.change(input(), { target: { value: "sto" } });
    fireEvent.click(screen.getByRole("option", { name: "Stout" }));
    expect(onChange).toHaveBeenCalledWith(2);
    expect((input() as HTMLInputElement).value).toBe("Stout");
  });

  it("marks the chosen beer aria-selected", () => {
    render(<Controlled initial={2} onChange={vi.fn()} />);
    expect((input() as HTMLInputElement).value).toBe("Stout");
    fireEvent.click(input());
    expect(screen.getByRole("option", { name: "Stout" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("option", { name: "Pils" }).getAttribute("aria-selected")).toBe("false");
  });

  it("closes the list on Escape without letting the key reach the dialog", () => {
    render(<Controlled onChange={vi.fn()} />);
    key("ArrowDown");
    const event = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
    const outside = vi.fn();
    document.addEventListener("keydown", outside);
    fireEvent(input(), event);
    document.removeEventListener("keydown", outside);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(input().getAttribute("aria-expanded")).toBe("false");
    expect(event.defaultPrevented).toBe(true);
    expect(outside).not.toHaveBeenCalled();
  });

  it("leaves Escape alone when the list is closed", () => {
    render(<Controlled onChange={vi.fn()} />);
    const event = new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true });
    fireEvent(input(), event);
    expect(event.defaultPrevented).toBe(false);
  });

  it("keeps the current beer selected when the filter hides it", () => {
    const onChange = vi.fn();
    render(<Controlled initial={2} onChange={onChange} />);
    fireEvent.change(input(), { target: { value: "ipa" } });
    expect(options()).toEqual(["Session ipa", "Old IPA"]);
    key("Escape");
    expect(onChange).not.toHaveBeenCalled();
    expect((input() as HTMLInputElement).value).toBe("Stout");
    fireEvent.change(input(), { target: { value: "porter" } });
    fireEvent.blur(input());
    expect((input() as HTMLInputElement).value).toBe("Stout");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("marks itself invalid and points at the error when given one", () => {
    render(<BeerPicker beers={beers} value={null} onChange={vi.fn()} errorId="err" />);
    expect(input().getAttribute("aria-invalid")).toBe("true");
    expect(input().getAttribute("aria-describedby")).toBe("err");
  });
});
