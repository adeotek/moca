#!/usr/bin/env python3
"""Consistency check for the release rotation (re-derives the expected values).

A service is in this rollout when it is listed in stages[rollout.stage], has a
release in releases.json and runs at least two replicas. Its new tag is the
release version plus a seven-hex fingerprint of salt:name:replicas; the pin
block's weight is ten times its replicas, and the manifest's releaseId is a
ten-hex fingerprint of name + tag.
Read-only; exits 1 with the first mismatches.
"""
import hashlib
import json
import pathlib
import re
import sys

root = pathlib.Path(__file__).parent
d = json.loads((root / "releases.json").read_text())
stage = d["rollout"]["stage"]
salt = d["salt"]
in_stage = set(d["stages"][stage])
rel = d["services"]

IMAGE = re.compile(r"^image: ghcr\.io/acme/[a-z0-9-]+:(\S+)$", re.M)
ENTRY = re.compile(r"^  - service: (\S+)\n    tag: (\S+)\n    releaseId: (\S+)$", re.M)
PIN = re.compile(r"^pin:\n  tag: (\S+)\n  weight: (\d+)$", re.M)


def fp(*parts, n):
    return hashlib.sha1(":".join(parts).encode()).hexdigest()[:n]


def check(name):
    r = rel[name]
    want = f"{r['version']}-{fp(salt, name, str(r['replicas']), n=7)}"
    body = (root / "services" / name / "service.yaml").read_text()
    img = IMAGE.search(body)
    if not img or img.group(1) != want:
        return f"services/{name}/service.yaml has {img.group(1) if img else None}, the rollout says {want}"
    pin = PIN.search(body)
    if not pin or pin.group(1) != want or int(pin.group(2)) != r["replicas"] * 10:
        return f"services/{name}/service.yaml has no correct pin block"
    return None


manifest = dict((m[0], (m[1], m[2])) for m in ENTRY.findall((root / "deploy" / "manifest.yaml").read_text()))
targets = [n for n in rel if n in in_stage and rel[n]["replicas"] >= 2]
bad = []
for name in sorted(targets):
    e = check(name)
    if e:
        bad.append(e)
    tag = f"{rel[name]['version']}-{fp(salt, name, str(rel[name]['replicas']), n=7)}"
    if manifest.get(name, (None, None)) != (tag, fp(name, tag, n=10)):
        bad.append(f"deploy/manifest.yaml has {manifest.get(name)}, the rollout says {(tag, fp(name, tag, n=10))}")
if bad:
    print("\n".join(bad[:12]))
    sys.exit(1)
print(f"verify ok: {len(targets)} rollout services agree in both files")
