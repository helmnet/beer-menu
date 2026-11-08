import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { owner, stubApi, testPassword } from "../testApi";
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
    render(<LoginForm onSignedIn={onSignedIn} />);
    const pw = testPassword();
    submit("owner", pw);
    await waitFor(() => expect(onSignedIn).toHaveBeenCalledWith(owner));
    expect(calls).toEqual([{ key: "POST /api/session", body: { username: "owner", password: pw } }]);
  });

  it("shows a generic message on 401", async () => {
    stubApi({
      "POST /api/session": { status: 401, body: { error: "invalid_credentials", message: "x" } },
    });
    const onSignedIn = vi.fn();
    render(<LoginForm onSignedIn={onSignedIn} />);
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
    render(<LoginForm onSignedIn={vi.fn()} />);
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
    render(<LoginForm onSignedIn={vi.fn()} />);
    submit("owner", testPassword());
    expect((await screen.findByRole("alert")).textContent).toBe(
      "Too many failed attempts. Try again in 1 minute.",
    );
  });
});
