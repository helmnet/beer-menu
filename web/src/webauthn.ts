import { useEffect, useRef } from "react";
import { lowerModals } from "./components/Modal";

/**
 * WebAuthnOptions is the API's `{ publicKey }` options for a passkey prompt, in the WebAuthn
 * Level 3 JSON form the browser's parse helpers take.
 */
export interface WebAuthnOptions<T> {
  publicKey: T;
}

/**
 * passkeysSupported reports whether this page can use passkeys for a server whose passkeys work
 * at `origin` (passkeys_origin): only on exactly that origin, in a secure context, with
 * PublicKeyCredential and its JSON helpers (research R2). Another host name or the IP address
 * gives false, because the prompt would always fail there (FR-016).
 */
export function passkeysSupported(origin: string | null | undefined): boolean {
  return (
    !!origin &&
    origin === location.origin &&
    globalThis.isSecureContext === true &&
    typeof globalThis.PublicKeyCredential?.parseCreationOptionsFromJSON === "function" &&
    typeof globalThis.PublicKeyCredential.parseRequestOptionsFromJSON === "function" &&
    !!navigator.credentials
  );
}

/** PasskeyReply is the credential.toJSON() to send to the API, or "cancelled" when the admin closed the prompt. */
export type PasskeyReply = object | "cancelled";

/**
 * createPasskey shows the device prompt to make a passkey from creation options and returns its
 * toJSON(), or "cancelled" when the admin closes or times out the prompt (NotAllowedError) or
 * `signal` aborts it (AbortError). A device that already holds a passkey for this account
 * (InvalidStateError) throws an Error saying so; other errors pass on.
 */
export function createPasskey(
  options: WebAuthnOptions<PublicKeyCredentialCreationOptionsJSON>,
  signal?: AbortSignal,
): Promise<PasskeyReply> {
  return prompt(
    () => navigator.credentials.create({ publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(options.publicKey), signal }),
    signal,
  );
}

/**
 * getPasskey shows the device prompt to use a passkey for request options and returns its
 * toJSON(), or "cancelled" when the admin closes or times out the prompt (NotAllowedError) or
 * `signal` aborts it (AbortError). Other errors pass on.
 */
export function getPasskey(
  options: WebAuthnOptions<PublicKeyCredentialRequestOptionsJSON>,
  signal?: AbortSignal,
): Promise<PasskeyReply> {
  return prompt(
    () => navigator.credentials.get({ publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey), signal }),
    signal,
  );
}

/**
 * usePromptAbort lets a dialog cancel the passkey prompt it started: closing the dialog does not
 * stop the browser's prompt, and a password manager's prompt would otherwise linger (FR-010).
 * `signal()` starts a new prompt's signal; `abort()` cancels it, as does unmounting.
 */
export function usePromptAbort() {
  const current = useRef<AbortController | null>(null);
  useEffect(() => () => current.current?.abort(), []);
  return {
    signal: () => (current.current = new AbortController()).signal,
    abort: () => current.current?.abort(),
  };
}

// prompt lowers open dialogs while the device prompt runs, so password-manager prompts drawn in
// the page can be used, and restores them however the prompt ends. It stops waiting when `signal`
// aborts, because an extension that ignores the signal would keep the page unclickable meanwhile.
async function prompt(run: () => Promise<Credential | null>, signal?: AbortSignal): Promise<PasskeyReply> {
  if (signal?.aborted) return "cancelled"; // no lower/restore flicker for a prompt already cancelled
  const restore = lowerModals();
  let onAbort = () => {};
  const aborted = new Promise<never>((_, reject) => {
    onAbort = () => reject(new DOMException("Aborted", "AbortError"));
    signal?.addEventListener("abort", onAbort, { once: true });
  });
  try {
    const cred = (await Promise.race([run(), aborted])) as PublicKeyCredential | null;
    return cred ? cred.toJSON() : "cancelled";
  } catch (err) {
    if (err instanceof DOMException && (err.name === "NotAllowedError" || err.name === "AbortError")) return "cancelled";
    if (err instanceof DOMException && err.name === "InvalidStateError") {
      throw new Error("This device already has a passkey for this account.");
    }
    throw err;
  } finally {
    signal?.removeEventListener("abort", onAbort);
    restore();
  }
}
