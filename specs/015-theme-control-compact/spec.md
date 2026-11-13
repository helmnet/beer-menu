# Feature Specification: Compact Theme Control in a Narrow Side Panel

**Feature Branch**: `compmodder26/theme_mode_display_crowding`

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "When the side nav bar is resized too small, the theme buttons become extremely crowded. Example [screenshot: at the narrowest side panel width the Light, Dark and System labels run into each other and the System segment overflows the control]. I think beyond a certain point, the text should be hidden and just the icons shown"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Readable theme choice at any side panel width (Priority: P1)

An admin drags the side panel narrower. While there is room, the theme control at the bottom of the panel shows each option as an icon plus its word (Light, Dark, System), as today. Once the panel gets too narrow for the words to fit cleanly, the control switches to icons only (sun, moon, monitor). The three options stay evenly spaced, inside the control's border, and never overlap. When the admin widens the panel again, the words come back.

**Why this priority**: This is the whole feature. The current crowded control is hard to read and the selected segment spills past the control's edge.

**Independent Test**: Sign in as an admin, drag the side panel from its widest to its narrowest width, and check the theme control at each width: either icon plus label with no overlap, or icons only, never a crowded or overflowing mix.

**Acceptance Scenarios**:

1. **Given** the side panel is at its default width, **When** the admin looks at the theme control, **Then** each option shows its icon and its word, as before this feature.
2. **Given** the side panel is at its narrowest allowed width, **When** the admin looks at the theme control, **Then** each option shows only its icon, the three options are evenly spaced, and nothing overlaps or extends past the control's border.
3. **Given** the panel is being dragged narrower, **When** the width passes the point where the words no longer fit cleanly, **Then** all three words hide together (never just one or two) and the icons stay.
4. **Given** the control shows icons only, **When** the admin widens the panel past that point again, **Then** the words reappear.
5. **Given** the control shows icons only, **When** the admin clicks an icon or uses the keyboard to move between options, **Then** the theme changes exactly as it does with labels shown, and the selected option keeps its accent fill.

---

### User Story 2 - Icons stay identifiable without their words (Priority: P2)

When only icons show, an admin who is unsure what an icon means can hover over it (or focus it with the keyboard) to see its name. A screen reader user hears "Light", "Dark" or "System" for each option at every width.

**Why this priority**: Icons alone are usually clear (sun, moon, monitor), but the hidden words must not cost anyone the ability to tell the options apart.

**Independent Test**: With the panel at its narrowest width, hover each icon and check its name appears; use a screen reader or the accessibility tree to check each radio option still has its name.

**Acceptance Scenarios**:

1. **Given** the control shows icons only, **When** the admin hovers the pointer over an option, **Then** the option's name (Light, Dark or System) appears as a tooltip.
2. **Given** any panel width, **When** assistive technology reads the theme control, **Then** it announces a group named "Theme" with three options named Light, Dark and System, and which one is selected.

---

### Edge Cases

- Narrowest allowed side panel width (200 px): icons only, no overlap, no overflow.
- Widths near the switch point: the control must not flicker between the two looks while the width holds steady, and must never show a state where words are clipped, overlapping or wrapped onto two lines.
- Larger text (a bigger default font size or text-only zoom): the switch follows the text size, so an admin with larger text gets icons only sooner rather than crowded words. Page zoom scales the panel and the words together, so it changes neither the fit nor the look.
- Small screens, where the side panel opens as a drawer: the same rule applies to the control in the drawer.
- Light, dark and system themes: the icon-only look reads correctly and the selected option keeps its accent fill with a white icon in every theme.
- Touch screens: each icon-only option keeps the same tap target size it has today.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The theme control MUST show each option as icon plus word (Light, Dark, System) whenever the words fit on one line inside their segments without touching or overlapping each other or the control's border.
- **FR-002**: When the words do not fit as described in FR-001, the theme control MUST hide all three words and show only the three icons.
- **FR-003**: Words MUST hide and show for all three options together; the control MUST NOT mix labelled and icon-only options.
- **FR-004**: The switch between the two looks MUST follow the control's available width, both when the admin drags the side panel and when the browser window or text size changes, without a page reload.
- **FR-005**: In icon-only form, the three options MUST stay evenly spaced, centred in their segments, and fully inside the control's border at every allowed side panel width.
- **FR-006**: In icon-only form, each option MUST show its name (Light, Dark, System) as a tooltip on pointer hover.
- **FR-007**: At every width, each option MUST keep its accessible name (Light, Dark, System), and the group MUST keep its accessible name "Theme".
- **FR-008**: Selecting a theme, keyboard navigation between options, the focus outline and the selected option's accent fill MUST work the same in both looks.
- **FR-009**: Each option's click and tap target MUST be no smaller in icon-only form than it is today.
- **FR-010**: The change MUST affect only the look of the theme control in the admin side panel. The theme setting itself, where it is stored, and the TV display MUST not change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At every side panel width from the minimum (200 px) to the maximum (400 px), in steps of about 10 px, and in the phone drawer at a 320 px wide window (the drawer's narrowest common width, 85% of the window), the theme control shows no overlapping or clipped text and no option extends past the control's border.
- **SC-002**: At the default side panel width (256 px) and default text size, the control shows icons and words, unchanged from today.
- **SC-003**: At the minimum side panel width, an admin can pick each of the three themes on the first try with pointer, keyboard and touch.
- **SC-004**: An automated accessibility check finds the same three named options in the theme group at the minimum and default widths.

## Assumptions

- The switch point is decided by whether the words fit in the space the control actually has, not by a fixed side panel width, so it also handles text zoom and font changes. A fixed width threshold is acceptable only if it gives the same result at default text size.
- The tooltip is the browser's standard hover tooltip (or an equivalent); it is not shown on touch screens, where the icons plus accessible names are enough.
- The tooltip is only required in icon-only form; showing it with words visible too is allowed but not required.
- The icons stay the same (sun for Light, moon for Dark, monitor for System).
- The side panel's minimum (200 px) and maximum (400 px) widths do not change.
- No other side panel content (navigation links, version text, Sign out) is in scope.
