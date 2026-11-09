// Outline and part colors read on both light and dark backgrounds. Colors are SVG
// presentation attributes, never style text, so the nginx CSP needs no style-src.
const outline = "#8a8a8a";
const rubber = "#2e2e2e";
const steel = "#a7adb3";
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
 * BeerArt draws a beer as original inline SVG: a shaker pint glass when `onTap` (a keg of the
 * beer is on a tap), otherwise a keg in `kegShape`, the corny keg by default or the sanke keg
 * for sanke and euro keg types. The liquid takes `color`; with `color` null the vessel is drawn
 * empty. It makes no network request and scales by its viewBox, so `height` alone sizes it for
 * the admin list or the TV. The root has role img, a label naming the beer and vessel, and for
 * a keg `data-keg-shape`.
 */
export default function BeerArt({
  onTap = false,
  kegShape = "corny",
  color,
  name,
  height = 64,
}: {
  onTap?: boolean;
  kegShape?: "corny" | "sanke";
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
      data-keg-shape={onTap ? undefined : kegShape}
    >
      {onTap ? <Glass color={color} /> : kegShape === "sanke" ? <SankeKeg color={color} /> : <Keg color={color} />}
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

/**
 * SankeKeg is a straight-sided steel sanke keg seen from the side: top and bottom chimes with
 * rounded hand-hold cutouts, two rolling rings around the body, and the spear fitting showing
 * above the top chime. It is wider and squatter than the corny keg in the same viewBox. The
 * whole body is the liquid, like a full keg.
 */
function SankeKeg({ color }: { color: string | null }) {
  return (
    <g strokeLinejoin="round">
      <rect data-part="liquid" x={10} y={19} width={40} height={64} rx={2} fill={color ?? "none"} stroke={outline} strokeWidth={1.5} />
      <path d="M14 26 V76" stroke="#ffffff" strokeOpacity={0.45} strokeWidth={1.5} strokeLinecap="round" />
      <rect x={8} y={36} width={44} height={4} rx={2} fill={steel} stroke={outline} strokeWidth={1} />
      <rect x={8} y={62} width={44} height={4} rx={2} fill={steel} stroke={outline} strokeWidth={1} />
      <rect x={27} y={2} width={6} height={5} rx={1} fill={steel} stroke={outline} strokeWidth={1} />
      <path
        d="M10 8 Q10 6 12 6 H48 Q50 6 50 8 V21 H10 Z M15 13 Q15 10 18 10 H24 Q27 10 27 13 Q27 16 24 16 H18 Q15 16 15 13 Z M33 13 Q33 10 36 10 H42 Q45 10 45 13 Q45 16 42 16 H36 Q33 16 33 13 Z"
        fill={steel}
        fillRule="evenodd"
        stroke={outline}
        strokeWidth={1}
      />
      <path
        d="M10 81 H50 V92 Q50 94 48 94 H12 Q10 94 10 92 Z M16 87 Q16 85 18 85 H24 Q26 85 26 87 Q26 89 24 89 H18 Q16 89 16 87 Z M34 87 Q34 85 36 85 H42 Q44 85 44 87 Q44 89 42 89 H36 Q34 89 34 87 Z"
        fill={steel}
        fillRule="evenodd"
        stroke={outline}
        strokeWidth={1}
      />
    </g>
  );
}
