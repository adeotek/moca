#!/usr/bin/env bash
# Skill gate — drives the built moca binary against a scripted provider and
# asserts the self-managed skills contract end to end (SPECS §12). Hermetic:
# no key, no network.
#
#   make build && bash test/skillgate/run.sh
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
cd "$REPO" && go test -tags skillgate -count=1 -race -v ./test/skillgate -args -moca "$MOCA"
