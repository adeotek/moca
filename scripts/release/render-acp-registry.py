#!/usr/bin/env python3
"""Render the ACP registry entry for a release: `moca/agent.json` + `moca/icon.svg`.

Usage:
  scripts/release/render-acp-registry.py --tag v0.7.8 --out /tmp/acp-registry

The output directory is what a pull request to `agentclientprotocol/registry` adds:
the folder name is the agent id and `agent.json`'s `id` must equal it. Archive URLs
follow the published naming contract (`moca-<tag>-<goos>-<goarch>.tar.gz|zip`), so a
tag whose packages are not all published yields URLs that 404 — the script says so
when `--verify` is passed and a HEAD request fails.

The registry requires a `binary` distribution covering darwin, linux and windows
(CI rejects a subset), a 16x16 monochrome `currentColor` icon, and an agent whose
`initialize` returns at least one `authMethods` entry. moca satisfies all three from
v0.7.8 on; on an older tag the darwin entries have nothing to point at.
"""
from __future__ import annotations

import argparse
import json
import pathlib
import sys
import urllib.request

IDENT = "moca"
REPO = "https://github.com/adeotek/moca"
DESCRIPTION = "Minimal, token-efficient coding agent: one Go binary, eight frozen tools, native ACP"

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

    binary = {}
    for key, (goos, goarch, ext) in TARGETS.items():
        url = archive_url(tag, goos, goarch, ext)
        binary[key] = {
            "archive": url,
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
        if bad:
            print(f"not published for {tag}: {', '.join(bad)} — the registry needs all six", file=sys.stderr)

    agent = {
        "id": IDENT,
        "name": IDENT,
        "version": version,
        "description": DESCRIPTION,
        "repository": REPO,
        "website": args.website,
        "authors": ["adeotek"],
        "license": "MIT",
        "icon": "icon.svg",
        "distribution": {"binary": binary},
    }

    out = pathlib.Path(args.out) / IDENT
    out.mkdir(parents=True, exist_ok=True)
    (out / "agent.json").write_text(json.dumps(agent, indent=2) + "\n", encoding="utf-8")
    icon = root / "docs" / "assets" / "icon.svg"
    (out / "icon.svg").write_text(icon.read_text(encoding="utf-8"), encoding="utf-8")
    print(f"wrote {out}/agent.json and {out}/icon.svg for {tag}")
    # A --verify run that found unpublished targets exits 1: the files are
    # written either way, so they can be inspected, but the caller must not
    # mistake "rendered" for "ready to submit".
    return 1 if bad else 0


if __name__ == "__main__":
    raise SystemExit(main())
