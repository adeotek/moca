#!/usr/bin/env bash
# markers SESSION_FILE — the `shortcut:` markers a run actually wrote.
#
# Why a shared helper: the first scan of my own looked at reply text only and
# reported 0 markers in 84 transcripts. That was a measurement bug, not model
# behaviour — the session file embeds the whole system prompt (which contains
# the word), so a naive grep of the file finds the instruction, and the marker
# itself is written as a **code comment inside a write/edit tool payload**, not
# in the prose answer. Both mistakes produce a confident zero.
#
# So this reads structured entries and counts only markers an assistant caused:
#   * tool_use payloads (write/edit `content`, `new_string`, `patch`) — where it
#     normally lands, as a comment in the code it just wrote;
#   * assistant text/thinking blocks — a run may also report one in prose.
# The user's own message and the system prompt are excluded by construction.
#
# Output (stdout), one line per transcript:
#   markers=<n> with-trigger=<n> no-trigger=<n> files=<comma list, or ->
# `with-trigger` counts markers naming a revisit condition ("revisit when …",
# "until …", "when X, …"); the rest are exactly what `/debt`'s no-trigger case
# exists to surface.
markers() {
  [ -n "${1:-}" ] && [ -f "$1" ] || { echo "markers: no session file: ${1:-}" >&2; return 1; }
  python3 - "$1" <<'PY'
import json, re, sys

MARK = re.compile(r"shortcut:\s*(.+)", re.I)
# A trigger is the half that makes a marker useful: the condition to revisit.
TRIGGER = re.compile(r"(revisit|until|once|when |if |after |before |because )", re.I)

texts = []   # (kind, text) pairs we are allowed to count in
for line in open(sys.argv[1], encoding="utf-8", errors="replace"):
    line = line.strip()
    if not line.startswith("{"):
        continue
    try:
        row = json.loads(line)
    except Exception:
        continue
    t = row.get("type")
    if t == "tool_use":
        payload = (row.get("toolUse") or {}).get("input") or {}
        for k in ("content", "new_string", "patch", "text"):
            v = payload.get(k)
            if isinstance(v, str):
                texts.append(("tool:" + str(payload.get("path") or payload.get("file_path") or "?"), v))
    elif t == "message":
        msg = row.get("message") or {}
        if msg.get("role") != "assistant":
            continue  # the user's words and the system prompt never count
        for b in (msg.get("content") or []):
            if b.get("type") in ("text", "thinking") and isinstance(b.get("text"), str):
                texts.append(("assistant", b["text"]))

total = trig = 0
files = []
for kind, body in texts:
    for m in MARK.finditer(body):
        total += 1
        if TRIGGER.search(m.group(1)):
            trig += 1
    if kind.startswith("tool:") and MARK.search(body) and kind[5:] != "?" and kind[5:] not in files:
        files.append(kind[5:])

print("markers=%d with-trigger=%d no-trigger=%d files=%s"
      % (total, trig, total - trig, ",".join(files) if files else "-"))
PY
}
