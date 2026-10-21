#!/usr/bin/env bash
# skills scenario — two sessions in one repo (the fixture's suite needs the
# env var its Makefile sets, and the repo ships a stale rule about it).
#
# pass iff:
#   (d) the suite is green the repo's own way (CALC_STRICT=1);
#   (a) an auto skill records how this repo runs its tests;
#   (e) the stale rule the fixture shipped is corrected or deleted, not duplicated;
#   (b) session 2 was steered by the skill: either the transcript reads the
#       body (the read tool, search, or a reader head through the shell — a
#       mention inside an echo is none), or the skill's DESCRIPTION names a
#       command session 2 then ran. The design only promises that the listing
#       steers, so a run that never opens the file but takes the command its
#       description named has used the skill — requiring a read measured the
#       file, not the feature;
#   (c) session 2 does not repeat the naive command three times (counted only
#       when the command actually ran, never on a mention or a refusal).
#
# Variant scoping (-c off): with the feature off no skill can exist, so (a),
# (b) and (e) are REPORT-ONLY there (printed, not required); (c) and (d) stay
# required in both variants — session 2 must not flail unaided either.
set -u
VARIANT="${VARIANT:-on}"
required=1
[ "$VARIANT" = "off" ] && required=0

CALC_STRICT=1 go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }

# (a)/(e): what the auto skills say.
rule=0 stale=0
for f in .moca/skills/moca-*/SKILL.md; do
  [ -f "$f" ] || continue
  grep -q '^auto: true' "$f" || continue
  if grep -qE 'make test|CALC_STRICT' "$f"; then rule=1; fi
  # (e) is about the CLAIM, not the wording: a line still directing the reader
  # to plain `go test ./...` (with no make test / CALC_STRICT next to it) is the
  # stale rule — a minimally corrected rule that keeps the old sentence tail is
  # a correction, and must not be scored as duplication.
  if grep -E 'go test' "$f" | grep -vqE 'make test|CALC_STRICT|strict'; then stale=1; fi
done
if [ "$rule" != 1 ]; then
  if [ "$required" = 1 ]; then echo "(a) no auto skill records the repo's test command"; exit 1; fi
  echo "report: (a) no auto skill records the repo's test command (feature off)"
fi
if [ "$stale" = 1 ]; then
  if [ "$required" = 1 ]; then echo "(e) the stale rule still stands"; exit 1; fi
  echo "report: (e) the stale rule still stands (feature off)"
fi

# (b)/(c): the second session's transcript. --selftest has no session.
if [ -n "${SESSION:-}" ] && [ -f "${SESSION:-}" ]; then
  python3 - "$SESSION" "$required" <<'PY' || exit 1
import glob, json, re, sys

sess, required = sys.argv[1], sys.argv[2] == "1"
READERS = re.compile(r"^(cat|head|tail|less|more|sed|awk|grep|rg|stat|wc|sha256sum)\b")


RTK_RUN = ("test", "err", "summary", "proxy", "run")  # permissions/rtk.go's rtkRun


def segments(cmd):
    # `rtk <verb> -- make test` runs `make test` (§10's unwrap): the verb goes
    # with the wrapper, or a steered session reads as unsteered (v073-cd3).
    out = []
    for p in re.split(r"&&|\|\||;|\||\n", cmd):
        p = p.strip()
        m = re.match(r"^rtk\s+(\w+)\s+(?:--\s+)?(.*)$", p)
        if m and m.group(1).lower() in RTK_RUN:
            p = m.group(2)
        if p:
            out.append(p)
    return out


def naive(cmd):
    # The fixture's trap: a plain `go test` without its Makefile's env var.
    for s in segments(cmd):
        s = re.sub(r"^(sudo|env|time)\s+", "", s)
        s = re.sub(r"^(\w+=\S+\s+)+", "", s)
        s = re.sub(r"^cd\s+\S+\s+", "", s)
        if re.match(r"^go\s+test\b", s) and "make" not in s and "CALC_STRICT" not in s:
            return True
    return False


calls, results = {}, {}
for line in open(sess, errors="ignore"):
    try:
        e = json.loads(line)
    except json.JSONDecodeError:
        continue
    if e.get("type") == "tool_use" and e.get("toolUse"):
        call = e["toolUse"].get("call") or {}
        calls[call.get("id")] = call
    if e.get("type") == "tool_result":
        tr = e.get("toolResult") or {}
        results[tr.get("callId")] = tr.get("content") or ""

def norm(s):
    s = re.sub(r"^(sudo|env|time)\s+", "", s)
    s = re.sub(r"^(\w+=\S+\s+)+", "", s)
    s = re.sub(r"^cd\s+\S+\s+", "", s)
    return s.strip()


def ran(cmd):
    return [norm(p) for p in segments(cmd)]


# (b), second limb: the commands the run actually took, and the commands the
# skill's description named. A description that says "make test" and a run that
# ran `make test` is the skill doing its job through the listing.
executed = []
for cid, call in calls.items():
    if call.get("name") == "shell":
        cmd = (call.get("input") or {}).get("command", "")
        if not (results.get(cid) or "").startswith("refused:"):
            executed.extend(ran(cmd))
CMDSHAPE = re.compile(r"\b(make\s+[\w.-]+|go\s+(?:test|vet|build|run)\b[\w./-]*|pytest|npm\s+(?:run\s+)?[\w:-]+|cargo\s+(?:test|build))\b")
named = []
for f in glob.glob(".moca/skills/moca-*/SKILL.md"):
    desc = ""
    try:
        head = open(f, errors="ignore").read().split("---")[1]
    except (IndexError, OSError):
        continue
    for line in head.splitlines():
        if line.startswith("description:"):
            desc = line
            break
    for m in CMDSHAPE.finditer(desc):
        pair = " ".join(m.group(1).split()[:2])
        if pair and any(e.startswith(pair) for e in executed):
            named.append(pair)

reads, naive_n = [], 0
for cid, call in calls.items():
    name, inp = call.get("name"), call.get("input") or {}
    blob = json.dumps(inp)
    # (b): a READ act on the skill — the read tool, search, or a reader head
    # through the shell (`cat .moca/skills/.../SKILL.md`); a mention inside an
    # echo is none of those.
    if ".moca/skills/" in blob and name in ("read", "search"):
        reads.append(blob)
    if name == "shell":
        cmd = inp.get("command", "")
        if ".moca/skills/" in blob and (segments(cmd) or [""]) and READERS.match(segments(cmd)[0]):
            reads.append(blob)
        # (c): count only a command that RAN the naive form, never a mention
        # (`echo`, a commit message) and never a refusal.
        if naive(cmd) and not (results.get(cid) or "").startswith("refused:"):
            naive_n += 1
if naive_n >= 3:
    print(f"(c) session 2 repeated the naive command {naive_n} times")
    sys.exit(1)
if named:
    print(f"report: (b) the description named {named[0]!r} and session 2 ran it")
elif not reads:
    if required:
        print("(b) session 2 neither read the skill nor ran a command its description named")
        sys.exit(1)
    print("report: (b) session 2 never read the skill (feature off)")
PY
fi
