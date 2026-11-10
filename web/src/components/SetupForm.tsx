import { FormEvent, useState } from "react";
import { Admin, ApiError, apiFetch, errorMessage } from "../api";

/**
 * SetupForm creates the first admin with POST /api/setup. It checks the 12-character minimum
 * and the confirmation before sending, shows the server's message on 400, calls `onCreated`
 * with the new (signed-in) admin on success, and `onSetupDone` when the API says an admin
 * already exists (409).
 */
export default function SetupForm({
  onCreated,
  onSetupDone,
}: {
  onCreated: (admin: Admin) => void;
  onSetupDone: () => void;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    // Code points, as the API counts them.
    const length = [...password].length;
    if (length < 12 || length > 256) {
      setError("Password must be 12 to 256 characters.");
      return;
    }
    if (password !== confirm) {
      setError("The passwords do not match.");
      return;
    }
    setError("");
    setBusy(true);
    try {
      onCreated(await apiFetch<Admin>("POST", "/api/setup", { username, password }));
    } catch (err) {
      if (err instanceof ApiError && err.code === "setup_done") {
        onSetupDone();
        return;
      }
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="stack-form" onSubmit={submit}>
      <h2>Create the first admin</h2>
      <label>
        Username
        <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required />
      </label>
      <label>
        Password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          required
        />
      </label>
      <label>
        Confirm password
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          autoComplete="new-password"
          required
        />
      </label>
      {error && <p role="alert">{error}</p>}
      <button type="submit" className="btn btn-primary" disabled={busy}>
        Create admin
      </button>
    </form>
  );
}
