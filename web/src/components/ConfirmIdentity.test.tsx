import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { nextTask, stubApi, stubPasskeys, stubPendingPasskey, testPassword, webAuthnOptions } from "../testApi";
import ConfirmIdentity from "./ConfirmIdentity";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function setup(hasFactor: boolean, more: { hasTotp?: boolean; passkeyOrigin?: string | null } = {}) {
  const props = { onConfirmed: vi.fn(), onCancel: vi.fn(), onSignedOut: vi.fn() };
  const { unmount } = render(<ConfirmIdentity hasFactor={hasFactor} {...more} {...props} />);
  return { ...props, unmount };
}

const type = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } });
const confirm = () => fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

describe("ConfirmIdentity", () => {
  it("asks only for the password without a factor", async () => {
    const calls = stubApi({ "POST /api/2fa/confirm": { status: 204 } });
    const props = setup(false);
    expect(screen.getByRole("dialog", { name: "Confirm it is you" })).toBeTruthy();
    expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    expect(screen.queryByRole("button", { name: "Use a recovery code" })).toBeNull();
    const pw = testPassword();
    type("Password", pw);
    confirm();
    await waitFor(() => expect(props.onConfirmed).toHaveBeenCalled());
    expect(calls).toEqual([{ key: "POST /api/2fa/confirm", body: { password: pw } }]);
  });

  it("sends the password and an authenticator code", async () => {
    const calls = stubApi({ "POST /api/2fa/confirm": { status: 204 } });
    const props = setup(true);
    const code = screen.getByLabelText("Authenticator code");
    expect(code.getAttribute("autocomplete")).toBe("one-time-code");
    expect(code.getAttribute("inputmode")).toBe("numeric");
    const pw = testPassword();
    type("Password", pw);
    type("Authenticator code", "123 456");
    confirm();
    await waitFor(() => expect(props.onConfirmed).toHaveBeenCalled());
    expect(calls).toEqual([{ key: "POST /api/2fa/confirm", body: { password: pw, code: "123 456" } }]);
  });

  it("switches to a recovery code and sends only that", async () => {
    const calls = stubApi({ "POST /api/2fa/confirm": { status: 204 } });
    const props = setup(true);
    type("Authenticator code", "123456");
    fireEvent.click(screen.getByRole("button", { name: "Use a recovery code" }));
    expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    const pw = testPassword();
    type("Password", pw);
    type("Recovery code", "abcde-fghij");
    confirm();
    await waitFor(() => expect(props.onConfirmed).toHaveBeenCalled());
    expect(calls).toEqual([{ key: "POST /api/2fa/confirm", body: { password: pw, recovery_code: "abcde-fghij" } }]);
    fireEvent.click(screen.getByRole("button", { name: "Use an authenticator code" }));
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
  });

  it.each([
    [403, "wrong_password", "The current password is wrong.", {}, "The current password is wrong."],
    [403, "invalid_code", "That code did not work.", {}, "That code did not work."],
    [429, "too_many_attempts", "x", { "Retry-After": "120" }, "Too many failed attempts. Try again in 2 minutes."],
  ])("shows a %i %s", async (status, error, message, headers, shown) => {
    stubApi({ "POST /api/2fa/confirm": { status, body: { error, message }, headers } });
    const props = setup(true);
    type("Password", testPassword());
    type("Authenticator code", "123456");
    confirm();
    expect((await screen.findByRole("alert")).textContent).toBe(shown);
    expect(props.onConfirmed).not.toHaveBeenCalled();
  });

  it("cancels, and reports a 401 as signed out", async () => {
    stubApi({ "POST /api/2fa/confirm": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const props = setup(false);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(props.onCancel).toHaveBeenCalled();
    type("Password", testPassword());
    confirm();
    await waitFor(() => expect(props.onSignedOut).toHaveBeenCalled());
  });

  it("does not confirm when closed while the confirm is in flight", async () => {
    const calls = stubApi({ "POST /api/2fa/confirm": { status: 204 } });
    const props = setup(false);
    type("Password", testPassword());
    confirm();
    props.unmount();
    await waitFor(() => expect(calls).toHaveLength(1));
    await nextTask();
    expect(props.onConfirmed).not.toHaveBeenCalled();
  });
});

describe("ConfirmIdentity with a passkey", () => {
  const setupPasskey = (hasTotp: boolean, passkeyOrigin: string | null = location.origin) =>
    setup(true, { hasTotp, passkeyOrigin });

  it("confirms with the password and a passkey", async () => {
    stubPasskeys({ id: "assertion" });
    const calls = stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions, "POST /api/2fa/confirm": { status: 204 } });
    const props = setupPasskey(false);
    expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    expect(screen.getByRole("button", { name: "Use a recovery code" })).toBeTruthy();
    const pw = testPassword();
    type("Password", pw);
    confirm();
    await waitFor(() => expect(props.onConfirmed).toHaveBeenCalled());
    expect(calls).toEqual([
      { key: "POST /api/2fa/confirm/options", body: undefined },
      { key: "POST /api/2fa/confirm", body: { password: pw, passkey: { id: "assertion" } } },
    ]);
  });

  it("switches from the authenticator code to a passkey", async () => {
    stubPasskeys();
    stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions, "POST /api/2fa/confirm": { status: 204 } });
    const props = setupPasskey(true);
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Use a passkey" }));
    expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    type("Password", testPassword());
    confirm();
    await waitFor(() => expect(props.onConfirmed).toHaveBeenCalled());
  });

  it("stays open when the prompt is cancelled", async () => {
    stubPasskeys("cancel");
    const calls = stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions });
    const props = setupPasskey(false);
    type("Password", testPassword());
    confirm();
    expect((await screen.findByRole("alert")).textContent).toBe("Passkey check was cancelled.");
    expect(calls.map((c) => c.key)).toEqual(["POST /api/2fa/confirm/options"]);
    expect(props.onConfirmed).not.toHaveBeenCalled();
  });

  it("does nothing when closed while the prompt is open, even if the prompt then succeeds", async () => {
    const prompt = stubPendingPasskey();
    const calls = stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions, "POST /api/2fa/confirm": { status: 204 } });
    const props = setupPasskey(false);
    type("Password", testPassword());
    confirm();
    await waitFor(() => expect(navigator.credentials.get).toHaveBeenCalled());
    props.unmount();
    prompt.approve({ id: "assertion" });
    await nextTask();
    expect(calls.map((c) => c.key)).toEqual(["POST /api/2fa/confirm/options"]);
    expect(props.onConfirmed).not.toHaveBeenCalled();
  });

  it("cancels the open prompt when closed", async () => {
    const prompt = stubPendingPasskey();
    stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions });
    const props = setupPasskey(false);
    type("Password", testPassword());
    confirm();
    await waitFor(() => expect(navigator.credentials.get).toHaveBeenCalled());
    expect(prompt.signal()?.aborted).toBe(false);
    props.unmount();
    expect(prompt.signal()?.aborted).toBe(true);
  });

  it("cancels the prompt when closed while its options load", async () => {
    stubPasskeys();
    const calls = stubApi({ "POST /api/2fa/confirm/options": webAuthnOptions });
    const props = setupPasskey(false);
    type("Password", testPassword());
    confirm();
    props.unmount();
    await waitFor(() => expect(calls).toHaveLength(1));
    await nextTask(); // let getPasskey see the aborted signal
    expect(navigator.credentials.get).not.toHaveBeenCalled();
    expect(props.onConfirmed).not.toHaveBeenCalled();
  });

  it("does not offer a passkey under another address", () => {
    stubPasskeys();
    stubApi({});
    setupPasskey(true, "https://taplist.example.org");
    expect(screen.queryByRole("button", { name: "Use a passkey" })).toBeNull();
  });
});
