# Quickstart: validate selective CI

## Local

```sh
sh .github/ci/changed-sides.test.sh          # self-check of the path rules; prints OK
printf 'web/src/App.tsx\n' | sh .github/ci/changed-sides.sh
# api=false / web=true / services=web
```

## On GitHub (one throwaway pull request per row, then close it)

| Change in the pull request | Expect ran | Expect Skipped |
|----------------------------|------------|----------------|
| one file under `web/` | `changes`, `web`, `images` (web image checked, smoke test ran) | `api`, `openapi` |
| one file under `api/` | `changes`, `api`, `openapi`, `images` (api image checked) | `web` |
| `docker-compose.yaml` | all | none |
| `.github/workflows/ci.yml` | all | none |
| only `specs/**/*.yaml` | `changes` | `api`, `web`, `openapi`, `images` |
| only a `.md` file | `changes`, `secrets` | `api`, `web`, `openapi`, `images` |
| `api/x.go` moved to `specs/x.go` | `changes`, `api`, `openapi`, `images` | `web` |
| a web test changed to fail | `web` **fails** (SC-006) | `api`, `openapi` |

For the `images` rows, open the job log: the user, root-refusal and Trivy steps name only the
changed side's image, and the smoke test passes.

No row leaves a check pending once the run finishes (SC-004). Compare the web-only row's total
time with a recent full run on `main` (SC-005: at least a third shorter).

## Results (2026-11-13, T011)

Throwaway draft pull requests #50–#57, based on this branch, then closed and their branches
deleted. Feature pull request: #49.

| Row | PR | Ran | Skipped | Matches |
|-----|----|-----|---------|---------|
| one file under `web/` | #50 | `changes`, `web`, `images` | `api`, `openapi` | yes |
| one file under `api/` | #51 | `changes`, `api`, `openapi`, `images` | `web` | yes |
| `docker-compose.yaml` | #52 | all | none | yes |
| `.github/workflows/ci.yml` | #53 | all | none | yes |
| only `specs/**/*.yaml` | #54 | `changes` | `api`, `web`, `openapi`, `images` | yes |
| only a `.md` file | #55 | `changes`, `secrets` | `api`, `web`, `openapi`, `images` | yes |
| `api/…/srm_test.go` moved to `specs/` | #56 | `changes`, `api`, `openapi`, `images` | `web` | yes |
| a web test changed to fail | #57 | `web` **failed**, `images` passed | `api`, `openapi` | yes (SC-006) |

- No check was left pending in any row (SC-004); skipped jobs show as "skipping".
- `images` logs: #50 listed and checked only `beer-menu-web`, #51 only `beer-menu-api`, #52 both;
  the smoke test passed in all three.
- SC-005 **not met**: the web-only run used 170 s of job time (`changes` 6 s, `web` 60 s,
  `images` 104 s) against 234 s for the full run on #49, 27% less, short of a third. Wall-clock
  time barely moves, because `images` still builds both images for the smoke test and dominates.
  The saving is the skipped `api`/`openapi` jobs plus one image's checks and Trivy scan.
