import { fireEvent, screen, within } from "@testing-library/react";
import { onTestFinished, vi } from "vitest";
import type {
  BatchSummary,
  Beer,
  BrewfatherStatus,
  ImportResult,
  Keg,
  KegType,
  Menu,
  Passkey,
  Settings,
  Tap,
  TwoFactorState,
} from "./api";

/** Reply is one stubbed API response. */
export interface Reply {
  status: number;
  body?: unknown;
  headers?: Record<string, string>;
}

/** Call is one request the stub received, with its JSON body parsed. */
export interface Call {
  key: string;
  body?: unknown;
}

/**
 * stubApi replaces fetch for a test. `routes` maps "METHOD /path" to a reply and may be
 * changed later in the test; an unlisted request fails the call. Returns the received calls.
 */
export function stubApi(routes: Record<string, Reply>): Call[] {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      const key = `${init?.method ?? "GET"} ${path}`;
      calls.push({ key, body: init?.body ? JSON.parse(init.body as string) : undefined });
      const reply = routes[key];
      if (!reply) throw new Error(`unexpected request ${key}`);
      return new Response(reply.body === undefined ? null : JSON.stringify(reply.body), {
        status: reply.status,
        headers: reply.headers,
      });
    }),
  );
  return calls;
}

/** testPassword returns a random password so no fixed password is in the repo. */
export function testPassword(): string {
  return crypto.randomUUID();
}

/** owner is the Admin body the stubs return. */
export const owner = { id: 1, username: "owner", created_at: "2026-11-08T12:00:00Z", two_factor: false };

/** twoFactorState returns a GET /api/2fa body for stubs: no factor, both kinds available. */
export function twoFactorState(overrides: Partial<TwoFactorState> = {}): TwoFactorState {
  return {
    enabled: false,
    required: false,
    totp_available: true,
    passkeys_available: true,
    passkeys_origin: "http://localhost:3000",
    totp: null,
    passkeys: [],
    recovery_codes_left: 0,
    ...overrides,
  };
}

/** passkey returns a Passkey body for stubs: "Phone", never used. */
export function passkey(overrides: Partial<Passkey> = {}): Passkey {
  return { id: 1, name: "Phone", created_at: "2026-11-10T12:00:00Z", last_used_at: null, ...overrides };
}

/** webAuthnOptions is a WebAuthn options body for stubs; stubPasskeys passes it through unparsed. */
export const webAuthnOptions = { status: 200, body: { publicKey: { challenge: "Y2hhbGxlbmdl" } } };

/**
 * stubPasskeys makes this page passkey-capable for one test: a secure context, PublicKeyCredential
 * whose JSON helpers pass options through, and navigator.credentials whose create and get resolve
 * to a credential with toJSON() = `reply`, or reject with NotAllowedError (a closed prompt) for
 * "cancel". Returns the mock behind create and get.
 */
export function stubPasskeys(reply: object | "cancel" = { id: "credential" }) {
  const prompt = vi.fn(async () => {
    if (reply === "cancel") throw new DOMException("The operation was not allowed.", "NotAllowedError");
    return { toJSON: () => reply };
  });
  vi.stubGlobal("isSecureContext", true);
  vi.stubGlobal("PublicKeyCredential", {
    parseCreationOptionsFromJSON: (o: unknown) => o,
    parseRequestOptionsFromJSON: (o: unknown) => o,
  });
  Object.defineProperty(navigator, "credentials", { value: { create: prompt, get: prompt }, configurable: true });
  onTestFinished(() => {
    Reflect.deleteProperty(navigator, "credentials");
  });
  return prompt;
}

/**
 * stubPendingPasskey stubs a passkey prompt that waits for `approve(reply)`; `signal()` is the
 * AbortSignal the prompt was given.
 */
export function stubPendingPasskey() {
  let signal: AbortSignal | undefined;
  let approve: (cred: { toJSON: () => object }) => void = () => {};
  stubPasskeys().mockImplementationOnce((opts?: { signal?: AbortSignal }) => {
    signal = opts?.signal;
    return new Promise((resolve) => (approve = resolve));
  });
  return { signal: () => signal, approve: (reply: object) => approve({ toJSON: () => reply }) };
}

/** inDialog queries inside the open dialog titled `title`. */
export const inDialog = (title: string) => within(screen.getByRole("dialog", { name: title }));

/** clickInDialog clicks the button named `button` in the open dialog titled `title`. */
export const clickInDialog = (title: string, button: string) =>
  fireEvent.click(inDialog(title).getByRole("button", { name: button }));

/** nextTask waits one macrotask, so pending promise chains (a stubbed fetch, a prompt) run on. */
export const nextTask = () => new Promise((resolve) => setTimeout(resolve, 0));

/** beer returns a Beer body for stubs: a current, ready, manual beer with no optional data. */
export function beer(overrides: Partial<Beer> = {}): Beer {
  return {
    id: 1,
    name: "Pale",
    style: null,
    abv: null,
    ibu: null,
    srm: null,
    color: null,
    description: null,
    brewer: null,
    brew_date: null,
    status: "ready",
    source: "manual",
    source_ref: null,
    archived_at: null,
    on_tap: false,
    sync: null,
    created_at: "2026-11-09T12:00:00Z",
    updated_at: "2026-11-09T12:00:00Z",
    ...overrides,
  };
}

/** brewfatherStatus returns a GET /api/brewfather body for stubs: set up but not connected. */
export function brewfatherStatus(overrides: Partial<BrewfatherStatus> = {}): BrewfatherStatus {
  return {
    configured: true,
    connected: false,
    reconnect_needed: false,
    interval: "off",
    sync_running: false,
    retry_at: null,
    last_sync: null,
    ...overrides,
  };
}

/** batch returns a BatchSummary for stubs: a fermenting batch that is not imported. */
export function batch(overrides: Partial<BatchSummary> = {}): BatchSummary {
  return {
    id: "batch-1",
    name: "Pale Ale",
    number: 1,
    status: "fermenting",
    brew_date: "2026-11-01",
    beer_id: null,
    ...overrides,
  };
}

/** importResult returns an ImportResult for stubs: batch-1 imported as a new Brewfather beer. */
export function importResult(overrides: Partial<ImportResult> = {}): ImportResult {
  return {
    batch_id: "batch-1",
    outcome: "imported",
    beer: beer({ name: "Pale Ale", source: "brewfather", source_ref: "batch-1" }),
    adjusted: [],
    message: null,
    ...overrides,
  };
}

/** tap returns a Tap body for stubs: an enabled, empty tap. */
export function tap(overrides: Partial<Tap> = {}): Tap {
  return {
    id: 1,
    position: 1,
    label: "Tap 1",
    enabled: true,
    keg: null,
    created_at: "2026-11-09T12:00:00Z",
    updated_at: "2026-11-09T12:00:00Z",
    ...overrides,
  };
}

/** keg returns a Keg body for stubs: a full, waiting 5 gallon corny keg of beer 1. */
export function keg(overrides: Partial<Keg> = {}): Keg {
  return {
    id: 1,
    beer_id: 1,
    beer_name: "Pale",
    beer_color: null,
    type: "corny_5_gal",
    size_l: 18.93,
    remaining_l: 18.93,
    tap_id: null,
    tap_label: null,
    state: "waiting",
    tapped_at: null,
    kicked_at: null,
    created_at: "2026-11-09T12:00:00Z",
    updated_at: "2026-11-09T12:00:00Z",
    ...overrides,
  };
}

/** kegTypes is the GET /api/keg-types body, research R4's table. */
export const kegTypes: KegType[] = [
  { type: "corny_2_5_gal", label: "Corny 2.5 gal", size_l: 9.46, shape: "corny" },
  { type: "corny_3_gal", label: "Corny 3 gal", size_l: 11.36, shape: "corny" },
  { type: "corny_5_gal", label: "Corny 5 gal", size_l: 18.93, shape: "corny" },
  { type: "sanke_sixth", label: "Sixth barrel", size_l: 19.56, shape: "sanke" },
  { type: "sanke_quarter", label: "Quarter barrel", size_l: 29.34, shape: "sanke" },
  { type: "sanke_half", label: "Half barrel", size_l: 58.67, shape: "sanke" },
  { type: "euro_20_l", label: "Euro 20 L", size_l: 20, shape: "sanke" },
  { type: "euro_30_l", label: "Euro 30 L", size_l: 30, shape: "sanke" },
  { type: "euro_50_l", label: "Euro 50 L", size_l: 50, shape: "sanke" },
];

/** tvDefaults is every TV setting added in feature 10 at its default. */
const tvDefaults = {
  layout: "grid",
  title_align: "left",
  title_font: "system",
  text_font: "system",
  show_style: true,
  show_abv: true,
  show_ibu: true,
  show_srm: false,
  show_description: false,
  title_color: "",
  text_color: "",
} as const;

/** menu returns a GET /api/menu body for stubs: no taps, nothing on deck, default display. */
export function menu(overrides: Partial<Menu> = {}): Menu {
  return { taps: [], on_deck: [], display: { title: "", theme: "dark", page_seconds: 15, ...tvDefaults }, ...overrides };
}

/** settings returns a GET /api/settings body for stubs: the defaults, with overrides. */
export function settings(overrides: Partial<Settings> = {}): Settings {
  return { title: "", theme: "dark", units: "us", page_seconds: 15, ...tvDefaults, ...overrides };
}
