import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { formatVolume, volumeToLitres } from "../units";
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

/** item returns the card of the tap labelled label. */
function item(label: string) {
  return within(screen.getByRole("list", { name: "Taps" })).getByRole("article", { name: label });
}

function keys(calls: { key: string }[]) {
  return calls.map((c) => c.key);
}

/** pickBeer chooses the beer named name through the keg form's Beer combobox inside container. */
async function pickBeer(container: HTMLElement, name: string) {
  const input = within(container).getByRole("combobox", { name: "Beer" });
  fireEvent.change(input, { target: { value: name } });
  fireEvent.click(await within(container).findByRole("option", { name }));
}

describe("TapsPanel taps", () => {
  it("lists taps in position order with labels and a disabled marker", async () => {
    stubApi(routes([one, two, three]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    expect(screen.getByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
    const cards = within(screen.getByRole("list", { name: "Taps" })).getAllByRole("article");
    expect(cards.map((c) => c.getAttribute("aria-label"))).toEqual(["Left", "Middle", "Right"]);
    expect(within(item("Left")).getByRole("heading").textContent).toBe("1. Left");
    expect(within(item("Middle")).getByRole("heading").textContent).toBe("2. Middle (disabled)");
  });

  it("adds a tap from the Add tap dialog with no label, sending {}, then reloads", async () => {
    const r = routes([one], { "POST /api/taps": { status: 201, body: tap({ id: 4, position: 2, label: "Tap 2" }) } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    r["GET /api/taps"] = { status: 200, body: [one, tap({ id: 4, position: 2, label: "Tap 2" })] };
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    const dialog = screen.getByRole("dialog", { name: "Add tap" });
    expect((within(dialog).getByLabelText("Label") as HTMLInputElement).value).toBe("");
    expect(keys(calls)).not.toContain("POST /api/taps");
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("article", { name: "Tap 2" })).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/taps")?.body).toEqual({});
    expect(screen.queryByRole("dialog", { name: "Add tap" })).toBeNull();
  });

  it("sends the label typed in the Add tap dialog", async () => {
    const r = routes([one], { "POST /api/taps": { status: 201, body: tap({ id: 4, position: 2, label: "Guest" }) } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    const dialog = screen.getByRole("dialog", { name: "Add tap" });
    fireEvent.change(within(dialog).getByLabelText("Label"), { target: { value: "Guest" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Add tap" })).toBeNull());
    expect(calls.find((c) => c.key === "POST /api/taps")?.body).toEqual({ label: "Guest" });
  });

  it("keeps the Add tap dialog open with the field error on a 400", async () => {
    const calls = stubApi(
      routes([one], {
        "POST /api/taps": {
          status: 400,
          body: { error: "invalid", message: "Some fields are invalid.", fields: { label: "Must be 1 to 50 characters." } },
        },
      }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    const dialog = screen.getByRole("dialog", { name: "Add tap" });
    fireEvent.change(within(dialog).getByLabelText("Label"), { target: { value: "x".repeat(51) } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await within(dialog).findByText("Must be 1 to 50 characters.")).toBeTruthy();
    expect(within(dialog).getByLabelText("Label").getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("dialog", { name: "Add tap" })).toBe(dialog);
    expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(1);
  });

  it("closes the Add tap dialog on Cancel or Escape without a request", async () => {
    const calls = stubApi(routes([one]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Add tap" })).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog", { name: "Add tap" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Add tap" }));
    fireEvent(screen.getByRole("dialog", { name: "Add tap" }), new Event("cancel", { cancelable: true }));
    expect(screen.queryByRole("dialog", { name: "Add tap" })).toBeNull();
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
  });

  it("opens the same Add tap dialog from the top bar slot it is given", async () => {
    const r = routes([one], { "POST /api/taps": { status: 201, body: tap({ id: 4, position: 2, label: "Tap 2" }) } });
    const calls = stubApi(r);
    const slot = document.createElement("div");
    document.body.appendChild(slot);
    render(<TapsPanel onSignedOut={vi.fn()} actionSlot={slot} />);
    await screen.findByRole("article", { name: "Left" });
    expect(screen.getAllByRole("button", { name: "Add tap" })).toHaveLength(2);
    r["GET /api/taps"] = { status: 200, body: [one, tap({ id: 4, position: 2, label: "Tap 2" })] };
    fireEvent.click(within(slot).getByRole("button", { name: "Add tap" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Add tap" })).getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("article", { name: "Tap 2" })).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/taps")?.body).toEqual({});
    slot.remove();
  });

  it("renames a tap", async () => {
    const r = routes([one], { "PATCH /api/taps/1": { status: 200, body: { ...one, label: "Pils" } } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    r["GET /api/taps"] = { status: 200, body: [{ ...one, label: "Pils" }] };
    expect(within(item("Left")).queryByLabelText("Label")).toBeNull();
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Rename" }));
    const form = within(item("Left")).getByRole("form", { name: "Rename Left" });
    fireEvent.change(within(form).getByLabelText("Label"), { target: { value: "Pils" } });
    fireEvent.click(within(form).getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("article", { name: "Pils" })).toBeTruthy();
    expect(calls.find((c) => c.key === "PATCH /api/taps/1")?.body).toEqual({ label: "Pils" });
    expect(screen.queryByRole("form", { name: "Rename Pils" })).toBeNull();
  });

  it("closes the rename form on Cancel or an unchanged label, sending nothing", async () => {
    const calls = stubApi(routes([one]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Rename" }));
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("form", { name: "Rename Left" })).toBeNull();
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Rename" }));
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save" }));
    expect(screen.queryByRole("form", { name: "Rename Left" })).toBeNull();
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
  });

  it("summarizes the taps under the heading", async () => {
    stubApi(routes([{ ...one, keg: keg({ tap_id: 1, state: "on_tap" }) }, three], {
      "GET /api/kegs?state=waiting": { status: 200, body: [keg({ id: 2 })] },
    }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    expect(await screen.findByText("2 taps, 1 with a keg, 1 keg waiting")).toBeTruthy();
  });

  it("moves a tap up by sending the full reordered id list", async () => {
    const r = routes([one, two, three], { "PUT /api/taps/order": { status: 200, body: [] } });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
    fireEvent.click(within(item("Middle")).getByRole("button", { name: "Move up" }));
    await waitFor(() => expect(keys(calls)).toContain("PUT /api/taps/order"));
    expect(calls.find((c) => c.key === "PUT /api/taps/order")?.body).toEqual({ ids: [2, 1, 3] });
  });

  it("offers no Move up on the first tap and no Move down on the last", async () => {
    stubApi(routes([one, two, three]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Left" });
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
    await screen.findByRole("article", { name: "Left" });
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
    await screen.findByRole("article", { name: "Right" });
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Remove" }));
    expect(confirm).toHaveBeenCalledWith("Remove Right?");
    expect(keys(calls)).not.toContain("DELETE /api/taps/3");

    confirm.mockReturnValue(true);
    r["GET /api/taps"] = { status: 200, body: [one] };
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(screen.queryByRole("article", { name: "Right" })).toBeNull());
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
    await screen.findByRole("article", { name: "Left" });
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
    expect(within(item("Left")).getByText(new RegExp(formatVolume(12.34, "us")))).toBeTruthy();
    expect(within(item("Left")).queryByRole("button", { name: "Add keg" })).toBeNull();
  });

  it("offers Add keg on an empty tap, opening the Add keg dialog for that tap", async () => {
    const r = kegRoutes([], {
      "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] },
      "POST /api/kegs": { status: 201, body: keg({ tap_id: 2, state: "on_tap" }) },
    });
    const calls = stubApi(r);
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    expect(within(item("Right")).queryByRole("button", { name: "Put on tap" })).toBeNull();
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Add keg" }));
    const dialog = screen.getByRole("dialog", { name: "Add keg" });
    await pickBeer(dialog, "Pale");
    r["GET /api/taps"] = { status: 200, body: [withKeg, { ...empty, keg: keg({ id: 12, beer_name: "Pale", tap_id: 2, state: "on_tap" }) }] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Add keg" })).toBeNull());
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({ beer_id: 1, type: "corny_5_gal", tap_id: 2 });
    expect(await within(item("Right")).findByText("Pale")).toBeTruthy();
    expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(2);
  });

  it("closes the Add keg dialog on Cancel without a request", async () => {
    const calls = stubApi(kegRoutes([], { "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Add keg" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Add keg" })).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog", { name: "Add keg" })).toBeNull();
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
  });

  it("returns to sign-in when the session ends while saving from the Add keg dialog", async () => {
    stubApi(
      kegRoutes([], {
        "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Pale" })] },
        "POST /api/kegs": { status: 401, body: { error: "unauthenticated", message: "x" } },
      }),
    );
    const onSignedOut = vi.fn();
    render(<TapsPanel onSignedOut={onSignedOut} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Right")).getByRole("button", { name: "Add keg" }));
    const dialog = screen.getByRole("dialog", { name: "Add keg" });
    await pickBeer(dialog, "Pale");
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
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
    expect(within(item("Left")).getByRole("form", { name: "Edit keg of Stout" })).toBeTruthy();
  });

  it("shows no on-deck list (it has its own page)", async () => {
    stubApi(kegRoutes([half]));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    expect(screen.queryByRole("region", { name: "On deck" })).toBeNull();
    expect(screen.queryByText("Half barrel")).toBeNull();
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
    expect(await within(item("Left")).findByText(new RegExp(formatVolume(11.87, "us")))).toBeTruthy();
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
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ remaining_l: volumeToLitres(2.5, "us") });
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

  it("draws each keg's level as its filled artwork, with gallons and no meter or inline style", async () => {
    const quarter = { ...tapped, remaining_l: 18.93 / 4 };
    stubApi(routes([{ ...one, keg: quarter }, empty]));
    const { container } = render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    const glass = within(item("Left")).getByRole("img");
    expect(glass.getAttribute("data-vessel")).toBe("glass");
    expect(glass.getAttribute("aria-label")).toMatch(/25% left$/);
    expect(within(item("Left")).getByText(new RegExp(formatVolume(18.93 / 4, "us")))).toBeTruthy();
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
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Kick" }));
    expect(confirm).toHaveBeenCalledWith("Kick the Stout keg? It moves to keg history.");
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);

    confirm.mockReturnValue(true);
    r["GET /api/taps"] = { status: 200, body: [one, empty] };
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Kick" }));
    expect(await within(item("Left")).findByRole("button", { name: "Add keg" })).toBeTruthy();
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ kicked: true });
  });
});

describe("TapsPanel reloads", () => {
  it("shows the server message for a failed keg change", async () => {
    stubApi(
      kegRoutes([], { "PATCH /api/kegs/10": { status: 409, body: { error: "conflict", message: "Try again." } } }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Take off" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Try again.");
  });

  it("reloads the taps after a pour and a level set", async () => {
    const calls = stubApi(
      kegRoutes([], {
        "POST /api/kegs/10/pours": { status: 200, body: tapped },
        "PATCH /api/kegs/10": { status: 200, body: tapped },
      }),
    );
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Pour" }));
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs/10/pours"));
    fireEvent.click(await within(item("Left")).findByRole("button", { name: "Set level" }));
    fireEvent.change(within(item("Left")).getByLabelText("Remaining (gal)"), { target: { value: "1" } });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save level" }));
    await waitFor(() => expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(3));
    expect(keys(calls)).toContain("PATCH /api/kegs/10");
  });

  it("reloads the taps after a tap-only change", async () => {
    const calls = stubApi(kegRoutes([], { "PATCH /api/taps/1": { status: 200, body: { ...withKeg, enabled: false } } }));
    render(<TapsPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Disable" }));
    await waitFor(() => expect(keys(calls).filter((k) => k === "GET /api/taps")).toHaveLength(2));
  });
});

describe("TapsPanel metric", () => {
  const metricKeg = { ...tapped, size_l: 18.93, remaining_l: 12.5 };
  const metricRoutes = (extra: Record<string, Reply> = {}) =>
    routes([{ ...one, keg: metricKeg }, empty], extra);

  it("shows levels in litres", async () => {
    stubApi(metricRoutes());
    render(<TapsPanel units="metric" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    expect(within(item("Left")).getByText("12.50 L of 18.93 L")).toBeTruthy();
    expect(item("Left").textContent).not.toMatch(/\d\.\d\d gal/);
  });

  it("takes a pour in mL, prefilled 500, and sends litres", async () => {
    const calls = stubApi(metricRoutes({ "POST /api/kegs/10/pours": { status: 200, body: metricKeg } }));
    render(<TapsPanel units="metric" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Pour" }));
    expect((within(item("Left")).getByLabelText("Pour (mL)") as HTMLInputElement).value).toBe("500");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs/10/pours"));
    expect(calls.find((c) => c.key === "POST /api/kegs/10/pours")?.body).toEqual({ amount_l: 0.5 });
  });

  it("sets the level in litres", async () => {
    const calls = stubApi(metricRoutes({ "PATCH /api/kegs/10": { status: 200, body: metricKeg } }));
    render(<TapsPanel units="metric" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Set level" }));
    const input = within(item("Left")).getByLabelText("Remaining (L)") as HTMLInputElement;
    expect(input.value).toBe("12.50");
    fireEvent.change(input, { target: { value: "10" } });
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Save level" }));
    await waitFor(() => expect(keys(calls)).toContain("PATCH /api/kegs/10"));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ remaining_l: 10 });
  });

  it("shows US units when told so", async () => {
    stubApi(metricRoutes());
    render(<TapsPanel units="us" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    expect(within(item("Left")).getByText("3.30 gal of 5.00 gal")).toBeTruthy();
  });

  it("restarts an open keg form in the new units when the units change", async () => {
    stubApi(metricRoutes({ "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Stout" })] } }));
    const { rerender } = render(<TapsPanel units="us" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Edit keg" }));
    const form = () => screen.getByRole("form", { name: "Edit keg of Stout" });
    fireEvent.change(within(form()).getByLabelText("Remaining (gal)"), { target: { value: "1" } });
    rerender(<TapsPanel units="metric" onSignedOut={vi.fn()} />);
    expect((within(form()).getByLabelText("Size (L)") as HTMLInputElement).value).toBe("18.93");
    expect((within(form()).getByLabelText("Remaining (L)") as HTMLInputElement).value).toBe("12.50");
  });

  it("restarts an open level form in the new units when the units change", async () => {
    stubApi(metricRoutes());
    const { rerender } = render(<TapsPanel units="us" onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(item("Left")).getByRole("button", { name: "Set level" }));
    fireEvent.change(within(item("Left")).getByLabelText("Remaining (gal)"), { target: { value: "1" } });
    rerender(<TapsPanel units="metric" onSignedOut={vi.fn()} />);
    expect((within(item("Left")).getByLabelText("Remaining (L)") as HTMLInputElement).value).toBe("12.50");
  });
});
