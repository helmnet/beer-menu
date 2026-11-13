# Data Model: Run Only the CI Checks a Change Needs

No stored data. The only "data" is what flows through one CI run.

## Change

The list of file paths between the base and head commits (research R1). Rename detection is off (`--no-renames`), so a moved file is listed under both its old
(deleted) and new (added) path; moving a file out of `api/` still runs the API checks. Deletions
appear under the deleted path.

## File group

Each path maps to exactly one group by the first matching rule in research R6: `api`, `web`,
`both`, or `neither`.

## Side flags (output of the `changes` job)

| Field | Values | Rule |
|-------|--------|------|
| `api` | `true` / `false` | `true` if any path is `api` or `both`, or the fallback (R2) fired |
| `web` | `true` / `false` | `true` if any path is `web` or `both`, or the fallback fired |
| `services` | `api`, `web`, `api web`, or empty | compose services whose image checks run; matches the flags |

See [contracts/changes-job.md](contracts/changes-job.md) for which job reads which flag.
