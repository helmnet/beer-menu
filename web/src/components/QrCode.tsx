import { useMemo } from "react";
import { encode } from "uqr";

/**
 * QrCode draws `value` as a QR code: an SVG with `role="img"` named by `label`, whose dark
 * modules are one `<path>` (research R4). It uses SVG elements only, with no style attribute and
 * no innerHTML, so the CSP stays unchanged; the size and the light background come from the
 * `qr-code` CSS class.
 */
export default function QrCode({ value, label }: { value: string; label: string }) {
  const { size, d } = useMemo(() => {
    // A 4-module quiet zone, as the QR standard asks, so phone cameras find the code.
    const qr = encode(value, { border: 4 });
    let path = "";
    qr.data.forEach((row, y) =>
      row.forEach((dark, x) => {
        if (dark) path += `M${x} ${y}h1v1h-1z`;
      }),
    );
    return { size: qr.size, d: path };
  }, [value]);

  return (
    <svg className="qr-code" role="img" aria-label={label} viewBox={`0 0 ${size} ${size}`} shapeRendering="crispEdges">
      <path d={d} fill="#000" />
    </svg>
  );
}
