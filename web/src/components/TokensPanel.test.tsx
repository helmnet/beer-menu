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

  it("shows a new secret once with a copy button", async () => {
    const secret = `bmt_${crypto.randomUUID()}`;
    const routes = {
      "GET /api/tokens": { status: 200, body: [] as unknown[] },
      "POST /api/tokens": { status: 201, body: { ...unused, name: "ha", scope: "write", secret } },
    };
    const calls = stubApi(routes);
    const writeText = vi.fn(async () => {});
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    render(<TokensPanel onSignedOut={vi.fn()} />);
    await screen.findByRole("button", { name: "Create token" });
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "ha" } });
    fireEvent.change(screen.getByLabelText("Scope"), { target: { value: "write" } });
    routes["GET /api/tokens"] = { status: 200, body: [{ ...unused, name: "ha", scope: "write" }] };
    fireEvent.click(screen.getByRole("button", { name: "Create token" }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(secret)).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/tokens")?.body).toEqual({ name: "ha", scope: "write" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Copy" }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(secret));

    fireEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.body.textContent).not.toContain(secret);
    expect(await screen.findByText("ha")).toBeTruthy();
    expect(document.body.textContent).not.toContain(secret);
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
