# Research: Danger Button Styling

## R1. Which red, and which text color

**Decision**: Add one token, `--danger-fill: #c0352b`, set once on `.admin` and not overridden by
the dark tokens. Danger buttons use it for fill and border, with white text, in both themes.

**Rationale**: This mirrors `--accent`, which also keeps one value in both themes. Contrast
(WCAG relative luminance):

| Fill | Text | Ratio |
|------|------|-------|
| `#c0352b` | `#ffffff` | 5.55:1 (passes 4.5:1) |
| `#e5534b` (dark `--danger`) | `#ffffff` | 3.70:1 (fails) |
| `#e5534b` | `#0b0c0f` | 5.28:1 (passes, but dark text on red) |
| `#4169e1` (accent, for reference) | `#ffffff` | 4.85:1 |

One fill with white text matches the primary button (white on blue) and passes in both themes.

**Alternatives considered**:

- Reuse `--danger`: its dark value (`#e5534b`) is tuned for red *text* on a dark background
  (4.74:1 on `--surface`). White on it fails at 3.70:1. Darkening it would break error text in
  dark mode.
- Dark text on `#e5534b` in dark mode only: passes, but the button would read differently from
  the primary button across themes, and needs a second set of overrides in two dark blocks.

## R2. How the style is applied

**Decision**: A modifier class `btn-danger` next to `btn`, the same way `btn-primary` works:
`className="btn btn-danger"`. Its rule copies `.admin .btn-primary` and `.admin .btn-primary:hover`
with `--danger-fill` in place of `--accent`.

**Rationale**: Reuses the existing button system. Size, padding, radius, font, disabled
(`.admin .btn:disabled`) and keyboard focus (`.admin :focus-visible`, accent ring) already come
from shared rules, so FR-001 and FR-003 hold with no extra CSS.

**Alternatives considered**: A shared React `Button` component with a `variant` prop. Rejected:
the app uses plain `<button className="btn ...">` everywhere; a component for one class adds code
and churn (Principle VII).

## R3. Focus ring color

**Decision**: Keep the shared accent-blue focus ring on danger buttons.

**Rationale**: FR-003 asks for focus feedback consistent with the primary button, which uses the
same ring. The 2px offset puts the ring on the page background, not on the red fill, so its
contrast is unchanged.

## R4. How it is tested

**Decision**: In each affected component's existing test (the five panels plus `KegHistory`), assert that the listed buttons
have the `btn-danger` class, and that one neighbouring non-danger button (for example Edit or
Rename) does not.

**Rationale**: Guards FR-004 and FR-006 (right buttons, only those). Color and contrast are CSS
and cannot be checked by jsdom; R1's table records the computed ratios, and quickstart covers a
visual check in both themes.
