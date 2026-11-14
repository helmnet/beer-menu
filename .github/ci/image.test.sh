#!/bin/sh
# Self-check for image.sh: the committed Dockerfiles give digest-pinned refs, and a missing, unpinned
# doubled, CRLF or dash-prefixed FROM fails with no output. The `changes` job in ci.yml runs it.
# Run locally with: sh .github/ci/image.test.sh
set -eu

script="$(dirname "$0")/image.sh"
failed=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp "$script" "$tmp/image.sh"
digest=sha256:$(printf '%064d' 0)

# ok NAME: the committed Dockerfile gives one IMAGE:TAG@sha256:DIGEST ref.
ok() {
  if ! out=$(sh "$script" "$1") || ! printf '%s' "$out" | grep -qxE '[^@]+:[^@]+@sha256:[0-9a-f]{64}'; then
    printf 'FAIL %s: got %s\n' "$1" "$out"
    failed=1
  fi
}

# bad CASE DOCKERFILE: image.sh must fail and print nothing on stdout.
bad() {
  mkdir -p "$tmp/images/$1"
  printf '%s\n' "$2" >"$tmp/images/$1/Dockerfile"
  if out=$(sh "$tmp/image.sh" "$1" 2>/dev/null) || [ -n "$out" ]; then
    printf 'FAIL %s: expected failure, got %s\n' "$1" "$out"
    failed=1
  fi
}

ok gitleaks
ok trivy
bad no-digest "FROM alpine:3"
bad no-tag "FROM alpine@$digest"
bad two-from "FROM a:1@$digest
FROM b:2@$digest"
bad empty ""
bad crlf "FROM a:1@$digest$(printf '\r')"
bad dash "FROM -eX=1:y@$digest"
if sh "$tmp/image.sh" missing >/dev/null 2>&1; then echo "FAIL missing file"; failed=1; fi
if sh "$script" >/dev/null 2>&1; then echo "FAIL no argument"; failed=1; fi

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo OK
