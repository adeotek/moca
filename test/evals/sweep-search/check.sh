#!/usr/bin/env bash
# pass: the retry line, then the same line with the configured connect timeout
# appended — one exact answer, over an untouched tree.
#
# The first turn is an ordinary single search (the policy lives in exactly one
# docs file); the follow-up is one cheap config lookup. Nothing here needs a
# script, which is the point: the codemode pilot's control arm is a task the
# ordinary tools already do well.
set -u

[ -n "${SESSION:-}" ] && [ -f "$SESSION" ] || { echo "no session (selftest arm)"; exit 1; }

base=$(git rev-list --max-parents=0 HEAD)
dirty=$(git status --porcelain -- . ':!.moca' 2>/dev/null | grep -v '^?? ' | head -3)
[ -z "$dirty" ] || { echo "the run modified tracked files: $dirty"; exit 1; }
untracked=$(git status --porcelain -- . ':!.moca' 2>/dev/null | sed -n 's/^?? //p' | head -3)
[ -z "$untracked" ] || { echo "the run left untracked files: $untracked"; exit 1; }
committed=$(git diff --name-only "$base" -- . ':!.moca' 2>/dev/null | head -3)
[ -z "$committed" ] || { echo "the run committed changes to: $committed"; exit 1; }

. "$(dirname "${BASH_SOURCE[0]}")/../_lib/answer_text.sh"
text=$(answer_text "$SESSION") || exit 1

python3 - "$text" <<'PY'
import sys

want = "retries=5 base=400ms cap=4000ms connect=2500ms"
got = " ".join(line.strip() for line in sys.argv[1].splitlines() if line.strip())
if got != want:
    print(f"answer = {got!r}, want {want!r}")
    sys.exit(1)
PY
