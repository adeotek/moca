#!/usr/bin/env bash
# pass: the build and tests are green, the two known sizes render exactly as
# internal/human renders them, AND the formatting was reused rather than
# re-implemented.
#
# Arm 1 pins the RENDERING, not a shape: a regex like [0-9.]+(B|KiB|MiB) is
# satisfied by "1536 B", so a duplicate formatter emitting raw bytes would pass
# a shape check. Only something that matches human.Bytes's own contract prints
# "1.5 KiB" and "2.0 MiB".
# Arm 2 catches the duplicate that DOES reproduce the spelling — it must divide
# by 1024 somewhere, which nothing outside internal/human has any reason to do.
# It keys on the arithmetic rather than on the unit strings so that help text
# ("show sizes in KiB") cannot fail an agent that reused the helper.
# A deliberately obfuscated duplicate (2*512, a table of divisors) slips past
# this arm; the scenario measures the ordinary shortcut, not evasion. The
# helper arm below is the backstop for a duplicate that is not
# arithmetic-shaped at all.
# The last arm is not the reuse gate — report already calls human.Bytes — it
# only catches a deleted helper.
set -u

go build ./... >/dev/null 2>&1 || { echo "build fails"; exit 1; }
go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
# generated, not committed: the fixture stays source-only and these two sizes
# are the checker's
head -c 1536 /dev/zero > "$tmp/tiny.bin"
head -c 2097152 /dev/zero > "$tmp/big.bin"

out=$(go run ./cmd/inventory list --sizes "$tmp" 2>&1) || { echo "--sizes fails: $out"; exit 1; }
grep -q '1\.5 KiB' <<<"$out" || { echo "tiny.bin not rendered as 1.5 KiB: $out"; exit 1; }
grep -q '2\.0 MiB' <<<"$out" || { echo "big.bin not rendered as 2.0 MiB: $out"; exit 1; }

bad=$(grep -rnE '\b(1024|1048576)\b|<<[[:space:]]*[0-9(]|>>[[:space:]]*[0-9(]' --include='*.go' --exclude='*_test.go' . | grep -v '^\./internal/human/' || true)
[ -z "$bad" ] || { echo "re-implemented size formatting (an IEC divisor outside internal/human):"; echo "$bad"; exit 1; }

# Production only: a duplicate formatter whose only human.Bytes reference sits
# in a _test.go file is exactly the leak arm 4 exists for — counting test files
# let it pass while the shipped code never touched the helper.
grep -rq 'human\.Bytes(' ./cmd ./internal --include='*.go' --exclude='*_test.go' || { echo "existing helper not called"; exit 1; }
