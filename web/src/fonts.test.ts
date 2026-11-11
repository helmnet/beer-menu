import { describe, expect, it } from "vitest";
import { FONTS } from "./fonts";

describe("FONTS", () => {
  it("lists the 8 font keys in data-model order, system first, each with a label", () => {
    expect(FONTS.map((f) => f.key)).toEqual(["system", "inter", "lora", "roboto-slab", "oswald", "nunito", "bebas-neue", "pacifico"]);
    expect(FONTS.map((f) => f.label)).toEqual(["System", "Inter", "Lora", "Roboto Slab", "Oswald", "Nunito", "Bebas Neue", "Pacifico"]);
  });
});
