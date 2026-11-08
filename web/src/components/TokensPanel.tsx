import { FormEvent, useEffect, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated, Token } from "../api";

/**
 * TokensPanel lists API tokens (GET /api/tokens), creates them (POST /api/tokens) and revokes
 * them after confirmation (DELETE /api/tokens/{id}). A new token's secret is shown once in a
 * dialog with a copy button and is dropped from state when the dialog closes, so it cannot be
 * shown again. Any 401 calls `onSignedOut`.
 */
export default function TokensPanel({ onSignedOut }: { onSignedOut: () => void }) {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<Token["scope"]>("read");
  const [secret, setSecret] = useState("");
  const [copied, setCopied] = useState("");
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

  function create(e: FormEvent) {
    e.preventDefault();
    run(async () => {
      const created = await apiFetch<Token & { secret: string }>("POST", "/api/tokens", { name, scope });
      setSecret(created.secret);
      setCopied("");
      setName("");
      await load();
    });
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
    <section>
      <h2>API tokens</h2>
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
                  <button type="button" onClick={() => revoke(t)}>
                    Revoke
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <form onSubmit={create}>
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
        <button type="submit">Create token</button>
      </form>
      {error && <p role="alert">{error}</p>}
      {secret && (
        <div role="dialog" aria-label="New API token">
          <p>Copy this token now. It will not be shown again.</p>
          <code>{secret}</code>
          <button type="button" onClick={copy}>
            Copy
          </button>
          {copied && <p>{copied}</p>}
          <button type="button" onClick={() => setSecret("")}>
            Close
          </button>
        </div>
      )}
    </section>
  );
}
