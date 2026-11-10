import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated, Token } from "../api";
import Modal from "./Modal";

/**
 * TokensPanel is the API Tokens page, one card: it lists API tokens (GET /api/tokens) and
 * revokes them after confirmation (DELETE /api/tokens/{id}). "New token" opens the "New API
 * token" dialog with Name and Scope; a server error keeps the form open with its message. On
 * success (POST /api/tokens) the dialog swaps the form for the secret, shown once with Copy
 * and Close, and the list reloads. Closing the dialog, by Close or Escape, drops the secret from
 * state, so it cannot be shown again. Any 401 calls `onSignedOut`.
 */
export default function TokensPanel({ onSignedOut }: { onSignedOut: () => void }) {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<Token["scope"]>("read");
  const [secret, setSecret] = useState("");
  const [copied, setCopied] = useState("");
  const [formError, setFormError] = useState("");
  const [error, setError] = useState("");

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

  async function create(e: FormEvent) {
    e.preventDefault();
    setFormError("");
    try {
      const created = await apiFetch<Token & { secret: string }>("POST", "/api/tokens", { name, scope });
      setSecret(created.secret);
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
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

  return (
    <section className="card">
      <div className="page-header">
        <h2>API Tokens</h2>
        <button type="button" className="btn btn-primary" onClick={openDialog}>
          <Plus aria-hidden="true" />
          New token
        </button>
      </div>
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
      {error && <p role="alert">{error}</p>}
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
