#!/usr/bin/env bash
# This test is only about the SOURCING: a scenario-shaped consumer must end up
# with `carries_injection` defined, and a missing scan must be a hard error
# rather than a silent pass. The scan's own verdicts are pinned by
# refusal_filter_test.sh, which sources the filter directly — sourcing the body
# for them was cwd-dependent (the body runs its own assertions and exits).
set -u
LIB="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/_lib" "$tmp/scenario"
cp "$LIB/skills_inject_check.sh" "$LIB/refusal_filter.sh" "$tmp/_lib/"

# A scenario-shaped consumer, exactly as test/evals/<scenario>/check.sh sources
# the body. It is not expected to run to completion — the body exits on its own
# fixture assertions — so the probe is its output, not its status.
cat >"$tmp/scenario/check.sh" <<'EOF'
#!/usr/bin/env bash
set -u
. "$(cd "$(dirname "$0")/../_lib" && pwd)/skills_inject_check.sh"
EOF
out=$(cd "$tmp/scenario" && bash "$tmp/scenario/check.sh" 2>&1 || true)
case "$out" in
  *"cannot source"*|*"command not found"*|*"did not define carries_injection"*)
    echo "FAIL scenario-shaped source: $out"; exit 1 ;;
esac

# And the body must fail closed when the scan cannot be found at all.
mkdir -p "$tmp/broken/_lib" "$tmp/broken/scenario"
cp "$LIB/skills_inject_check.sh" "$tmp/broken/_lib/"
cp "$tmp/scenario/check.sh" "$tmp/broken/scenario/"
out=$(cd "$tmp/broken/scenario" && bash "$tmp/broken/scenario/check.sh" 2>&1 || true)
case "$out" in
  *"refusing to score with the injection scan disabled"*) ;;
  *) echo "FAIL a checker without its scan must refuse to score: $out"; exit 1 ;;
esac

echo "ok"

