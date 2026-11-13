import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import QrCode from "./QrCode";

afterEach(cleanup);

describe("QrCode", () => {
  it("draws one SVG path with an accessible name and no style attribute", () => {
    const { container } = render(
      <QrCode value="otpauth://totp/Beer%20Menu:owner?secret=ABCD&issuer=Beer%20Menu" label="Setup QR code" />,
    );
    const img = screen.getByRole("img", { name: "Setup QR code" });
    expect(container.querySelectorAll("svg")).toHaveLength(1);
    expect(img.tagName.toLowerCase()).toBe("svg");
    const paths = container.querySelectorAll("path");
    expect(paths).toHaveLength(1);
    expect(paths[0].getAttribute("d")).toMatch(/^M\d+ \d+h1v1h-1z/);
    expect(container.querySelector("[style]")).toBeNull();
  });
});
