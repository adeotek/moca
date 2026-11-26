#!/usr/bin/env bash
# The ordinary shell route for changeset-registry: discover the installed pages,
# regenerate each from the data with its derived digest, regenerate the
# gofmt-aligned registry, then verify.
set -euo pipefail
cd "${1:-.}"
python3 - <<'PY'
import hashlib, json, pathlib, re
root = pathlib.Path(".")
data = {p["name"]: p for p in json.loads((root / "plugins" / "data.json").read_text())["plugins"]}
pages = sorted(p.stem for p in (root / "capabilities").glob("*.go") if p.stem != "spec")
targets = [n for n in pages if n in data and data[n]["enabled"]]

def dg(name, n):
    p = data[name]
    return hashlib.sha1(f"{name}|{p['kind']}|{p['endpoint']}".encode()).hexdigest()[:n]

for name in targets:
    p = data[name]
    ident = name.replace("-", " ").title().replace(" ", "")
    (root / "capabilities" / f"{name}.go").write_text(
        "package capabilities\n\n"
        f"// {ident} is generated from plugins/data.json — edit the data, not this file.\n"
        f"var {ident} = Spec{{\n"
        f"\tName:     \"{name}\",\n\tKind:     \"{p['kind']}\",\n"
        f"\tEndpoint: \"{p['endpoint']}\",\n\tTimeout:  {p['timeoutMs']},\n"
        f"\tDigest:   \"{dg(name, 8)}\",\n}}\n")

pairs = sorted((n, data[n]["kind"], dg(n, 6)) for n in targets)
width = max(len(f'"{n}":') for n, _, _ in pairs)
rows = "\n".join('\t' + (f'"{n}":'.ljust(width)) + f' "{k}@{d}",' for n, k, d in pairs)
(root / "plugins" / "registry.go").write_text(
    "package plugins\n\n"
    "// registry maps an enabled plugin to the kind its factory expects.\n"
    "// Generated from plugins/data.json — regenerate, do not hand-edit.\n"
    "var registry = map[string]string{\n" + rows + "\n}\n")
print("pages=%d" % len(targets))
PY
gofmt -w capabilities plugins
python3 verify.py
