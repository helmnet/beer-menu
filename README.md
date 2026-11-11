# beer-menu

A self-hosted digital tap list for a home bar: a React admin app manages the beers, kegs and
taps, and a TV shows the current menu in a browser. It runs as two containers: a Go REST API
that owns a SQLite database, and nginx serving the React app and proxying `/api` to the API.

More documentation:

- [API](docs/api.md): endpoints, tokens and access rules for scripts and Home Assistant.
- [Development](docs/development.md): repo layout, local development, tests, CI and releases.

## Run with Docker Compose

From a checkout of this repo, with the published images:

```sh
export PUID=$(id -u) PGID=$(id -g)
mkdir -p data
docker compose pull
docker compose up -d
```

To build the images from the checkout instead, run `docker compose up -d --build`.

Open `http://localhost:8080/` for the admin app and `http://localhost:8080/display` for the
tap list. Both containers run as `PUID:PGID`, which can be any non-root UID and GID (never 0)
that owns the data folder. Both containers refuse to start as UID 0 or GID 0, or with root as
a supplementary group.

| Variable | Default | Meaning |
| --- | --- | --- |
| `PUID`, `PGID` | `65532` | UID and GID both containers run as |
| `BEER_MENU_DATA` | `./data` | Host folder mounted as the API's `/data` |
| `BEER_MENU_WEB_PORT` | `8080` | Host port of the web container |
| `BEER_MENU_VERSION` | `latest` | Image tag; production pins an exact `X.Y.Z` |

The database is `data/beermenu.db`; backups go to `data/backups/`.

## Upgrade

Set `BEER_MENU_VERSION` to the new `X.Y.Z`, then:

```sh
docker compose pull
docker compose up -d
```

At startup the API backs up the database before it applies any pending migrations (see
[Restore from a backup](#restore-from-a-backup)).

## Admins and sign-in

**First admin.** A fresh install has no admin and no default password. Either open
`http://localhost:8080/` and create the first admin on the setup screen, or set both of these
on the `api` service before the first start (see the commented lines in
`docker-compose.yaml`):

| Variable | Meaning |
| --- | --- |
| `BEER_MENU_ADMIN_USERNAME` | Username of the first admin |
| `BEER_MENU_ADMIN_PASSWORD` | Its password, 12 to 256 characters; pass it from a secret, never commit it |

They are used only while no admin exists and are ignored after that. Setting only one, or a
value that breaks the rules, stops the API with an error. Remove them once the admin exists.

**Cookie `Secure` flag.** `BEER_MENU_COOKIE_SECURE` (default `true`) marks the session cookie
`Secure`, so browsers send it only over HTTPS and to `http://localhost`. Set it to `false`
only when you reach the app over plain HTTP by LAN address or hostname; otherwise sign-in
appears to work but the next request is not signed in. Behind a TLS proxy, leave it on.

**Install as an app.** Browsers offer to install the admin app only over HTTPS, so open it by
its HTTPS hostname first. On iOS, use Share > Add to Home Screen; the installed app has its own
sign-in, separate from Safari's.

**Locked out.** If every admin has lost their password, reset one from the server:

```sh
docker compose exec -it api /beer-menu-api reset-password <username>
```

It asks for the new password twice without echoing it, refuses unless run in a terminal
(`-it`), and ends all of that admin's sessions. There is no way to reset a password over HTTP.

**API tokens.** Admins create tokens in the admin app for scripts and Home Assistant, with
scope `read` or `write`. A token's secret is shown once, when it is created; only its hash is
stored. Every secret starts with `bmt_`, so secret scanners can find leaked ones. Revoke a
leaked token in the admin app. See [docs/api.md](docs/api.md) for how clients use them.

## TV display

Open `/display` on the TV (Fully Kiosk on a Fire TV Stick); it needs no credential. It shows
the enabled taps in order, then an "On deck" page of waiting kegs and beers still being made,
each drawn filled to its keg level. Pages hold up to 8 cards (4 × 2); with more than one page
they rotate every 15 seconds (a display setting) with a "N / M" indicator in the corner. It
checks for menu changes every 10 seconds, keeps the last menu through outages with a small
"offline" notice after about 2 minutes, and reloads itself about once a day to pick up
upgrades.

The display shows only display fields (tap label, beer name, style, ABV, IBU, SRM,
description, color, keg shape and level). The full beer library, keg volumes and units need a
credential.

## Display settings

The admin app's "Display settings" panel sets:

| Setting | Values | Default | Applies to |
| --- | --- | --- | --- |
| Title | text, up to 60 characters; empty shows no title | empty | TV |
| Theme | dark or light | dark | TV only (the admin app is unchanged) |
| Units | US (gallons, pours in fluid ounces) or metric (litres, pours in millilitres) | US | admin app volumes |
| Page seconds | whole seconds, 5 to 300 | 15 | TV page rotation |
| Layout | Grid (cards, 8 a page), Vertical list or Side by side (6 rows a page) | Grid | TV |
| Title alignment, font and color | left, center or right; one of 8 fonts shipped with the app; any color or the theme's | left, System, theme | TV title |
| Text font and color | one of 8 fonts; any color or the theme's | System, theme | tap and on deck text |
| Tap details | show or hide style, ABV, IBU, SRM and description | style, ABV and IBU shown | tap cards only |

A preview beside the form shows the TV with the unsaved settings and the current menu. The TV
picks a saved change up on its next menu check, within about 10 seconds.

## Brewfather import

The admin app can connect one Brewfather account, list its batches, and import them as beers.
The Brewfather API key is stored encrypted in the database, and the API never returns it. The
encryption key lives outside the database, so a copy of the database or a backup alone cannot
reveal the Brewfather key.

Generate the encryption key once:

```sh
openssl rand -base64 32 > beer-menu-secret-key
chown "$PUID:$PGID" beer-menu-secret-key
chmod 0600 beer-menu-secret-key
```

Give it to the API in one of two ways:

| Variable | Meaning |
| --- | --- |
| `BEER_MENU_SECRET_KEY_FILE` | Path to a file holding the key, such as a compose secret (preferred; wins if both are set) |
| `BEER_MENU_SECRET_KEY` | The key itself |

Prefer the file: environment values show in `docker inspect` and in the process environment.
`docker-compose.yaml` has a commented example that mounts the file as a compose secret. The key
is standard base64 of exactly 32 bytes; a key that is set but unreadable or malformed stops the
API at startup with an error naming the variable, never the value. With neither variable set,
the API runs and the admin app says Brewfather import is not set up.

Keep a copy of the key somewhere other than the data folder. Losing it, or changing it, loses
only the Brewfather connection: the admin app shows "Reconnect needed" and you enter the
Brewfather API key again. Imported beers are not affected. A restored backup needs the same
encryption key to keep its connection.

## Restore from a backup

Before it applies pending migrations, the API copies the database to
`data/backups/beermenu-<timestamp>-v<N>.db`, where `<N>` is the schema version it was taken
from. It keeps the 5 newest copies. To go back to one of them:

1. Stop the stack:

   ```sh
   docker compose down
   ```

2. Copy the chosen backup over the database and delete the stale WAL files:

   ```sh
   cp data/backups/beermenu-<timestamp>-v<N>.db data/beermenu.db
   rm -f data/beermenu.db-wal data/beermenu.db-shm
   ```

3. Give the file to the container user and make it readable only by that user:

   ```sh
   chown "$PUID:$PGID" data/beermenu.db
   chmod 0600 data/beermenu.db
   ```

4. Start the stack:

   ```sh
   docker compose up -d
   ```

The API backs up the restored file and re-applies any migrations newer than `v<N>`.

A restore brings back admins, passwords and tokens as they were when the backup was taken.
Tokens revoked since the backup must be revoked again, and admins removed or passwords
changed since then must be removed or changed again.

A restored connection to Brewfather opens only with the encryption key the API had when the
backup was taken; otherwise reconnect (see [Brewfather import](#brewfather-import)).

Backups hold password and token hashes. Keep them, like the database, at mode `0600` and
owned by the container user.
