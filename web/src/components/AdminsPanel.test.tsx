import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, testPassword, twoFactorState } from "../testApi";
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
  it("lists admins and adds one in the Add admin dialog, then closes it and reloads", async () => {
    const routes = {
      "GET /api/admins": { status: 200, body: [owner] as unknown[] },
      "POST /api/admins": { status: 201, body: second },
    };
    const calls = stubApi(routes);
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    expect(screen.queryByLabelText("Username")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    const dialog = screen.getByRole("dialog", { name: "Add admin" });
    const pw = testPassword();
    fireEvent.change(within(dialog).getByLabelText("Username"), { target: { value: "second" } });
    fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: pw } });
    routes["GET /api/admins"] = { status: 200, body: [owner, second] };
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect(await screen.findByText("second")).toBeTruthy();
    expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull();
    expect(calls.find((c) => c.key === "POST /api/admins")?.body).toEqual({ username: "second", password: pw });
    // Reopened, the dialog starts empty.
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    expect((within(screen.getByRole("dialog", { name: "Add admin" })).getByLabelText("Username") as HTMLInputElement).value).toBe("");
  });

  it("keeps the dialog open with the server message when the username is taken", async () => {
    stubApi({
      "GET /api/admins": { status: 200, body: [owner] },
      "POST /api/admins": {
        status: 409,
        body: { error: "username_taken", message: "That username is already in use." },
      },
    });
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    const dialog = screen.getByRole("dialog", { name: "Add admin" });
    fireEvent.change(within(dialog).getByLabelText("Username"), { target: { value: "OWNER" } });
    fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    expect((await within(dialog).findByRole("alert")).textContent).toBe("That username is already in use.");
    expect(screen.getByRole("dialog", { name: "Add admin" })).toBe(dialog);
    expect((within(dialog).getByLabelText("Username") as HTMLInputElement).value).toBe("OWNER");
  });

  it("closes the Add admin dialog on Cancel, Close or Escape without a request", async () => {
    const calls = stubApi({ "GET /api/admins": { status: 200, body: [owner] } });
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    const dialog = () => screen.getByRole("dialog", { name: "Add admin" });
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    fireEvent.click(within(dialog()).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    fireEvent.click(within(dialog()).getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    fireEvent(dialog(), new Event("cancel", { cancelable: true }));
    expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull();
    expect(calls.map((c) => c.key)).toEqual(["GET /api/admins"]);
  });

  it("opens the same Add admin dialog from the top bar slot it is given", async () => {
    stubApi({ "GET /api/admins": { status: 200, body: [owner] } });
    const slot = document.createElement("div");
    document.body.appendChild(slot);
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} actionSlot={slot} />);
    await screen.findByText("owner");
    expect(screen.getAllByRole("button", { name: "Add admin" })).toHaveLength(2);
    fireEvent.click(within(slot).getByRole("button", { name: "Add admin" }));
    expect(screen.getByRole("dialog", { name: "Add admin" })).toBeTruthy();
    slot.remove();
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

  it("reports a 401 from Add admin as signed out", async () => {
    stubApi({
      "GET /api/admins": { status: 200, body: [owner] },
      "POST /api/admins": { status: 401, body: { error: "unauthenticated", message: "x" } },
    });
    const onSignedOut = vi.fn();
    render(<AdminsPanel me={owner} onSignedOut={onSignedOut} />);
    await screen.findByText("owner");
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    const dialog = screen.getByRole("dialog", { name: "Add admin" });
    fireEvent.change(within(dialog).getByLabelText("Username"), { target: { value: "second" } });
    fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: testPassword() } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
    expect(within(dialog).queryByRole("alert")).toBeNull();
  });

  it("drops the typed password when the Add admin dialog is cancelled", async () => {
    stubApi({ "GET /api/admins": { status: 200, body: [owner] } });
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    const pw = () => within(screen.getByRole("dialog", { name: "Add admin" })).getByLabelText("Password") as HTMLInputElement;
    expect(pw().autocomplete).toBe("new-password");
    fireEvent.change(pw(), { target: { value: testPassword() } });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    expect(pw().value).toBe("");
  });

  it("sends one POST when Add admin is submitted twice", async () => {
    const calls = stubApi({
      "GET /api/admins": { status: 200, body: [owner] as unknown[] },
      "POST /api/admins": { status: 201, body: second },
    });
    render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
    await screen.findByText("owner");
    fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
    const dialog = screen.getByRole("dialog", { name: "Add admin" });
    fireEvent.change(within(dialog).getByLabelText("Username"), { target: { value: "second" } });
    fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: testPassword() } });
    const form = within(dialog).getByRole("button", { name: "Save" }).closest("form") as HTMLFormElement;
    fireEvent.submit(form);
    fireEvent.submit(form);
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull());
    expect(calls.filter((c) => c.key === "POST /api/admins")).toHaveLength(1);
  });

  describe("confirm identity", () => {
    const confirmRequired = { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } };

    it("confirms over the Add admin dialog, then adds", async () => {
      const routes: Parameters<typeof stubApi>[0] = {
        "GET /api/admins": { status: 200, body: [owner] },
        "POST /api/admins": confirmRequired,
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
        "POST /api/2fa/confirm": { status: 204 },
      };
      const calls = stubApi(routes);
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      await screen.findByText("owner");
      fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
      const add = screen.getByRole("dialog", { name: "Add admin" });
      const pw = testPassword();
      fireEvent.change(within(add).getByLabelText("Username"), { target: { value: "second" } });
      fireEvent.change(within(add).getByLabelText("Password"), { target: { value: pw } });
      fireEvent.click(within(add).getByRole("button", { name: "Save" }));
      const confirm = await screen.findByRole("dialog", { name: "Confirm it is you" });
      expect(screen.getByRole("dialog", { name: "Add admin" })).toBe(add);
      expect(within(add).queryByRole("alert")).toBeNull();
      routes["POST /api/admins"] = { status: 201, body: second };
      routes["GET /api/admins"] = { status: 200, body: [owner, second] };
      const myPw = testPassword();
      fireEvent.change(within(confirm).getByLabelText("Password"), { target: { value: myPw } });
      fireEvent.click(within(confirm).getByRole("button", { name: "Confirm" }));
      expect(await screen.findByText("second")).toBeTruthy();
      expect(screen.queryByRole("dialog", { name: "Add admin" })).toBeNull();
      expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
      const posts = calls.filter((c) => c.key === "POST /api/admins");
      expect(posts).toHaveLength(2);
      expect(posts[1].body).toEqual({ username: "second", password: pw });
      expect(calls.find((c) => c.key === "POST /api/2fa/confirm")?.body).toEqual({ password: myPw });
    });

    it("keeps the Add admin dialog and sends nothing more when the confirm is cancelled", async () => {
      const calls = stubApi({
        "GET /api/admins": { status: 200, body: [owner] },
        "POST /api/admins": confirmRequired,
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
      });
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      await screen.findByText("owner");
      fireEvent.click(screen.getByRole("button", { name: "Add admin" }));
      const add = screen.getByRole("dialog", { name: "Add admin" });
      fireEvent.change(within(add).getByLabelText("Username"), { target: { value: "second" } });
      fireEvent.change(within(add).getByLabelText("Password"), { target: { value: testPassword() } });
      fireEvent.click(within(add).getByRole("button", { name: "Save" }));
      const confirm = await screen.findByRole("dialog", { name: "Confirm it is you" });
      fireEvent.click(within(confirm).getByRole("button", { name: "Cancel" }));
      expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
      expect(screen.getByRole("dialog", { name: "Add admin" })).toBe(add);
      expect(calls.filter((c) => c.key === "POST /api/admins")).toHaveLength(1);
    });

    it("confirms, then removes", async () => {
      const routes: Parameters<typeof stubApi>[0] = {
        "GET /api/admins": { status: 200, body: [owner, second] },
        "DELETE /api/admins/2": confirmRequired,
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
        "POST /api/2fa/confirm": { status: 204 },
      };
      const calls = stubApi(routes);
      vi.stubGlobal("confirm", () => true);
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      await screen.findByText("second");
      fireEvent.click(removeButton("second"));
      const dialog = await screen.findByRole("dialog", { name: "Confirm it is you" });
      expect(screen.getByText("second")).toBeTruthy();
      routes["DELETE /api/admins/2"] = { status: 204 };
      routes["GET /api/admins"] = { status: 200, body: [owner] };
      fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: testPassword() } });
      fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
      await waitFor(() => expect(screen.queryByText("second")).toBeNull());
      expect(calls.filter((c) => c.key === "DELETE /api/admins/2")).toHaveLength(2);
    });
  });

  describe("two-factor", () => {
    const withTwoFactor = { ...second, two_factor: true };
    const clearButton = (username: string) =>
      within(screen.getByText(username).closest("li") as HTMLElement).queryByRole("button", { name: "Clear 2FA" });

    it("shows a 2FA badge on admins who have it", async () => {
      stubApi({ "GET /api/admins": { status: 200, body: [owner, withTwoFactor] } });
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      const row = (await screen.findByText("second")).closest("li") as HTMLElement;
      expect(within(row).getByText("2FA")).toBeTruthy();
      expect(within(screen.getByText("owner").closest("li") as HTMLElement).queryByText("2FA")).toBeNull();
    });

    it("offers Clear 2FA only on other admins with 2FA", async () => {
      const me = { ...owner, two_factor: true };
      stubApi({ "GET /api/admins": { status: 200, body: [me, second, { ...withTwoFactor, id: 3, username: "third" }] } });
      render(<AdminsPanel me={me} onSignedOut={vi.fn()} />);
      await screen.findByText("third");
      expect(clearButton("owner")).toBeNull();
      expect(clearButton("second")).toBeNull();
      expect(clearButton("third")).toBeTruthy();
    });

    it("asks before clearing, then clears and reloads", async () => {
      const routes = {
        "GET /api/admins": { status: 200, body: [owner, withTwoFactor] as unknown[] },
        "DELETE /api/admins/2/2fa": { status: 204 },
      };
      const calls = stubApi(routes);
      const confirm = vi.fn(() => false);
      vi.stubGlobal("confirm", confirm);
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      await screen.findByText("second");
      fireEvent.click(clearButton("second")!);
      expect(confirm).toHaveBeenCalled();
      expect(calls.map((c) => c.key)).not.toContain("DELETE /api/admins/2/2fa");

      confirm.mockReturnValue(true);
      routes["GET /api/admins"] = { status: 200, body: [owner, second] };
      fireEvent.click(clearButton("second")!);
      await waitFor(() => expect(clearButton("second")).toBeNull());
      expect(calls.map((c) => c.key)).toContain("DELETE /api/admins/2/2fa");
      expect(within(screen.getByText("second").closest("li") as HTMLElement).queryByText("2FA")).toBeNull();
    });

    it("confirms identity when asked, then clears", async () => {
      const routes: Parameters<typeof stubApi>[0] = {
        "GET /api/admins": { status: 200, body: [owner, withTwoFactor] },
        "DELETE /api/admins/2/2fa": { status: 403, body: { error: "confirm_required", message: "Confirm it is you first." } },
        "GET /api/2fa": { status: 200, body: twoFactorState({}) },
        "POST /api/2fa/confirm": { status: 204 },
      };
      const calls = stubApi(routes);
      vi.stubGlobal("confirm", () => true);
      render(<AdminsPanel me={owner} onSignedOut={vi.fn()} />);
      await screen.findByText("second");
      fireEvent.click(clearButton("second")!);
      // The caller has no factor of their own, so the password alone confirms.
      const dialog = await screen.findByRole("dialog", { name: "Confirm it is you" });
      expect(within(dialog).queryByLabelText("Authenticator code")).toBeNull();
      expect(clearButton("second")).toBeTruthy();
      routes["DELETE /api/admins/2/2fa"] = { status: 204 };
      routes["GET /api/admins"] = { status: 200, body: [owner, second] };
      const pw = testPassword();
      fireEvent.change(within(dialog).getByLabelText("Password"), { target: { value: pw } });
      fireEvent.click(within(dialog).getByRole("button", { name: "Confirm" }));
      await waitFor(() => expect(clearButton("second")).toBeNull());
      expect(screen.queryByRole("dialog", { name: "Confirm it is you" })).toBeNull();
      expect(calls.filter((c) => c.key === "DELETE /api/admins/2/2fa")).toHaveLength(2);
      expect(calls.find((c) => c.key === "POST /api/2fa/confirm")?.body).toEqual({ password: pw });
    });
  });
});
