import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { keg, kegTypes, Reply, stubApi } from "../testApi";
import KegHistory from "./KegHistory";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const stout = keg({
  id: 4,
  beer_name: "Stout",
  type: "sanke_half",
  size_l: 58.67,
  remaining_l: 0,
  tap_id: 1,
  tap_label: "Left",
  state: "kicked",
  tapped_at: "2026-10-01T18:00:00Z",
  kicked_at: "2026-11-02T20:00:00Z",
});
const never = keg({ id: 3, beer_name: "Pale", state: "kicked", kicked_at: "2026-10-15T12:00:00Z" });

function routes(extra: Record<string, Reply> = {}): Record<string, Reply> {
  return {
    "GET /api/kegs?state=kicked": { status: 200, body: [stout, never] },
    "GET /api/keg-types": { status: 200, body: kegTypes },
    ...extra,
  };
}

function row(name: string) {
  return screen.getByText(name).closest("tr") as HTMLElement;
}

const day = (iso: string) => new Date(iso).toLocaleDateString();

describe("KegHistory", () => {
  it("lists kicked kegs with beer, tap, type, size and dates", async () => {
    stubApi(routes());
    render(<KegHistory onSignedOut={vi.fn()} />);
    await screen.findByText("Half barrel");
    const cells = (name: string) => within(row(name)).getAllByRole("cell").map((c) => c.textContent);
    expect(cells("Stout").slice(0, 6)).toEqual([
      "Stout",
      "Left",
      "Half barrel",
      "15.50 gal",
      day("2026-10-01T18:00:00Z"),
      day("2026-11-02T20:00:00Z"),
    ]);
    expect(cells("Pale").slice(0, 6)).toEqual(["Pale", "—", "Corny 5 gal", "5.00 gal", "—", day("2026-10-15T12:00:00Z")]);
    expect(screen.getAllByRole("row").slice(1).map((r) => within(r).getAllByRole("cell")[0].textContent)).toEqual([
      "Stout",
      "Pale",
    ]);
  });

  it("shows sizes in litres with units metric and in gallons with units us", async () => {
    stubApi(routes());
    const { unmount } = render(<KegHistory units="metric" onSignedOut={vi.fn()} />);
    await screen.findByText("Half barrel");
    expect(within(row("Stout")).getByText("58.67 L")).toBeTruthy();
    expect(within(row("Pale")).getByText("18.93 L")).toBeTruthy();
    unmount();
    render(<KegHistory units="us" onSignedOut={vi.fn()} />);
    await screen.findByText("Half barrel");
    expect(within(row("Stout")).getByText("15.50 gal")).toBeTruthy();
  });

  it("deletes a keg only after confirmation", async () => {
    const r = routes({ "DELETE /api/kegs/4": { status: 204 } });
    const calls = stubApi(r);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    render(<KegHistory onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    fireEvent.click(within(row("Stout")).getByRole("button", { name: "Delete" }));
    expect(confirm).toHaveBeenCalledWith("Delete the Stout keg from history permanently?");
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/kegs/4");

    confirm.mockReturnValue(true);
    r["GET /api/kegs?state=kicked"] = { status: 200, body: [never] };
    fireEvent.click(within(row("Stout")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByText("Stout")).toBeNull());
    expect(calls.map((c) => c.key)).toContain("DELETE /api/kegs/4");
  });

  it("marks each Delete as a danger button", async () => {
    stubApi(routes());
    render(<KegHistory onSignedOut={vi.fn()} />);
    await screen.findByText("Stout");
    const deletes = screen.getAllByRole("button", { name: "Delete" });
    expect(deletes).toHaveLength(2);
    for (const button of deletes) expect(button.classList.contains("btn-danger")).toBe(true);
  });

  it("reports a 401 as signed out", async () => {
    stubApi(routes({ "GET /api/kegs?state=kicked": { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<KegHistory onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});
