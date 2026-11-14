#!/bin/sh
# Self-check for changed-sides.sh: feeds fixed file lists and compares the full output with the
# expected api/web/services/notices lines. The `changes` job in ci.yml runs it before classifying.
# Run locally with: sh .github/ci/changed-sides.test.sh
set -eu

script="$(dirname "$0")/changed-sides.sh"
failed=0

# check NAME INPUT API WEB SERVICES NOTICES
check() {
  expected=$(printf 'api=%s\nweb=%s\nservices=%s\nnotices=%s' "$3" "$4" "$5" "$6")
  actual=$(printf '%s' "$2" | sh "$script") || actual="(exit $?) $actual"
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL %s\n--- expected\n%s\n--- actual\n%s\n' "$1" "$expected" "$actual"
    failed=1
  fi
}

nl='
'
check "web file" "web/src/App.tsx" false true "web" false
check "api file" "api/cmd/main.go" true false "api" false
check "openapi doc" "api/openapi.yaml" true false "api" false
check "node version" ".tool-versions" false true "web" false
check "compose file" "docker-compose.yaml" true true "api web" false
check "ci workflow" ".github/workflows/ci.yml" true true "api web" true
check "ci helper" ".github/ci/changed-sides.sh" true true "api web" true
check "trivy image" ".github/ci/images/trivy/Dockerfile" true true "api web" true
check "gitleaks image" ".github/ci/images/gitleaks/Dockerfile" true true "api web" true
check "both sides" "api/x.go${nl}web/y.ts" true true "api web" false
check "web plus spec" "web/a.ts${nl}specs/016-x/contracts/a.yaml" false true "web" false
check "api markdown" "api/README.md" false false "" false
check "move out of api" "api/x.go${nl}specs/x.go" true false "api" false
check "root markdown plus api" "README.md${nl}api/x.go" true false "api" false
check "spec openapi copy" "specs/001-x/contracts/openapi.yaml" false false "" false
check "release workflow" ".github/workflows/release.yml" false false "" false
check "dependabot" ".github/dependabot.yml" false false "" false
check "newline in name" "api/a${nl}api=false${nl}web=false" true false "api" false
check "empty input" "" false false "" false
check "fallback" "ALL" true true "api web" true
check "go sum" "api/go.sum" true false "api" true
check "go mod" "api/go.mod" true false "api" true
check "swagger ui" "api/internal/httpapi/docs/swagger-ui/fetch.sh" true false "api" true
check "swagger readme" "api/internal/httpapi/docs/swagger-ui/README.md" false false "" false
check "package lock" "web/package-lock.json" false true "web" true
check "package json" "web/package.json" false true "web" true
check "notices output" "web/THIRD_PARTY_NOTICES" false true "web" true
check "license copy" "api/LICENSE" true false "api" true
check "root license" "LICENSE" false false "" true
check "notices script" "scripts/third-party-notices.sh" false false "" true
check "nested go sum" "api/x/go.sum" true false "api" false

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo OK
