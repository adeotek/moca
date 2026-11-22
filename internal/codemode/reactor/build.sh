#!/usr/bin/env bash
# Build the QuickJS-NG reactor: the one WASM artifact moca ships.
#
#   ./build.sh                 # build and install the artifact
#   ./build.sh --check         # rebuild twice, then diff against the committed artifact
#
# Both inputs are pinned below and were verified on 2026-11-23:
#   wasi-sdk 29.0 — 119,441,678 B, sha256 87d1d1a2…a0171, matching the plan's
#                   truncated digest. The release TAG is `wasi-sdk-29`; the plan
#                   wrote `wasi-sdk-29.0`, which 404s.
#   quickjs-ng    — v0.9.0, commit 670492dd342dace0bb7bd6fbfbde8f0bc5651224.
#
# Outputs (committed):
#   qjs-moca-reactor.wasm   the reactor, embedded by internal/codemode/engine.go
#   LICENSE.quickjs-ng      copied from the checkout, never retyped by hand
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

WASI_TAG="wasi-sdk-29"
WASI_VERSION="29.0"
WASI_TARBALL="wasi-sdk-${WASI_VERSION}-x86_64-linux.tar.gz"
WASI_BYTES=119441678
WASI_SHA256="87d1d1a2879d139cdc624b968efad3d4a97b8078cdff95e63ac88ecafd1a0171"
QJS_PIN="670492dd342dace0bb7bd6fbfbde8f0bc5651224"
QJS_URL="${QJS_URL:-https://github.com/quickjs-ng/quickjs.git}"
OUT="qjs-moca-reactor.wasm"

die() { echo "build.sh: $*" >&2; exit 1; }

[ "$(uname -s)" = Linux ] && [ "$(uname -m)" = x86_64 ] \
  || die "the pinned SDK is x86_64 Linux only (the artifact it produces is portable)"

work="$(mktemp -d "${TMPDIR:-/tmp}/moca-reactor.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# ---- toolchain -----------------------------------------------------------
sdk="$work/wasi-sdk"
if [ -n "${WASI_SDK_DIR:-}" ] && [ -x "${WASI_SDK_DIR}/bin/clang" ]; then
  sdk="$WASI_SDK_DIR"                       # reuse an unpacked SDK
else
  curl -fsSL --retry 3 -o "$work/$WASI_TARBALL" \
    "https://github.com/WebAssembly/wasi-sdk/releases/download/${WASI_TAG}/${WASI_TARBALL}"
  got_bytes=$(wc -c < "$work/$WASI_TARBALL")
  [ "$got_bytes" = "$WASI_BYTES" ] || die "$WASI_TARBALL is $got_bytes bytes, the pin says $WASI_BYTES"
  got_sha=$(sha256sum "$work/$WASI_TARBALL" | cut -d' ' -f1)
  [ "$got_sha" = "$WASI_SHA256" ] || die "$WASI_TARBALL sha256 is $got_sha, not $WASI_SHA256"
  tar -xzf "$work/$WASI_TARBALL" -C "$work"
  mv "$work/wasi-sdk-${WASI_VERSION}-x86_64-linux" "$sdk"
fi

git clone --quiet --depth 1 "$QJS_URL" "$work/quickjs" || die "clone $QJS_URL failed"
git -C "$work/quickjs" fetch --quiet --depth 1 origin "$QJS_PIN" || die "no such revision $QJS_PIN"
git -C "$work/quickjs" checkout --quiet FETCH_HEAD
qjs_rev=$(git -C "$work/quickjs" rev-parse HEAD)

# ---- compile -------------------------------------------------------------
# The reactor is initialized, never started: -mexec-model=reactor yields
# _initialize and no _start. Three flags matter, each learned the hard way:
#   * no `--allow-undefined`: it makes wasm-ld satisfy every undefined symbol as
#     an import instead of linking libc, which collapses the whole import
#     section into one synthetic module named after the output file;
#   * no quickjs-libc.c: its os/std helpers pull in exactly the filesystem
#     imports abi.json forbids;
#   * no `--export=memory`: wasm-ld defines memory rather than exporting a
#     symbol, and it exports it anyway;
#   * `-g0 -Wl,--strip-debug`: DWARF carries the build's absolute paths, so an
#     unstripped artifact is only reproducible on the machine that built it.
#     Stripping makes two builds in two temp dirs byte-identical (and saves
#     ~220 KiB in the embedded binary);
#   * the prefix maps: quickjs embeds its own source paths in __FILE__, so
#     without them two builds in two directories differ by a few dozen bytes.
compile() {
  "$sdk/bin/clang" --target=wasm32-wasip1 --sysroot="$sdk/share/wasi-sysroot" \
    -O2 -fno-exceptions -g0 -I"$work/quickjs" \
    -ffile-prefix-map="$work"=. -fmacro-prefix-map="$work"=. \
    -mexec-model=reactor -Wl,--strip-debug \
    -Wl,--export=qjs_init -Wl,--export=qjs_run -Wl,--export=qjs_destroy \
    -Wl,--export=moca_alloc -Wl,--export=moca_free \
    -Wl,--initial-memory=$((32*1024*1024)) -Wl,--max-memory=$((32*1024*1024)) \
    -Wl,-z,stack-size=$((2*1024*1024)) \
    -o "$1" \
    moca_host.c \
    "$work/quickjs/quickjs.c" "$work/quickjs/libregexp.c" "$work/quickjs/libunicode.c" \
    "$work/quickjs/cutils.c" "$work/quickjs/libbf.c"
}

# The output BASENAME is part of the artifact: wasm-ld writes it into the module
# name section. So both builds use the same basename in different directories —
# naming them a.wasm/b.wasm would make every comparison fail for a reason that
# has nothing to do with reproducibility.
mkdir -p "$work/a" "$work/b"
compile "$work/a/$OUT"
compile "$work/b/$OUT"
cmp "$work/a/$OUT" "$work/b/$OUT" || die "two clean builds differ — the artifact is not reproducible"

if [ "${1:-}" = "--check" ]; then
  [ -f "$OUT" ] || die "no committed $OUT to check"
  cmp "$work/a/$OUT" "$OUT" || die "$OUT does not match a clean build"
  echo "ok: $OUT matches a clean build of quickjs-ng $qjs_rev ($(wc -c < "$OUT") bytes)"
  exit 0
fi

cp "$work/a/$OUT" "$OUT"
cp "$work/quickjs/LICENSE" LICENSE.quickjs-ng
echo "wrote $OUT ($(wc -c < "$OUT") bytes) from quickjs-ng $qjs_rev"
echo "next: go test ./internal/codemode -run TestReactorIsEmbedded -v"
