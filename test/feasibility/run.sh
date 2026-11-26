#!/usr/bin/env bash
# Phase 6 feasibility: run each route's hand-written programme against a fresh
# copy of a fixture and grade the bytes. No model, no provider, no spend.
#
# Requests are the route's minimum logical work requests, declared in REQ below
# and argued in docs/plans/2026-11-26-changeset-feasibility-phase6.md: the floor
# is three for every route, which is the finding, not a failure of the harness.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$here/../.." && pwd)
evals="$repo/test/evals"
bad=0
printf '%-18s %-8s %-8s %-9s %-10s %s\n' scenario route requests edits artefact result
for scen in changeset-pins changeset-registry; do
  case "$scen" in
    changeset-pins) shell=pins-shell.sh; edits=pins-edits.py ;;
    changeset-registry) shell=registry-shell.sh; edits=registry-edits.py ;;
  esac
  for route in shell edits; do
    work=$(mktemp -d "${TMPDIR:-/tmp}/feas.XXXXXX")
    cp -a "$evals/$scen/fixture/." "$work/"
    if [ "$route" = shell ]; then prog="$here/routes/$shell"; else prog="$here/routes/$edits"; fi
    out=$( cd "$work" && { case "$prog" in
        *.sh) bash "$prog" . 2>&1 ;;
        *) python3 "$prog" 2>&1 ;;
      esac; } )
    rc=$?
    # bytes: every written file equals the reference image, every other file is
    # byte-identical to the fixture, and nothing appeared or vanished.
    diffout=$(python3 "$here/grade.py" "$evals/$scen/fixture" "$evals/$scen/testdata/reference" "$work" 2>&1 | head -3)
    if [ $rc -eq 0 ] && [ -z "$diffout" ]; then res=pass; else res=FAIL; bad=1; fi
    req=3
    n=$(printf '%s\n' "$out" | grep -oE '(edits|pages)=[0-9]+' | head -1)
    printf '%-18s %-8s %-8s %-9s %-10s %s\n' "$scen" "$route" "$req" "${n:-—}" "$(wc -c < "$prog")B" "$res"
    [ "$res" = FAIL ] && printf '  %s\n' "$diffout"
    rm -rf "$work"
  done
  # The codemode route is declared, not executed: the guest has no hash
  # function, so it cannot derive these fixtures' values at all.
  js="$here/routes/${scen#changeset-}.js"
  printf '%-18s %-8s %-8s %-9s %-10s %s\n' "$scen" codemode 3 "—" "$(wc -c < "$js")B" "blocked (no hash in guest)"
done
exit $bad
