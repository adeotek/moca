#!/usr/bin/env python3
"""Grade one route's tree: every written file equals the reference image, every
other file equals the fixture byte for byte, and nothing appeared or vanished."""
import pathlib
import sys

fixture, reference, work = (pathlib.Path(p) for p in sys.argv[1:4])


def files(root):
    return {p.relative_to(root).as_posix() for p in root.rglob("*") if p.is_file()}


fx, ref, wk = files(fixture), files(reference), files(work)
bad = []
for rel in sorted(wk - fx - {"verify.py"}):
    bad.append(f"created: {rel}")
for rel in sorted(fx - wk):
    bad.append(f"deleted: {rel}")
for rel in sorted(wk & fx):
    want = reference / rel if rel in ref else fixture / rel
    if (work / rel).read_bytes() != want.read_bytes():
        import difflib
        w = (work / rel).read_text(errors="replace").splitlines()
        r = want.read_text(errors="replace").splitlines()
        d = list(difflib.unified_diff(r, w, "expected", "got", lineterm="", n=1))[:8]
        bad.append(f"differs: {rel}\n  " + "\n  ".join(d))
for rel in sorted(ref - wk):
    bad.append(f"missing: {rel}")
if bad:
    print("\n".join(bad[:6]))
    sys.exit(1)
sys.exit(0)
