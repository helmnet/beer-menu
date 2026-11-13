import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { stubApi, testPassword, twoFactorState } from "../testApi";
import PasswordForm from "./PasswordForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function submit(current: string, next: string, confirm = next) {
  fireEvent.change(screen.getByLabelText("Current password"), { target: { value: current } });
  fireEvent.change(screen.getByLabelText("New password"), { target: { value: next } });
  fireEvent.change(screen.getByLabelText("Confirm new password"), { target: { value: confirm } });
  fireEvent.click(screen.getByRole("button", { name: "Change password" }));
}

describe("PasswordForm", () => {
  it("requires matching new entries", async () => {
    const calls = stubApi({});
    render(<PasswordForm onSignedOut={vi.fn()} />);
    submit(testPassword(), testPassword(), testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe("The new passwords do not match.");
    expect(calls).toEqual([]);
  });

  it("changes the password and shows success", async () => {
    const calls = stubApi({ "POST /api/password": { status: 204 } });
    render(<PasswordForm onSignedOut={vi.fn()} />);
    const current = testPassword();
    const next = testPassword();
    submit(current, next);
    expect(await screen.findByText("Password changed. Your other sessions have ended.")).toBeTruthy();
    expect(calls).toEqual([
      { key: "POST /api/password", body: { current_password: current, new_password: next } },
    ]);
    expect((screen.getByLabelText("New password") as HTMLInputElement).value).toBe("");
  });

  it("shows a wrong current password", async () => {
    stubApi({
      "POST /api/password": {
        status: 403,
        body: { error: "wrong_password", message: "The current password is wrong." },
      },
    });
    render(<PasswordForm onSignedOut={vi.fn()} />);
    submit(testPassword(), testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe("The current password is wrong.");
  });

  it("shows when to retry on 429", async () => {
    stubApi({
      "POST /api/password": {
        status: 429,
        body: { error: "too_many_attempts", message: "x" },
        headers: { "Retry-After": "600" },
      },
    });
    render(<PasswordForm onSignedOut={vi.fn()} />);
    submit(testPassword(), testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe(
      "Too many failed attempts. Try again in 10 minutes.",
    );
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "POST /api/password": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const onSignedOut = vi.fn();
    render(<PasswordForm onSignedOut={onSignedOut} />);
    submit(testPassword(), testPassword());
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  describe("confirm identity", () => {
    const confirmRequired = { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } };

    it("confirms, then sends the same change again", async () => {
      const routes: Parameters<typeof stubApi>[0] = {
        "POST /api/password": confirmRequired,
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
        "POST /api/2fa/confirm": { status: 204 },
      };
      const calls = stubApi(routes);
      render(<PasswordForm onSignedOut={vi.fn()} />);
      const current = testPassword();
      const next = testPassword();
      submit(current, next);
      const confirm = await screen.findByRole("dialog", { name: "Confirm it is you" });
      expect(screen.queryByRole("alert")).toBeNull();
      routes["POST /api/password"] = { status: 204 };
      fireEvent.change(within(confirm).getByLabelText("Password"), { target: { value: current } });
      fireEvent.click(within(confirm).getByRole("button", { name: "Confirm" }));
      expect(await screen.findByText("Password changed. Your other sessions have ended.")).toBeTruthy();
      expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
      const posts = calls.filter((c) => c.key === "POST /api/password");
      expect(posts).toHaveLength(2);
      expect(posts[1].body).toEqual({ current_password: current, new_password: next });
    });

    it("sends nothing more when the confirm is cancelled", async () => {
      const calls = stubApi({
        "POST /api/password": confirmRequired,
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
      });
      render(<PasswordForm onSignedOut={vi.fn()} />);
      submit(testPassword(), testPassword());
      const confirm = await screen.findByRole("dialog", { name: "Confirm it is you" });
      fireEvent.click(within(confirm).getByRole("button", { name: "Cancel" }));
      expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
      expect(screen.queryByText("Password changed. Your other sessions have ended.")).toBeNull();
      expect(calls.filter((c) => c.key === "POST /api/password")).toHaveLength(1);
      expect((screen.getByRole("button", { name: "Change password" }) as HTMLButtonElement).disabled).toBe(false);
    });
  });
});
