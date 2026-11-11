/** Admin is an admin account as the API returns it. */
export interface Admin {
  id: number;
  username: string;
  created_at: string;
}

/** Token is an API token as the API lists it; it never holds the secret. */
export interface Token {
  id: number;
  name: string;
  scope: "read" | "write";
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

/** BeerStatus is a beer's brewing state. It does not pick the artwork vessel. */
export type BeerStatus = "planned" | "fermenting" | "conditioning" | "ready";

/**
 * Beer is a beer as the API returns it. Optional fields are always present, as null when
 * empty; `color` is the "#rrggbb" display color for `srm`, null without one. `on_tap` is true
 * while a current keg of this beer is on a tap; it picks the artwork vessel.
 */
export interface Beer {
  id: number;
  name: string;
  style: string | null;
  abv: number | null;
  ibu: number | null;
  srm: number | null;
  color: string | null;
  description: string | null;
  brewer: string | null;
  brew_date: string | null;
  status: BeerStatus;
  source: "manual" | "brewfather";
  source_ref: string | null;
  archived_at: string | null;
  on_tap: boolean;
  sync: BeerSync | null;
  created_at: string;
  updated_at: string;
}

/**
 * BeerSync is a Brewfather beer's sync state, null on manual beers. `not_found` means
 * Brewfather no longer has its batch; `archive_pending` means the batch was archived but the
 * beer still has a current keg, so it is archived once that keg is kicked.
 */
export interface BeerSync {
  synced_at: string | null;
  not_found: boolean;
  archive_pending: boolean;
}

/** SyncInterval is how often the API syncs Brewfather on its own. */
export type SyncInterval = "off" | "15m" | "1h" | "6h" | "24h";

/**
 * LastSync is the last sync's outcome. `finished_at`, `ok` and `message` are null while it
 * runs.
 */
export interface LastSync {
  started_at: string;
  finished_at: string | null;
  ok: boolean | null;
  message: string | null;
  updated: number;
  imported: number;
  not_found: number;
}

/**
 * BrewfatherStatus is the body of GET /api/brewfather. `configured` is false when the server
 * has no encryption key; `reconnect_needed` when stored credentials no longer decrypt;
 * `retry_at` is when Brewfather calls resume while its rate limit holds. It never holds the
 * user ID or the API key.
 */
export interface BrewfatherStatus {
  configured: boolean;
  connected: boolean;
  reconnect_needed: boolean;
  interval: SyncInterval;
  sync_running: boolean;
  retry_at: string | null;
  last_sync: LastSync | null;
}

/** BatchSummary is one Brewfather batch that is not archived; `beer_id` is its imported beer or null. */
export interface BatchSummary {
  id: string;
  name: string;
  number: number | null;
  status: BeerStatus;
  brew_date: string | null;
  beer_id: number | null;
}

/**
 * ImportResult is the outcome of importing one batch. `beer` is the new beer, or the existing
 * one for `already_imported`; `adjusted` names beer fields cleaned to pass the beer rules.
 */
export interface ImportResult {
  batch_id: string;
  outcome: "imported" | "already_imported" | "archived" | "not_found" | "rate_limited" | "failed";
  beer: Beer | null;
  adjusted: string[];
  message: string | null;
}

/** KegTypeKey names one row of the API's fixed keg type table. */
export type KegTypeKey =
  | "corny_2_5_gal"
  | "corny_3_gal"
  | "corny_5_gal"
  | "sanke_sixth"
  | "sanke_quarter"
  | "sanke_half"
  | "euro_20_l"
  | "euro_30_l"
  | "euro_50_l";

/** KegType is one row of GET /api/keg-types: its label, default size in litres and drawing. */
export interface KegType {
  type: KegTypeKey;
  label: string;
  size_l: number;
  shape: "corny" | "sanke";
}

/** KegState is derived by the API: kicked, else on a tap, else waiting (on deck). */
export type KegState = "on_tap" | "waiting" | "kicked";

/**
 * Keg is a keg as the API returns it. Volumes are litres with two decimals. `tap_label` is the
 * tap's live label while on a tap, else the label of the last tap it was on.
 */
export interface Keg {
  id: number;
  beer_id: number;
  beer_name: string;
  beer_color: string | null;
  type: KegTypeKey;
  size_l: number;
  remaining_l: number;
  tap_id: number | null;
  tap_label: string | null;
  state: KegState;
  tapped_at: string | null;
  kicked_at: string | null;
  created_at: string;
  updated_at: string;
}

/** Tap is a tap as the API returns it, with its current keg or null. */
export interface Tap {
  id: number;
  position: number;
  label: string;
  enabled: boolean;
  keg: Keg | null;
  created_at: string;
  updated_at: string;
}

/**
 * MenuBeer is a beer as the public menu gives it: display fields only, nulls when empty. `srm`
 * and `description` come on tap beers only, each left out when unset.
 */
export interface MenuBeer {
  name: string;
  style: string | null;
  abv: number | null;
  ibu: number | null;
  color: string | null;
  srm?: number;
  description?: string;
}

/** MenuKeg is a keg as the public menu gives it: its drawing and the fraction left, 0 to 1. */
export interface MenuKeg {
  shape: "corny" | "sanke";
  level: number;
}

/** MenuTap is one enabled tap on the menu; `keg` is null exactly when `beer` is null (empty). */
export interface MenuTap {
  label: string;
  beer: MenuBeer | null;
  keg: MenuKeg | null;
}

/**
 * OnDeckItem is one on-deck entry: a waiting keg (`keg` set, `status` null) or a beer still
 * being made (`keg` null, `status` set).
 */
export interface OnDeckItem {
  beer: MenuBeer;
  keg: MenuKeg | null;
  status: "fermenting" | "conditioning" | null;
}

/** Theme is the TV's color theme. It never changes the admin app. */
export type Theme = "dark" | "light";

/** Units picks how the admin app shows and takes volumes; the API always speaks litres. */
export type Units = "us" | "metric";

/** Layout is the TV layout: today's card grid, one row per item, or taps and on deck side by side. */
export type Layout = "grid" | "list" | "columns";

/** TitleAlign is the TV title's alignment. */
export type TitleAlign = "left" | "center" | "right";

/** FontKey names a font shipped with the app (fonts.ts); `system` is the font from before them. */
export type FontKey = "system" | "inter" | "lora" | "roboto-slab" | "oswald" | "nunito" | "bebas-neue" | "pacifico";

/**
 * MenuDisplay is the public subset of Settings the TV reads from the menu: no units. Colors are
 * lowercase "#rrggbb", or "" for the theme's own color.
 */
export interface MenuDisplay {
  title: string;
  theme: Theme;
  page_seconds: number;
  layout: Layout;
  title_align: TitleAlign;
  title_font: FontKey;
  text_font: FontKey;
  show_style: boolean;
  show_abv: boolean;
  show_ibu: boolean;
  show_srm: boolean;
  show_description: boolean;
  title_color: string;
  text_color: string;
}

/** Settings is the body of GET and PATCH /api/settings: every display setting. */
export interface Settings extends MenuDisplay {
  units: Units;
}

/**
 * Menu is the body of the public GET /api/menu: enabled taps in order, then on deck, and the
 * TV's display settings. `display` is optional because an API older than feature 7 omits it.
 */
export interface Menu {
  taps: MenuTap[];
  on_deck: OnDeckItem[];
  display?: MenuDisplay;
}

/**
 * ApiError is thrown by apiFetch for any non-2xx response. `code` and `message` come from the
 * API's `{"error", "message"}` body (`code` is "unknown" when the body is not that shape), and
 * `retryAfter` is the `Retry-After` header in seconds when present. `fields` maps each invalid
 * field to its message when the API reports a validation failure.
 */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly retryAfter?: number,
    readonly fields?: Record<string, string>,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * apiFetch calls the same-origin API with an optional JSON body and returns the parsed JSON
 * response, or undefined for 204. It throws ApiError for any non-2xx status. `signal`, when
 * given, aborts the request; fetch then rejects with its abort error.
 */
export async function apiFetch<T = unknown>(
  method: string,
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const res = await fetch(path, {
    method,
    signal,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.ok) {
    return (res.status === 204 ? undefined : await res.json()) as T;
  }
  let code = "unknown";
  let message = `Request failed (${res.status}).`;
  let fields: Record<string, string> | undefined;
  try {
    const data = (await res.json()) as { error?: unknown; message?: unknown; fields?: unknown };
    if (typeof data.error === "string" && typeof data.message === "string") {
      code = data.error;
      message = data.message;
    }
    if (data.fields && typeof data.fields === "object") {
      fields = Object.fromEntries(
        Object.entries(data.fields).filter((e): e is [string, string] => typeof e[1] === "string"),
      );
    }
  } catch {
    // Not JSON (for example an nginx error page): keep the generic message.
  }
  const retry = Number.parseInt(res.headers.get("Retry-After") ?? "", 10);
  throw new ApiError(res.status, code, message, Number.isNaN(retry) ? undefined : retry, fields);
}

/** isUnauthenticated reports whether err is a 401 from the API, meaning the session has ended. */
export function isUnauthenticated(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401;
}

/**
 * errorMessage returns the text to show for err: for a 429 with Retry-After, when to retry (the
 * local clock time for Brewfather's rate limit, else how many minutes to wait); otherwise the
 * API's message.
 */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError && err.status === 429 && err.retryAfter !== undefined) {
    if (err.code === "provider_rate_limited") {
      const at = new Date(Date.now() + err.retryAfter * 1000).toLocaleTimeString();
      return `Brewfather's rate limit is reached. Try again after ${at}.`;
    }
    const minutes = Math.max(1, Math.ceil(err.retryAfter / 60));
    return `Too many failed attempts. Try again in ${minutes} ${minutes === 1 ? "minute" : "minutes"}.`;
  }
  return err instanceof Error ? err.message : String(err);
}
