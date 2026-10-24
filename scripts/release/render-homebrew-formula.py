#!/usr/bin/env python3
"""Render the Homebrew formula for a release: `Formula/moca.rb`, ready for a tap.

Usage:
  scripts/release/render-homebrew-formula.py --tag v0.7.8 --out /tmp/homebrew-tap

A formula installs a prebuilt tarball only when one exists, which is why the darwin
packages were added to the release workflow: without them every macOS user needs a
Go toolchain on their machine. The four URLs and their sha256 come from the release's
own `checksums.txt`, so the formula cannot disagree with the published archives — and
a tag missing a package exits 1 naming it.

The formula declares no `version`: Homebrew scans it out of the archive URL, and
`brew audit --strict` rejects the redundant stanza. `livecheck` comes before the
platform blocks for the same reason (ComponentsOrder). The output is what
`adeotek/homebrew-tap` carries: `Formula/moca.rb`. Install with
`brew install adeotek/tap/moca`; that copy then refuses `moca update` by design
(`update.PackageManaged` prints `brew upgrade moca`).
"""
from __future__ import annotations

import argparse
import pathlib
import sys
import urllib.error
import urllib.request

REPO = "https://github.com/adeotek/moca"
DESC = "Coding agent in one static Go binary"

# (brew platform block, cpu block) -> (goos, goarch, extension)
TARGETS = {
    ("on_macos", "on_arm"): ("darwin", "arm64", "tar.gz"),
    ("on_macos", "on_intel"): ("darwin", "amd64", "tar.gz"),
    ("on_linux", "on_arm"): ("linux", "arm64", "tar.gz"),
    ("on_linux", "on_intel"): ("linux", "amd64", "tar.gz"),
}


def archive_name(tag: str, goos: str, goarch: str, ext: str) -> str:
    return f"moca-{tag}-{goos}-{goarch}.{ext}"


def archive_url(tag: str, goos: str, goarch: str, ext: str) -> str:
    return f"{REPO}/releases/download/{tag}/{archive_name(tag, goos, goarch, ext)}"


def fetch_checksums(tag: str) -> dict[str, str]:
    """The release's own checksums.txt, as {asset: sha256}."""
    url = f"{REPO}/releases/download/{tag}/checksums.txt"
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:  # noqa: S310
            text = resp.read().decode("utf-8")
    except urllib.error.URLError as exc:
        raise SystemExit(f"cannot read {url}: {exc}\npublish the release before rendering its formula")
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


def render(tag: str, sums: dict[str, str]) -> tuple[str, list[str]]:
    missing = [
        archive_name(tag, *t) for t in TARGETS.values() if archive_name(tag, *t) not in sums
    ]

    lines = [
        "# This file is generated — edit scripts/release/render-homebrew-formula.py in",
        f"# {REPO} and re-run it, do not hand-edit the tap.",
        "class Moca < Formula",
        f'  desc "{DESC}"',
        f'  homepage "{REPO}"',
        '  license "MIT"',
        "",
        "  livecheck do",
        "    url :stable",
        "    strategy :github_latest",
        "  end",
        "",
    ]
    for platform in ("on_macos", "on_linux"):
        lines.append(f"  {platform} do")
        for cpu in ("on_arm", "on_intel"):
            goos, goarch, ext = TARGETS[(platform, cpu)]
            name = archive_name(tag, goos, goarch, ext)
            lines.append(f"    {cpu} do")
            lines.append(f'      url "{archive_url(tag, goos, goarch, ext)}"')
            lines.append(f'      sha256 "{sums.get(name, "0" * 64)}"')
            lines.append("    end")
            lines.append("")
        lines.pop()  # no blank line before the closing `end`
        lines.append("  end")
        lines.append("")
    lines += [
        "  def install",
        '    bin.install "moca"',
        "  end",
        "",
        "  test do",
        '    assert_match "moca v#{version}", shell_output("#{bin}/moca --version")',
        "  end",
        "end",
    ]
    return "\n".join(lines) + "\n", missing


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--tag", required=True, help="release tag, e.g. v0.7.8")
    ap.add_argument("--out", required=True, help="tap checkout to write Formula/moca.rb into")
    args = ap.parse_args()

    tag = args.tag.strip()
    if not tag.startswith("v") or tag.count(".") != 2:
        print(f"tag must look like vX.Y.Z, got {tag!r}", file=sys.stderr)
        return 2

    formula, missing = render(tag, fetch_checksums(tag))
    out = pathlib.Path(args.out) / "Formula"
    out.mkdir(parents=True, exist_ok=True)
    (out / "moca.rb").write_text(formula, encoding="utf-8")
    print(f"wrote {out / 'moca.rb'} for {tag}")
    if missing:
        print(f"not published for {tag}: {', '.join(missing)} — the formula needs darwin and linux", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
