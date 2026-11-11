import {
  Beer,
  FlaskConical,
  GlassWater,
  History,
  Hourglass,
  KeyRound,
  Library,
  Lock,
  LogOut,
  LucideIcon,
  Menu,
  Plug,
  Shield,
  Tv,
  Users,
  X,
} from "lucide-react";
import { KeyboardEvent, PointerEvent, ReactNode, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { clampNavWidth, loadNavWidth, MIN_NAV_WIDTH, saveNavWidth } from "../prefs";

/** GroupKey names a side panel group. */
export type GroupKey = "menu" | "administration" | "integrations";

/** PageKey names an admin page. */
export type PageKey =
  | "taps"
  | "on-deck"
  | "keg-history"
  | "beers"
  | "display"
  | "admins"
  | "tokens"
  | "password"
  | "brewfather";

/** groups is the side panel tree (data-model.md "Page tree"), in display order. */
const groups: { key: GroupKey; label: string; icon: LucideIcon; pages: { key: PageKey; label: string; icon: LucideIcon }[] }[] = [
  {
    key: "menu",
    label: "Menu",
    icon: Beer,
    pages: [
      { key: "taps", label: "Taps", icon: GlassWater },
      { key: "on-deck", label: "On Deck", icon: Hourglass },
      { key: "keg-history", label: "Keg History", icon: History },
      { key: "beers", label: "Beer Library", icon: Library },
      { key: "display", label: "Display Settings", icon: Tv },
    ],
  },
  {
    key: "administration",
    label: "Administration",
    icon: Shield,
    pages: [
      { key: "admins", label: "Admin Users", icon: Users },
      { key: "tokens", label: "API Tokens", icon: KeyRound },
      { key: "password", label: "Change Password", icon: Lock },
    ],
  },
  {
    key: "integrations",
    label: "Integrations",
    icon: Plug,
    pages: [{ key: "brewfather", label: "Brewfather", icon: FlaskConical }],
  },
];

/** groupOf returns the group that holds page. */
export function groupOf(page: PageKey): GroupKey {
  return groups.find((g) => g.pages.some((p) => p.key === page))!.key;
}

/** isPageKey reports whether s names an admin page, for untrusted input such as the URL hash. */
export function isPageKey(s: string): s is PageKey {
  return groups.some((g) => g.pages.some((p) => p.key === s));
}

/** pageLabel returns the page's label as shown in the side panel and as its heading. */
function pageLabel(page: PageKey): string {
  return groups.flatMap((g) => g.pages).find((p) => p.key === page)!.label;
}

const STEP = 16;

/**
 * SideNav is the admin side panel: a navigation landmark named "Admin" with the three groups
 * (each a button with `aria-expanded` controlling its page list) and, under the open group
 * `openGroup` only, its pages; the current `page` has `aria-current="page"`. It is controlled:
 * a group click calls `onGroup`, a page click `onSelect`. The bottom holds the `theme` slot,
 * "Version <version>" ("unknown" when empty, nothing while `version` is null, i.e. loading)
 * and Sign out (`onSignOut`).
 *
 * Below 768 px (CSS only) the panel is a drawer: a top bar (a `banner`) holds the menu button
 * ("Open menu" / "Close menu", `aria-expanded`, `aria-controls` the navigation), the current
 * page's title and an empty element handed to `actionSlot`, where the page can portal its
 * primary action. Opening moves focus to the current page item, or to the first group when
 * that page's group is collapsed. Selecting a page, clicking the decorative backdrop or
 * pressing Escape closes it and returns focus to the menu button; the Escape listener exists
 * only while the drawer is open. SideNav owns this open state, since nothing else needs it.
 *
 * SideNav owns the panel width: loaded from prefs, clamped to the window and re-clamped on
 * window resize, changed by dragging the "Resize side panel" separator (pointer capture) or
 * with Left/Right arrows on it (16 px), and saved on every change. It writes the width as
 * `--nav-width` on its nearest `.admin` ancestor through CSSOM, the one style the CSP allows
 * (research R8); no element it renders carries a style attribute.
 */
export default function SideNav({
  page,
  openGroup,
  onGroup,
  onSelect,
  version,
  onSignOut,
  theme,
  actionSlot,
}: {
  page: PageKey;
  openGroup: GroupKey | null;
  onGroup: (group: GroupKey) => void;
  onSelect: (page: PageKey) => void;
  version: string | null;
  onSignOut: () => void;
  theme?: ReactNode;
  actionSlot?: (el: HTMLDivElement | null) => void;
}) {
  const id = useId();
  const navRef = useRef<HTMLElement>(null);
  const menuRef = useRef<HTMLButtonElement>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [width, setWidth] = useState(() => loadNavWidth());
  // The width the owner asked for; a narrow window shrinks what shows, not this.
  const wanted = useRef(width);
  const [dragging, setDragging] = useState(false);
  const [maxWidth, setMaxWidth] = useState(() => clampNavWidth(Infinity, window.innerWidth));

  useLayoutEffect(() => {
    const root = navRef.current?.closest<HTMLElement>(".admin");
    root?.style.setProperty("--nav-width", `${width}px`);
  }, [width]);

  useEffect(() => {
    const onResize = () => {
      setMaxWidth(clampNavWidth(Infinity, window.innerWidth));
      setWidth(clampNavWidth(wanted.current, window.innerWidth));
    };
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    if (!drawerOpen || !navRef.current) return;
    const current = navRef.current.querySelector<HTMLElement>('[aria-current="page"]');
    const target = current && !current.closest("[hidden]") ? current : navRef.current.querySelector<HTMLElement>(".nav-group");
    target?.focus();
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key === "Escape") closeDrawer();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [drawerOpen]);

  function closeDrawer() {
    setDrawerOpen(false);
    menuRef.current?.focus();
  }

  function select(p: PageKey) {
    onSelect(p);
    if (drawerOpen) closeDrawer();
  }

  function resize(px: number) {
    const next = clampNavWidth(px, window.innerWidth);
    wanted.current = next;
    setWidth(next);
    saveNavWidth(next);
  }

  function onHandleKey(e: KeyboardEvent) {
    if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
      e.preventDefault();
      resize(width + (e.key === "ArrowRight" ? STEP : -STEP));
    }
  }

  function onPointerDown(e: PointerEvent<HTMLDivElement>) {
    e.currentTarget.setPointerCapture(e.pointerId);
    setDragging(true);
  }

  function onPointerMove(e: PointerEvent<HTMLDivElement>) {
    if (!dragging || !navRef.current) return;
    resize(e.clientX - navRef.current.getBoundingClientRect().left);
  }

  function onPointerUp(e: PointerEvent<HTMLDivElement>) {
    if (e.currentTarget.hasPointerCapture(e.pointerId)) e.currentTarget.releasePointerCapture(e.pointerId);
    setDragging(false);
  }

  return (
    <>
      <header className="topbar">
        <button
          ref={menuRef}
          type="button"
          className="icon-btn"
          aria-label={drawerOpen ? "Close menu" : "Open menu"}
          aria-expanded={drawerOpen}
          aria-controls={`${id}-nav`}
          onClick={() => (drawerOpen ? closeDrawer() : setDrawerOpen(true))}
        >
          {drawerOpen ? <X aria-hidden="true" /> : <Menu aria-hidden="true" />}
        </button>
        <span className="topbar-title">{pageLabel(page)}</span>
        <div ref={actionSlot} className="topbar-action" />
      </header>
      {drawerOpen && <div className="nav-backdrop" aria-hidden="true" onClick={closeDrawer} />}
      <nav ref={navRef} id={`${id}-nav`} className={drawerOpen ? "side-nav nav-open" : "side-nav"} aria-label="Admin">
        {/* The nav stretches to the page height for its background; this column stays in view. */}
        <div className="side-inner">
          <div className="side-brand">
            <Beer aria-hidden="true" />
            <span>Beer Menu</span>
          </div>
          <ul className="nav-tree">
            {groups.map((g) => {
              const open = openGroup === g.key;
              const listId = `${id}-${g.key}`;
              return (
                <li key={g.key}>
                  <button
                    type="button"
                    className="nav-item nav-group"
                    aria-expanded={open}
                    aria-controls={listId}
                    onClick={() => onGroup(g.key)}
                  >
                    <g.icon aria-hidden="true" />
                    <span>{g.label}</span>
                  </button>
                  <ul id={listId} className="nav-pages" hidden={!open}>
                    {g.pages.map((p) => (
                      <li key={p.key}>
                        <button
                          type="button"
                          className="nav-item nav-page"
                          aria-current={page === p.key ? "page" : undefined}
                          onClick={() => select(p.key)}
                        >
                          <p.icon aria-hidden="true" />
                          <span>{p.label}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </li>
              );
            })}
          </ul>
          <div className="nav-bottom">
            {theme}
            <p className="muted nav-version">{version !== null && `Version ${version || "unknown"}`}</p>
            <button type="button" className="nav-item" onClick={onSignOut}>
              <LogOut aria-hidden="true" />
              <span>Sign out</span>
            </button>
          </div>
          <div
            role="separator"
            className={dragging ? "resize-handle dragging" : "resize-handle"}
            aria-label="Resize side panel"
            aria-orientation="vertical"
            aria-valuenow={width}
            aria-valuemin={MIN_NAV_WIDTH}
            aria-valuemax={maxWidth}
            tabIndex={0}
            onKeyDown={onHandleKey}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={onPointerUp}
          />
        </div>
      </nav>
    </>
  );
}
