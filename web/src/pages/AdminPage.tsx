import { Beer } from "lucide-react";
import { useEffect, useState } from "react";
import { Admin, apiFetch, errorMessage, isUnauthenticated, Settings, Units } from "../api";
import AdminsPanel from "../components/AdminsPanel";
import BeersPanel from "../components/BeersPanel";
import BrewfatherPanel from "../components/BrewfatherPanel";
import KegHistory from "../components/KegHistory";
import LoginForm from "../components/LoginForm";
import OnDeckPanel from "../components/OnDeckPanel";
import PasswordForm from "../components/PasswordForm";
import SettingsPanel from "../components/SettingsPanel";
import SetupForm from "../components/SetupForm";
import SideNav, { GroupKey, groupOf, isPageKey, PageKey } from "../components/SideNav";
import TapsPanel from "../components/TapsPanel";
import ThemeControl from "../components/ThemeControl";
import TokensPanel from "../components/TokensPanel";
import { loadTheme, saveTheme, Theme } from "../prefs";
import "./AdminPage.css";

type View =
  | { kind: "loading" }
  | { kind: "setup" }
  | { kind: "login" }
  | { kind: "signedIn"; admin: Admin }
  | { kind: "error"; message: string };

/**
 * AdminPage is the admin app at `/` and every non-display path, rendered inside one
 * `div.admin.theme-<light|dark|system>` root that scopes all admin styles. The theme is this
 * browser's saved choice (prefs; default system, which follows the device through CSS),
 * applied on every view, sign-in and setup included; ThemeControl at the bottom of the side
 * panel changes and saves it, with no server request. On load it asks GET /api/setup and
 * GET /api/session, then shows the first-admin setup form, the sign-in form, a loading note or
 * an error, each in a centered card with no side panel. Signed in, it is a shell: SideNav (side
 * panel, or top bar and drawer on a phone) and a `main` that mounts only the current page. The
 * current page is kept in the URL hash (`#beers`, `#on-deck`): choosing a page in SideNav sets
 * it, adding a browser history entry, so Back and Forward move between pages, and load, reload
 * and every sign-in show the hash's page, or Taps when the hash names no page; Sign out clears
 * the hash, so only a sign-in after a 401 returns to the same page. A hash, not a path, since
 * the `display` page would clash with the `/display` TV route. The page's primary
 * action (Add tap, Add keg, Add beer, Add admin or New token) is also portaled into the top
 * bar's slot. Since a page mounts when shown, it
 * fetches fresh data each time and pages need not refresh each other.
 *
 * After each sign-in it reads GET /api/health once for the version (a failure or empty
 * version shows "unknown", with no error) and GET /api/settings once for the volume units,
 * which Display Settings' saves update too; units stay US until then or if that read fails.
 * Any 401, from those reads or from a page, returns to the sign-in form.
 */
export default function AdminPage() {
  const [view, setView] = useState<View>({ kind: "loading" });
  const [theme, setTheme] = useState<Theme>(loadTheme);
  const [page, setPage] = useState<PageKey>(hashPage);
  const [openGroup, setOpenGroup] = useState<GroupKey | null>(() => groupOf(hashPage()));
  const [units, setUnits] = useState<Units>("us");
  // null until the health check answers, so "unknown" never flashes before the real version.
  const [version, setVersion] = useState<string | null>(null);
  // The phone top bar's element for the page's primary action, set once SideNav mounts.
  const [actionSlot, setActionSlot] = useState<HTMLDivElement | null>(null);

  useEffect(() => {
    let cancelled = false;
    loadView().then((v) => {
      if (!cancelled) setView(v);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  // Back, Forward and hand-edited hashes all arrive here.
  useEffect(() => {
    const onHash = () => show(hashPage());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const isSignedIn = view.kind === "signedIn";
  useEffect(() => {
    if (!isSignedIn) return;
    let cancelled = false;
    apiFetch<{ version?: string }>("GET", "/api/health")
      .then((h) => {
        if (!cancelled) setVersion(h.version ?? "");
      })
      .catch(() => {
        if (!cancelled) setVersion("");
      });
    apiFetch<Settings>("GET", "/api/settings")
      .then((s) => {
        if (!cancelled) setUnits(s.units);
      })
      .catch((err) => {
        if (!cancelled && isUnauthenticated(err)) toLogin();
      });
    return () => {
      cancelled = true;
    };
  }, [isSignedIn]);

  function signedIn(admin: Admin) {
    show(hashPage());
    setView({ kind: "signedIn", admin });
  }
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
    // Replace, not push, and no hashchange: the next sign-in starts on Taps. The absolute href,
    // since a "//host/..." path read as a relative URL names another origin and throws.
    window.history.replaceState(null, "", window.location.href.split("#")[0]);
    toLogin();
  }

  function changeTheme(t: Theme) {
    setTheme(t);
    saveTheme(t);
  }

  function show(p: PageKey) {
    setPage(p);
    setOpenGroup(groupOf(p));
  }

  function select(p: PageKey) {
    // Setting the hash pushes a history entry; re-choosing the current page adds none.
    if (p !== page) window.location.hash = p;
    show(p);
  }

  if (view.kind !== "signedIn") {
    return (
      <div className={`admin theme-${theme}`}>
        <main className="auth">
          <div className="card auth-card">
            <h1 className="brand">
              <Beer aria-hidden="true" />
              Beer Menu
            </h1>
            {view.kind === "loading" && <p className="muted">Loading…</p>}
            {view.kind === "error" && <p role="alert">{view.message}</p>}
            {view.kind === "setup" && <SetupForm onCreated={signedIn} onSetupDone={toLogin} />}
            {view.kind === "login" && <LoginForm onSignedIn={signedIn} />}
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className={`admin theme-${theme}`}>
      <div className="shell">
        <SideNav
          page={page}
          openGroup={openGroup}
          onGroup={(g) => setOpenGroup(openGroup === g ? null : g)}
          onSelect={select}
          version={version}
          onSignOut={signOut}
          theme={<ThemeControl theme={theme} onChange={changeTheme} />}
          actionSlot={setActionSlot}
        />
        <main className="content">
          {pageView(page, view.admin, units, actionSlot, toLogin, (s) => setUnits(s.units))}
        </main>
      </div>
    </div>
  );
}

/** pageView is the one page component shown for `page`. */
function pageView(
  page: PageKey,
  me: Admin,
  units: Units,
  actionSlot: HTMLElement | null,
  toLogin: () => void,
  onSettings: (s: Settings) => void,
) {
  switch (page) {
    case "taps":
      return <TapsPanel units={units} actionSlot={actionSlot} onSignedOut={toLogin} />;
    case "on-deck":
      return <OnDeckPanel units={units} actionSlot={actionSlot} onSignedOut={toLogin} />;
    case "keg-history":
      return <KegHistory units={units} onSignedOut={toLogin} />;
    case "beers":
      return <BeersPanel actionSlot={actionSlot} onSignedOut={toLogin} />;
    case "display":
      return <SettingsPanel onSignedOut={toLogin} onChange={onSettings} />;
    case "admins":
      return <AdminsPanel me={me} actionSlot={actionSlot} onSignedOut={toLogin} />;
    case "tokens":
      return <TokensPanel actionSlot={actionSlot} onSignedOut={toLogin} />;
    case "password":
      return <PasswordForm onSignedOut={toLogin} />;
    case "brewfather":
      return <BrewfatherPanel onSignedOut={toLogin} />;
  }
}

/** hashPage is the page named by the URL hash, or Taps when it names none. */
function hashPage(): PageKey {
  const key = window.location.hash.slice(1);
  return isPageKey(key) ? key : "taps";
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
