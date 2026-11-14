# Data Model: Admin App User Guide

The feature adds no database or API data. This file lists the guide's pages, which the build
script's navigation list defines, and the screenshots each page needs.

## Guide page

| Field | Rule |
|-------|------|
| `slug` | Unique. It names the fragment (`src/pages/<slug>.html`) and the output file (`<slug>.html`; the home page is `index.html`). |
| `title` | Shown in the nav, the page `<h1>` and `<title>`. For admin pages, it is the exact side-panel label. |
| `group` | One of `Start`, `Menu`, `Administration`, `Integrations` (spec FR-005). The Menu, Administration and Integrations order follows the app's side panel. |
| `screenshots` | One or more for every admin page, Install with Docker excepted. |

| Group | Slug | Title | Required screenshots (state shown) |
|-------|------|-------|-------------------------------------|
| Start | `index` | Home | the TV display |
| Start | `install` | Install with Docker | none |
| Start | `getting-started` | Getting started | first-run setup form; sign-in form; TV display with sample taps |
| Menu | `taps` | Taps | taps with kegs on them, one tap empty |
| Menu | `on-deck` | On Deck | waiting kegs listed |
| Menu | `keg-history` | Keg History | kicked kegs listed |
| Menu | `beers` | Beer Library | beer list; add/edit beer form |
| Menu | `display` | Display Settings | settings page |
| Administration | `admins` | Admin Users | two sample admins |
| Administration | `tokens` | API Tokens | token list; new-token-shown-once dialog |
| Administration | `password` | Change Password | form |
| Administration | `two-factor` | Two-Factor Sign-In | page with no factors; authenticator setup step |
| Integrations | `brewfather` | Brewfather | not connected |

## Screenshot

| Field | Rule |
|-------|------|
| file | `src/images/<slug>[-<state>].png`, 1280×800 CSS px, admin theme Dark, about 300 KB or less |
| alt | Required and not empty. It says what the screenshot shows (FR-007). |
| data | Comes only from `guide/seed.sh` on a throwaway stack (FR-008). |

## Page body (fragment) structure

1. `<p>` what the page is for.
2. One or more `<figure>` elements, each with an `<img alt>` and a `<figcaption>`.
3. One `<h2>` per task, then an `<ol>` of steps.
4. A `<div class="warning">` wherever an action cannot be undone or a secret is shown once
   (FR-012).
