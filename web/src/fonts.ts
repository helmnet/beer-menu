import type { FontKey } from "./api";
// Latin subset only. Each sheet is just @font-face rules: a browser downloads a font file only
// when text on the page uses that family, so the TV fetches at most its two chosen fonts.
import "@fontsource/inter/latin-400.css";
import "@fontsource/inter/latin-700.css";
import "@fontsource/lora/latin-400.css";
import "@fontsource/lora/latin-700.css";
import "@fontsource/roboto-slab/latin-400.css";
import "@fontsource/roboto-slab/latin-700.css";
import "@fontsource/oswald/latin-400.css";
import "@fontsource/oswald/latin-700.css";
import "@fontsource/nunito/latin-400.css";
import "@fontsource/nunito/latin-700.css";
// Bebas Neue and Pacifico ship one weight; bold text in them is synthesized by the browser.
import "@fontsource/bebas-neue/latin-400.css";
import "@fontsource/pacifico/latin-400.css";

/** Font is one font the TV offers: its settings key and its name in the admin app. */
export interface Font {
  key: FontKey;
  label: string;
}

/**
 * FONTS is every font the TV offers, in the order the admin app lists them; `system` (the font
 * from before the font settings) comes first. The files ship with the app and nginx serves them
 * from its own origin, so no font comes from another site (research R5). Each font's class,
 * `font-<key>`, is in DisplayPage.css and falls back to the system font if its file fails to load.
 */
export const FONTS: readonly Font[] = [
  { key: "system", label: "System" },
  { key: "inter", label: "Inter" },
  { key: "lora", label: "Lora" },
  { key: "roboto-slab", label: "Roboto Slab" },
  { key: "oswald", label: "Oswald" },
  { key: "nunito", label: "Nunito" },
  { key: "bebas-neue", label: "Bebas Neue" },
  { key: "pacifico", label: "Pacifico" },
];
