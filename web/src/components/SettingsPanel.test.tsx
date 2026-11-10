import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { settings, stubApi } from "../testApi";
import SettingsPanel from "./SettingsPanel";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const unauthenticated = { status: 401, body: { error: "unauthenticated", message: "Sign in." } };
const loaded = settings({ title: "Helm Taproom", theme: "light", units: "metric", page_seconds: 20 });

function value(label: string) {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

function type(label: string, v: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value: v } });
}

/** errorFor returns the error text linked to the control labelled label, or null. */
function errorFor(label: string): string | null {
  const id = screen.getByLabelText(label).getAttribute("aria-describedby");
  return id ? (document.getElementById(id)?.textContent ?? null) : null;
}

/** shown renders the panel and waits for the loaded title. */
async function shown(onChange = vi.fn(), onSignedOut = vi.fn()) {
  render(<SettingsPanel onChange={onChange} onSignedOut={onSignedOut} />);
  await waitFor(() => expect(value("Title")).toBe("Helm Taproom"));
  return { onChange, onSignedOut };
}

describe("SettingsPanel", () => {
  it("loads and shows the settings and reports them", async () => {
    const calls = stubApi({ "GET /api/settings": { status: 200, body: loaded } });
    const { onChange } = await shown();
    expect(calls.map((c) => c.key)).toEqual(["GET /api/settings"]);
    expect(value("Theme")).toBe("light");
    expect(value("Units")).toBe("metric");
    expect(value("Page seconds")).toBe("20");
    const options = (label: string) =>
      Array.from((screen.getByLabelText(label) as HTMLSelectElement).options).map((o) => [o.value, o.text]);
    expect(options("Theme")).toEqual([
      ["dark", "Dark"],
      ["light", "Light"],
    ]);
    expect(options("Units")).toEqual([
      ["us", "US"],
      ["metric", "Metric"],
    ]);
    const seconds = screen.getByLabelText("Page seconds");
    expect(seconds.getAttribute("type")).toBe("number");
    expect(seconds.getAttribute("min")).toBe("5");
    expect(seconds.getAttribute("max")).toBe("300");
    expect(seconds.getAttribute("step")).toBe("1");
    expect(onChange).toHaveBeenCalledWith(loaded);
  });

  it("saves only the changed fields and reports the reply", async () => {
    const reply = { ...loaded, units: "us" as const, page_seconds: 30 };
    const calls = stubApi({
      "GET /api/settings": { status: 200, body: loaded },
      "PATCH /api/settings": { status: 200, body: reply },
    });
    const { onChange } = await shown();
    type("Units", "us");
    type("Page seconds", "30");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(reply));
    expect(calls.find((c) => c.key === "PATCH /api/settings")?.body).toEqual({ units: "us", page_seconds: 30 });
  });

  it("sends nothing when nothing changed", async () => {
    const calls = stubApi({ "GET /api/settings": { status: 200, body: loaded } });
    const { onChange } = await shown();
    type("Title", "Helm Taproom");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await new Promise((r) => setTimeout(r, 0));
    expect(calls.map((c) => c.key)).toEqual(["GET /api/settings"]);
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("shows field errors beside their fields and keeps what was typed", async () => {
    stubApi({
      "GET /api/settings": { status: 200, body: loaded },
      "PATCH /api/settings": {
        status: 400,
        body: {
          error: "invalid_request",
          message: "Some fields are invalid.",
          fields: { title: "Use at most 60 characters.", page_seconds: "Use 5 to 300 seconds." },
        },
      },
    });
    const { onChange } = await shown();
    const long = "x".repeat(61);
    type("Title", long);
    type("Page seconds", "4");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(errorFor("Title")).toBe("Use at most 60 characters."));
    expect(errorFor("Page seconds")).toBe("Use 5 to 300 seconds.");
    expect(screen.getByLabelText("Title").getAttribute("aria-invalid")).toBe("true");
    expect(errorFor("Theme")).toBeNull();
    expect(screen.getByRole("alert").textContent).toBe("Some fields are invalid.");
    expect(value("Title")).toBe(long);
    expect(value("Page seconds")).toBe("4");
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("reports a 401 on load as signed out", async () => {
    stubApi({ "GET /api/settings": unauthenticated });
    const onSignedOut = vi.fn();
    render(<SettingsPanel onChange={vi.fn()} onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("reports a 401 on save as signed out", async () => {
    stubApi({ "GET /api/settings": { status: 200, body: loaded }, "PATCH /api/settings": unauthenticated });
    const { onSignedOut } = await shown();
    type("Theme", "dark");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("shows other errors", async () => {
    stubApi({ "GET /api/settings": { status: 500, body: { error: "internal", message: "Something broke." } } });
    const onChange = vi.fn();
    render(<SettingsPanel onChange={onChange} onSignedOut={vi.fn()} />);
    expect((await screen.findByRole("alert")).textContent).toBe("Something broke.");
    expect(onChange).not.toHaveBeenCalled();
  });
});
