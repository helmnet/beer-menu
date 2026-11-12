# Data Model: Two-Factor Sign-In for Admins

Migration `0006_two_factor.sql` takes the database from version 5 to 6. Feature 1's backup runs
before it. All times are Unix seconds. Every new table references `users(id) ON DELETE CASCADE`,
so removing an admin removes all of their 2FA data.

## `totp_factors` (one per admin at most, FR-012)

| Column | Type | Rule |
| --- | --- | --- |
| `user_id` | INTEGER PK, FK users | one authenticator per admin |
| `secret_sealed` | TEXT NOT NULL | `secrets.Box` seal, purpose `totp:<user_id>` (R6); never returned |
| `last_step` | INTEGER NOT NULL | last accepted 30-s step; a code is accepted only for a later step (FR-010) |
| `created_at` | INTEGER NOT NULL | |
| `last_used_at` | INTEGER | set on each accepted sign-in or confirm |

## `passkeys` (at most 10 per admin, FR-013)

| Column | Type | Rule |
| --- | --- | --- |
| `id` | INTEGER PK | used in routes |
| `user_id` | INTEGER NOT NULL, FK users | index `passkeys_user_id` |
| `name` | TEXT NOT NULL | 1–64 code points, trimmed, no control characters (FR-013) |
| `credential_id` | BLOB NOT NULL UNIQUE | from the authenticator |
| `public_key` | BLOB NOT NULL | COSE key |
| `sign_count` | INTEGER NOT NULL | updated on each use; a non-zero count that does not increase is refused (cloned key) |
| `transports` | TEXT NOT NULL | JSON array, passed back in `allowCredentials` |
| `backup_eligible` | INTEGER NOT NULL | 0/1 flag the library checks on each use |
| `created_at` | INTEGER NOT NULL | |
| `last_used_at` | INTEGER | |

Exactly the fields `webauthn.Credential` needs to verify an assertion; the attestation statement
is not kept.

## `recovery_codes`

| Column | Type | Rule |
| --- | --- | --- |
| `user_id` | INTEGER NOT NULL, FK users | |
| `code_hash` | BLOB NOT NULL | SHA-256 of the normalized code (R7) |
| `used_at` | INTEGER | NULL until used; a used code never works again (FR-018) |

Primary key `(user_id, code_hash)`. A set is 10 rows. "Remaining" = rows with `used_at IS NULL`.

## `auth_challenges` (R8)

| Column | Type | Rule |
| --- | --- | --- |
| `id_hash` | BLOB PK | SHA-256 of the `bm_pending` ID, or the session's `id_hash` for kind `setup` |
| `user_id` | INTEGER NOT NULL, FK users | |
| `kind` | TEXT NOT NULL | CHECK in (`second_factor`, `enroll`, `setup`) |
| `webauthn` | TEXT | in-flight WebAuthn session JSON (challenge, allowed IDs, UV) or NULL |
| `totp_pending_sealed` | TEXT | sealed secret of an unconfirmed authenticator setup, or NULL |
| `created_at` | INTEGER NOT NULL | |
| `expires_at` | INTEGER NOT NULL | `created_at + 300` (FR-002) |

A `setup` row is deleted with its session (ON DELETE of sessions is not a FK because the key is
shared; `DeleteSession` and session-ending paths delete it in the same transaction).

## `sessions` (changed)

| Column | Type | Rule |
| --- | --- | --- |
| `confirmed_at` | INTEGER (new, NULL) | set by `POST /api/2fa/confirm` and at sign-in; sensitive changes need it within 300 s (R9) |

Sign-in sets `confirmed_at = now`, so an admin can set up 2FA right after signing in without a
second prompt. For an admin with 2FA, sign-in already took the password and a factor, so for the
next 5 minutes sensitive changes need no new prompt (spec Assumptions).

## `settings` (row, no schema change)

| Key | Values | Default |
| --- | --- | --- |
| `require_2fa` | `"1"` / `"0"` | missing or anything else reads as off (FR-026) |

## Derived state

- **Has 2FA** = a `totp_factors` row or at least one `passkeys` row. Recovery codes alone do not
  count; removing the last factor deletes them (FR-025).
- **Admin list** gains `two_factor: boolean` (FR-028).

## State transitions (one admin)

```text
           add first factor (confirmed)            remove last factor / turn off
 no 2FA ─────────────────────────────────▶ 2FA on ─────────────────────────────▶ no 2FA
   │      + 10 recovery codes, end other         (refused while require_2fa=1)    − codes,
   │        sessions (FR-017, FR-024)                                               end others
   │
   │ require_2fa=1 and correct password
   ▼
 enroll pending ── first factor confirmed ──▶ 2FA on + new session + codes shown

 2FA on ── cleared by other admin (FR-027) or reset-2fa (FR-029) ──▶ no 2FA,
           all sessions, pending steps and codes deleted
```

Sign-in for an admin with 2FA:

```text
 password ok ─▶ second_factor pending (5 min, single use) ─▶ valid code / recovery / passkey ─▶ session
                     │ wrong factor: limiter failure, pending kept until expiry
                     ├ cancelled (DELETE /api/session/pending): row deleted, cookie cleared
                     └ expired, used, or factors cleared: 401, start over

 An enroll pending ends the same ways: first factor confirmed, cancelled (Sign out), or expired.
```
