#!/bin/sh
# Prints the digest-pinned image ref from .github/ci/images/NAME/Dockerfile. Workflows run scanner
# images from there instead of hardcoding them, so Dependabot can update them. Fails unless the file
# has exactly one FROM line of the form IMAGE:TAG@sha256:DIGEST, so a step never runs an empty or
# unpinned ref. Self-check: image.test.sh.
# Usage: img=$(sh .github/ci/image.sh NAME)
set -eu
if [ "$#" -ne 1 ]; then
  echo "usage: image.sh NAME" >&2
  exit 2
fi
file="$(dirname "$0")/images/$1/Dockerfile"
# grep -c: a second FROM line must fail, not pass as a second ref.
if [ ! -f "$file" ] || [ "$(grep -c '^FROM' "$file")" -ne 1 ] ||
  ! ref=$(grep '^FROM' "$file" | grep -xE 'FROM[[:space:]]+[^[:space:]@]+:[^[:space:]@]+@sha256:[0-9a-f]{64}[[:space:]]*'); then
  echo "::error::$file must have exactly one FROM IMAGE:TAG@sha256:DIGEST line" >&2
  exit 1
fi
ref=$(printf '%s\n' "$ref" | awk '{print $2}')
printf '%s\n' "$ref"
