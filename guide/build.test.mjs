// Checks the built guide against specs/017-user-guide/contracts/guide-site.md and its changes in
// specs/018-guide-compose-install/contracts/guide-site.md: builds once, then reads every page with
// regular expressions (no HTML parser dependency). Run: node --test guide/
import assert from "node:assert/strict";
import { cpSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { before, test } from "node:test";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { build, deriveCompose, escape, extractTokens, NAV, REQUIRED_TOKENS } from "./build.mjs";

const root = join(fileURLToPath(import.meta.url), "..", "..");
let out;
const pages = new Map();
const main = (html) => html.match(/<main id="main">([\s\S]*)<\/main>/)[1];
const nav = (html) => html.match(/<nav aria-label="Guide">([\s\S]*?)<\/nav>/)[1];
const compose = () => deriveCompose(readFileSync(join(root, "docker-compose.yaml"), "utf8"));
const COPY_SCRIPT = '<script src="copy.js" defer></script>';
// href and src values in any quoting, so href='javascript:…' or an unquoted value cannot slip past.
const URL_ATTR = /\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]*))/gi;
const urls = (html) => [...html.matchAll(URL_ATTR)].map((m) => m[1] ?? m[2] ?? m[3]);
// Browsers also split attributes on / and after a closing quote: <svg/onload=…>, <a href="x"onclick=…>.
const HANDLER = /<[^>]*[\s/"']on[a-z]+\s*=/i;
// Script by other means: a frame or object document, or a base that redirects copy.js.
const EMBED = /<(?:base|iframe|frame|object|embed)\b|\ssrcdoc\s*=/i;

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
    for (const url of urls(html)) {
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

test("no inline script and no inline event handlers", () => {
  const scripts = /<script\b[^>]*>[\s\S]*?<\/script>/gi;
  for (const [f, html] of pages) {
    for (const [tag] of html.matchAll(scripts)) assert.equal(tag, COPY_SCRIPT, f);
    assert.doesNotMatch(html.replace(scripts, ""), /<script/i, f);
    assert.doesNotMatch(html, HANDLER, f);
    assert.doesNotMatch(html, EMBED, f);
  }
});

test("script and link checks catch known bypasses", () => {
  for (const s of ['<svg/onload=alert(1)>', '<a href="x.html"onclick="alert(1)">', "<b\nonmouseover='x'>"]) assert.match(s, HANDLER, s);
  for (const s of ['<base href="https://e.example/">', "<IFRAME src=x>", '<p srcdoc="x">', "<object data=x>", "<embed src=x>"]) assert.match(s, EMBED, s);
  assert.deepEqual(urls(`<a href='javascript:a'><a href=javascript:b><a HREF="javascript:c d"><img src = "e.png">`), ["javascript:a", "javascript:b", "javascript:c d", "e.png"]);
});

test("derived compose file is image-only", () => {
  const yaml = compose();
  for (const re of [/^services:$/m, /^  api:$/m, /^  web:$/m, /image: ghcr\.io\/helmnet\/beer-menu-api:/, /image: ghcr\.io\/helmnet\/beer-menu-web:/]) {
    assert.match(yaml, re);
  }
  for (const re of [/#/, /^\s*build:/m, /\n\n\n/]) assert.doesNotMatch(yaml, re);
  assert.equal(yaml, yaml.trim());
  assert.equal(
    deriveCompose("# c\nservices:\n  a:\n    build: ./a\n    # x\n    image: i\n\n\n\n  b:\n    image: j\n# end\n"),
    "services:\n  a:\n    image: i\n\n  b:\n    image: j",
  );
  assert.equal(deriveCompose("# c\r\nservices:\r\n  a:\r\n    build: ./a\r\n\r\n\r\n    image: i\r\n"), "services:\n  a:\n\n    image: i");
});

test("install page shows the derived compose file and no clone steps", () => {
  const body = main(pages.get("install.html"));
  const shown = [...body.matchAll(/<pre class="copy"><code>([\s\S]*?)<\/code><\/pre>/g)].map((m) => m[1]);
  assert.ok(shown.includes(escape(compose())), "install page shows the derived compose file");
  assert.ok(body.includes(COPY_SCRIPT), "install page loads copy.js");
  const pres = body.match(/<pre\b[^>]*>/g);
  assert.ok(pres.length > 1);
  assert.deepEqual(pres.filter((p) => p !== '<pre class="copy">'), [], "every example has a Copy button");
  for (const re of [/git clone/i, /Download ZIP/i, /beer-menu-main/]) assert.doesNotMatch(body, re);
  // up -d pulls missing images itself; pull belongs only to the update steps.
  assert.doesNotMatch(body.slice(0, body.indexOf("</ol>")), /docker compose pull/, "first start has no pull step");
});

test("install page documents every variable the API and compose file read", () => {
  const dir = join(root, "api", "cmd", "beer-menu-api");
  const names = new Set();
  for (const f of readdirSync(dir).filter((f) => f.endsWith(".go") && !f.endsWith("_test.go"))) {
    for (const [, n] of readFileSync(join(dir, f), "utf8").matchAll(/"(BEER_MENU_[A-Z_]+)"/g)) names.add(n);
  }
  for (const [, n] of readFileSync(join(root, "docker-compose.yaml"), "utf8").matchAll(/\$\{([A-Z_]+)/g)) names.add(n);
  // Internal to the containers; changing them breaks the data mount or the web proxy.
  names.delete("BEER_MENU_DATA_DIR");
  names.delete("BEER_MENU_PORT");
  assert.ok(names.size, "no variables found");
  const body = main(pages.get("install.html"));
  for (const n of names) assert.match(body, new RegExp("<code>[^<]*\\b" + n + "\\b"), n);
  const section = body.indexOf("<h2>Environment variables</h2>");
  const steps = [body.indexOf("<ol>"), body.indexOf(escape(compose()))];
  assert.ok(steps.every((i) => i >= 0 && i < section), "Environment variables comes after the basic steps");
});

test("home page links the GitHub repository", () => {
  const body = main(pages.get("index.html"));
  const link = '<a href="https://github.com/helmnet/beer-menu">';
  assert.equal(body.split(link).length - 1, 1, "exactly one repository link");
  assert.ok(body.indexOf(link) < body.indexOf("<figure>"), "repository link comes before the first figure");
});

/** runCopy runs copy.js against one fake pre.copy block with the given navigator. */
function runCopy(navigator) {
  const code = { textContent: "services:\n  api:" };
  const fake = { wrappers: [], timeouts: [], selected: undefined, code };
  const pre = { querySelector: () => code, before: (el) => fake.wrappers.push(el) };
  const document = {
    querySelectorAll: (s) => (s === "pre.copy" ? [pre] : []),
    createElement: (tag) => ({
      tag,
      children: [],
      listeners: {},
      append(...els) { this.children.push(...els); },
      addEventListener(t, f) { this.listeners[t] = f; },
    }),
  };
  const getSelection = () => ({ selectAllChildren: (n) => (fake.selected = n) });
  const setTimeout = (f) => fake.timeouts.push(f);
  vm.runInContext(readFileSync(join(root, "guide", "src", "copy.js"), "utf8"), vm.createContext({ document, navigator, getSelection, setTimeout }));
  return fake;
}

test("copy.js adds a Copy button that copies the code text", async () => {
  let written;
  const ok = runCopy({ clipboard: { writeText: async (t) => { written = t; } } });
  assert.equal(ok.wrappers.length, 1);
  const [wrapper] = ok.wrappers;
  assert.equal(wrapper.tag, "div");
  assert.equal(wrapper.className, "copy-block");
  assert.equal(wrapper.children.length, 2);
  assert.equal(wrapper.children[0].querySelector(), ok.code, "the wrapper holds the pre");
  const button = wrapper.children[1];
  assert.equal(button.tag, "button");
  assert.equal(button.type, "button");
  assert.equal(button.className, "copy-button");
  assert.equal(button.textContent, "Copy");
  await button.listeners.click();
  assert.equal(written, ok.code.textContent);
  assert.equal(button.textContent, "Copied");
  assert.equal(ok.timeouts.length, 1);
  ok.timeouts[0]();
  assert.equal(button.textContent, "Copy");

  for (const navigator of [{ clipboard: { writeText: async () => { throw new Error("denied"); } } }, {}]) {
    const failed = runCopy(navigator);
    const button = failed.wrappers[0].children[1];
    await button.listeners.click();
    assert.equal(button.textContent, "Copy failed");
    assert.equal(failed.selected, failed.code);
  }

  assert.ok(existsSync(join(out, "copy.js")), "copy.js is in _site");
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
