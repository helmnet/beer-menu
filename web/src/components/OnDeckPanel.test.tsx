import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { beer, keg, kegTypes, Reply, stubApi } from "../testApi";
import OnDeckPanel from "./OnDeckPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const onTap = keg({ id: 1, beer_id: 1, beer_name: "Tapped", tap_id: 3, tap_label: "Left", state: "on_tap" });
const waitOld = keg({ id: 2, beer_id: 2, beer_name: "Zwickel", type: "sanke_half", size_l: 58.67, remaining_l: 58.67 });
const waitNew = keg({ id: 3, beer_id: 3, beer_name: "Amber", created_at: "2026-11-10T12:00:00Z" });

const beers = [
  beer({ id: 1, name: "Tapped", status: "ready" }),
  beer({ id: 2, name: "Zwickel", status: "conditioning" }),
  beer({ id: 3, name: "Amber", status: "fermenting" }),
  beer({ id: 10, name: "stout", status: "fermenting", style: "Dry Stout", abv: 4.2 }),
  beer({ id: 11, name: "Bock", status: "fermenting" }),
  beer({ id: 12, name: "Saison", status: "conditioning" }),
  beer({ id: 13, name: "apa", status: "conditioning" }),
  beer({ id: 14, name: "Apa", status: "conditioning" }),
  beer({ id: 15, name: "Planned", status: "planned" }),
  beer({ id: 16, name: "Ready", status: "ready" }),
];

function routes(extra: Record<string, Reply> = {}): Record<string, Reply> {
  return {
    "GET /api/kegs": { status: 200, body: [onTap, waitOld, waitNew] },
    "GET /api/beers": { status: 200, body: beers },
    "GET /api/keg-types": { status: 200, body: kegTypes },
    ...extra,
  };
}

const list = () => screen.getByRole("list", { name: "On deck" });
const names = () => within(list()).getAllByRole("article").map((a) => a.getAttribute("aria-label"));
const item = (name: string) => within(list()).getByRole("article", { name });
const keys = (calls: { key: string }[]) => calls.map((c) => c.key);

describe("OnDeckPanel", () => {
  it("lists waiting kegs in API order, then conditioning, then fermenting beers without a keg", async () => {
    stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 2, name: "On Deck" })).toBeTruthy();
    await screen.findByRole("article", { name: "Zwickel" });
    // Ties by name ignoring case fall back to id: "apa" (13) before "Apa" (14).
    expect(names()).toEqual(["Zwickel", "Amber", "apa", "Apa", "Saison", "Bock", "stout"]);
  });

  it("lists a beer with a current keg once, as its keg, and leaves out other statuses", async () => {
    stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Amber" });
    expect(within(list()).getAllByRole("article", { name: "Amber" })).toHaveLength(1);
    expect(within(item("Amber")).getByRole("button", { name: "Edit keg" })).toBeTruthy();
    for (const name of ["Tapped", "Planned", "Ready"]) {
      expect(within(list()).queryByRole("article", { name })).toBeNull();
    }
  });

  it("shows beers with style, ABV and status", async () => {
    stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "stout" });
    expect(within(item("stout")).getByText("Dry Stout")).toBeTruthy();
    expect(within(item("stout")).getByText("4.2%")).toBeTruthy();
    expect(within(item("stout")).getByText("Fermenting")).toBeTruthy();
    expect(within(item("Saison")).getByText("Conditioning")).toBeTruthy();
    expect(within(item("stout")).queryByRole("button")).toBeNull();
  });

  it("gives waiting kegs only Edit keg and Kick", async () => {
    stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Zwickel" });
    expect(within(item("Zwickel")).getByText("Half barrel")).toBeTruthy();
    const buttons = within(item("Zwickel"))
      .getAllByRole("button")
      .map((b) => b.textContent);
    expect(buttons).toEqual(["Edit keg", "Kick"]);
    for (const name of ["Pour", "Set level"]) {
      expect(within(list()).queryByRole("button", { name })).toBeNull();
    }
  });

  it("kicks a waiting keg and reloads", async () => {
    vi.stubGlobal("confirm", vi.fn(() => true));
    const calls = stubApi(routes({ "PATCH /api/kegs/2": { status: 200, body: { ...waitOld, state: "kicked" } } }));
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Zwickel" });
    fireEvent.click(within(item("Zwickel")).getByRole("button", { name: "Kick" }));
    await waitFor(() => expect(keys(calls).filter((k) => k === "GET /api/kegs")).toHaveLength(2));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/2")?.body).toEqual({ kicked: true });
  });

  it("edits a waiting keg inside its card", async () => {
    stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Zwickel" });
    fireEvent.click(within(item("Zwickel")).getByRole("button", { name: "Edit keg" }));
    expect(within(item("Zwickel")).getByRole("form", { name: "Edit keg of Zwickel" })).toBeTruthy();
  });

  it("says Nothing on deck when the list is empty", async () => {
    stubApi(routes({ "GET /api/kegs": { status: 200, body: [onTap] } , "GET /api/beers": { status: 200, body: [beers[0]] } }));
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    expect(await screen.findByText("Nothing on deck")).toBeTruthy();
    expect(screen.queryByRole("list", { name: "On deck" })).toBeNull();
  });

  it("adds a waiting keg from the Add keg dialog and reloads", async () => {
    const r = routes({ "POST /api/kegs": { status: 201, body: keg({ id: 4, beer_id: 11, beer_name: "Bock" }) } });
    const calls = stubApi(r);
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Bock" });
    fireEvent.click(screen.getByRole("button", { name: "Add keg" }));
    const dialog = screen.getByRole("dialog", { name: "Add keg" });
    fireEvent.change(within(dialog).getByRole("combobox", { name: "Beer" }), { target: { value: "bo" } });
    fireEvent.click(await within(dialog).findByRole("option", { name: "Bock" }));
    r["GET /api/kegs"] = { status: 200, body: [onTap, waitOld, waitNew, keg({ id: 4, beer_id: 11, beer_name: "Bock" })] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Add keg" })).toBeNull());
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({ beer_id: 11, type: "corny_5_gal" });
    await waitFor(() => expect(within(item("Bock")).getByRole("button", { name: "Kick" })).toBeTruthy());
  });

  it("opens the same Add keg dialog from the top bar slot it is given", async () => {
    stubApi(routes());
    const slot = document.createElement("div");
    document.body.appendChild(slot);
    render(<OnDeckPanel onSignedOut={vi.fn()} actionSlot={slot} />);
    await screen.findByRole("article", { name: "Zwickel" });
    expect(screen.getAllByRole("button", { name: "Add keg" })).toHaveLength(2);
    fireEvent.click(within(slot).getByRole("button", { name: "Add keg" }));
    expect(screen.getByRole("dialog", { name: "Add keg" })).toBeTruthy();
    slot.remove();
  });

  it("closes the Add keg dialog on Cancel without a request", async () => {
    const calls = stubApi(routes());
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("article", { name: "Bock" });
    fireEvent.click(screen.getByRole("button", { name: "Add keg" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Add keg" })).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(keys(calls).filter((k) => !k.startsWith("GET"))).toEqual([]);
  });

  it.each(["GET /api/kegs", "GET /api/beers", "GET /api/keg-types"])("reports a 401 from %s as signed out", async (key) => {
    stubApi(routes({ [key]: { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<OnDeckPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("shows other load errors", async () => {
    stubApi(routes({ "GET /api/beers": { status: 500, body: { error: "internal", message: "boom" } } }));
    render(<OnDeckPanel onSignedOut={vi.fn()} />);
    expect((await screen.findByRole("alert")).textContent).toBe("boom");
  });
});
