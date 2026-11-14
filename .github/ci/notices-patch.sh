#!/bin/sh
# Stages PATCH (the notices job's artifact in ci.yml) in the current repo and fails unless it only
# changes or creates the outputs of scripts/third-party-notices.sh as plain, non-executable files.
# notices-push runs it before committing. Self-check: notices-patch.test.sh.
# Usage: sh .github/ci/notices-patch.sh PATCH
set -eu
outputs=$(sh scripts/third-party-notices.sh --outputs)
git apply --index "$1"
# --no-renames: a rename would list only its new name. --summary lists every creation, deletion,
# mode change and symlink; only a new regular file is allowed.
if git diff --cached --no-renames --name-only | grep -vxF "$outputs" ||
  git diff --cached --no-renames --summary | grep -v '^ create mode 100644 '; then
  echo "::error::the notices patch changes files other than the generated ones, or their type or mode" >&2
  exit 1
fi
