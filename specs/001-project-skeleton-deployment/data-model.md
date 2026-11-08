# Data Model: Project Skeleton and Deployment

**Feature**: [spec.md](./spec.md) | **Research**: [research.md](./research.md)

Feature 1 stores no menu data. Its model is the database file itself, its schema version, its
migrations and its backups. The menu tables from the brief arrive with features 2 to 7.

## Database file

| Item | Value |
| --- | --- |
| Path | `/data/beermenu.db` (folder from env `BEER_MENU_DATA_DIR`, default `/data`) |
| Journal | WAL (`beermenu.db-wal`, `beermenu.db-shm` next to it) |
| Owner and mode | the container user (the non-root UID/GID the deployer assigns), `0600` (also the `-wal` and `-shm` files) |

## Schema version

- Stored in SQLite's `PRAGMA user_version` (integer in the file header).
- `0` means a brand-new database: no migration applied, no backup needed.
- `N` means migrations `0001` to `N` are applied.
- Rule: if `user_version` is greater than the newest embedded migration, the API exits without
  changing the file (downgrade refusal, FR-014).

## Migration

| Field | Rule |
| --- | --- |
| File name | `NNNN_snake_name.sql`, four digits, starting at `0001`, no gaps, no duplicates |
| Version | the number `NNNN` |
| Body | plain SQL; runs in one transaction with `PRAGMA user_version = NNNN` |

Validation at startup: the embedded set must be numbered `1..max` with no gaps. A gap or a bad
name is a build defect, caught by a unit test and refused at startup.

### Migration 0001: baseline

Creates the `settings` key-value table from the brief, used by later features (display title,
units) and present now so the migration and backup path runs on a real schema change.

| Column | Type | Rule |
| --- | --- | --- |
| `key` | TEXT | primary key, not null |
| `value` | TEXT | not null |

No rows are inserted.

## Backup file

| Item | Value |
| --- | --- |
| Folder | `/data/backups/`, mode `0700`, created on first use |
| Name | `beermenu-<YYYYMMDDTHHMMSS.mmmZ>-v<from>.db`, UTC |
| Pattern | `^beermenu-\d{8}T\d{6}\.\d{3}Z-v\d+\.db$` |
| Owner and mode | same owner as the database, `0600` |
| Retention | 5 newest matching files; others matching the pattern are deleted; non-matching files are never touched |

## Startup state flow

```text
running as UID 0 or GID 0 ──yes──> exit 1 ("refusing to run as root")
        │
check data folder writable ──fail──> exit 1 ("data folder /data not writable")
        │
open DB, read user_version (v), newest migration (m)
        │
   v > m ──> exit 1, file unchanged ("database is newer than this release")
   v = m ──> serve (no backup)
   v = 0 ──> migrate 1..m ──> serve
 0<v<m  ──> backup ──fail──> exit 1, no migration
              │
              ok ──> prune to 5 ──> migrate v+1..m
                                      │
                       migration fails ──> roll back that migration, exit 1 (backup kept)
                                      │
                                      ok ──> serve
```
