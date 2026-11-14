#!/bin/sh
# Prints the digest-pinned image ref from ci/images/NAME/Dockerfile (outside .github/, where Dependabot
# may not open pull requests). Workflows run scanner
# images from there instead of hardcoding them, so Dependabot can update them. Fails unless the file
# has exactly one FROM line of the form IMAGE:TAG@sha256:DIGEST, so a step never runs an empty or
# unpinned ref. Self-check: image.test.sh.
# Usage: img=$(sh .github/ci/image.sh NAME)
set -eu
if [ "$#" -ne 1 ]; then
  echo "usage: image.sh NAME" >&2
  exit 2
fi
file="$(dirname "$0")/../../ci/images/$1/Dockerfile"
# grep -c: a second FROM line must fail, not pass as a second ref. No trailing whitespace allowed, so a
# CRLF line fails here instead of passing the \r on to docker run. The ref must start [a-z0-9] (as every
# Docker image name does): a leading - would make docker run read it as an option and run the next
# argument as an unpinned image.
if [ ! -f "$file" ] || [ "$(grep -c '^FROM' "$file")" -ne 1 ] ||
  ! ref=$(grep -xE 'FROM[[:space:]]+[a-z0-9][^[:space:]@]*:[^[:space:]@]+@sha256:[0-9a-f]{64}' "$file"); then
  echo "::error::$file must have exactly one FROM IMAGE:TAG@sha256:DIGEST line" >&2
  exit 1
fi
printf '%s\n' "$ref" | awk '{print $2}'
