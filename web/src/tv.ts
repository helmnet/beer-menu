import { type Dispatch, type SetStateAction, useEffect, useState } from "react";
import type { FontKey, Layout, Menu, MenuDisplay, MenuTap, OnDeckItem, TitleAlign } from "./api";

// 4 × 2 keeps each glass big enough to read as a gauge from 3 m on a 16:9 TV (research R12).
// shortcut: fixed 8 per page, tuned for 16:9; measure if other screens matter.
const CARDS_PER_PAGE = 8;
// 6 rows keep a 1080p row tall enough to read name and details from 3 m (research R7).
// shortcut: fixed 6 rows, tuned for 16:9 at 3 m; tune from the quickstart screenshots.
const ROWS_PER_PAGE = 6;

/**
 * TvDisplay is every TV setting, each checked: the menu's `display` without `page_seconds`, plus
 * `pageMs`, the page interval in milliseconds.
 */
export type TvDisplay = Omit<MenuDisplay, "page_seconds"> & { pageMs: number };

/** DEFAULT_DISPLAY is every TV setting at its default: the TV as it looked before the settings. */
export const DEFAULT_DISPLAY: TvDisplay = {
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

const LAYOUTS: readonly Layout[] = ["grid", "list", "columns"];
const ALIGNS: readonly TitleAlign[] = ["left", "center", "right"];
const FONT_KEYS: readonly FontKey[] = ["system", "inter", "lora", "roboto-slab", "oswald", "nunito", "bebas-neue", "pacifico"];
// The API sends colors lowercase; a color reaches CSS, so nothing wider may pass (research R3).
const COLOR = /^#[0-9a-f]{6}$/;

/** obj narrows a JSON value to an indexable non-null object. */
const obj = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

/** opt reports whether v is null or of JS type t: an object here would throw as a React child. */
const opt = (v: unknown, t: "string" | "number") => v === null || typeof v === t;
/** optOrAbsent is opt for a field the API leaves out when unset. */
const optOrAbsent = (v: unknown, t: "string" | "number") => v === undefined || opt(v, t);
const isBeer = (b: unknown) =>
  obj(b) &&
  typeof b.name === "string" &&
  opt(b.style, "string") &&
  opt(b.color, "string") &&
  opt(b.abv, "number") &&
  opt(b.ibu, "number") &&
  optOrAbsent(b.srm, "number") &&
  optOrAbsent(b.description, "string");
const isKeg = (k: unknown) => k === null || (obj(k) && typeof k.level === "number" && typeof k.shape === "string");
const isTap = (t: unknown) => obj(t) && typeof t.label === "string" && (t.beer === null || isBeer(t.beer)) && isKeg(t.keg);
const isOnDeck = (d: unknown) => obj(d) && isBeer(d.beer) && isKeg(d.keg);

/**
 * isMenu reports whether a reply body has the menu's two arrays and every item is shaped well
 * enough that rendering cannot throw: a beer has a string name, style and color each null or a
 * string, ABV and IBU each null or a number, and SRM and description each absent, null or a
 * number and a string; a keg has a numeric level and a string shape. A tap has a string label,
 * a beer or null and a keg or null; an on-deck item has a beer (never null) and a keg or null.
 * A thrown render would unmount the TV and stop polling, so a bad body must fail here and count
 * as a failed check (FR-021).
 */
export function isMenu(m: unknown): m is Menu {
  return obj(m) && Array.isArray(m.taps) && Array.isArray(m.on_deck) && m.taps.every(isTap) && m.on_deck.every(isOnDeck);
}

/**
 * SAMPLE_MENU is the menu the Display Settings preview draws when the real menu cannot be read:
 * a tap with SRM and a long description, a plain tap, an empty tap, a waiting keg and a beer
 * being made, so every setting shows something. Colors are the API's SRM colors.
 */
export const SAMPLE_MENU: Menu = {
  taps: [
    {
      label: "Tap 1",
      beer: {
        name: "Sample Pale Ale",
        style: "American Pale Ale",
        abv: 5.4,
        ibu: 38,
        color: "#f8a600",
        srm: 6.2,
        description:
          "A bright, crisp pale ale with citrus and pine from late hops, a light malt backbone and a clean, dry finish that keeps you coming back for another sip.",
      },
      keg: { shape: "corny", level: 0.7 },
    },
    {
      label: "Tap 2",
      beer: { name: "Sample Stout", style: "Dry Irish Stout", abv: 4.3, ibu: 42, color: "#3b0607", srm: 38 },
      keg: { shape: "sanke", level: 0.35 },
    },
    { label: "Tap 3", beer: null, keg: null },
  ],
  on_deck: [
    {
      beer: { name: "Sample Lager", style: "Munich Helles", abv: 4.9, ibu: 18, color: "#ffbf42" },
      keg: { shape: "corny", level: 1 },
      status: null,
    },
    { beer: { name: "Sample Saison", style: "Saison", abv: 6.5, ibu: 28, color: "#fbb123" }, keg: null, status: "fermenting" },
  ],
};

/** oneOf returns v when it is one of allowed, else fallback. */
const oneOf = <T extends string>(v: unknown, allowed: readonly T[], fallback: T): T => (allowed.includes(v as T) ? (v as T) : fallback);

/** bool returns v when it is a boolean, else fallback. */
const bool = (v: unknown, fallback: boolean) => (typeof v === "boolean" ? v : fallback);

/** color returns v when it is "" or a lowercase #rrggbb, else fallback. */
const color = (v: unknown, fallback: string) => (v === "" || (typeof v === "string" && COLOR.test(v)) ? v : fallback);

/**
 * displayOf reads the TV settings from a menu's `display` value, checking each field on its own
 * and falling back to `base` (by default DEFAULT_DISPLAY) when it is missing or malformed, so an
 * API that predates a field, or a bad value in it, still draws the menu. The preview passes the
 * last good settings as `base`, so a half-typed field keeps the last good value.
 */
export function displayOf(d: unknown, base: TvDisplay = DEFAULT_DISPLAY): TvDisplay {
  if (!obj(d)) return base;
  const s = d.page_seconds;
  return {
    title: typeof d.title === "string" ? d.title : base.title,
    theme: oneOf(d.theme, ["dark", "light"], base.theme),
    pageMs: typeof s === "number" && Number.isInteger(s) && s >= 5 && s <= 300 ? s * 1000 : base.pageMs,
    layout: oneOf(d.layout, LAYOUTS, base.layout),
    title_align: oneOf(d.title_align, ALIGNS, base.title_align),
    title_font: oneOf(d.title_font, FONT_KEYS, base.title_font),
    text_font: oneOf(d.text_font, FONT_KEYS, base.text_font),
    show_style: bool(d.show_style, base.show_style),
    show_abv: bool(d.show_abv, base.show_abv),
    show_ibu: bool(d.show_ibu, base.show_ibu),
    show_srm: bool(d.show_srm, base.show_srm),
    show_description: bool(d.show_description, base.show_description),
    title_color: color(d.title_color, base.title_color),
    text_color: color(d.text_color, base.text_color),
  };
}

/**
 * Page is one screen of the rotation. In `grid` and `list` it holds taps or on deck items, never
 * both; in `columns` it holds both, either possibly empty.
 */
export type Page = { taps?: MenuTap[]; onDeck?: OnDeckItem[] };

/** chunks splits items into runs of n. */
function chunks<T>(items: T[], n: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += n) out.push(items.slice(i, i + n));
  return out;
}

/**
 * pagesOf splits a menu into the pages of a layout, so every item shows once per rotation.
 * `grid`: taps in pages of 8 cards, then on deck in pages of 8. `list`: the same with 6 rows a
 * page. `columns`: page i holds tap rows chunk i beside on deck rows chunk i, as many pages as the
 * longer column needs; the shorter column is empty on the rest. An empty menu has no pages.
 */
export function pagesOf(menu: Menu, layout: Layout): Page[] {
  if (layout === "columns") {
    const taps = chunks(menu.taps, ROWS_PER_PAGE);
    const onDeck = chunks(menu.on_deck, ROWS_PER_PAGE);
    return Array.from({ length: Math.max(taps.length, onDeck.length) }, (_, i) => ({
      taps: taps[i] ?? [],
      onDeck: onDeck[i] ?? [],
    }));
  }
  const n = layout === "list" ? ROWS_PER_PAGE : CARDS_PER_PAGE;
  return [...chunks(menu.taps, n).map((taps) => ({ taps })), ...chunks(menu.on_deck, n).map((onDeck) => ({ onDeck }))];
}

/**
 * usePager turns pages: with more than one page it moves to the next page every `pageMs`,
 * wrapping past the last back to page 1. Only rotation starting or stopping, or a new interval,
 * restarts the timer, so a new menu keeps the timing and a changed interval brings the next turn
 * one new interval later. When the page is past the last one (a smaller menu or layout) it goes
 * back to page 1. Returns the page index and its setter, for manual paging.
 */
export function usePager(count: number, pageMs: number): [number, Dispatch<SetStateAction<number>>] {
  const [page, setPage] = useState(0);
  const rotating = count > 1;

  useEffect(() => {
    if (!rotating) return;
    let next: ReturnType<typeof setTimeout>;
    const turn = () => {
      setPage((p) => p + 1);
      next = setTimeout(turn, pageMs);
    };
    next = setTimeout(turn, pageMs);
    return () => clearTimeout(next);
  }, [rotating, pageMs]);

  // React re-renders before painting, so the out-of-range page is never drawn.
  if (page > 0 && page >= count) setPage(0);
  return [page >= count ? 0 : page, setPage];
}
