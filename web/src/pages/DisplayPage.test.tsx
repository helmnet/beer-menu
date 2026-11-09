import { act, cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Menu, MenuBeer, MenuTap, OnDeckItem } from "../api";
import { menu, stubApi } from "../testApi";
import AdminPage from "./AdminPage";
import DisplayPage from "./DisplayPage";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function menuBeer(overrides: Partial<MenuBeer> = {}): MenuBeer {
  return { name: "Pale Ale", style: "American Pale Ale", abv: 5.2, ibu: 35, color: "#e58500", ...overrides };
}

function fullTap(label: string, overrides: Partial<MenuBeer> = {}, level = 0.6): MenuTap {
  return { label, beer: menuBeer(overrides), keg: { shape: "corny", level } };
}

const emptyTap = (label: string): MenuTap => ({ label, beer: null, keg: null });

/** show renders the page against a stubbed menu and waits for the first check to land. */
async function show(body: Menu) {
  const calls = stubApi({ "GET /api/menu": { status: 200, body } });
  const view = render(<DisplayPage reload={vi.fn()} />);
  await screen.findByRole("main");
  await vi.waitFor(() => expect(calls).toHaveLength(1));
  await act(async () => {});
  return { ...view, calls, main: view.container.querySelector("main") as HTMLElement };
}

describe("DisplayPage taps", () => {
  it("shows each tap's label, beer, style, ABV and IBU in menu order", async () => {
    const { main } = await show(
      menu({ taps: [fullTap("Tap 1"), fullTap("Tap 2", { name: "Stout", style: "Dry Stout", abv: 4.2, ibu: 40 })] }),
    );
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

  it("draws the glass with the level in its label and no level number on screen", async () => {
    const { main } = await show(menu({ taps: [fullTap("Tap 1")] }));
    const art = within(main).getByRole("img");
    expect(art.getAttribute("data-vessel")).toBe("glass");
    expect(art.getAttribute("aria-label")).toBe("Pale Ale, pint glass, 60% left");
    expect(main.textContent).not.toMatch(/\d+\s*%\s*left|gal/);
  });

  it("shows an empty tap as its label, Empty and an empty glass", async () => {
    const { main } = await show(menu({ taps: [emptyTap("Tap 3")] }));
    expect(within(main).getByText("Tap 3")).toBeTruthy();
    expect(within(main).getByText("Empty")).toBeTruthy();
    const art = within(main).getByRole("img");
    expect(art.getAttribute("data-vessel")).toBe("glass");
    expect(art.querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("none");
  });

  it("leaves out a missing style, ABV and IBU", async () => {
    const { main } = await show(menu({ taps: [fullTap("Tap 1", { style: null, abv: null, ibu: null })] }));
    const card = within(main).getByRole("listitem");
    expect(card.textContent).toBe("Tap 1Pale Ale");
    expect(card.textContent).not.toMatch(/null|0|IBU|%/);
    for (const el of Array.from(card.querySelectorAll("p, span"))) expect(el.textContent).not.toBe("");
  });

  it("shows Nothing on tap with no taps and nothing on deck", async () => {
    await show(menu());
    expect(screen.getByText("Nothing on tap")).toBeTruthy();
  });

  it.each([
    [1, "taps-s"],
    [4, "taps-s"],
    [5, "taps-m"],
    [8, "taps-m"],
  ])("sizes a page of %s cards with %s", async (n, size) => {
    const { main } = await show(menu({ taps: Array.from({ length: n }, (_, i) => fullTap(`Tap ${i + 1}`)) }));
    expect(main.classList.contains("display")).toBe(true);
    expect(main.classList.contains(size)).toBe(true);
  });

  it("uses no style attribute and sends no credential header", async () => {
    const { container } = await show(
      menu({
        taps: [fullTap("Tap 1"), emptyTap("Tap 2")],
        on_deck: [{ beer: menuBeer({ name: "Stout" }), keg: { shape: "sanke", level: 1 }, status: null }],
      }),
    );
    expect(container.querySelectorAll("[style]")).toHaveLength(0);
    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("GET");
    expect(Object.keys((init.headers ?? {}) as Record<string, string>)).toEqual([]);
  });

  it("leaves the body untouched for the admin page", async () => {
    stubApi({ "GET /api/setup": { status: 200, body: { needed: true } } });
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Create admin" });
    expect(document.body.className).toBe("");
    expect(document.body.hasAttribute("style")).toBe(false);
    expect(document.querySelector(".display")).toBeNull();
  });
});

const deckKeg = (name: string, shape: "corny" | "sanke" = "corny", level = 0.5): OnDeckItem => ({
  beer: menuBeer({ name }),
  keg: { shape, level },
  status: null,
});

describe("DisplayPage on deck", () => {
  it("draws waiting kegs in their shape with color, name, style, ABV and level", async () => {
    await show(menu({ on_deck: [deckKeg("Porter"), deckKeg("Helles", "sanke")] }));
    const deck = screen.getByRole("list", { name: "On deck" });
    const arts = within(deck).getAllByRole("img");
    expect(arts.map((a) => a.getAttribute("data-vessel"))).toEqual(["keg", "keg"]);
    expect(arts.map((a) => a.getAttribute("data-keg-shape"))).toEqual(["corny", "sanke"]);
    expect(arts[0].querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("#e58500");
    expect(arts[0].getAttribute("aria-label")).toBe("Porter, keg, 50% left");
    expect(within(deck).getByText("Porter")).toBeTruthy();
    expect(within(deck).getAllByText("American Pale Ale")).toHaveLength(2);
    expect(within(deck).getAllByText("5.2%")).toHaveLength(2);
    expect(deck.textContent).not.toMatch(/\d+\s*%\s*left|gal/);
  });

  it("draws beers still being made as a full corny keg with their status", async () => {
    await show(
      menu({
        on_deck: [
          { beer: menuBeer({ name: "Saison", color: "#c94c0c" }), keg: null, status: "conditioning" },
          { beer: menuBeer({ name: "Kölsch" }), keg: null, status: "fermenting" },
        ],
      }),
    );
    const deck = screen.getByRole("list", { name: "On deck" });
    const arts = within(deck).getAllByRole("img");
    expect(arts.map((a) => a.getAttribute("data-keg-shape"))).toEqual(["corny", "corny"]);
    expect(arts[0].getAttribute("aria-label")).toBe("Saison, keg");
    expect(arts[0].querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("#c94c0c");
    expect(within(deck).getByText("Conditioning")).toBeTruthy();
    expect(within(deck).getByText("Fermenting")).toBeTruthy();
    expect(deck.textContent?.indexOf("Saison")).toBeLessThan(deck.textContent?.indexOf("Kölsch") ?? -1);
  });

  it("has no On deck page when nothing is on deck", async () => {
    await show(menu({ taps: [fullTap("Tap 1")] }));
    expect(screen.queryByRole("list", { name: "On deck" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "On deck" })).toBeNull();
  });

  it("does not say Nothing on tap when only on deck has items", async () => {
    await show(menu({ on_deck: [deckKeg("Porter")] }));
    expect(screen.queryByText("Nothing on tap")).toBeNull();
    expect(screen.getByRole("heading", { name: "On deck" })).toBeTruthy();
    expect(screen.getByRole("list", { name: "On deck" })).toBeTruthy();
  });
});

/** FakeReply makes one fetch result from the request's abort signal. */
type FakeReply = (signal: AbortSignal) => Promise<unknown>;

const good =
  (body: unknown): FakeReply =>
  async () => ({ ok: true, status: 200, headers: new Headers(), json: async () => body });
const status500: FakeReply = async () => ({
  ok: false,
  status: 500,
  headers: new Headers(),
  json: async () => ({ error: "internal", message: "boom" }),
});
const networkDown: FakeReply = async () => {
  throw new TypeError("Failed to fetch");
};
// hang never answers; it rejects only when the page aborts the request.
const hang: FakeReply = (signal) =>
  new Promise((_, reject) => signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError"))));

/**
 * poll stubs fetch with replies that change per call: `next` gives each call's reply and may be
 * reassigned. Fake timers drive the page; the fetch stub returns plain objects so no real I/O
 * is needed between ticks.
 */
function poll(first: FakeReply) {
  vi.useFakeTimers();
  const state = { next: first, calls: 0 };
  vi.stubGlobal(
    "fetch",
    vi.fn((_path: string, init: RequestInit) => {
      state.calls++;
      return state.next(init.signal as AbortSignal);
    }),
  );
  return state;
}

const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const one = (name: string) => menu({ taps: [fullTap("Tap 1", { name })] });

describe("DisplayPage polling", () => {
  it("replaces the menu with the next one every 10 s", async () => {
    const s = poll(good(one("Pale Ale")));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(screen.getByText("Pale Ale")).toBeTruthy();
    s.next = good(one("Stout"));
    await tick(9_999);
    expect(s.calls).toBe(1);
    await tick(1);
    expect(s.calls).toBe(2);
    expect(screen.getByText("Stout")).toBeTruthy();
    expect(screen.queryByText("Pale Ale")).toBeNull();
  });

  it("waits for a slow reply before scheduling the next check", async () => {
    const slow: FakeReply = () => new Promise((resolve) => setTimeout(() => resolve(good(one("Slow"))(new AbortController().signal)), 5_000));
    const s = poll(slow);
    render(<DisplayPage reload={vi.fn()} />);
    await tick(10_000);
    expect(s.calls).toBe(1);
    expect(screen.getByText("Slow")).toBeTruthy();
    await tick(4_999);
    expect(s.calls).toBe(1);
    await tick(1);
    expect(s.calls).toBe(2);
  });

  it.each([
    ["a network error", networkDown],
    ["a 500", status500],
    ["a body that is not a menu", good({ taps: [] })],
    ["a body with no arrays", good({ taps: null, on_deck: "x" })],
    ["a null tap", good({ taps: [null], on_deck: [] })],
    ["a tap whose beer is a string", good({ taps: [{ label: "T", beer: "x", keg: null }], on_deck: [] })],
    ["an on-deck item with no beer", good({ taps: [], on_deck: [{ beer: null, keg: null, status: null }] })],
    // Objects as React children throw on render, which would unmount the page and stop polling.
    ["a tap beer whose style is an object", good(menu({ taps: [fullTap("T", { style: {} as never })] }))],
    ["a tap beer whose ABV is an object", good(menu({ taps: [fullTap("T", { abv: {} as never })] }))],
    ["a tap beer whose IBU is an array of objects", good(menu({ taps: [fullTap("T", { ibu: [{}] as never })] }))],
    ["a tap beer whose color is an object", good(menu({ taps: [fullTap("T", { color: {} as never })] }))],
    ["an on-deck beer whose ABV is an object", good(menu({ on_deck: [{ beer: menuBeer({ abv: {} as never }), keg: null, status: null }] }))],
  ])("keeps the last menu after %s and retries 10 s later", async (_, bad) => {
    const s = poll(good(one("Pale Ale")));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    s.next = bad;
    await tick(10_000);
    expect(s.calls).toBe(2);
    expect(screen.getByText("Pale Ale")).toBeTruthy();
    // 11 more failures make 12 in a row: the bad body counted as one.
    for (let i = 0; i < 11; i++) await tick(10_000);
    expect(screen.getByRole("status").textContent).toBe("offline");
    expect(s.calls).toBe(13);
    s.next = good(one("Stout"));
    await tick(10_000);
    expect(s.calls).toBe(14);
    expect(screen.getByText("Stout")).toBeTruthy();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("aborts a request still pending after 8 s and counts it as a failure", async () => {
    const s = poll(good(one("Pale Ale")));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    let aborted = false;
    s.next = (signal) => {
      signal.addEventListener("abort", () => (aborted = true));
      return hang(signal);
    };
    await tick(10_000);
    await tick(7_999);
    expect(aborted).toBe(false);
    await tick(1);
    expect(aborted).toBe(true);
    expect(screen.getByText("Pale Ale")).toBeTruthy();
    s.next = good(one("Stout"));
    await tick(10_000);
    expect(s.calls).toBe(3);
    expect(screen.getByText("Stout")).toBeTruthy();
  });

  it("shows no menu while the server is down, then the first good reply", async () => {
    const s = poll(networkDown);
    const { container } = render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(container.querySelector("main")?.textContent).toBe("");
    s.next = good(one("Pale Ale"));
    await tick(10_000);
    expect(screen.getByText("Pale Ale")).toBeTruthy();
  });

  it("shows offline after 12 failed checks in a row and clears it on a good one", async () => {
    const s = poll(good(one("Pale Ale")));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    s.next = status500;
    for (let i = 1; i <= 11; i++) await tick(10_000);
    expect(s.calls).toBe(12);
    expect(screen.queryByRole("status")).toBeNull();
    await tick(10_000);
    expect(screen.getByRole("status").textContent).toBe("offline");
    expect(screen.getByText("Pale Ale")).toBeTruthy();
    s.next = good(one("Pale Ale"));
    await tick(10_000);
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("reloads only on the first good check after 8,640 ticks", { timeout: 120_000 }, async () => {
    const s = poll(good(one("Pale Ale")));
    const reload = vi.fn();
    render(<DisplayPage reload={reload} />);
    // 8,639 checks (the first plus 8,638 more), all good: too soon.
    await tick(8_638 * 10_000);
    expect(s.calls).toBe(8_639);
    expect(reload).not.toHaveBeenCalled();
    s.next = networkDown;
    await tick(10_000);
    expect(s.calls).toBe(8_640);
    expect(reload).not.toHaveBeenCalled();
    s.next = good(one("Pale Ale"));
    await tick(10_000);
    expect(reload).toHaveBeenCalledTimes(1);
    await tick(10 * 10_000);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("stops polling on unmount", async () => {
    const s = poll(good(one("Pale Ale")));
    const { unmount } = render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    unmount();
    await tick(60_000);
    expect(s.calls).toBe(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("aborts a pending request and clears its timeout on unmount", async () => {
    let aborted = false;
    const s = poll((signal) => {
      signal.addEventListener("abort", () => (aborted = true));
      return hang(signal);
    });
    const { unmount } = render(<DisplayPage reload={vi.fn()} />);
    await tick(1_000);
    unmount();
    await tick(0);
    expect(aborted).toBe(true);
    await tick(60_000);
    expect(s.calls).toBe(1);
    expect(vi.getTimerCount()).toBe(0);
  });
});

const taps = (n: number, name = "Pale Ale") => Array.from({ length: n }, (_, i) => fullTap(`Tap ${i + 1}`, { name }));
const deck = (n: number) => Array.from({ length: n }, (_, i) => deckKeg(`Beer ${i + 1}`));
/** cards returns the text of each card's first line: a tap's label or an on-deck beer's name. */
const cards = () => screen.queryAllByRole("listitem").map((li) => li.querySelector("p")?.textContent);
const pager = () => document.querySelector(".pager")?.textContent;
const range = (from: number, to: number, prefix: string) =>
  Array.from({ length: to - from + 1 }, (_, i) => `${prefix} ${from + i}`);

describe("DisplayPage rotation", () => {
  it("shows 12 taps and 3 on-deck items as 3 pages of 15 s each, in order, then starts over", async () => {
    poll(good(menu({ taps: taps(12), on_deck: deck(3) })));
    const { container } = render(<DisplayPage reload={vi.fn()} />);
    const main = () => container.querySelector("main") as HTMLElement;
    await tick(0);
    expect(cards()).toEqual(range(1, 8, "Tap"));
    expect(pager()).toBe("1 / 3");
    expect(main().classList.contains("taps-m")).toBe(true);
    expect(screen.queryByRole("heading", { name: "On deck" })).toBeNull();
    await tick(14_999);
    expect(pager()).toBe("1 / 3");
    await tick(1);
    expect(cards()).toEqual(range(9, 12, "Tap"));
    expect(pager()).toBe("2 / 3");
    expect(main().classList.contains("taps-s")).toBe(true);
    await tick(15_000);
    expect(screen.getByRole("heading", { name: "On deck" })).toBeTruthy();
    const list = screen.getByRole("list", { name: "On deck" });
    expect(within(list).getAllByRole("img").map((a) => a.getAttribute("data-vessel"))).toEqual(["keg", "keg", "keg"]);
    expect(cards()).toEqual(range(1, 3, "Beer"));
    expect(pager()).toBe("3 / 3");
    await tick(15_000);
    expect(cards()).toEqual(range(1, 8, "Tap"));
    expect(pager()).toBe("1 / 3");
  });

  it("has one page, no indicator and no rotation timer with 3 taps and nothing on deck", async () => {
    poll(good(menu({ taps: taps(3) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(pager()).toBeUndefined();
    // Only the next poll is pending.
    expect(vi.getTimerCount()).toBe(1);
    await tick(15_000);
    expect(cards()).toEqual(range(1, 3, "Tap"));
    expect(vi.getTimerCount()).toBe(1);
  });

  it("alternates the tap page and the On deck page with 3 taps and 2 on-deck items", async () => {
    poll(good(menu({ taps: taps(3), on_deck: deck(2) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(cards()).toEqual(range(1, 3, "Tap"));
    expect(pager()).toBe("1 / 2");
    await tick(15_000);
    expect(cards()).toEqual(range(1, 2, "Beer"));
    expect(pager()).toBe("2 / 2");
    await tick(15_000);
    expect(cards()).toEqual(range(1, 3, "Tap"));
  });

  it("puts 20 on-deck items on 3 pages of 8, 8 and 4 after the tap page, with no +N more", async () => {
    poll(good(menu({ taps: taps(2), on_deck: deck(20) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(cards()).toEqual(range(1, 2, "Tap"));
    expect(pager()).toBe("1 / 4");
    await tick(15_000);
    expect(cards()).toEqual(range(1, 8, "Beer"));
    await tick(15_000);
    expect(cards()).toEqual(range(9, 16, "Beer"));
    await tick(15_000);
    expect(cards()).toEqual(range(17, 20, "Beer"));
    expect(pager()).toBe("4 / 4");
    expect(document.body.textContent).not.toMatch(/more/);
  });

  it("moves to a page that exists when an update leaves fewer pages", async () => {
    const s = poll(good(menu({ taps: taps(12), on_deck: deck(3) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    await tick(30_000);
    expect(pager()).toBe("3 / 3");
    s.next = good(menu({ taps: taps(3) }));
    await tick(10_000);
    expect(cards()).toEqual(range(1, 3, "Tap"));
    expect(pager()).toBeUndefined();
    // Back to 3 pages: the rotation starts at page 1, not where it was before.
    s.next = good(menu({ taps: taps(12), on_deck: deck(3) }));
    await tick(10_000);
    expect(pager()).toBe("1 / 3");
    await tick(15_000);
    expect(pager()).toBe("2 / 3");
  });

  it("stays on the current page when an update adds a page", async () => {
    const s = poll(good(menu({ taps: taps(12) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    // Second time round, on page 2 of 2. One tick per turn: act renders only when it ends.
    for (let i = 0; i < 3; i++) await tick(15_000);
    expect(pager()).toBe("2 / 2");
    s.next = good(menu({ taps: taps(12), on_deck: deck(1) }));
    await tick(5_000);
    expect(pager()).toBe("2 / 3");
    expect(cards()).toEqual(range(9, 12, "Tap"));
    await tick(10_000);
    expect(pager()).toBe("3 / 3");
  });

  it("keeps the current page across a menu update", async () => {
    const s = poll(good(menu({ taps: taps(12) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    await tick(15_000);
    expect(pager()).toBe("2 / 2");
    s.next = good(menu({ taps: taps(12, "Stout") }));
    // The check at 20 s lands mid-page.
    await tick(5_000);
    expect(screen.getAllByText("Stout")).toHaveLength(4);
    expect(pager()).toBe("2 / 2");
    // The page still turns at 30 s, not 15 s after the update.
    await tick(9_999);
    expect(pager()).toBe("2 / 2");
    await tick(1);
    expect(pager()).toBe("1 / 2");
  });

  it("keeps rotating the last menu while checks fail", async () => {
    const s = poll(good(menu({ taps: taps(12) })));
    render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    s.next = networkDown;
    await tick(15_000);
    expect(cards()).toEqual(range(9, 12, "Tap"));
    await tick(15_000);
    expect(cards()).toEqual(range(1, 8, "Tap"));
    expect(s.calls).toBe(4);
  });

  it("clears the rotation timer on unmount", async () => {
    const s = poll(good(menu({ taps: taps(12) })));
    const { unmount } = render(<DisplayPage reload={vi.fn()} />);
    await tick(0);
    expect(vi.getTimerCount()).toBe(2);
    unmount();
    await tick(60_000);
    expect(s.calls).toBe(1);
    expect(vi.getTimerCount()).toBe(0);
  });
});
