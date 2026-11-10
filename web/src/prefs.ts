/**
 * prefs reads and writes the admin app's per-browser preferences in localStorage: the theme
 * (`beer-menu.admin.theme`: light, dark or system, default system) and the side panel width
 * (`beer-menu.admin.nav-width`: integer CSS px, default 256). They are separate from the TV
 * theme on the server. Every storage access is guarded, so blocked storage (private mode) or
 * a missing or invalid value falls back to the defaults and nothing throws.
 */

/** Theme is the admin app theme choice; system follows the device. */
export type Theme = "light" | "dark" | "system";

const THEME_KEY = "beer-menu.admin.theme";
const WIDTH_KEY = "beer-menu.admin.nav-width";

/** DEFAULT_NAV_WIDTH, MIN_NAV_WIDTH and MAX_NAV_WIDTH bound the side panel, in CSS px. */
const DEFAULT_NAV_WIDTH = 256;
export const MIN_NAV_WIDTH = 200;
const MAX_NAV_WIDTH = 400;

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Blocked storage: the choice applies for this visit only.
  }
}

/** loadTheme returns the saved theme, or system when none or an invalid one is saved. */
export function loadTheme(): Theme {
  const v = read(THEME_KEY);
  return v === "light" || v === "dark" ? v : "system";
}

/** saveTheme stores theme for this browser. */
export function saveTheme(theme: Theme) {
  write(THEME_KEY, theme);
}

/**
 * clampNavWidth limits px to 200–400 and to at most half of windowWidth, as a whole number.
 * Half the window wins over the 200 minimum, so the main area always keeps half the screen.
 */
export function clampNavWidth(px: number, windowWidth: number): number {
  return Math.min(Math.max(Math.round(px), MIN_NAV_WIDTH), MAX_NAV_WIDTH, Math.floor(windowWidth / 2));
}

/**
 * loadNavWidth returns the saved width clamped to the current window, or 256 when none or a
 * non-integer is saved. The stored value itself is left alone.
 */
export function loadNavWidth(windowWidth = window.innerWidth): number {
  const v = read(WIDTH_KEY);
  const px = v !== null && /^\d+$/.test(v) ? Number(v) : DEFAULT_NAV_WIDTH;
  return clampNavWidth(px, windowWidth);
}

/** saveNavWidth stores px for this browser. */
export function saveNavWidth(px: number) {
  write(WIDTH_KEY, String(px));
}
