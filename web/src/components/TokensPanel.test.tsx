import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { stubApi } from "../testApi";
import TokensPanel from "./TokensPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const unused = {
  id: 2,
  name: "script",
  scope: "read",
  created_at: "2026-11-08T12:00:00Z",
  last_used_at: null,
  revoked_at: null,
};
const revoked = {
  id: 1,
  name: "old",
  scope: "write",
  created_at: "2026-11-01T12:00:00Z",
  last_used_at: "2026-11-02T12:00:00Z",
  revoked_at: "2026-11-03T12:00:00Z",
};

function row(name: string) {
  return screen.getByText(name).closest("tr") as HTMLElement;
}

describe("TokensPanel", () => {
  it("lists tokens with scope, times and revoked state", async () => {
    stubApi({ "GET /api/tokens": { status: 200, body: [unused, revoked] } });
    render(<TokensPanel onSignedOut={vi.fn()} />);
    await screen.findByText("script");
    expect(within(row("script")).getByText("read")).toBeTruthy();
    expect(within(row("script")).getByText("never")).toBeTruthy();
    expect(within(row("script")).getByRole("button", { name: "Revoke" })).toBeTruthy();
    expect(within(row("old")).getByText("write")).toBeTruthy();
    expect(within(row("old")).getByText("Revoked")).toBeTruthy();
    expect(within(row("old")).queryByRole("button", { name: "Revoke" })).toBeNull();
  });

  it("opens the same New API token dialog from the top bar slot it is given", async () => {
    stubApi({ "GET /api/tokens": { status: 200, body: [unused] } });
    const slot = document.createElement("div");
    document.body.appendChild(slot);
    render(<TokensPanel onSignedOut={vi.fn()} actionSlot={slot} />);
    await screen.findByText("script");
    expect(screen.getAllByRole("button", { name: "New token" })).toHaveLength(2);
    fireEvent.click(within(slot).getByRole("button", { name: "New token" }));
    expect(screen.getByRole("dialog", { name: "New API token" })).toBeTruthy();
    slot.remove();
  });

  it("creates a token in the New API token dialog and shows its secret once", async () => {
    const secret = `bmt_${crypto.randomUUID()}`;
    const routes = {
      "GET /api/tokens": { status: 200, body: [] as unknown[] },
      "POST /api/tokens": { status: 201, body: { ...unused, name: "ha", scope: "write", secret } },
    };
    const calls = stubApi(routes);
    const writeText = vi.fn(async () => {});
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<TokensPanel onSignedOut={vi.fn()} />);
    expect(screen.queryByLabelText("Name")).toBeNull();
    fireEvent.click(await screen.findByRole("button", { name: "New token" }));
    const dialog = screen.getByRole("dialog", { name: "New API token" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "ha" } });
    fireEvent.change(within(dialog).getByLabelText("Scope"), { target: { value: "write" } });
    routes["GET /api/tokens"] = { status: 200, body: [{ ...unused, name: "ha", scope: "write" }] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Create token" }));

    expect(await within(dialog).findByText(secret)).toBeTruthy();
    expect(within(dialog).queryByLabelText("Name")).toBeNull();
    expect(calls.find((c) => c.key === "POST /api/tokens")?.body).toEqual({ name: "ha", scope: "write" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Copy" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(secret));

    // The secret view's Close; the dialog's icon Close (last) does the same.
    const closes = within(dialog).getAllByRole("button", { name: "Close" });
    expect(closes).toHaveLength(2);
    fireEvent.click(closes[0]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.body.textContent).not.toContain(secret);
    expect(await screen.findByText("ha")).toBeTruthy();
    expect(document.body.textContent).not.toContain(secret);

    fireEvent.click(screen.getByRole("button", { name: "New token" }));
    expect(screen.getByRole("dialog", { name: "New API token" }).textContent).not.toContain(secret);
    expect((within(screen.getByRole("dialog")).getByLabelText("Name") as HTMLInputElement).value).toBe("");
  });

  it("forgets the secret when the dialog is closed with Escape", async () => {
    const secret = `bmt_${crypto.randomUUID()}`;
    stubApi({
      "GET /api/tokens": { status: 200, body: [] },
      "POST /api/tokens": { status: 201, body: { ...unused, name: "ha", secret } },
    });
    render(<TokensPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "New token" }));
    const dialog = screen.getByRole("dialog", { name: "New API token" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "ha" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Create token" }));
    await within(dialog).findByText(secret);
    fireEvent(dialog, new Event("cancel", { cancelable: true }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.body.textContent).not.toContain(secret);
  });

  it("never shows a secret whose create answered after the dialog closed", async () => {
    const secret = `bmt_${crypto.randomUUID()}`;
    let answer!: (r: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_path: string, init?: RequestInit) =>
        init?.method === "POST" ? new Promise<Response>((r) => (answer = r)) : new Response("[]", { status: 200 }),
      ),
    );
    render(<TokensPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "New token" }));
    let dialog = screen.getByRole("dialog", { name: "New API token" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: "ha" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Create token" }));
    await waitFor(() => expect(answer).toBeDefined());
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    answer(new Response(JSON.stringify({ ...unused, name: "ha", secret }), { status: 201 }));
    await waitFor(() => expect(vi.mocked(fetch)).toHaveBeenCalledTimes(3));
    fireEvent.click(screen.getByRole("button", { name: "New token" }));
    dialog = screen.getByRole("dialog", { name: "New API token" });
    expect(within(dialog).getByLabelText("Name")).toBeTruthy();
    expect(document.body.textContent).not.toContain(secret);
  });

  it("keeps the form open with the error on a 400", async () => {
    stubApi({
      "GET /api/tokens": { status: 200, body: [] },
      "POST /api/tokens": { status: 400, body: { error: "invalid", message: "Name must be 1 to 100 characters." } },
    });
    render(<TokensPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "New token" }));
    const dialog = screen.getByRole("dialog", { name: "New API token" });
    fireEvent.change(within(dialog).getByLabelText("Name"), { target: { value: " " } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Create token" }));
    expect((await within(dialog).findByRole("alert")).textContent).toBe("Name must be 1 to 100 characters.");
    expect(within(dialog).getByLabelText("Name")).toBeTruthy();
    expect(screen.getByRole("dialog", { name: "New API token" })).toBe(dialog);
  });

  it("revokes after confirmation", async () => {
    const routes = {
      "GET /api/tokens": { status: 200, body: [unused] as unknown[] },
      "DELETE /api/tokens/2": { status: 204 },
    };
    const calls = stubApi(routes);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    render(<TokensPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Revoke" }));
    expect(confirm).toHaveBeenCalled();
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/tokens/2");

    confirm.mockReturnValue(true);
    routes["GET /api/tokens"] = { status: 200, body: [{ ...unused, revoked_at: "2026-11-08T13:00:00Z" }] };
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));
    expect(await screen.findByText("Revoked")).toBeTruthy();
    expect(calls.map((c) => c.key)).toContain("DELETE /api/tokens/2");
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "GET /api/tokens": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    render(<TokensPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});
