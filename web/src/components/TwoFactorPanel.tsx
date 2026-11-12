import { FormEvent, useEffect, useState } from "react";
import { ApiError, apiFetch, errorMessage, isUnauthenticated, Passkey, RecoveryCodesReply, TwoFactorState } from "../api";
import ConfirmIdentity from "./ConfirmIdentity";
import Modal from "./Modal";
import RecoveryCodes from "./RecoveryCodes";
import TwoFactorSetup from "./TwoFactorSetup";

/** REQUIRE_LABEL names the require setting, on its switch and in the refusal it causes. */
const REQUIRE_LABEL = "Require two-factor sign-in for all admins";

/** when describes a factor's added and last-used times. */
function when(created: string, lastUsed: string | null): string {
  const used = lastUsed ? `last used ${new Date(lastUsed).toLocaleString()}` : "never used";
  return `added ${new Date(created).toLocaleString()}, ${used}`;
}

/**
 * TwoFactorPanel is the Two-Factor Sign-In page: the heading above one card showing the
 * signed-in admin's state from GET /api/2fa: whether two-factor sign-in is on, a switch for
 * whether it is required for every admin (PUT /api/2fa/required; disabled with a hint until the
 * admin has a factor of their own), and, when on, how many recovery codes are left (with a warning at 2
 * or fewer) and a list of the second factors, each with its type, name, added and last-used
 * time, never a secret. A passkey can be renamed inline (PATCH /api/2fa/passkeys/{id}). Each
 * factor can be removed (DELETE /api/2fa/totp or /api/2fa/passkeys/{id}) and 2FA turned off
 * (DELETE /api/2fa), each after the browser asks to be sure. "New recovery codes"
 * (POST /api/2fa/recovery-codes) shows the new set once with `RecoveryCodes`. A removal refused
 * with 409 two_factor_required says the require setting is on, naming it. A 403
 * confirm_required on any of these opens `ConfirmIdentity` and runs the action again once
 * confirmed. `TwoFactorSetup` adds a factor. The state reloads after every change. Errors show
 * the API's message; a 401 calls `onSignedOut`.
 */
export default function TwoFactorPanel({ onSignedOut }: { onSignedOut: () => void }) {
  const [state, setState] = useState<TwoFactorState | null>(null);
  const [error, setError] = useState("");
  const [loads, setLoads] = useState(0);
  const [retry, setRetry] = useState<(() => Promise<void>) | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [renaming, setRenaming] = useState<Passkey | null>(null);
  const [name, setName] = useState("");
  const [renameError, setRenameError] = useState("");

  useEffect(() => {
    let cancelled = false;
    apiFetch<TwoFactorState>("GET", "/api/2fa")
      .then((s) => {
        if (!cancelled) setState(s);
      })
      .catch((err) => {
        if (cancelled) return;
        if (isUnauthenticated(err)) onSignedOut();
        else setError(errorMessage(err));
      });
    return () => {
      cancelled = true;
    };
  }, [loads]);

  const reload = () => setLoads((n) => n + 1);

  // run does a sensitive action; confirm_required asks to confirm, then run runs it again.
  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else if (err instanceof ApiError && err.code === "confirm_required") setRetry(() => action);
      else if (err instanceof ApiError && err.code === "two_factor_required")
        setError(`"${REQUIRE_LABEL}" is on, so you must keep at least one second factor. Add another first, or turn that setting off.`);
      else setError(errorMessage(err));
    }
  }

  function remove(label: string, path: string) {
    if (!window.confirm(`Remove ${label}? Your other sessions will be signed out.`)) return;
    run(async () => {
      await apiFetch("DELETE", path);
      reload();
    });
  }

  function turnOff() {
    if (!window.confirm("Turn off two-factor sign-in? This removes every factor and recovery code.")) return;
    run(async () => {
      await apiFetch("DELETE", "/api/2fa");
      reload();
    });
  }

  function setRequired(required: boolean) {
    run(async () => {
      await apiFetch("PUT", "/api/2fa/required", { required });
      reload();
    });
  }

  function newCodes() {
    run(async () => setCodes((await apiFetch<RecoveryCodesReply>("POST", "/api/2fa/recovery-codes")).recovery_codes));
  }

  function startRename(p: Passkey) {
    setRenaming(p);
    setName(p.name);
    setRenameError("");
  }

  async function rename(e: FormEvent) {
    e.preventDefault();
    if (!renaming) return;
    setRenameError("");
    try {
      await apiFetch("PATCH", `/api/2fa/passkeys/${renaming.id}`, { name });
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setRenameError(errorMessage(err));
      return;
    }
    setRenaming(null);
    reload();
  }

  return (
    <section className="page">
      <h2 className="page-title">Two-Factor Sign-In</h2>
      <div className="card">
        {state && (
          <>
            <p>Two-factor sign-in is {state.enabled ? "on" : "off"}.</p>
            <label>
              <input
                type="checkbox"
                checked={state.required}
                disabled={!state.enabled}
                onChange={(e) => setRequired(e.target.checked)}
              />
              {REQUIRE_LABEL}
            </label>
            {!state.enabled && <p>Add a second factor for yourself before requiring one for every admin.</p>}
            {state.enabled && (
              <>
                <p>Recovery codes left: {state.recovery_codes_left} of 10.</p>
                {state.recovery_codes_left <= 2 && (
                  <p>
                    <strong>
                      {state.recovery_codes_left === 0
                        ? "No recovery codes left."
                        : `Only ${state.recovery_codes_left} recovery code${state.recovery_codes_left === 1 ? "" : "s"} left.`}{" "}
                      Make new recovery codes soon.
                    </strong>
                  </p>
                )}
                <ul className="plain-list" aria-label="Second factors">
                  {state.totp && (
                    <li>
                      <span>Authenticator app: {when(state.totp.created_at, state.totp.last_used_at)}</span>
                      <button type="button" className="btn" onClick={() => remove("the authenticator app", "/api/2fa/totp")}>
                        Remove
                      </button>
                    </li>
                  )}
                  {state.passkeys.map((p) =>
                    renaming?.id === p.id ? (
                      <li key={p.id}>
                        <form className="inline-form" onSubmit={rename}>
                          <label>
                            Passkey name
                            <input value={name} onChange={(e) => setName(e.target.value)} maxLength={64} required />
                          </label>
                          {renameError && <p role="alert">{renameError}</p>}
                          <div className="btn-row">
                            <button type="submit" className="btn btn-primary">
                              Save
                            </button>
                            <button type="button" className="btn" onClick={() => setRenaming(null)}>
                              Cancel
                            </button>
                          </div>
                        </form>
                      </li>
                    ) : (
                      <li key={p.id}>
                        <span>
                          Passkey "{p.name}": {when(p.created_at, p.last_used_at)}
                        </span>
                        <span className="btn-row">
                          <button type="button" className="btn" onClick={() => startRename(p)}>
                            Rename
                          </button>
                          <button
                            type="button"
                            className="btn"
                            onClick={() => remove(`the passkey "${p.name}"`, `/api/2fa/passkeys/${p.id}`)}
                          >
                            Remove
                          </button>
                        </span>
                      </li>
                    ),
                  )}
                </ul>
                <div className="btn-row">
                  <button type="button" className="btn" onClick={newCodes}>
                    New recovery codes
                  </button>
                  <button type="button" className="btn" onClick={turnOff}>
                    Turn off 2FA
                  </button>
                </div>
              </>
            )}
            <TwoFactorSetup state={state} onDone={reload} onSignedOut={onSignedOut} />
            {retry && (
              <ConfirmIdentity
                hasFactor={state.enabled}
                hasTotp={!!state.totp}
                passkeyOrigin={state.passkeys.length > 0 && state.passkeys_available ? state.passkeys_origin : null}
                onConfirmed={() => {
                  setRetry(null);
                  run(retry);
                }}
                onCancel={() => setRetry(null)}
                onSignedOut={onSignedOut}
              />
            )}
            {codes && (
              // Only Done closes the dialog, so the codes are not lost by accident.
              <Modal title="New recovery codes" onClose={() => {}}>
                <RecoveryCodes
                  codes={codes}
                  onDone={() => {
                    setCodes(null);
                    reload();
                  }}
                />
              </Modal>
            )}
          </>
        )}
        {error && <p role="alert">{error}</p>}
      </div>
    </section>
  );
}
