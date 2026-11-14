#!/bin/sh
# Self-check for .githooks/pre-commit: stages files in a throwaway repo with a stub notices script
# and checks when the hook regenerates. The `changes` job in ci.yml runs it.
# Run locally with: sh .github/ci/pre-commit.test.sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
cd "$repo"
git init -q
mkdir -p .githooks .github/ci api scripts
cp "$here/../../.githooks/pre-commit" .githooks/
cp "$here/changed-sides.sh" .github/ci/
# Stub: records each regenerate run; --outputs names the file the hook stages.
cat >scripts/third-party-notices.sh <<'STUB'
if [ "${1:-}" = --outputs ]; then echo ran; else echo x >>ran; fi
STUB
failed=0

# check NAME WANT FILE...: WANT is run or skip.
check() {
  name=$1 want=$2
  shift 2
  git rm -rqf --cached --ignore-unmatch .
  rm -f ran
  for f in "$@"; do echo x >"$f"; done
  git add -- "$@"
  if ! sh .githooks/pre-commit; then
    echo "FAIL $name: hook exited non-zero"
    failed=1
    return
  fi
  if [ -f ran ]; then got=run; else got=skip; fi
  if [ "$got" != "$want" ]; then
    echo "FAIL $name: want $want, got $got"
    failed=1
  fi
}

check "ci helper only" skip .github/ci/x.sh
mkdir -p .github/workflows
check "ci workflow only" skip .github/workflows/ci.yml
check "go sum" run api/go.sum
check "ci helper plus go sum" run .github/ci/x.sh api/go.sum
check "other file" skip api/x.go
# A user's GIT_LITERAL_PATHSPECS must not turn the exclude into a literal path that matches nothing.
# Exported, not a prefix: sh leaves unspecified whether a prefix on a function call is exported. Keep last.
export GIT_LITERAL_PATHSPECS=1
check "literal pathspecs env" run api/go.sum

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo OK
