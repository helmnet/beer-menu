# UI Contract: Theme Control

## Component interface (unchanged)

`web/src/components/ThemeControl.tsx` default export `ThemeControl({ theme, onChange })`.
`theme` is `"light" | "dark" | "system"`; choosing another option calls `onChange(value)`.

## Rendered structure

```text
<fieldset class="theme-control" role="radiogroup">
  <legend class="sr-only">Theme</legend>
  <label class="theme-option[ selected]" title="Light">     title = option name (new)
    <input type="radio" name=… value="light">
    <svg aria-hidden="true">                                 Sun / Moon / Monitor
    <span class="theme-label">Light</span>                   class is new
  </label>
  … Dark, System
</fieldset>
```

## Behavior

| Condition | Result |
|-----------|--------|
| Any width | Group name "Theme"; radios named Light, Dark, System; one checked |
| Any width | Hovering an option shows its name as a native tooltip |
| Bottom block ≥ threshold | Icon and word shown, words on one line |
| Bottom block < threshold | All three words visually hidden together; icons centred; names kept |
| Selected option | Accent fill, white icon (and word), in both looks |
| Keyboard focus | Focus outline on the segment, in both looks |

## Styling hooks (`web/src/pages/AdminPage.css`)

- `.admin .nav-bottom` is a size container (`container-type: inline-size`).
- One `@container (width < 14.4rem)` rule visually hides `.theme-control .theme-label`.
- `.theme-label` has `white-space: nowrap` so a word never wraps to a second line.
