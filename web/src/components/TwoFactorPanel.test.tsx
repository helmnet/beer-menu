import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { passkey, stubApi, testPassword, twoFactorState } from "../testApi";
import TwoFactorPanel from "./TwoFactorPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("TwoFactorPanel", () => {
  it("shows that two-factor sign-in is off", async () => {
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState() } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 2, name: "Two-Factor Sign-In" })).toBeTruthy();
    expect(await screen.findByText("Two-factor sign-in is off.")).toBeTruthy();
    expect(screen.queryByText(/required for every admin/)).toBeNull();
    expect(screen.queryByText(/Recovery codes left/)).toBeNull();
  });

  it("shows on, required and the recovery codes left", async () => {
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({
          enabled: true,
          required: true,
          totp: { created_at: "2026-11-10T12:00:00Z", last_used_at: null },
          recovery_codes_left: 7,
        }),
      },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect(await screen.findByText("Two-factor sign-in is on.")).toBeTruthy();
    expect(screen.getByText("Two-factor sign-in is required for every admin.")).toBeTruthy();
    expect(screen.getByText("Recovery codes left: 7 of 10.")).toBeTruthy();
  });

  it("lists each factor with its type, name, added and last-used time", async () => {
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({
          enabled: true,
          passkeys: [passkey(), passkey({ id: 2, name: "Laptop", last_used_at: "2026-11-11T12:00:00Z" })],
          recovery_codes_left: 10,
        }),
      },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const list = await screen.findByRole("list", { name: "Second factors" });
    const items = Array.from(list.querySelectorAll("li")).map((li) => li.textContent);
    expect(items).toHaveLength(2);
    expect(items[0]).toContain("Passkey");
    expect(items[0]).toContain("Phone");
    expect(items[0]).toContain("added");
    expect(items[0]).toContain("never used");
    expect(items[1]).toContain("Laptop");
    expect(items[1]).toContain("last used");
    expect(items[1]).not.toContain("never used");
  });

  it("shows a load error, and reports a 401 as signed out", async () => {
    stubApi({ "GET /api/2fa": { status: 500, body: { error: "internal", message: "Internal error." } } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect((await screen.findByRole("alert")).textContent).toBe("Internal error.");
    cleanup();
    stubApi({ "GET /api/2fa": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    render(<TwoFactorPanel onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("adds an authenticator and reloads the state", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": { status: 200, body: twoFactorState() },
      "POST /api/2fa/totp/setup": { status: 200, body: { secret: "JBSWY3DPEHPK3PXP", uri: "otpauth://totp/x" } },
      "POST /api/2fa/totp": { status: 201, body: {} },
    };
    stubApi(routes);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Add authenticator app" }));
    fireEvent.change(await screen.findByLabelText("Authenticator code"), { target: { value: "123456" } });
    routes["GET /api/2fa"] = {
      status: 200,
      body: twoFactorState({ enabled: true, totp: { created_at: "2026-11-10T12:00:00Z", last_used_at: null } }),
    };
    fireEvent.click(screen.getByRole("button", { name: "Verify" }));
    expect(await screen.findByText("Two-factor sign-in is on.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Add authenticator app" })).toBeNull();
  });

  const confirmRequired = { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } };
  const totpOn = { created_at: "2026-11-10T12:00:00Z", last_used_at: "2026-11-11T09:00:00Z" };

  // confirmWithCode fills the "Confirm it is you" dialog with a password and authenticator code.
  async function confirmWithCode() {
    const dialog = await screen.findByRole("dialog", { name: "Confirm it is you" });
    fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.change(within(dialog).getByLabelText("Authenticator code"), { target: { value: "123456" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
  }

  it("lists the authenticator app", async () => {
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const list = await screen.findByRole("list", { name: "Second factors" });
    expect(list.textContent).toContain("Authenticator app");
    expect(list.textContent).toContain("last used");
  });

  it("warns when 2 or fewer recovery codes are left", async () => {
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 3 }) } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect(await screen.findByText("Recovery codes left: 3 of 10.")).toBeTruthy();
    expect(screen.queryByText(/Make new recovery codes soon/)).toBeNull();
    cleanup();
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 2 }) } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect(await screen.findByText("Only 2 recovery codes left. Make new recovery codes soon.")).toBeTruthy();
  });

  it("makes new recovery codes after confirming and shows them once", async () => {
    const codes = Array.from({ length: 10 }, (_, i) => `abcd${"234567abcd"[i]}-efghi`);
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 1 }) },
      "POST /api/2fa/recovery-codes": confirmRequired,
      "POST /api/2fa/confirm": { status: 204 },
    };
    const calls = stubApi(routes);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "New recovery codes" }));
    routes["POST /api/2fa/recovery-codes"] = { status: 200, body: { recovery_codes: codes } };
    await confirmWithCode();
    const list = await screen.findByRole("list", { name: "Recovery codes" });
    expect(list.querySelectorAll("li")).toHaveLength(10);
    expect(calls.filter((c) => c.key === "POST /api/2fa/recovery-codes")).toHaveLength(2);
    routes["GET /api/2fa"] = { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) };
    fireEvent.click(screen.getByLabelText("I saved these codes"));
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(await screen.findByText("Recovery codes left: 10 of 10.")).toBeTruthy();
    expect(screen.queryByRole("list", { name: "Recovery codes" })).toBeNull();
  });

  it("renames a passkey inline", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, passkeys: [passkey()], recovery_codes_left: 10 }) },
      "PATCH /api/2fa/passkeys/1": { status: 200, body: passkey({ name: "Work phone" }) },
    };
    const calls = stubApi(routes);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Rename" }));
    const input = screen.getByLabelText("Passkey name") as HTMLInputElement;
    expect(input.value).toBe("Phone");
    fireEvent.change(input, { target: { value: "Work phone" } });
    routes["GET /api/2fa"] = {
      status: 200,
      body: twoFactorState({ enabled: true, passkeys: [passkey({ name: "Work phone" })], recovery_codes_left: 10 }),
    };
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText(/Work phone/)).toBeTruthy();
    expect(screen.queryByLabelText("Passkey name")).toBeNull();
    expect(calls.find((c) => c.key === "PATCH /api/2fa/passkeys/1")?.body).toEqual({ name: "Work phone" });
  });

  it("keeps the rename open with the API's message", async () => {
    stubApi({
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, passkeys: [passkey()], recovery_codes_left: 10 }) },
      "PATCH /api/2fa/passkeys/1": { status: 400, body: { error: "invalid_request", message: "Name the passkey." } },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Rename" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Name the passkey.");
    expect(screen.getByLabelText("Passkey name")).toBeTruthy();
  });

  it("removes a factor after asking, confirming identity when needed", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({ enabled: true, totp: totpOn, passkeys: [passkey()], recovery_codes_left: 10 }),
      },
      "DELETE /api/2fa/passkeys/1": confirmRequired,
      "POST /api/2fa/confirm": { status: 204 },
    };
    const calls = stubApi(routes);
    const ask = vi.fn(() => false);
    vi.stubGlobal("confirm", ask);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const row = (await screen.findByText(/Phone/)).closest("li") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    expect(ask).toHaveBeenCalled();
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/2fa/passkeys/1");

    ask.mockReturnValue(true);
    fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    routes["DELETE /api/2fa/passkeys/1"] = { status: 204 };
    routes["GET /api/2fa"] = { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) };
    await confirmWithCode();
    await waitFor(() => expect(screen.queryByText(/Phone/)).toBeNull());
    expect(calls.filter((c) => c.key === "DELETE /api/2fa/passkeys/1")).toHaveLength(2);
    expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
  });

  it("removes the authenticator app", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) },
      "DELETE /api/2fa/totp": { status: 204 },
    };
    stubApi(routes);
    vi.stubGlobal("confirm", () => true);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const row = (await screen.findByText(/Authenticator app/)).closest("li") as HTMLElement;
    routes["GET /api/2fa"] = { status: 200, body: twoFactorState() };
    fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    expect(await screen.findByText("Two-factor sign-in is off.")).toBeTruthy();
  });

  it("turns 2FA off after asking and refreshes to the off state", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({ enabled: true, totp: totpOn, passkeys: [passkey()], recovery_codes_left: 10 }),
      },
      "DELETE /api/2fa": { status: 204 },
    };
    const calls = stubApi(routes);
    const ask = vi.fn(() => false);
    vi.stubGlobal("confirm", ask);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("button", { name: "Turn off 2FA" }));
    expect(calls.map((c) => c.key)).not.toContain("DELETE /api/2fa");
    ask.mockReturnValue(true);
    routes["GET /api/2fa"] = { status: 200, body: twoFactorState() };
    fireEvent.click(screen.getByRole("button", { name: "Turn off 2FA" }));
    expect(await screen.findByText("Two-factor sign-in is off.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Turn off 2FA" })).toBeNull();
    expect(screen.queryByRole("list", { name: "Second factors" })).toBeNull();
  });

  it("reports a 401 from a factor action as signed out", async () => {
    stubApi({
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) },
      "DELETE /api/2fa": { status: 401, body: { error: "unauthenticated", message: "x" } },
    });
    vi.stubGlobal("confirm", () => true);
    const onSignedOut = vi.fn();
    render(<TwoFactorPanel onSignedOut={onSignedOut} />);
    fireEvent.click(await screen.findByRole("button", { name: "Turn off 2FA" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});
