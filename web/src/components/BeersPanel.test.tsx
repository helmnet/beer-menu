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

  it("adds a beer through the Add beer dialog, then closes it and reloads", async () => {
    const routes = {
      "GET /api/beers": { status: 200, body: [pale] as unknown[] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
      "POST /api/beers": { status: 201, body: stout },
    };
    const calls = stubApi(routes);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Add beer" }));
    const dialog = screen.getByRole("dialog", { name: "Add beer" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "Stout" } });
    routes["GET /api/beers"] = { status: 200, body: [pale, stout] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Stout")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "Add beer" })).toBeNull();
    expect(calls.find((c) => c.key === "POST /api/beers")?.body).toEqual({ name: "Stout", status: "ready" });
  });

  it("opens the same Add beer dialog from the top bar slot it is given", async () => {
    stubApi({ "GET /api/beers": { status: 200, body: [pale] }, "GET /api/srm-colors": { status: 200, body: { colors: [] } } });
    const slot = document.createElement("div");
    document.body.appendChild(slot);
    render(<BeersPanel onSignedOut={vi.fn()} actionSlot={slot} />);
    await screen.findByText("Pale");
    expect(screen.getAllByRole("button", { name: "Add beer" })).toHaveLength(2);
    fireEvent.click(within(slot).getByRole("button", { name: "Add beer" }));
    expect(screen.getByRole("dialog", { name: "Add beer" })).toBeTruthy();
    slot.remove();
  });

  it("opens the Edit dialog filled with the beer's values", async () => {
    stubApi({
      "GET /api/beers": { status: 200, body: [pale] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
    });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Edit" }));
    const dialog = screen.getByRole("dialog", { name: "Edit Pale" });
    expect((within(dialog).getByLabelText("Name") as HTMLInputElement).value).toBe("Pale");
    expect((within(dialog).getByLabelText("ABV") as HTMLInputElement).value).toBe("5.2");
  });

  it("saves an edit, closes the dialog and reloads", async () => {
    const routes = {
      "GET /api/beers": { status: 200, body: [pale] as unknown[] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
      "PATCH /api/beers/1": { status: 200, body: { ...pale, abv: 5.5 } },
    };
    const calls = stubApi(routes);
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Edit" }));
    const dialog = screen.getByRole("dialog", { name: "Edit Pale" });
    fireEvent.change(within(dialog).getByLabelText("ABV"), { target: { value: "5.5" } });
    routes["GET /api/beers"] = { status: 200, body: [{ ...pale, abv: 5.5 }] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await within(row("Pale")).findByText("5.5%")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "Edit Pale" })).toBeNull();
    expect(calls.find((c) => c.key === "PATCH /api/beers/1")?.body).toEqual({ abv: 5.5 });
  });

  it("keeps the dialog open with field errors on a 400", async () => {
    const calls = stubApi({
      "GET /api/beers": { status: 200, body: [pale] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
      "POST /api/beers": {
        status: 400,
        body: { error: "invalid", message: "Some fields are invalid.", fields: { name: "Name is required." } },
      },
    });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Add beer" }));
    const dialog = screen.getByRole("dialog", { name: "Add beer" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await within(dialog).findByText("Name is required.")).toBeTruthy();
    expect(screen.getByRole("dialog", { name: "Add beer" })).toBe(dialog);
    expect(calls.filter((c) => c.key === "GET /api/beers")).toHaveLength(1);
  });

  it("closes the dialog on Cancel or Escape and sends nothing", async () => {
    const calls = stubApi({
      "GET /api/beers": { status: 200, body: [pale] },
      "GET /api/srm-colors": { status: 200, body: { colors: [] } },
    });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    fireEvent.click(screen.getByRole("button", { name: "Add beer" }));
    fireEvent.click(within(screen.getByRole("dialog", { name: "Add beer" })).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    fireEvent.click(within(row("Pale")).getByRole("button", { name: "Edit" }));
    fireEvent(screen.getByRole("dialog", { name: "Edit Pale" }), new Event("cancel", { cancelable: true }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(calls.map((c) => c.key).filter((k) => !k.startsWith("GET"))).toEqual([]);
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

  it("draws a beer with a keg on a tap as a glass and the others as kegs", async () => {
    stubApi({ "GET /api/beers": { status: 200, body: [{ ...pale, on_tap: true }, stout] } });
    render(<BeersPanel onSignedOut={vi.fn()} />);
    await screen.findByText("Pale");
    expect(within(row("Pale")).getByRole("img").getAttribute("data-vessel")).toBe("glass");
    expect(within(row("Stout")).getByRole("img").getAttribute("data-vessel")).toBe("keg");
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
  describe("Brewfather beers", () => {
    const sync = { synced_at: "2026-11-09T12:00:00Z", not_found: false, archive_pending: false };
    const imported = beer({ id: 3, name: "Imported", source: "brewfather", source_ref: "b3", sync });

    it("labels Brewfather beers and not manual ones", async () => {
      stubApi({ "GET /api/beers": { status: 200, body: [pale, imported] } });
      render(<BeersPanel onSignedOut={vi.fn()} />);
      await screen.findByText("Imported");
      expect(within(row("Imported")).getByText("Brewfather")).toBeTruthy();
      expect(within(row("Pale")).queryByText("Brewfather")).toBeNull();
    });

    it("re-syncs a Brewfather beer and reloads; manual beers have no Re-sync", async () => {
      const routes = {
        "GET /api/beers": { status: 200, body: [pale, imported] as unknown[] },
        "POST /api/beers/3/sync": { status: 200, body: { beer: { ...imported, abv: 6.1 }, adjusted: [] } },
      };
      const calls = stubApi(routes);
      render(<BeersPanel onSignedOut={vi.fn()} />);
      await screen.findByText("Imported");
      expect(within(row("Pale")).queryByRole("button", { name: "Re-sync" })).toBeNull();
      routes["GET /api/beers"] = { status: 200, body: [pale, { ...imported, abv: 6.1 }] };
      fireEvent.click(within(row("Imported")).getByRole("button", { name: "Re-sync" }));
      expect(await within(row("Imported")).findByText("6.1%")).toBeTruthy();
      expect(calls.map((c) => c.key)).toEqual(["GET /api/beers", "POST /api/beers/3/sync", "GET /api/beers"]);
    });

    it("notes a beer whose batch Brewfather no longer has", async () => {
      stubApi({ "GET /api/beers": { status: 200, body: [{ ...imported, sync: { ...sync, not_found: true } }] } });
      render(<BeersPanel onSignedOut={vi.fn()} />);
      await screen.findByText("Imported");
      expect(within(row("Imported")).getByText("Not found in Brewfather")).toBeTruthy();
    });

    it("notes a beer whose batch was archived while it has a keg", async () => {
      stubApi({ "GET /api/beers": { status: 200, body: [{ ...imported, sync: { ...sync, archive_pending: true } }] } });
      render(<BeersPanel onSignedOut={vi.fn()} />);
      await screen.findByText("Imported");
      expect(within(row("Imported")).getByText("Archived in Brewfather; kick its keg to archive")).toBeTruthy();
      expect(within(row("Imported")).queryByText("Not found in Brewfather")).toBeNull();
    });

    it("shows the retry time when Brewfather's rate limit is reached", async () => {
      stubApi({
        "GET /api/beers": { status: 200, body: [imported] },
        "POST /api/beers/3/sync": {
          status: 429,
          body: { error: "provider_rate_limited", message: "x", retry_at: "2026-11-09T18:42:00Z" },
          headers: { "Retry-After": "120" },
        },
      });
      render(<BeersPanel onSignedOut={vi.fn()} />);
      await screen.findByText("Imported");
      const before = Date.now();
      fireEvent.click(within(row("Imported")).getByRole("button", { name: "Re-sync" }));
      const text = (await screen.findByRole("alert")).textContent ?? "";
      const at = (ms: number) => new Date(ms).toLocaleTimeString();
      expect([at(before + 120_000), at(Date.now() + 120_000)]).toContain(text.replace(/^Brewfather's rate limit is reached\. Try again after (.*)\.$/, "$1"));
    });
  });
});
