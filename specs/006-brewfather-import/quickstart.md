# Quickstart: Brewfather Import

Validation guide for feature 6. Routes and shapes are in
[contracts/brewfather.openapi.yaml](./contracts/brewfather.openapi.yaml), storage in
[data-model.md](./data-model.md).

Never paste a real Brewfather key into a file in the repo, a test, a commit or a chat log.
Sections 2 to 6 use the owner's real account; the key is read from the terminal into a shell
variable only.

## Prerequisites

- Feature 5 quickstart prerequisites (Docker Compose v2, Go 1.27, Node 26, `curl`, `jq`).
- A Brewfather account with at least one batch, and an API key with the `batches.read` scope
  (Brewfather: Settings → API keys → Manage → Create API key).
- An encryption key for the server: `openssl rand -base64 32`, kept outside the repo.

## P. Real-account probe (before task T016)

Run once by the owner, in a terminal, before the Brewfather client is written. It checks the
two things the docs leave open (research R1): whether `include` returns every mapped field, and
whether `complete=true` pages fit the 4 MiB cap. The key is typed at a hidden prompt and passed
to curl on stdin, so it is not in shell history or the process list. Output shows field names
and sizes only. Run it in `bash` (the owner's login shell is fish).

```bash
read -rp "Brewfather user ID: " BF_USER; read -rsp "API key: " BF_KEY; echo
bf() { printf 'user = "%s:%s"\n' "$BF_USER" "$BF_KEY" | curl -sS -K - "https://api.brewfather.app/v2/$1"; }
INC='recipe.style.name,recipe.abv,measuredAbv,estimatedIbu,estimatedColor,batchNotes,tasteNotes'
for s in Planning Brewing Fermenting Conditioning Completed; do
  printf '%s: ' $s; bf "batches?status=$s&limit=50&include=$INC" | jq -c 'length'
done
bf "batches?status=Completed&limit=1&include=$INC" | jq '.[0] | paths(scalars) | join(".")' | sort -u
bf "batches?limit=50&complete=true" | wc -c
unset BF_USER BF_KEY; unset -f bf
```

Record in research.md R1: the count per status (shows the filter works), which mapped fields
appear in the path list (`name`, `batchNo`, `status`, `brewDate`, `measuredAbv`, `estimatedIbu`,
`estimatedColor`, `batchNotes`, `tasteNotes`, `recipe.name`, `recipe.style.name`,
`recipe.abv`), and the byte size of a 50-batch `complete=true` page. If any field is missing,
T016 uses `complete=true` (or adds the field to `include`); if the page size is near 4 MiB, it
lowers `limit`.

## 0. Automated tests

```sh
(cd api && go test -race ./...)
(cd web && npm ci && npm test && npm run build)
npx --yes @redocly/cli@2.71.0 lint api/openapi.yaml
```

Expected: all pass, including:

- `secrets`: round trip; wrong key, tampered value and other purpose all fail to open.
- `brewfather` against a TLS test server: Basic header, paging with `start_after`, field and
  status mapping, 401 / 403 / 404 / 429 with `Retry-After` / 500 / slow server / oversized and
  malformed bodies, no redirects followed.
- `importer` on real SQLite with a fake provider: import, duplicate, dismissal, re-sync
  overwrites measurements only, archive on transition and pending while a keg is current,
  not-found, sync counts, single-run lock, scheduler cadence across a restart, rate gate.
- `beers.Sanitize` output always passes `Normalize`.
- Migration 0005 applied to a v4 database with beers and kegs.
- `httpapi`: every new route in the route walk and `x-access` test; admin-only routes refuse
  tokens; no response contains `user_id` or the key; captured logs and the database file plus
  a backup never contain the fake key.
- Startup: malformed `BEER_MENU_SECRET_KEY` / `_FILE` refuses to start without printing it;
  none set starts with `configured: false`.
- Web: `BrewfatherPanel` (states, connect clears the key field, sync polling, import results)
  and `BeersPanel` (source label, re-sync, not-found and archive-pending notes).

## 1. Local stack with an encryption key

From the worktree, with the feature 5 local-stack recipe (scratch data dir, cookie Secure off,
admin from env). Add to the override file:

```yaml
services:
  api:
    environment:
      BEER_MENU_SECRET_KEY_FILE: /run/secrets/beer_menu_secret_key
    secrets: [beer_menu_secret_key]
secrets:
  beer_menu_secret_key:
    file: <scratch>/secret_key   # openssl rand -base64 32 > <scratch>/secret_key
```

Sign in with curl (cookie jar `$J`, plus `Origin` matching the host for writes) and make a
`write` token `$W` and a `read` token `$R`. `B=http://localhost:<port>`.

Expected: `curl -s -b $J $B/api/brewfather` gives `configured: true, connected: false`.

## 2. Connect (story 1)

```sh
read -r BF_USER; read -rs BF_KEY
jq -n --arg u "$BF_USER" --arg k "$BF_KEY" '{user_id:$u, api_key:$k}' |
  curl -s -b $J -H "Origin: $B" -X PUT -d @- $B/api/brewfather/connection | jq
```

Expected: status with `connected: true`; the output has no user ID or key. Then:

- Same call with a wrong key: 400 `credentials_refused`, still `connected: true` with the old
  credentials (nothing replaced).
- Same call with `Authorization: Bearer $W`: 403.
- `docker compose logs api | grep -c "$BF_KEY"` prints `0`.
- `grep -c "$BF_KEY" <scratch>/data/beermenu.db <scratch>/data/backups/*` prints `0` for each.

## 3. List and import (story 2)

```sh
curl -s -H "Authorization: Bearer $W" "$B/api/brewfather/batches" | jq '.batches[:5]'
curl -s -H "Authorization: Bearer $R" "$B/api/brewfather/batches"   # 403
```

Expected: every batch that is not archived, newest brew date first, `beer_id` null, and no
archived batch. Confirm for one batch that name, status and brew date match Brewfather (checks
research R1's `include` list and `status` filter on a real account). Import an archived
batch's ID: outcome `archived`, no beer created.

Import one Fermenting batch with `POST /api/brewfather/imports` `{"batch_ids":["<id>"]}`.
Expected: `imported`, beer with source `brewfather`, style, ABV, IBU, SRM, brew date,
description from tasting or batch notes, status `fermenting`, `sync.synced_at` set. Repeat:
`already_imported`, same beer. Open `/display`: the beer is on the on-deck page.

## 4. Re-sync (story 3)

PATCH the beer's name locally. In Brewfather change the batch status to Conditioning. Then
`POST /api/beers/<id>/sync` with `$W`. Expected: status `conditioning`, local name kept.
`POST /api/beers/<manual id>/sync`: 409 `not_imported`.

## 5. Archive from Brewfather (story 3, FR-017a)

Put a keg of the imported beer on a tap. Archive the batch in Brewfather. Re-sync: beer not
archived, `sync.archive_pending: true`, beer list shows the note. Kick the keg; re-sync:
`archived_at` set, `archive_pending: false`. Unarchive the beer locally and re-sync: it stays
unarchived.

## 6. Schedule and Sync now (story 4)

```sh
curl -s -b $J -H "Origin: $B" -X PUT -d '{"interval":"15m"}' $B/api/brewfather/schedule | jq .interval
curl -s -X POST -H "Authorization: Bearer $W" $B/api/brewfather/sync | jq .sync_running   # true
```

Expected: a second immediate `sync` gives 409 `sync_running`; status later shows `last_sync`
with counts. Move a new batch to Fermenting in Brewfather: within 16 minutes it is imported and
on the TV's on-deck page. Restart the API: the next sync runs on the same cadence. Delete an
auto-imported beer (no kegs); later syncs do not bring it back; importing it by hand does.

## 7. Restore without the key (story 1.7)

Stop the stack, replace the secret key file with a new `openssl rand -base64 32`, start.
Expected: API starts; status `reconnect_needed: true`; no sync runs; connecting again clears it.
Set the file to `not-base64!`: the API refuses to start and the log names
`BEER_MENU_SECRET_KEY_FILE` but not the file's contents. Remove both variables: the API starts
with `configured: false` and every other page works.

## 8. Clean up

`docker compose -p <project> down`; delete the scratch data dir and secret key file; `unset
BF_USER BF_KEY`. Revoke the Brewfather API key if it was made only for this test.
