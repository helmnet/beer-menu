import { describe, expect, it } from "vitest";
import { flOzToLitres, formatGallons, gallonsToLitres, inputToLitres, litresToGallons } from "./units";

// Default sizes of every keg type in research R4.
const sizes = [9.46, 11.36, 18.93, 19.56, 29.34, 58.67, 20, 30, 50];

describe("units", () => {
  it.each(sizes)("round-trips %s L through two-decimal gallons within 0.02 L", (l) => {
    // Two-decimal gallons are up to 0.005 gal (about 0.019 L) off the litres.
    expect(Math.abs(gallonsToLitres(Number(litresToGallons(l).toFixed(2))) - l)).toBeLessThanOrEqual(0.02);
  });

  it.each([0.01, 2.5, 5.17, 7.93, 15.5])("stores %s gal within 0.005 gal of what was typed", (g) => {
    expect(Math.abs(litresToGallons(gallonsToLitres(g)) - g)).toBeLessThanOrEqual(0.005);
  });

  it("rounds litres to 0.01", () => {
    expect(gallonsToLitres(5)).toBe(18.93);
    expect(flOzToLitres(16)).toBe(0.47);
  });

  it("formats gallons to two decimals", () => {
    expect(formatGallons(18.93)).toBe("5.00 gal");
    expect(formatGallons(58.67)).toBe("15.50 gal");
    expect(formatGallons(0)).toBe("0.00 gal");
  });

  it("converts number inputs and passes other text through for the API to refuse", () => {
    expect(inputToLitres("16", flOzToLitres)).toBe(0.47);
    expect(inputToLitres("5", gallonsToLitres)).toBe(18.93);
    expect(inputToLitres("", gallonsToLitres)).toBe("");
    expect(inputToLitres(" ", gallonsToLitres)).toBe(" ");
    expect(inputToLitres("abc", gallonsToLitres)).toBe("abc");
  });
});
