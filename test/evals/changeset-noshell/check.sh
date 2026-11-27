#!/usr/bin/env bash
# pass: every file the task names holds the bytes its data implies, every
# other file is byte-identical, no file appeared, `verify.py` ran (and
# succeeded) after the last mutation, and the two closing lines carry
# the counts the fixture implies.
#
# The tree is graded before the transcript: a run that produced the right
# bytes but never verified them has not done the task, and one that
# verified before its last edit proved nothing — each failure names
# which of the two it was.
set -u

[ -n "${SESSION:-}" ] && [ -f "$SESSION" ] || { echo "no session (selftest arm)"; exit 1; }
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
exec python3 "$here/../_lib/changeset_check.py" changeset-noshell "$PWD" "$SESSION" "$here"
