#!/usr/bin/env bash
# Context gate — drives the built moca binary against a scripted provider
# and checks the context governor end to end (SPECS §14). Hermetic: no key,
# no network beyond the local test server.
#
#   make build && bash test/contextgate/run.sh
#
# Overrides: MOCA=<binary> to point at a different build.
set -u
GATE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$GATE/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"
if ! command -v go >/dev/null 2>&1; then
  gobin="$(mise where go 2>/dev/null || true)"
  [ -n "${gobin:-}" ] && export PATH="$gobin/bin:$PATH"
fi
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }
cd "$REPO" && go test -tags contextgate -count=1 -race -v ./test/contextgate -args -moca "$MOCA"
