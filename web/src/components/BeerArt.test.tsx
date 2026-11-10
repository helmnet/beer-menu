import { cleanup, render } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import BeerArt, { colorForSrm } from "./BeerArt";

afterEach(cleanup);

function draw(onTap: boolean, color: string | null, height?: number) {
  const { container } = render(<BeerArt onTap={onTap} color={color} name="Honey IPA" height={height} />);
  const svg = container.querySelector("svg") as SVGSVGElement;
  return { svg, liquid: svg.querySelector('[data-part="liquid"]') as SVGElement };
}

describe("BeerArt", () => {
  it("draws a pint glass for a beer on tap and a keg otherwise", () => {
    const glass = draw(true, "#ffbf42");
    expect(glass.svg.getAttribute("data-vessel")).toBe("glass");
    expect(glass.svg.getAttribute("role")).toBe("img");
    expect(glass.svg.getAttribute("aria-label")).toBe("Honey IPA, pint glass");
    cleanup();
    const keg = draw(false, "#470606");
    expect(keg.svg.getAttribute("data-vessel")).toBe("keg");
    expect(keg.svg.getAttribute("aria-label")).toBe("Honey IPA, keg");
    cleanup();
    // onTap defaults to false: a beer with no keg on a tap is a keg.
    const { container } = render(<BeerArt color="#470606" name="Honey IPA" />);
    expect(container.querySelector("svg")?.getAttribute("data-vessel")).toBe("keg");
  });

  it("draws a corny keg by default and a sanke keg on request", () => {
    const shape = (kegShape?: "corny" | "sanke") => {
      const { container } = render(<BeerArt kegShape={kegShape} color="#9b3200" name="Honey IPA" />);
      const svg = container.querySelector("svg") as SVGSVGElement;
      const result = {
        vessel: svg.getAttribute("data-vessel"),
        shape: svg.getAttribute("data-keg-shape"),
        fill: svg.querySelector('[data-part="liquid"]')?.getAttribute("fill"),
        label: svg.getAttribute("aria-label"),
      };
      cleanup();
      return result;
    };
    const corny = { vessel: "keg", shape: "corny", fill: "#9b3200", label: "Honey IPA, keg" };
    expect(shape()).toEqual(corny);
    expect(shape("corny")).toEqual(corny);
    expect(shape("sanke")).toEqual({ ...corny, shape: "sanke" });
  });

  it.each(["corny", "sanke"] as const)("draws the glass on tap whatever the keg shape (%s)", (kegShape) => {
    const { container } = render(<BeerArt onTap kegShape={kegShape} color="#9b3200" name="Honey IPA" />);
    const svg = container.querySelector("svg") as SVGSVGElement;
    expect(svg.getAttribute("data-vessel")).toBe("glass");
    expect(svg.getAttribute("aria-label")).toBe("Honey IPA, pint glass");
  });

  it("draws an empty sanke keg without a color", () => {
    const { container } = render(<BeerArt kegShape="sanke" color={null} name="Honey IPA" />);
    expect(container.querySelector('[data-part="liquid"]')?.getAttribute("fill")).toBe("none");
  });

  it.each([true, false])("fills the liquid with the color, or nothing (onTap %s)", (onTap) => {
    expect(draw(onTap, "#9b3200").liquid.getAttribute("fill")).toBe("#9b3200");
    cleanup();
    expect(draw(onTap, null).liquid.getAttribute("fill")).toBe("none");
  });

  it("fills the keg body from the top ring to the foot", () => {
    const { liquid } = draw(false, "#36080a");
    const top = Number(liquid.getAttribute("y"));
    // The top ring ends at y 22 and the foot starts at y 84.
    expect(top).toBeLessThanOrEqual(22);
    expect(top + Number(liquid.getAttribute("height"))).toBeGreaterThanOrEqual(84);
  });

  it("sets the height and scales by viewBox", () => {
    const { svg } = draw(true, null, 64);
    expect(svg.getAttribute("height")).toBe("64");
    expect(svg.getAttribute("viewBox")).toBeTruthy();
  });

  it.each([
    [true, "corny"],
    [false, "corny"],
    [false, "sanke"],
  ] as const)("uses no style attribute or external reference (onTap %s, %s)", (onTap, kegShape) => {
    const { container } = render(<BeerArt onTap={onTap} kegShape={kegShape} color="#9b3200" name="Honey IPA" />);
    const svg = container.querySelector("svg") as SVGSVGElement;
    for (const el of [svg, ...Array.from(svg.querySelectorAll("*"))]) {
      expect(el.hasAttribute("style")).toBe(false);
      for (const attr of Array.from(el.attributes)) {
        expect(attr.name.endsWith("href")).toBe(false);
        expect(attr.value).not.toMatch(/url\(|https?:/);
      }
    }
  });
});

describe("BeerArt level", () => {
  const foamFill = "#f3ead7";

  function art(props: { onTap?: boolean; kegShape?: "corny" | "sanke"; level?: number; name?: string }) {
    const { container } = render(<BeerArt color="#9b3200" name="Pale Ale" {...props} />);
    const svg = container.querySelector("svg") as SVGSVGElement;
    return {
      svg,
      liquid: svg.querySelector('[data-part="liquid"]'),
      foam: svg.querySelector(`[fill="${foamFill}"]`),
      label: svg.getAttribute("aria-label"),
    };
  }

  // ys lists the y of every M and L point in a path.
  const ys = (d: string) => Array.from(d.matchAll(/[ML]\s*[\d.]+\s+([\d.]+)/g), (m) => Number(m[1]));

  it("draws exactly as before with no level", () => {
    const glass = art({ onTap: true });
    expect(glass.liquid?.getAttribute("d")).toBe("M11.3 22 L16.8 90 L43.2 90 L48.7 22 Z");
    expect(glass.foam?.getAttribute("d")).toBe("M10.5 12 L11.3 22 L48.7 22 L49.5 12 Z");
    expect(glass.label).toBe("Pale Ale, pint glass");
    cleanup();
    const corny = art({});
    expect([corny.liquid?.getAttribute("y"), corny.liquid?.getAttribute("height")]).toEqual(["20", "66"]);
    expect(corny.liquid?.getAttribute("stroke")).toBeTruthy();
    expect(corny.label).toBe("Pale Ale, keg");
    cleanup();
    const sanke = art({ kegShape: "sanke" });
    expect([sanke.liquid?.getAttribute("y"), sanke.liquid?.getAttribute("height")]).toEqual(["19", "64"]);
  });

  it("fills the glass from the base to the level, with foam on top", () => {
    const { liquid, foam } = art({ onTap: true, level: 0.5 });
    const y = ys(liquid?.getAttribute("d") ?? "");
    expect(Math.min(...y)).toBe(56);
    expect(Math.max(...y)).toBe(90);
    expect(Math.max(...ys(foam?.getAttribute("d") ?? ""))).toBe(56);
    // DisplayPage.css restyles .foam for the light theme.
    expect(foam?.getAttribute("class")).toBe("foam");
  });

  it.each([
    ["corny", 20, 66],
    ["sanke", 19, 64],
  ] as const)("fills the %s keg body from its bottom and keeps the outline", (kegShape, top, height) => {
    const { svg, liquid } = art({ kegShape, level: 0.5 });
    expect(Number(liquid?.getAttribute("height"))).toBe(height / 2);
    expect(Number(liquid?.getAttribute("y")) + Number(liquid?.getAttribute("height"))).toBe(top + height);
    expect(liquid?.getAttribute("fill")).toBe("#9b3200");
    const outline = Array.from(svg.querySelectorAll("rect")).find(
      (r) => r.getAttribute("y") === String(top) && r.getAttribute("height") === String(height),
    );
    expect(outline?.getAttribute("fill")).toBe("none");
    expect(outline?.getAttribute("stroke")).toBeTruthy();
  });

  it.each([true, false])("draws no liquid and no foam at level 0 (onTap %s)", (onTap) => {
    const { liquid, foam } = art({ onTap, level: 0 });
    expect(liquid === null || liquid.getAttribute("fill") === "none").toBe(true);
    expect(foam).toBeNull();
  });

  it("draws a 2% sliver for any beer left, and a full vessel at 1", () => {
    const glass = art({ onTap: true, level: 0.001 });
    const top = Math.min(...ys(glass.liquid?.getAttribute("d") ?? ""));
    expect(top).toBeCloseTo(90 - 0.02 * 68, 1);
    // The foam is at most half the sliver, so the glass still looks nearly empty.
    expect(top - Math.min(...ys(glass.foam?.getAttribute("d") ?? ""))).toBeLessThanOrEqual((90 - top) / 2);
    cleanup();
    expect(art({ onTap: true, level: 1 }).liquid?.getAttribute("d")).toBe("M11.3 22 L16.8 90 L43.2 90 L48.7 22 Z");
    cleanup();
    const full = art({ level: 1 }).liquid;
    expect([full?.getAttribute("y"), full?.getAttribute("height")]).toEqual(["22", "64"]);
  });

  it.each([
    ["corny", 84],
    ["sanke", 81],
  ] as const)("shows the %s keg sliver above its bottom part", (kegShape, bottomTop) => {
    const sliver = art({ kegShape, level: 0.001 }).liquid;
    expect(Number(sliver?.getAttribute("y"))).toBeLessThan(bottomTop);
  });

  it.each([
    [0.6, "60"],
    [0.001, "1"],
    [0.996, "99"],
    [1, "100"],
    [0, "0"],
  ])("labels the glass at level %s with its percent left", (level, pct) => {
    expect(art({ onTap: true, level }).label).toBe(`Pale Ale, pint glass, ${pct}% left`);
  });

  it("labels a keg with its level", () => {
    expect(art({ level: 0.5 }).label).toBe("Pale Ale, keg, 50% left");
  });

  it.each([
    [true, "corny"],
    [false, "corny"],
    [false, "sanke"],
  ] as const)("uses no style attribute with a level (onTap %s, %s)", (onTap, kegShape) => {
    const { svg } = art({ onTap, kegShape, level: 0.4 });
    for (const el of [svg, ...Array.from(svg.querySelectorAll("*"))]) {
      expect(el.hasAttribute("style")).toBe(false);
      expect(el.hasAttribute("id")).toBe(false);
    }
    expect(svg.querySelector("clipPath")).toBeNull();
  });
});

describe("colorForSrm", () => {
  // Same table shape and expectations as TestColorFor in api/internal/beers/srm_test.go.
  const colors = Array.from({ length: 40 }, (_, i) => `#0000${(i + 1).toString(16).padStart(2, "0")}`);

  it("maps every SRM from 0 to 40 like the API", () => {
    expect(colorForSrm(colors, 0)).toBe(colors[0]);
    for (let n = 1; n <= 40; n++) expect(colorForSrm(colors, n)).toBe(colors[n - 1]);
  });

  it("rounds halves up and clamps above the table", () => {
    expect(colorForSrm(colors, 4.4)).toBe(colors[3]);
    expect(colorForSrm(colors, 4.5)).toBe(colors[4]);
    expect(colorForSrm(colors, 60)).toBe(colors[39]);
    expect(colorForSrm(colors, 100)).toBe(colors[39]);
  });

  it("gives null without an SRM or a table", () => {
    expect(colorForSrm(colors, null)).toBeNull();
    expect(colorForSrm([], 5)).toBeNull();
  });
});
