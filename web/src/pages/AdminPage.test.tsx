import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, testPassword } from "../testApi";
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
  "GET /api/admins": { status: 200, body: [] },
  "GET /api/tokens": { status: 200, body: [] },
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
