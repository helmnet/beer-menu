import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiFetch } from "./api";

function stubFetch(status: number, body?: unknown, headers: Record<string, string> = {}) {
  const fetchMock = vi.fn(async () =>
    new Response(body === undefined ? null : JSON.stringify(body), { status, headers }),
  );
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiFetch", () => {
  it("sends JSON same-origin and returns the parsed body", async () => {
    const fetchMock = stubFetch(200, { needed: true });
    const got = await apiFetch<{ needed: boolean }>("POST", "/api/setup", { username: "owner" });
    expect(got).toEqual({ needed: true });
    expect(fetchMock).toHaveBeenCalledWith("/api/setup", {
      method: "POST",
      credentials: "same-origin",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: "owner" }),
    });
  });

  it("sends no body or content type without a body", async () => {
    const fetchMock = stubFetch(200, {});
    await apiFetch("GET", "/api/session");
    expect(fetchMock).toHaveBeenCalledWith("/api/session", {
      method: "GET",
      credentials: "same-origin",
      headers: {},
      body: undefined,
    });
  });

  it("returns undefined on 204", async () => {
    stubFetch(204);
    expect(await apiFetch("DELETE", "/api/session")).toBeUndefined();
  });

  it("throws ApiError from the error body", async () => {
    stubFetch(409, { error: "setup_done", message: "Setup is already complete." });
    const err = await apiFetch("POST", "/api/setup", {}).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({
      status: 409,
      code: "setup_done",
      message: "Setup is already complete.",
      retryAfter: undefined,
    });
  });

  it("keeps the per-field messages of a validation error", async () => {
    stubFetch(400, {
      error: "invalid_request",
      message: "Some fields are invalid.",
      fields: { abv: "Must be 0 to 25 with at most one decimal place.", name: "Required." },
    });
    const err = await apiFetch("POST", "/api/beers", {}).catch((e: unknown) => e);
    expect(err).toMatchObject({
      status: 400,
      code: "invalid_request",
      fields: { abv: "Must be 0 to 25 with at most one decimal place.", name: "Required." },
    });
  });

  it("has no fields when the error body has none", async () => {
    stubFetch(409, { error: "setup_done", message: "done" });
    const err = await apiFetch("POST", "/api/setup", {}).catch((e: unknown) => e);
    expect((err as ApiError).fields).toBeUndefined();
  });

  it("reads Retry-After in seconds", async () => {
    stubFetch(
      429,
      { error: "too_many_attempts", message: "Too many failed attempts." },
      { "Retry-After": "840" },
    );
    const err = await apiFetch("POST", "/api/session", {}).catch((e: unknown) => e);
    expect(err).toMatchObject({ status: 429, code: "too_many_attempts", retryAfter: 840 });
  });

  it("throws ApiError when the error body is not JSON", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("bad gateway", { status: 502 })));
    const err = await apiFetch("GET", "/api/setup").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 502, code: "unknown" });
  });
});
