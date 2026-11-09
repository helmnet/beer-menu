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

/**
 * ApiError is thrown by apiFetch for any non-2xx response. `code` and `message` come from the
 * API's `{"error", "message"}` body (`code` is "unknown" when the body is not that shape), and
 * `retryAfter` is the `Retry-After` header in seconds when present.
 */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly retryAfter?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * apiFetch calls the same-origin API with an optional JSON body and returns the parsed JSON
 * response, or undefined for 204. It throws ApiError for any non-2xx status.
 */
export async function apiFetch<T = unknown>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: body === undefined ? {} : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (res.ok) {
    return (res.status === 204 ? undefined : await res.json()) as T;
  }
  let code = "unknown";
  let message = `Request failed (${res.status}).`;
  try {
    const data = (await res.json()) as { error?: unknown; message?: unknown };
    if (typeof data.error === "string" && typeof data.message === "string") {
      code = data.error;
      message = data.message;
    }
  } catch {
    // Not JSON (for example an nginx error page): keep the generic message.
  }
  const retry = Number.parseInt(res.headers.get("Retry-After") ?? "", 10);
  throw new ApiError(res.status, code, message, Number.isNaN(retry) ? undefined : retry);
}

/** isUnauthenticated reports whether err is a 401 from the API, meaning the session has ended. */
export function isUnauthenticated(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401;
}

/**
 * errorMessage returns the text to show for err: for a 429 with Retry-After, how many minutes
 * to wait; otherwise the API's message.
 */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError && err.status === 429 && err.retryAfter !== undefined) {
    const minutes = Math.max(1, Math.ceil(err.retryAfter / 60));
    return `Too many failed attempts. Try again in ${minutes} ${minutes === 1 ? "minute" : "minutes"}.`;
  }
  return err instanceof Error ? err.message : String(err);
}
