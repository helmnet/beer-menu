// Volumes in the admin app follow the units setting (research R9). The API only speaks litres.
import type { Units } from "./api";

/** LITRES_PER_GALLON is one US liquid gallon in litres. */
export const LITRES_PER_GALLON = 3.785411784;

/** LITRES_PER_FL_OZ is one US fluid ounce (1/128 US gallon) in litres. */
export const LITRES_PER_FL_OZ = 0.0295735295625;

// The API stores litres rounded to 0.01 (research R2); rounding here sends what it will keep.
const roundL = (l: number) => Math.round(l * 100) / 100;

/** table holds, per units, litres per volume unit and per pour unit, the labels and the pour prefill. */
const table: Record<Units, { volume: number; volumeUnit: string; pour: number; pourUnit: string; prefill: number }> = {
  us: { volume: LITRES_PER_GALLON, volumeUnit: "gal", pour: LITRES_PER_FL_OZ, pourUnit: "fl oz", prefill: 16 },
  // 500 mL is a common metric serving.
  metric: { volume: 1, volumeUnit: "L", pour: 0.001, pourUnit: "mL", prefill: 500 },
};

/** volumeText is litres in units to two decimals, the text of a volume input: "5.00" or "18.93". */
export function volumeText(l: number, units: Units): string {
  return (l / table[units].volume).toFixed(2);
}

/** formatVolume shows litres in units to two decimals with the unit: "5.00 gal" or "18.93 L". */
export function formatVolume(l: number, units: Units): string {
  return `${volumeText(l, units)} ${table[units].volumeUnit}`;
}

/** volumeToLitres converts a volume typed in units (US gallons or litres) to litres rounded to 0.01. */
export function volumeToLitres(n: number, units: Units): number {
  return roundL(n * table[units].volume);
}

/** pourToLitres converts a pour typed in units (US fl oz or mL) to litres rounded to 0.01. */
export function pourToLitres(n: number, units: Units): number {
  return roundL(n * table[units].pour);
}

/** volumeUnit is the label of a volume in units: "gal" or "L". */
export const volumeUnit = (units: Units) => table[units].volumeUnit;

/** pourUnit is the label of a pour in units: "fl oz" or "mL". */
export const pourUnit = (units: Units) => table[units].pourUnit;

/** pourPrefill is the pour form's starting amount in units: 16 fl oz or 500 mL. */
export const pourPrefill = (units: Units) => table[units].prefill;

/**
 * inputToLitres converts an input's text with convert (for example a volumeToLitres call).
 * Text that is not a number is returned as is so the API refuses it by field, because
 * JSON.stringify would turn NaN into null.
 */
export function inputToLitres(text: string, convert: (n: number) => number): string | number {
  const n = Number(text);
  return text.trim() === "" || Number.isNaN(n) ? text : convert(n);
}
