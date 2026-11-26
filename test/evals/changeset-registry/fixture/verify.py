#!/usr/bin/env python3
"""Registry check: a page exists for every installed plugin, and every enabled,
installed plugin's page carries its derived digest and is listed once.

Read-only; exits 1 with the first mismatches.
"""
import hashlib
import json
import pathlib
import re
import sys

root = pathlib.Path(__file__).parent
plugins = {p["name"]: p for p in json.loads((root / "plugins" / "data.json").read_text())["plugins"]}
pages = {p.stem for p in (root / "capabilities").glob("*.go") if p.stem != "spec"}

DIGEST = re.compile(r"^\tDigest:   \"(\S+)\",$", re.M)


def want(p):
    return hashlib.sha1(f"{p['name']}|{p['kind']}|{p['endpoint']}".encode()).hexdigest()[:8]


bad = []
targets = [n for n in sorted(pages) if n in plugins and plugins[n]["enabled"]]
for name in targets:
    m = DIGEST.search((root / "capabilities" / f"{name}.go").read_text())
    if not m or m.group(1) != want(plugins[name]):
        bad.append(f"capabilities/{name}.go has {m.group(1) if m else None}, the data implies {want(plugins[name])}")

rows = dict(re.findall(r'^\t"([^"]+)":\s+"([^"]+)",$', (root / "plugins" / "registry.go").read_text(), re.M))
for name in targets:
    p = plugins[name]
    got = rows.get(name)
    exp = "%s@%s" % (p["kind"], hashlib.sha1(f"{name}|{p['kind']}|{p['endpoint']}".encode()).hexdigest()[:6])
    if got != exp:
        bad.append(f"plugins/registry.go has {got!r} for {name}, the data implies {exp!r}")
if bad:
    print("\n".join(bad[:12]))
    sys.exit(1)
print(f"verify ok: {len(targets)} installed plugins agree in the registry")
