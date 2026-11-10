import { cleanup, render } from "@testing-library/react";
import { Beer } from "lucide-react";
import { afterEach, describe, expect, it } from "vitest";

afterEach(cleanup);

// Research R2: the CSP has no style-src, so an icon must not carry a style attribute.
describe("lucide-react icons", () => {
  it("render an svg with no style attribute, hidden from assistive tech", () => {
    const { container } = render(<Beer aria-hidden="true" />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg?.hasAttribute("style")).toBe(false);
    expect(svg?.getAttribute("aria-hidden")).toBe("true");
    expect(container.querySelectorAll("[style]")).toHaveLength(0);
  });
});
