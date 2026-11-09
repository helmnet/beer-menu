import { vi } from "vitest";
import type { Beer } from "./api";

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
    created_at: "2026-11-09T12:00:00Z",
    updated_at: "2026-11-09T12:00:00Z",
    ...overrides,
  };
}
