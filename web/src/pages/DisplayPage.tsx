import { useEffect, useState } from "react";
import { apiFetch, type Menu, type MenuBeer, type MenuTap, type OnDeckItem, type Theme } from "../api";
import BeerArt from "../components/BeerArt";
import "./DisplayPage.css";

// 10 s keeps a change on screen within 30 s with room for a slow check (FR-020).
const POLL_MS = 10_000;
// Abort before the next check is due, so a hung request never stalls the chain.
const TIMEOUT_MS = 8_000;
// 12 checks is at least 2 minutes without a good one (FR-022), counted in ticks, not the TV clock.
const OFFLINE_AFTER = 12;
// 8,640 ticks of 10 s is about a day (FR-024); ticks, because the TV clock drifts and jumps.
const RELOAD_AFTER = 8_640;
// 4 × 2 keeps each glass big enough to read as a gauge from 3 m on a 16:9 TV (research R12).
// shortcut: fixed 8 per page, tuned for 16:9; measure if other screens matter.
const CARDS_PER_PAGE = 8;

/** Page is one screen of the rotation: tap cards or on-deck cards, never both. */
type Page = { taps: MenuTap[]; onDeck?: never } | { onDeck: OnDeckItem[]; taps?: never };

/** chunks splits items into runs of CARDS_PER_PAGE. */
function chunks<T>(items: T[]): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += CARDS_PER_PAGE) out.push(items.slice(i, i + CARDS_PER_PAGE));
  return out;
}

/** obj narrows a JSON value to an indexable non-null object. */
const obj = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;
/** opt reports whether v is null or of JS type t: an object here would throw as a React child. */
const opt = (v: unknown, t: "string" | "number") => v === null || typeof v === t;
const isBeer = (b: unknown) =>
  obj(b) && typeof b.name === "string" && opt(b.style, "string") && opt(b.color, "string") &&
  opt(b.abv, "number") && opt(b.ibu, "number");
const isKeg = (k: unknown) => k === null || (obj(k) && typeof k.level === "number" && typeof k.shape === "string");
const isTap = (t: unknown) => obj(t) && typeof t.label === "string" && (t.beer === null || isBeer(t.beer)) && isKeg(t.keg);
const isOnDeck = (d: unknown) => obj(d) && isBeer(d.beer) && isKeg(d.keg);

/**
 * isMenu reports whether a reply body has the menu's two arrays and every item is shaped well
 * enough that rendering cannot throw: a beer has a string name, style and color each null or a
 * string, and ABV and IBU each null or a number; a keg has a numeric level and a string shape.
 * A tap has a string label, a beer or null and a keg or null; an on-deck item has a beer (never
 * null) and a keg or null. A thrown render would unmount the page and stop polling, so a bad
 * body must fail here and count as a failed check (FR-021).
 */
function isMenu(m: unknown): m is Menu {
  return obj(m) && Array.isArray(m.taps) && Array.isArray(m.on_deck) && m.taps.every(isTap) && m.on_deck.every(isOnDeck);
}

/**
 * displayOf reads the TV settings from a menu's `display`, checking each field on its own and
 * falling back to its default (no title, dark, 15 s pages) when it is missing or malformed, so
 * an API that predates `display`, or a bad value in it, still draws the menu.
 */
function displayOf(menu: Menu | null): { title: string; theme: Theme; pageMs: number } {
  const d: unknown = menu?.display;
  const title = obj(d) && typeof d.title === "string" ? d.title : "";
  const theme = obj(d) && d.theme === "light" ? "light" : "dark";
  const s = obj(d) && typeof d.page_seconds === "number" ? d.page_seconds : 0;
  const pageSeconds = Number.isInteger(s) && s >= 5 && s <= 300 ? s : 15;
  return { title, theme, pageMs: pageSeconds * 1000 };
}

/** Facts is a beer's ABV as "N%" and, when `ibu`, its IBU as "N IBU", each only when known. */
function Facts({ beer, ibu }: { beer: MenuBeer; ibu: boolean }) {
  const showIbu = ibu && beer.ibu !== null;
  if (beer.abv === null && !showIbu) return null;
  return (
    <p className="facts">
      {beer.abv !== null && <span>{beer.abv}%</span>}
      {showIbu && <span>{beer.ibu} IBU</span>}
    </p>
  );
}

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
 * Enabled taps show as cards in menu order: label, a pint glass filled to the keg level, beer
 * name, style, ABV and IBU (each left out when unknown); an empty tap shows "Empty" and an
 * empty glass. On-deck items follow on their own pages under an "On deck" heading: waiting
 * kegs in their keg shape filled to their level, and beers still being made as a full corny keg
 * with their status. Pages hold at most 8 cards, taps first; the grid class is `taps-s` for 1
 * to 4 cards on the page and `taps-m` for 5 to 8. With more than one page a separate timer turns
 * the page every `page_seconds` and a "N / M" indicator shows. The page number survives menu updates and
 * failed checks; when it is past the last page (after the last page turns, or when an update
 * leaves fewer pages) it goes back to page 1. "Nothing on tap" shows only when both lists are
 * empty. No level number is shown; the level is only the fill and the art's label. No inline
 * styles (CSP): classes from DisplayPage.css.
 *
 * The menu's `display` sets the theme (class `light` on the page for the light palette in
 * DisplayPage.css, else dark), the title, shown as one heading line above the cards on every page
 * when non-empty, and the page interval. A changed interval restarts the page timer, so the
 * next turn comes one new interval after the check lands; an unchanged one keeps the timing.
 * `display` comes from the last good check, so failed checks keep the theme, title and interval, and
 * each field falls back to its default when missing or malformed (`displayOf`) without failing
 * the check.
 */
export default function DisplayPage({ reload = () => window.location.reload() }: { reload?: () => void }) {
  const [menu, setMenu] = useState<Menu | null>(null);
  const [failures, setFailures] = useState(0);
  const [page, setPage] = useState(0);

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

  const pages: Page[] = [
    ...chunks(menu?.taps ?? []).map((taps) => ({ taps })),
    ...chunks(menu?.on_deck ?? []).map((onDeck) => ({ onDeck })),
  ];
  const rotating = pages.length > 1;
  const { title, theme, pageMs } = displayOf(menu);

  // Only rotation starting or stopping, or a new interval, restarts this chain; a menu update keeps the timing.
  useEffect(() => {
    if (!rotating) return;
    let next: ReturnType<typeof setTimeout>;
    const turn = () => {
      setPage((p) => p + 1);
      next = setTimeout(turn, pageMs);
    };
    next = setTimeout(turn, pageMs);
    return () => clearTimeout(next);
  }, [rotating, pageMs]);

  // Past the last page (a turn or a smaller menu) starts over; React re-renders before painting.
  if (page > 0 && page >= pages.length) setPage(0);
  const cur = pages[page];
  const size = (cur?.taps ?? cur?.onDeck ?? []).length <= 4 ? "taps-s" : "taps-m";

  return (
    <main className={`display ${size}${theme === "light" ? " light" : ""}`}>
      {title && <h1 className="title">{title}</h1>}
      {menu && pages.length === 0 && <p className="nothing">Nothing on tap</p>}
      {cur?.taps && (
        <ul className="cards" aria-label="Taps">
          {cur.taps.map((t, i) => (
            // Labels need not be unique and the list is replaced whole each check.
            <li key={i} className="card">
              <p className="label">{t.label}</p>
              <BeerArt onTap color={t.beer?.color ?? null} name={t.beer?.name ?? "Empty"} level={t.keg?.level} height={240} />
              <div className="text">
                {t.beer ? (
                  <>
                    <p className="name">{t.beer.name}</p>
                    {t.beer.style !== null && <p className="style">{t.beer.style}</p>}
                    <Facts beer={t.beer} ibu />
                  </>
                ) : (
                  <p className="name">Empty</p>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
      {cur?.onDeck && (
        <>
          <h2 className="heading">On deck</h2>
          <ul className="cards" aria-label="On deck">
            {cur.onDeck.map((d, i) => (
              <li key={i} className="card">
                <BeerArt kegShape={d.keg?.shape} color={d.beer.color} name={d.beer.name} level={d.keg?.level} height={240} />
                <div className="text">
                  <p className="name">{d.beer.name}</p>
                  {d.beer.style !== null && <p className="style">{d.beer.style}</p>}
                  <Facts beer={d.beer} ibu={false} />
                  {d.status && <p className="status">{d.status === "fermenting" ? "Fermenting" : "Conditioning"}</p>}
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
      {rotating && (
        <div className="pager">
          {page + 1} / {pages.length}
        </div>
      )}
      {failures >= OFFLINE_AFTER && (
        <div className="offline" role="status">
          offline
        </div>
      )}
    </main>
  );
}
