import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, testPassword } from "../testApi";
import AdminsPanel from "./AdminsPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const second = { id: 2, username: "second", created_at: "2026-11-08T13:00:00Z" };

function removeButton(username: string) {
  const row = screen.getByText(username).closest("li") as HTMLElement;
  return within(row).getByRole("button", { name: "Remove" });
}

describe("AdminsPanel", () => {
  it("lists admins and adds one", async () => {
    const routes = {
      "GET /api/admins": { status: 200, body: [owner] as unknown[] },
      "POST /api/admins": { status: 201, body: second },
    };
    const calls = stubApi(routes);
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    const pw = testPassword();
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "second" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: pw } });
    routes["GET /api/admins"] = { status: 200, body: [owner, second] };
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    expect(await screen.findByText("second")).toBeTruthy();
    expect(calls.find((c) => c.key === "POST /api/admins")?.body).toEqual({ username: "second", password: pw });
  });

  it("shows the server message when the username is taken", async () => {
    stubApi({
      "GET /api/admins": { status: 200, body: [owner] },
      "POST /api/admins": {
        status: 409,
        body: { error: "username_taken", message: "That username is already in use." },
      },
    });
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    fireEvent.change(screen.getByLabelText("Username"), { target: { value: "OWNER" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    expect((await screen.findByRole("alert")).textContent).toBe("That username is already in use.");
  });

  it("removes another admin after confirmation", async () => {
    const routes = {
      "GET /api/admins": { status: 200, body: [owner, second] as unknown[] },
      "DELETE /api/admins/2": { status: 204 },
    };
    const calls = stubApi(routes);
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    const onSignedOut = vi.fn();
    render(<AdminsPanel me={owner} onSignedOut={onSignedOut} />);
    await screen.findByText("second");
    fireEvent.click(removeButton("second"));
    expect(confirm).toHaveBeenCalled();
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/admins/2");

    confirm.mockReturnValue(true);
    routes["GET /api/admins"] = { status: 200, body: [owner] };
    fireEvent.click(removeButton("second"));
    await waitFor(() => expect(screen.queryByText("second")).toBeNull());
    expect(onSignedOut).not.toHaveBeenCalled();
  });

  it("shows the last-admin message", async () => {
    stubApi({
      "GET /api/admins": { status: 200, body: [owner] },
      "DELETE /api/admins/1": {
        status: 409,
        body: { error: "last_admin", message: "The last admin cannot be removed." },
      },
    });
    vi.stubGlobal("confirm", () => true);
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    fireEvent.click(removeButton("owner"));
    expect((await screen.findByRole("alert")).textContent).toBe("The last admin cannot be removed.");
  });

  it("signs out after removing yourself", async () => {
    stubApi({
      "GET /api/admins": { status: 200, body: [owner, second] },
      "DELETE /api/admins/1": { status: 204 },
    });
    vi.stubGlobal("confirm", () => true);
    const onSignedOut = vi.fn();
    render(<AdminsPanel me={owner} onSignedOut={onSignedOut} />);
    await screen.findByText("owner");
    fireEvent.click(removeButton("owner"));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "GET /api/admins": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    render(<AdminsPanel me={owner} onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});
