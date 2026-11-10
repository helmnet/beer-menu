import { afterEach, describe, expect, it, vi } from "vitest";
import { clampNavWidth, loadNavWidth, loadTheme, saveNavWidth, saveTheme } from "./prefs";

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe("theme", () => {
  it("defaults to system", () => {
    expect(loadTheme()).toBe("system");
  });

  it.each(["light", "dark", "system"] as const)("reads back %s", (theme) => {
    saveTheme(theme);
    expect(localStorage.getItem("beer-menu.admin.theme")).toBe(theme);
    expect(loadTheme()).toBe(theme);
  });

  it("reads an invalid stored value as system", () => {
    localStorage.setItem("beer-menu.admin.theme", "purple");
    expect(loadTheme()).toBe("system");
  });
});

describe("nav width", () => {
  it("defaults to 256", () => {
    expect(loadNavWidth()).toBe(256);
  });

  it("reads back a saved width", () => {
    saveNavWidth(300);
    expect(localStorage.getItem("beer-menu.admin.nav-width")).toBe("300");
    expect(loadNavWidth()).toBe(300);
  });

  it.each(["abc", "250.5", "", "-10"])("reads a non-integer stored width %j as 256", (stored) => {
    localStorage.setItem("beer-menu.admin.nav-width", stored);
    expect(loadNavWidth()).toBe(256);
  });

  it("clamps a stored width on read", () => {
    localStorage.setItem("beer-menu.admin.nav-width", "900");
    expect(loadNavWidth()).toBe(400);
  });

  it.each([
    [100, 1920, 200],
    [256, 1920, 256],
    [900, 1920, 400],
    [400, 700, 350],
    [300, 601, 300],
    [350, 601, 300],
  ])("clamps %i px in a %i px window to %i", (px, windowWidth, want) => {
    expect(clampNavWidth(px, windowWidth)).toBe(want);
  });
});

describe("blocked storage", () => {
  it("falls back to the defaults and does not throw", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(loadTheme()).toBe("system");
    expect(loadNavWidth()).toBe(256);
    expect(() => saveTheme("dark")).not.toThrow();
    expect(() => saveNavWidth(300)).not.toThrow();
  });
});
