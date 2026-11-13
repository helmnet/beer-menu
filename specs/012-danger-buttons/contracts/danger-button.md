# UI Contract: Danger Button

## Markup

```html
<button type="button" class="btn btn-danger">Delete</button>
```

`btn-danger` is a modifier on `btn`, used the same way as `btn-primary`. It is only valid inside
the admin app (`.admin`).

## Appearance

| Property | Value |
|----------|-------|
| Fill and border | `var(--danger-fill)` = `#c0352b`, same in light and dark themes |
| Text | `#ffffff` (5.55:1 on the fill) |
| Size, padding, radius, font | Inherited from `.btn` (unchanged) |
| Hover | `filter: brightness(1.08)`, as `btn-primary` |
| Disabled | Inherited from `.btn:disabled` (opacity 0.55, not-allowed cursor) |
| Keyboard focus | Inherited from `.admin :focus-visible` (2px accent ring, 2px offset) |

## Buttons that carry it (FR-004)

| Page | Component | Button |
|------|-----------|--------|
| Beer Library | `web/src/components/BeersPanel.tsx` | Delete (each beer row) |
| Keg History | `web/src/components/KegHistory.tsx` | Delete (each keg row) |
| Admin Users | `web/src/components/AdminsPanel.tsx` | Remove (each row) |
| API Tokens | `web/src/components/TokensPanel.tsx` | Revoke (each row) |
| Two-Factor Sign-In | `web/src/components/TwoFactorPanel.tsx` | Turn off 2FA |
| Two-Factor Sign-In | `web/src/components/TwoFactorPanel.tsx` | Remove (authenticator app) |
| Two-Factor Sign-In | `web/src/components/TwoFactorPanel.tsx` | Remove (each passkey) |
| Two-Factor Sign-In | `web/src/components/TwoFactorPanel.tsx` | New recovery codes |
| Brewfather | `web/src/components/BrewfatherPanel.tsx` | Disconnect |

No other button carries `btn-danger`. Labels, handlers and confirm prompts are unchanged.
