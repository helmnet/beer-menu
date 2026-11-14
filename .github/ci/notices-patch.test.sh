#!/bin/sh
# Self-check for notices-patch.sh: builds patches in a throwaway repo and checks which ones it
# accepts. The `changes` job in ci.yml runs it. Run locally with: sh .github/ci/notices-patch.test.sh
set -eu

here=$(cd "$(dirname "$0")" && pwd)
repo=$(mktemp -d)
trap 'rm -rf "$repo"' EXIT
cd "$repo"
git init -q
mkdir -p api web scripts
cp "$here/../../scripts/third-party-notices.sh" scripts/
seq 1 50 >LICENSE
seq 1 50 >other
for f in api/THIRD_PARTY_NOTICES web/THIRD_PARTY_NOTICES web/LICENSE; do echo old >"$f"; done
git add -A
git -c user.name=t -c user.email=t@t commit -qm base
failed=0

# check NAME WANT: WANT is ok or fail. The working tree change for the patch is already made.
check() {
  git add -A
  git diff --cached --binary >.git/test.patch
  git reset -q --hard
  if sh "$here/notices-patch.sh" .git/test.patch >/dev/null 2>&1; then got=ok; else got=fail; fi
  git reset -q --hard
  if [ "$got" != "$2" ]; then
    printf 'FAIL %s: want %s, got %s\n' "$1" "$2" "$got"
    failed=1
  fi
}

echo new >>web/THIRD_PARTY_NOTICES && check "modify output" ok
cp LICENSE api/LICENSE && check "create output" ok
echo new >>other && check "other file" fail
git mv other api/LICENSE && check "rename onto missing output" fail
git rm -q web/LICENSE && check "delete output" fail
chmod +x web/LICENSE && check "make output executable" fail
rm web/LICENSE && ln -s ../other web/LICENSE && check "output becomes symlink" fail
ln -s ../other api/LICENSE && check "new output is symlink" fail

if [ "$failed" -ne 0 ]; then exit 1; fi
echo "notices-patch: all checks passed"
