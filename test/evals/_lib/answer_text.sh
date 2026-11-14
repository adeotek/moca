#!/usr/bin/env bash
# answer_text SESSION_FILE — the last assistant message's prose, with fenced
# code blocks stripped.
#
# The transcript is one JSON object per line (`internal/session`): the entries
# we care about are {"type":"message","message":{"role":"assistant",
# "content":[{"type":"text","text":…},{"type":"thinking",…}]}}. Walking from
# the end and taking the first assistant entry that CARRIES TEXT is what makes
# a thinking-only final entry skip backwards to the real answer — an entry with
# only {"type":"thinking"} blocks is not an answer.
answer_text() {
  [ -n "${1:-}" ] && [ -f "$1" ] || { echo "answer_text: no session file: ${1:-}" >&2; return 1; }
  python3 - "$1" <<'PY'
import json, re, sys

last = ""
for line in open(sys.argv[1], encoding="utf-8", errors="replace"):
    line = line.strip()
    if not line.startswith("{"):
        continue
    try:
        row = json.loads(line)
    except Exception:
        continue
    if row.get("type") != "message":
        continue
    msg = row.get("message") or {}
    if msg.get("role") != "assistant":
        continue
    texts = [b.get("text", "") for b in (msg.get("content") or []) if b.get("type") == "text"]
    if not texts:
        continue          # thinking-only turn: not an answer, look further back
    last = "\n".join(t for t in texts if t.strip())

# Fenced blocks are code, not prose: the write-up is what sits between them.
print(re.sub(r"```.*?```", "", last, flags=re.S).strip())
PY
}
