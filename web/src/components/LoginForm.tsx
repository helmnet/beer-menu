import { FormEvent, useState } from "react";
import { Admin, apiFetch, errorMessage, isUnauthenticated } from "../api";

/**
 * LoginForm signs in with POST /api/session and calls `onSignedIn` with the admin. A 401 shows
 * one generic message whatever the cause, and a 429 says how many minutes to wait, from the
 * Retry-After header.
 */
export default function LoginForm({ onSignedIn }: { onSignedIn: (admin: Admin) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      onSignedIn(await apiFetch<Admin>("POST", "/api/session", { username, password }));
    } catch (err) {
      setError(loginError(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit}>
      <h2>Sign in</h2>
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
          autoComplete="current-password"
          required
        />
      </label>
      {error && <p role="alert">{error}</p>}
      <button type="submit" disabled={busy}>
        Sign in
      </button>
    </form>
  );
}

function loginError(err: unknown): string {
  return isUnauthenticated(err) ? "Wrong username or password." : errorMessage(err);
}
