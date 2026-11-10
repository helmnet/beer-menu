import { useEffect, useState } from "react";
import { Admin, apiFetch, errorMessage, isUnauthenticated, Units } from "../api";
import AdminsPanel from "../components/AdminsPanel";
import BeersPanel from "../components/BeersPanel";
import BrewfatherPanel from "../components/BrewfatherPanel";
import KegHistory from "../components/KegHistory";
import LoginForm from "../components/LoginForm";
import PasswordForm from "../components/PasswordForm";
import SettingsPanel from "../components/SettingsPanel";
import SetupForm from "../components/SetupForm";
import TapsPanel from "../components/TapsPanel";
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
 * form or the signed-in view with the taps, beer library, Brewfather, display settings, keg
 * history, admins, tokens and change-password sections. A keg change in the taps panel that can move a beer on or off a tap or
 * into history (not a pour or level set) reloads the keg history and the beer library's artwork.
 * A Brewfather import or finished sync reloads the beer library. Volumes in the taps panel and
 * keg history follow the units setting SettingsPanel loads and saves, US until it loads. Any 401 from a later call, including the panels' calls, returns to the
 * sign-in form.
 */
export default function AdminPage() {
  const [view, setView] = useState<View>({ kind: "loading" });
  // Bumped when TapsPanel reports a keg change: KegHistory remounts and BeersPanel re-fetches its list.
  const [kegChanges, setKegChanges] = useState(0);
  // Bumped when BrewfatherPanel imports or syncs beers: BeersPanel re-fetches its list.
  const [beerChanges, setBeerChanges] = useState(0);
  const [units, setUnits] = useState<Units>("us");

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
          <TapsPanel units={units} onSignedOut={toLogin} onKegsChanged={() => setKegChanges((n) => n + 1)} />
          {/* Both counters only grow, so their sum changes whenever either does. */}
          <BeersPanel onSignedOut={toLogin} refresh={kegChanges + beerChanges} />
          <BrewfatherPanel onSignedOut={toLogin} onBeersChanged={() => setBeerChanges((n) => n + 1)} />
          <SettingsPanel onSignedOut={toLogin} onChange={(s) => setUnits(s.units)} />
          <KegHistory key={kegChanges} units={units} onSignedOut={toLogin} />
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
