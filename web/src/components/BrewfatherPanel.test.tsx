import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { batch, beer, brewfatherStatus, importResult, stubApi } from "../testApi";
import BrewfatherPanel from "./BrewfatherPanel";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

const connected = brewfatherStatus({ connected: true });
const noBatches = { status: 200, body: { fetched_at: "2026-11-09T12:00:00Z", batches: [] } };

// fakeKey returns an obviously fake API key so no real-looking secret is in the repo.
const fakeKey = () => `fake-key-${crypto.randomUUID()}`;

function renderPanel(props: Partial<{ onSignedOut: () => void }> = {}) {
  return render(<BrewfatherPanel onSignedOut={props.onSignedOut ?? vi.fn()} />);
}

function keyField() {
  return screen.getByLabelText("API key") as HTMLInputElement;
}

function fillForm(key: string) {
  fireEvent.change(screen.getByLabelText("User ID"), { target: { value: "user-1" } });
  fireEvent.change(keyField(), { target: { value: key } });
}

function row(name: string) {
  return screen.getByText(name).closest("tr") as HTMLElement;
}

describe("BrewfatherPanel connection", () => {
  it("shows the Brewfather heading with a Connection card and, when connected, a Batches card", async () => {
    stubApi({
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": { status: 200, body: { fetched_at: "2026-11-09T12:00:00Z", batches: [batch()] } },
    });
    renderPanel();
    expect(screen.getByRole("heading", { level: 2, name: "Brewfather" })).toBeTruthy();
    const connection = await screen.findByRole("region", { name: "Connection" });
    expect(within(connection).getByText("Connected")).toBeTruthy();
    expect(within(connection).getByRole("button", { name: "Disconnect" })).toBeTruthy();
    expect(within(connection).getByLabelText("Sync interval")).toBeTruthy();
    expect(within(connection).getByRole("button", { name: "Sync now" })).toBeTruthy();
    const batches = screen.getByRole("region", { name: "Batches" });
    expect(await within(batches).findByRole("checkbox", { name: "Import Pale Ale" })).toBeTruthy();
    expect(within(batches).getByRole("button", { name: "Refresh" })).toBeTruthy();
    expect(within(batches).getByRole("button", { name: "Import selected" })).toBeTruthy();
  });

  it("shows no Batches card when not connected", async () => {
    stubApi({ "GET /api/brewfather": { status: 200, body: brewfatherStatus() } });
    renderPanel();
    expect(await screen.findByRole("region", { name: "Connection" })).toBeTruthy();
    expect(screen.queryByRole("region", { name: "Batches" })).toBeNull();
  });

  it("says import is not set up, with no form, when the server has no encryption key", async () => {
    stubApi({ "GET /api/brewfather": { status: 200, body: brewfatherStatus({ configured: false }) } });
    renderPanel();
    expect(await screen.findByText(/BEER_MENU_SECRET_KEY_FILE/)).toBeTruthy();
    expect(screen.queryByLabelText("API key")).toBeNull();
    expect(screen.queryByLabelText("Sync interval")).toBeNull();
  });

  it("shows an empty connect form with a password field for the key when not connected", async () => {
    stubApi({ "GET /api/brewfather": { status: 200, body: brewfatherStatus() } });
    renderPanel();
    expect(await screen.findByText("Not connected")).toBeTruthy();
    expect(keyField().type).toBe("password");
    expect(keyField().value).toBe("");
    expect((screen.getByLabelText("User ID") as HTMLInputElement).value).toBe("");
    expect(screen.queryByRole("button", { name: "Disconnect" })).toBeNull();
  });

  it("connects with both values and clears the key field", async () => {
    const calls = stubApi({
      "GET /api/brewfather": { status: 200, body: brewfatherStatus() },
      "PUT /api/brewfather/connection": { status: 200, body: connected },
      "GET /api/brewfather/batches": noBatches,
    });
    renderPanel();
    await screen.findByText("Not connected");
    const key = fakeKey();
    fillForm(key);
    fireEvent.click(screen.getByRole("button", { name: "Connect" }));
    expect(await screen.findByText("Connected")).toBeTruthy();
    expect(calls.find((c) => c.key === "PUT /api/brewfather/connection")?.body).toEqual({ user_id: "user-1", api_key: key });
    expect(screen.queryByLabelText("API key")).toBeNull();
  });

  it.each([
    ["credentials_refused", 400, "Brewfather refused this user ID and API key."],
    ["no_batch_access", 400, "This API key cannot read batches. Give it batch read access in Brewfather."],
    ["provider_unreachable", 502, "Brewfather could not be reached; try again."],
    ["provider_rate_limited", 429, "Brewfather's rate limit is reached. Try again later."],
  ])("shows %s as a plain message and clears the key field", async (code, status, message) => {
    stubApi({
      "GET /api/brewfather": { status: 200, body: brewfatherStatus() },
      "PUT /api/brewfather/connection": { status, body: { error: code, message } },
    });
    renderPanel();
    await screen.findByText("Not connected");
    fillForm(fakeKey());
    fireEvent.click(screen.getByRole("button", { name: "Connect" }));
    expect((await screen.findByRole("alert")).textContent).toBe(message);
    expect(keyField().value).toBe("");
    expect((screen.getByLabelText("User ID") as HTMLInputElement).value).toBe("user-1");
  });

  it("disconnects only after confirmation", async () => {
    const routes = {
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": noBatches,
      "DELETE /api/brewfather/connection": { status: 204 },
    };
    const calls = stubApi(routes);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Disconnect" }));
    expect(confirm).toHaveBeenCalled();
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/brewfather/connection");

    confirm.mockReturnValue(true);
    routes["GET /api/brewfather"] = { status: 200, body: brewfatherStatus() };
    fireEvent.click(screen.getByRole("button", { name: "Disconnect" }));
    expect(await screen.findByText("Not connected")).toBeTruthy();
    expect(calls.map((c) => c.key)).toContain("DELETE /api/brewfather/connection");
    expect(screen.getByLabelText("API key")).toBeTruthy();
  });

  it("asks to reconnect with the form when the stored key no longer decrypts", async () => {
    const calls = stubApi({
      "GET /api/brewfather": { status: 200, body: brewfatherStatus({ connected: true, reconnect_needed: true }) },
    });
    renderPanel();
    expect(await screen.findByText("Reconnect needed")).toBeTruthy();
    expect(keyField().type).toBe("password");
    expect(screen.queryByRole("button", { name: "Sync now" })).toBeNull();
    expect(calls.map((c) => c.key)).toEqual(["GET /api/brewfather"]);
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "GET /api/brewfather": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    renderPanel({ onSignedOut });
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});

describe("BrewfatherPanel batch list", () => {
  it("is not shown or loaded when not connected", async () => {
    const calls = stubApi({ "GET /api/brewfather": { status: 200, body: brewfatherStatus() } });
    renderPanel();
    await screen.findByText("Not connected");
    expect(screen.queryByRole("button", { name: "Import selected" })).toBeNull();
    expect(calls.map((c) => c.key)).toEqual(["GET /api/brewfather"]);
  });

  it("lists batches with name, number, status and brew date, imported ones by beer name", async () => {
    stubApi({
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": {
        status: 200,
        body: {
          fetched_at: "2026-11-09T12:00:00Z",
          batches: [
            batch({ id: "b1", name: "Pale Ale", number: 12, status: "fermenting", brew_date: "2026-11-01" }),
            batch({ id: "b2", name: "Stout batch", number: 11, status: "ready", beer_id: 7 }),
            batch({ id: "b3", name: "Old batch", number: 10, beer_id: 8 }),
          ],
        },
      },
      "GET /api/beers": { status: 200, body: [beer({ id: 7, name: "House Stout" })] },
      "GET /api/beers?archived=true": { status: 200, body: [beer({ id: 8, name: "Old Ale" })] },
    });
    renderPanel();
    await screen.findByText("Pale Ale");
    expect(within(row("Pale Ale")).getByText("12")).toBeTruthy();
    expect(within(row("Pale Ale")).getByText("fermenting")).toBeTruthy();
    expect(within(row("Pale Ale")).getByText("2026-11-01")).toBeTruthy();
    expect(within(row("Pale Ale")).getByRole("checkbox")).toBeTruthy();
    expect(await within(row("Stout batch")).findByText("Imported as House Stout")).toBeTruthy();
    expect(within(row("Stout batch")).queryByRole("checkbox")).toBeNull();
    expect(within(row("Old batch")).getByText("Imported as Old Ale")).toBeTruthy();
  });

  it("refreshes from Brewfather on request", async () => {
    const calls = stubApi({
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": noBatches,
      "GET /api/brewfather/batches?refresh=true": {
        status: 200,
        body: { fetched_at: "2026-11-09T12:05:00Z", batches: [batch({ name: "New batch" })] },
      },
    });
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Refresh" }));
    expect(await screen.findByText("New batch")).toBeTruthy();
    expect(calls.map((c) => c.key)).toContain("GET /api/brewfather/batches?refresh=true");
  });

  it("imports the checked batches, shows each result and reports the beers changed", async () => {
    const routes = {
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": {
        status: 200,
        body: {
          fetched_at: "2026-11-09T12:00:00Z",
          batches: [batch({ id: "b1", name: "Pale Ale" }), batch({ id: "b2", name: "IPA" }), batch({ id: "b3", name: "Lager" })],
        },
      },
      "POST /api/brewfather/imports": {
        status: 200,
        body: {
          results: [
            importResult({ batch_id: "b1", beer: beer({ id: 5, name: "Pale Ale", source: "brewfather" }), adjusted: ["abv"] }),
            importResult({ batch_id: "b3", outcome: "archived", beer: null, message: "Archived in Brewfather." }),
          ],
        },
      },
      "GET /api/beers": { status: 200, body: [beer({ id: 5, name: "Pale Ale" })] },
      "GET /api/beers?archived=true": { status: 200, body: [] },
    };
    const calls = stubApi(routes);
    renderPanel();
    await screen.findByText("Lager");
    const importButton = screen.getByRole("button", { name: "Import selected" }) as HTMLButtonElement;
    expect(importButton.disabled).toBe(true);
    fireEvent.click(within(row("Pale Ale")).getByRole("checkbox"));
    fireEvent.click(within(row("Lager")).getByRole("checkbox"));
    routes["GET /api/brewfather/batches"] = {
      status: 200,
      body: {
        fetched_at: "2026-11-09T12:00:00Z",
        batches: [batch({ id: "b1", name: "Pale Ale", beer_id: 5 }), batch({ id: "b2", name: "IPA" })],
      },
    };
    fireEvent.click(importButton);
    const results = await screen.findByRole("list", { name: "Import results" });
    expect(calls.find((c) => c.key === "POST /api/brewfather/imports")?.body).toEqual({ batch_ids: ["b1", "b3"] });
    expect(within(results).getByText(/Pale Ale: Imported as Pale Ale/).textContent).toContain("Adjusted: abv");
    expect(within(results).getByText(/Lager: Archived in Brewfather/).textContent).toContain("Archived in Brewfather.");
    expect(await within(row("IPA")).findByRole("checkbox")).toBeTruthy();
    expect(await screen.findByText("Imported as Pale Ale")).toBeTruthy();
  });

  it("shows the retry time when Brewfather's rate limit is reached", async () => {
    stubApi({
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": {
        status: 429,
        body: { error: "provider_rate_limited", message: "x", retry_at: "2026-11-09T18:42:00Z" },
        headers: { "Retry-After": "600" },
      },
    });
    const before = Date.now();
    renderPanel();
    const text = (await screen.findByRole("alert")).textContent ?? "";
    const at = (ms: number) => new Date(ms).toLocaleTimeString();
    expect([at(before + 600_000), at(Date.now() + 600_000)]).toContain(text.replace(/^.*Try again after (.*)\.$/, "$1"));
  });
});

describe("BrewfatherPanel schedule and sync", () => {
  const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

  it("shows the interval and saves a change", async () => {
    const calls = stubApi({
      "GET /api/brewfather": { status: 200, body: brewfatherStatus({ interval: "1h" }) },
      "PUT /api/brewfather/schedule": { status: 200, body: brewfatherStatus({ interval: "6h" }) },
    });
    renderPanel();
    const select = (await screen.findByLabelText("Sync interval")) as HTMLSelectElement;
    expect(select.value).toBe("1h");
    fireEvent.change(select, { target: { value: "6h" } });
    await waitFor(() => expect(calls.find((c) => c.key === "PUT /api/brewfather/schedule")?.body).toEqual({ interval: "6h" }));
    await waitFor(() => expect(select.value).toBe("6h"));
  });

  it("syncs now, polls every 2 s while running, then shows the result and reports the beers changed", async () => {
    vi.useFakeTimers();
    const lastSync = {
      started_at: "2026-11-09T18:00:00Z",
      finished_at: "2026-11-09T18:00:03Z",
      ok: true,
      message: "Updated 4 beers, imported 1.",
      updated: 4,
      imported: 1,
      not_found: 2,
    };
    const routes = {
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": noBatches,
      "POST /api/brewfather/sync": { status: 202, body: { ...connected, sync_running: true } },
    };
    const calls = stubApi(routes);
    renderPanel();
    await tick(0);
    fireEvent.click(screen.getByRole("button", { name: "Sync now" }));
    await tick(0);
    expect(screen.getByText("Sync running…")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Sync now" }) as HTMLButtonElement).disabled).toBe(true);
    const polls = () => calls.filter((c) => c.key === "GET /api/brewfather").length;
    expect(polls()).toBe(1);

    routes["GET /api/brewfather"] = { status: 200, body: { ...connected, sync_running: true } };
    await tick(2000);
    expect(polls()).toBe(2);
    await tick(2000);
    expect(polls()).toBe(3);

    routes["GET /api/brewfather"] = { status: 200, body: { ...connected, last_sync: lastSync } };
    await tick(2000);
    expect(polls()).toBe(4);
    expect(screen.queryByText("Sync running…")).toBeNull();
    const shown = screen.getByText(/Last sync/).textContent ?? "";
    expect(shown).toContain(new Date(lastSync.finished_at).toLocaleString());
    expect(shown).toContain("Updated 4 beers, imported 1.");
    expect(shown).toContain("Updated 4, imported 1, not found 2.");

    await tick(10_000);
    expect(polls()).toBe(4);
  });

  it("follows a sync that was already running when Sync now gets 409 sync_running", async () => {
    vi.useFakeTimers();
    const lastSync = {
      started_at: "2026-11-09T18:00:00Z",
      finished_at: "2026-11-09T18:00:03Z",
      ok: true,
      message: "Updated 2 beers.",
      updated: 2,
      imported: 0,
      not_found: 0,
    };
    const routes: Record<string, { status: number; body: unknown }> = {
      "GET /api/brewfather": { status: 200, body: connected },
      "GET /api/brewfather/batches": noBatches,
      "POST /api/brewfather/sync": { status: 409, body: { error: "sync_running", message: "A sync is already running." } },
    };
    const calls = stubApi(routes);
    renderPanel();
    await tick(0);
    routes["GET /api/brewfather"] = { status: 200, body: { ...connected, sync_running: true } };
    fireEvent.click(screen.getByRole("button", { name: "Sync now" }));
    await tick(0);
    const polls = () => calls.filter((c) => c.key === "GET /api/brewfather").length;
    expect(polls()).toBe(2);
    expect(screen.getByText("Sync running…")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();

    await tick(2000);
    expect(polls()).toBe(3);
    routes["GET /api/brewfather"] = { status: 200, body: { ...connected, last_sync: lastSync } };
    await tick(2000);
    expect(polls()).toBe(4);
    expect(screen.getByText(/Last sync/).textContent).toContain("Updated 2 beers.");
  });

  it("shows a failed last sync's message", async () => {
    stubApi({
      "GET /api/brewfather": {
        status: 200,
        body: brewfatherStatus({
          connected: true,
          last_sync: {
            started_at: "2026-11-09T18:00:00Z",
            finished_at: "2026-11-09T18:00:15Z",
            ok: false,
            message: "Brewfather could not be reached.",
            updated: 0,
            imported: 0,
            not_found: 0,
          },
        }),
      },
      "GET /api/brewfather/batches": noBatches,
    });
    renderPanel();
    const shown = (await screen.findByText(/Last sync failed/)).textContent ?? "";
    expect(shown).toContain("Brewfather could not be reached.");
  });

  it("keeps polling after a failed status poll", async () => {
    vi.useFakeTimers();
    const routes: Record<string, { status: number; body: unknown }> = {
      "GET /api/brewfather": { status: 200, body: { ...connected, sync_running: true } },
      "GET /api/brewfather/batches": noBatches,
    };
    stubApi(routes);
    renderPanel();
    await tick(0);
    routes["GET /api/brewfather"] = { status: 502, body: { error: "unknown", message: "Bad gateway." } };
    await tick(2000);
    expect(screen.getByRole("alert").textContent).toBe("Bad gateway.");
    routes["GET /api/brewfather"] = { status: 200, body: connected };
    await tick(2000);
    expect(screen.queryByText("Sync running…")).toBeNull();
  });

  it("stops polling on unmount", async () => {
    vi.useFakeTimers();
    const calls = stubApi({
      "GET /api/brewfather": { status: 200, body: { ...connected, sync_running: true } },
      "GET /api/brewfather/batches": noBatches,
    });
    const { unmount } = renderPanel();
    await tick(0);
    await tick(2000);
    const polls = () => calls.filter((c) => c.key === "GET /api/brewfather").length;
    expect(polls()).toBe(2);
    unmount();
    await tick(10_000);
    expect(polls()).toBe(2);
  });
});
