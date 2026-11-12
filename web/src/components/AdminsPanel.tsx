import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Admin, apiFetch, errorMessage, isUnauthenticated } from "../api";
import Modal from "./Modal";

/**
 * AdminsPanel is the Admin Users page: the "Admin Users" heading with "Add admin", then a card
 * listing admins (GET /api/admins), each removable after confirmation (DELETE /api/admins/{id}).
 * Admins with two-factor sign-in show a "2FA" badge, and every other admin with it gets "Clear
 * 2FA", which asks first, then removes their factors and recovery codes and ends their sessions
 * (DELETE /api/admins/{id}/2fa) and reloads the list.
 * "Add admin" opens the "Add admin" dialog with a username and initial password; Save sends POST
 * /api/admins, then closes it, clears the fields and reloads the list, while a server error, such
 * as a taken username, keeps it open with the message. Removing the last admin shows the server's
 * message. Removing `me` ends this session, so it then calls `onSignedOut`, as it does for any
 * 401. Given `actionSlot` (the phone top bar's slot), a second Add admin is portaled there; CSS
 * shows one of the two.
 */
export default function AdminsPanel({
  me,
  onSignedOut,
  actionSlot,
}: {
  me: Admin;
  onSignedOut: () => void;
  actionSlot?: HTMLElement | null;
}) {
  const [admins, setAdmins] = useState<Admin[]>([]);
  const [adding, setAdding] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState("");
  const [busy, setBusy] = useState(false);
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

  function close() {
    setAdding(false);
    setUsername("");
    setPassword("");
    setFormError("");
  }

  async function add(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setFormError("");
    setBusy(true);
    try {
      await apiFetch("POST", "/api/admins", { username, password });
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else setFormError(errorMessage(err));
      return;
    } finally {
      setBusy(false);
    }
    close();
    run(load);
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

  function clearTwoFactor(a: Admin) {
    if (!window.confirm(`Clear two-factor sign-in for "${a.username}"? They will be signed out everywhere.`)) return;
    run(async () => {
      await apiFetch("DELETE", `/api/admins/${a.id}/2fa`);
      await load();
    });
  }

  const addAdmin = (
    <button type="button" className="btn btn-primary page-action" onClick={() => setAdding(true)}>
      <Plus aria-hidden="true" />
      Add admin
    </button>
  );

  return (
    <section className="page">
      <div className="page-header">
        <h2>Admin Users</h2>
        {addAdmin}
      </div>
      {actionSlot && createPortal(addAdmin, actionSlot)}
      <div className="card">
        <ul className="plain-list">
          {admins.map((a) => (
            <li key={a.id}>
              <span>{a.username}</span> (added {new Date(a.created_at).toLocaleString()})
              {a.two_factor && <span className="badge">2FA</span>}{" "}
              {a.two_factor && a.id !== me.id && (
                <button type="button" className="btn" onClick={() => clearTwoFactor(a)}>
                  Clear 2FA
                </button>
              )}
              <button type="button" className="btn" onClick={() => remove(a)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
        {error && <p role="alert">{error}</p>}
      </div>
      {adding && (
        <Modal title="Add admin" onClose={close}>
          <form className="inline-form" onSubmit={add}>
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
            {formError && <p role="alert">{formError}</p>}
            <div className="btn-row">
              <button type="submit" className="btn btn-primary" disabled={busy}>
                Save
              </button>
              <button type="button" className="btn" onClick={close}>
                Cancel
              </button>
            </div>
          </form>
        </Modal>
      )}
    </section>
  );
}
