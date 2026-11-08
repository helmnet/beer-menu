import { FormEvent, useState } from "react";
import { apiFetch, errorMessage, isUnauthenticated } from "../api";

/**
 * PasswordForm changes the signed-in admin's password with POST /api/password. It checks the
 * two new entries match before sending, shows the server's message for a wrong current
 * password, how long to wait when rate limited, and a success note (the API has ended the
 * admin's other sessions). Any 401 calls `onSignedOut`.
 */
export default function PasswordForm({ onSignedOut }: { onSignedOut: () => void }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setDone(false);
    if (next !== confirm) {
      setError("The new passwords do not match.");
      return;
    }
    setError("");
    setBusy(true);
    try {
      await apiFetch("POST", "/api/password", { current_password: current, new_password: next });
      setCurrent("");
      setNext("");
      setConfirm("");
      setDone(true);
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h2>Change password</h2>
      <label>
        Current password
        <input
          type="password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          autoComplete="current-password"
          required
        />
      </label>
      <label>
        New password
        <input
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          autoComplete="new-password"
          required
        />
      </label>
      <label>
        Confirm new password
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
          required
        />
      </label>
      {error && <p role="alert">{error}</p>}
      {done && <p>Password changed. Your other sessions have ended.</p>}
      <button type="submit" disabled={busy}>
        Change password
      </button>
    </form>
  );
}
