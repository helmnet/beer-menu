import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createPasskey, getPasskey, passkeysSupported } from "./webauthn";

const creation = {
  publicKey: {
    challenge: "Y2hhbGxlbmdl",
    rp: { name: "Beer Menu" },
    user: { id: "AAAAAAAAAAE", name: "owner", displayName: "owner" },
    pubKeyCredParams: [{ type: "public-key" as const, alg: -7 }],
  },
};
const request = { publicKey: { challenge: "Y2hhbGxlbmdl" } };

// stubCredentials gives navigator a credentials container with create and get.
function stubCredentials(create: unknown, get: unknown) {
  Object.defineProperty(navigator, "credentials", { value: { create, get }, configurable: true });
}

beforeEach(() => {
  vi.stubGlobal("isSecureContext", true);
  vi.stubGlobal("PublicKeyCredential", {
    parseCreationOptionsFromJSON: vi.fn((o: object) => ({ parsed: "create", ...o })),
    parseRequestOptionsFromJSON: vi.fn((o: object) => ({ parsed: "get", ...o })),
  });
  stubCredentials(vi.fn(), vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  Reflect.deleteProperty(navigator, "credentials");
});

describe("passkeysSupported", () => {
  it("is true on the configured origin in a capable browser", () => {
    expect(passkeysSupported(location.origin)).toBe(true);
  });

  it("is false without an origin or on another host name or the IP address", () => {
    expect(passkeysSupported(null)).toBe(false);
    expect(passkeysSupported(undefined)).toBe(false);
    expect(passkeysSupported("https://taplist.example.org")).toBe(false);
    expect(passkeysSupported(location.origin.replace("localhost", "127.0.0.1"))).toBe(false);
  });

  it("is false outside a secure context", () => {
    vi.stubGlobal("isSecureContext", false);
    expect(passkeysSupported(location.origin)).toBe(false);
  });

  it("is false without PublicKeyCredential or its JSON helpers", () => {
    vi.stubGlobal("PublicKeyCredential", undefined);
    expect(passkeysSupported(location.origin)).toBe(false);
    vi.stubGlobal("PublicKeyCredential", { parseRequestOptionsFromJSON: vi.fn() });
    expect(passkeysSupported(location.origin)).toBe(false);
    vi.stubGlobal("PublicKeyCredential", { parseCreationOptionsFromJSON: vi.fn() });
    expect(passkeysSupported(location.origin)).toBe(false);
  });
});

describe("createPasskey and getPasskey", () => {
  it("create with the parsed options and return toJSON()", async () => {
    const create = vi.fn(async () => ({ toJSON: () => ({ id: "made" }) }));
    const get = vi.fn(async () => ({ toJSON: () => ({ id: "used" }) }));
    stubCredentials(create, get);
    expect(await createPasskey(creation)).toEqual({ id: "made" });
    expect(create).toHaveBeenCalledWith({ publicKey: { parsed: "create", ...creation.publicKey } });
    expect(await getPasskey(request)).toEqual({ id: "used" });
    expect(get).toHaveBeenCalledWith({ publicKey: { parsed: "get", ...request.publicKey } });
  });

  it("map a cancelled prompt (NotAllowedError) to cancelled", async () => {
    const cancel = vi.fn(async () => {
      throw new DOMException("The operation either timed out or was not allowed.", "NotAllowedError");
    });
    stubCredentials(cancel, cancel);
    expect(await createPasskey(creation)).toBe("cancelled");
    expect(await getPasskey(request)).toBe("cancelled");
  });

  it("say when the device already has a passkey here", async () => {
    stubCredentials(
      vi.fn(async () => {
        throw new DOMException("x", "InvalidStateError");
      }),
      vi.fn(),
    );
    await expect(createPasskey(creation)).rejects.toThrow("This device already has a passkey for this account.");
  });

  it("pass other errors on", async () => {
    const boom = new DOMException("Security problem", "SecurityError");
    stubCredentials(vi.fn(), vi.fn(async () => Promise.reject(boom)));
    await expect(getPasskey(request)).rejects.toBe(boom);
  });
});
