#!/bin/sh
# Self-check for changed-sides.sh: feeds fixed file lists and compares the full output with the
# expected api/web/services lines. The `changes` job in ci.yml runs it before classifying.
# Run locally with: sh .github/ci/changed-sides.test.sh
set -eu

script="$(dirname "$0")/changed-sides.sh"
failed=0

# check NAME INPUT API WEB SERVICES
check() {
  expected=$(printf 'api=%s\nweb=%s\nservices=%s' "$3" "$4" "$5")
  actual=$(printf '%s' "$2" | sh "$script") || actual="(exit $?) $actual"
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL %s\n--- expected\n%s\n--- actual\n%s\n' "$1" "$expected" "$actual"
    failed=1
  fi
}

nl='
'
check "web file" "web/src/App.tsx" false true "web"
check "api file" "api/cmd/main.go" true false "api"
check "openapi doc" "api/openapi.yaml" true false "api"
check "node version" ".tool-versions" false true "web"
check "compose file" "docker-compose.yaml" true true "api web"
check "ci workflow" ".github/workflows/ci.yml" true true "api web"
check "ci helper" ".github/ci/changed-sides.sh" true true "api web"
check "both sides" "api/x.go${nl}web/y.ts" true true "api web"
check "web plus spec" "web/a.ts${nl}specs/016-x/contracts/a.yaml" false true "web"
check "api markdown" "api/README.md" false false ""
check "move out of api" "api/x.go${nl}specs/x.go" true false "api"
check "root markdown plus api" "README.md${nl}api/x.go" true false "api"
check "spec openapi copy" "specs/001-x/contracts/openapi.yaml" false false ""
check "release workflow" ".github/workflows/release.yml" false false ""
check "dependabot" ".github/dependabot.yml" false false ""
check "newline in name" "api/a${nl}api=false${nl}web=false" true false "api"
check "empty input" "" false false ""
check "fallback" "ALL" true true "api web"

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo OK
