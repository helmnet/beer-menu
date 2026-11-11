import { describe, expect, it } from "vitest";
import { menu } from "./testApi";
import type { Layout, MenuTap, OnDeckItem } from "./api";
import { displayOf, isMenu, pagesOf, SAMPLE_MENU, type TvDisplay } from "./tv";

const defaults: TvDisplay = {
  title: "",
  theme: "dark",
  pageMs: 15_000,
  layout: "grid",
  title_align: "left",
  title_font: "system",
  text_font: "system",
  show_style: true,
  show_abv: true,
  show_ibu: true,
  show_srm: false,
  show_description: false,
  title_color: "",
  text_color: "",
};

describe("displayOf", () => {
  it.each([
    ["no display", undefined],
    ["a null display", null],
    ["a string display", "x"],
  ])("returns every default for %s", (_, d) => {
    expect(displayOf(d)).toEqual(defaults);
  });

  it("reads every field of a good display", () => {
    const d = {
      ...menu().display,
      title: "Helm",
      theme: "light",
      page_seconds: 20,
      layout: "columns",
      title_align: "center",
      title_font: "pacifico",
      text_font: "oswald",
      show_style: false,
      show_abv: false,
      show_ibu: false,
      show_srm: true,
      show_description: true,
      title_color: "#ffaa00",
      text_color: "#102030",
    };
    const { page_seconds: _, ...rest } = d;
    expect(displayOf(d)).toEqual({ ...rest, pageMs: 20_000 });
  });

  it.each([
    ["an unknown layout", { layout: "tiles" }, { layout: "grid" }],
    ["an unknown alignment", { title_align: "middle" }, { title_align: "left" }],
    ["an unknown title font", { title_font: "comic-sans" }, { title_font: "system" }],
    ["an unknown text font", { text_font: 3 }, { text_font: "system" }],
    ['show_abv "no"', { show_abv: "no" }, { show_abv: true }],
    ["show_srm 1", { show_srm: 1 }, { show_srm: false }],
    ['color "red"', { title_color: "red" }, { title_color: "" }],
    ['color "#abc"', { text_color: "#abc" }, { text_color: "" }],
    ["a color with CSS in it", { text_color: "#123456;color:red" }, { text_color: "" }],
  ])("falls back for %s and keeps the other fields", (_, bad, fixed) => {
    const good = { ...menu().display, title: "Helm", layout: "list", title_font: "lora", show_ibu: false, title_color: "#aabbcc" };
    const { page_seconds: _ps, ...rest } = good;
    expect(displayOf({ ...good, ...bad })).toEqual({ ...rest, pageMs: 15_000, ...fixed });
  });

  it("falls back to a given base instead of the defaults", () => {
    const base = { ...defaults, pageMs: 40_000, layout: "list" as const };
    expect(displayOf({ page_seconds: 2, layout: "tiles", title: "X" }, base)).toEqual({ ...base, title: "X" });
  });
});

const tapN = (n: number): MenuTap[] => Array.from({ length: n }, (_, i) => ({ label: `Tap ${i + 1}`, beer: null, keg: null }));
const deckN = (n: number): OnDeckItem[] =>
  Array.from({ length: n }, (_, i) => ({
    beer: { name: `Beer ${i + 1}`, style: null, abv: null, ibu: null, color: null },
    keg: null,
    status: "fermenting",
  }));
/** shape describes pages as "t<taps>/d<on deck>" per page, "-" for a missing list. */
const shape = (layout: Layout, taps: number, onDeck: number) =>
  pagesOf(menu({ taps: tapN(taps), on_deck: deckN(onDeck) }), layout).map((p) => `t${p.taps?.length ?? "-"}/d${p.onDeck?.length ?? "-"}`);

describe("pagesOf", () => {
  it("keeps grid as before: 8 cards a page, taps then on deck", () => {
    expect(shape("grid", 12, 3)).toEqual(["t8/d-", "t4/d-", "t-/d3"]);
  });

  it("gives list tap pages of 6 rows, then on deck pages of 6", () => {
    expect(shape("list", 13, 7)).toEqual(["t6/d-", "t6/d-", "t1/d-", "t-/d6", "t-/d1"]);
  });

  it("gives columns the longer column's page count, tap chunk i beside on deck chunk i", () => {
    expect(shape("columns", 13, 7)).toEqual(["t6/d6", "t6/d1", "t1/d0"]);
    expect(shape("columns", 2, 14)).toEqual(["t2/d6", "t0/d6", "t0/d2"]);
    const pages = pagesOf(menu({ taps: tapN(8), on_deck: deckN(3) }), "columns");
    expect(pages[1].taps?.map((t) => t.label)).toEqual(["Tap 7", "Tap 8"]);
    expect(pages[0].onDeck?.map((d) => d.beer.name)).toEqual(["Beer 1", "Beer 2", "Beer 3"]);
  });

  it("gives an empty menu no pages, but columns with taps and no on deck one page", () => {
    for (const layout of ["grid", "list", "columns"] as const) expect(shape(layout, 0, 0)).toEqual([]);
    expect(shape("list", 3, 0)).toEqual(["t3/d-"]);
    expect(shape("columns", 3, 0)).toEqual(["t3/d0"]);
    expect(shape("columns", 0, 2)).toEqual(["t0/d2"]);
  });

  it.each([
    ["grid", 5],
    ["list", 6],
    ["columns", 4],
  ] as const)("shows 24 taps and 12 on deck items exactly once in %s, on %s pages", (layout, count) => {
    const m = menu({ taps: tapN(24), on_deck: deckN(12) });
    const pages = pagesOf(m, layout);
    expect(pages).toHaveLength(count);
    expect(pages.flatMap((p) => p.taps ?? [])).toEqual(m.taps);
    expect(pages.flatMap((p) => p.onDeck ?? [])).toEqual(m.on_deck);
  });
});

describe("SAMPLE_MENU", () => {
  it("has a tap with SRM and a long description, an empty tap and 2 on deck items, and is a menu", () => {
    expect(isMenu(SAMPLE_MENU)).toBe(true);
    expect(SAMPLE_MENU.taps.length).toBeGreaterThanOrEqual(3);
    expect(SAMPLE_MENU.taps.some((t) => t.beer?.srm !== undefined && (t.beer.description?.length ?? 0) > 100)).toBe(true);
    expect(SAMPLE_MENU.taps.some((t) => t.beer === null && t.keg === null)).toBe(true);
    expect(SAMPLE_MENU.on_deck).toHaveLength(2);
  });
});

describe("isMenu", () => {
  const withBeer = (extra: object) =>
    menu({
      taps: [
        {
          label: "T",
          beer: { name: "B", style: null, abv: null, ibu: null, color: null, ...extra } as never,
          keg: { shape: "corny", level: 1 },
        },
      ],
    });

  it("accepts a tap beer with or without srm and description", () => {
    expect(isMenu(withBeer({}))).toBe(true);
    expect(isMenu(withBeer({ srm: 5, description: "x" }))).toBe(true);
  });

  it.each([
    ["an object description", { description: {} }],
    ["a string srm", { srm: "5" }],
  ])("refuses a tap beer with %s, which would throw as a React child", (_, extra) => {
    expect(isMenu(withBeer(extra))).toBe(false);
  });
});
