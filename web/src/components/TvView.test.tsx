import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Menu, MenuBeer, MenuDisplay, MenuTap, OnDeckItem } from "../api";
import { menu } from "../testApi";
import { displayOf, pagesOf } from "../tv";
import TvView from "./TvView";

afterEach(cleanup);

function menuBeer(overrides: Partial<MenuBeer> = {}): MenuBeer {
  return { name: "Pale Ale", style: "American Pale Ale", abv: 5.2, ibu: 35, color: "#e58500", ...overrides };
}

function fullTap(label: string, overrides: Partial<MenuBeer> = {}, level = 0.6): MenuTap {
  return { label, beer: menuBeer(overrides), keg: { shape: "corny", level } };
}

const emptyTap = (label: string): MenuTap => ({ label, beer: null, keg: null });

const deckKeg = (name: string, shape: "corny" | "sanke" = "corny", level = 0.5): OnDeckItem => ({
  beer: menuBeer({ name }),
  keg: { shape, level },
  status: null,
});

/**
 * view renders TvView for menu m (with `display` changed by `d`) the way DisplayPage does, on
 * page `page` with `failures` failed checks, and returns the `main` element.
 */
function view(m: Menu | null, d: Partial<MenuDisplay> = {}, page = 0, failures = 0) {
  const display = displayOf(m ? { ...m.display, ...d } : undefined);
  const pages = m ? pagesOf(m, display.layout) : [];
  const r = render(<TvView menu={m} display={display} pages={pages} page={page} failures={failures} offlineAfter={12} />);
  return { ...r, main: r.container.querySelector("main") as HTMLElement };
}

describe("TvView grid with default settings", () => {
  it("shows each tap's label, beer, style, ABV and IBU in menu order", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1"), fullTap("Tap 2", { name: "Stout", style: "Dry Stout", abv: 4.2, ibu: 40 })] }));
    const cards = within(main).getAllByRole("listitem");
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByText("Tap 1")).toBeTruthy();
    expect(within(cards[0]).getByText("Pale Ale")).toBeTruthy();
    expect(within(cards[0]).getByText("American Pale Ale")).toBeTruthy();
    expect(within(cards[0]).getByText("5.2%")).toBeTruthy();
    expect(within(cards[0]).getByText("35 IBU")).toBeTruthy();
    expect(within(cards[1]).getByText("Stout")).toBeTruthy();
    expect(within(cards[1]).getByText("4.2%")).toBeTruthy();
  });

  it("draws the glass with the level in its label and no level number on screen", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1")] }));
    const art = within(main).getByRole("img");
    expect(art.getAttribute("data-vessel")).toBe("glass");
    expect(art.getAttribute("aria-label")).toBe("Pale Ale, pint glass, 60% left");
    expect(main.textContent).not.toMatch(/\d+\s*%\s*left|gal/);
  });

  it("shows an empty tap as its label, Empty and an empty glass", () => {
    const { main } = view(menu({ taps: [emptyTap("Tap 3")] }));
    expect(within(main).getByText("Tap 3")).toBeTruthy();
    expect(within(main).getByText("Empty")).toBeTruthy();
    expect(within(main).getByRole("img").querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("none");
  });

  it("leaves out a missing style, ABV and IBU", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1", { style: null, abv: null, ibu: null })] }));
    const card = within(main).getByRole("listitem");
    expect(card.textContent).toBe("Tap 1Pale Ale");
    for (const el of Array.from(card.querySelectorAll("p, span"))) expect(el.textContent).not.toBe("");
  });

  it("shows Nothing on tap with no taps and nothing on deck, and nothing before a menu", () => {
    view(menu());
    expect(screen.getByText("Nothing on tap")).toBeTruthy();
    cleanup();
    expect(view(null).main.textContent).toBe("");
  });

  it.each([
    [1, "taps-s"],
    [4, "taps-s"],
    [5, "taps-m"],
    [8, "taps-m"],
  ])("sizes a page of %s cards with %s", (n, size) => {
    const { main } = view(menu({ taps: Array.from({ length: n }, (_, i) => fullTap(`Tap ${i + 1}`)) }));
    expect(main.classList.contains("display")).toBe(true);
    expect(main.classList.contains(size)).toBe(true);
  });

  it("draws on deck kegs and beers being made under an On deck heading", () => {
    view(
      menu({
        on_deck: [deckKeg("Porter"), { beer: menuBeer({ name: "Saison" }), keg: null, status: "conditioning" }],
      }),
    );
    expect(screen.getByRole("heading", { name: "On deck" })).toBeTruthy();
    const deck = screen.getByRole("list", { name: "On deck" });
    expect(
      within(deck)
        .getAllByRole("img")
        .map((a) => a.getAttribute("data-keg-shape")),
    ).toEqual(["corny", "corny"]);
    expect(within(deck).getByText("Porter")).toBeTruthy();
    expect(within(deck).getByText("Conditioning")).toBeTruthy();
    expect(within(deck).getAllByText("5.2%")).toHaveLength(2);
    expect(within(deck).queryByText("35 IBU")).toBeNull();
  });

  it("shows the given page and the pager only with more than one page", () => {
    const m = menu({ taps: Array.from({ length: 9 }, (_, i) => fullTap(`Tap ${i + 1}`)), on_deck: [deckKeg("Porter")] });
    view(m, {}, 1);
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
    expect(screen.getByText("Tap 9")).toBeTruthy();
    expect(document.querySelector(".pager")?.textContent).toBe("2 / 3");
    cleanup();
    view(menu({ taps: [fullTap("Tap 1")] }));
    expect(document.querySelector(".pager")).toBeNull();
  });

  it("shows the offline notice from offlineAfter failures", () => {
    view(menu({ taps: [fullTap("Tap 1")] }), {}, 0, 11);
    expect(screen.queryByRole("status")).toBeNull();
    cleanup();
    view(menu({ taps: [fullTap("Tap 1")] }), {}, 0, 12);
    expect(screen.getByRole("status").textContent).toBe("offline");
  });

  it("shows the title as one heading, as text, and the light theme as a class", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1")] }), { title: "<b>Helm</b>", theme: "light" });
    const title = screen.getByRole("heading", { level: 1 });
    expect(title.textContent).toBe("<b>Helm</b>");
    expect(title.className).toBe("title");
    expect(main.querySelector("b")).toBeNull();
    expect(main.classList.contains("light")).toBe(true);
  });

  it("uses no style attribute with default settings", () => {
    const { container } = view(menu({ taps: [fullTap("Tap 1"), emptyTap("Tap 2")], on_deck: [deckKeg("Porter")] }));
    expect(container.querySelectorAll("[style]")).toHaveLength(0);
  });
});

describe("TvView tap details", () => {
  const rich = (overrides: Partial<MenuBeer> = {}) => fullTap("Tap 1", { srm: 5.6, description: "Crisp and bright.", ...overrides });
  const card = () => screen.getAllByRole("listitem")[0];

  it.each([
    ["show_style", "American Pale Ale"],
    ["show_abv", "5.2%"],
    ["show_ibu", "35 IBU"],
  ] as const)("hides the detail when %s is off", (key, text) => {
    view(menu({ taps: [rich()] }), { [key]: false });
    expect(within(card()).queryByText(text)).toBeNull();
    expect(within(card()).getByText("Pale Ale")).toBeTruthy();
    cleanup();
    view(menu({ taps: [rich()] }));
    expect(within(card()).getByText(text)).toBeTruthy();
  });

  it("shows SRM rounded only when on and known", () => {
    view(menu({ taps: [rich()] }));
    expect(within(card()).queryByText(/SRM/)).toBeNull();
    cleanup();
    view(menu({ taps: [rich(), fullTap("Tap 2")] }), { show_srm: true });
    expect(within(card()).getByText("SRM 6")).toBeTruthy();
    expect(screen.getAllByRole("listitem")[1].textContent).not.toMatch(/SRM/);
  });

  it("shows the description in a description element only when on and known", () => {
    view(menu({ taps: [rich()] }));
    expect(document.querySelector(".description")).toBeNull();
    cleanup();
    view(menu({ taps: [rich(), fullTap("Tap 2")] }), { show_description: true });
    expect(card().querySelector(".description")?.textContent).toBe("Crisp and bright.");
    expect(screen.getAllByRole("listitem")[1].querySelector(".description")).toBeNull();
  });

  it("treats a null SRM or description as missing", () => {
    view(menu({ taps: [rich({ srm: null as never, description: null as never })] }), { show_srm: true, show_description: true });
    expect(card().textContent).not.toMatch(/SRM|null/);
    expect(card().querySelector(".description")).toBeNull();
  });

  it("draws description markup as text", () => {
    view(menu({ taps: [rich({ description: "<b>bold</b>" })] }), { show_description: true });
    expect(card().querySelector(".description")?.textContent).toBe("<b>bold</b>");
    expect(card().querySelector("b")).toBeNull();
  });

  it("leaves only label, name and art with every switch off", () => {
    view(menu({ taps: [rich()] }), { show_style: false, show_abv: false, show_ibu: false, show_srm: false, show_description: false });
    expect(card().textContent).toBe("Tap 1Pale Ale");
    expect(within(card()).getByRole("img")).toBeTruthy();
  });

  it("shows Empty for an empty tap whatever the switches", () => {
    view(menu({ taps: [emptyTap("Tap 2")] }), { show_srm: true, show_description: true, show_style: false });
    expect(card().textContent).toBe("Tap 2Empty");
  });

  it("keeps on deck details whatever the switches", () => {
    const beer = menuBeer({ name: "Saison", srm: 4, description: "Farmhouse." });
    const all = { show_style: false, show_abv: false, show_ibu: true, show_srm: true, show_description: true };
    view(menu({ on_deck: [{ beer, keg: null, status: "fermenting" }] }), all);
    expect(card().textContent).toBe("SaisonAmerican Pale Ale5.2%Fermenting");
  });
});

describe("TvView list and columns", () => {
  const rows = (list: HTMLElement) => within(list).getAllByRole("listitem");

  it("draws list rows of art, label, name and details in tap order, then on deck rows", () => {
    const m = menu({
      taps: [fullTap("Tap 1"), fullTap("Tap 2", { name: "Stout", ibu: null }), emptyTap("Tap 3")],
      on_deck: [deckKeg("Porter")],
    });
    const { main } = view(m, { layout: "list" });
    expect(main.classList.contains("layout-list")).toBe(true);
    expect(main.classList.contains("taps-s")).toBe(false);
    const taps = rows(screen.getByRole("list", { name: "Taps" }));
    expect(taps.map((r) => r.className)).toEqual(["row", "row", "row"]);
    expect(taps[0].firstElementChild?.getAttribute("data-vessel")).toBe("glass");
    expect(taps.map((r) => r.textContent)).toEqual([
      "Tap 1Pale AleAmerican Pale Ale5.2%35 IBU",
      "Tap 2StoutAmerican Pale Ale5.2%",
      "Tap 3Empty",
    ]);
    expect(screen.queryByRole("heading", { name: "On deck" })).toBeNull();
    cleanup();
    view(m, { layout: "list" }, 1);
    expect(screen.getByRole("heading", { name: "On deck" })).toBeTruthy();
    const deck = rows(screen.getByRole("list", { name: "On deck" }));
    expect(deck[0].firstElementChild?.getAttribute("data-vessel")).toBe("keg");
    expect(deck[0].textContent).toBe("PorterAmerican Pale Ale5.2%");
  });

  it("honors the detail switches in rows", () => {
    view(menu({ taps: [fullTap("Tap 1", { srm: 9.4, description: "Toasty." })] }), {
      layout: "list",
      show_style: false,
      show_ibu: false,
      show_srm: true,
      show_description: true,
    });
    const row = screen.getAllByRole("listitem")[0];
    expect(row.textContent).toBe("Tap 1Pale Ale5.2%SRM 9Toasty.");
    expect(row.querySelector(".description")?.textContent).toBe("Toasty.");
  });

  it("draws columns with taps beside on deck on one page", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1"), fullTap("Tap 2")], on_deck: [deckKeg("Porter")] }), {
      layout: "columns",
    });
    expect(main.classList.contains("layout-columns")).toBe(true);
    expect(screen.getByRole("heading", { name: "On tap" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "On deck" })).toBeTruthy();
    expect(rows(screen.getByRole("list", { name: "Taps" }))).toHaveLength(2);
    expect(rows(screen.getByRole("list", { name: "On deck" }))).toHaveLength(1);
    expect(document.querySelector(".pager")).toBeNull();
  });

  it("says Nothing on deck in the on deck column when on deck is empty", () => {
    view(menu({ taps: [fullTap("Tap 1")] }), { layout: "columns" });
    expect(screen.getByText("Nothing on deck")).toBeTruthy();
    expect(screen.queryByRole("list", { name: "On deck" })).toBeNull();
    expect(rows(screen.getByRole("list", { name: "Taps" }))).toHaveLength(1);
  });

  it("leaves blank space, with no placeholder rows, after the shorter column's last item", () => {
    const taps = Array.from({ length: 8 }, (_, i) => fullTap(`Tap ${i + 1}`));
    view(menu({ taps, on_deck: [deckKeg("Porter")] }), { layout: "columns" }, 1);
    expect(rows(screen.getByRole("list", { name: "Taps" })).map((r) => r.textContent?.slice(0, 5))).toEqual(["Tap 7", "Tap 8"]);
    expect(screen.queryByRole("list", { name: "On deck" })).toBeNull();
    expect(screen.queryByText("Nothing on deck")).toBeNull();
    expect(document.querySelector(".pager")?.textContent).toBe("2 / 2");
  });

  it("says Nothing on tap with an empty menu in every layout", () => {
    for (const layout of ["list", "columns"] as const) {
      view(menu(), { layout });
      expect(screen.getByText("Nothing on tap")).toBeTruthy();
      cleanup();
    }
  });
});

describe("TvView title and fonts", () => {
  const title = () => screen.queryByRole("heading", { level: 1 });

  it.each([
    ["left", "title"],
    ["center", "title align-center"],
    ["right", "title align-right"],
  ] as const)("aligns the title %s", (title_align, className) => {
    view(menu(), { title: "Helm", title_align });
    expect(title()?.className).toBe(className);
  });

  it("puts the title font on the title only", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1")] }), { title: "Helm", title_font: "pacifico" });
    expect(title()?.classList.contains("font-pacifico")).toBe(true);
    expect(main.classList.contains("font-pacifico")).toBe(false);
    expect(main.classList.contains("font-system")).toBe(true);
  });

  it("puts the text font on the page and keeps the title in its own font", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1")] }), { title: "Helm", text_font: "oswald" });
    expect(main.classList.contains("font-oswald")).toBe(true);
    expect(title()?.classList.contains("font-system")).toBe(true);
    expect(title()?.classList.contains("font-oswald")).toBe(false);
  });

  it("draws no title row for an empty title whatever the alignment and font", () => {
    view(menu({ taps: [fullTap("Tap 1")] }), { title: "", title_align: "center", title_font: "lora" });
    expect(title()).toBeNull();
    expect(document.querySelector(".title")).toBeNull();
  });
});

describe("TvView colors", () => {
  it("sets the title and text colors as custom properties on the root only", () => {
    const { main, container } = view(menu({ taps: [fullTap("Tap 1")] }), { title: "Helm", title_color: "#ffaa00", text_color: "#102030" });
    expect(main.style.getPropertyValue("--title-color")).toBe("#ffaa00");
    expect(main.style.getPropertyValue("--tv-text")).toBe("#102030");
    expect([...container.querySelectorAll("[style]")]).toEqual([main]);
  });

  it("sets only the color that is chosen", () => {
    const { main } = view(menu(), { text_color: "#102030" });
    expect(main.style.getPropertyValue("--title-color")).toBe("");
    expect(main.style.getPropertyValue("--tv-text")).toBe("#102030");
  });

  it("sets neither for theme colors, so there is no style attribute", () => {
    const { main } = view(menu({ taps: [fullTap("Tap 1")] }), { title: "Helm" });
    expect(main.hasAttribute("style")).toBe(false);
  });
});
