import { FormEvent, useEffect, useState } from "react";
import { Admin, apiFetch, errorMessage, isUnauthenticated } from "../api";

/**
 * AdminsPanel is the Admin Users page, one card: it lists admins (GET /api/admins), adds one with
 * a username and initial password (POST /api/admins) and removes one after confirmation (DELETE
 * /api/admins/{id}), showing the server's message for a taken username or the last admin. Removing
 * `me` ends this session, so it then calls `onSignedOut`, as it does for any 401.
 */
export default function AdminsPanel({ me, onSignedOut }: { me: Admin; onSignedOut: () => void }) {
  const [admins, setAdmins] = useState<Admin[]>([]);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
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

  const load = async () => setAdmins(await apiFetch<Admin[]>("GET", "/api/admins"));

  useEffect(() => {
    run(load);
  }, []);

  function add(e: FormEvent) {
    e.preventDefault();
    run(async () => {
      await apiFetch("POST", "/api/admins", { username, password });
      setUsername("");
      setPassword("");
      await load();
    });
  }

  function remove(a: Admin) {
    const self = a.id === me.id;
    if (!window.confirm(self ? "Remove yourself? You will be signed out." : `Remove admin "${a.username}"?`)) return;
    run(async () => {
      await apiFetch("DELETE", `/api/admins/${a.id}`);
      if (self) onSignedOut();
      else await load();
    });
  }

  return (
    <section className="card">
      <h2>Admin Users</h2>
      <ul className="plain-list">
        {admins.map((a) => (
          <li key={a.id}>
            <span>{a.username}</span> (added {new Date(a.created_at).toLocaleString()}){" "}
            <button type="button" className="btn" onClick={() => remove(a)}>
              Remove
            </button>
          </li>
        ))}
      </ul>
      <form className="stack-form" onSubmit={add}>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" required />
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
        <button type="submit" className="btn btn-primary">
          Add admin
        </button>
      </form>
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
