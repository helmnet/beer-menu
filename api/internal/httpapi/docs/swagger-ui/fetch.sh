#!/bin/sh
# Downloads the pinned swagger-ui-dist tarball, checks its npm sha512 and copies the files the
# docs page needs, byte-identical, into this directory. See README.md for upgrade steps.
set -eu

version=5.34.0
integrity=sha512-H6tAIjr3XqE31PbkZl0OFya1ziRLDfTwh3J6OHdumPtPCLfjTnyLeagl/55kFNZcTOz9a9QI9WP0FXldwiGJag==
url="https://registry.npmjs.org/swagger-ui-dist/-/swagger-ui-dist-$version.tgz"

dest=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/pkg.tgz" "$url"
got="sha512-$(openssl dgst -sha512 -binary "$tmp/pkg.tgz" | openssl base64 -A)"
if [ "$got" != "$integrity" ]; then
	echo "fetch.sh: checksum mismatch for $url" >&2
	echo "  want $integrity" >&2
	echo "  got  $got" >&2
	exit 1
fi

mkdir "$tmp/x"
tar -xzf "$tmp/pkg.tgz" -C "$tmp/x"
for f in swagger-ui-bundle.js swagger-ui.css swagger-ui-bundle.js.LICENSE.txt LICENSE NOTICE; do
	cp "$tmp/x/package/$f" "$dest/$f"
done
