// Checks the built guide against specs/017-user-guide/contracts/guide-site.md: builds once, then
// reads every page with regular expressions (no HTML parser dependency). Run: node --test guide/
import assert from "node:assert/strict";
import { cpSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import { fileURLToPath } from "node:url";
import { build, extractTokens, NAV, REQUIRED_TOKENS } from "./build.mjs";

const root = join(fileURLToPath(import.meta.url), "..", "..");
let out;
const pages = new Map();
const main = (html) => html.match(/<main id="main">([\s\S]*)<\/main>/)[1];
const nav = (html) => html.match(/<nav aria-label="Guide">([\s\S]*?)<\/nav>/)[1];

before(() => {
  out = build({ root });
  for (const f of readdirSync(out).filter((f) => f.endsWith(".html"))) pages.set(f, readFileSync(join(out, f), "utf8"));
});

test("every nav entry has a page and every page is in the nav", () => {
  assert.deepEqual([...pages.keys()].sort(), NAV.map((p) => `${p.slug}.html`).sort());
});

test("each page has lang, charset, viewport and the guide title", () => {
  for (const [f, html] of pages) {
    assert.match(html, /<html lang="en">/, f);
    assert.match(html, /<meta charset="utf-8">/, f);
    assert.match(html, /<meta name="viewport" content="width=device-width, initial-scale=1">/, f);
    assert.match(html, /<title>[^<]+ – Beer Menu User Guide<\/title>/, f);
  }
});

test("each page declares the admin app's dark tokens", () => {
  const css = readFileSync(join(root, "web", "src", "pages", "AdminPage.css"), "utf8");
  const darkBlock = css.match(/\.admin\.theme-dark\s*\{([^}]*)\}/)[1];
  const dark = [...darkBlock.matchAll(/(--[\w-]+):\s*([^;]+);/g)].map((m) => `${m[1]}:${m[2].trim()};`);
  assert.ok(dark.length, "no dark tokens found in AdminPage.css");
  for (const [f, html] of pages) {
    const rootRule = html.match(/:root\{([^}]*)\}/)[1];
    for (const t of REQUIRED_TOKENS) assert.match(rootRule, new RegExp(`(^|;)${t}:[^;]+;`), `${f} ${t}`);
    for (const d of dark) assert.ok(rootRule.includes(d), `${f}: missing dark value ${d}`);
  }
});

test("one h1 and no skipped heading level", () => {
  for (const [f, html] of pages) {
    assert.equal(html.match(/<h1[\s>]/g).length, 1, f);
    const levels = [...main(html).matchAll(/<h([1-6])[\s>]/g)].map((m) => Number(m[1]));
    assert.equal(levels[0], 1, f);
    levels.forEach((l, i) => i && assert.ok(l <= levels[i - 1] + 1, `${f}: h${levels[i - 1]} then h${l}`));
    for (const m of nav(html).matchAll(/<h([1-6])[\s>]/g)) assert.equal(m[1], "2", `${f} nav heading`);
  }
});

test("each nav lists every page in order and marks only the current one", () => {
  for (const { slug } of NAV) {
    const html = nav(pages.get(`${slug}.html`));
    assert.deepEqual([...html.matchAll(/<a href="([^"]+)"/g)].map((m) => m[1]), NAV.map((p) => `${p.slug}.html`));
    assert.deepEqual([...html.matchAll(/<h2>([^<]+)<\/h2>/g)].map((m) => m[1]), [...new Set(NAV.map((p) => p.group))]);
    const current = [...html.matchAll(/<a href="([^"]+)" aria-current="page"/g)].map((m) => m[1]);
    assert.deepEqual(current, [`${slug}.html`]);
  }
});

test("local links resolve and external links use https", () => {
  for (const [f, html] of pages) {
    for (const [, url] of html.matchAll(/(?:href|src)="([^"]*)"/g)) {
      if (url.startsWith("#") || url.startsWith("mailto:") || url.startsWith("https:")) continue;
      assert.doesNotMatch(url, /^[a-z][a-z0-9+.-]*:/i, `${f}: ${url} is not https`);
      assert.ok(existsSync(join(out, url.split("#")[0])), `${f}: ${url} does not exist`);
    }
  }
});

test("images have alt text and links name their target", () => {
  for (const [f, html] of pages) {
    for (const [img] of html.matchAll(/<img\b[^>]*>/g)) assert.match(img, /\salt="[^"]*\S[^"]*"/, `${f}: ${img}`);
    for (const [, text] of html.matchAll(/<a\b[^>]*>([\s\S]*?)<\/a>/g)) {
      const words = text.replace(/<[^>]+>/g, "").trim().toLowerCase();
      assert.ok(words && !["click here", "here", "this page"].includes(words), `${f}: link text "${words}"`);
    }
  }
});

test("no script and no inline event handlers", () => {
  for (const [f, html] of pages) {
    assert.doesNotMatch(html, /<script/i, f);
    assert.doesNotMatch(html, /<[^>]*\son[a-z]+\s*=/i, f);
  }
});

test("admin pages and Getting started have a screenshot and numbered steps", () => {
  for (const p of NAV.filter((p) => p.group !== "Start" || p.slug === "getting-started")) {
    const body = main(pages.get(`${p.slug}.html`));
    assert.match(body, /<figure>[\s\S]*?<img [^>]*src="images\/[^"]+"/, p.slug);
    assert.match(body, /<h2>[\s\S]*?<ol>/, p.slug);
  }
});

test("guide.css takes every color from the tokens", () => {
  assert.doesNotMatch(readFileSync(join(root, "guide", "src", "guide.css"), "utf8"), /#[0-9a-f]{3,8}\b|rgba?\(|hsla?\(/i);
});

test("build refuses a symbolic link in guide/src", () => {
  const tmp = mkdtempSync(join(tmpdir(), "guide-"));
  try {
    cpSync(join(root, "guide", "src"), join(tmp, "guide", "src"), { recursive: true });
    cpSync(join(root, "web", "src", "pages", "AdminPage.css"), join(tmp, "web", "src", "pages", "AdminPage.css"));
    symlinkSync("/etc/hostname", join(tmp, "guide", "src", "images", "leak.png"));
    assert.throws(() => build({ root: tmp }), /symbolic link\(s\) in guide\/src: .*leak\.png/);
    assert.ok(!existsSync(join(tmp, "guide", "_site")), "nothing is built");
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
});

test("token extraction fails clearly without the dark block", () => {
  assert.throws(() => extractTokens(".admin { --bg: #fff; }"), /no "\.admin\.theme-dark" block/);
});
