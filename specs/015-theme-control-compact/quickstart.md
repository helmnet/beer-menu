# Quickstart: Compact Theme Control

## Automated

```sh
cd web
npx vitest run --maxWorkers=2 src/components/ThemeControl.test.tsx
npm run build   # runs tsc --noEmit, then vite build
```

Expected: tests pass, including each option having `title` equal to its name and radios still
named Light, Dark, System ([contract](contracts/theme-control.md)).

## Manual (real browser; jsdom cannot check layout)

Run the admin app (`docker compose up` or the Vite dev server) and sign in.

1. **Default width (256 px)**: icons and words show, no overlap (SC-002).
2. **Sweep**: drag the side panel from 400 px to 200 px and back in steps of about 10 px. At each
   step the control is either all icon + word with clear space, or all icons only; never
   overlapping, clipped, wrapped or past the border (SC-001). No flicker while holding still.
3. **Minimum (200 px)**: icons only, evenly spaced. Hover each icon: tooltip Light / Dark /
   System. Click each, then use arrow keys: theme changes, selected segment has accent fill and
   a white icon, focus outline visible (SC-003).
4. **Larger text**: at 256 px, raise the browser's default font size (e.g. to 20 px) or use
   text-only zoom: control goes icon-only rather than crowding. Page zoom (Ctrl +) scales panel
   and words together, so it keeps the words and must not crowd either.
5. **Themes**: repeat step 3 in light, dark and system theme.
6. **Phone drawer**: narrow the window under the phone breakpoint, open the drawer: control is
   clean (labels or icons, per its width); each segment is still 44 px tall. Repeat with the
   window at 320 px wide (DevTools device toolbar), the drawer's narrowest case in SC-001.
7. **Accessibility**: in DevTools accessibility tree at 200 px and 256 px, the "Theme" group
   lists three radios named Light, Dark, System (SC-004).
