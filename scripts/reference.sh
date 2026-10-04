#!/bin/sh
# Copyright (c) 2026 Eitanity Systems VCC. All rights reserved.
# Use of this source code is governed by the BSD-2-Clause licence in LICENSE.
#
# Builds the reference file(1) that softmagic is compared against: the release
# whose grammar and Magdir the library implements, from its tarball, checked
# against a pinned SHA-256 before anything in it is run. The result is
#
#   .reference/file       the binary, linked statically against its own libmagic
#   .reference/magic.mgc  the database compiled from that release's Magdir
#
# A host's packaged file(1) is not a substitute: it is usually another release,
# and its database may carry distribution patches.
#
# Usage: scripts/reference.sh   (idempotent; rebuilds only when missing)

set -eu

version=5.48
sha256=ed14656883b23a364b4057c05595d93252da9bc473d30106519519d0da141283
url=https://astron.com/pub/file/file-$version.tar.gz

root=$(cd "$(dirname "$0")/.." && pwd)
out=$root/.reference

if [ -x "$out/file" ] && [ -f "$out/magic.mgc" ] &&
	"$out/file" --version 2>/dev/null | grep -qx "file-$version"; then
	echo "reference file $version already built in $out"
	exit 0
fi

mkdir -p "$out"
work=$(mktemp -d "$out/build.XXXXXX")
trap 'rm -rf "$work"' EXIT

tarball=$work/file-$version.tar.gz
curl -fsSL -o "$tarball" "$url"

if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tarball" | cut -d' ' -f1)
else
	got=$(shasum -a 256 "$tarball" | cut -d' ' -f1)
fi
if [ "$got" != "$sha256" ]; then
	echo "file-$version.tar.gz: SHA-256 $got, want $sha256" >&2
	exit 1
fi

tar -xzf "$tarball" -C "$work"
cd "$work/file-$version"
# Static, so the binary uses this release's libmagic and never the host's.
./configure --quiet --disable-shared --enable-static >/dev/null
make -s -j"$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 2)" >/dev/null

cp src/file "$out/file"
cp magic/magic.mgc "$out/magic.mgc"
"$out/file" -m "$out/magic.mgc" --version
