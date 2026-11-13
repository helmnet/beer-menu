# Research: Compact Theme Control

## R1. How to detect "the words no longer fit"

- **Decision**: make `.admin .nav-bottom` a size container (`container-type: inline-size`) and
  add an `@container (width < 14.4rem)` rule that hides the words.
- **Rationale**: the control's width follows the side panel and the phone drawer, not the
  viewport, so a media query cannot see it. A container query reacts to the panel drag, window
  resize and a larger default font size with no JavaScript (FR-004). The threshold is in `rem`,
  the unit of the words' font size (`0.85rem`), so a larger default font raises the pixel
  threshold and the control goes icon-only sooner (spec edge case). Not `em`: in a container
  condition `em` resolves against `.nav-bottom`'s font size, fixed at 15 px by `.admin`, so it
  would not follow the default font size. Browser page zoom scales panel and words together, so
  it changes neither the fit nor the look. Container queries already ship in this
  app (`DisplayPage.css`), so browser support is proven here.
- **Container choice**: `.nav-bottom`, not the `fieldset` itself. A container cannot query its
  own size, only its descendants can, and the rule must style the fieldset's children; putting
  it on the parent also avoids `fieldset` layout quirks. `.nav-bottom` is a full-width grid
  column, so its inline size is the control's width. `inline-size` containment only drops
  intrinsic width, which the block does not depend on (it fills the nav column).
- **Alternatives considered**:
  - ResizeObserver + measuring `scrollWidth` in React: exact, but adds state, an effect and a
    test harness for layout jsdom cannot do. Rejected (VII).
  - Fixed threshold on the stored nav width (`--nav-width` < N px): misses the phone drawer and
    text zoom. Rejected.
  - Shrinking font or wrapping words: still crowded at 200 px; spec asks for icons only.

## R2. Threshold value

- **Decision**: the threshold is the smallest `.nav-bottom` width at which all three segments
  show icon + word on one line with clear space, measured in a browser at default text size,
  then written in `rem` (px ÷ 16, not rounded: 14.5rem = 232 px would hide the words at the
  default 231 px block). Measured: 230 px, so `14.4rem` (230.4 px). Words wrap is forbidden by
  `white-space: nowrap` on the word so a too-small threshold shows as overflow, not as a
  two-line label, and is caught in the quickstart sweep.
- **Estimate** (before measuring): the widest segment is "System": 4 px padding ×2 +
  16 px icon + 4 px gap + ≈48 px word ≈ 76 px; × 3 columns + 2 × 2 px gaps + 4 px padding + 2 px
  border ≈ 238 px of control, i.e. about 15 rem. The default panel (256 px) gives a 231 px block
  (256 − 1 px border − 2 × 12 px padding), so SC-002 (words at default width) had to be checked.
- **Measured** (tasks.md T001): with the 4 px icon-word gap the words need 236 px, more than the
  default 231 px block. The gap became 2 px (segment padding stays 4 px), so the words need
  230 px and show at 256 px. The measured number is in the CSS comment.
- **Rationale**: a measured number avoids both the screenshot's crowding and needless hiding.

## R3. Hiding the words without losing accessible names

- **Decision**: wrap each word in `<span class="theme-label">` and, inside the container rule,
  hide it with the visually-hidden pattern already used by `.sr-only` (1 px clip, absolute
  position), not `display: none`.
- **Rationale**: the radio's accessible name comes from its `<label>` text. `display: none` text
  is dropped from the name; clipped text is kept (FR-007). The group name "Theme" comes from the
  existing `.sr-only` legend and is unchanged.
- **Reuse**: the `.admin .sr-only` rule (`web/src/pages/AdminPage.css`, line ~116) cannot be
  shared: a selector inside an `@container` block cannot join a selector list outside it, and
  `.sr-only` must stay unconditional. So the container rule repeats the same declarations, with a
  one-line comment pointing at `.sr-only`. This is the only repeat in the feature.

## R4. Tooltip

- **Decision**: put `title={o.label}` on each option's `<label>`, at all widths.
- **Rationale**: native tooltip, no component, works for pointer hover (FR-006). Spec allows it
  with words visible. The `title` equals the accessible name, so screen readers do not announce
  a different description (browsers drop a description identical to the name in most readers;
  harmless otherwise). Keyboard focus does not show native tooltips; spec only requires hover,
  and focused radios still announce their name.
- **Alternatives considered**: a custom tooltip component (more code, focus handling) rejected
  (VII); `title` only inside the compact state is impossible from CSS and needless.

## R5. Layout in icon-only form

- **Decision**: no change to the grid (`repeat(3, minmax(0, 1fr))`) or `min-height`; with the
  word hidden only the centred icon remains, and the hidden span is absolutely positioned so it
  takes no space and the `gap` between icon and word has no visible effect.
- **Rationale**: keeps even spacing and today's tap target size (FR-005, FR-009), including the
  44 px touch rule.
- **No flicker near the switch point**: the hidden word is absolutely positioned and the
  container is `.nav-bottom`, whose width comes from the side panel, not from its content. Hiding
  or showing the word never changes the width the query tests, so the look cannot oscillate
  while the width holds steady (spec edge case).
