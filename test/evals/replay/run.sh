#!/usr/bin/env bash
# Trajectory-replay gate: build the binary, then replay one synthetic session
# twice on it — identical request bytes, zero prefix breaks, no divergence.
set -euo pipefail
REPO="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$REPO"
make build >/dev/null
go test -tags replay ./test/evals/replay -count=1
