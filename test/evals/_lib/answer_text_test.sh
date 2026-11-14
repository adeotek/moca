#!/usr/bin/env bash
# Proves answer_text on two synthetic transcripts, the way skills_inject_c_test
# proves its checker: no model, no network.
#
# Case 1 — a final answer after tool calls. Case 2 — the last assistant entry
# carries only a thinking block, and the extractor must skip it and return the
# previous assistant message's text. That is the shape a reasoning model
# produces, and an extractor that took the literal last entry would return
# nothing and fail a run that answered correctly.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
. "$here/answer_text.sh"

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
bad=0

cat > "$tmp/one.jsonl" <<'JSONL'
{"type":"session","session":{"workdir":"/tmp/x"}}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"refactor it"}]}}
{"type":"tool_use","toolUse":{"name":"read","input":{}}}
{"type":"tool_result","toolResult":{"id":"a","content":"…"}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Refactored Summary."},{"type":"thinking","thinking":"considering"}]}}
JSONL
got=$(answer_text "$tmp/one.jsonl")
[ "$got" = "Refactored Summary." ] && echo "ok   answer-text-after-tools" \
  || { echo "FAIL answer-text-after-tools: got $got"; bad=1; }

cat > "$tmp/two.jsonl" <<'JSONL'
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"The real answer, in prose."}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm, still deliberating"}]}}
JSONL
got=$(answer_text "$tmp/two.jsonl")
[ "$got" = "The real answer, in prose." ] && echo "ok   answer-text-skips-a-thinking-only-turn" \
  || { echo "FAIL answer-text-skips-a-thinking-only-turn: got $got"; bad=1; }

cat > "$tmp/three.jsonl" <<'JSONL'
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Step one.\n```go\nfunc x() {}\n```\nStep two."}]}}
JSONL
got=$(answer_text "$tmp/three.jsonl")
case "$got" in
  *"func x()"*) echo "FAIL answer-text-strips-code-fences: kept the fence"; bad=1 ;;
  *"Step one."*"Step two."*) echo "ok   answer-text-strips-code-fences" ;;
  *) echo "FAIL answer-text-strips-code-fences: got $got"; bad=1 ;;
esac

# Fail closed, do not skip: a missing session must fail, or a scenario that
# forgot to pass SESSION would read as passing.
if answer_text "$tmp/nope.jsonl" >/dev/null 2>&1; then
  echo "FAIL answer-text-fails-closed: a missing file returned success"; bad=1
else
  echo "ok   answer-text-fails-closed"
fi
exit "$bad"
