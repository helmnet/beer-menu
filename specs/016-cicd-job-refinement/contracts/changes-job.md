# Contract: `changes` job and the checks that read it

## Classifier script

`.github/ci/changed-sides.sh`

- **Input**: newline-separated repo-relative paths on stdin, or the single word `ALL` to force the
  fallback.
- **Output** (stdout, `$GITHUB_OUTPUT` format):

  ```text
  api=true
  web=false
  services=api
  ```

- **Exit**: always 0 for any input. It never fails a run because of what changed.
- **Rules**: research R6, in one `case` statement.
- **`ALL`**: a repo-root file literally named `ALL` would also trigger the fallback; harmless, it
  only runs every check.

## Job gating in `ci.yml`

| Check (job name) | Runs when | When skipped |
|------------------|-----------|--------------|
| `changes` | always | never |
| `api` | `api == 'true'` | reports Skipped |
| `openapi` | `api == 'true'` | reports Skipped |
| `web` | `web == 'true'` | reports Skipped |
| `images` | `api == 'true' \|\| web == 'true'` | reports Skipped |

Inside `images`: `docker compose build` builds both images; one step lists the changed images once
with `docker compose config --images $SERVICES` into `IMAGES` and fails the job if the list is
empty (otherwise the loops would pass on nothing); the user, root-refusal and Trivy steps loop
over `$IMAGES` only; the smoke test runs unchanged.

## Required checks

`changes`, `api`, `web`, `openapi`, `images` (workflow `ci`) and `secrets` (workflow `secrets`).
Skipped counts as passing; failing or pending blocks the merge.
