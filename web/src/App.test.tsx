import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import App from "./App";

afterEach(() => {
  cleanup();
  window.history.pushState({}, "", "/");
});

describe("App", () => {
  it("renders the admin page at /", () => {
    window.history.pushState({}, "", "/");
    render(<App />);
    expect(screen.getByRole("heading", { name: "Beer Menu" })).toBeTruthy();
  });

  it("renders the display placeholder at /display", () => {
    window.history.pushState({}, "", "/display");
    render(<App />);
    expect(screen.getByText("Tap list coming soon")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "Beer Menu" })).toBeNull();
  });

  it("renders the display placeholder below /display/", () => {
    window.history.pushState({}, "", "/display/tv1");
    render(<App />);
    expect(screen.getByText("Tap list coming soon")).toBeTruthy();
  });

  it("renders the admin page for /displayx", () => {
    window.history.pushState({}, "", "/displayx");
    render(<App />);
    expect(screen.getByRole("heading", { name: "Beer Menu" })).toBeTruthy();
  });
});
