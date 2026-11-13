import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { apiFetch, errorMessage, isConfirmRequired, isUnauthenticated, Token } from "../api";
import { useConfirm } from "./ConfirmIdentity";
import Modal from "./Modal";

/**
 * TokensPanel is the API Tokens page, the heading with "New token" above one card: it lists API tokens (GET /api/tokens) and
 * revokes them after confirmation (DELETE /api/tokens/{id}). "New token" opens the "New API
 * token" dialog with Name and Scope; a server error keeps the form open with its message. On
 * success (POST /api/tokens) the dialog swaps the form for the secret, shown once with Copy
 * and Close, and the list reloads. A 403 confirm_required on create loads the caller's factors
 * (GET /api/2fa), opens `ConfirmIdentity` over the dialog and creates again once confirmed. Closing the dialog, by Close or Escape, drops the secret from
 * state, so it cannot be shown again. Any 401 calls `onSignedOut`. Given `actionSlot` (the phone
 * top bar's slot), a second New token is portaled there; CSS shows one of the two. Under the
 * list, an "API docs" link opens the read-only API docs (/api/docs) in a new tab.
 */
export default function TokensPanel({ onSignedOut, actionSlot }: { onSignedOut: () => void; actionSlot?: HTMLElement | null }) {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<Token["scope"]>("read");
  const [secret, setSecret] = useState("");
  const [copied, setCopied] = useState("");
  const [formError, setFormError] = useState("");
  const [error, setError] = useState("");
  const identity = useConfirm(onSignedOut, setError);

  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    }
  }

  const load = async () => setTokens(await apiFetch<Token[]>("GET", "/api/tokens"));

  useEffect(() => {
    run(load);
  }, []);

  function openDialog() {
    // A create answered after its dialog closed leaves a secret behind; never show it later.
    setSecret("");
    setCopied("");
    setName("");
    setScope("read");
    setFormError("");
    setCreating(true);
  }

  function closeDialog() {
    setCreating(false);
    setSecret("");
    setCopied("");
  }

  function create(e: FormEvent) {
    e.preventDefault();
    save({ name, scope });
  }

  // save posts the new token; on confirm_required it confirms, then posts the same request again.
  async function save(req: { name: string; scope: Token["scope"] }) {
    setFormError("");
    try {
      const created = await apiFetch<Token & { secret: string }>("POST", "/api/tokens", req);
      setSecret(created.secret);
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else if (isConfirmRequired(err)) identity.ask(() => save(req));
      else setFormError(errorMessage(err));
      return;
    }
    run(load);
  }

  function revoke(t: Token) {
    if (!window.confirm(`Revoke token "${t.name}"? Clients using it stop working at once.`)) return;
    run(async () => {
      await apiFetch("DELETE", `/api/tokens/${t.id}`);
      await load();
    });
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied("Copied.");
    } catch {
      // No clipboard on plain HTTP: the secret stays on screen to copy by hand.
      setCopied("Copy failed. Select the token and copy it by hand.");
    }
  }

  const newToken = (
    <button type="button" className="btn btn-primary page-action" onClick={openDialog}>
      <Plus aria-hidden="true" />
      New token
    </button>
  );

  return (
    <section className="page">
      <div className="page-header">
        <h2>API Tokens</h2>
        {newToken}
      </div>
      {actionSlot && createPortal(newToken, actionSlot)}
      <div className="card">
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Scope</th>
                <th>Created</th>
                <th>Last used</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.id}>
                  <td>{t.name}</td>
                  <td>{t.scope}</td>
                  <td>{new Date(t.created_at).toLocaleString()}</td>
                  <td>{t.last_used_at ? new Date(t.last_used_at).toLocaleString() : "never"}</td>
                  <td>
                    {t.revoked_at ? (
                      "Revoked"
                    ) : (
                      <button type="button" className="btn" onClick={() => revoke(t)}>
                        Revoke
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="muted">
          Calling the API? See the{" "}
          <a href="/api/docs" target="_blank" rel="noopener noreferrer">
            API docs
          </a>
          .
        </p>
        {error && <p role="alert">{error}</p>}
      </div>
      {identity.dialog}
      {creating && (
        <Modal title="New API token" onClose={closeDialog}>
          {secret ? (
            <div className="secret-box">
              <p>Copy this token now. It will not be shown again.</p>
              <code>{secret}</code>
              {copied && <p>{copied}</p>}
              <div className="btn-row">
                <button type="button" className="btn" onClick={copy}>
                  Copy
                </button>
                <button type="button" className="btn btn-primary" onClick={closeDialog}>
                  Close
                </button>
              </div>
            </div>
          ) : (
            <form className="inline-form" onSubmit={create}>
              <label>
                Name
                <input value={name} onChange={(e) => setName(e.target.value)} required />
              </label>
              <label>
                Scope
                <select value={scope} onChange={(e) => setScope(e.target.value as Token["scope"])}>
                  <option value="read">read</option>
                  <option value="write">write</option>
                </select>
              </label>
              {formError && <p role="alert">{formError}</p>}
              <div className="btn-row">
                <button type="submit" className="btn btn-primary">
                  Create token
                </button>
                <button type="button" className="btn" onClick={closeDialog}>
                  Cancel
                </button>
              </div>
            </form>
          )}
        </Modal>
      )}
    </section>
  );
}
