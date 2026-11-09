// US units until feature 7 adds a units setting (FR-065). The API only speaks litres.

/** LITRES_PER_GALLON is one US liquid gallon in litres. */
export const LITRES_PER_GALLON = 3.785411784;

/** LITRES_PER_FL_OZ is one US fluid ounce (1/128 US gallon) in litres. */
export const LITRES_PER_FL_OZ = 0.0295735295625;

// The API stores litres rounded to 0.01 (research R2); rounding here sends what it will keep.
const roundL = (l: number) => Math.round(l * 100) / 100;

/** litresToGallons converts litres to US gallons, unrounded. */
export function litresToGallons(l: number): number {
  return l / LITRES_PER_GALLON;
}

/** gallonsToLitres converts US gallons to litres rounded to 0.01. */
export function gallonsToLitres(g: number): number {
  return roundL(g * LITRES_PER_GALLON);
}

/** flOzToLitres converts US fluid ounces to litres rounded to 0.01. */
export function flOzToLitres(oz: number): number {
  return roundL(oz * LITRES_PER_FL_OZ);
}

/** gallons is litres as US gallons to two decimals, the text of a gallons input: "5.00". */
export function gallons(l: number): string {
  return litresToGallons(l).toFixed(2);
}

/** formatGallons shows litres as US gallons to two decimals, as "5.00 gal". */
export function formatGallons(l: number): string {
  return `${gallons(l)} gal`;
}

/**
 * inputToLitres converts an input's text with convert (gallonsToLitres or flOzToLitres). Text
 * that is not a number is returned as is so the API refuses it by field, because
 * JSON.stringify would turn NaN into null.
 */
export function inputToLitres(text: string, convert: (n: number) => number): string | number {
  const n = Number(text);
  return text.trim() === "" || Number.isNaN(n) ? text : convert(n);
}
