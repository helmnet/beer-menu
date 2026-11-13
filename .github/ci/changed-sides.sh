#!/bin/sh
# Decides which CI checks a change needs. Reads repo-relative paths on stdin, one per line, or the
# single word ALL to force every check. Prints api=, web= and services= lines for $GITHUB_OUTPUT.
# Always exits 0: what changed must never fail the run. The case below is the single source for
# which files trigger which checks; a new file the builds depend on must be added here in the same
# change. Self-check: changed-sides.test.sh.
api=false
web=false
while IFS= read -r path || [ -n "$path" ]; do
  case $path in
    *.md) ;;
    api/*) api=true ;;
    web/* | .tool-versions) web=true ;;
    .github/workflows/ci.yml | .github/ci/* | docker-compose.yaml) api=true web=true ;;
    # A root file literally named ALL also forces every check; harmless.
    ALL) api=true web=true ;;
    *) ;;
  esac
done

services=
if [ "$api" = true ]; then services=api; fi
if [ "$web" = true ]; then services="${services:+$services }web"; fi
printf 'api=%s\nweb=%s\nservices=%s\n' "$api" "$web" "$services"
exit 0
