# Data Model: Authentication and API Tokens

One new migration, `api/internal/store/migrations/0002_auth.sql`, takes the schema from version
1 to 2. As with every migration, the API backs up a version 1 database with data before applying
it, and a test applies it to a version 1 database (constitution IV).

Times are stored as `INTEGER` Unix seconds (UTC) and returned in JSON as RFC 3339 strings.
Secrets are never stored in plain form.

## users

An admin. Brief table `users`.

| Column | Type | Rules |
| --- | --- | --- |
| `id` | `INTEGER PRIMARY KEY` | |
| `username` | `TEXT NOT NULL UNIQUE COLLATE NOCASE` | trimmed; `^[A-Za-z0-9._-]{1,64}$` (FR-006) |
| `password_hash` | `TEXT NOT NULL` | argon2id PHC string (research R1) |
| `created_at` | `INTEGER NOT NULL` | |

- First admin: inserted only when the table is empty, in one statement (research R9).
- Removal: `DELETE FROM users WHERE id = ? AND (SELECT COUNT(*) FROM users) > 1`. Zero rows
  affected with an existing ID means "last admin" (FR-011). Sessions go with it by cascade.

## sessions

A signed-in browser.

| Column | Type | Rules |
| --- | --- | --- |
| `id_hash` | `BLOB PRIMARY KEY` | SHA-256 of the 32-byte session ID (research R4) |
| `user_id` | `INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE` | |
| `created_at` | `INTEGER NOT NULL` | |
| `last_seen_at` | `INTEGER NOT NULL` | written at most once a minute |
| `expires_at` | `INTEGER NOT NULL` | `created_at + 30 days` |

Index: `sessions(user_id)` for "end all sessions of this admin".

**Valid** when `created_at <= now`, `now < expires_at` and `now < last_seen_at + 7 days`
(FR-016; the first check stops a backwards clock jump from extending a session). Expired rows are
deleted when met and by a sweep at startup.

**Lifecycle**:

```text
sign-in ──> active ──(any request, ≥1 min since last write)──> active (last_seen_at = now)
              │
              ├── sign-out ............................ row deleted
              ├── password change ..................... other rows of that admin deleted
              ├── reset-password ...................... all rows of that admin deleted
              ├── admin removed ....................... all rows deleted (cascade)
              └── idle 7 days or 30 days old .......... invalid, deleted when met
```

## api_tokens

A machine credential. Brief table `api_tokens`. Not tied to an admin (spec Key Entities).

| Column | Type | Rules |
| --- | --- | --- |
| `id` | `INTEGER PRIMARY KEY` | |
| `name` | `TEXT NOT NULL` | 1 to 64 characters after trimming; duplicates allowed |
| `token_hash` | `BLOB NOT NULL UNIQUE` | SHA-256 of the full secret `bmt_...` (research R6) |
| `scope` | `TEXT NOT NULL CHECK (scope IN ('read','write'))` | |
| `created_at` | `INTEGER NOT NULL` | |
| `last_used_at` | `INTEGER` | `NULL` = never used; updated at most once a minute |
| `revoked_at` | `INTEGER` | `NULL` = active |

**Lifecycle**: `active ──revoke──> revoked` (terminal). Revoking a revoked token is a no-op
success. A revoked or unknown token gets `401`.

## Login attempt records (memory only)

Not in the database. A map from lowercased username to the timestamps of its failures in the
last 15 minutes, plus a count of attempts still being checked, which count as failures. Limit: 5
per username. Usernames with failures in the window are never evicted; at 10,000 tracked,
expired ones are swept (research R8). Lost on restart by design.

## Access levels (code, not data)

| Access | Session | `read` token | `write` token | No credential |
| --- | --- | --- | --- | --- |
| `public` | allowed | allowed | allowed | allowed |
| `read` | allowed | allowed | allowed | 401 |
| `write` | allowed | 403 | allowed | 401 |
| `admin` (default) | allowed | 403 | 403 | 401 |

Unknown, malformed or revoked bearer credential: 401 on every non-public route, even when a
valid session cookie is also sent (research R7).

## Backup and restore note

Backups now hold password hashes, session ID hashes and token hashes. The feature 1 file modes
(`0600`, folder `0700`) already cover them. A restore brings back admins and tokens as they were
at backup time; the README's restore steps gain a line telling the owner to revoke again any
token revoked since the backup.
