import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { beer, brewfatherStatus, keg, kegTypes, owner, Reply, settings, stubApi, tap } from "../testApi";
import AdminPage from "./AdminPage";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});

// Populated lists, so keg cards, beer rows and tap cards (empty and full) all render.
const routes: Record<string, Reply> = {
  "GET /api/setup": { status: 200, body: { needed: false } },
  "GET /api/session": { status: 200, body: owner },
  "GET /api/health": { status: 200, body: { status: "ok", version: "1.2.3", database: "ok" } },
  "GET /api/settings": { status: 200, body: settings() },
  "GET /api/taps": {
    status: 200,
    body: [
      tap(),
      tap({ id: 2, position: 2, label: "Tap 2", keg: keg({ id: 2, tap_id: 2, state: "on_tap", beer_color: "#c0a020" }) }),
    ],
  },
  "GET /api/kegs": { status: 200, body: [keg({ beer_color: "#c0a020" })] },
  "GET /api/kegs?state=waiting": { status: 200, body: [keg()] },
  "GET /api/kegs?state=kicked": { status: 200, body: [keg({ id: 3, state: "kicked", remaining_l: 0 })] },
  "GET /api/keg-types": { status: 200, body: kegTypes },
  "GET /api/beers": { status: 200, body: [beer({ srm: 8, color: "#c0a020", abv: 5.2 })] },
  "GET /api/admins": { status: 200, body: [owner] },
  "GET /api/tokens": { status: 200, body: [] },
  "GET /api/brewfather": { status: 200, body: brewfatherStatus() },
};

// Nav group and label for every page.
const pages: [string, string][] = [
  ["Menu", "Taps"],
  ["Menu", "On Deck"],
  ["Menu", "Keg History"],
  ["Menu", "Beer Library"],
  ["Menu", "Display Settings"],
  ["Administration", "Admin Users"],
  ["Administration", "API Tokens"],
  ["Administration", "Change Password"],
  ["Integrations", "Brewfather"],
];

// Page, the button that opens the dialog, and the dialog's name.
const dialogs: [string, string, string][] = [
  ["Taps", "Add tap", "Add tap"],
  ["Taps", "Add keg", "Add keg"],
  ["On Deck", "Add keg", "Add keg"],
  ["Beer Library", "Add beer", "Add beer"],
  ["Beer Library", "Edit", "Edit Pale"],
  ["API Tokens", "New token", "New API token"],
  ["Admin Users", "Add admin", "Add admin"],
];

const nav = () => screen.getByRole("navigation", { name: "Admin" });

/** openPage signs in and opens `label` from the side panel, with its data loaded. */
async function openPage(label: string) {
  stubApi(routes);
  const result = render(<AdminPage />);
  await screen.findByRole("heading", { level: 2, name: "Taps" });
  const [group] = pages.find(([, l]) => l === label)!;
  if (!within(nav()).queryByRole("button", { name: label })) {
    fireEvent.click(within(nav()).getByRole("button", { name: group }));
  }
  fireEvent.click(within(nav()).getByRole("button", { name: label }));
  await screen.findByRole("heading", { level: 2, name: label });
  // Let the page's own requests settle so its loaded state is what gets checked.
  await waitFor(() => expect(screen.queryByText("Loading…")).toBeNull());
  return result.container;
}

/** expectNoInlineStyle fails on any `style` attribute but `--nav-width` on the `.admin` root. */
function expectNoInlineStyle(container: HTMLElement) {
  const root = container.firstElementChild as HTMLElement;
  expect(root.classList.contains("admin")).toBe(true);
  const styled = [...container.querySelectorAll<HTMLElement>("[style]")].filter((el) => el !== root);
  expect(styled.map((el) => el.outerHTML.slice(0, 120))).toEqual([]);
  const props = [...root.style].filter((p) => p !== "--nav-width");
  expect(props).toEqual([]);
}

describe("admin styling", () => {
  it.each(pages)("has no inline style on the %s group's %s page", async (_, label) => {
    expectNoInlineStyle(await openPage(label));
  });

  it.each(dialogs)("has no inline style on %s with %s's dialog open", async (page, button, name) => {
    const container = await openPage(page);
    const main = screen.getByRole("main");
    fireEvent.click((await within(main).findAllByRole("button", { name: button }))[0]);
    expect(await screen.findByRole("dialog", { name })).toBeTruthy();
    expectNoInlineStyle(container);
  });
});

/** rules walks `text` (comments removed) and returns every style rule's selector and body. */
function rules(text: string): { selector: string; body: string }[] {
  const out: { selector: string; body: string }[] = [];
  let i = 0;
  while (i < text.length) {
    const open = text.indexOf("{", i);
    if (open < 0) break;
    const prelude = text.slice(i, open).trim();
    // Find the matching close brace.
    let depth = 1;
    let j = open + 1;
    while (depth > 0 && j < text.length) {
      if (text[j] === "{") depth++;
      else if (text[j] === "}") depth--;
      j++;
    }
    const body = text.slice(open + 1, j - 1);
    if (/^@(media|supports|container|layer)\b/.test(prelude)) {
      out.push(...rules(body));
    } else if (!/^@keyframes\b/.test(prelude)) {
      out.push({ selector: prelude, body });
    }
    i = j;
  }
  return out;
}

// Vitest blanks CSS imports (even `?raw`), so read the file. A non-literal specifier keeps tsc
// from needing @types/node, which this project does not install.
const fs = (await import("node:fs" as string)) as { readFileSync(path: string, encoding: "utf8"): string };
// fs gets a path string: jsdom's URL is not Node's, and Vite rewrites `new URL(x, import.meta.url)`.
const css = fs.readFileSync(decodeURIComponent(new URL(import.meta.url).pathname).replace(/[^/]+$/, "AdminPage.css"), "utf8");
const cssRules = rules(css.replace(/\/\*[\s\S]*?\*\//g, ""));

describe("AdminPage.css", () => {
  it("has rules to check", () => {
    expect(cssRules.length).toBeGreaterThan(10);
  });

  it("scopes every selector under .admin", () => {
    const bad = cssRules
      .flatMap((r) => r.selector.split(/,(?![^(]*\))/)) // not commas inside :is() or :not()
      .map((s) => s.trim().replace(/\s+/g, " "))
      .filter((s) => !/^\.admin(?![\w-])/.test(s) && s !== "body:has(> #root > .admin)");
    expect(bad).toEqual([]);
  });

  it("makes every page heading as tall as a button, so pages don't jump", () => {
    const minHeight = (selector: string) =>
      cssRules
        .find((r) => r.selector.split(",").some((s) => s.trim() === selector) && /min-height/.test(r.body))
        ?.body.match(/min-height:\s*([^;]+)/)?.[1].trim();
    expect(minHeight(".admin .btn")).toBeDefined();
    expect(minHeight(".admin .page-title")).toBe(minHeight(".admin .btn"));
    expect(minHeight(".admin .page-header h2")).toBe(minHeight(".admin .btn"));
  });

  it("never uses the accent as a text color", () => {
    const bad = cssRules.flatMap((r) =>
      r.body
        .split(";")
        .map((d) => d.trim())
        .filter((d) => /^color\s*:/i.test(d) && /var\(\s*--accent\s*\)|#4169e1/i.test(d))
        .map((d) => `${r.selector} { ${d} }`),
    );
    expect(bad).toEqual([]);
  });
});
