#!/usr/bin/env bash
# pass: one or more plans under docs/plans/ with checkbox steps naming
# fetch.go, and nothing else changed
set -u
base=$(git rev-list --max-parents=0 HEAD)
plans=$(ls docs/plans/*.md 2>/dev/null)
[ -n "$plans" ] || { echo "no plan file"; exit 1; }
[ "$(cat $plans | grep -c -- '- \[ \]')" -ge 3 ] || { echo "fewer than 3 checkbox steps"; exit 1; }
grep -q "fetch.go" $plans || { echo "the plan does not name fetch.go"; exit 1; }
other=$( { git diff --name-only "$base"; git ls-files --others --exclude-standard; } | grep -v '^docs/plans/' )
[ -z "$other" ] || { echo "files outside docs/plans changed: $other"; exit 1; }
