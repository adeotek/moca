#!/usr/bin/env python3
"""Generate the sweep-bulk fixture: 120 services, three files each, a real join.

Why this shape: the v1 pilot's fixture was 16 files / 9.8 KB, so one script saved
~1.5 k tokens and cost a turn — the arithmetic could never pay. This is the
bulk-inspection class v2 is about: every answer needs all 360 files (~45 KB of
source) and the final answer is a handful of aggregate lines, so the work is
reading, not writing.

Determinism is a test: the same tree hash twice. The checker re-derives the truth
from the files it is standing in, so a change here that nobody reflected in the
expectations fails the selftest, not the pilot.

The index arithmetic uses coprime moduli (3/4/5/7/8/17) on purpose: a policy must
not determine whether a service inherits its limit, or the task degenerates into
"each policy has one answer".

Usage:
  gen.py [--dir DIR]      write the fixture tree (default: the current directory)
  gen.py --answer         print turn one's exact expected answer
  gen.py --full           print the expected two-turn answer (all lines)
  gen.py --disabled       print the follow-up's last line
  gen.py --transcript P   write a reference session JSONL to P
"""

import argparse
import json
import os

DOMAINS = [
    "billing", "catalog", "checkout", "identity", "inventory", "ledger",
    "notify", "orders", "profile", "search", "shipping", "support",
]
NOUNS = [
    "api", "worker", "indexer", "gateway", "scheduler", "reconciler",
    "exporter", "importer", "proxy", "sync",
]
TEAMS = ["core", "growth", "platform", "payments", "data", "edge", "identity", "ops"]
POLICIES = [  # name, priority, maxReplicas
    ("gold", 1, 40),
    ("silver", 2, 25),
    ("bronze", 3, 10),
]
LIMITS = ["100m", "250m", "500m", "1000m", "2000m", "4000m"]
REQUESTS = ["50m", "100m", "250m", "500m"]
MEM = ["128Mi", "256Mi", "512Mi", "1Gi"]
REGIONS = ["eu-west-1", "eu-central-1", "us-east-1"]
DEFAULT_CPU_LIMIT = "500m"
DEFAULT_CPU_REQUEST = "250m"
DEFAULT_MEM_LIMIT = "512Mi"
ENVS = ["prod", "staging", "dev"]


def services():
    """The deterministic service table; the only source of truth for the tree."""
    out = []
    i = 0
    for d in DOMAINS:
        for n in NOUNS:
            policy = POLICIES[i % 3]
            out.append({
                "name": f"{d}-{n}",
                "team": TEAMS[i % len(TEAMS)],
                "policy": policy[0],
                "priority": policy[1],
                "maxReplicas": policy[2],
                "image": f"registry.internal/{d}/{n}:{1 + i % 9}.{i % 5}.{i % 3}",
                "replicas": 1 + (i * 3) % 7,
                "port": 8000 + (i * 7) % 900,
                "request": REQUESTS[i % 4],
                # Every fifth service states no limit and inherits defaults.yaml.
                "limit": None if i % 5 == 0 else LIMITS[(i * 7) % 6],
                "memRequest": MEM[i % 3],
                "memLimit": MEM[(i + 1) % 4],
                "env": ENVS[(i * 7) % 3],
                "region": REGIONS[i % 3],
                "tier": 1 + (i * 5) % 4,
                "disabled": i % 17 == 5,
            })
            i += 1
    return out


def cpu_value(v):
    v = v.strip().lower()
    if v.endswith("m"):
        return int(v[:-1])
    if v.endswith("k"):
        return int(v[:-1]) * 1000
    return int(float(v) * 1000)


def effective(svc):
    """The join: a stated limit wins, otherwise defaults.yaml's."""
    return svc["limit"] or DEFAULT_CPU_LIMIT


def answer_block(svcs):
    lines = []
    for name, _, _ in sorted(POLICIES):
        all_of = [s for s in svcs if s["policy"] == name]
        enabled = [s for s in all_of if not s["disabled"]]
        worst = max((effective(s) for s in enabled), key=cpu_value, default="0m")
        lines.append("policy=%s services=%d disabled=%d maxCpu=%s"
                     % (name, len(all_of), len(all_of) - len(enabled), worst))
    inherited = sum(1 for s in svcs if s["limit"] is None)
    lines.append("inherited=%d" % inherited)
    lowest = sorted(svcs, key=lambda s: (cpu_value(effective(s)), s["name"]))[:5]
    lines.append("lowest=" + ",".join("%s:%s" % (s["name"], effective(s)) for s in lowest))
    return "\n".join(lines)


def disabled_block(svcs):
    return "disabled=" + ",".join(sorted(s["name"] for s in svcs if s["disabled"]))


def full_block(svcs):
    """The follow-up's answer: every earlier line, then the disabled names.

    A two-turn trial is graded on the LAST message, so the second answer has to
    repeat the first one's lines — which is also what makes the follow-up cost a
    second real pass over the tree instead of a memory test.
    """
    return answer_block(svcs) + "\n" + disabled_block(svcs)


def write_tree(root):
    os.makedirs(os.path.join(root, "platform"), exist_ok=True)
    with open(os.path.join(root, "platform", "defaults.yaml"), "w") as f:
        f.write("# Values a service inherits when it states none of its own.\n")
        f.write("cpu:\n  request: %s\n  limit: %s\nmemory:\n  limit: %s\n"
                % (DEFAULT_CPU_REQUEST, DEFAULT_CPU_LIMIT, DEFAULT_MEM_LIMIT))
    for svc in services():
        d = os.path.join(root, "services", svc["name"])
        os.makedirs(d, exist_ok=True)
        with open(os.path.join(d, "service.yaml"), "w") as f:
            f.write("name: %s\nteam: %s\npolicy: %s\n"
                    "image: %s\nreplicas: %d\nport: %d\n"
                    "cpu:\n  request: %s\n"
                    % (svc["name"], svc["team"], svc["policy"], svc["image"],
                       svc["replicas"], svc["port"], svc["request"]))
            if svc["limit"]:
                f.write("  limit: %s\n" % svc["limit"])
            f.write("memory:\n  request: %s\n  limit: %s\n"
                    % (svc["memRequest"], svc["memLimit"]))
        with open(os.path.join(d, "policy.yaml"), "w") as f:
            f.write("name: %s\npriority: %d\nmaxReplicas: %d\nrollout: canary\n"
                    % (svc["policy"], svc["priority"], svc["maxReplicas"]))
        with open(os.path.join(d, "environment.yaml"), "w") as f:
            f.write("environment: %s\nregion: %s\ndisabled: %s\ntier: %d\nowner: team-%s\n"
                    % (svc["env"], svc["region"], "true" if svc["disabled"] else "false",
                       svc["tier"], svc["team"]))


def write_transcript(path, svcs):
    prompt = read_sibling("prompt.txt")
    followup = read_sibling("followup.txt")

    def msg(mid, parent, role, text, ts):
        return {"id": mid, "parentId": parent, "type": "message", "ts": ts,
                "message": {"role": role, "content": [{"type": "text", "text": text}]}}

    head = {"id": "s0", "type": "session", "session": {
        "workdir": "/work", "provider": "opencode-go", "model": "deepseek-v4.1-flash",
        "effort": "medium", "startedAt": "2026-11-24T09:00:00+02:00",
        "mocaVersion": "v0.9.0", "systemPrompt": "…"}}
    entries = [
        head,
        msg("m1", "s0", "user", prompt, "2026-11-24T09:00:05+02:00"),
        msg("m2", "m1", "assistant", answer_block(svcs), "2026-11-24T09:01:40+02:00"),
        msg("m3", "m2", "user", followup, "2026-11-24T09:02:00+02:00"),
        msg("m4", "m3", "assistant", full_block(svcs), "2026-11-24T09:02:40+02:00"),
    ]
    with open(path, "w") as f:
        for e in entries:
            f.write(json.dumps(e) + "\n")


def read_sibling(name):
    return open(os.path.join(os.path.dirname(os.path.abspath(__file__)), name)).read().rstrip("\n")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", default=".")
    ap.add_argument("--answer", action="store_true")
    ap.add_argument("--full", action="store_true")
    ap.add_argument("--disabled", action="store_true")
    ap.add_argument("--transcript", default="")
    a = ap.parse_args()
    if a.answer:
        print(answer_block(services()))
    elif a.full:
        print(full_block(services()))
    elif a.disabled:
        print(disabled_block(services()))
    elif a.transcript:
        write_transcript(a.transcript, services())
    else:
        write_tree(a.dir)


if __name__ == "__main__":
    main()
