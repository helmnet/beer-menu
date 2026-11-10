import { describe, expect, it } from "vitest";
import type { Units } from "./api";
import {
  formatVolume,
  inputToLitres,
  LITRES_PER_GALLON,
  pourPrefill,
  pourToLitres,
  pourUnit,
  volumeText,
  volumeToLitres,
  volumeUnit,
} from "./units";

// Default sizes of every keg type in research R4.
const sizes = [9.46, 11.36, 18.93, 19.56, 29.34, 58.67, 20, 30, 50];

describe("units", () => {
  it.each(sizes)("round-trips %s L through two-decimal gallons within 0.02 L", (l) => {
    // Two-decimal gallons are up to 0.005 gal (about 0.019 L) off the litres.
    expect(Math.abs(volumeToLitres(Number(volumeText(l, "us")), "us") - l)).toBeLessThanOrEqual(0.02);
  });

  it.each(sizes)("round-trips %s L through metric exactly", (l) => {
    expect(volumeToLitres(Number(volumeText(l, "metric")), "metric")).toBe(l);
  });

  it.each([0.01, 2.5, 5.17, 7.93, 15.5])("stores %s gal within 0.005 gal of what was typed", (g) => {
    expect(Math.abs(volumeToLitres(g, "us") / LITRES_PER_GALLON - g)).toBeLessThanOrEqual(0.005);
  });

  it("formats volumes to two decimals with their unit", () => {
    expect(formatVolume(18.93, "us")).toBe("5.00 gal");
    expect(formatVolume(58.67, "us")).toBe("15.50 gal");
    expect(formatVolume(0, "us")).toBe("0.00 gal");
    expect(formatVolume(18.93, "metric")).toBe("18.93 L");
    expect(formatVolume(20, "metric")).toBe("20.00 L");
  });

  it("gives volume input text without the unit", () => {
    expect(volumeText(18.93, "us")).toBe("5.00");
    expect(volumeText(18.93, "metric")).toBe("18.93");
  });

  it("converts to litres rounded to 0.01", () => {
    expect(volumeToLitres(5, "us")).toBe(18.93);
    expect(volumeToLitres(18.93, "metric")).toBe(18.93);
    expect(volumeToLitres(18.934, "metric")).toBe(18.93);
    expect(pourToLitres(16, "us")).toBe(0.47);
    expect(pourToLitres(500, "metric")).toBe(0.5);
  });

  it("labels and prefills per units", () => {
    expect(volumeUnit("us")).toBe("gal");
    expect(volumeUnit("metric")).toBe("L");
    expect(pourUnit("us")).toBe("fl oz");
    expect(pourUnit("metric")).toBe("mL");
    expect(pourPrefill("us")).toBe(16);
    expect(pourPrefill("metric")).toBe(500);
  });

  it("never changes a litre value when the units switch us, metric, us", () => {
    for (const l of sizes) {
      const shown = (["us", "metric", "us"] as Units[]).map((u) => volumeText(l, u));
      expect(shown[2]).toBe(shown[0]);
      expect(volumeToLitres(Number(shown[1]), "metric")).toBe(l);
    }
  });

  it("converts number inputs and passes other text through for the API to refuse", () => {
    const pour = (n: number) => pourToLitres(n, "us");
    const gal = (n: number) => volumeToLitres(n, "us");
    expect(inputToLitres("16", pour)).toBe(0.47);
    expect(inputToLitres("5", gal)).toBe(18.93);
    expect(inputToLitres("", gal)).toBe("");
    expect(inputToLitres(" ", gal)).toBe(" ");
    expect(inputToLitres("abc", gal)).toBe("abc");
    expect(inputToLitres("abc", (n) => volumeToLitres(n, "metric"))).toBe("abc");
  });
});
