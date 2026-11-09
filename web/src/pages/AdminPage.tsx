import { useEffect, useState } from "react";
import { Admin, apiFetch, errorMessage, isUnauthenticated } from "../api";
import AdminsPanel from "../components/AdminsPanel";
import LoginForm from "../components/LoginForm";
import PasswordForm from "../components/PasswordForm";
import SetupForm from "../components/SetupForm";
import TokensPanel from "../components/TokensPanel";

type View =
  | { kind: "loading" }
  | { kind: "setup" }
  | { kind: "login" }
  | { kind: "signedIn"; admin: Admin }
  | { kind: "error"; message: string };

/**
 * AdminPage is the admin app at `/` and every non-display path. On load it asks
 * GET /api/setup and GET /api/session, then shows the first-admin setup form, the sign-in
 * form or the signed-in view with the admins, tokens and change-password sections. Any 401
 * from a later call, including the panels' calls, returns to the sign-in form.
 */
export default function AdminPage() {
  const [view, setView] = useState<View>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    loadView().then((v) => {
      if (!cancelled) setView(v);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const signedIn = (admin: Admin) => setView({ kind: "signedIn", admin });
  const toLogin = () => setView({ kind: "login" });

  async function signOut() {
    try {
      await apiFetch("DELETE", "/api/session");
    } catch (err) {
      if (!isUnauthenticated(err)) {
        setView({ kind: "error", message: errorMessage(err) });
        return;
      }
    }
    toLogin();
  }

  return (
    <main>
      <h1>Beer Menu</h1>
      {view.kind === "loading" && <p>Loading…</p>}
      {view.kind === "error" && <p role="alert">{view.message}</p>}
      {view.kind === "setup" && <SetupForm onCreated={signedIn} onSetupDone={toLogin} />}
      {view.kind === "login" && <LoginForm onSignedIn={signedIn} />}
      {view.kind === "signedIn" && (
        <section>
          <p>Signed in as {view.admin.username}</p>
          <button type="button" onClick={signOut}>
            Sign out
          </button>
          <AdminsPanel me={view.admin} onSignedOut={toLogin} />
          <TokensPanel onSignedOut={toLogin} />
          <PasswordForm onSignedOut={toLogin} />
        </section>
      )}
    </main>
  );
}

/** loadView picks the first view from the setup state and the current session. */
async function loadView(): Promise<View> {
  try {
    const setup = await apiFetch<{ needed: boolean }>("GET", "/api/setup");
    if (setup.needed) return { kind: "setup" };
    return { kind: "signedIn", admin: await apiFetch<Admin>("GET", "/api/session") };
  } catch (err) {
    if (isUnauthenticated(err)) return { kind: "login" };
    return { kind: "error", message: errorMessage(err) };
  }
}
