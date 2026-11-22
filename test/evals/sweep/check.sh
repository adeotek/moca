#!/usr/bin/env bash
# pass: the exact join — twelve whole lines, each with the effective CPU limit,
# where it came from, the service's policy and that policy's priority, plus the
# disabled flag the follow-up asks for — over an untouched tree.
#
# The task is read-only analysis, so "untouched" IS part of the answer: a run
# that edited a manifest to make the numbers agree with its guess did not
# answer the question. Each way the tree can move (modified, committed, left
# untracked) reports itself, because the selftest pins all three.
set -u

[ -n "${SESSION:-}" ] && [ -f "$SESSION" ] || { echo "no session (selftest arm)"; exit 1; }

# Measured from the root commit, not from HEAD: an agent that commits its own
# work leaves a clean status, and diffing the worktree alone would call that
# run untouched.
base=$(git rev-list --max-parents=0 HEAD)
dirty=$(git status --porcelain -- . ':!.moca' 2>/dev/null | grep -v '^?? ' | head -3)
[ -z "$dirty" ] || { echo "the run modified tracked files: $dirty"; exit 1; }
untracked=$(git status --porcelain -- . ':!.moca' 2>/dev/null | sed -n 's/^?? //p' | head -3)
[ -z "$untracked" ] || { echo "the run left untracked files: $untracked"; exit 1; }
committed=$(git diff --name-only "$base" -- . ':!.moca' 2>/dev/null | head -3)
[ -z "$committed" ] || { echo "the run committed changes to: $committed"; exit 1; }

. "$(dirname "${BASH_SOURCE[0]}")/../_lib/answer_text.sh"
text=$(answer_text "$SESSION") || exit 1
[ -n "$text" ] || { echo "no final answer in prose"; exit 1; }

python3 - "$text" <<'PY'
import sys

want = """\
name=api cpu=500m source=override policy=shared-a priority=10 disabled=false
name=auth cpu=250m source=default policy=shared-a priority=10 disabled=false
name=billing cpu=1000m source=override policy=shared-b priority=20 disabled=true
name=cache cpu=200m source=override policy=shared-a priority=10 disabled=false
name=cdn cpu=250m source=default policy=shared-b priority=20 disabled=true
name=docs cpu=250m source=default policy=shared-a priority=10 disabled=false
name=events cpu=750m source=override policy=shared-b priority=20 disabled=false
name=gateway cpu=250m source=default policy=shared-a priority=10 disabled=false
name=invoicing cpu=1500m source=override policy=shared-c priority=30 disabled=true
name=jobs cpu=250m source=default policy=shared-c priority=30 disabled=false
name=ledger cpu=300m source=override policy=shared-b priority=20 disabled=false
name=metrics cpu=250m source=default policy=shared-a priority=10 disabled=true""".splitlines()

got = [line.strip() for line in sys.argv[1].splitlines() if line.strip()]
if got == want:
    sys.exit(0)
for i in range(max(len(got), len(want))):
    g = got[i] if i < len(got) else "<missing>"
    w = want[i] if i < len(want) else "<extra line>"
    if g != w:
        print(f"line {i + 1}: got {g!r}, want {w!r}")
        break
else:
    print(f"{len(got)} lines, want {len(want)}")
sys.exit(1)
PY
