import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Keg } from "../api";
import { beer, keg, kegTypes, stubApi } from "../testApi";
import { volumeToLitres } from "../units";
import KegCard from "./KegCard";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const onTap = keg({ id: 10, beer_name: "Stout", remaining_l: 12.34, tap_id: 1, tap_label: "Left", state: "on_tap" });
const waiting = keg({ id: 11, beer_name: "Lager", type: "sanke_half", size_l: 58.67, remaining_l: 58.67 });

function renderCard(k: Keg, props: Partial<Parameters<typeof KegCard>[0]> = {}) {
  const onChanged = vi.fn();
  const onSignedOut = vi.fn();
  const utils = render(
    <KegCard keg={k} types={kegTypes} onChanged={onChanged} onSignedOut={onSignedOut} {...props} />,
  );
  return { ...utils, onChanged, onSignedOut };
}

function keys(calls: { key: string }[]) {
  return calls.map((c) => c.key);
}

const buttonNames = () => screen.getAllByRole("button").map((b) => b.textContent);

describe("KegCard", () => {
  it("shows the beer, type, level and actions in order for a keg on a tap", () => {
    stubApi({});
    renderCard(onTap);
    expect(screen.getByText("Stout")).toBeTruthy();
    expect(screen.getByText("Corny 5 gal")).toBeTruthy();
    expect(screen.getByText("3.26 gal of 5.00 gal")).toBeTruthy();
    expect(screen.getByRole("img").getAttribute("data-vessel")).toBe("glass");
    expect(buttonNames()).toEqual(["Pour", "Set level", "Edit keg", "Take off", "Kick"]);
  });

  it("offers only Edit keg and Kick for a waiting keg", () => {
    stubApi({});
    renderCard(waiting);
    expect(screen.getByRole("img").getAttribute("data-keg-shape")).toBe("sanke");
    expect(buttonNames()).toEqual(["Edit keg", "Kick"]);
    for (const name of ["Pour", "Set level", "Take off"]) {
      expect(screen.queryByRole("button", { name })).toBeNull();
    }
  });

  it("draws the level as a bar with no style attribute", () => {
    stubApi({});
    const { container } = renderCard(onTap);
    const bar = container.querySelector("progress");
    expect(bar?.value).toBeCloseTo(12.34 / 18.93);
    expect(container.querySelectorAll("[style]")).toHaveLength(0);
  });

  it("records a pour prefilled 16 fl oz, sent in litres", async () => {
    const calls = stubApi({ "POST /api/kegs/10/pours": { status: 200, body: onTap } });
    const { onChanged } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Pour" }));
    expect((screen.getByLabelText("Pour (fl oz)") as HTMLInputElement).value).toBe("16");
    fireEvent.click(screen.getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(calls.find((c) => c.key === "POST /api/kegs/10/pours")?.body).toEqual({ amount_l: 0.47 });
    expect(screen.queryByLabelText("Pour (fl oz)")).toBeNull();
  });

  it("records a pour prefilled 500 mL in metric", async () => {
    const calls = stubApi({ "POST /api/kegs/10/pours": { status: 200, body: onTap } });
    renderCard(onTap, { units: "metric" });
    fireEvent.click(screen.getByRole("button", { name: "Pour" }));
    expect((screen.getByLabelText("Pour (mL)") as HTMLInputElement).value).toBe("500");
    fireEvent.click(screen.getByRole("button", { name: "Save pour" }));
    await waitFor(() => expect(keys(calls)).toContain("POST /api/kegs/10/pours"));
    expect(calls.find((c) => c.key === "POST /api/kegs/10/pours")?.body).toEqual({ amount_l: 0.5 });
  });

  it("sets a changed level and sends nothing for an unchanged one", async () => {
    const calls = stubApi({ "PATCH /api/kegs/10": { status: 200, body: onTap } });
    const { onChanged } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Set level" }));
    fireEvent.click(screen.getByRole("button", { name: "Save level" }));
    expect(screen.queryByLabelText("Remaining (gal)")).toBeNull();
    expect(calls).toEqual([]);

    fireEvent.click(screen.getByRole("button", { name: "Set level" }));
    const input = screen.getByLabelText("Remaining (gal)") as HTMLInputElement;
    expect(input.value).toBe("3.26");
    fireEvent.change(input, { target: { value: "2.5" } });
    fireEvent.click(screen.getByRole("button", { name: "Save level" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ remaining_l: volumeToLitres(2.5, "us") });
  });

  it("restarts an open level form in the new units", () => {
    stubApi({});
    const props = { keg: onTap, types: kegTypes, onChanged: vi.fn(), onSignedOut: vi.fn() };
    const { rerender } = render(<KegCard {...props} units="us" />);
    fireEvent.click(screen.getByRole("button", { name: "Set level" }));
    fireEvent.change(screen.getByLabelText("Remaining (gal)"), { target: { value: "1" } });
    rerender(<KegCard {...props} units="metric" />);
    expect((screen.getByLabelText("Remaining (L)") as HTMLInputElement).value).toBe("12.34");
  });

  it("opens the edit keg form inside the card, and Cancel closes it", async () => {
    stubApi({
      "GET /api/beers": { status: 200, body: [beer({ id: 1, name: "Stout" })] },
      "GET /api/keg-types": { status: 200, body: kegTypes },
    });
    const { container } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Edit keg" }));
    const form = screen.getByRole("form", { name: "Edit keg of Stout" });
    expect(container.contains(form)).toBe(true);
    fireEvent.click(within(form).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("form", { name: "Edit keg of Stout" })).toBeNull();
  });

  it("takes a keg off its tap", async () => {
    const calls = stubApi({ "PATCH /api/kegs/10": { status: 200, body: { ...onTap, tap_id: null } } });
    const { onChanged } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Take off" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ tap_id: null });
  });

  it("kicks only after confirmation", async () => {
    const calls = stubApi({ "PATCH /api/kegs/10": { status: 200, body: { ...onTap, state: "kicked" } } });
    const confirm = vi.fn(() => false);
    vi.stubGlobal("confirm", confirm);
    const { onChanged } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Kick" }));
    expect(confirm).toHaveBeenCalledWith("Kick the Stout keg? It moves to keg history.");
    expect(calls).toEqual([]);
    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole("button", { name: "Kick" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    expect(calls.find((c) => c.key === "PATCH /api/kegs/10")?.body).toEqual({ kicked: true });
  });

  it("shows a server message and does not report a change", async () => {
    stubApi({ "PATCH /api/kegs/10": { status: 409, body: { error: "conflict", message: "Try again." } } });
    const { onChanged } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Take off" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Try again.");
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("reports a 401 as signed out", async () => {
    stubApi({ "PATCH /api/kegs/10": { status: 401, body: { error: "unauthenticated", message: "x" } } });
    const { onSignedOut } = renderCard(onTap);
    fireEvent.click(screen.getByRole("button", { name: "Take off" }));
    await waitFor(() => expect(onSignedOut).toHaveBeenCalled());
  });
});
