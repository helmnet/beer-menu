# Quickstart: Two-Factor Sign-In for Admins

Validation guide for feature 11. Routes and schemas: [contracts/two-factor.openapi.yaml](./contracts/two-factor.openapi.yaml).
Tables and states: [data-model.md](./data-model.md). Design choices: [research.md](./research.md).

## Prerequisites

- The Docker Compose stack behind NPM, reached at an HTTPS host name, with
  `BEER_MENU_SECRET_KEY` (or `_FILE`) set and `BEER_MENU_PUBLIC_URL` set to that origin.
- Two admins, `alice` and `bob`, and a `write` API token `$TOKEN`.
- An authenticator app (any) and one device with a fingerprint, face unlock or security key.
- For scenario 7, a second deployment (or restart) with `BEER_MENU_PUBLIC_URL` unset.

## 1. Automated checks

```sh
cd api && go test -race ./... && govulncheck ./...
cd web && npx vitest run --maxWorkers=2 && npm run lint && npm run build
npx @redocly/cli lint api/openapi.yaml
```

Expected: all pass. Coverage that matters: TOTP RFC 6238 vectors, ±1 step, replay refusal;
recovery codes single use and replaced as a set; pending step expiry, single use, cookie binding
and deletion on factor clear and admin removal; second-step failures counted by the limiter;
wrong password response identical with and without 2FA; software-authenticator passkey
registration and sign-in, with bad origin, bad RP ID, no UV, other admin's credential and replayed
challenge refused; `confirm_required` on every sensitive route; `AccessEnroll` allows only the
setup routes (route walk); tokens refused on every new route; migration 5 to 6 on a v5 database;
`reset-2fa` command; web components for each new screen with `navigator.credentials` stubbed.

## 2. Authenticator app (User Story 1, SC-001, SC-002)

1. Sign in as `alice`, open Administration › Two-Factor Sign-In, choose "Add authenticator app".
2. Scan the QR code, enter a wrong code: refused, nothing saved. Enter the current code.
3. Expected: 10 recovery codes shown once, with Copy and Download; `alice`'s session in a second
   browser is signed out. Time the whole setup: under 2 minutes.
4. Sign out, sign in with the password. Expected: a code prompt, no access to any page yet.
5. Enter the code just used: refused. Wait for the next code and enter it: signed in.
6. Enter 5 wrong codes on a fresh sign-in: the 6th attempt says to wait.

## 3. API tokens unchanged (SC-005, FR-030, FR-031)

```sh
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/beers -H "Authorization: Bearer $TOKEN"
# 200, same as before
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/2fa -H "Authorization: Bearer $TOKEN"
# 403
```

## 4. Passkey with biometrics (User Story 2, SC-003, SC-006)

1. On the device with fingerprint or face unlock, open the admin app as an installed PWA, sign
   in as `alice`, add a passkey named "Test phone", approve the device prompt.
2. Sign out and sign in: after the password, choose Passkey and approve. Expected: signed in
   within 10 seconds of entering the password.
3. Repeat in a normal browser tab on the same device: same result.
4. Cancel the device prompt once: no session, and the code option still works.

## 5. Recovery (User Story 3, SC-004)

1. Sign in as `alice` with a recovery code. Expected: signed in; the same code is refused next
   time; the page shows 9 remaining.
2. Make a new set: an old unused code is refused afterwards.
3. As `bob`, open Admin Users: `alice` shows 2FA on. Clear it, confirming it is you if asked.
   Expected: `alice` is signed out everywhere and signs in with the password alone.
4. Operator path:

   ```sh
   docker compose exec api beer-menu-api reset-2fa alice
   ```

   Expected: logged with username and time; `alice` signs in with the password alone.

## 6. Require 2FA for all (User Story 5)

1. As `bob` (no 2FA), try to turn on "Require 2FA for all admins": refused.
2. As `alice` (2FA on), turn it on after confirming password and code. `bob` stays signed in.
3. `bob` signs out and in: after the password, only the setup screen is shown. Try
   `GET /api/beers` from that browser's devtools: 401. Finish setup: recovery codes shown, signed in.
4. As `alice`, try to remove her last factor: refused, naming the setting.
5. As `alice`, clear `bob`'s 2FA. Sign in as `bob`, reach the setup screen, press Sign out, then reload and go back:
   setup cannot continue, and the login form shows.
6. Wait 6 minutes after a confirm and try to remove a factor: asked to confirm again.

## 7. No passkey support (FR-016)

1. Run with `BEER_MENU_PUBLIC_URL` unset, or open the app by IP address.
2. Expected: the passkey option says it is not available here; authenticator codes still work.
   With `BEER_MENU_PUBLIC_URL` set, opening the app under any other address gives the same
   message, naming the address to use.
3. Unset the encryption key too: the authenticator option says it is not available; no 500s.
