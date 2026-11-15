#!/bin/sh
# Self-check for changed-sides.sh: feeds fixed file lists and compares the full output with the
# expected api/web/guide/services/notices/workflows lines. The `changes` job in ci.yml runs it before
# classifying.
# Run locally with: sh .github/ci/changed-sides.test.sh
set -eu

script="$(dirname "$0")/changed-sides.sh"
failed=0

# check NAME INPUT API WEB GUIDE SERVICES NOTICES WORKFLOWS
check() {
  expected=$(printf 'api=%s\nweb=%s\nguide=%s\nservices=%s\nnotices=%s\nworkflows=%s' "$3" "$4" "$5" "$6" "$7" "$8")
  actual=$(printf '%s' "$2" | sh "$script") || actual="(exit $?) $actual"
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL %s\n--- expected\n%s\n--- actual\n%s\n' "$1" "$expected" "$actual"
    failed=1
  fi
}

nl='
'
check "web file" "web/src/App.tsx" false true false "web" false false
check "api file" "api/cmd/main.go" true false false "api" false false
check "openapi doc" "api/openapi.yaml" true false false "api" false false
check "node version" ".tool-versions" false true false "web" false false
check "compose file" "docker-compose.yaml" true true true "api web" false false
check "api startup" "api/cmd/beer-menu-api/main.go" true false true "api" false false
check "api startup test" "api/cmd/beer-menu-api/startup_test.go" true false true "api" false false
check "api internal" "api/internal/store/store.go" true false false "api" false false
check "ci workflow" ".github/workflows/ci.yml" true true true "api web" true true
check "ci helper" ".github/ci/changed-sides.sh" true true true "api web" true true
check "trivy image" "ci/images/trivy/Dockerfile" true true false "api web" false false
check "gitleaks image" "ci/images/gitleaks/Dockerfile" true true false "api web" false false
check "both sides" "api/x.go${nl}web/y.ts" true true false "api web" false false
check "web plus spec" "web/a.ts${nl}specs/016-x/contracts/a.yaml" false true false "web" false false
check "api markdown" "api/README.md" false false false "" false false
check "move out of api" "api/x.go${nl}specs/x.go" true false false "api" false false
check "root markdown plus api" "README.md${nl}api/x.go" true false false "api" false false
check "spec openapi copy" "specs/001-x/contracts/openapi.yaml" false false false "" false false
check "release workflow" ".github/workflows/release.yml" false false false "" false true
check "dependabot" ".github/dependabot.yml" false false false "" false false
check "newline in name" "api/a${nl}api=false${nl}web=false" true false false "api" false false
check "empty input" "" false false false "" false false
check "fallback" "ALL" true true true "api web" true true
check "go sum" "api/go.sum" true false false "api" true false
check "go mod" "api/go.mod" true false false "api" true false
check "swagger ui" "api/internal/httpapi/docs/swagger-ui/fetch.sh" true false false "api" true false
check "swagger readme" "api/internal/httpapi/docs/swagger-ui/README.md" false false false "" false false
check "package lock" "web/package-lock.json" false true false "web" true false
check "package json" "web/package.json" false true false "web" true false
check "notices output" "web/THIRD_PARTY_NOTICES" false true false "web" true false
check "license copy" "api/LICENSE" true false false "api" true false
check "root license" "LICENSE" false false false "" true false
check "notices script" "scripts/third-party-notices.sh" false false false "" true false
check "guide build" "guide/build.mjs" false false true "" false false
check "guide readme" "guide/README.md" false false false "" false false
check "guide color source" "web/src/pages/AdminPage.css" false true true "web" false false
check "pages workflow" ".github/workflows/pages.yml" false false true "" false true
check "nested go sum" "api/x/go.sum" true false false "api" false false
check "other workflow" ".github/workflows/secrets.yml" false false false "" false true
check "workflow markdown" ".github/workflows/README.md" false false false "" false false
check "actionlint image" "ci/images/actionlint/Dockerfile" false false false "" false true
check "actionlint config" ".github/actionlint.yaml" false false false "" false true
check "actionlint config yml" ".github/actionlint.yml" false false false "" false true

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo OK
