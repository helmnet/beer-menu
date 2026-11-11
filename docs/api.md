# API

Every client, the admin app and the TV included, uses the same REST API under `/api`.
[`api/openapi.yaml`](../api/openapi.yaml) is the full contract; the running API also serves it
as browsable docs at `/api/docs`. This page is an overview.

## Access

Each operation's `x-access` in `api/openapi.yaml` says who may call it:

| Access | Who |
| --- | --- |
| `public` | Anyone, no credential |
| `read` | An admin session, or any token (`read` or `write`) |
| `write` | An admin session, or a `write` token |
| `admin` | An admin session only |

With no credential, a `read` call gets 401. A `read` token on a `write` call gets 403.

**Tokens.** Admins create tokens in the admin app with scope `read` or `write`. Clients send
them as `Authorization: Bearer <token>`. A token's secret is shown once, when it is created;
only its hash is stored. Every secret starts with `bmt_`, so secret scanners can find leaked
ones (`.gitleaks.toml` has a rule for them). Revoke a leaked token in the admin app.

`GET /api/health` is public and reports the app version and whether the database answers.

## Beers

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/beers` | read | Current beers by name; `?archived=true` lists only archived ones |
| `GET` | `/api/beers/{id}` | read | One beer, archived or not |
| `POST` | `/api/beers` | write | Add a beer |
| `PATCH` | `/api/beers/{id}` | write | Edit, archive (`{"archived": true}`) or restore (`{"archived": false}`) |
| `DELETE` | `/api/beers/{id}` | write | Delete a beer permanently |
| `GET` | `/api/srm-colors` | read | The SRM color table |

The full library is not public: the TV display reads `GET /api/menu`, which holds only what
the TV shows. `PATCH` merges: fields left out are kept, `null` clears an optional field, and a
value sets it. `source` and `source_ref` cannot be set by clients.

Beers have no uploaded images. Each beer's artwork is drawn by the web app as inline SVG, a
corny keg filled with the `color` the API derives from the beer's SRM. Only a beer on a tap is
drawn as a pint glass. Beer reads include `on_tap`. A beer with a current keg (on a tap or
waiting) cannot be archived (409 `beer_has_keg`), and a beer with any keg, current or kicked,
cannot be deleted (409 `beer_has_kegs`); archive it instead.

## Taps and kegs

Taps are the slots on the kegerator; kegs hold beers.

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/api/taps` | read | Taps in order, each with its current keg or `null` |
| `GET` | `/api/taps/{id}` | read | One tap |
| `POST` | `/api/taps` | write | Add a tap at the end (default label "Tap N") |
| `PATCH` | `/api/taps/{id}` | write | Rename, enable or disable |
| `PUT` | `/api/taps/order` | write | Set the order: `{"ids": [...]}` listing every tap once |
| `DELETE` | `/api/taps/{id}` | write | Remove a tap with no keg; at least one tap must remain |
| `GET` | `/api/kegs` | read | `?state=current` (default), `waiting` or `kicked` (history); `beer_id`, `tap_id` filter |
| `GET` | `/api/kegs/{id}` | read | One keg |
| `POST` | `/api/kegs` | write | Add a keg, on a tap (`tap_id`) or waiting off-tap |
| `PATCH` | `/api/kegs/{id}` | write | Correct, set `remaining_l`, move (`tap_id`, `null` takes it off) or kick (`{"kicked": true}`) |
| `DELETE` | `/api/kegs/{id}` | write | Delete a keg record |
| `POST` | `/api/kegs/{id}/pours` | write | Record a pour: `{"amount_l": 0.47}` |
| `GET` | `/api/keg-types` | read | The keg types with default sizes and drawing shape |

Volumes are litres in the API, rounded to 0.01 L; the admin app shows them in the units
setting. Home Assistant and keg scales should use `POST /api/kegs/{id}/pours` with a `write`
token: concurrent pours all count, and the level stops at 0 without kicking the keg. A tap
holds one current keg; a kicked keg is read-only history.

Conflicts answer 409 with one of these `error` codes: `tap_occupied`, `tap_has_keg`,
`last_tap`, `keg_kicked`, `beer_has_keg`, `beer_has_kegs`.

## Menu

`GET /api/menu` is the only public read; the TV display polls it every 10 seconds. It returns
display fields only (tap label, beer name, style, ABV, IBU, color, keg shape) and the keg level
as a fraction from 0 to 1: no ids, sizes, volumes or timestamps. Tap beers also carry `srm` and
`description`, each left out when the beer has none; on deck beers never carry them. Home
Assistant and scripts that need volumes use the credentialed `GET /api/kegs` and `GET /api/taps`
with a token. The menu also carries `display`: every display setting below except `units`,
which is not public.

## Display settings

| Setting | Values | Default | Applies to |
| --- | --- | --- | --- |
| `title` | text, up to 60 characters; empty shows no title | empty | TV |
| `theme` | `dark` or `light` | `dark` | TV only (the admin app is unchanged) |
| `units` | `us` or `metric` | `us` | admin app volumes |
| `page_seconds` | whole seconds, 5 to 300 | 15 | TV page rotation |
| `layout` | `grid`, `list` (one row per item) or `columns` (taps beside on deck) | `grid` | TV |
| `title_align` | `left`, `center` or `right` | `left` | TV title |
| `title_font`, `text_font` | `system`, `inter`, `lora`, `roboto-slab`, `oswald`, `nunito`, `bebas-neue` or `pacifico` | `system` | TV title; all other TV text |
| `show_style`, `show_abv`, `show_ibu` | `true` or `false` | `true` | tap cards only |
| `show_srm`, `show_description` | `true` or `false` | `false` | tap cards only |
| `title_color`, `text_color` | `#rrggbb` (returned lowercase), or empty for the theme's color | empty | TV title; tap and on deck text |

`GET /api/settings` (read) returns all of them; `PATCH /api/settings` (write) changes the ones
named, all or nothing. Switches must be JSON booleans. Colors are not contrast-checked.

## Brewfather

The endpoints under `/api/brewfather` connect one Brewfather account, list its batches, and
import them as beers. The stored Brewfather API key is never returned. Setup of the encryption
key is in the [README](../README.md#brewfather-import).
