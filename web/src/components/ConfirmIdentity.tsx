import { FormEvent, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated } from "../api";
import { getPasskey, passkeysSupported } from "../webauthn";
import Modal from "./Modal";

/** Proof is the second factor a confirm sends. */
type Proof = "code" | "passkey" | "recovery";

/**
 * ConfirmIdentity is the "Confirm it is you" dialog shown before a sensitive 2FA change. It
 * posts the password to POST /api/2fa/confirm and, when `hasFactor`, exactly one proof: an
 * authenticator code (when `hasTotp`, the default), a passkey (when `passkeyOrigin` is set and
 * passkeys work on this page; the device prompt runs on Confirm, after
 * POST /api/2fa/confirm/options), or a recovery code (which is then spent). Buttons switch
 * between the proofs on offer. A 204 calls `onConfirmed`; the API's message shows for a wrong
 * password, code or passkey, a cancelled prompt keeps the dialog open, and a 429 says how long to
 * wait. Cancel or Escape calls `onCancel`; a 401 calls `onSignedOut`.
 */
export default function ConfirmIdentity({
  hasFactor,
  hasTotp = true,
  passkeyOrigin = null,
  onConfirmed,
  onCancel,
  onSignedOut,
}: {
  hasFactor: boolean;
  hasTotp?: boolean;
  passkeyOrigin?: string | null;
  onConfirmed: () => void;
  onCancel: () => void;
  onSignedOut: () => void;
}) {
  const offered: Proof[] = [
    ...(hasTotp ? ["code" as const] : []),
    ...(passkeysSupported(passkeyOrigin) ? ["passkey" as const] : []),
    "recovery",
  ];
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [proof, setProof] = useState<Proof>(offered[0]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      let sent: object = {};
      if (hasFactor && proof === "passkey") {
        const passkey = await getPasskey(await apiFetch("POST", "/api/2fa/confirm/options"));
        if (passkey === "cancelled") {
          setBusy(false);
          setError("Passkey check was cancelled.");
          return;
        }
        sent = { passkey };
      } else if (hasFactor) sent = proof === "recovery" ? { recovery_code: code } : { code };
      await apiFetch("POST", "/api/2fa/confirm", { password, ...sent });
    } catch (err) {
      setBusy(false);
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
      return;
    }
    onConfirmed();
  }

  function switchTo(p: Proof) {
    setProof(p);
    setCode("");
    setError("");
  }

  const switchLabel: Record<Proof, string> = {
    code: "Use an authenticator code",
    passkey: "Use a passkey",
    recovery: "Use a recovery code",
  };

  return (
    <Modal title="Confirm it is you" onClose={onCancel}>
      <form className="stack-form" onSubmit={submit}>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {hasFactor && proof === "code" && (
          <label>
            Authenticator code
            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              autoComplete="one-time-code"
              inputMode="numeric"
              maxLength={32}
              required
            />
          </label>
        )}
        {hasFactor && proof === "passkey" && <p>Your device will ask for your fingerprint, face or PIN.</p>}
        {hasFactor && proof === "recovery" && (
          <label>
            Recovery code
            <input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="off" maxLength={32} required />
          </label>
        )}
        {hasFactor &&
          offered
            .filter((p) => p !== proof)
            .map((p) => (
              <button key={p} type="button" className="btn" onClick={() => switchTo(p)}>
                {switchLabel[p]}
              </button>
            ))}
        {error && <p role="alert">{error}</p>}
        <div className="btn-row">
          <button type="submit" className="btn btn-primary" disabled={busy}>
            Confirm
          </button>
          <button type="button" className="btn" onClick={onCancel}>
            Cancel
          </button>
        </div>
      </form>
    </Modal>
  );
}
