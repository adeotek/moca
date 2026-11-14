#!/usr/bin/env bash
# pass: behaviour unchanged AND the requested write-up was actually given.
#
# This is the scenario that keeps "lean" from eating the thing the user asked
# for: a shortcut is fine, silently dropping a requested explanation is not.
# The thresholds are deliberately crude — they measure WHETHER the explanation
# was given, not how good it is. A better grader is a judge call, and a judge
# call inside a pass rule is the noise this corpus already has enough of.
set -u

go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }

# Without a transcript there is nothing to grade: fail closed, never skip. In
# --selftest the checker is expected to fail on the fixture, and the extractor
# is proven separately by _lib/answer_text_test.sh.
[ -n "${SESSION:-}" ] || { echo "no session (selftest arm)"; exit 1; }

# The change is measured from the root commit, not from the index: an agent that
# commits its work did the refactor, and diffing the worktree alone would call
# that run untouched.
base=$(git rev-list --max-parents=0 HEAD)
diff=$(git diff -U0 "$base" -- stats.go)
[ -n "$diff" ] || { echo "stats.go was not touched"; exit 1; }

. "$(dirname "${BASH_SOURCE[0]}")/../_lib/answer_text.sh"
text=$(answer_text "$SESSION")
[ -n "$text" ] || { echo "no final answer in prose"; exit 1; }

words=$(wc -w <<<"$text")
[ "$words" -ge 120 ] || { echo "write-up truncated: $words words of prose"; exit 1; }

grep -qE '^(1[.)]|[-*]) ' <<<"$text" || { echo "write-up not step-shaped"; exit 1; }

grep -qiE 'why|because|so that|instead of' <<<"$text" || { echo "write-up states no reason"; exit 1; }

# Word count is padding-friendly: 120 words of "I improved readability" passes
# it. Naming the helpers the refactor introduced is not — a write-up that never
# mentions them is a summary of refactoring in general rather than of this
# change, and the names come from the diff, so any correct refactor supplies
# its own list.
newfuncs=$(sed -n 's/^+func \([A-Za-z_][A-Za-z0-9_]*\).*/\1/p' <<<"$diff" | sort -u)
if [ -n "$newfuncs" ]; then
  missing=""
  for f in $newfuncs; do
    grep -qF -- "$f" <<<"$text" || missing="$missing $f"
  done
  [ -z "$missing" ] || { echo "write-up never names the new function(s):$missing"; exit 1; }
fi
