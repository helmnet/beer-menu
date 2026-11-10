import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { batch, brewfatherStatus, importResult, keg, owner, stubApi, tap, testPassword } from "../testApi";
import AdminPage from "./AdminPage";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const unauthenticated = {
  status: 401,
  body: { error: "unauthenticated", message: "Sign in or send a valid API token." },
};

// The signed-in view's panels load these on mount.
const panels = {
  "GET /api/taps": { status: 200, body: [] },
  "GET /api/kegs?state=waiting": { status: 200, body: [] },
  "GET /api/kegs?state=kicked": { status: 200, body: [] },
  "GET /api/keg-types": { status: 200, body: [] },
  "GET /api/beers": { status: 200, body: [] },
  "GET /api/admins": { status: 200, body: [] },
  "GET /api/tokens": { status: 200, body: [] },
  "GET /api/brewfather": { status: 200, body: brewfatherStatus() },
};

function fillCredentials(password: string) {
  fireEvent.change(screen.getByLabelText("Username"), { target: { value: "owner" } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
}

describe("AdminPage", () => {
  it("shows setup when needed, then the signed-in view", async () => {
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: true } },
      "POST /api/setup": { status: 201, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Create admin" });
    const pw = testPassword();
    fillCredentials(pw);
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: pw } });
    fireEvent.click(screen.getByRole("button", { name: "Create admin" }));
    expect(await screen.findByText("Signed in as owner")).toBeTruthy();
    expect(calls.map((c) => c.key).slice(0, 2)).toEqual(["GET /api/setup", "POST /api/setup"]);
  });

  it("switches to sign-in when setup was already done", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: true } },
      "POST /api/setup": { status: 409, body: { error: "setup_done", message: "done" } },
    });
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Create admin" });
    const pw = testPassword();
    fillCredentials(pw);
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: pw } });
    fireEvent.click(screen.getByRole("button", { name: "Create admin" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("shows sign-in without a session, then signs in and out", async () => {
    const routes = {
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": unauthenticated,
      "POST /api/session": { status: 200, body: owner },
      "DELETE /api/session": { status: 204 },
      ...panels,
    };
    const calls = stubApi(routes);
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Sign in" });
    fillCredentials(testPassword());
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByText("Signed in as owner")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(calls.map((c) => c.key)).toContain("DELETE /api/session");
  });

  it("shows the signed-in view for an existing session", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    expect(await screen.findByText("Signed in as owner")).toBeTruthy();
  });

  it("shows the beer library when signed in", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { name: "Beer library" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Add beer" })).toBeTruthy();
  });

  it("shows the taps and keg history when signed in", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { name: "Taps" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Keg history" })).toBeTruthy();
  });

  it("reloads the keg history after a kick", async () => {
    const onTap = keg({ id: 10, beer_name: "Stout", tap_id: 1, state: "on_tap" });
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
      "GET /api/taps": { status: 200, body: [tap({ keg: onTap })] },
      "PATCH /api/kegs/10": { status: 200, body: { ...onTap, state: "kicked" } },
    });
    vi.stubGlobal("confirm", () => true);
    render(<AdminPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Kick" }));
    const histories = () => calls.filter((c) => c.key === "GET /api/kegs?state=kicked").length;
    await waitFor(() => expect(histories()).toBe(2));
  });

  it("reloads the beer library after a keg goes on a tap", async () => {
    const waiting = keg({ id: 11, beer_name: "Stout" });
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
      "GET /api/taps": { status: 200, body: [tap()] },
      "GET /api/kegs?state=waiting": { status: 200, body: [waiting] },
      "PATCH /api/kegs/11": { status: 200, body: { ...waiting, tap_id: 1, state: "on_tap" } },
    });
    render(<AdminPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Put on tap" }));
    const libraries = () => calls.filter((c) => c.key === "GET /api/beers").length;
    await waitFor(() => expect(libraries()).toBe(2));
  });

  it("shows the Brewfather panel when signed in", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { name: "Brewfather" })).toBeTruthy();
    expect(await screen.findByText("Not connected")).toBeTruthy();
  });

  it("reloads the beer library after a Brewfather import", async () => {
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
      "GET /api/brewfather": { status: 200, body: brewfatherStatus({ connected: true }) },
      "GET /api/brewfather/batches": { status: 200, body: { fetched_at: "2026-11-09T12:00:00Z", batches: [batch()] } },
      "POST /api/brewfather/imports": { status: 200, body: { results: [importResult()] } },
    });
    render(<AdminPage />);
    fireEvent.click(await screen.findByRole("checkbox", { name: "Import Pale Ale" }));
    const libraries = () => calls.filter((c) => c.key === "GET /api/beers").length;
    expect(libraries()).toBe(1);
    fireEvent.click(screen.getByRole("button", { name: "Import selected" }));
    await waitFor(() => expect(libraries()).toBe(2));
  });

  it.each(Object.keys(panels))("returns to sign-in on a 401 from %s", async (key) => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
      [key]: unauthenticated,
    });
    render(<AdminPage />);
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("returns to sign-in when the session ended elsewhere", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      "DELETE /api/session": unauthenticated,
      ...panels,
    });
    render(<AdminPage />);
    fireEvent.click(await screen.findByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("returns to sign-in on a 401 from the password form", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": { status: 200, body: owner },
      ...panels,
      "POST /api/password": unauthenticated,
    });
    render(<AdminPage />);
    await screen.findByText("Signed in as owner");
    const pw = testPassword();
    fireEvent.change(screen.getByLabelText("Current password"), { target: { value: testPassword() } });
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: pw } });
    fireEvent.change(screen.getByLabelText("Confirm new password"), { target: { value: pw } });
    fireEvent.click(screen.getByRole("button", { name: "Change password" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("shows an error when the API is unreachable", async () => {
    stubApi({ "GET /api/setup": { status: 502 } });
    render(<AdminPage />);
    expect((await screen.findByRole("alert")).textContent).toBe("Request failed (502).");
  });
});
