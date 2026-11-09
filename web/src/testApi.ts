import { vi } from "vitest";
import type { Beer, Keg, KegType, Tap } from "./api";

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
export const owner = { id: 1, username: "owner", created_at: "2026-11-08T12:00:00Z" };

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
    created_at: "2026-11-09T12:00:00Z",
    updated_at: "2026-11-09T12:00:00Z",
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
