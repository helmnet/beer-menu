import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { formatGallons, gallonsToLitres } from "../units";
import { beer, keg, kegTypes, Reply, stubApi, tap } from "../testApi";
import TapsPanel from "./TapsPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const one = tap({ id: 1, position: 1, label: "Left" });
const two = tap({ id: 2, position: 2, label: "Middle", enabled: false });
const three = tap({ id: 3, position: 3, label: "Right" });

/** routes returns the stubs the panel loads on mount, with the given taps, plus extra. */
function routes(taps: unknown[], extra: Record<string, Reply> = {}): Record<string, Reply> {
  return {
    "GET /api/taps": { status: 200, body: taps },
    "GET /api/kegs?state=waiting": { status: 200, body: [] },
    "GET /api/keg-types": { status: 200, body: kegTypes },
    ...extra,
  };
}

/** item returns the list item of the tap labelled label. */
function item(label: string) {
  return within(screen.getByRole("list", { name: "Taps" })).getByRole("listitem", { name: label });
}

function keys(calls: { key: string }[]) {
  return calls.map((c) => c.key);
}

describe("TapsPanel taps", () => {
  it("lists taps in position order with labels and a disabled marker", async () => {
    stubApi(routes([one, two, three]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    const items = within(screen.getByRole("list", { name: "Taps" })).getAllByRole("listitem");
    expect(items.map((li) => li.getAttribute("aria-label"))).toEqual(["Left", "Middle", "Right"]);
    expect(within(item("Left")).getByRole("heading").textContent).toBe("1. Left");
    expect(within(item("Middle")).getByRole("heading").textContent).toBe("2. Middle (disabled)");
  });

  it("adds a tap and reloads", async () => {
    const r = routes([one], { "POST /api/taps": { status: 201, body: tap({ id: 4, position: 2, label: "Tap 2" }) } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    r["GET /api/taps"] = { status: 200, body: [one, tap({ id: 4, position: 2, label: "Tap 2" })] };
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    expect(await screen.findByRole("listitem", { name: "Tap 2" })).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/taps")?.body).toEqual({});
  });

  it("renames a tap", async () => {
    const r = routes([one], { "PATCH /api/taps/1": { status: 200, body: { ...one, label: "Pils" } } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    r["GET /api/taps"] = { status: 200, body: [{ ...one, label: "Pils" }] };
    fireEvent.change(within(item("Left")).getByLabelText("Label"), { target: { value: "Pils" } });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Rename" }));
    expect(await screen.findByRole("listitem", { name: "Pils" })).toBeTruthy();
    expect(calls.find((c) => c.key === "PATCH /api/taps/1")?.body).toEqual({ label: "Pils" });
  });

  it("moves a tap up by sending the full reordered id list", async () => {
    const r = routes([one, two, three], { "PUT /api/taps/order": { status: 200, body: [] } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    fireEvent.click(within(item("Middle")).getByRole("button", { name: "Move up" }));
    await waitFor(() => expect(keys(calls)).toContain("PUT /api/taps/order"));
    expect(calls.find((c) => c.key === "PUT /api/taps/order")?.body).toEqual({ ids: [2, 1, 3] });
  });

  it("offers no Move up on the first tap and no Move down on the last", async () => {
    stubApi(routes([one, two, three]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    expect(within(item("Left")).queryByRole("button", { name: "Move up" })).toBeNull();
    expect(within(item("Left")).getByRole("button", { name: "Move down" })).toBeTruthy();
    expect(within(item("Right")).getByRole("button", { name: "Move up" })).toBeTruthy();
    expect(within(item("Right")).queryByRole("button", { name: "Move down" })).toBeNull();
  });

  it("disables an enabled tap and enables a disabled one", async () => {
    const calls = stubApi(
      routes([one, two], {
        "PATCH /api/taps/1": { status: 200, body: { ...one, enabled: false } },
        "PATCH /api/taps/2": { status: 200, body: { ...two, enabled: true } },
      }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/taps/1"));
    expect(calls.find((c) => c.key === "PATCH /api/taps/1")?.body).toEqual({ enabled: false });
    fireEvent.click(within(item("Middle")).getByRole("button", { name: "Enable" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/taps/2"));
    expect(calls.find((c) => c.key === "PATCH /api/taps/2")?.body).toEqual({ enabled: true });
  });

  it("removes a tap only after confirmation", async () => {
    const r = routes([one, three], { "DELETE /api/taps/3": { status: 204 } });
    const calls = stubApi(r);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Right" });
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Remove" }));
    expect(confirm).toHaveBeenCalledWith("Remove Right?");
    expect(keys(calls)).not.toContain("DELETE /api/taps/3");

    confirm.mockReturnValue(true);
    r["GET /api/taps"] = { status: 200, body: [one] };
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(screen.queryByRole("listitem", { name: "Right" })).toBeNull());
    expect(keys(calls)).toContain("DELETE /api/taps/3");
  });

  it("shows the server message for a conflict", async () => {
    stubApi(
      routes([one], {
        "DELETE /api/taps/1": { status: 409, body: { error: "last_tap", message: "The last tap cannot be removed." } },
      }),
    );
    vi.stubGlobal("confirm", () => true);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("listitem", { name: "Left" });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Remove" }));
    expect((await screen.findByRole("alert")).textContent).toBe("The last tap cannot be removed.");
  });

  it("reports a 401 as signed out", async () => {
    stubApi(routes([], { "GET /api/taps": { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<TapsPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});

const tapped = keg({
  id: 10,
  beer_name: "Stout",
  beer_color: "#470606",
  type: "corny_5_gal",
  remaining_l: 12.34,
  tap_id: 1,
  tap_label: "Left",
  state: "on_tap",
  tapped_at: "2026-11-09T12:00:00Z",
});
const half = keg({ id: 11, beer_name: "Lager", beer_color: "#ffbf42", type: "sanke_half", size_l: 58.67, remaining_l: 58.67 });
const withKeg = { ...one, keg: tapped };
const empty = tap({ id: 2, position: 2, label: "Right" });

/** kegRoutes stubs taps Left (with tapped) and Right (empty) and the waiting kegs. */
function kegRoutes(waiting: unknown[], extra: Record<string, Reply> = {}) {
  return routes([withKeg, empty], { "GET /api/kegs?state=waiting": { status: 200, body: waiting }, ...extra });
}

describe("TapsPanel kegs", () => {
  it("shows a tap's keg as a glass with its beer, type and level", async () => {
    stubApi(kegRoutes([]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    const art = within(item("Left")).getByRole("img");
    expect(art.getAttribute("data-vessel")).toBe("glass");
    expect(art.getAttribute("aria-label")).toBe("Stout, pint glass, 65% left");
    expect(art.querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("#470606");
    expect(within(item("Left")).getByText("Corny 5 gal")).toBeTruthy();
    expect(within(item("Left")).getByText(new RegExp(formatGallons(12.34)))).toBeTruthy();
    expect(within(item("Left")).queryByRole("button", { name: "Add keg" })).toBeNull();
  });

  it("offers Add keg on an empty tap, opening the keg form for that tap", async () => {
    const calls = stubApi(
      kegRoutes([], {
        "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] },
        "POST /api/kegs": { status: 201, body: keg({ tap_id: 2, state: "on_tap" }) },
      }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    expect(within(item("Right")).queryByRole("button", { name: "Put on tap" })).toBeNull();
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Add keg" }));
    const form = screen.getByRole("form", { name: "Add keg" });
    await within(form).findByRole("option", { name: "Pale" });
    fireEvent.change(within(form).getByLabelText("Beer"), { target: { value: "1" } });
    fireEvent.click(within(form).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("form", { name: "Add keg" })).toBeNull());
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({ beer_id: 1, type: "corny_5_gal", tap_id: 2 });
  });

  it("puts a waiting keg on an empty tap", async () => {
    const calls = stubApi(kegRoutes([half], { "PATCH /api/kegs/11": { status: 200, body: { ...half, tap_id: 2 } } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    const select = within(item("Right")).getByLabelText("Waiting keg") as HTMLSelectElement;
    expect(select.value).toBe("11");
    expect(within(select).getByRole("option").textContent).toBe("Lager, Half barrel");
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Put on tap" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/kegs/11"));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/11")?.body).toEqual({ tap_id: 2 });
  });

  it("takes a keg off its tap", async () => {
    const calls = stubApi(kegRoutes([], { "PATCH /api/kegs/10": { status: 200, body: { ...tapped, tap_id: null } } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Take off" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/kegs/10"));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ tap_id: null });
  });

  it("opens the keg form to edit a tap's keg", async () => {
    stubApi(kegRoutes([], { "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Stout" })] } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Edit keg" }));
    expect(screen.getByRole("form", { name: "Edit keg of Stout" })).toBeTruthy();
  });

  it("lists waiting kegs on deck in their keg's shape, with Add keg without a tap", async () => {
    const calls = stubApi(
      kegRoutes([half], {
        "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] },
        "POST /api/kegs": { status: 201, body: keg() },
      }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    const deck = await screen.findByRole("region", { name: "On deck" });
    const lager = await within(deck).findByRole("listitem");
    const art = within(lager).getByRole("img");
    expect(art.getAttribute("data-vessel")).toBe("keg");
    expect(art.getAttribute("data-keg-shape")).toBe("sanke");
    expect(art.querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("#ffbf42");
    expect(within(lager).getByText("Lager")).toBeTruthy();
    expect(within(lager).getByText("Half barrel")).toBeTruthy();
    expect(within(lager).getByText(new RegExp(formatGallons(58.67)))).toBeTruthy();

    fireEvent.click(within(deck).getByRole("button", { name: "Add keg" }));
    const form = screen.getByRole("form", { name: "Add keg" });
    await within(form).findByRole("option", { name: "Pale" });
    fireEvent.change(within(form).getByLabelText("Beer"), { target: { value: "1" } });
    fireEvent.click(within(form).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs"));
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({ beer_id: 1, type: "corny_5_gal" });
  });

  it("reports a 401 from the keg types as signed out", async () => {
    stubApi(kegRoutes([], { "GET /api/keg-types": { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<TapsPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});

describe("TapsPanel levels", () => {
  it("records a 16 fl oz pour in two clicks and shows the new level", async () => {
    const r = kegRoutes([], { "POST /api/kegs/10/pours": { status: 200, body: { ...tapped, remaining_l: 11.87 } } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    r["GET /api/taps"] = { status: 200, body: [{ ...withKeg, keg: { ...tapped, remaining_l: 11.87 } }, empty] };
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Pour" }));
    expect((within(item("Left")).getByLabelText("Pour (fl oz)") as HTMLInputElement).value).toBe("16");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save pour" }));
    expect(await within(item("Left")).findByText(new RegExp(formatGallons(11.87)))).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/kegs/10/pours")?.body).toEqual({ amount_l: 0.47 });
    expect(within(item("Left")).queryByLabelText("Pour (fl oz)")).toBeNull();
  });

  it("sets the level in gallons", async () => {
    const calls = stubApi(kegRoutes([], { "PATCH /api/kegs/10": { status: 200, body: tapped } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Set level" }));
    const input = within(item("Left")).getByLabelText("Remaining (gal)") as HTMLInputElement;
    expect(input.value).toBe("3.26");
    fireEvent.change(input, { target: { value: "2.5" } });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save level" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/kegs/10"));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ remaining_l: gallonsToLitres(2.5) });
  });

  it("sends nothing when the level is saved unchanged", async () => {
    const calls = stubApi(kegRoutes([]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Set level" }));
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save level" }));
    expect(within(item("Left")).queryByLabelText("Remaining (gal)")).toBeNull();
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
  });

  it("pours from a waiting keg on deck", async () => {
    const calls = stubApi(kegRoutes([half], { "POST /api/kegs/11/pours": { status: 200, body: half } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    const deck = await screen.findByRole("region", { name: "On deck" });
    fireEvent.click(await within(deck).findByRole("button", { name: "Pour" }));
    fireEvent.change(within(deck).getByLabelText("Pour (fl oz)"), { target: { value: "12" } });
    fireEvent.click(within(deck).getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs/11/pours"));
    expect(calls.find((c) => c.key === "POST /api/kegs/11/pours")?.body).toEqual({ amount_l: 0.35 });
  });

  it("draws each keg's level as its filled artwork, with gallons and no meter or inline style", async () => {
    const quarter = { ...tapped, remaining_l: 18.93 / 4 };
    const waiting = { ...half, remaining_l: 58.67 * 0.3 };
    stubApi(routes([{ ...one, keg: quarter }, empty], { "GET /api/kegs?state=waiting": { status: 200, body: [waiting] } }));
    const { container } = render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    const glass = within(item("Left")).getByRole("img");
    expect(glass.getAttribute("data-vessel")).toBe("glass");
    expect(glass.getAttribute("aria-label")).toMatch(/25% left$/);
    expect(within(item("Left")).getByText(new RegExp(formatGallons(18.93 / 4)))).toBeTruthy();
    const deck = await screen.findByRole("region", { name: "On deck" });
    const kegArt = await within(deck).findByRole("img");
    expect(kegArt.getAttribute("data-keg-shape")).toBe("sanke");
    expect(kegArt.getAttribute("aria-label")).toBe("Lager, keg, 30% left");
    expect(screen.queryAllByRole("meter")).toHaveLength(0);
    expect(container.querySelectorAll("[style]")).toHaveLength(0);
  });
});

describe("TapsPanel kick", () => {
  it("kicks a keg only after confirmation, then reloads", async () => {
    const r = kegRoutes([], { "PATCH /api/kegs/10": { status: 200, body: { ...tapped, state: "kicked" } } });
    const calls = stubApi(r);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Kick" }));
    expect(confirm).toHaveBeenCalledWith("Kick the Stout keg? It moves to keg history.");
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
    expect(onKegsChanged).not.toHaveBeenCalled();

    confirm.mockReturnValue(true);
    r["GET /api/taps"] = { status: 200, body: [one, empty] };
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Kick" }));
    expect(await within(item("Left")).findByRole("button", { name: "Add keg" })).toBeTruthy();
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ kicked: true });
    expect(onKegsChanged).toHaveBeenCalledTimes(1);
  });
});

describe("TapsPanel onKegsChanged", () => {
  it("fires after putting a waiting keg on a tap", async () => {
    stubApi(kegRoutes([half], { "PATCH /api/kegs/11": { status: 200, body: { ...half, tap_id: 2 } } }));
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Put on tap" }));
    await waitFor(() => expect(onKegsChanged).toHaveBeenCalledTimes(1));
  });

  it("fires after taking a keg off its tap", async () => {
    stubApi(kegRoutes([], { "PATCH /api/kegs/10": { status: 200, body: { ...tapped, tap_id: null } } }));
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Take off" }));
    await waitFor(() => expect(onKegsChanged).toHaveBeenCalledTimes(1));
  });

  it("fires after creating a keg", async () => {
    stubApi(
      kegRoutes([], {
        "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] },
        "POST /api/kegs": { status: 201, body: keg({ tap_id: 2, state: "on_tap" }) },
      }),
    );
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Add keg" }));
    const form = screen.getByRole("form", { name: "Add keg" });
    await within(form).findByRole("option", { name: "Pale" });
    fireEvent.change(within(form).getByLabelText("Beer"), { target: { value: "1" } });
    fireEvent.click(within(form).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onKegsChanged).toHaveBeenCalledTimes(1));
  });

  it("does not fire after a failed keg change", async () => {
    stubApi(
      kegRoutes([], { "PATCH /api/kegs/10": { status: 409, body: { error: "conflict", message: "Try again." } } }),
    );
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Take off" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Try again.");
    expect(onKegsChanged).not.toHaveBeenCalled();
  });

  it("does not fire after a pour or a level set", async () => {
    const calls = stubApi(
      kegRoutes([], {
        "POST /api/kegs/10/pours": { status: 200, body: tapped },
        "PATCH /api/kegs/10": { status: 200, body: tapped },
      }),
    );
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Pour" }));
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs/10/pours"));
    fireEvent.click(await within(item("Left")).findByRole("button", { name: "Set level" }));
    fireEvent.change(within(item("Left")).getByLabelText("Remaining (gal)"), { target: { value: "1" } });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save level" }));
    await waitFor(() => expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(3));
    expect(keys(calls)).toContain("PATCH /api/kegs/10");
    expect(onKegsChanged).not.toHaveBeenCalled();
  });

  it("does not fire after a tap-only change", async () => {
    const calls = stubApi(kegRoutes([], { "PATCH /api/taps/1": { status: 200, body: { ...withKeg, enabled: false } } }));
    const onKegsChanged = vi.fn();
    render(<TapsPanel onSignedOut={vi.fn()} onKegsChanged={onKegsChanged} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(2));
    expect(onKegsChanged).not.toHaveBeenCalled();
  });
});
