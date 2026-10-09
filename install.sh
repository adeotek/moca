#!/usr/bin/env bash
# moca installer — Linux.
#
# Downloads a release package (the latest one, or --version <v>), verifies it
# against the release's checksums.txt, and installs the moca binary.
#
# Re-running is safe and idempotent: an existing installation is updated in
# place — the script never fails because moca is already installed. The
# default target is the directory of the moca already on PATH, else
# ~/.local/bin.
#
# usage:
#   ./install.sh [--version <vX.Y.Z>] [--dir <path>]
#
# env: MOCA_REPO (default adeotek/moca), MOCA_API_BASE (default the GitHub API)
set -euo pipefail

REPO="${MOCA_REPO:-adeotek/moca}"
API_BASE="${MOCA_API_BASE:-https://api.github.com}"
VERSION=""
DIR=""

die() { printf 'moca: %s\n' "$*" >&2; exit 1; }
note() { printf '%s\n' "$*"; }

usage() {
	cat <<'EOF'
moca installer — Linux

usage: install.sh [--version <vX.Y.Z>] [--dir <path>]

  --version <v>   install this release instead of the latest one
  --dir <path>    install into this directory (default: the directory of
                  the moca already on PATH, else ~/.local/bin)
  -h, --help      this help

Downloads the release package for this platform, verifies it against the
release's checksums.txt, and updates an existing installation in place.
EOF
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version | -v)
		[ $# -ge 2 ] || die "--version needs a value"
		VERSION="$2"
		shift 2
		;;
	--dir | -d)
		[ $# -ge 2 ] || die "--dir needs a value"
		DIR="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown argument: $1 (try --help)"
		;;
	esac
done

[ "$(uname -s)" = "Linux" ] || die "this installer is for Linux — on Windows use install.ps1; on macOS use 'go install github.com/${REPO}/cmd/moca@latest'"

case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) die "unsupported architecture: $(uname -m) (linux/amd64 and linux/arm64 are published)" ;;
esac

# Tolerate both "0.2.0-beta" and "v0.2.0-beta".
case "$VERSION" in
"" | v*) ;;
*) VERSION="v$VERSION" ;;
esac

fetch() { # fetch <url> <outfile>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		die "needs curl or wget"
	fi
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ -n "$VERSION" ]; then
	note "moca installer: release $VERSION, linux/$ARCH"
	RELEASE_URL="$API_BASE/repos/$REPO/releases/tags/$VERSION"
else
	note "moca installer: latest release, linux/$ARCH"
	RELEASE_URL="$API_BASE/repos/$REPO/releases?per_page=1"
fi
fetch "$RELEASE_URL" "$TMP/release.json" || die "could not fetch release metadata ($RELEASE_URL)"

TAG="$(grep -o '"tag_name": *"[^"]*"' "$TMP/release.json" | head -n 1 | sed 's/.*"\([^"]*\)"$/\1/')"
[ -n "$TAG" ] || die "could not parse the release tag (unexpected API response)"
ASSET="moca-${TAG}-linux-${ARCH}.tar.gz"

URLS="$(grep -o '"browser_download_url": *"[^"]*"' "$TMP/release.json" | sed 's/.*"\([^"]*\)"$/\1/')"
PACKAGE_URL="$(printf '%s\n' "$URLS" | grep -- "/${ASSET}\$" | head -n 1 || true)"
SUMS_URL="$(printf '%s\n' "$URLS" | grep -- '/checksums.txt$' | head -n 1 || true)"
[ -n "$PACKAGE_URL" ] || die "release $TAG has no package for linux/$ARCH"

note "downloading $ASSET…"
fetch "$PACKAGE_URL" "$TMP/$ASSET" || die "download failed: $PACKAGE_URL"

if [ -n "$SUMS_URL" ]; then
	note "verifying checksum…"
	fetch "$SUMS_URL" "$TMP/checksums.txt" || die "could not download checksums.txt"
	grep -q -- "  ${ASSET}\$" "$TMP/checksums.txt" || die "checksums.txt has no entry for ${ASSET}"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$TMP" && grep -- "  ${ASSET}\$" checksums.txt | sha256sum -c - >/dev/null 2>&1) || die "checksum mismatch — aborting (nothing was installed)"
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$TMP" && grep -- "  ${ASSET}\$" checksums.txt | shasum -a 256 -c - >/dev/null 2>&1) || die "checksum mismatch — aborting (nothing was installed)"
	else
		note "warning: sha256sum/shasum not found — skipping checksum verification"
	fi
else
	note "warning: release $TAG has no checksums.txt — skipping checksum verification"
fi

tar -xzf "$TMP/$ASSET" -C "$TMP" moca || die "could not extract $ASSET"
[ -f "$TMP/moca" ] || die "the package contains no moca binary"

if [ -z "$DIR" ]; then
	if EXISTING="$(command -v moca 2>/dev/null)" && [ -n "$EXISTING" ]; then
		RESOLVED="$(readlink -f -- "$EXISTING" 2>/dev/null || printf '%s' "$EXISTING")"
		DIR="$(dirname -- "$RESOLVED")"
	else
		DIR="$HOME/.local/bin"
	fi
fi
if [ -e "$DIR" ] && [ ! -w "$DIR" ]; then
	die "no write permission for $DIR — re-run with sudo, or pass --dir <path>"
fi
mkdir -p "$DIR" || die "could not create $DIR"

if [ -e "$DIR/moca" ]; then
	note "updating existing installation at $DIR/moca"
else
	note "installing to $DIR/moca"
fi

# Write beside the target and rename: a running moca keeps the old inode and
# the swap stays atomic (writing onto a running binary fails with ETXTBSY).
cp -- "$TMP/moca" "$DIR/.moca.new.$$"
chmod 0755 "$DIR/.moca.new.$$"
mv -f -- "$DIR/.moca.new.$$" "$DIR/moca"

"$DIR/moca" --version || note "warning: $DIR/moca --version failed"
note "installed moca ${TAG} → $DIR/moca"

case ":$PATH:" in
*":$DIR:"*) ;;
*) note "note: $DIR is not on PATH — add it to use 'moca' directly" ;;
esac
