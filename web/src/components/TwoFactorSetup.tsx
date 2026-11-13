import { Plus } from "lucide-react";
import { FormEvent, useState } from "react";
import { Admin, ApiError, apiFetch, errorMessage, FactorAdded, isConfirmRequired, isUnauthenticated, TotpSetup, TwoFactorState } from "../api";
import { createPasskey, passkeysSupported } from "../webauthn";
import { useConfirm } from "./ConfirmIdentity";
import Modal from "./Modal";
import QrCode from "./QrCode";
import RecoveryCodes from "./RecoveryCodes";

/** MAX_PASSKEYS is the API's limit of passkeys per admin. */
const MAX_PASSKEYS = 10;

/** TOO_MANY is the message at MAX_PASSKEYS, for the 409 and the disabled button alike. */
const TOO_MANY = "You have 10 passkeys. Remove one before adding another.";

/**
 * TwoFactorSetup adds a second factor for the signed-in admin whose GET /api/2fa `state` it is
 * given. "Add authenticator app" starts POST /api/2fa/totp/setup and opens a dialog with the QR
 * code and the key in groups of 4; a code from the app is sent to POST /api/2fa/totp. A wrong
 * code shows the error and keeps the dialog open. When the API returns recovery codes (the first
 * factor) they are shown with `RecoveryCodes` before `onDone`; otherwise `onDone` is called at
 * once. `onDone` gets the admin the API returns when adding the factor finished an enroll
 * sign-in. A 403 confirm_required opens `ConfirmIdentity` and starts again once confirmed. Without
 * an encryption key on the server it says authenticator apps are not available, and once an
 * authenticator exists it offers no second one.
 *
 * "Add passkey" opens a dialog asking for the passkey's name; Add fetches
 * POST /api/2fa/passkeys/options, runs the device prompt and sends the name and credential to
 * POST /api/2fa/passkeys, with the same recovery-code, confirm and error handling. A cancelled
 * prompt keeps the dialog open. At 10 passkeys the button is disabled with a hint (FR-013).
 * Passkeys are offered only when the server has them, this page is at `passkeys_origin` (else
 * the note names the address to use, FR-016) and the browser supports them. A 401 calls
 * `onSignedOut`.
 */
export default function TwoFactorSetup({
  state,
  onDone,
  onSignedOut,
}: {
  state: TwoFactorState;
  onDone: (admin?: Admin) => void;
  onSignedOut: () => void;
}) {
  const [setup, setSetup] = useState<TotpSetup | null>(null);
  const [naming, setNaming] = useState(false);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [codes, setCodes] = useState<string[] | null>(null);
  const [signedIn, setSignedIn] = useState<Admin | undefined>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const identity = useConfirm(onSignedOut, setError, state);

  // fail handles an API error of flow: 401 signs out, confirm_required asks to confirm, else shows it.
  function fail(err: unknown, flow: "totp" | "passkey" = "totp") {
    if (isUnauthenticated(err)) onSignedOut();
    else if (isConfirmRequired(err)) {
      setSetup(null);
      setNaming(false);
      identity.ask(flow === "totp" ? start : () => setNaming(true));
    } else if (err instanceof ApiError && err.code === "too_many_passkeys") setError(TOO_MANY);
    else setError(errorMessage(err));
  }

  async function start() {
    setError("");
    setBusy(true);
    try {
      setCode("");
      setSetup(await apiFetch<TotpSetup>("POST", "/api/2fa/totp/setup"));
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  }

  async function verify(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const added = await apiFetch<FactorAdded>("POST", "/api/2fa/totp", { code });
      shown(added);
    } catch (err) {
      fail(err);
    } finally {
      setBusy(false);
    }
  }

  async function addPasskey(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const credential = await createPasskey(await apiFetch("POST", "/api/2fa/passkeys/options"));
      if (credential === "cancelled") {
        setError("Passkey setup was cancelled.");
        return;
      }
      shown(await apiFetch<FactorAdded>("POST", "/api/2fa/passkeys", { name, credential }));
    } catch (err) {
      fail(err, "passkey");
    } finally {
      setBusy(false);
    }
  }

  // shown ends an add: the recovery codes first when there are any, else straight to finish.
  function shown(added: FactorAdded) {
    if (!added.recovery_codes) return finish(added.admin);
    setSignedIn(added.admin);
    setCodes(added.recovery_codes);
  }

  function finish(admin?: Admin) {
    setSetup(null);
    setNaming(false);
    setName("");
    setCodes(null);
    onDone(admin);
  }

  function cancel() {
    setSetup(null);
    setNaming(false);
    setError("");
  }

  const passkeyNote = !state.passkeys_available
    ? "Passkeys are not available: this server has no public URL."
    : state.passkeys_origin !== location.origin
      ? `Passkeys are not available here. Open ${state.passkeys_origin} to use them.`
      : !passkeysSupported(state.passkeys_origin)
        ? "Passkeys are not available in this browser."
        : null;
  const full = state.passkeys.length >= MAX_PASSKEYS;
  const addTotp = !state.totp && state.totp_available;
  return (
    <>
      {!state.totp && !state.totp_available && (
        <p className="muted">Authenticator apps are not available: this server has no encryption key.</p>
      )}
      {passkeyNote && <p className="muted">{passkeyNote}</p>}
      {!passkeyNote && full && <p className="muted">{TOO_MANY}</p>}
      {(addTotp || !passkeyNote) && (
        <div className="btn-row">
          {addTotp && (
            <button type="button" className="btn btn-primary" onClick={start} disabled={busy}>
              <Plus aria-hidden="true" />
              Add authenticator app
            </button>
          )}
          {!passkeyNote && (
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => {
                setError("");
                setNaming(true);
              }}
              disabled={busy || full}
            >
              <Plus aria-hidden="true" />
              Add passkey
            </button>
          )}
        </div>
      )}
      {!setup && !naming && error && <p role="alert">{error}</p>}
      {identity.dialog}
      {naming && (
        // While the codes show, only their Done closes the dialog, so they are not lost by accident.
        <Modal title="Add passkey" onClose={codes ? () => {} : cancel}>
          {codes ? (
            <RecoveryCodes codes={codes} onDone={() => finish(signedIn)} />
          ) : (
            <form className="stack-form" onSubmit={addPasskey}>
              <p>Name this passkey so you can tell it apart later, then approve the prompt on your device.</p>
              <label>
                Passkey name
                <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} required />
              </label>
              {error && <p role="alert">{error}</p>}
              <div className="btn-row">
                <button type="submit" className="btn btn-primary" disabled={busy}>
                  Add
                </button>
                <button type="button" className="btn" onClick={cancel}>
                  Cancel
                </button>
              </div>
            </form>
          )}
        </Modal>
      )}
      {setup && (
        // While the codes show, only their Done closes the dialog, so they are not lost by accident.
        <Modal title="Add authenticator app" onClose={codes ? () => {} : cancel}>
          {codes ? (
            <RecoveryCodes codes={codes} onDone={() => finish(signedIn)} />
          ) : (
            <form className="stack-form" onSubmit={verify}>
              <p>Scan this QR code with your authenticator app, or type the key into it.</p>
              <QrCode value={setup.uri} label="QR code for your authenticator app" />
              <p>
                Key: <code>{setup.secret.match(/.{1,4}/g)?.join(" ")}</code>
              </p>
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
              {error && <p role="alert">{error}</p>}
              <div className="btn-row">
                <button type="submit" className="btn btn-primary" disabled={busy}>
                  Verify
                </button>
                <button type="button" className="btn" onClick={cancel}>
                  Cancel
                </button>
              </div>
            </form>
          )}
        </Modal>
      )}
    </>
  );
}
