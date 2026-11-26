#!/usr/bin/env python3
"""Consistency check for the release rotation.

Compares what the two files say with what releases.json says: every released
service must carry the release tag in its service.yaml and in the manifest.
Read-only; exits 1 with the first mismatches, so a half-applied rotation is
visible from here rather than in a deploy.
"""
import json
import pathlib
import re
import sys

root = pathlib.Path(__file__).parent
releases = json.loads((root / "releases.json").read_text())["services"]

IMAGE = re.compile(r"^image: ghcr\.io/acme/[a-z0-9-]+:(\S+)$", re.M)
ENTRY = re.compile(r"^  - service: (\S+)\n    tag: (\S+)$", re.M)


def service_tag(name):
    m = IMAGE.search((root / "services" / name / "service.yaml").read_text())
    return m.group(1) if m else None


manifest_tags = dict(ENTRY.findall((root / "deploy" / "manifest.yaml").read_text()))

bad = []
for name, want in sorted(releases.items()):
    have = service_tag(name)
    if have != want:
        bad.append(f"services/{name}/service.yaml has {have}, releases.json says {want}")
    in_manifest = manifest_tags.get(name)
    if in_manifest != want:
        bad.append(f"deploy/manifest.yaml has {in_manifest}, releases.json says {want}")
if bad:
    print("\n".join(bad[:12]))
    sys.exit(1)
print(f"verify ok: {len(releases)} released services agree in both files")
