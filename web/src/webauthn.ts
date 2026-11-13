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
 * toJSON(), or "cancelled" when the admin closes or times out the prompt (NotAllowedError). A
 * device that already holds a passkey for this account (InvalidStateError) throws an Error
 * saying so; other errors pass on.
 */
export function createPasskey(options: WebAuthnOptions<PublicKeyCredentialCreationOptionsJSON>): Promise<PasskeyReply> {
  return prompt(() =>
    navigator.credentials.create({ publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(options.publicKey) }),
  );
}

/**
 * getPasskey shows the device prompt to use a passkey for request options and returns its
 * toJSON(), or "cancelled" when the admin closes or times out the prompt (NotAllowedError).
 * Other errors pass on.
 */
export function getPasskey(options: WebAuthnOptions<PublicKeyCredentialRequestOptionsJSON>): Promise<PasskeyReply> {
  return prompt(() =>
    navigator.credentials.get({ publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey) }),
  );
}

async function prompt(run: () => Promise<Credential | null>): Promise<PasskeyReply> {
  try {
    const cred = (await run()) as PublicKeyCredential | null;
    return cred ? cred.toJSON() : "cancelled";
  } catch (err) {
    if (err instanceof DOMException && err.name === "NotAllowedError") return "cancelled";
    if (err instanceof DOMException && err.name === "InvalidStateError") {
      throw new Error("This device already has a passkey for this account.");
    }
    throw err;
  }
}
