import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { volumeToLitres } from "../units";
import { beer, keg, kegTypes, Reply, stubApi } from "../testApi";
import KegForm from "./KegForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const pale = beer({ id: 1, name: "Pale" });
const stout = beer({ id: 2, name: "Stout" });

function routes(extra: Record<string, Reply> = {}): Record<string, Reply> {
  return {
    "GET /api/beers": { status: 200, body: [pale, stout] },
    "GET /api/keg-types": { status: 200, body: kegTypes },
    ...extra,
  };
}

function value(label: string) {
  return (screen.getByLabelText(label) as HTMLInputElement).value;
}

function type(label: string, v: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value: v } });
}

/** errorFor returns the error text linked to the input labelled label, or null. */
function errorFor(label: string): string | null {
  const id = screen.getByLabelText(label).getAttribute("aria-describedby");
  return id ? (document.getElementById(id)?.textContent ?? null) : null;
}

describe("KegForm", () => {
  it("loads the keg types and current beers, defaulting to a 5 gallon corny", async () => {
    const calls = stubApi(routes());
    render(<KegForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    expect(await screen.findByRole("option", { name: "Half barrel" })).toBeTruthy();
    expect(screen.getByRole("option", { name: "Stout" })).toBeTruthy();
    expect(value("Type")).toBe("corny_5_gal");
    expect(value("Size (gal)")).toBe("5.00");
    expect(value("Remaining (gal)")).toBe("5.00");
    expect(calls.map((c) => c.key).sort()).toEqual(["GET /api/beers", "GET /api/keg-types"]);
  });

  it("prefills size and remaining in gallons from the chosen type", async () => {
    stubApi(routes());
    render(<KegForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Type", "sanke_half");
    expect(value("Size (gal)")).toBe("15.50");
    expect(value("Remaining (gal)")).toBe("15.50");
  });

  it("adds a keg on a tap without the prefilled volumes", async () => {
    const created = keg({ id: 5, type: "sanke_half", tap_id: 2, state: "on_tap" });
    const calls = stubApi(routes({ "POST /api/kegs": { status: 201, body: created } }));
    const onSaved = vi.fn();
    render(<KegForm tapId={2} onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Beer", "1");
    type("Type", "sanke_half");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(created));
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({ beer_id: 1, type: "sanke_half", tap_id: 2 });
  });

  it("adds a waiting keg with the volumes the user changed, in litres", async () => {
    const calls = stubApi(routes({ "POST /api/kegs": { status: 201, body: keg() } }));
    const onSaved = vi.fn();
    render(<KegForm onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Beer", "2");
    type("Size (gal)", "4");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({
      beer_id: 2,
      type: "corny_5_gal",
      size_l: volumeToLitres(4, "us"),
    });
  });

  it("edits only the changed fields and never resends untouched volumes", async () => {
    const existing = keg({ id: 7, beer_id: 1, type: "euro_30_l", size_l: 30, remaining_l: 12.34 });
    const calls = stubApi(routes({ "PATCH /api/kegs/7": { status: 200, body: existing } }));
    const onSaved = vi.fn();
    const onCancel = vi.fn();
    render(<KegForm keg={existing} onSaved={onSaved} onCancel={onCancel} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    expect(value("Beer")).toBe("1");
    expect(value("Type")).toBe("euro_30_l");
    expect(value("Size (gal)")).toBe("7.93");
    expect(value("Remaining (gal)")).toBe("3.26");

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onCancel).toHaveBeenCalled();
    expect(calls.map((c) => c.key)).not.toContain("PATCH /api/kegs/7");

    type("Beer", "2");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/7")?.body).toEqual({ beer_id: 2 });

    type("Remaining (gal)", "2.5");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(2));
    expect(calls.filter((c) => c.key === "PATCH /api/kegs/7")[1].body).toEqual({
      beer_id: 2,
      remaining_l: volumeToLitres(2.5, "us"),
    });
  });

  it("corrects only the type on edit, keeping the size (FR-020)", async () => {
    const existing = keg({ id: 7, type: "euro_30_l", size_l: 30, remaining_l: 12.34 });
    const calls = stubApi(routes({ "PATCH /api/kegs/7": { status: 200, body: existing } }));
    const onSaved = vi.fn();
    render(<KegForm keg={existing} onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Type", "sanke_half");
    expect(value("Size (gal)")).toBe("7.93");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(calls.find((c) => c.key === "PATCH /api/kegs/7")?.body).toEqual({ type: "sanke_half" });
  });

  it("shows each field error next to its field", async () => {
    stubApi(
      routes({
        "POST /api/kegs": {
          status: 400,
          body: {
            error: "invalid_request",
            message: "Some fields are invalid.",
            fields: { beer_id: "Choose a current beer.", remaining_l: "Must be 0 to the size." },
          },
        },
      }),
    );
    const onSaved = vi.fn();
    render(<KegForm onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Remaining (gal)", "9");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(errorFor("Beer")).toBe("Choose a current beer."));
    expect(errorFor("Remaining (gal)")).toBe("Must be 0 to the size.");
    expect(errorFor("Size (gal)")).toBeNull();
    expect(screen.getByLabelText("Beer").getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("alert").textContent).toBe("Some fields are invalid.");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("reports a 401 as signed out", async () => {
    stubApi(routes({ "POST /api/kegs": { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<KegForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={onSignedOut} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Beer", "1");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });

  it("reports a 401 while loading as signed out", async () => {
    stubApi(routes({ "GET /api/keg-types": { status: 401, body: { error: "unauthenticated", message: "x" } } }));
    const onSignedOut = vi.fn();
    render(<KegForm onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={onSignedOut} />);
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});

describe("KegForm metric", () => {
  it("labels volumes in litres and prefills a corny type in litres", async () => {
    stubApi(routes());
    render(<KegForm units="metric" onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    expect(value("Size (L)")).toBe("18.93");
    expect(value("Remaining (L)")).toBe("18.93");
    type("Type", "corny_3_gal");
    expect(value("Size (L)")).toBe("11.36");
  });

  it("sends typed litres", async () => {
    const calls = stubApi(routes({ "POST /api/kegs": { status: 201, body: keg() } }));
    const onSaved = vi.fn();
    render(<KegForm units="metric" onSaved={onSaved} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    await screen.findByRole("option", { name: "Half barrel" });
    type("Beer", "1");
    type("Size (L)", "19");
    type("Remaining (L)", "12.345");
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(calls.find((c) => c.key === "POST /api/kegs")?.body).toEqual({
      beer_id: 1,
      type: "corny_5_gal",
      size_l: 19,
      remaining_l: 12.35,
    });
  });

  it("shows an edited keg in litres and keeps US labels with units us", async () => {
    stubApi(routes());
    const existing = keg({ id: 7, size_l: 30, remaining_l: 12.34 });
    const { unmount } = render(
      <KegForm units="metric" keg={existing} onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />,
    );
    expect(value("Size (L)")).toBe("30.00");
    expect(value("Remaining (L)")).toBe("12.34");
    unmount();
    render(<KegForm units="us" keg={existing} onSaved={vi.fn()} onCancel={vi.fn()} onSignedOut={vi.fn()} />);
    expect(value("Size (gal)")).toBe("7.93");
  });
});
