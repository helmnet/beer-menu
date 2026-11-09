// Outline and part colors read on both light and dark backgrounds. Colors are SVG
// presentation attributes, never style text, so the nginx CSP needs no style-src.
const outline = "#8a8a8a";
const rubber = "#2e2e2e";
const foam = "#f3ead7";

/**
 * colorForSrm returns the display color for srm from `colors` (the API's table, where
 * colors[i] is SRM i+1): srm rounded to the nearest integer, halves up, clamped to 1..40. This
 * must match ColorFor in api/internal/beers/srm.go; both are tested with the same cases. It
 * returns null without an SRM or before the table has loaded.
 */
export function colorForSrm(colors: string[], srm: number | null): string | null {
  if (srm === null) return null;
  const i = Math.min(Math.max(Math.round(srm), 1), 40);
  return colors[i - 1] ?? null;
}

/**
 * BeerArt draws a beer as original inline SVG: a shaker pint glass when `onTap`, a 5 gallon
 * corny keg otherwise. Taps arrive in feature 4, so until then no caller sets `onTap` and every
 * beer is a keg. The liquid takes `color`; with `color` null the vessel is drawn empty. It makes
 * no network request and scales by its viewBox, so `height` alone sizes it for the admin list or
 * the TV. The root has role img and a label naming the beer and vessel.
 */
export default function BeerArt({
  onTap = false,
  color,
  name,
  height = 64,
}: {
  onTap?: boolean;
  color: string | null;
  name: string;
  height?: number;
}) {
  return (
    <svg
      viewBox="0 0 60 100"
      height={height}
      width={height * 0.6}
      role="img"
      aria-label={`${name}, ${onTap ? "pint glass" : "keg"}`}
      data-vessel={onTap ? "glass" : "keg"}
    >
      {onTap ? <Glass color={color} /> : <Keg color={color} />}
    </svg>
  );
}

/** Glass is a shaker pint: straight walls tapering to a narrower, thicker base. */
function Glass({ color }: { color: string | null }) {
  return (
    <g strokeLinejoin="round">
      <path data-part="liquid" d="M11.3 22 L16.8 90 L43.2 90 L48.7 22 Z" fill={color ?? "none"} />
      {color && <path d="M10.5 12 L11.3 22 L48.7 22 L49.5 12 Z" fill={foam} />}
      <path d="M8 6 L15 94 L45 94 L52 6" fill="none" stroke={outline} strokeWidth={2} />
      <path d="M15.6 90 L44.4 90" stroke={outline} strokeWidth={1.5} />
      <path d="M14 28 L17.5 82" stroke="#ffffff" strokeOpacity={0.45} strokeWidth={1.5} strokeLinecap="round" />
    </g>
  );
}

/**
 * Keg is a corny keg seen from the side: a rubber top ring with two handle cutouts, a rubber
 * foot, and between them a body with weld seams. The whole body is the liquid, like a full keg.
 */
function Keg({ color }: { color: string | null }) {
  return (
    <g strokeLinejoin="round">
      <rect data-part="liquid" x={13} y={20} width={34} height={66} rx={3} fill={color ?? "none"} stroke={outline} strokeWidth={1.5} />
      {/* Translucent gray seams read on pale and near-black beer alike. */}
      <path d="M13 30 H47 M13 76 H47" stroke={outline} strokeOpacity={0.7} strokeWidth={1} />
      <path d="M17 26 V80" stroke="#ffffff" strokeOpacity={0.45} strokeWidth={1.5} strokeLinecap="round" />
      <path
        d="M12 9 Q12 6 15 6 H45 Q48 6 48 9 V22 H12 Z M17 10 H26 V15 H17 Z M34 10 H43 V15 H34 Z"
        fill={rubber}
        fillRule="evenodd"
        stroke={outline}
        strokeWidth={1}
      />
      <rect x={11} y={84} width={38} height={10} rx={3} fill={rubber} stroke={outline} strokeWidth={1} />
    </g>
  );
}
