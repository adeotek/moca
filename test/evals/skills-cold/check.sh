#!/usr/bin/env bash
# skills-cold — cold-start recording (v0.7.0 follow-up): the fixture ships NO
# .moca/skills, so anything under it afterwards was recorded by the run.
# The scenario measures whether a session that had to *discover* the repo's
# runner records it, which is the v0.7.0 recording-rate question. It is a
# measurement scenario (a record, not a release gate) — session 1 is asked to
# fix a failing suite and nothing else; recording has to come from the
# decision-point hint.
#
# pass iff:
#   (a) the suite is green through the repo's own runner;
#   (b) at least one auto skill exists and its body names that runner —
#       the session recorded the thing it had to discover;
#   (c) no `(user, …)` rule exists (-p has no /remember).
#
# Variant scoping (-c off): with the feature off no skill can exist, so (b) is
# REPORT-ONLY there — the session must still get the suite green (a), or the
# off arm would score a pass for doing nothing.
set -u
VARIANT="${VARIANT:-on}"
required=1
[ "$VARIANT" = "off" ] && required=0

CALC_STRICT=1 go test ./... >/dev/null 2>&1 || { echo "(a) the suite is not green"; exit 1; }

recorded=0 wrong=0
for f in .moca/skills/moca-*/SKILL.md; do
  [ -f "$f" ] || continue
  grep -q '^auto: true' "$f" || continue
  if grep -Eq 'make test|CALC_STRICT' "$f"; then recorded=1; fi
  grep -qF '(user,' "$f" && { echo "(c) a (user, …) rule exists without /remember"; exit 1; }
done

if [ "$recorded" != 1 ]; then
  if [ "$required" = 1 ]; then
    echo "(b) no auto skill records the runner the session had to discover (recording rate: miss)"
    exit 1
  fi
  echo "report: (b) no auto skill records the runner (feature off)"
fi
