#!/usr/bin/env bash
# §14 ship gate — one unattended moca session fixes a failing test in a
# fixture repo; shipgate_test.go then checks the transcript and the repo.
#
#   make build && bash test/shipgate/run.sh
#
# The model key comes from the repo .env via `env:` indirection (never
# echoed). Overrides: MOCA=<binary> to point at a different build.
set -u
GATE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$GATE/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"

# Go lives under mise on some hosts (go.mod pins it; mise reads it); the
# session's shell tool needs it on PATH.
if ! command -v go >/dev/null 2>&1; then
  gobin="$(mise where go 2>/dev/null || true)"
  [ -n "${gobin:-}" ] && export PATH="$gobin/bin:$PATH"
fi
command -v go >/dev/null 2>&1 || { echo "go not found (mise install?)"; exit 2; }

if [ -f "$REPO/.env" ]; then set -a; . "$REPO/.env"; set +a; fi
if [ -z "${OPENCODE_GO_KEY:-}" ]; then echo "OPENCODE_GO_KEY not set (repo .env missing?)"; exit 2; fi
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }

work=$(mktemp -d "${TMPDIR:-/tmp}/moca-shipgate.XXXXXX")
data=$(mktemp -d "${TMPDIR:-/tmp}/moca-shipgate-data.XXXXXX")
cfg=$(mktemp -d "${TMPDIR:-/tmp}/moca-shipgate-cfg.XXXXXX")

# Keep the artifacts when anything failed (the session file is the evidence
# to inspect); remove them when the gate passed.
cleanup() {
  if [ "${code:-1}" = 0 ] && [ "${check:-1}" = 0 ]; then
    rm -rf "$work" "$data" "$cfg"
  else
    echo "artifacts kept for inspection: $work $data $cfg"
  fi
}
trap cleanup EXIT

cp -r "$GATE/fixture/." "$work/"
(
  cd "$work" || exit 1
  git init -q -b main
  git config user.name "moca shipgate"
  git config user.email "shipgate@example.com"
  git add -A
  git commit -qm "fixture"
)

PROMPT="The test suite is failing. Find and fix the bug, make go test ./... pass, then commit the fix on a new branch named fix/sum."

(
  cd "$work" || exit 1
  XDG_DATA_HOME="$data" XDG_CONFIG_HOME="$cfg" \
    "$MOCA" --config "$GATE/config.jsonc" -p "$PROMPT" \
    > "$data/gate-stdout.txt" 2> "$data/gate-stderr.txt"
)
code=$?

sess=$(ls -t "$data"/moca/sessions/*.jsonl 2>/dev/null | head -1)
if [ -z "$sess" ]; then echo "no session recorded under $data"; exit 2; fi
if command -v rtk >/dev/null 2>&1; then
  echo "rtk:     present (gate 6 requires it to be used)"
else
  echo "rtk:     absent (gate 6 requires the prompt to hide it)"
fi
echo "repo:    $work"
echo "session: $sess"
echo "exit:    $code"
echo "--- stderr tail:"
tail -12 "$data/gate-stderr.txt"

cd "$REPO" || exit 1
go test -tags shipgate ./test/shipgate -count=1 -args -session "$sess" -repo "$work" -exit "$code"
check=$?
exit "$check"
