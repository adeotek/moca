#!/usr/bin/env bash
# Proves markers on synthetic transcripts. No model, no network.
#
# The two mistakes this pins, both of which produced a confident "0 markers"
# in a hand scan:
#   1. counting the session entry, which embeds the system prompt containing the
#      instruction's own `shortcut:` example -> a false positive everywhere;
#   2. reading only the assistant's prose -> a false negative, because the
#      marker is normally a comment inside a write/edit payload.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
. "$here/markers.sh"

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
bad=0
check() { # check NAME EXPECTED ACTUAL
  if [ "$2" != "$3" ]; then
    printf 'FAIL %s\n  want: %s\n  got:  %s\n' "$1" "$2" "$3" >&2
    bad=1
  else
    printf 'ok   %s\n' "$1"
  fi
}

# 1. The marker lives in a write payload as a code comment, with a trigger.
cat > "$tmp/in-write.jsonl" <<'JSONL'
{"type":"session","session":{"workdir":"/tmp/x"},"systemPrompt":"Mark a limit as `shortcut: <limit>, revisit when <trigger>`."}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"add the flag"}]}}
{"type":"tool_use","toolUse":{"name":"write","input":{"path":"main.go","content":"// shortcut: only handles --sizes, revisit when --all is requested\nfunc f() {}\n"}}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}
JSONL
check "marker-in-a-write-payload" "markers=1 with-trigger=1 no-trigger=0 files=main.go" "$(markers "$tmp/in-write.jsonl")"

# 2. The same marker in the user's words must not count.
cat > "$tmp/in-user.jsonl" <<'JSONL'
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"use shortcut: whatever, revisit when bored"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Sure."}]}}
JSONL
check "the-users-words-do-not-count" "markers=0 with-trigger=0 no-trigger=0 files=-" "$(markers "$tmp/in-user.jsonl")"

# 3. The system prompt embedding the instruction must not count either.
cat > "$tmp/in-sys.jsonl" <<'JSONL'
{"type":"session","session":{"workdir":"/tmp/x"},"systemPrompt":"A limit accepted on purpose is marked in code as `shortcut: <the limit>, revisit when <trigger>`."}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Nothing to do."}]}}
JSONL
check "the-system-prompt-does-not-count" "markers=0 with-trigger=0 no-trigger=0 files=-" "$(markers "$tmp/in-sys.jsonl")"

# 4. A marker with no trigger is the /debt no-trigger case, and is counted apart.
cat > "$tmp/in-notrigger.jsonl" <<'JSONL'
{"type":"tool_use","toolUse":{"name":"edit","input":{"path":"store.go","new_string":"// shortcut: no retry on 5xx\n"}}}
JSONL
check "no-trigger-is-counted-apart" "markers=1 with-trigger=0 no-trigger=1 files=store.go" "$(markers "$tmp/in-notrigger.jsonl")"

# 5. A prose mention (a run reporting its own shortcuts) counts, per marker.
cat > "$tmp/in-prose.jsonl" <<'JSONL'
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"I left two:\n- shortcut: no pagination, revisit when a list exceeds 1k\n- shortcut: ASCII only"},{"type":"thinking","text":"shortcut: thinking counts too, until asked otherwise"}]}}
JSONL
check "prose-and-thinking-count" "markers=3 with-trigger=2 no-trigger=1 files=-" "$(markers "$tmp/in-prose.jsonl")"

# 6. A missing file fails closed rather than reporting zero markers.
if markers "$tmp/nope.jsonl" >/dev/null 2>&1; then
  printf 'FAIL a missing transcript must fail, not report zero\n' >&2
  bad=1
else
  printf 'ok   missing-transcript-fails-closed\n'
fi

[ "$bad" -eq 0 ] && printf 'all marker pins pass\n'
exit "$bad"
