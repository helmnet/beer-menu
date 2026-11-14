#!/bin/sh
# Decides which CI checks a change needs. Reads repo-relative paths on stdin, one per line, or the
# single word ALL to force every check. Prints api=, web=, guide=, services= and notices= lines
# for $GITHUB_OUTPUT. guide=true means the user guide (guide/) or its color source may have
# changed. notices=true means scripts/third-party-notices.sh may change its output; the
# pre-commit hook (.githooks/pre-commit) reuses that answer for staged files.
# Always exits 0: what changed must never fail the run. The case below is the single source for
# which files trigger which checks; a new file the builds depend on must be added here in the same
# change. Self-check: changed-sides.test.sh.
api=false
web=false
guide=false
notices=false
while IFS= read -r path || [ -n "$path" ]; do
  case $path in
    *.md) ;;
    guide/*) guide=true ;;
    # Inputs and outputs of scripts/third-party-notices.sh.
    api/go.mod | api/go.sum | api/internal/httpapi/docs/swagger-ui/* | api/THIRD_PARTY_NOTICES | api/LICENSE)
      api=true notices=true ;;
    api/*) api=true ;;
    web/package.json | web/package-lock.json | web/THIRD_PARTY_NOTICES | web/LICENSE) web=true notices=true ;;
    # The guide reads its color tokens from the admin app's stylesheet.
    web/src/pages/AdminPage.css) web=true guide=true ;;
    web/* | .tool-versions) web=true ;;
    LICENSE | scripts/third-party-notices.sh) notices=true ;;
    .github/workflows/ci.yml | .github/ci/*) api=true web=true guide=true notices=true ;;
    .github/workflows/pages.yml) guide=true ;;
    # ci/images/*: scanner image pins; a Trivy bump must run the images job.
    ci/images/* | docker-compose.yaml) api=true web=true ;;
    # A root file literally named ALL also forces every check; harmless.
    ALL) api=true web=true guide=true notices=true ;;
    *) ;;
  esac
done

services=
if [ "$api" = true ]; then services=api; fi
if [ "$web" = true ]; then services="${services:+$services }web"; fi
printf 'api=%s\nweb=%s\nguide=%s\nservices=%s\nnotices=%s\n' "$api" "$web" "$guide" "$services" "$notices"
exit 0
