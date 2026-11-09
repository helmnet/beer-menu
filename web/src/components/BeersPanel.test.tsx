import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { beer, stubApi } from "../testApi";
import BeersPanel from "./BeersPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const pale = beer({ id: 1, name: "Pale", style: "APA", abv: 5.2, srm: 4, color: "#ffbf42" });
const stout = beer({ id: 2, name: "Stout", style: "Dry Stout", abv: 4.2, status: "fermenting", srm: 35, color: "#470606" });

function row(name: string) {
  return screen.getByText(name).closest("tr") as HTMLElement;
}

describe("BeersPanel", () => {
  it("lists beers with name, style, ABV and status", async () => {
    stubApi({ "GET /api/beers": { status: 200, body: [pale, stout] } });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    expect(within(row("Pale")).getByText("APA")).toBeTruthy();
    expect(within(row("Pale")).getByText("5.2%")).toBeTruthy();
    expect(within(row("Pale")).getByText("ready")).toBeTruthy();
    expect(within(row("Stout")).getByText("Dry Stout")).toBeTruthy();
    expect(within(row("Stout")).getByText("fermenting")).toBeTruthy();
  });

  it("adds a beer through the form", async () => {
    const routes = {
      "GET /api/beers": { status: 200, body: [pale] as unknown[] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
      "POST /api/beers": { status: 201, body: stout },
    };
    const calls = stubApi(routes);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Add beer" }));
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Stout" } });
    routes["GET /api/beers"] = { status: 200, body: [pale, stout] };
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Stout")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    expect(calls.find((c) => c.key === "POST /api/beers")?.body).toEqual({ name: "Stout", status: "ready" });
  });

  it("opens the form with the beer's values to edit", async () => {
    stubApi({
      "GET /api/beers": { status: 200, body: [pale] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
    });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Edit" }));
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Pale");
    expect((screen.getByLabelText("ABV") as HTMLInputElement).value).toBe("5.2");
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "GET /api/beers": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    render(<BeersPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("draws each beer as a keg filled with its color, without the color table", async () => {
    const calls = stubApi({ "GET /api/beers": { status: 200, body: [pale, stout, beer({ id: 3, name: "Plain" })] } });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    const art = (name: string) => {
      const svg = within(row(name)).getByRole("img");
      return [svg.getAttribute("data-vessel"), svg.querySelector('[data-part="liquid"]')?.getAttribute("fill")];
    };
    expect(art("Pale")).toEqual(["keg", "#ffbf42"]);
    expect(art("Stout")).toEqual(["keg", "#470606"]);
    expect(art("Plain")).toEqual(["keg", "none"]);
    expect(within(row("Stout")).getByRole("img").getAttribute("aria-label")).toBe("Stout, keg");
    expect(calls.map((c) => c.key)).toEqual(["GET /api/beers"]);
  });

  it("shows archived beers on request, marked archived", async () => {
    const old = beer({ id: 9, name: "Old Ale", archived_at: "2026-11-01T12:00:00Z" });
    const calls = stubApi({
      "GET /api/beers": { status: 200, body: [pale] },
      "GET /api/beers?archived=true": { status: 200, body: [old] },
    });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(screen.getByLabelText("Show archived"));
    expect(await screen.findByText("Old Ale")).toBeTruthy();
    expect(screen.queryByText("Pale")).toBeNull();
    expect(within(row("Old Ale")).getByText("archived")).toBeTruthy();
    expect(within(row("Old Ale")).queryByRole("button", { name: "Archive" })).toBeNull();
    expect(calls.map((c) => c.key)).toContain("GET /api/beers?archived=true");

    fireEvent.click(screen.getByLabelText("Show archived"));
    expect(await screen.findByText("Pale")).toBeTruthy();
  });

  it("archives a current beer", async () => {
    const routes = {
      "GET /api/beers": { status: 200, body: [pale, stout] as unknown[] },
      "PATCH /api/beers/1": { status: 200, body: { ...pale, archived_at: "2026-11-09T13:00:00Z" } },
    };
    const calls = stubApi(routes);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    routes["GET /api/beers"] = { status: 200, body: [stout] };
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Archive" }));
    await waitFor(() => expect(screen.queryByText("Pale")).toBeNull());
    expect(calls.find((c) => c.key === "PATCH /api/beers/1")?.body).toEqual({ archived: true });
    expect(screen.getByText("Stout")).toBeTruthy();
  });

  it("restores an archived beer", async () => {
    const old = beer({ id: 9, name: "Old Ale", archived_at: "2026-11-01T12:00:00Z" });
    const routes = {
      "GET /api/beers": { status: 200, body: [pale] as unknown[] },
      "GET /api/beers?archived=true": { status: 200, body: [old] as unknown[] },
      "PATCH /api/beers/9": { status: 200, body: { ...old, archived_at: null } },
    };
    const calls = stubApi(routes);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(screen.getByLabelText("Show archived"));
    await screen.findByText("Old Ale");
    routes["GET /api/beers?archived=true"] = { status: 200, body: [] };
    fireEvent.click(within(row("Old Ale")).getByRole("button", { name: "Restore" }));
    await waitFor(() => expect(screen.queryByText("Old Ale")).toBeNull());
    expect(calls.find((c) => c.key === "PATCH /api/beers/9")?.body).toEqual({ archived: false });
  });

  it("deletes only after confirmation", async () => {
    const routes = {
      "GET /api/beers": { status: 200, body: [pale, stout] as unknown[] },
      "DELETE /api/beers/1": { status: 204 },
    };
    const calls = stubApi(routes);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Delete" }));
    expect(confirm).toHaveBeenCalledWith("Delete Pale permanently?");
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/beers/1");
    expect(screen.getByText("Pale")).toBeTruthy();

    confirm.mockReturnValue(true);
    routes["GET /api/beers"] = { status: 200, body: [stout] };
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByText("Pale")).toBeNull());
    expect(calls.map((c) => c.key)).toContain("DELETE /api/beers/1");
  });
});
