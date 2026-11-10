import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import ThemeControl from "./ThemeControl";

afterEach(cleanup);

describe("ThemeControl", () => {
  it("is a radio group named Theme with Light, Dark and System", () => {
    render(<ThemeControl theme="system" onChange={vi.fn()} />);
    const group = screen.getByRole("radiogroup", { name: "Theme" });
    expect(within(group).getAllByRole("radio").map((r) => r.getAttribute("value"))).toEqual(["light", "dark", "system"]);
    for (const name of ["Light", "Dark", "System"]) expect(within(group).getByRole("radio", { name })).toBeTruthy();
  });

  it.each([
    ["light", "Light"],
    ["dark", "Dark"],
    ["system", "System"],
  ] as const)("checks %s when given it", (theme, name) => {
    render(<ThemeControl theme={theme} onChange={vi.fn()} />);
    expect((screen.getByRole("radio", { name }) as HTMLInputElement).checked).toBe(true);
    expect(screen.getAllByRole("radio").filter((r) => (r as HTMLInputElement).checked)).toHaveLength(1);
  });

  it.each([
    ["Light", "light"],
    ["Dark", "dark"],
  ])("calls onChange when %s is chosen", (name, want) => {
    const onChange = vi.fn();
    render(<ThemeControl theme="system" onChange={onChange} />);
    fireEvent.click(screen.getByRole("radio", { name }));
    expect(onChange).toHaveBeenCalledWith(want);
  });

  it("calls onChange with system", () => {
    const onChange = vi.fn();
    render(<ThemeControl theme="dark" onChange={onChange} />);
    fireEvent.click(screen.getByRole("radio", { name: "System" }));
    expect(onChange).toHaveBeenCalledWith("system");
  });
});
