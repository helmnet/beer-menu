import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { MenuTap } from "../api";
import { menu, settings, stubApi } from "../testApi";
import { FONTS } from "../fonts";
import { SAMPLE_MENU } from "../tv";
import SettingsPanel from "./SettingsPanel";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const unauthenticated = { status: 401, body: { error: "unauthenticated", message: "Sign in." } };
const loaded = settings({ title: "Helm Taproom", theme: "light", units: "metric", page_seconds: 20 });

function value(label: string) {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

function type(label: string, v: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value: v } });
}

/** errorFor returns the error text linked to the control labelled label, or null. */
function errorFor(label: string): string | null {
  const id = screen.getByLabelText(label).getAttribute("aria-describedby");
  return id ? (document.getElementById(id)?.textContent ?? null) : null;
}

/** shown renders the panel and waits for the loaded title. */
async function shown(onChange = vi.fn(), onSignedOut = vi.fn()) {
  render(<SettingsPanel onChange={onChange} onSignedOut={onSignedOut} />);
  await waitFor(() => expect(value("Title")).toBe("Helm Taproom"));
  return { onChange, onSignedOut };
}

describe("SettingsPanel", () => {
  it("loads and shows the settings and reports them", async () => {
    const calls = stubApi({ "GET /api/settings": { status: 200, body: loaded } });
    const { onChange } = await shown();
    expect(calls.map((c) => c.key).sort()).toEqual(["GET /api/menu", "GET /api/settings"]);
    expect(value("Theme")).toBe("light");
    expect(value("Units")).toBe("metric");
    expect(value("Page seconds")).toBe("20");
    const options = (label: string) =>
      Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => [o.value, o.text]);
    expect(options("Theme")).toEqual([
      ["dark", "Dark"],
      ["light", "Light"],
    ]);
    expect(options("Units")).toEqual([
      ["us", "US"],
      ["metric", "Metric"],
    ]);
    const seconds = screen.getByLabelText("Page seconds");
    expect(seconds.getAttribute("type")).toBe("number");
    expect(seconds.getAttribute("min")).toBe("5");
    expect(seconds.getAttribute("max")).toBe("300");
    expect(seconds.getAttribute("step")).toBe("1");
    expect(onChange).toHaveBeenCalledWith(loaded);
  });

  it("saves only the changed fields and reports the reply", async () => {
    const reply = { ...loaded, units: "us" as const, page_seconds: 30 };
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: loaded },
      "PATCH /api/settings": { status: 200, body: reply },
    });
    const { onChange } = await shown();
    type("Units", "us");
    type("Page seconds", "30");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(reply));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ units: "us", page_seconds: 30 });
  });

  it("sends nothing when nothing changed", async () => {
    const calls = stubApi({ "GET /api/settings": { status: 200, body: loaded } });
    const { onChange } = await shown();
    type("Title", "Helm Taproom");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await new Promise((r) => setTimeout(r, 0));
    expect(calls.filter((c) => c.key !== "GET /api/menu").map((c) => c.key)).toEqual(["GET /api/settings"]);
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("shows field errors beside their fields and keeps what was typed", async () => {
    stubApi({
      "GET /api/settings": { status: 200, body: loaded },
      "PATCH /api/settings": {
        status: 400,
        body: {
          error: "invalid_request",
          message: "Some fields are invalid.",
          fields: { title: "Use at most 60 characters.", page_seconds: "Use 5 to 300 seconds." },
        },
      },
    });
    const { onChange } = await shown();
    const long = "x".repeat(61);
    type("Title", long);
    type("Page seconds", "4");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(errorFor("Title")).toBe("Use at most 60 characters."));
    expect(errorFor("Page seconds")).toBe("Use 5 to 300 seconds.");
    expect(screen.getByLabelText("Title").getAttribute("aria-invalid")).toBe("true");
    expect(errorFor("Theme")).toBeNull();
    expect(screen.getByRole("alert").textContent).toBe("Some fields are invalid.");
    expect(value("Title")).toBe(long);
    expect(value("Page seconds")).toBe("4");
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("reports a 401 on load as signed out", async () => {
    stubApi({ "GET /api/settings": unauthenticated });
    const onSignedOut = vi.fn();
    render(<SettingsPanel onChange={vi.fn()} onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("reports a 401 on save as signed out", async () => {
    stubApi({ "GET /api/settings": { status: 200, body: loaded }, "PATCH /api/settings": unauthenticated });
    const { onSignedOut } = await shown();
    type("Theme", "dark");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("shows other errors", async () => {
    stubApi({ "GET /api/settings": { status: 500, body: { error: "internal", message: "Something broke." } } });
    const onChange = vi.fn();
    render(<SettingsPanel onChange={onChange} onSignedOut={vi.fn()} />);
    expect((await screen.findByRole("alert")).textContent).toBe("Something broke.");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("shows the five tap detail switches and saves only the changed ones as booleans", async () => {
    const start = { ...loaded, show_ibu: false, show_srm: true };
    const reply = { ...start, show_style: false, show_ibu: true };
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: start },
      "PATCH /api/settings": { status: 200, body: reply },
    });
    const { onChange } = await shown();
    const box = (label: string) => screen.getByLabelText(label) as HTMLInputElement;
    expect(screen.getByRole("group", { name: "Tap details" })).toBeTruthy();
    expect(["Style", "ABV", "IBU", "SRM", "Description"].map((l) => [box(l).type, box(l).checked])).toEqual([
      ["checkbox", true],
      ["checkbox", true],
      ["checkbox", false],
      ["checkbox", true],
      ["checkbox", false],
    ]);
    fireEvent.click(box("Style"));
    fireEvent.click(box("IBU"));
    fireEvent.click(box("Description"));
    fireEvent.click(box("Description"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(reply));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ show_style: false, show_ibu: true });
    expect(box("Style").checked).toBe(false);
  });

  it("shows the title alignment and both fonts and saves only the changed ones", async () => {
    const start = { ...loaded, title_align: "center" as const, title_font: "lora" as const, text_font: "inter" as const };
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: start },
      "PATCH /api/settings": { status: 200, body: { ...start, title_align: "right", text_font: "bebas-neue" } },
    });
    await shown();
    const options = (label: string) => Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => [o.value, o.text]);
    expect(screen.getByRole("group", { name: "Title" })).toBeTruthy();
    expect(screen.getByRole("group", { name: "Text" })).toBeTruthy();
    expect([value("Title alignment"), value("Title font"), value("Text font")]).toEqual(["center", "lora", "inter"]);
    expect(options("Title alignment")).toEqual([
      ["left", "Left"],
      ["center", "Center"],
      ["right", "Right"],
    ]);
    const fonts = FONTS.map((f) => [f.key, f.label]);
    expect(options("Title font")).toEqual(fonts);
    expect(options("Text font")).toEqual(fonts);
    type("Title alignment", "right");
    type("Text font", "bebas-neue");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.some((c) => c.key === "PATCH /api/settings")).toBe(true));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ title_align: "right", text_font: "bebas-neue" });
  });

  it("shows color pickers with the stored color, or the theme's text color and a note", async () => {
    stubApi({ "GET /api/settings": { status: 200, body: { ...loaded, title_color: "#ffaa00", text_color: "" } } });
    await shown();
    const picker = (label: string) => screen.getByLabelText(label) as HTMLInputElement;
    expect(picker("Title color").type).toBe("color");
    expect(picker("Title color").value).toBe("#ffaa00");
    // loaded is the light theme, whose TV text is #141416.
    expect(picker("Text color").value).toBe("#141416");
    const note = (label: string) => document.getElementById(picker(label).getAttribute("aria-describedby") ?? "")?.textContent;
    expect(note("Text color")).toBe("Theme color");
    expect(note("Title color")).toBeUndefined();
    type("Theme", "dark");
    expect(picker("Text color").value).toBe("#f2f2f2");
  });

  it("resets a color to the theme's and saves colors lowercase with no contrast warning", async () => {
    const start = { ...loaded, title_color: "#ffaa00" };
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: start },
      "PATCH /api/settings": { status: 200, body: { ...start, title_color: "", text_color: "#ececec" } },
    });
    await shown();
    fireEvent.click(screen.getByRole("button", { name: "Reset title color" }));
    expect(document.getElementById(screen.getByLabelText("Title color").getAttribute("aria-describedby") ?? "")?.textContent).toBe(
      "Theme color",
    );
    expect((screen.getByRole("button", { name: "Reset title color" }) as HTMLButtonElement).disabled).toBe(true);
    // Light grey text on the light theme's white cards: low contrast, saved anyway.
    type("Text color", "#ECECEC");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.some((c) => c.key === "PATCH /api/settings")).toBe(true));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ title_color: "", text_color: "#ececec" });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the layout select and saves a changed layout", async () => {
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: { ...loaded, layout: "list" } },
      "PATCH /api/settings": { status: 200, body: { ...loaded, layout: "columns" } },
    });
    await shown();
    expect(value("Layout")).toBe("list");
    expect(Array.from((screen.getByLabelText("Layout") as HTMLSelectElement).options).map((o) => [o.value, o.text])).toEqual([
      ["grid", "Grid"],
      ["list", "Vertical list"],
      ["columns", "Side by side"],
    ]);
    type("Layout", "columns");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(value("Layout")).toBe("columns"));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ layout: "columns" });
  });
});

const nineTaps: MenuTap[] = Array.from({ length: 9 }, (_, i) => ({
  label: `Tap ${i + 1}`,
  beer: { name: `Beer ${i + 1}`, style: null, abv: 5, ibu: null, color: null },
  keg: { shape: "corny", level: 0.5 },
}));
// 9 taps and 1 on deck item: 3 pages in grid and list, 2 in columns.
const previewMenu = menu({ taps: nineTaps, on_deck: [{ beer: { name: "Porter", style: null, abv: null, ibu: null, color: null }, keg: null, status: "fermenting" }] });

const preview = () => screen.getByRole("region", { name: "TV preview" });
const tv = () => preview().querySelector(".display") as HTMLElement;
const previewPager = () => preview().querySelector(".pager")?.textContent;
const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

/** shownWithMenu renders the panel with `body` as the menu reply and waits for both reads. */
async function shownWithMenu(body: unknown = previewMenu, status = 200) {
  const calls = stubApi({
    "GET /api/settings": { status: 200, body: loaded },
    "GET /api/menu": { status, body },
    "PATCH /api/settings": { status: 200, body: loaded },
  });
  await shown();
  await waitFor(() => expect(preview().querySelector("li")).toBeTruthy());
  return calls;
}

describe("SettingsPanel preview", () => {
  it("draws the TV from the loaded settings and the menu", async () => {
    await shownWithMenu();
    expect(within(preview()).getByRole("heading", { level: 1 }).textContent).toBe("Helm Taproom");
    expect(tv().classList.contains("light")).toBe(true);
    expect(tv().tagName).toBe("DIV");
    expect(within(preview()).getByText("Beer 1")).toBeTruthy();
    expect(previewPager()).toBe("1 / 3");
  });

  it("follows the draft at once and sends nothing", async () => {
    const calls = await shownWithMenu();
    type("Title", "Draft Title");
    type("Layout", "columns");
    fireEvent.click(screen.getByLabelText("IBU"));
    expect(within(preview()).getByRole("heading", { level: 1 }).textContent).toBe("Draft Title");
    expect(tv().classList.contains("layout-columns")).toBe(true);
    expect(previewPager()).toBe("1 / 2");
    expect(calls.some((c) => c.key === "PATCH /api/settings")).toBe(false);
  });

  it.each([
    ["an error", {}, 500],
    ["a body that is not a menu", { taps: null }, 200],
  ])("draws the sample menu after %s", async (_, body, status) => {
    await shownWithMenu(body, status);
    expect(within(preview()).getByText(SAMPLE_MENU.taps[0].beer!.name)).toBeTruthy();
  });

  it("steps pages with Previous and Next, wrapping at both ends", async () => {
    await shownWithMenu();
    const next = within(preview().parentElement!).getByRole("button", { name: "Next" });
    const prev = within(preview().parentElement!).getByRole("button", { name: "Previous" });
    fireEvent.click(next);
    expect(previewPager()).toBe("2 / 3");
    fireEvent.click(next);
    fireEvent.click(next);
    expect(previewPager()).toBe("1 / 3");
    fireEvent.click(prev);
    expect(previewPager()).toBe("3 / 3");
    expect(within(preview()).getByText("Porter")).toBeTruthy();
  });

  it("goes back to page 1 when a draft layout has fewer pages, and still shows every item", async () => {
    await shownWithMenu();
    const next = screen.getByRole("button", { name: "Next" });
    fireEvent.click(next);
    fireEvent.click(next);
    expect(previewPager()).toBe("3 / 3");
    type("Layout", "columns");
    expect(previewPager()).toBe("1 / 2");
    const seen = new Set<string>();
    for (let i = 0; i < 2; i++) {
      for (const el of preview().querySelectorAll(".name")) seen.add(el.textContent ?? "");
      fireEvent.click(next);
    }
    expect([...seen].sort()).toEqual([...nineTaps.map((t) => t.beer!.name), "Porter"].sort());
  });

  it("turns pages by itself at the draft interval and keeps the last valid one", async () => {
    await shownWithMenu();
    vi.useFakeTimers();
    type("Page seconds", "30");
    type("Page seconds", "3");
    await tick(29_999);
    expect(previewPager()).toBe("1 / 3");
    await tick(1);
    expect(previewPager()).toBe("2 / 3");
    type("Page seconds", "5");
    await tick(5_000);
    expect(previewPager()).toBe("3 / 3");
  });
});
