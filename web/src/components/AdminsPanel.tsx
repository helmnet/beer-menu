import { Plus } from "lucide-react";
import { FormEvent, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { Admin, apiFetch, errorMessage, isConfirmRequired, isUnauthenticated } from "../api";
import { useConfirm } from "./ConfirmIdentity";
import Modal from "./Modal";

/**
 * AdminsPanel is the Admin Users page: the "Admin Users" heading with "Add admin", then a card
 * listing admins (GET /api/admins), each removable after confirmation (DELETE /api/admins/{id}).
 * Admins with two-factor sign-in show a "2FA" badge, and every other admin with it gets "Clear
 * 2FA", which asks first, then removes their factors and recovery codes and ends their sessions
 * (DELETE /api/admins/{id}/2fa) and reloads the list. A 403 confirm_required (on add, remove or
 * clear) loads the caller's factors (GET /api/2fa), opens `ConfirmIdentity` (over the Add admin
 * dialog when open) and runs the action again once confirmed.
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
  const identity = useConfirm(onSignedOut, setError);

  // run does an action; confirm_required asks to confirm (with the caller's factors), then runs it again.
  async function run(action: () => Promise<void>) {
    setError("");
    try {
      await action();
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else if (isConfirmRequired(err)) identity.ask(() => run(action));
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

  function add(e: FormEvent) {
    e.preventDefault();
    if (!busy) save({ username, password });
  }

  // save posts the new admin; on confirm_required the retry holds creds only until confirm or cancel.
  async function save(creds: { username: string; password: string }) {
    setFormError("");
    setBusy(true);
    try {
      await apiFetch("POST", "/api/admins", creds);
    } catch (err) {
      if (isUnauthenticated(err)) onSignedOut();
      else if (isConfirmRequired(err)) identity.ask(() => save(creds));
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
              <button type="button" className="btn btn-danger" onClick={() => remove(a)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
        {error && <p role="alert">{error}</p>}
      </div>
      {identity.dialog}
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
