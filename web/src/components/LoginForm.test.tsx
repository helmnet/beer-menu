import { StrictMode } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, stubPasskeys, testPassword, webAuthnOptions } from "../testApi";
import LoginForm from "./LoginForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function submit(username: string, password: string) {
  fireEvent.change(screen.getByLabelText("Username"), { target: { value: username } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("LoginForm", () => {
  it("signs in", async () => {
    const calls = stubApi({ "POST /api/session": { status: 200, body: owner } });
    const onSignedIn = vi.fn();
    render(<LoginForm onSignedIn={onSignedIn} onEnroll={vi.fn()} />);
    const pw = testPassword();
    submit("owner", pw);
    await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(owner));
    expect(calls).toEqual([{ key: "POST /api/session", body: { username: "owner", password: pw } }]);
  });

  it("hands a 202 enroll to onEnroll", async () => {
    stubApi({ "POST /api/session": { status: 202, body: { next: "enroll" } } });
    const onSignedIn = vi.fn();
    const onEnroll = vi.fn();
    render(<LoginForm onSignedIn={onSignedIn} onEnroll={onEnroll} />);
    submit("owner", testPassword());
    await waitFor(() => expect(onEnroll).toHaveBeenCalled());
    expect(onSignedIn).not.toHaveBeenCalled();
  });

  it("shows a notice it was opened with", () => {
    render(<LoginForm onSignedIn={vi.fn()} onEnroll={vi.fn()} notice="Sign-in timed out, start again." />);
    expect(screen.getByRole("alert").textContent).toBe("Sign-in timed out, start again.");
  });

  it("shows a generic message on 401", async () => {
    stubApi({
      "POST /api/session": { status: 401, body: { error: "invalid_credentials", message: "x" } },
    });
    const onSignedIn = vi.fn();
    render(<LoginForm onSignedIn={onSignedIn} onEnroll={vi.fn()} />);
    submit("owner", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe("Wrong username or password.");
    expect(onSignedIn).not.toHaveBeenCalled();
  });

  it("shows when to retry on 429", async () => {
    stubApi({
      "POST /api/session": {
        status: 429,
        body: { error: "too_many_attempts", message: "x" },
        headers: { "Retry-After": "811" },
      },
    });
    render(<LoginForm onSignedIn={vi.fn()} onEnroll={vi.fn()} />);
    submit("owner", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe(
      "Too many failed attempts. Try again in 14 minutes.",
    );
  });

  it("says one minute when under a minute is left", async () => {
    stubApi({
      "POST /api/session": {
        status: 429,
        body: { error: "too_many_attempts", message: "x" },
        headers: { "Retry-After": "30" },
      },
    });
    render(<LoginForm onSignedIn={vi.fn()} onEnroll={vi.fn()} />);
    submit("owner", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe(
      "Too many failed attempts. Try again in 1 minute.",
    );
  });

  describe("second step", () => {
    const pending = { status: 202, body: { next: "second_factor", methods: ["totp"] } };
    const twoFactorOwner = { ...owner, two_factor: true };

    async function toCodeStep(routes: Parameters<typeof stubApi>[0]) {
      const calls = stubApi({ "POST /api/session": pending, ...routes });
      const onSignedIn = vi.fn();
      render(<LoginForm onSignedIn={onSignedIn} onEnroll={vi.fn()} />);
      submit("owner", testPassword());
      await screen.findByLabelText("Authenticator code");
      return { calls, onSignedIn };
    }

    function sendCode(code: string) {
      fireEvent.change(screen.getByLabelText("Authenticator code"), { target: { value: code } });
      fireEvent.click(screen.getByRole("button", { name: "Verify" }));
    }

    it("asks for a code after a 202 and signs in with it", async () => {
      const { calls, onSignedIn } = await toCodeStep({ "POST /api/session/totp": { status: 200, body: twoFactorOwner } });
      const input = screen.getByLabelText("Authenticator code");
      expect(input.getAttribute("autocomplete")).toBe("one-time-code");
      expect(input.getAttribute("inputmode")).toBe("numeric");
      expect(document.activeElement).toBe(input);
      expect(screen.queryByLabelText("Password")).toBeNull();
      expect(onSignedIn).not.toHaveBeenCalled();
      sendCode("123 456");
      await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(twoFactorOwner));
      expect(calls[1]).toEqual({ key: "POST /api/session/totp", body: { code: "123 456" } });
    });

    it("keeps the code step after a wrong code", async () => {
      const { onSignedIn } = await toCodeStep({
        "POST /api/session/totp": { status: 401, body: { error: "invalid_code", message: "x" } },
      });
      sendCode("000000");
      expect((await screen.findByRole("alert")).textContent).toBe("That code did not work.");
      expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
      expect(onSignedIn).not.toHaveBeenCalled();
    });

    it("returns to the password step when the pending sign-in ended", async () => {
      await toCodeStep({
        "POST /api/session/totp": { status: 401, body: { error: "unauthenticated", message: "x" } },
      });
      sendCode("123456");
      expect((await screen.findByRole("alert")).textContent).toBe("Sign-in timed out, start again.");
      expect(screen.getByLabelText("Password")).toBeTruthy();
      expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    });

    it("shows when to retry on 429", async () => {
      await toCodeStep({
        "POST /api/session/totp": {
          status: 429,
          body: { error: "too_many_attempts", message: "x" },
          headers: { "Retry-After": "811" },
        },
      });
      sendCode("123456");
      expect((await screen.findByRole("alert")).textContent).toBe("Too many failed attempts. Try again in 14 minutes.");
    });

    it("signs in with a recovery code", async () => {
      const { calls, onSignedIn } = await toCodeStep({
        "POST /api/session": { status: 202, body: { next: "second_factor", methods: ["totp", "recovery"] } },
        "POST /api/session/recovery": { status: 200, body: twoFactorOwner },
      });
      fireEvent.click(screen.getByRole("button", { name: "Use a recovery code" }));
      const input = screen.getByLabelText("Recovery code");
      expect(input.getAttribute("autocomplete")).toBe("off");
      expect(screen.queryByLabelText("Authenticator code")).toBeNull();
      fireEvent.change(input, { target: { value: "abcde-fghij" } });
      fireEvent.click(screen.getByRole("button", { name: "Verify" }));
      await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(twoFactorOwner));
      expect(calls[1]).toEqual({ key: "POST /api/session/recovery", body: { code: "abcde-fghij" } });
    });

    it("keeps the recovery step after a wrong code and switches back", async () => {
      await toCodeStep({
        "POST /api/session": { status: 202, body: { next: "second_factor", methods: ["totp", "recovery"] } },
        "POST /api/session/recovery": { status: 401, body: { error: "invalid_code", message: "x" } },
      });
      fireEvent.click(screen.getByRole("button", { name: "Use a recovery code" }));
      fireEvent.change(screen.getByLabelText("Recovery code"), { target: { value: "aaaaa-aaaaa" } });
      fireEvent.click(screen.getByRole("button", { name: "Verify" }));
      expect((await screen.findByRole("alert")).textContent).toBe("That code did not work.");
      expect(screen.getByLabelText("Recovery code")).toBeTruthy();
      fireEvent.click(screen.getByRole("button", { name: "Use another method" }));
      expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
      expect(screen.queryByLabelText("Recovery code")).toBeNull();
    });

    it("asks for a recovery code at once when it is the only method", async () => {
      stubApi({ "POST /api/session": { status: 202, body: { next: "second_factor", methods: ["recovery"] } } });
      render(<LoginForm onSignedIn={vi.fn()} onEnroll={vi.fn()} />);
      submit("owner", testPassword());
      const input = await screen.findByLabelText("Recovery code");
      expect(input.getAttribute("type")).toBeNull();
      expect(document.activeElement).toBe(input);
      expect(screen.queryByRole("button", { name: "Use another method" })).toBeNull();
    });

    it("offers no recovery code when none are left", async () => {
      await toCodeStep({});
      expect(screen.queryByRole("button", { name: "Use a recovery code" })).toBeNull();
    });

    it("cancels the pending sign-in", async () => {
      const { calls } = await toCodeStep({ "DELETE /api/session/pending": { status: 204 } });
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
      expect(await screen.findByLabelText("Password")).toBeTruthy();
      await waitFor(() => expect(calls.map((c) => c.key)).toContain("DELETE /api/session/pending"));
      expect(screen.queryByLabelText("Authenticator code")).toBeNull();
    });
  });
});

describe("LoginForm passkey step", () => {
  const twoFactorOwner = { ...owner, two_factor: true };
  const pendingWith = (methods: string[], origin: string = location.origin) => ({
    status: 202,
    body: { next: "second_factor", methods, passkeys_origin: origin },
  });

  async function toSecondStep(
    methods: string[],
    routes: Parameters<typeof stubApi>[0],
    origin?: string,
    strict = false,
  ) {
    const calls = stubApi({ "POST /api/session": pendingWith(methods, origin), ...routes });
    const onSignedIn = vi.fn();
    const form = <LoginForm onSignedIn={onSignedIn} onEnroll={vi.fn()} />;
    render(strict ? <StrictMode>{form}</StrictMode> : form);
    submit("owner", testPassword());
    await waitFor(() => expect(screen.queryByLabelText("Password")).toBeNull());
    return { calls, onSignedIn };
  }

  const usePasskey = () => fireEvent.click(screen.getByRole("button", { name: "Use a passkey" }));

  it("offers a choice and starts no prompt when more than one method is usable", async () => {
    const prompt = stubPasskeys();
    const { calls } = await toSecondStep(["totp", "passkey", "recovery"], {});
    expect(screen.getByRole("button", { name: "Use a passkey" })).toBeTruthy();
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
    expect(prompt).not.toHaveBeenCalled();
    expect(calls.map((c) => c.key)).toEqual(["POST /api/session"]);
  });

  it("signs in with a passkey at once when it is the only method", async () => {
    stubPasskeys({ id: "assertion" });
    const { calls, onSignedIn } = await toSecondStep(["passkey", "recovery"], {
      "POST /api/session/passkey/options": webAuthnOptions,
      "POST /api/session/passkey": { status: 200, body: twoFactorOwner },
    });
    await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(twoFactorOwner));
    expect(calls.slice(1)).toEqual([
      { key: "POST /api/session/passkey/options", body: undefined },
      { key: "POST /api/session/passkey", body: { id: "assertion" } },
    ]);
  });

  it("shows the step busy while the automatic prompt is open, and starts it once", async () => {
    const prompt = stubPasskeys();
    prompt.mockReturnValue(new Promise(() => {}));
    const { calls } = await toSecondStep(["passkey"], { "POST /api/session/passkey/options": webAuthnOptions }, undefined, true);
    await waitFor(() => expect(prompt).toHaveBeenCalled());
    const button = screen.getByRole("button", { name: "Use a passkey" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    usePasskey();
    await new Promise((r) => setTimeout(r, 0));
    expect(prompt).toHaveBeenCalledTimes(1);
    expect(calls.filter((c) => c.key === "POST /api/session/passkey/options")).toHaveLength(1);
  });

  it("starts no prompt when this browser cannot use passkeys", async () => {
    const { calls } = await toSecondStep(["passkey"], {});
    expect(screen.getByText("Passkeys are not available in this browser.")).toBeTruthy();
    expect(calls.map((c) => c.key)).toEqual(["POST /api/session"]);
  });

  it("leaves the step without an error when the automatic prompt is refused, and the button retries", async () => {
    const prompt = stubPasskeys({ id: "assertion" });
    let refuse = () => {};
    // The refusal waits until the step has rendered with its button disabled, as a real prompt does.
    prompt.mockImplementationOnce(
      () => new Promise((_, reject) => (refuse = () => reject(new DOMException("No user gesture.", "NotAllowedError")))),
    );
    const { calls, onSignedIn } = await toSecondStep(["passkey", "recovery"], {
      "POST /api/session/passkey/options": webAuthnOptions,
      "POST /api/session/passkey": { status: 200, body: twoFactorOwner },
    });
    const button = screen.getByRole("button", { name: "Use a passkey" }) as HTMLButtonElement;
    await waitFor(() => expect(prompt).toHaveBeenCalled());
    expect(button.disabled).toBe(true);
    refuse();
    await waitFor(() => expect(button.disabled).toBe(false));
    // autoFocus missed the button while it was disabled, so focus must be moved to it.
    expect(document.activeElement).toBe(button);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("button", { name: "Use a recovery code" })).toBeTruthy();
    expect(onSignedIn).not.toHaveBeenCalled();
    usePasskey();
    await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(twoFactorOwner));
    expect(prompt).toHaveBeenCalledTimes(2);
    expect(calls.map((c) => c.key).filter((k) => k === "POST /api/session/passkey")).toHaveLength(1);
  });

  it("keeps the step open and offers the code when the prompt is cancelled", async () => {
    stubPasskeys("cancel");
    const { calls, onSignedIn } = await toSecondStep(["totp", "passkey"], { "POST /api/session/passkey/options": webAuthnOptions });
    usePasskey();
    expect((await screen.findByRole("alert")).textContent).toBe("Passkey sign-in was cancelled.");
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Use a passkey" })).toBeTruthy();
    expect(calls.map((c) => c.key)).not.toContain("POST /api/session/passkey");
    expect(onSignedIn).not.toHaveBeenCalled();
  });

  it("says when the automatic passkey was not accepted", async () => {
    stubPasskeys();
    await toSecondStep(["passkey"], {
      "POST /api/session/passkey/options": webAuthnOptions,
      "POST /api/session/passkey": { status: 401, body: { error: "passkey_failed", message: "x" } },
    });
    expect((await screen.findByRole("alert")).textContent).toBe("The passkey was not accepted.");
    expect(screen.getByRole("button", { name: "Use a passkey" })).toBeTruthy();
  });

  it("returns to the password step when the pending sign-in ended", async () => {
    stubPasskeys();
    // The automatic prompt ends the step at once, so this does not wait for it like toSecondStep.
    stubApi({
      "POST /api/session": pendingWith(["passkey"]),
      "POST /api/session/passkey/options": { status: 401, body: { error: "unauthenticated", message: "x" } },
    });
    render(<LoginForm onSignedIn={vi.fn()} onEnroll={vi.fn()} />);
    submit("owner", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe("Sign-in timed out, start again.");
    expect(screen.getByLabelText("Password")).toBeTruthy();
  });

  it("hides the passkey method when opened under another address", async () => {
    stubPasskeys();
    await toSecondStep(["totp", "passkey"], {}, "https://taplist.example.org");
    expect(screen.queryByRole("button", { name: "Use a passkey" })).toBeNull();
    expect(screen.getByText("Passkeys work only at https://taplist.example.org.")).toBeTruthy();
    expect(screen.getByLabelText("Authenticator code")).toBeTruthy();
  });
});
