#!/usr/bin/env python3
"""Render the ACP registry entry for a release: `moca/agent.json` + `moca/icon.svg`.

Usage:
  scripts/release/render-acp-registry.py --tag v0.7.8 --out /tmp/acp-registry

The output directory is what a pull request to `agentclientprotocol/registry` adds:
the folder name is the agent id and `agent.json`'s `id` must equal it. Archive URLs
follow the published naming contract (`moca-<tag>-<goos>-<goarch>.tar.gz|zip`), so a
tag whose packages are not all published yields URLs that 404 — the script says so
when `--verify` is passed and a HEAD request fails.

The registry's requirements, all of which this script encodes:

- a `binary` distribution covering darwin, linux and windows (CI rejects a subset);
- a `license_url` and a `sha256` per archive (the registry's validator downloads the
  archive and refuses a checksum mismatch, so the checksums come from the release's
  own `checksums.txt` — never typed in by hand);
- an SVG icon at `<id>/icon.svg`, 16x16 and monochrome `currentColor`,
  rendered from `docs/assets/icon-acp.svg` (the derived mark; the full-colour
  brand art at `docs/assets/icon.svg` is not registry-compliant);
- an agent whose `initialize` returns at least one `authMethods` entry.

moca satisfies all of them from v0.7.8 on; on an older tag the darwin entries have
nothing to point at.
"""
from __future__ import annotations

import argparse
import json
import pathlib
import sys
import urllib.error
import urllib.request

IDENT = "moca"
REPO = "https://github.com/adeotek/moca"
# The registry shows a display name and a one-line description in an editor's
# agent picker. Both come from branding.json, so this entry cannot describe the
# product differently from the formula and the manifests beside it; `id` stays
# the folder name the registry requires, `name` is what a user reads.
BRAND = json.loads((pathlib.Path(__file__).resolve().parent / "branding.json").read_text(encoding="utf-8"))
NAME = BRAND["registryName"]
DESCRIPTION = BRAND["line"]

# registry key -> (goos, goarch, extension)
TARGETS = {
    "darwin-aarch64": ("darwin", "arm64", "tar.gz"),
    "darwin-x86_64": ("darwin", "amd64", "tar.gz"),
    "linux-aarch64": ("linux", "arm64", "tar.gz"),
    "linux-x86_64": ("linux", "amd64", "tar.gz"),
    "windows-aarch64": ("windows", "arm64", "zip"),
    "windows-x86_64": ("windows", "amd64", "zip"),
}


def archive_url(tag: str, goos: str, goarch: str, ext: str) -> str:
    return f"{REPO}/releases/download/{tag}/moca-{tag}-{goos}-{goarch}.{ext}"


def archive_name(tag: str, goos: str, goarch: str, ext: str) -> str:
    return f"moca-{tag}-{goos}-{goarch}.{ext}"


def fetch_checksums(tag: str) -> dict[str, str]:
    """The release's own checksums.txt, as {asset: sha256}."""
    url = f"{REPO}/releases/download/{tag}/checksums.txt"
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:  # noqa: S310
            text = resp.read().decode("utf-8")
    except urllib.error.URLError as exc:
        raise SystemExit(f"cannot read {url}: {exc}\npublish the release before rendering its registry entry")
    sums: dict[str, str] = {}
    for line in text.splitlines():
        parts = line.split()
        if len(parts) != 2:
            continue
        sha, name = parts[0].lower(), parts[1].lstrip("*")
        if len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha):
            raise SystemExit(f"checksums.txt carries a malformed sha256 for {name}: {sha!r}")
        sums[name] = sha
    if not sums:
        raise SystemExit(f"{url} lists no checksums")
    return sums


def icon_problems(text: str) -> list[str]:
    """The registry's icon rules, checked rather than assumed: 16x16, vector,
    monochrome through currentColor, and small enough to belong in a manifest."""
    problems: list[str] = []
    if 'viewBox="0 0 16 16"' not in text:
        problems.append("viewBox must be 0 0 16 16")
    if "currentColor" not in text:
        problems.append("must paint with currentColor (monochrome)")
    if "<image" in text:
        problems.append("must be vector paths, not an embedded raster")
    size = len(text.encode("utf-8"))
    if size > 8192:
        problems.append(f"{size} bytes is too large for a registry icon (max 8192)")
    return problems


def main() -> int:
    root = pathlib.Path(__file__).resolve().parents[2]
    ap = argparse.ArgumentParser()
    ap.add_argument("--tag", required=True, help="release tag, e.g. v0.7.8")
    ap.add_argument("--out", required=True, help="directory to write <id>/ into")
    ap.add_argument("--website", default=REPO, help="website field (default: the repository)")
    ap.add_argument("--verify", action="store_true", help="HEAD every archive URL")
    args = ap.parse_args()

    tag = args.tag.strip()
    if not tag.startswith("v") or tag.count(".") != 2:
        print(f"tag must look like vX.Y.Z, got {tag!r}", file=sys.stderr)
        return 2
    version = tag[1:]

    sums = fetch_checksums(tag)

    binary = {}
    missing: list[str] = []
    for key, (goos, goarch, ext) in TARGETS.items():
        name = archive_name(tag, goos, goarch, ext)
        sha = sums.get(name)
        if sha is None:
            missing.append(name)
            continue
        binary[key] = {
            "archive": archive_url(tag, goos, goarch, ext),
            "sha256": sha,
            "cmd": "moca.exe" if goos == "windows" else "./moca",
            "args": ["acp"],
        }

    bad: list[str] = []
    if args.verify:
        for key, entry in binary.items():
            req = urllib.request.Request(entry["archive"], method="HEAD")
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:  # noqa: S310
                    code = resp.status
            except Exception as exc:  # noqa: BLE001 - a 404 is the answer we want to report
                code = getattr(exc, "code", str(exc))
            print(f"{key}: {code} {entry['archive']}", file=sys.stderr)
            if code != 200:
                bad.append(key)

    if missing:
        print(f"checksums.txt has no entry for: {', '.join(missing)}", file=sys.stderr)
    if bad:
        print(f"not published for {tag}: {', '.join(bad)} — the registry needs all six", file=sys.stderr)

    agent = {
        "id": IDENT,
        "name": NAME,
        "version": version,
        "description": DESCRIPTION,
        "repository": REPO,
        "website": args.website,
        "authors": ["adeotek"],
        "license": "MIT",
        "license_url": f"{REPO}/blob/main/LICENSE",
        "distribution": {"binary": binary},
    }

    icon = root / "docs" / "assets" / "icon-acp.svg"
    icon_text = icon.read_text(encoding="utf-8")
    icon_bad = icon_problems(icon_text)
    for problem in icon_bad:
        print(f"icon: {problem}", file=sys.stderr)

    out = pathlib.Path(args.out) / IDENT
    out.mkdir(parents=True, exist_ok=True)
    (out / "agent.json").write_text(json.dumps(agent, indent=2) + "\n", encoding="utf-8")
    (out / "icon.svg").write_text(icon_text, encoding="utf-8")
    print(f"wrote {out}/agent.json and {out}/icon.svg for {tag}")
    # An incomplete run exits 1: the files are written either way, so they can be
    # inspected, but the caller must not mistake "rendered" for "ready to submit".
    return 1 if (bad or missing or icon_bad) else 0


if __name__ == "__main__":
    raise SystemExit(main())
