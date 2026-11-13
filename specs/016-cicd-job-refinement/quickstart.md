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
