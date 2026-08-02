#!/usr/bin/env bash
# Build release binaries and package them with the files redistribution requires.
#
# Apache-2.0 asks anyone redistributing this software to include the license and
# carry the NOTICE forward, and our MIT/BSD dependencies require their own
# copyright notices be preserved in binaries that link them. Packaging that by
# hand is how those files get forgotten, so this script always includes them.
#
# Usage: scripts/make-release.sh [version]
#        scripts/make-release.sh          # reads version from internal/version

set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-$(grep -oP 'const Version = "\K[^"]+' internal/version/version.go)}"
OUT="os-builds"

# Files that must travel with every binary.
LEGAL=(LICENSE NOTICE THIRD-PARTY-NOTICES)

for f in "${LEGAL[@]}"; do
	[ -f "$f" ] || { echo "missing required file: $f" >&2; exit 1; }
done

echo "Packaging ls-horizons v${VERSION}"

# Regenerate notices so a dependency change can't silently drift out of sync.
./scripts/gen-notices.sh > THIRD-PARTY-NOTICES
echo "  regenerated THIRD-PARTY-NOTICES"

build() {
	local goos="$1" goarch="$2" subdir="$3" binname="$4" archive="$5"

	# CGO_ENABLED=0 keeps the binary statically linked, as the README promises.
	# Without it the native target inherits cgo from the environment, links
	# against the build host's glibc, and then refuses to start on any machine
	# with an older one. The cross-compiled targets disable cgo on their own,
	# so pinning it here just makes every target behave the same way.
	#
	# -trimpath rewrites embedded source paths to their module-relative form.
	# Without it every release binary carries the absolute paths of whoever's
	# machine built it, which is both noise and a small privacy leak, and it
	# makes two builds of the same commit differ for no useful reason.
	#
	# -s -w drops the symbol table and DWARF, cutting roughly a third of the
	# size. Panic tracebacks survive this: Go resolves them from its own
	# pclntab, not from DWARF. Only external debuggers lose out, and they
	# should be pointed at a locally built binary anyway.
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath -ldflags "-s -w" -o "$OUT/$subdir/$binname" ./cmd/ls-horizons
	echo "  built $OUT/$subdir/$binname ($(du -h "$OUT/$subdir/$binname" | cut -f1))"

	# Stage a versioned directory so extracting doesn't scatter LICENSE and
	# NOTICE into the user's working directory. The platform belongs in that
	# directory name as well as in the archive name: without it, every platform
	# unpacks to the same path, and anyone fetching two of them into one folder
	# silently overwrites one binary with another for a different architecture.
	# Naming it after the archive keeps what you download and what you get in
	# sync.
	local top="ls-horizons-v${VERSION}-${subdir}"
	local stage
	stage="$(mktemp -d)"
	local dir="$stage/$top"
	mkdir -p "$dir"
	cp "$OUT/$subdir/$binname" "$dir/"
	cp "${LEGAL[@]}" "$dir/"

	rm -f "$OUT/$archive"
	case "$archive" in
		*.zip) (cd "$stage" && zip -qr "$OLDPWD/$OUT/$archive" "$top") ;;
		*)     tar czf "$OUT/$archive" -C "$stage" "$top" ;;
	esac
	rm -rf "$stage"
	echo "  packaged $OUT/$archive"
}

build linux  amd64 linux-amd64   ls-horizons     "ls-horizons-v${VERSION}-linux-amd64.tar.gz"
build darwin arm64 mac-arm       ls-horizons     "ls-horizons-v${VERSION}-mac-arm.tar.gz"
build windows amd64 windows-amd64 ls-horizons.exe "ls-horizons-v${VERSION}-windows-amd64.zip"

echo
echo "Archive contents:"
tar tzf "$OUT/ls-horizons-v${VERSION}-linux-amd64.tar.gz" | sed 's/^/  /'
