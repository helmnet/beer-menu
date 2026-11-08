import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, testPassword } from "../testApi";
import SetupForm from "./SetupForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function fill(username: string, password: string, confirm = password) {
  fireEvent.change(screen.getByLabelText("Username"), { target: { value: username } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
  fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: confirm } });
  fireEvent.click(screen.getByRole("button", { name: "Create admin" }));
}

describe("SetupForm", () => {
  it("creates the first admin", async () => {
    const calls = stubApi({ "POST /api/setup": { status: 201, body: owner } });
    const onCreated = vi.fn();
    render(<SetupForm onCreated={onCreated} onSetupDone={vi.fn()} />);
    const pw = testPassword();
    fill("owner", pw);
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(owner));
    expect(calls).toEqual([{ key: "POST /api/setup", body: { username: "owner", password: pw } }]);
  });

  it("checks length and confirmation before sending", () => {
    const calls = stubApi({});
    render(<SetupForm onCreated={vi.fn()} onSetupDone={vi.fn()} />);
    fill("owner", "x".repeat(11));
    expect(screen.getByRole("alert").textContent).toBe("Password must be 12 to 256 characters.");
    fill("owner", testPassword(), testPassword());
    expect(screen.getByRole("alert").textContent).toBe("The passwords do not match.");
    expect(calls).toEqual([]);
  });

  it("shows the server message on 400", async () => {
    stubApi({
      "POST /api/setup": {
        status: 400,
        body: { error: "invalid_request", message: "Username must be 1 to 64 letters, digits, dots, underscores or hyphens." },
      },
    });
    render(<SetupForm onCreated={vi.fn()} onSetupDone={vi.fn()} />);
    fill("bad name", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe(
      "Username must be 1 to 64 letters, digits, dots, underscores or hyphens.",
    );
  });

  it("reports setup done on 409", async () => {
    stubApi({ "POST /api/setup": { status: 409, body: { error: "setup_done", message: "done" } } });
    const onSetupDone = vi.fn();
    render(<SetupForm onCreated={vi.fn()} onSetupDone={onSetupDone} />);
    fill("owner", testPassword());
    await waitFor(() => expect(onSetupDone).toHaveBeenCalled());
  });
});
