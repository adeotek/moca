#!/usr/bin/env bash
# Goal gate — drives the built moca TUI in tmux against a scripted provider
# and asserts the /goal contract end to end (SPECS §14). Hermetic: no key,
# no network. Requires tmux (a clear prerequisite failure, never a skip).
#
#   make build && bash test/goalgate/run.sh
#
# Overrides: MOCA=<binary> · GATE_DIR=<scratch dir, kept on failure>
set -u
GATE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$GATE/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"
GATE_DIR="${GATE_DIR:-${TMPDIR:-/tmp}/moca-goalgate-$$}"
if ! command -v go >/dev/null 2>&1; then
  gobin="$(mise where go 2>/dev/null || true)"
  [ -n "${gobin:-}" ] && export PATH="$gobin/bin:$PATH"
fi
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }
command -v tmux >/dev/null 2>&1 || { echo "goalgate requires tmux (install it; the gate never skips silently)"; exit 2; }
mkdir -p "$GATE_DIR"
echo "goalgate: artifacts under $GATE_DIR"
cd "$REPO" && GATE_DIR="$GATE_DIR" go test -tags goalgate -count=1 -v ./test/goalgate -args -moca "$MOCA" -gatedir "$GATE_DIR"
