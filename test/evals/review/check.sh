#!/usr/bin/env bash
# pass: a review that found the planted defects, in the shape /review promises,
# and that changed nothing.
#
# This is the only arm in the corpus that grades a BUILT-IN SKILL's effect on a
# real run. Until v0.8.1 the three leanness skills were pinned only by name
# resolution and help text — their output contract could rot with the suite
# green. Slash commands are TUI-only (ParseInput never reaches cmd/moca), so the
# prompt asks in words and the shape arms measure whether the skill was adopted
# headlessly, which is the behaviour an agent run actually gets.
#
# Four defects are planted in internal/store: a discarded write error that
# returns nil (silent data loss, store.go Save), an off-by-one allocation that
# panics and drops a task (store.go replace), `defer` inside a loop holding
# descriptors open (files.go Export) and an assignment to an uninitialised map
# that panics (store.go Index). The happy-path test passes, which is why they
# survived — the fixture's own test is green on purpose.
set -u

go build ./... >/dev/null 2>&1 || { echo "build fails"; exit 1; }

# A review leaves the tree alone. This is the strongest arm and the cheapest:
# an agent that "helps" by fixing what it found has not reviewed anything.
git status --porcelain | grep -q . && { echo "the review edited the repo:"; git status --porcelain | head -3; exit 1; }

[ -n "${SESSION:-}" ] || { echo "no session (selftest arm)"; exit 1; }
. "$(dirname "${BASH_SOURCE[0]}")/../_lib/answer_text.sh"
text=$(answer_text "$SESSION")
[ -n "$text" ] || { echo "no final answer in prose"; exit 1; }

# Findings: three of four, not four of four. The regexes match how a reviewer
# describes each defect, not one blessed phrasing; requiring all four would turn
# a wording choice into a failed run. A missed defect is printed by name so the
# transcript shows what the model did not see.
miss=""
hit=0
if grep -qiE 'ignore[sd]?|discard|swallow|unchecked|not check|never check' <<<"$text" &&
   grep -qiE 'writefile|write file|save' <<<"$text"; then hit=$((hit+1)); else miss="$miss discarded-write-error"; fi
if grep -qiE 'off[- ]by[- ]one|len\(tasks\)[- ]?1|index out of range|out of range|shorter|panics?' <<<"$text"; then hit=$((hit+1)); else miss="$miss off-by-one"; fi
if grep -qiE 'defer.*(in a |inside a |the )?loop|loop.*defer|file descriptor|descriptor|leak|not closed|never closed' <<<"$text"; then hit=$((hit+1)); else miss="$miss defer-in-loop"; fi
if grep -qiE 'nil map|uninitiali[sz]ed map|assignment to entry|zero[- ]value map' <<<"$text"; then hit=$((hit+1)); else miss="$miss nil-map"; fi
[ "$hit" -ge 3 ] || { echo "review found $hit of 4 planted defects (missed:$miss)"; exit 1; }

# Shape: two of the three markers the skill prescribes. Two, not three, so a
# paraphrase keeps the run — one, because a single bold line is something any
# model produces whether or not it read the skill.
shape=0
grep -qE '\*\*[^ *]+:[0-9]+' <<<"$text" && shape=$((shape+1))
grep -qiE 'must fix|should fix|nice to have' <<<"$text" && shape=$((shape+1))
grep -qiE '^verdict|verdict:' <<<"$text" && shape=$((shape+1))
[ "$shape" -ge 2 ] || { echo "review did not take the /review shape ($shape of 3 markers)"; exit 1; }
