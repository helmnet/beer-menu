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
 *
 * `level` (0 to 1, optional) fills the liquid to that fraction from the bottom: any level above
 * 0 shows at least a 2% sliver, 0 draws no liquid and no foam, and the label gains ", N% left"
 * (N rounded, but at least 1 above 0 and at most 99 below 1, so the label never says empty or
 * full when it is not). A keg keeps its full body outline above the liquid. The shapes are
 * computed, with no clip path, ids or style, so many drawings share a page under the CSP.
 * Without `level` the drawing is full, as before.
 */
export default function BeerArt({
  onTap = false,
  kegShape = "corny",
  color,
  name,
  height = 64,
  level,
}: {
  onTap?: boolean;
  kegShape?: "corny" | "sanke";
  color: string | null;
  name: string;
  height?: number;
  level?: number;
}) {
  const vessel = onTap ? "pint glass" : "keg";
  let label = `${name}, ${vessel}`;
  let fill: number | undefined;
  if (level !== undefined) {
    const l = Math.min(Math.max(level, 0), 1);
    fill = l > 0 ? Math.max(l, 0.02) : 0;
    let pct = Math.round(l * 100);
    if (l > 0) pct = Math.max(pct, 1);
    if (l < 1) pct = Math.min(pct, 99);
    label += `, ${pct}% left`;
  }
  return (
    <svg
      viewBox="0 0 60 100"
      height={height}
      width={height * 0.6}
      role="img"
      aria-label={label}
      data-vessel={onTap ? "glass" : "keg"}
      data-keg-shape={onTap ? undefined : kegShape}
    >
      {onTap ? (
        <Glass color={color} fill={fill ?? 1} />
      ) : kegShape === "sanke" ? (
        <SankeKeg color={color} fill={fill} />
      ) : (
        <Keg color={color} fill={fill} />
      )}
    </svg>
  );
}

// One decimal is finer than a TV pixel and keeps the full glass path text as it always was.
const r = (v: number) => Math.round(v * 10) / 10;

// The glass walls run from (11.3, 22) to (16.8, 90) and (48.7, 22) to (43.2, 90).
const wallLeft = (y: number) => r(11.3 + ((y - 22) * 5.5) / 68);
const wallRight = (y: number) => r(48.7 - ((y - 22) * 5.5) / 68);

/**
 * Glass is a shaker pint: straight walls tapering to a narrower, thicker base. The liquid
 * fills `fill` (0 to 1) of the glass from the base, with a foam band on top when there is any:
 * 10 units tall, or half the liquid's height when that is less.
 */
function Glass({ color, fill }: { color: string | null; fill: number }) {
  const top = r(90 - fill * 68);
  // Foam at most half the beer's height, so a nearly empty glass looks nearly empty.
  const head = r(top - Math.min(10, fill * 34));
  return (
    <g strokeLinejoin="round">
      {fill > 0 && (
        <path
          data-part="liquid"
          d={`M${wallLeft(top)} ${top} L16.8 90 L43.2 90 L${wallRight(top)} ${top} Z`}
          fill={color ?? "none"}
        />
      )}
      {color && fill > 0 && (
        <path
          className="foam"
          d={`M${wallLeft(head)} ${head} L${wallLeft(top)} ${top} L${wallRight(top)} ${top} L${wallRight(head)} ${head} Z`}
          fill={foam}
        />
      )}
      <path d="M8 6 L15 94 L45 94 L52 6" fill="none" stroke={outline} strokeWidth={2} />
      <path d="M15.6 90 L44.4 90" stroke={outline} strokeWidth={1.5} />
      <path d="M14 28 L17.5 82" stroke="#ffffff" strokeOpacity={0.45} strokeWidth={1.5} strokeLinecap="round" />
    </g>
  );
}

/**
 * KegBody is a keg's body rect. Without `fill` the whole body is the liquid, like a full keg;
 * with `fill` (0 to 1) a square-cornered liquid rect sits on the body bottom under the unfilled
 * full-size outline. Both kegs hide 2 units at each end of the body under their top and bottom
 * parts, so `fill` spans the visible body between them: a sliver shows and only 1 reaches the top.
 */
function KegBody({
  x,
  y,
  width,
  height,
  rx,
  color,
  fill,
}: {
  x: number;
  y: number;
  width: number;
  height: number;
  rx: number;
  color: string | null;
  fill?: number;
}) {
  if (fill === undefined) {
    return (
      <rect data-part="liquid" x={x} y={y} width={width} height={height} rx={rx} fill={color ?? "none"} stroke={outline} strokeWidth={1.5} />
    );
  }
  const h = fill > 0 ? r(2 + fill * (height - 4)) : 0;
  return (
    <>
      {h > 0 && <rect data-part="liquid" x={x} y={r(y + height - h)} width={width} height={h} fill={color ?? "none"} />}
      <rect x={x} y={y} width={width} height={height} rx={rx} fill="none" stroke={outline} strokeWidth={1.5} />
    </>
  );
}

/**
 * Keg is a corny keg seen from the side: a rubber top ring with two handle cutouts, a rubber
 * foot, and between them a body with weld seams. The body holds the liquid (see KegBody).
 */
function Keg({ color, fill }: { color: string | null; fill?: number }) {
  return (
    <g strokeLinejoin="round">
      <KegBody x={13} y={20} width={34} height={66} rx={3} color={color} fill={fill} />
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
 * body holds the liquid (see KegBody).
 */
function SankeKeg({ color, fill }: { color: string | null; fill?: number }) {
  return (
    <g strokeLinejoin="round">
      <KegBody x={10} y={19} width={40} height={64} rx={2} color={color} fill={fill} />
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
