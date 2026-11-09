import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { beer, stubApi } from "../testApi";
import BeerForm from "./BeerForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

// A 40-entry table whose entries are easy to tell apart.
const colors = Array.from({ length: 40 }, (_, i) => `#1000${(i + 1).toString(16).padStart(2, "0")}`);
const colorsReply = { status: 200, body: { colors } };

function preview() {
  const svg = screen.getByRole("img");
  return {
    vessel: svg.getAttribute("data-vessel"),
    fill: svg.querySelector('[data-part="liquid"]')?.getAttribute("fill"),
  };
}

function type(label: string, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

/** errorFor returns the error text linked to the input labelled label, or null. */
function errorFor(label: string): string | null {
  const id = screen.getByLabelText(label).getAttribute("aria-describedby");
  return id ? (document.getElementById(id)?.textContent ?? null) : null;
}

describe("BeerForm", () => {
  it("adds a beer with only the entered fields", async () => {
    const created = beer({ id: 7, name: "Pale", abv: 5.2, status: "fermenting" });
    const calls = stubApi({ "GET /api/srm-colors": colorsReply, "POST /api/beers": { status: 201, body: created } });
    const onSaved = vi.fn();
    render(<BeerForm onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    type("Name", "Pale");
    type("ABV", "5.2");
    type("IBU", "30");
    type("Brew date", "2026-10-01");
    type("Status", "fermenting");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(created));
    expect(calls.find((c) => c.key === "POST /api/beers")?.body).toEqual({
      name: "Pale",
      abv: 5.2,
      ibu: 30,
      brew_date: "2026-10-01",
      status: "fermenting",
    });
  });

  it("edits a beer with only the changed fields, sending null for a cleared one", async () => {
    const existing = beer({ id: 3, name: "Pale", style: "APA", abv: 5, srm: 4, description: "Hoppy" });
    const saved = { ...existing, style: null, abv: 5.4 };
    const calls = stubApi({ "GET /api/srm-colors": colorsReply, "PATCH /api/beers/3": { status: 200, body: saved } });
    const onSaved = vi.fn();
    render(<BeerForm beer={existing} onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Pale");
    expect((screen.getByLabelText("Description") as HTMLTextAreaElement).value).toBe("Hoppy");
    type("ABV", "5.4");
    type("Style", "");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(saved));
    expect(calls.find((c) => c.key === "PATCH /api/beers/3")?.body).toEqual({ abv: 5.4, style: null });
  });

  it("shows each field error next to its field", async () => {
    stubApi({
      "GET /api/srm-colors": colorsReply,
      "POST /api/beers": {
        status: 400,
        body: {
          error: "invalid_request",
          message: "Some fields are invalid.",
          fields: { name: "Name is required.", abv: "Must be 0 to 25 with at most one decimal place." },
        },
      },
    });
    const onSaved = vi.fn();
    render(<BeerForm onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    type("ABV", "150");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(errorFor("Name")).toBe("Name is required."));
    expect(errorFor("ABV")).toBe("Must be 0 to 25 with at most one decimal place.");
    expect(errorFor("Style")).toBeNull();
    expect(screen.getByLabelText("ABV").getAttribute("aria-invalid")).toBe("true");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("reports a 401 as signed out", async () => {
    stubApi({
      "GET /api/srm-colors": colorsReply,
      "POST /api/beers": { status: 401, body: { error: "unauthenticated", message: "x" } },
    });
    const onSignedOut = vi.fn();
    render(<BeerForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={onSignedOut} />);
    type("Name", "Pale");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("cancels without a request", () => {
    const calls = stubApi({ "GET /api/srm-colors": colorsReply });
    const onCancel = vi.fn();
    render(<BeerForm onSaved={vi.fn()} onCancel={onCancel} onSignedOut={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalled();
    expect(calls.filter((c) => !c.key.startsWith("GET"))).toEqual([]);
  });

  it("previews the artwork from the SRM input before saving, as a keg whatever the status", async () => {
    const calls = stubApi({ "GET /api/srm-colors": colorsReply });
    render(<BeerForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await waitFor(() => expect(calls.map((c) => c.key)).toContain("GET /api/srm-colors"));
    expect(preview()).toEqual({ vessel: "keg", fill: "none" });
    type("SRM", "20");
    await waitFor(() => expect(preview()).toEqual({ vessel: "keg", fill: colors[19] }));
    type("Status", "conditioning");
    expect(preview()).toEqual({ vessel: "keg", fill: colors[19] });
    type("SRM", "4.5");
    expect(preview().fill).toBe(colors[4]);
    type("SRM", "60");
    expect(preview().fill).toBe(colors[39]);
    expect(calls.filter((c) => c.key === "GET /api/srm-colors")).toHaveLength(1);
  });

  it("previews an empty vessel when the color table fails to load", async () => {
    const calls = stubApi({ "GET /api/srm-colors": { status: 502 } });
    render(<BeerForm beer={beer({ srm: 7 })} onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await waitFor(() => expect(calls).toHaveLength(1));
    expect(preview()).toEqual({ vessel: "keg", fill: "none" });
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
