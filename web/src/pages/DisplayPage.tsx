import { useEffect, useState } from "react";
import { apiFetch, type Menu } from "../api";
import TvView from "../components/TvView";
import { displayOf, isMenu, pagesOf, usePager } from "../tv";

// 10 s keeps a change on screen within 30 s with room for a slow check (FR-020).
const POLL_MS = 10_000;
// Abort before the next check is due, so a hung request never stalls the chain.
const TIMEOUT_MS = 8_000;
// 12 checks is at least 2 minutes without a good one (FR-022), counted in ticks, not the TV clock.
const OFFLINE_AFTER = 12;
// 8,640 ticks of 10 s is about a day (FR-024); ticks, because the TV clock drifts and jumps.
const RELOAD_AFTER = 8_640;

/**
 * DisplayPage is the full-screen TV tap list at `/display`. It needs no credential: it reads
 * the public GET /api/menu right away and then 10 s after each check settles (never two at
 * once), aborting a check after 8 s. Each good check replaces the menu; a failed one (network
 * error, error status, timeout or a body `isMenu` rejects as not safe to draw) keeps the last
 * menu on screen, and 12 failures in a row add a small "offline" notice until the next good
 * check. On the first good check after 8,640 checks (about a day) it calls `reload`, by default
 * a full page reload, so a server upgrade is picked up. Timing counts checks, never the TV
 * clock.
 *
 * The drawing is `TvView`, shared with the Display Settings preview. The menu's `display`
 * (checked field by field by `displayOf`, each falling back to its default without failing the
 * check) picks the layout and look; `pagesOf` splits the menu into that layout's pages and
 * `usePager` turns them every `page_seconds`. The page number survives menu updates and failed
 * checks; when it is past the last page it goes back to page 1. A changed interval restarts the
 * page timer, so the next turn comes one new interval after the check lands. `display` comes
 * from the last good check, so failed checks keep every setting. No style attributes in markup
 * (CSP); only validated colors reach CSS, as custom properties set through the CSSOM.
 */
export default function DisplayPage({ reload = () => window.location.reload() }: { reload?: () => void }) {
  const [menu, setMenu] = useState<Menu | null>(null);
  const [failures, setFailures] = useState(0);

  // reload is read once at mount: the page never changes it.
  useEffect(() => {
    let stopped = false;
    let ticks = 0;
    let failed = 0;
    let next: ReturnType<typeof setTimeout> | undefined;
    let timeout: ReturnType<typeof setTimeout> | undefined;
    let ctrl: AbortController | undefined;

    async function check() {
      ticks++;
      ctrl = new AbortController();
      const c = ctrl;
      timeout = setTimeout(() => c.abort(), TIMEOUT_MS);
      let good: Menu | null = null;
      try {
        const m = await apiFetch<unknown>("GET", "/api/menu", undefined, c.signal);
        if (isMenu(m)) good = m;
      } catch {
        // Any failure is the same: keep the last menu and try again next tick.
      }
      clearTimeout(timeout);
      if (stopped) return;
      if (good) {
        failed = 0;
        setMenu(good);
        if (ticks >= RELOAD_AFTER) {
          ticks = 0;
          reload();
        }
      } else {
        failed++;
      }
      setFailures(failed);
      next = setTimeout(check, POLL_MS);
    }

    void check();
    return () => {
      stopped = true;
      clearTimeout(next);
      clearTimeout(timeout);
      ctrl?.abort();
    };
  }, []);

  const display = displayOf(menu?.display);
  const pages = menu ? pagesOf(menu, display.layout) : [];
  const [page] = usePager(pages.length, display.pageMs);

  return <TvView menu={menu} display={display} pages={pages} page={page} failures={failures} offlineAfter={OFFLINE_AFTER} />;
}
