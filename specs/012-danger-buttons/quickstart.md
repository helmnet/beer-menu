# Quickstart: Validate Danger Buttons

## Automated

```sh
cd web
npx vitest run --maxWorkers=2
npx tsc --noEmit
```

Expected: all tests pass, including the new `btn-danger` class assertions in `BeersPanel`,
`KegHistory`, `AdminsPanel`, `TokensPanel`, `TwoFactorPanel` and `BrewfatherPanel` tests.

## Visual check

1. Start the app (`docker compose up`, or `npm run dev` in `web/` against a running API).
2. Sign in as an admin. Make sure there are at least one beer, one kicked keg, two admins, one API token, 2FA
   on with an authenticator app and a passkey, and Brewfather connected.
3. For each theme (Light, Dark, and System with the device in dark mode), open each page in
   [contracts/danger-button.md](./contracts/danger-button.md) and check:
   - Each listed button is red-filled with white text, the same size as before.
   - Hover brightens it slightly; Tab focus shows the blue ring; disabled (while a request
     runs) dims it.
   - No other button changed (Edit, Archive, Rename, Add beer and so on).
4. Repeat at phone width: red buttons in table rows wrap and size as before.
5. In Dark, dev tools show a danger button's computed background `rgb(192, 53, 43)` and text
   `rgb(255, 255, 255)`.
6. Click each red button and cancel the confirm prompt: nothing is deleted or changed.
