#!/usr/bin/env python3
"""The direct-edit route for changeset-registry: one page edit per plugin plus
one registry rewrite, counted as the pairs a model must spell out."""
import hashlib
import json
import pathlib

root = pathlib.Path(".")
data = {p["name"]: p for p in json.loads((root / "plugins" / "data.json").read_text())["plugins"]}
pages = sorted(p.stem for p in (root / "capabilities").glob("*.go") if p.stem != "spec")
targets = [n for n in pages if n in data and data[n]["enabled"]]

def dg(name, n):
    p = data[name]
    return hashlib.sha1(f"{name}|{p['kind']}|{p['endpoint']}".encode()).hexdigest()[:n]

edits = 0
for name in targets:
    p = data[name]
    f = root / "capabilities" / f"{name}.go"
    old = [l for l in f.read_text().splitlines() if l.startswith("\tDigest:")][0]
    f.write_text(f.read_text().replace(old, '\tDigest:   "%s",' % dg(name, 8)))
    edits += 1
pairs = sorted((n, data[n]["kind"], dg(n, 6)) for n in targets)
width = max(len(f'"{n}":') for n, _, _ in pairs)
rows = "\n".join('\t' + (f'"{n}":'.ljust(width)) + f' "{k}@{d}",' for n, k, d in pairs)
(root / "plugins" / "registry.go").write_text(
    "package plugins\n\n"
    "// registry maps an enabled plugin to the kind its factory expects.\n"
    "// Generated from plugins/data.json — regenerate, do not hand-edit.\n"
    "var registry = map[string]string{\n" + rows + "\n}\n")
print("edits=%d" % (edits + 1))
