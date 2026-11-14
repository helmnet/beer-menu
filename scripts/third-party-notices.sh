#!/bin/sh
# Writes api/THIRD_PARTY_NOTICES (Go modules and Go standard library compiled into the API binary,
# plus the Swagger UI files it embeds) and web/THIRD_PARTY_NOTICES (npm packages bundled into the
# web app), and copies the root LICENSE into api/ and web/, the Docker build contexts.
# Needs go and web/node_modules (npm ci). Re-run after any dependency change and commit the result:
#   sh scripts/third-party-notices.sh
# The pre-commit hook (.githooks/pre-commit) and CI run it too; --outputs prints the files it writes.
# Output is sorted and has no timestamps, so a re-run with the same dependencies changes nothing.
# Fails if a package has no license file or no known license id.
# -f: no globbing, since unquoted names below come from downloaded packages.
set -euf
export LC_ALL=C

# Single list of generated files, for the hook and CI to stage, diff or push.
outputs='api/THIRD_PARTY_NOTICES api/LICENSE web/THIRD_PARTY_NOTICES web/LICENSE'
if [ "${1-}" = --outputs ]; then
	printf '%s\n' $outputs
	exit 0
fi

root=$(cd "$(dirname "$0")/.." && pwd)
trap 'rm -f "$root/api/THIRD_PARTY_NOTICES.tmp" "$root/web/THIRD_PARTY_NOTICES.tmp"' EXIT

fail() {
	echo "third-party-notices.sh: $*" >&2
	exit 1
}

# notice NAME VERSION LICENSE DIR: prints one entry with every license, notice and patents file
# at the top of DIR (also names like swagger-ui-bundle.js.LICENSE.txt), verbatim.
notice() {
	files=$(ls "$4" | grep -iE '(^|\.)(licen[cs]e|copying|notice|patents)') || true
	printf '%s\n' "$files" | grep -qiE '(^|\.)(licen[cs]e|copying)' || fail "no license file for $1 in $4"
	[ -n "$3" ] || fail "no license id for $1"
	printf '================================================================================\n'
	printf '%s %s\nLicense: %s\n' "$1" "$2" "$3"
	for f in $files; do
		# A symlinked license could pull any local file into the committed, published notices.
		[ -f "$4/$f" ] && [ ! -L "$4/$f" ] || fail "$f of $1 is not a regular file"
		printf '\n----- %s -----\n\n' "$f"
		cat "$4/$f"
		printf '\n'
	done
	printf '\n'
}

# go_license MODULE: the SPDX id of the module's own LICENSE file, checked by hand.
go_license() {
	case $1 in
	github.com/google/go-tpm) echo Apache-2.0 ;;
	github.com/dustin/go-humanize | github.com/fxamacker/cbor/v2 | github.com/golang-jwt/jwt/v5 | \
		github.com/go-viper/mapstructure/v2 | github.com/philhofer/fwd | github.com/tinylib/msgp | \
		github.com/x448/float16) echo MIT ;;
	github.com/google/uuid | github.com/go-webauthn/webauthn | github.com/go-webauthn/x | \
		github.com/remyoudompheng/bigfft | golang.org/x/* | modernc.org/libc | modernc.org/mathutil | \
		modernc.org/memory | modernc.org/sqlite) echo BSD-3-Clause ;;
	*) fail "unknown license for Go module $1: read its license files and add it to go_license" ;;
	esac
}

api_notices() {
	cd "$root/api"
	goroot=$(go env GOROOT)
	if [ ! -f "$goroot/LICENSE" ]; then
		# Some distro Go packages drop LICENSE: switch to the upstream build of the same version
		# (go1.27.3-X:nodwarf5 -> go1.27.3), which go downloads once into the module cache.
		v=$(go env GOVERSION)
		export GOTOOLCHAIN="${v%%[!a-z0-9.]*}"
		goroot=$(go env GOROOT)
	fi
	[ -f "$goroot/LICENSE" ] || fail "no LICENSE in GOROOT $goroot (some distro Go packages drop it);" \
		"re-run with GOTOOLCHAIN=go<version> to use an upstream Go"
	notice "Go standard library" "go$(go list -m -f '{{.GoVersion}}')" BSD-3-Clause "$goroot"
	# Same target as the Dockerfile build, so platform-specific modules match the image.
	# Captured before sorting: sh has no pipefail, so a failing go list would be hidden by sort.
	mods=$(CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go list -deps \
		-f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' ./cmd/...)
	mods=$(printf '%s\n' "$mods" | sort -u)
	while read -r path version dir; do
		notice "$path" "$version" "$(go_license "$path")" "$dir"
	done <<-END
		$mods
	END
	docs=internal/httpapi/docs/swagger-ui
	notice swagger-ui-dist "$(sed -n 's/^version=//p' "$docs/fetch.sh")" Apache-2.0 "$docs"
}

web_notices() {
	cd "$root/web"
	# Runtime packages only; the first line is web/ itself. Fails if node_modules is missing.
	dirs=$(npm ls --omit=dev --all --parseable)
	# shortcut: type-only packages are named by hand; add any new one npm ls lists but Vite never bundles.
	dirs=$(printf '%s\n' "$dirs" | tail -n +2 | grep -v -e '/node_modules/@types/' -e '/node_modules/csstype$' | sort)
	# One line per package; license last, so read keeps an SPDX expression like "(MIT OR Apache-2.0)" whole.
	# package.json comes from downloaded packages: read as data, and fail on any field that would
	# break the line format below (a newline could add an entry pointing at any local directory).
	# shellcheck disable=SC2086 # npm package paths hold no spaces
	pkgs=$(node -e 'for (const d of process.argv.slice(1)) {
		const p = JSON.parse(require("fs").readFileSync(d + "/package.json", "utf8")), f = [d, p.name, p.version, p.license];
		if (!f.every((s) => typeof s === "string" && /^[^\r\n]+$/.test(s)) || /\s/.test(f.slice(0, 3).join("")))
			throw new Error("unexpected name, version or license in " + d + "/package.json");
		console.log(...f) }' $dirs)
	while read -r dir name version license; do
		notice "$name" "$version" "$license" "$dir"
	done <<-END
		$pkgs
	END
}

# write PART IMAGE: regenerates PART/THIRD_PARTY_NOTICES and PART/LICENSE.
write() {
	out="$root/$1/THIRD_PARTY_NOTICES"
	{
		printf 'Third-party software in the %s image\n\n' "$2"
		printf 'Beer Menu itself is licensed under GPL-3.0-or-later; see LICENSE. The software below is\n'
		printf 'distributed with it under its own license. Generated by scripts/third-party-notices.sh;\n'
		printf 'do not edit by hand.\n\n'
		"$1_notices"
	} >"$out.tmp"
	mv "$out.tmp" "$out"
	cp "$root/LICENSE" "$root/$1/LICENSE"
}

write api beer-menu-api
write web beer-menu-web
