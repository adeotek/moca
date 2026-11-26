#!/usr/bin/env python3
"""Consistency check between plugins/data.json, capabilities/*.go and registry.go.

Every enabled plugin must match its page, and registry.go must list exactly the
enabled plugins, sorted. Disabled plugins are not registered and their pages are
not the registry's business. Read-only; exits 1 with the first mismatches.
"""
import json
import pathlib
import re
import sys

root = pathlib.Path(__file__).parent
data = json.loads((root / "plugins" / "data.json").read_text())["plugins"]

FIELD = re.compile(r'\b(Name|Kind|Endpoint|Timeout):\s*(?:"([^"]*)"|(\d+))')
ROW = re.compile(r'^\t"(\S+)":\s+"(\S+)",$', re.M)

bad = []
pages = {}
for page in (root / "capabilities").glob("*.go"):
    fields = {}
    for key, text, num in FIELD.findall(page.read_text()):
        fields[key] = text if text else num
    pages[fields.get("Name", page.stem)] = fields

for p in data:
    if not p["enabled"]:
        continue
    page = pages.get(p["name"])
    if page is None:
        bad.append(f"capabilities/{p['name']}.go is missing")
        continue
    for key, want in (("Kind", p["kind"]), ("Endpoint", p["endpoint"]), ("Timeout", str(p["timeoutMs"]))):
        if page.get(key) != want:
            bad.append(f"capabilities/{p['name']}.go has {key}={page.get(key)}, data.json says {want}")

rows = ROW.findall((root / "plugins" / "registry.go").read_text())
want_rows = sorted((p["name"], p["kind"]) for p in data if p["enabled"])
if rows != want_rows:
    extra = [r for r in rows if r not in want_rows][:6]
    missing = [r for r in want_rows if r not in rows][:6]
    bad.append(f"plugins/registry.go rows wrong (missing {missing}, extra {extra}, sorted {rows == sorted(rows)})")

if bad:
    print("\n".join(bad[:12]))
    sys.exit(1)
print(f"verify ok: {len(want_rows)} enabled plugins registered")
