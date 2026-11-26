#!/usr/bin/env python3
"""The direct-edit route for changeset-pins: one exact edit per target.

An edit is one {old,new} pair the model must type; the count is the product
the route has to produce, which is what a model would spend tokens on.
"""
import hashlib
import json
import pathlib
import re

root = pathlib.Path(".")
d = json.loads((root / "releases.json").read_text())
salt, stage = d["salt"], d["rollout"]["stage"]
edits = 0
for name in d["stages"][stage]:
    if name not in d["services"]:
        continue
    f = root / "services" / name / "service.yaml"
    body = f.read_text()
    replicas = int([l.split(":")[1] for l in body.splitlines() if l.startswith("replicas:")][0])
    if replicas < 2:
        continue
    tag = "%s-%s" % (d["services"][name]["version"],
                     hashlib.sha256(f"{salt}:{name}:{replicas}".encode()).hexdigest()[:7])
    rid = hashlib.sha256(f"{name}:{tag}".encode()).hexdigest()[:10]
    old = [l for l in body.splitlines() if l.startswith("image:")][0]
    f.write_text(body.replace(old, "image: ghcr.io/acme/%s:%s" % (name, tag)) +
                 "pin:\n  tag: %s\n  weight: %d\n" % (tag, replicas * 10))
    edits += 1
    m = root / "deploy" / "manifest.yaml"
    s = m.read_text()
    dedent = r"(  - service: %s\n    tag: )\S+(\n    releaseId: )\S+" % re.escape(name)
    s, n = re.subn(dedent, lambda mm: mm.group(1) + tag + mm.group(2) + rid, s)
    assert n == 1, name
    m.write_text(s)
    edits += 1
print("edits=%d" % edits)
