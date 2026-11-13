import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import SideNav, { GroupKey, groupOf, isPageKey, PageKey } from "./SideNav";

afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.restoreAllMocks();
});

/** Shell holds page and group state the way AdminPage does, inside a `.admin` root. */
function Shell({
  start = "taps",
  version = "1.2.3",
  onSelect = () => {},
  onSignOut = () => {},
}: {
  start?: PageKey;
  version?: string;
  onSelect?: (p: PageKey) => void;
  onSignOut?: () => void;
}) {
  const [page, setPage] = useState<PageKey>(start);
  const [openGroup, setOpenGroup] = useState<GroupKey | null>(groupOf(start));
  return (
    <div className="admin" data-testid="root">
      <SideNav
        page={page}
        openGroup={openGroup}
        onGroup={(g) => setOpenGroup(openGroup === g ? null : g)}
        onSelect={(p) => {
          onSelect(p);
          setPage(p);
          setOpenGroup(groupOf(p));
        }}
        version={version}
        onSignOut={onSignOut}
      />
    </div>
  );
}

const nav = () => screen.getByRole("navigation", { name: "Admin" });
const navButtons = () =>
  within(nav())
    .getAllByRole("button")
    .map((b) => b.textContent);

describe("SideNav", () => {
  it("shows the three groups and only the open group's pages", () => {
    render(<Shell />);
    expect(navButtons()).toEqual([
      "Menu",
      "Taps",
      "On Deck",
      "Keg History",
      "Beer Library",
      "Display Settings",
      "Administration",
      "Integrations",
      "Sign out",
    ]);
    const menu = within(nav()).getByRole("button", { name: "Menu" });
    expect(menu.getAttribute("aria-expanded")).toBe("true");
    expect(document.getElementById(menu.getAttribute("aria-controls") ?? "")).not.toBeNull();
    expect(within(nav()).getByRole("button", { name: "Administration" }).getAttribute("aria-expanded")).toBe("false");
  });

  it("opens one group at a time", () => {
    render(<Shell />);
    fireEvent.click(within(nav()).getByRole("button", { name: "Administration" }));
    expect(within(nav()).getByRole("button", { name: "Administration" }).getAttribute("aria-expanded")).toBe("true");
    expect(within(nav()).getByRole("button", { name: "Menu" }).getAttribute("aria-expanded")).toBe("false");
    expect(within(nav()).queryByRole("button", { name: "Taps" })).toBeNull();
    expect(within(nav()).getByRole("button", { name: "API Tokens" })).toBeTruthy();
    expect(
      within(document.getElementById(within(nav()).getByRole("button", { name: "Administration" }).getAttribute("aria-controls")!)!)
        .getAllByRole("button")
        .map((b) => b.textContent),
    ).toEqual(["Admin Users", "API Tokens", "Change Password", "Two-Factor Sign-In"]);
    fireEvent.click(within(nav()).getByRole("button", { name: "Integrations" }));
    expect(within(nav()).getByRole("button", { name: "Brewfather" })).toBeTruthy();
    expect(within(nav()).queryByRole("button", { name: "API Tokens" })).toBeNull();
  });

  it("selects a page and marks it current", () => {
    const onSelect = vi.fn();
    render(<Shell onSelect={onSelect} />);
    expect(within(nav()).getByRole("button", { name: "Taps" }).getAttribute("aria-current")).toBe("page");
    fireEvent.click(within(nav()).getByRole("button", { name: "Administration" }));
    fireEvent.click(within(nav()).getByRole("button", { name: "Admin Users" }));
    expect(onSelect).toHaveBeenCalledWith("admins");
    expect(within(nav()).getByRole("button", { name: "Admin Users" }).getAttribute("aria-current")).toBe("page");
  });

  it("shows the version and Sign out", () => {
    const onSignOut = vi.fn();
    render(<Shell onSignOut={onSignOut} />);
    expect(within(nav()).getByText("Version 1.2.3")).toBeTruthy();
    fireEvent.click(within(nav()).getByRole("button", { name: "Sign out" }));
    expect(onSignOut).toHaveBeenCalled();
  });

  it("shows an unknown version when none is given", () => {
    render(<Shell version="" />);
    expect(within(nav()).getByText("Version unknown")).toBeTruthy();
  });

  it("resizes with the arrow keys, saves the width and writes it on the .admin root only", () => {
    render(<Shell />);
    const root = screen.getByTestId("root");
    const handle = screen.getByRole("separator", { name: "Resize side panel" });
    expect(handle.getAttribute("aria-orientation")).toBe("vertical");
    expect(handle.getAttribute("aria-valuenow")).toBe("256");
    expect(handle.getAttribute("aria-valuemin")).toBe("200");
    expect(handle.getAttribute("aria-valuemax")).toBe("400");
    expect(root.style.getPropertyValue("--nav-width")).toBe("256px");

    fireEvent.keyDown(handle, { key: "ArrowRight" });
    expect(handle.getAttribute("aria-valuenow")).toBe("272");
    expect(root.style.getPropertyValue("--nav-width")).toBe("272px");
    expect(localStorage.getItem("beer-menu.admin.nav-width")).toBe("272");

    fireEvent.keyDown(handle, { key: "ArrowLeft" });
    fireEvent.keyDown(handle, { key: "ArrowLeft" });
    expect(handle.getAttribute("aria-valuenow")).toBe("240");
    expect(localStorage.getItem("beer-menu.admin.nav-width")).toBe("240");

    const styled = [...document.querySelectorAll("[style]")];
    expect(styled).toEqual([root]);
    expect([...root.style].filter((p) => p !== "--nav-width")).toEqual([]);
  });

  it("keeps the width within its limits", () => {
    localStorage.setItem("beer-menu.admin.nav-width", "400");
    render(<Shell />);
    const handle = screen.getByRole("separator", { name: "Resize side panel" });
    fireEvent.keyDown(handle, { key: "ArrowRight" });
    expect(handle.getAttribute("aria-valuenow")).toBe("400");
  });

  it("starts from the saved width", () => {
    localStorage.setItem("beer-menu.admin.nav-width", "320");
    render(<Shell />);
    expect(screen.getByTestId("root").style.getPropertyValue("--nav-width")).toBe("320px");
  });
});

describe("SideNav phone drawer", () => {
  const menuButton = () => screen.getByRole("button", { name: /^(Open|Close) menu$/ });
  // The backdrop is decorative (aria-hidden) and has no role, so it is found by its class.
  const backdrop = () => document.querySelector(".nav-backdrop");

  it("has a menu button that controls the navigation", () => {
    render(<Shell />);
    const button = screen.getByRole("button", { name: "Open menu" });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    expect(button.getAttribute("aria-controls")).toBe(nav().id);
    expect(backdrop()).toBeNull();
  });

  it("shows the current page title in the top bar", () => {
    render(<Shell start="tokens" />);
    expect(within(screen.getByRole("banner")).getByText("API Tokens")).toBeTruthy();
  });

  it("knows the two-factor page", () => {
    expect(isPageKey("two-factor")).toBe(true);
    expect(groupOf("two-factor")).toBe("administration");
    render(<Shell start="two-factor" />);
    expect(within(screen.getByRole("banner")).getByText("Two-Factor Sign-In")).toBeTruthy();
  });

  it("opens, moves focus to the current page and closes on selecting a page", () => {
    const onSelect = vi.fn();
    render(<Shell onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    expect(menuButton().getAttribute("aria-expanded")).toBe("true");
    expect(menuButton().getAttribute("aria-label")).toBe("Close menu");
    expect(document.activeElement).toBe(within(nav()).getByRole("button", { name: "Taps" }));
    expect(backdrop()?.getAttribute("aria-hidden")).toBe("true");

    fireEvent.click(within(nav()).getByRole("button", { name: "On Deck" }));
    expect(onSelect).toHaveBeenCalledWith("on-deck");
    expect(menuButton().getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Open menu" }));
  });

  it("focuses the first group when the current page's group is collapsed", () => {
    render(<Shell />);
    fireEvent.click(within(nav()).getByRole("button", { name: "Menu" }));
    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    expect(document.activeElement).toBe(within(nav()).getByRole("button", { name: "Menu" }));
  });

  it("stays open when a group is opened", () => {
    render(<Shell />);
    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    fireEvent.click(within(nav()).getByRole("button", { name: "Administration" }));
    expect(menuButton().getAttribute("aria-expanded")).toBe("true");
  });

  it("closes on the menu button, the backdrop or Escape, keeping the page", () => {
    const onSelect = vi.fn();
    render(<Shell onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    fireEvent.click(screen.getByRole("button", { name: "Close menu" }));
    expect(menuButton().getAttribute("aria-expanded")).toBe("false");

    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    fireEvent.click(backdrop()!);
    expect(menuButton().getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(menuButton());

    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    fireEvent.keyDown(document.activeElement!, { key: "Escape" });
    expect(menuButton().getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(menuButton());
    expect(onSelect).not.toHaveBeenCalled();
    expect(within(nav()).getByRole("button", { name: "Taps" }).getAttribute("aria-current")).toBe("page");
  });

  it("ignores Escape while closed", () => {
    render(<Shell />);
    const taps = within(nav()).getByRole("button", { name: "Taps" });
    taps.focus();
    fireEvent.keyDown(taps, { key: "Escape" });
    expect(document.activeElement).toBe(taps);
    expect(menuButton().getAttribute("aria-expanded")).toBe("false");
  });

  it("offers a slot in the top bar for the page's primary action", () => {
    const slot = vi.fn();
    render(
      <div className="admin">
        <SideNav
          page="taps"
          openGroup="menu"
          onGroup={() => {}}
          onSelect={() => {}}
          version="1"
          onSignOut={() => {}}
          actionSlot={slot}
        />
      </div>,
    );
    const el = slot.mock.calls.find(([e]) => e)?.[0] as HTMLElement;
    expect(within(screen.getByRole("banner")).getByText("Taps")).toBeTruthy();
    expect(screen.getByRole("banner").contains(el)).toBe(true);
  });
});
