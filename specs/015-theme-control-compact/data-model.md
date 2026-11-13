# Data Model: Compact Theme Control

No data changes. The theme preference (`beer-menu.admin.theme`: light, dark or system) and the
side panel width (`beer-menu.admin.nav-width`, 200–400 px) in `web/src/prefs.ts` keep their keys,
values and defaults. No API, database or migration change.

The only state is visual and derived by the browser, not stored:

| Look | When | Shows |
|------|------|-------|
| Labelled | `.nav-bottom` width ≥ threshold (R2) | icon + word per option |
| Icon-only | `.nav-bottom` width < threshold | icon per option; words visually hidden |

Transitions happen on any width or text-size change, in both directions, with no reload.
