import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { menu, stubApi } from "./testApi";

beforeEach(() => {
  stubApi({ "GET /api/menu": { status: 200, body: menu() }, "GET /api/setup": { status: 200, body: { needed: true } } });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.pushState({}, "", "/");
});

describe("App", () => {
  it("renders the admin page at /", () => {
    window.history.pushState({}, "", "/");
    render(<App />);
    expect(screen.getByRole("heading", { name: "Beer Menu" })).toBeTruthy();
  });

  it("renders the display at /display", async () => {
    window.history.pushState({}, "", "/display");
    render(<App />);
    expect(await screen.findByText("Nothing on tap")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "Beer Menu" })).toBeNull();
  });

  it("renders the display without the admin shell or styles", async () => {
    window.history.pushState({}, "", "/display");
    const { container } = render(<App />);
    expect(await screen.findByText("Nothing on tap")).toBeTruthy();
    expect(container.querySelector(".admin")).toBeNull();
    expect(screen.queryByRole("navigation", { name: "Admin" })).toBeNull();
  });

  it("renders the display below /display/", async () => {
    window.history.pushState({}, "", "/display/tv1");
    render(<App />);
    expect(await screen.findByText("Nothing on tap")).toBeTruthy();
  });

  it("renders the admin page for /displayx", () => {
    window.history.pushState({}, "", "/displayx");
    render(<App />);
    expect(screen.getByRole("heading", { name: "Beer Menu" })).toBeTruthy();
  });
});
