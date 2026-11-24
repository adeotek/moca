#!/usr/bin/env bash
# pass: the exact five-line aggregate from turn one, restated with the disabled
# list after the follow-up, over an untouched tree (the tree is graded second and
# reported, so a failure names what actually went wrong).
#
# The task is read-only analysis, so an edit or a stray file fails the run: a run
# that changed a manifest to make its numbers agree with a guess did not answer
# the question.
#
# The truth is re-derived here from the files the checker is standing in, not
# imported from gen.py: if the generator changes and the expectations do not, the
# selftest fails loudly instead of the pilot quietly grading against a stale
# answer.
set -u

[ -n "${SESSION:-}" ] && [ -f "$SESSION" ] || { echo "no session (selftest arm)"; exit 1; }

# The tree is checked but reported after the answer: a run that answered
# correctly and also left a scratch file must say so, and a run that answered
# wrongly must not have that hidden behind a tree complaint.
base=$(git rev-list --max-parents=0 HEAD)
tree=""
dirty=$(git status --porcelain -- . ':!.moca' 2>/dev/null | grep -v '^?? ' | head -3)
[ -z "$dirty" ] || tree+="the run modified tracked files: $dirty"$'\n'
untracked=$(git status --porcelain -- . ':!.moca' 2>/dev/null | sed -n 's/^?? //p' | head -3)
[ -z "$untracked" ] || tree+="the run left untracked files: $untracked"$'\n'
committed=$(git diff --name-only "$base" -- . ':!.moca' 2>/dev/null | head -3)
[ -z "$committed" ] || tree+="the run committed changes to: $committed"$'\n'

. "$(dirname "${BASH_SOURCE[0]}")/../_lib/answer_text.sh"
text=$(answer_text "$SESSION") || exit 1
[ -n "$text" ] || { echo "no final answer in prose"; exit 1; }

python3 - "$text" <<'PY'
import glob
import os
import re
import sys

text = sys.argv[1]


def scalar(path, section, key):
    """`section: { key: value }` from a two-level YAML file, or ''."""
    out, cur = "", ""
    for line in open(path):
        line = line.rstrip("\n")
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if not line.startswith(" "):
            cur = line.split(":", 1)[0]
            continue
        if cur == section:
            k, _, v = line.strip().partition(":")
            if k == key:
                out = v.strip()
    return out


def top(path, key):
    for line in open(path):
        if line.startswith(key + ":"):
            return line.split(":", 1)[1].strip()
    return ""


def cpu_value(v):
    v = v.strip().lower()
    if v.endswith("m"):
        return int(v[:-1])
    if v.endswith("k"):
        return int(v[:-1]) * 1000
    return int(float(v) * 1000)


default_limit = scalar("platform/defaults.yaml", "cpu", "limit")
if not default_limit:
    print("fixture is broken: no platform/defaults.yaml cpu limit")
    sys.exit(1)

svcs = []
for d in sorted(glob.glob("services/*/")):
    name = os.path.basename(d.rstrip("/"))
    stated = scalar(d + "service.yaml", "cpu", "limit")
    limit = stated or default_limit
    svcs.append({
        "name": name,
        "policy": top(d + "service.yaml", "policy"),
        "limit": limit,
        "stated": bool(stated),
        "enabled": top(d + "environment.yaml", "disabled") != "true",
    })
if len(svcs) != 120:
    print("fixture is broken: %d services, want 120" % len(svcs))
    sys.exit(1)

want_policies = {}
for pol in sorted({s["policy"] for s in svcs}):
    group = [s for s in svcs if s["policy"] == pol]
    enabled = [s for s in group if s["enabled"]]
    want_policies[pol] = (
        len(group),
        len(group) - len(enabled),
        max((s["limit"] for s in enabled), key=cpu_value, default="0m"),
    )
want_inherited = sum(1 for s in svcs if not s["stated"])
lowest = sorted(svcs, key=lambda s: (cpu_value(s["limit"]), s["name"]))[:5]
want_lowest = [(s["name"], s["limit"]) for s in lowest]
want_disabled = sorted(s["name"] for s in svcs if not s["enabled"])

got_policies, got = {}, {}
for line in text.splitlines():
    line = line.strip().strip("`").strip()
    m = re.fullmatch(r"policy=(\w+) services=(\d+) disabled=(\d+) maxCpu=(\S+)", line)
    if m:
        got_policies[m.group(1)] = (int(m.group(2)), int(m.group(3)), m.group(4))
        continue
    m = re.fullmatch(r"inherited=(\d+)", line)
    if m:
        got["inherited"] = int(m.group(1))
        continue
    m = re.fullmatch(r"lowest=(\S+)", line)
    if m:
        pairs = []
        for item in m.group(1).split(","):
            name, _, lim = item.partition(":")
            pairs.append((name.strip(), lim.strip()))
        got["lowest"] = pairs
        continue
    m = re.fullmatch(r"disabled=(.*)", line)
    if m:
        got["disabled"] = [n.strip() for n in m.group(1).split(",") if n.strip()]

if not got_policies:
    print("no policy lines in the answer: %r" % text[:200])
    sys.exit(1)
if got_policies != want_policies:
    print("policy aggregate is wrong:\n got %s\n want %s" % (got_policies, want_policies))
    sys.exit(1)
if got.get("inherited") != want_inherited:
    print("inherited = %r, want %d" % (got.get("inherited"), want_inherited))
    sys.exit(1)
if got.get("lowest") != want_lowest:
    print("lowest five = %r, want %r" % (got.get("lowest"), want_lowest))
    sys.exit(1)
if got.get("disabled") != want_disabled:
    print("disabled = %r, want %r" % (got.get("disabled"), want_disabled))
    sys.exit(1)
PY
# The grader's status has to be captured before the tree check runs: a bare
# `python3 ... || exit 1` followed by another command would let the last
# command's status swallow a wrong answer (which is exactly how the selftest's
# wrong-answer arm caught this reordering).
answer=$?
if [ "$answer" -ne 0 ]; then
  [ -n "$tree" ] && printf '%s' "$tree"
  exit 1
fi
[ -z "$tree" ] || { printf '%s' "$tree"; exit 1; }
