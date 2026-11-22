#!/usr/bin/env bash
# Build the QuickJS-NG reactor: the one WASM artifact moca ships.
#
#   WASI_SDK_SHA256=<sha256> QJS_REF=<commit-or-tag> ./build.sh [--check]
#
# Two inputs are deliberately NOT guessed here (plan v13, §Artifact/ABI pins the
# SDK; the QuickJS-NG revision is still an open decision):
#
#   WASI_SDK_SHA256  the full digest of wasi-sdk-29.0-x86_64-linux.tar.gz.
#                    The plan records 119,441,678 B and a truncated digest
#                    (87d1d1a2…a0171); the full value must be supplied, because
#                    a truncated digest cannot be verified and this script
#                    refuses to unpack a toolchain it cannot check.
#   QJS_REF          the quickjs-ng commit or tag to build. Pin a commit, not
#                    a branch: the artifact is committed, so two builds must be
#                    byte-identical.
#
# Outputs (committed):
#   qjs-moca-reactor.wasm   the reactor
#   LICENSE.quickjs-ng      copied from the checkout, never retyped by hand
#
# `--check` rebuilds into a temp dir and diffs against the committed artifact:
# that is the "two clean builds byte-identical" gate Task 2 names.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

WASI_TAG="wasi-sdk-29"
WASI_VERSION="29.0"
WASI_TARBALL="wasi-sdk-${WASI_VERSION}-x86_64-linux.tar.gz"
WASI_BYTES=119441678
# Verified 2026-11-23 against the release asset: size and digest both match the
# plan's truncated pin (87d1d1a2…a0171).
WASI_SHA256="87d1d1a2879d139cdc624b968efad3d4a97b8078cdff95e63ac88ecafd1a0171"
QJS_URL="${QJS_URL:-https://github.com/quickjs-ng/quickjs.git}"
OUT="qjs-moca-reactor.wasm"

die() { echo "build.sh: $*" >&2; exit 1; }

QJS_PIN="670492dd342dace0bb7bd6fbfbde8f0bc5651224"

[ "$(uname -s)" = Linux ] && [ "$(uname -m)" = x86_64 ] \
  || die "the pinned SDK is x86_64 Linux only (the plan's build host)"

QJS_REF="${QJS_REF:-$QJS_PIN}"
WASI_SDK_SHA256="${WASI_SDK_SHA256:-$WASI_SHA256}"

work="$(mktemp -d "${TMPDIR:-/tmp}/moca-reactor.XXXXXX")"
trap 'rm -rf "$work"' EXIT

sdk="$work/wasi-sdk"
if [ ! -x "$sdk/bin/clang" ]; then
  [ -f "$work/$WASI_TARBALL" ] || curl -fsSL --retry 3 -o "$work/$WASI_TARBALL" \
    "https://github.com/WebAssembly/wasi-sdk/releases/download/${WASI_TAG}/${WASI_TARBALL}"
  got_bytes=$(wc -c < "$work/$WASI_TARBALL")
  [ "$got_bytes" = "$WASI_BYTES" ] || die "$WASI_TARBALL is $got_bytes bytes, the pin says $WASI_BYTES"
  got_sha=$(sha256sum "$work/$WASI_TARBALL" | cut -d' ' -f1)
  [ "$got_sha" = "$WASI_SDK_SHA256" ] || die "$WASI_TARBALL sha256 is $got_sha, not $WASI_SDK_SHA256"
  tar -xzf "$work/$WASI_TARBALL" -C "$work"
  mv "$work/wasi-sdk-${WASI_VERSION}-x86_64-linux" "$sdk"
fi

git clone --quiet --depth 1 "$QJS_URL" "$work/quickjs" || die "clone $QJS_URL failed"
git -C "$work/quickjs" fetch --quiet --depth 1 origin "$QJS_REF" || die "no such QJS_REF $QJS_REF"
git -C "$work/quickjs" checkout --quiet FETCH_HEAD
qjs_rev=$(git -C "$work/quickjs" rev-parse HEAD)

# The reactor: the C host (moca_host.c) plus quickjs-ng's amalgam, compiled for
# wasm32-wasi as a REACTOR (no _start; the engine calls _initialize). Exports
# and imports are pinned by abi.json and checked by VerifyModule after the
# build, so a compiler that adds an import fails here rather than at runtime.
CC="$sdk/bin/clang"
"$CC" --target=wasm32-wasi --sysroot="$sdk/share/wasi-sysroot" \
  -O2 -flto -fno-exceptions -DQUICKJS_NG=1 \
  -I"$work/quickjs" \
  -mexec-model=reactor -Wl,--export=memory \
  -Wl,--export=qjs_init -Wl,--export=qjs_run -Wl,--export=qjs_destroy \
  -Wl,--export=moca_alloc -Wl,--export=moca_free \
  -Wl,--initial-memory=$((32*1024*1024)) \
  -Wl,--max-memory=$((32*1024*1024)) \
  -o "$work/$OUT" \
  moca_host.c \
  "$work/quickjs/quickjs.c" "$work/quickjs/libregexp.c" "$work/quickjs/libunicode.c" \
  "$work/quickjs/cutils.c" "$work/quickjs/libbf.c"

# Reproducibility: the same inputs must produce the same bytes, or the
# committed artifact cannot be audited against its source.
"$CC" --target=wasm32-wasi --sysroot="$sdk/share/wasi-sysroot" \
  -O2 -flto -fno-exceptions -DQUICKJS_NG=1 \
  -I"$work/quickjs" \
  -mexec-model=reactor -Wl,--export=memory \
  -Wl,--export=qjs_init -Wl,--export=qjs_run -Wl,--export=qjs_destroy \
  -Wl,--export=moca_alloc -Wl,--export=moca_free \
  -Wl,--initial-memory=$((32*1024*1024)) \
  -Wl,--max-memory=$((32*1024*1024)) \
  -o "$work/$OUT.2" \
  moca_host.c \
  "$work/quickjs/quickjs.c" "$work/quickjs/libregexp.c" "$work/quickjs/libunicode.c" \
  "$work/quickjs/cutils.c" "$work/quickjs/libbf.c"
cmp "$work/$OUT" "$work/$OUT.2" || die "two clean builds differ — the artifact is not reproducible"

if [ "${1:-}" = "--check" ]; then
  [ -f "$OUT" ] || die "no committed $OUT to check"
  cmp "$work/$OUT" "$OUT" || die "$OUT does not match a clean build"
  echo "ok: $OUT matches a clean build of quickjs-ng $qjs_rev"
  exit 0
fi

cp "$work/$OUT" "$OUT"
cp "$work/quickjs/LICENSE" LICENSE.quickjs-ng
echo "wrote $OUT ($(wc -c < "$OUT") bytes) from quickjs-ng $qjs_rev"
echo "now add the //go:embed line in internal/codemode/engine.go and run"
echo "  go test ./internal/codemode -run TestReactorIsEmbedded -v"
