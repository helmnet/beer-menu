import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { brewfatherStatus, keg, kegTypes, owner, Reply, settings, stubApi, tap, testPassword } from "../testApi";
import AdminPage from "./AdminPage";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});

const unauthenticated = {
  status: 401,
  body: { error: "unauthenticated", message: "Sign in or send a valid API token." },
};

const signedIn = {
  "GET /api/setup": { status: 200, body: { needed: false } },
  "GET /api/session": { status: 200, body: owner },
};

// The shell loads health and settings after sign-in; each page loads its own lists when shown.
const panels: Record<string, Reply> = {
  "GET /api/health": { status: 200, body: { status: "ok", version: "1.2.3", database: "ok" } },
  "GET /api/settings": { status: 200, body: settings() },
  "GET /api/taps": { status: 200, body: [] },
  "GET /api/kegs": { status: 200, body: [] },
  "GET /api/kegs?state=waiting": { status: 200, body: [] },
  "GET /api/kegs?state=kicked": { status: 200, body: [] },
  "GET /api/keg-types": { status: 200, body: [] },
  "GET /api/beers": { status: 200, body: [] },
  "GET /api/admins": { status: 200, body: [] },
  "GET /api/tokens": { status: 200, body: [] },
  "GET /api/brewfather": { status: 200, body: brewfatherStatus() },
};

// Nav group, page label and the page's h2, for every page.
const pages: [string, string, string][] = [
  ["Menu", "Taps", "Taps"],
  ["Menu", "On Deck", "On Deck"],
  ["Menu", "Keg History", "Keg History"],
  ["Menu", "Beer Library", "Beer Library"],
  ["Menu", "Display Settings", "Display Settings"],
  ["Administration", "Admin Users", "Admin Users"],
  ["Administration", "API Tokens", "API Tokens"],
  ["Administration", "Change Password", "Change Password"],
  ["Integrations", "Brewfather", "Brewfather"],
];

function fillCredentials(password: string) {
  fireEvent.change(screen.getByLabelText("Username"), { target: { value: "owner" } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
}

const nav = () => screen.getByRole("navigation", { name: "Admin" });

/** goTo opens page `label` from the side panel, opening its group first when collapsed. */
function goTo(label: string) {
  const [group] = pages.find(([, l]) => l === label) ?? [];
  if (!within(nav()).queryByRole("button", { name: label })) {
    fireEvent.click(within(nav()).getByRole("button", { name: group }));
  }
  fireEvent.click(within(nav()).getByRole("button", { name: label }));
}

const heading = (name: string) => screen.queryByRole("heading", { level: 2, name });
const count = (calls: { key: string }[], key: string) => calls.filter((c) => c.key === key).length;

describe("AdminPage", () => {
  it("shows setup when needed, then the Taps page", async () => {
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: true } },
      "POST /api/setup": { status: 201, body: owner },
      ...panels,
    });
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Create admin" });
    expect(screen.queryByRole("navigation", { name: "Admin" })).toBeNull();
    const pw = testPassword();
    fillCredentials(pw);
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: pw } });
    fireEvent.click(screen.getByRole("button", { name: "Create admin" }));
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
    expect(calls.map((c) => c.key).slice(0, 2)).toEqual(["GET /api/setup", "POST /api/setup"]);
  });

  it("switches to sign-in when setup was already done", async () => {
    stubApi({
      "GET /api/setup": { status: 200, body: { needed: true } },
      "POST /api/setup": { status: 409, body: { error: "setup_done", message: "done" } },
    });
    render(<AdminPage />);
    await screen.findByRole("button", { name: "Create admin" });
    const pw = testPassword();
    fillCredentials(pw);
    fireEvent.change(screen.getByLabelText("Confirm password"), { target: { value: pw } });
    fireEvent.click(screen.getByRole("button", { name: "Create admin" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("shows sign-in without a session, then signs in and out", async () => {
    const calls = stubApi({
      "GET /api/setup": { status: 200, body: { needed: false } },
      "GET /api/session": unauthenticated,
      "POST /api/session": { status: 200, body: owner },
      "DELETE /api/session": { status: 204 },
      ...panels,
    });
    const { container } = render(<AdminPage />);
    await screen.findByRole("button", { name: "Sign in" });
    expect(container.firstElementChild?.classList.contains("admin")).toBe(true);
    expect(screen.queryByRole("navigation", { name: "Admin" })).toBeNull();
    fillCredentials(testPassword());
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();

    fireEvent.click(within(nav()).getByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(calls.map((c) => c.key)).toContain("DELETE /api/session");
  });

  it("returns to the hash's page after a 401 and signing in again", async () => {
    const routes: Record<string, Reply> = {
      ...signedIn,
      ...panels,
      "GET /api/tokens": unauthenticated,
      "POST /api/session": { status: 200, body: owner },
    };
    stubApi(routes);
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("API Tokens");
    await screen.findByRole("button", { name: "Sign in" });
    routes["GET /api/tokens"] = panels["GET /api/tokens"];
    fillCredentials(testPassword());
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("heading", { level: 2, name: "API Tokens" })).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "API Tokens" }).getAttribute("aria-current")).toBe("page");
    expect(window.location.hash).toBe("#tokens");
  });

  it("clears the hash on Sign out, so signing in again shows Taps", async () => {
    stubApi({
      ...signedIn,
      ...panels,
      "DELETE /api/session": { status: 204 },
      "POST /api/session": { status: 200, body: owner },
    });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("API Tokens");
    await screen.findByRole("heading", { level: 2, name: "API Tokens" });
    const length = window.history.length;
    fireEvent.click(within(nav()).getByRole("button", { name: "Sign out" }));
    await screen.findByRole("button", { name: "Sign in" });
    expect(window.location.hash).toBe("");
    expect(window.history.length).toBe(length);
    fillCredentials(testPassword());
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
    expect(window.location.hash).toBe("");
  });

  it("still signs out when the path starts with //", async () => {
    // nginx serves the app at any path; "//evil.example/x" as a relative URL is another origin.
    window.history.replaceState(null, "", `${window.location.origin}//evil.example/x#tokens`);
    stubApi({ ...signedIn, ...panels, "DELETE /api/session": { status: 204 } });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "API Tokens" });
    fireEvent.click(within(nav()).getByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
    expect(window.location.pathname).toBe("//evil.example/x");
    expect(window.location.hash).toBe("");
  });

  it("puts the chosen page in the URL hash, once per page", async () => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    const start = window.history.length;
    goTo("Beer Library");
    await screen.findByRole("heading", { level: 2, name: "Beer Library" });
    expect(window.location.hash).toBe("#beers");
    expect(window.location.pathname).toBe("/");
    expect(window.history.length).toBe(start + 1);
    goTo("Beer Library");
    expect(window.history.length).toBe(start + 1);
    goTo("Display Settings");
    await screen.findByRole("heading", { level: 2, name: "Display Settings" });
    expect(window.location.hash).toBe("#display");
    expect(window.location.pathname).toBe("/");
  });

  it("goes back to the previous page on Back", async () => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("Keg History");
    await screen.findByRole("heading", { level: 2, name: "Keg History" });
    goTo("API Tokens");
    await screen.findByRole("heading", { level: 2, name: "API Tokens" });
    window.history.back();
    expect(await screen.findByRole("heading", { level: 2, name: "Keg History" })).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "Keg History" }).getAttribute("aria-current")).toBe("page");
    window.history.back();
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
  });

  it("follows a hash change, opening the page's group", async () => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    window.location.hash = "brewfather";
    expect(await screen.findByRole("heading", { level: 2, name: "Brewfather" })).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "Brewfather" }).getAttribute("aria-current")).toBe("page");
    window.location.hash = "nope";
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
  });

  it("opens the hash's page on load", async () => {
    window.history.replaceState(null, "", "/#on-deck");
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { level: 2, name: "On Deck" })).toBeTruthy();
    expect(heading("Taps")).toBeNull();
  });

  it.each(["#nope", "#constructor", "#", "#taps%20"])("opens Taps for the hash %s", async (hash) => {
    window.history.replaceState(null, "", `/${hash}`);
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "Taps" }).getAttribute("aria-current")).toBe("page");
  });

  it("shows only the Taps page for an existing session", async () => {
    const calls = stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    expect(await screen.findByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
    expect(within(nav()).getByRole("button", { name: "Taps" }).getAttribute("aria-current")).toBe("page");
    for (const name of ["Keg History", "Beer Library", "Brewfather", "Display Settings", "API Tokens"]) {
      expect(heading(name)).toBeNull();
    }
    const keys = calls.map((c) => c.key);
    for (const key of ["GET /api/kegs?state=kicked", "GET /api/beers", "GET /api/brewfather", "GET /api/admins", "GET /api/tokens"]) {
      expect(keys).not.toContain(key);
    }
  });

  it.each(pages)("shows only the %s group's %s page", async (_, label, title) => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    const other = label === "Taps" ? "Keg History" : "Taps";
    if (label === "Taps") {
      goTo(other);
      await screen.findByRole("heading", { level: 2, name: other });
    }
    goTo(label);
    expect(await screen.findByRole("heading", { level: 2, name: title })).toBeTruthy();
    expect(heading(other)).toBeNull();
    expect(within(nav()).getByRole("button", { name: label }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("main")).toBeTruthy();
  });

  it.each([
    ["Taps", "Add tap"],
    ["On Deck", "Add keg"],
    ["Beer Library", "Add beer"],
    ["Admin Users", "Add admin"],
    ["API Tokens", "New token"],
  ])("shows the page title and %s's %s in the phone top bar", async (label, action) => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    if (label !== "Taps") goTo(label);
    await screen.findByRole("heading", { level: 2, name: label });
    const bar = screen.getByRole("banner");
    expect(within(bar).getByText(label)).toBeTruthy();
    expect(within(bar).getByRole("button", { name: action })).toBeTruthy();
    expect(within(screen.getByRole("main")).getByRole("button", { name: action })).toBeTruthy();
  });

  it("shows no top bar action on a page without one", async () => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("Keg History");
    await screen.findByRole("heading", { level: 2, name: "Keg History" });
    const bar = screen.getByRole("banner");
    expect(within(bar).getByText("Keg History")).toBeTruthy();
    // Only the menu button remains.
    expect(within(bar).getAllByRole("button").map((b) => b.getAttribute("aria-label"))).toEqual(["Open menu"]);
  });

  it("fetches fresh data when a page is shown again", async () => {
    const calls = stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("Keg History");
    await waitFor(() => expect(count(calls, "GET /api/kegs?state=kicked")).toBe(1));
    goTo("Taps");
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("Keg History");
    await waitFor(() => expect(count(calls, "GET /api/kegs?state=kicked")).toBe(2));
  });

  it("shows the On Deck page with its list and Add keg, in the shell's units", async () => {
    stubApi({
      ...signedIn,
      ...panels,
      "GET /api/settings": { status: 200, body: settings({ units: "metric" }) },
      "GET /api/kegs": { status: 200, body: [keg({ remaining_l: 12.5 })] },
      "GET /api/keg-types": { status: 200, body: kegTypes },
    });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("On Deck");
    expect(await screen.findByRole("heading", { level: 2, name: "On Deck" })).toBeTruthy();
    expect(await screen.findByText("12.50 L of 18.93 L")).toBeTruthy();
    expect(within(screen.getByRole("main")).getByRole("button", { name: "Add keg" })).toBeTruthy();
  });

  it("returns to sign-in on a 401 from GET /api/kegs on On Deck", async () => {
    stubApi({ ...signedIn, ...panels, "GET /api/kegs": unauthenticated });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("On Deck");
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("shows the version from the health check", async () => {
    stubApi({ ...signedIn, ...panels });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    expect(await within(nav()).findByText("Version 1.2.3")).toBeTruthy();
  });

  it("shows an unknown version when the health check fails, with no error", async () => {
    stubApi({ ...signedIn, ...panels, "GET /api/health": { status: 500, body: { error: "internal", message: "boom" } } });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    expect(await within(nav()).findByText("Version unknown")).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it.each([
    ["metric settings", { status: 200, body: settings({ units: "metric" }) }, "12.50 L of 18.93 L"],
    ["us settings", { status: 200, body: settings() }, "3.30 gal of 5.00 gal"],
    ["a failed settings read", { status: 500, body: { error: "internal", message: "boom" } }, "3.30 gal of 5.00 gal"],
  ])("shows tap volumes in the units from %s", async (_, reply, want) => {
    stubApi({
      ...signedIn,
      ...panels,
      "GET /api/taps": { status: 200, body: [tap({ keg: keg({ remaining_l: 12.5, tap_id: 1, state: "on_tap" }) })] },
      "GET /api/settings": reply,
    });
    render(<AdminPage />);
    expect(await screen.findByText(want)).toBeTruthy();
    expect(screen.getByRole("heading", { level: 2, name: "Taps" })).toBeTruthy();
  });

  it("shows US units until the settings load", async () => {
    let answer: (r: Response) => void = () => {};
    const routes: Record<string, unknown> = {
      "GET /api/setup": { needed: false },
      "GET /api/session": owner,
      "GET /api/health": { status: "ok", version: "1.2.3", database: "ok" },
      "GET /api/taps": [tap({ keg: keg({ remaining_l: 12.5, tap_id: 1, state: "on_tap" }) })],
      "GET /api/kegs?state=waiting": [],
      "GET /api/keg-types": [],
    };
    vi.stubGlobal(
      "fetch",
      vi.fn((path: string, init?: RequestInit) => {
        const key = `${init?.method ?? "GET"} ${path}`;
        if (key === "GET /api/settings") return new Promise<Response>((r) => (answer = r));
        return Promise.resolve(new Response(JSON.stringify(routes[key]), { status: 200 }));
      }),
    );
    render(<AdminPage />);
    expect(await screen.findByText("3.30 gal of 5.00 gal")).toBeTruthy();
    answer(new Response(JSON.stringify(settings({ units: "metric" })), { status: 200 }));
    expect(await screen.findByText("12.50 L of 18.93 L")).toBeTruthy();
  });

  it("follows a units change saved in Display Settings", async () => {
    stubApi({
      ...signedIn,
      ...panels,
      "GET /api/taps": { status: 200, body: [tap({ keg: keg({ remaining_l: 12.5, tap_id: 1, state: "on_tap" }) })] },
      "PATCH /api/settings": { status: 200, body: settings({ units: "metric" }) },
    });
    render(<AdminPage />);
    await screen.findByText("3.30 gal of 5.00 gal");
    goTo("Display Settings");
    const form = await screen.findByRole("form", { name: "Display settings" });
    await waitFor(() => expect((within(form).getByLabelText("Units") as HTMLSelectElement).value).toBe("us"));
    fireEvent.change(within(form).getByLabelText("Units"), { target: { value: "metric" } });
    fireEvent.click(within(form).getByRole("button", { name: "Save" }));
    await waitFor(() => expect((within(form).getByLabelText("Units") as HTMLSelectElement).value).toBe("metric"));
    goTo("Taps");
    expect(await screen.findByText("12.50 L of 18.93 L")).toBeTruthy();
  });

  it.each(["GET /api/settings", "GET /api/taps", "GET /api/kegs?state=waiting", "GET /api/keg-types"])(
    "returns to sign-in on a 401 from %s",
    async (key) => {
      stubApi({ ...signedIn, ...panels, [key]: unauthenticated });
      render(<AdminPage />);
      expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
    },
  );

  it.each([
    ["GET /api/kegs?state=kicked", "Keg History"],
    ["GET /api/beers", "Beer Library"],
    ["GET /api/admins", "Admin Users"],
    ["GET /api/tokens", "API Tokens"],
    ["GET /api/brewfather", "Brewfather"],
  ])("returns to sign-in on a 401 from %s on %s", async (key, page) => {
    stubApi({ ...signedIn, ...panels, [key]: unauthenticated });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo(page);
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("returns to sign-in when the session ended elsewhere", async () => {
    stubApi({ ...signedIn, ...panels, "DELETE /api/session": unauthenticated });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    fireEvent.click(within(nav()).getByRole("button", { name: "Sign out" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("returns to sign-in on a 401 from the password form", async () => {
    stubApi({ ...signedIn, ...panels, "POST /api/password": unauthenticated });
    render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    goTo("Change Password");
    const pw = testPassword();
    fireEvent.change(await screen.findByLabelText("Current password"), { target: { value: testPassword() } });
    fireEvent.change(screen.getByLabelText("New password"), { target: { value: pw } });
    fireEvent.change(screen.getByLabelText("Confirm new password"), { target: { value: pw } });
    fireEvent.click(within(screen.getByRole("main")).getByRole("button", { name: "Change password" }));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeTruthy();
  });

  it("uses the system theme when none is saved", async () => {
    stubApi({ ...signedIn, ...panels });
    const { container } = render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    expect(container.firstElementChild?.classList.contains("theme-system")).toBe(true);
    expect((within(nav()).getByRole("radio", { name: "System" }) as HTMLInputElement).checked).toBe(true);
  });

  it("switches to dark and saves it in this browser only", async () => {
    const calls = stubApi({ ...signedIn, ...panels });
    const { container } = render(<AdminPage />);
    await screen.findByRole("heading", { level: 2, name: "Taps" });
    await waitFor(() => expect(count(calls, "GET /api/settings")).toBe(1));
    fireEvent.click(within(nav()).getByRole("radio", { name: "Dark" }));
    const root = container.firstElementChild!;
    expect(root.classList.contains("theme-dark")).toBe(true);
    expect(root.classList.contains("theme-system")).toBe(false);
    expect(localStorage.getItem("beer-menu.admin.theme")).toBe("dark");
    expect(calls.filter((c) => c.key.includes("/api/settings")).map((c) => c.key)).toEqual(["GET /api/settings"]);
  });

  it("applies a saved light theme on load and on the sign-in screen", async () => {
    localStorage.setItem("beer-menu.admin.theme", "light");
    stubApi({ "GET /api/setup": { status: 200, body: { needed: false } }, "GET /api/session": unauthenticated });
    const { container } = render(<AdminPage />);
    expect(container.firstElementChild?.classList.contains("theme-light")).toBe(true);
    await screen.findByRole("button", { name: "Sign in" });
    expect(container.firstElementChild?.classList.contains("theme-light")).toBe(true);
    expect(screen.queryByRole("radiogroup", { name: "Theme" })).toBeNull();
  });

  it("shows an error when the API is unreachable", async () => {
    stubApi({ "GET /api/setup": { status: 502 } });
    render(<AdminPage />);
    expect((await screen.findByRole("alert")).textContent).toBe("Request failed (502).");
  });
});
