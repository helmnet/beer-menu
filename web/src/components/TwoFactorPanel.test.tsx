import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { passkey, stubApi, testPassword, twoFactorState } from "../testApi";
import TwoFactorPanel from "./TwoFactorPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const requireLabel = "Require two-factor sign-in for all admins";

describe("TwoFactorPanel", () => {
  it("shows that two-factor sign-in is off", async () => {
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState() } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 2, name: "Two-Factor Sign-In" })).toBeTruthy();
    expect(await screen.findByText("Two-factor sign-in is off.")).toBeTruthy();
    expect((screen.getByRole("checkbox", { name: requireLabel }) as HTMLInputElement).checked).toBe(false);
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
    expect((screen.getByRole("checkbox", { name: requireLabel }) as HTMLInputElement).checked).toBe(true);
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

  it("marks the destructive actions as danger buttons, not Rename", async () => {
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({
          enabled: true,
          totp: totpOn,
          passkeys: [passkey(), passkey({ id: 2, name: "Laptop" })],
          recovery_codes_left: 10,
        }),
      },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const removes = await screen.findAllByRole("button", { name: "Remove" });
    expect(removes).toHaveLength(3);
    for (const button of [
      ...removes,
      screen.getByRole("button", { name: "Turn off 2FA" }),
      screen.getByRole("button", { name: "New recovery codes" }),
    ]) {
      expect(button.classList.contains("btn-danger")).toBe(true);
    }
    for (const button of screen.getAllByRole("button", { name: "Rename" })) {
      expect(button.classList.contains("btn-danger")).toBe(false);
    }
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

  it("disables the require switch with a hint when the caller has no factor", async () => {
    stubApi({ "GET /api/2fa": { status: 200, body: twoFactorState() } });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const box = (await screen.findByRole("checkbox", { name: requireLabel })) as HTMLInputElement;
    expect(box.disabled).toBe(true);
    expect(screen.getByText("Add a second factor for yourself before requiring one for every admin.")).toBeTruthy();
  });

  it("turns the requirement on after confirming identity", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) },
      "PUT /api/2fa/required": confirmRequired,
      "POST /api/2fa/confirm": { status: 204 },
    };
    const calls = stubApi(routes);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const box = (await screen.findByRole("checkbox", { name: requireLabel })) as HTMLInputElement;
    expect(box.disabled).toBe(false);
    expect(box.checked).toBe(false);
    fireEvent.click(box);
    routes["PUT /api/2fa/required"] = { status: 204 };
    routes["GET /api/2fa"] = {
      status: 200,
      body: twoFactorState({ enabled: true, required: true, totp: totpOn, recovery_codes_left: 10 }),
    };
    await confirmWithCode();
    await waitFor(() => expect((screen.getByRole("checkbox", { name: requireLabel }) as HTMLInputElement).checked).toBe(true));
    const puts = calls.filter((c) => c.key === "PUT /api/2fa/required");
    expect(puts.map((c) => c.body)).toEqual([{ required: true }, { required: true }]);
  });

  it("disables turning the requirement on when the server can set up no factor", async () => {
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({
          enabled: true,
          totp: totpOn,
          recovery_codes_left: 10,
          totp_available: false,
          passkeys_available: false,
          passkeys_origin: null,
        }),
      },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const box = (await screen.findByRole("checkbox", { name: requireLabel })) as HTMLInputElement;
    expect(box.disabled).toBe(true);
    expect(screen.getByText(/no encryption key and no public URL/)).toBeTruthy();
  });

  it("still lets the requirement go off when the server can set up no factor", async () => {
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({
          enabled: true,
          required: true,
          totp: totpOn,
          recovery_codes_left: 10,
          totp_available: false,
          passkeys_available: false,
          passkeys_origin: null,
        }),
      },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const box = (await screen.findByRole("checkbox", { name: requireLabel })) as HTMLInputElement;
    expect(box.disabled).toBe(false);
    expect(screen.queryByText(/no encryption key and no public URL/)).toBeNull();
  });

  it("shows the API's message when the requirement cannot go on", async () => {
    const message = "No admin could set up two-factor sign-in: the server has no encryption key and no public URL.";
    stubApi({
      "GET /api/2fa": { status: 200, body: twoFactorState({ enabled: true, totp: totpOn, recovery_codes_left: 10 }) },
      "PUT /api/2fa/required": { status: 409, body: { error: "two_factor_unavailable", message } },
    });
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    fireEvent.click(await screen.findByRole("checkbox", { name: requireLabel }));
    expect((await screen.findByRole("alert")).textContent).toContain(message);
  });

  it("names the require setting when the last factor cannot go", async () => {
    const required = { status: 409, body: { error: "two_factor_required", message: "x" } };
    stubApi({
      "GET /api/2fa": {
        status: 200,
        body: twoFactorState({ enabled: true, required: true, totp: totpOn, recovery_codes_left: 10 }),
      },
      "DELETE /api/2fa/totp": required,
      "DELETE /api/2fa": required,
    });
    vi.stubGlobal("confirm", () => true);
    render(<TwoFactorPanel onSignedOut={vi.fn()} />);
    const row = (await screen.findByText(/Authenticator app/)).closest("li") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: "Remove" }));
    expect((await screen.findByRole("alert")).textContent).toContain(`"${requireLabel}" is on`);
    fireEvent.click(screen.getByRole("button", { name: "Turn off 2FA" }));
    await waitFor(() => expect(screen.getAllByRole("alert")).toHaveLength(1));
    expect(screen.getByRole("alert").textContent).toContain(`"${requireLabel}" is on`);
    expect(screen.getByText(/Authenticator app/)).toBeTruthy();
  });
});
