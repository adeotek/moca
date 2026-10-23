#!/usr/bin/env python3
"""Render the winget manifests for a release: three YAML files for `microsoft/winget-pkgs`.

Usage:
  scripts/release/render-winget-manifests.py --tag v0.7.9 --out /tmp/winget

The output directory is what a pull request adds under
`manifests/a/adeotek/moca/<version>/`:

  adeotek.moca.yaml               version manifest
  adeotek.moca.installer.yaml     installer manifest (the two windows zips)
  adeotek.moca.locale.en-US.yaml  default locale manifest

Everything version-specific is derived from the release's own `checksums.txt`, so the
`InstallerSha256` values cannot disagree with the archives they name.

Two YAML traps are encoded here rather than left to whoever edits the file by hand:

- a value containing `: ` must be quoted, or the line parses as a nested mapping
  (`ShortDescription` is exactly that shape);
- `ReleaseDate` must be quoted, or YAML types it as a date and winget's 1.12 schema
  rejects the manifest for not being a string.

`--verify` HEADs both archive URLs. Neither `winget validate` nor `winget install` can
run off Windows, so those stay CI's job — the manifests are otherwise checked against
the official schemas (`https://aka.ms/winget-manifest.*.1.12.0.schema.json`).
"""
from __future__ import annotations

import argparse
import json
import pathlib
import sys
import urllib.error
import urllib.request

IDENT = "adeotek.moca"
REPO = "https://github.com/adeotek/moca"
MANIFEST_VERSION = "1.12.0"
PUBLISHER = "adeotek"
AUTHOR = "George Benjamin-Schonberger"
COPYRIGHT = "Copyright (c) 2026 George Benjamin-Schonberger"
SHORT_DESCRIPTION = "A coding agent in one static Go binary: eight frozen tools, native ACP, any OpenAI-compatible model"
# one entry per rendered line: a block scalar keeps the sentences apart instead of
# running them into one wall of text in a client's package pane
DESCRIPTION = (
    "moca is a coding agent for the terminal with eight frozen tools, MCP without prompt bloat, and "
    "native Agent Client Protocol support so editors such as Zed and VS Code can drive it.",
    "It runs against Claude, ChatGPT, Ollama, OpenCode Zen, or any OpenAI-compatible endpoint, and keeps "
    "every session in plain files you can read.",
    "`moca update` replaces itself from the latest GitHub release; this package installs the same prebuilt binary.",
)
TAGS = ["cli", "go", "ai", "coding-agent", "acp", "terminal"]

# winget architecture -> (goarch, extension)
TARGETS = {"x64": ("amd64", "zip"), "arm64": ("arm64", "zip")}


def archive_name(tag: str, goarch: str, ext: str) -> str:
    return f"moca-{tag}-windows-{goarch}.{ext}"


def archive_url(tag: str, goarch: str, ext: str) -> str:
    return f"{REPO}/releases/download/{tag}/{archive_name(tag, goarch, ext)}"


def fetch_checksums(tag: str) -> dict[str, str]:
    """The release's own checksums.txt, as {asset: sha256} (upper-cased: winget's form)."""
    url = f"{REPO}/releases/download/{tag}/checksums.txt"
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:  # noqa: S310
            text = resp.read().decode("utf-8")
    except urllib.error.URLError as exc:
        raise SystemExit(f"cannot read {url}: {exc}\npublish the release before rendering its manifests")
    sums: dict[str, str] = {}
    for line in text.splitlines():
        parts = line.split()
        if len(parts) != 2:
            continue
        sha, name = parts[0].lower(), parts[1].lstrip("*")
        if len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha):
            raise SystemExit(f"checksums.txt carries a malformed sha256 for {name}: {sha!r}")
        sums[name] = sha.upper()
    if not sums:
        raise SystemExit(f"{url} lists no checksums")
    return sums


def ystr(value: str) -> str:
    """A YAML scalar, quoted whenever it could be misread (JSON double-quote syntax is valid YAML)."""
    needs = (
        ": " in value
        or value.endswith(":")
        or " #" in value
        or value[:1] in tuple("{}[]*&!%@`\"'|>-?")
        or value[:1].isdigit()
        or value != value.strip()
    )
    return json.dumps(value, ensure_ascii=False) if needs else value


def render(tag: str, sums: dict[str, str], date: str) -> tuple[dict[str, str], list[str]]:
    version = tag[1:]
    missing = [archive_name(tag, a, x) for (a, x) in TARGETS.values() if archive_name(tag, a, x) not in sums]

    installers = []
    for arch, (goarch, ext) in TARGETS.items():
        installers.append(
            f"- Architecture: {arch}\n"
            f"  InstallerUrl: {archive_url(tag, goarch, ext)}\n"
            f"  InstallerSha256: {sums.get(archive_name(tag, goarch, ext), '0' * 64)}"
        )

    files = {
        f"{IDENT}.yaml": (
            f"# yaml-language-server: $schema=https://aka.ms/winget-manifest.version.{MANIFEST_VERSION}.schema.json\n"
            f"\n"
            f"PackageIdentifier: {IDENT}\n"
            f"PackageVersion: {version}\n"
            f"DefaultLocale: en-US\n"
            f"ManifestType: version\n"
            f"ManifestVersion: {MANIFEST_VERSION}\n"
        ),
        f"{IDENT}.installer.yaml": (
            f"# yaml-language-server: $schema=https://aka.ms/winget-manifest.installer.{MANIFEST_VERSION}.schema.json\n"
            f"\n"
            f"PackageIdentifier: {IDENT}\n"
            f"PackageVersion: {version}\n"
            f"InstallerType: zip\n"
            f"NestedInstallerType: portable\n"
            f"NestedInstallerFiles:\n"
            f"- RelativeFilePath: moca.exe\n"
            f'ReleaseDate: "{date}"\n'
            f"Installers:\n" + "\n".join(installers) + "\n"
            f"ManifestType: installer\n"
            f"ManifestVersion: {MANIFEST_VERSION}\n"
        ),
        f"{IDENT}.locale.en-US.yaml": (
            f"# yaml-language-server: $schema=https://aka.ms/winget-manifest.defaultLocale.{MANIFEST_VERSION}.schema.json\n"
            f"\n"
            f"PackageIdentifier: {IDENT}\n"
            f"PackageVersion: {version}\n"
            f"PackageLocale: en-US\n"
            f"Publisher: {PUBLISHER}\n"
            f"PublisherUrl: https://github.com/{PUBLISHER}\n"
            f"PublisherSupportUrl: {REPO}/issues\n"
            f"Author: {ystr(AUTHOR)}\n"
            f"PackageName: moca\n"
            f"PackageUrl: {REPO}\n"
            f"License: MIT\n"
            f"LicenseUrl: {REPO}/blob/HEAD/LICENSE\n"
            f"Copyright: {ystr(COPYRIGHT)}\n"
            f"ShortDescription: {ystr(SHORT_DESCRIPTION)}\n"
            f"Description: |-\n"
            + "".join(f"  {line}\n" for line in DESCRIPTION)
            + "Tags:\n"
            + "".join(f"- {t}\n" for t in TAGS)
            + f"ReleaseNotesUrl: {REPO}/releases/tag/{tag}\n"
            f"ManifestType: defaultLocale\n"
            f"ManifestVersion: {MANIFEST_VERSION}\n"
        ),
    }
    return files, missing


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--tag", required=True, help="release tag, e.g. v0.7.9")
    ap.add_argument("--out", required=True, help="directory to write the three manifests into")
    ap.add_argument("--date", default=None, help="ReleaseDate (YYYY-MM-DD; default: today, UTC)")
    ap.add_argument("--verify", action="store_true", help="HEAD both archive URLs")
    args = ap.parse_args()

    tag = args.tag.strip()
    if not tag.startswith("v") or tag.count(".") != 2:
        print(f"tag must look like vX.Y.Z, got {tag!r}", file=sys.stderr)
        return 2

    import datetime

    date = args.date or datetime.datetime.now(datetime.UTC).strftime("%Y-%m-%d")
    files, missing = render(tag, fetch_checksums(tag), date)

    bad: list[str] = []
    if args.verify:
        for arch, (goarch, ext) in TARGETS.items():
            url = archive_url(tag, goarch, ext)
            req = urllib.request.Request(url, method="HEAD")
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:  # noqa: S310
                    code = resp.status
            except Exception as exc:  # noqa: BLE001 - a 404 is the answer we want to report
                code = getattr(exc, "code", str(exc))
            print(f"{arch}: {code} {url}", file=sys.stderr)
            if code != 200:
                bad.append(arch)

    out = pathlib.Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    for name, content in files.items():
        (out / name).write_text(content, encoding="utf-8")
    print(f"wrote {len(files)} manifests for {tag} into {out}")

    if missing:
        print(f"checksums.txt has no entry for: {', '.join(missing)}", file=sys.stderr)
    if bad:
        print(f"not published for {tag}: {', '.join(bad)} windows archives", file=sys.stderr)
    return 1 if (bad or missing) else 0


if __name__ == "__main__":
    raise SystemExit(main())
