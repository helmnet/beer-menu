import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { passkey, stubApi, stubPasskeys, testPassword, twoFactorState, webAuthnOptions } from "../testApi";
import TwoFactorSetup from "./TwoFactorSetup";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const secret = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP";
const setupReply = { status: 200, body: { secret, uri: `otpauth://totp/Beer%20Menu:owner?secret=${secret}&issuer=Beer%20Menu` } };
const codes = Array.from({ length: 10 }, (_, i) => `abcd${"234567abcd"[i]}-efghi`);

function renderSetup(state = twoFactorState()) {
  const props = { onDone: vi.fn(), onSignedOut: vi.fn() };
  render(<TwoFactorSetup state={state} {...props} />);
  return props;
}

const add = () => fireEvent.click(screen.getByRole("button", { name: "Add authenticator app" }));
const enterCode = (code: string) => {
  fireEvent.change(screen.getByLabelText("Authenticator code"), { target: { value: code } });
  fireEvent.click(screen.getByRole("button", { name: "Verify" }));
};

describe("TwoFactorSetup authenticator", () => {
  it("shows the QR code and the key in groups of 4", async () => {
    stubApi({ "POST /api/2fa/totp/setup": setupReply });
    renderSetup();
    add();
    expect(await screen.findByRole("dialog", { name: "Add authenticator app" })).toBeTruthy();
    expect(screen.getByRole("img", { name: "QR code for your authenticator app" })).toBeTruthy();
    expect(screen.getByText("JBSW Y3DP EHPK 3PXP JBSW Y3DP EHPK 3PXP")).toBeTruthy();
    const input = screen.getByLabelText("Authenticator code");
    expect(input.getAttribute("autocomplete")).toBe("one-time-code");
    expect(input.getAttribute("inputmode")).toBe("numeric");
  });

  it("keeps the form after a wrong code", async () => {
    const routes = {
      "POST /api/2fa/totp/setup": setupReply,
      "POST /api/2fa/totp": { status: 400, body: { error: "invalid_code", message: "That code did not work." } },
    };
    const calls = stubApi(routes);
    const props = renderSetup();
    add();
    await screen.findByRole("dialog");
    enterCode("123456");
    expect((await screen.findByRole("alert")).textContent).toBe("That code did not work.");
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
    expect(props.onDone).not.toHaveBeenCalled();
    expect(calls[1]).toEqual({ key: "POST /api/2fa/totp", body: { code: "123456" } });
  });

  it("shows the recovery codes of a first factor, then calls onDone", async () => {
    stubApi({
      "POST /api/2fa/totp/setup": setupReply,
      "POST /api/2fa/totp": { status: 201, body: { recovery_codes: codes } },
    });
    const props = renderSetup();
    add();
    await screen.findByRole("dialog");
    enterCode("123456");
    expect(await screen.findByRole("list", { name: "Recovery codes" })).toBeTruthy();
    expect(props.onDone).not.toHaveBeenCalled();
    fireEvent.click(screen.getByLabelText("I saved these codes"));
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(props.onDone).toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("calls onDone at once when no recovery codes come back", async () => {
    stubApi({ "POST /api/2fa/totp/setup": setupReply, "POST /api/2fa/totp": { status: 201, body: {} } });
    const props = renderSetup(twoFactorState({ enabled: true }));
    add();
    await screen.findByRole("dialog");
    enterCode("123456");
    await waitFor(() => expect(props.onDone).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("asks to confirm first when the confirm has expired", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "POST /api/2fa/totp/setup": { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } },
      "POST /api/2fa/confirm": { status: 204 },
    };
    stubApi(routes);
    renderSetup();
    add();
    expect(await screen.findByRole("dialog", { name: "Confirm it is you" })).toBeTruthy();
    routes["POST /api/2fa/totp/setup"] = setupReply;
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    expect(await screen.findByRole("dialog", { name: "Add authenticator app" })).toBeTruthy();
  });

  it("cancels without saving", async () => {
    stubApi({ "POST /api/2fa/totp/setup": setupReply });
    const props = renderSetup();
    add();
    await screen.findByRole("dialog");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(props.onDone).not.toHaveBeenCalled();
  });

  it("says when authenticator apps are not available", () => {
    stubApi({});
    renderSetup(twoFactorState({ totp_available: false }));
    expect(screen.getByText(/Authenticator apps are not available/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Add authenticator app" })).toBeNull();
  });

  it("offers nothing once an authenticator is set up", () => {
    stubApi({});
    renderSetup(twoFactorState({ enabled: true, totp: { created_at: "2026-11-10T12:00:00Z", last_used_at: null } }));
    expect(screen.queryByRole("button", { name: "Add authenticator app" })).toBeNull();
    // No passkeys in this browser either, so no empty button row (it would add a gap).
    expect(document.querySelector(".btn-row")).toBeNull();
  });

  it("shows other errors and reports a 401 as signed out", async () => {
    const routes: Parameters<typeof stubApi>[0] = {
      "POST /api/2fa/totp/setup": { status: 409, body: { error: "already_set_up", message: "An authenticator app is already set up." } },
    };
    stubApi(routes);
    const props = renderSetup();
    add();
    expect((await screen.findByRole("alert")).textContent).toBe("An authenticator app is already set up.");
    routes["POST /api/2fa/totp/setup"] = { status: 401, body: { error: "unauthenticated", message: "x" } };
    add();
    await waitFor(() => expect(props.onSignedOut).toHaveBeenCalled());
  });
});

describe("TwoFactorSetup passkey", () => {
  const openAdd = () => fireEvent.click(screen.getByRole("button", { name: "Add passkey" }));
  const nameAndAdd = (name: string) => {
    fireEvent.change(screen.getByLabelText("Passkey name"), { target: { value: name } });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
  };
  const tooMany = "You have 10 passkeys. Remove one before adding another.";

  it("adds a passkey with a name and shows the recovery codes of a first factor", async () => {
    stubPasskeys({ id: "new-credential" });
    const calls = stubApi({
      "POST /api/2fa/passkeys/options": webAuthnOptions,
      "POST /api/2fa/passkeys": { status: 201, body: { passkey: passkey(), recovery_codes: codes } },
    });
    const props = renderSetup();
    openAdd();
    expect(await screen.findByRole("dialog", { name: "Add passkey" })).toBeTruthy();
    nameAndAdd("Phone");
    expect(await screen.findByRole("list", { name: "Recovery codes" })).toBeTruthy();
    expect(calls.map((c) => c.key)).toEqual(["POST /api/2fa/passkeys/options", "POST /api/2fa/passkeys"]);
    expect(calls[1].body).toEqual({ name: "Phone", credential: { id: "new-credential" } });
    fireEvent.click(screen.getByLabelText("I saved these codes"));
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(props.onDone).toHaveBeenCalled();
  });

  it("calls onDone at once for a later passkey", async () => {
    stubPasskeys();
    stubApi({
      "POST /api/2fa/passkeys/options": webAuthnOptions,
      "POST /api/2fa/passkeys": { status: 201, body: { passkey: passkey({ id: 2, name: "Laptop" }) } },
    });
    const props = renderSetup(twoFactorState({ enabled: true, passkeys: [passkey()] }));
    openAdd();
    await screen.findByRole("dialog");
    nameAndAdd("Laptop");
    await waitFor(() => expect(props.onDone).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the dialog open when the device prompt is cancelled", async () => {
    stubPasskeys("cancel");
    const calls = stubApi({ "POST /api/2fa/passkeys/options": webAuthnOptions });
    const props = renderSetup();
    openAdd();
    await screen.findByRole("dialog");
    nameAndAdd("Phone");
    expect((await screen.findByRole("alert")).textContent).toBe("Passkey setup was cancelled.");
    expect(screen.getByLabelText("Passkey name")).toBeTruthy();
    expect(calls.map((c) => c.key)).toEqual(["POST /api/2fa/passkeys/options"]);
    expect(props.onDone).not.toHaveBeenCalled();
  });

  it("shows the API's message for a refused passkey", async () => {
    stubPasskeys();
    stubApi({
      "POST /api/2fa/passkeys/options": webAuthnOptions,
      "POST /api/2fa/passkeys": { status: 400, body: { error: "passkey_failed", message: "The passkey could not be added. Try again." } },
    });
    renderSetup();
    openAdd();
    await screen.findByRole("dialog");
    nameAndAdd("Phone");
    expect((await screen.findByRole("alert")).textContent).toBe("The passkey could not be added. Try again.");
  });

  it("shows too_many_passkeys, and disables Add passkey with the hint at 10", async () => {
    stubPasskeys();
    stubApi({ "POST /api/2fa/passkeys/options": { status: 409, body: { error: "too_many_passkeys", message: "x" } } });
    renderSetup(twoFactorState({ enabled: true, passkeys: [passkey()] }));
    openAdd();
    await screen.findByRole("dialog");
    nameAndAdd("Phone");
    expect((await screen.findByRole("alert")).textContent).toBe(tooMany);
    cleanup();
    const ten = Array.from({ length: 10 }, (_, i) => passkey({ id: i + 1, name: `Key ${i + 1}` }));
    renderSetup(twoFactorState({ enabled: true, passkeys: ten }));
    expect((screen.getByRole("button", { name: "Add passkey" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText(tooMany)).toBeTruthy();
  });

  it("asks to confirm first when the confirm has expired", async () => {
    stubPasskeys();
    const routes: Parameters<typeof stubApi>[0] = {
      "POST /api/2fa/passkeys/options": { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } },
      "POST /api/2fa/confirm": { status: 204 },
    };
    stubApi(routes);
    renderSetup();
    openAdd();
    await screen.findByRole("dialog");
    nameAndAdd("Phone");
    expect(await screen.findByRole("dialog", { name: "Confirm it is you" })).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    expect(await screen.findByRole("dialog", { name: "Add passkey" })).toBeTruthy();
    expect((screen.getByLabelText("Passkey name") as HTMLInputElement).value).toBe("Phone");
  });

  it("says passkeys are not available without browser support", () => {
    stubApi({});
    renderSetup();
    expect(screen.getByText("Passkeys are not available in this browser.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Add passkey" })).toBeNull();
  });

  it("says passkeys are not available when the server has none", () => {
    stubPasskeys();
    stubApi({});
    renderSetup(twoFactorState({ passkeys_available: false, passkeys_origin: null }));
    expect(screen.getByText("Passkeys are not available: this server has no public URL.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Add passkey" })).toBeNull();
  });

  it("names the address to use when opened under another one", () => {
    stubPasskeys();
    stubApi({});
    renderSetup(twoFactorState({ passkeys_origin: "https://taplist.example.org" }));
    expect(screen.getByText("Passkeys are not available here. Open https://taplist.example.org to use them.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Add passkey" })).toBeNull();
  });
});
