// Builds the static user guide (contract: specs/017-user-guide/contracts/guide-site.md).
// Changes: specs/018-guide-compose-install/contracts/guide-site.md.
// Inputs: guide/src/layout.html, guide/src/guide.css, guide/src/copy.js, guide/src/pages/<slug>.html
// (page bodies), guide/src/images/*, the admin app's color tokens in web/src/pages/AdminPage.css,
// and the root docker-compose.yaml, from which a fragment's {{compose}} is filled.
// Output: guide/_site/ (emptied first): one <slug>.html per NAV entry, guide.css, copy.js and images/.
// Fails, naming the problem, when a NAV entry has no fragment, a fragment has no NAV entry, or
// the .admin / .admin.theme-dark block or a required token is missing from AdminPage.css.
// Node standard library only. Run: node guide/build.mjs
import { cpSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/** NAV is the only list of guide pages, in nav order (data-model.md). */
export const NAV = [
  { slug: "index", title: "Home", group: "Start" },
  { slug: "install", title: "Install with Docker", group: "Start" },
  { slug: "getting-started", title: "Getting started", group: "Start" },
  { slug: "taps", title: "Taps", group: "Menu" },
  { slug: "on-deck", title: "On Deck", group: "Menu" },
  { slug: "keg-history", title: "Keg History", group: "Menu" },
  { slug: "beers", title: "Beer Library", group: "Menu" },
  { slug: "display", title: "Display Settings", group: "Menu" },
  { slug: "admins", title: "Admin Users", group: "Administration" },
  { slug: "tokens", title: "API Tokens", group: "Administration" },
  { slug: "password", title: "Change Password", group: "Administration" },
  { slug: "two-factor", title: "Two-Factor Sign-In", group: "Administration" },
  { slug: "brewfather", title: "Brewfather", group: "Integrations" },
];

/** REQUIRED_TOKENS are the custom properties guide.css uses; the build fails without any of them. */
export const REQUIRED_TOKENS = [
  "--bg", "--surface", "--side", "--text", "--muted", "--border",
  "--accent", "--accent-tint", "--danger", "--radius", "--nav-width",
];

export const escape = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

/** declarations returns the custom properties of the first rule whose selector is exactly selector. */
function declarations(css, selector) {
  for (const [, sel, body] of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (sel.trim() === selector) return Object.fromEntries([...body.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
  }
  throw new Error(`AdminPage.css: no "${selector}" block`);
}

/** extractTokens returns the admin app's dark tokens (.admin overridden by .admin.theme-dark) from css. */
export function extractTokens(css) {
  const plain = css.replace(/\/\*[\s\S]*?\*\//g, "");
  const tokens = { ...declarations(plain, ".admin"), ...declarations(plain, ".admin.theme-dark") };
  const missing = REQUIRED_TOKENS.filter((t) => !(t in tokens));
  if (missing.length) throw new Error(`AdminPage.css: missing token(s) ${missing.join(" ")}`);
  return tokens;
}

/**
 * deriveCompose returns the image-only compose file the Install page shows, from the root
 * docker-compose.yaml: lines whose first non-space character is # and build: lines removed, runs
 * of blank lines collapsed to one, trimmed. CRLF line ends become LF.
 * shortcut: only a one-line `build: <path>` is removed; its mapping form would leave its
 * context: lines, so extend this if the root file adopts it.
 */
export function deriveCompose(text) {
  return text
    .split(/\r?\n/)
    .filter((line) => !/^\s*(#|build:)/.test(line))
    .join("\n")
    .replace(/\n\s*\n/g, "\n\n")
    .trim();
}

/** renderNav is the nav markup for the page current: one h2 per group, then its links. */
function renderNav(current) {
  const groups = [...new Set(NAV.map((p) => p.group))];
  return groups
    .map((g) => {
      const links = NAV.filter((p) => p.group === g).map((p) => {
        const mark = p.slug === current ? ' aria-current="page"' : "";
        return `<li><a href="${p.slug}.html"${mark}>${escape(p.title)}</a></li>`;
      });
      return `<h2>${escape(g)}</h2>\n<ul>\n${links.join("\n")}\n</ul>`;
    })
    .join("\n");
}

/** build writes the site under root/guide/_site and returns that folder. */
export function build({ root = join(fileURLToPath(import.meta.url), "..", "..") } = {}) {
  const src = join(root, "guide", "src");
  const out = join(root, "guide", "_site");
  const tokens = extractTokens(readFileSync(join(root, "web", "src", "pages", "AdminPage.css"), "utf8"));
  const style = `<style>:root{${Object.entries(tokens).map(([k, v]) => `${k}:${v};`).join("")}}</style>`;
  const layout = readFileSync(join(src, "layout.html"), "utf8");

  const slugs = new Set(NAV.map((p) => p.slug));
  const orphans = readdirSync(join(src, "pages")).filter((f) => f.endsWith(".html") && !slugs.has(f.slice(0, -5)));
  if (orphans.length) throw new Error(`no nav entry for guide/src/pages/${orphans.join(", ")}`);
  const missing = NAV.filter((p) => !existsSync(join(src, "pages", `${p.slug}.html`)));
  if (missing.length) throw new Error(`no fragment guide/src/pages/<slug>.html for: ${missing.map((p) => p.slug).join(", ")}`);
  // The build and upload-pages-artifact (tar --dereference) follow links, so one could publish any file on the runner.
  const links = readdirSync(src, { recursive: true, withFileTypes: true }).filter((d) => d.isSymbolicLink());
  if (links.length) throw new Error(`symbolic link(s) in guide/src: ${links.map((d) => join(d.parentPath, d.name)).join(", ")}`);

  const compose = escape(deriveCompose(readFileSync(join(root, "docker-compose.yaml"), "utf8")));

  rmSync(out, { recursive: true, force: true });
  mkdirSync(out, { recursive: true });
  for (const p of NAV) {
    // A function, so $& or $n in the compose text are never read as replacement patterns.
    const content = readFileSync(join(src, "pages", `${p.slug}.html`), "utf8").trim().replace("{{compose}}", () => compose);
    const fill = { title: escape(p.title), tokens: style, nav: renderNav(p.slug), content };
    writeFileSync(join(out, `${p.slug}.html`), layout.replace(/\{\{(\w+)\}\}/g, (_, key) => {
      if (!(key in fill)) throw new Error(`layout.html: unknown placeholder {{${key}}}`);
      return fill[key];
    }));
  }
  cpSync(join(src, "guide.css"), join(out, "guide.css"));
  cpSync(join(src, "copy.js"), join(out, "copy.js"));
  cpSync(join(src, "images"), join(out, "images"), { recursive: true });
  return out;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    console.log(`guide built in ${build()}`);
  } catch (err) {
    console.error(`guide build failed: ${err.message}`);
    process.exit(1);
  }
}
