#!/usr/bin/env python3
"""Fail a pull request whose commits are authored or co-authored by an AI agent.

GitHub resolves a `Co-authored-by` trailer to the agent's account and credits it
in the repository's Contributors widget. That attribution is a one-way door:
rewriting the branch does not remove it, because tags, PR refs and GitHub's
commit index keep the old commits reachable. So the trailer has to be refused
before it lands, not cleaned up afterwards.

Only the author identity and `Co-authored-by` trailers are inspected:

  * a body that merely mentions an agent ("reviewed with Claude") is fine;
  * the committer is ignored — GitHub web-flow commits every squash merge.

Usage:
    check-agent-authors.py --range origin/main..HEAD
    check-agent-authors.py --selftest
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys

# Matched, case-insensitively, against "<name> <email>". Extend this table when
# a new coding agent starts shipping commits; keep each pattern narrow enough
# that a human contributor does not collide with it.
AGENT_PATTERNS: list[tuple[str, str]] = [
    # Product names, which is how the agents name themselves in the trailer.
    (r"\bclaude\b", "Claude"),
    (r"\bcopilot\b", "Copilot"),
    (r"\bcodex\b", "Codex"),
    (r"\bcursor[\s-]?agent\b", "Cursor Agent"),
    (r"\bdevin\b", "Devin"),
    (r"\bjules\b", "Jules"),
    (r"\bgemini\b", "Gemini"),
    (r"\bhermes[\s-]?agent\b", "Hermes Agent"),
    (r"\baider\b", "Aider"),
    (r"\bchatgpt\b", "ChatGPT"),
    (r"\b(?:amazon|aws)[\s-]?q\b", "Amazon Q"),
    (r"\bwindsurf\b", "Windsurf"),
    (r"\bcline\b", "Cline"),
    # Addresses that only an agent uses, whatever name it puts in front.
    (r"@anthropic\.com\b", "the Anthropic agent address"),
    (r"@cursor\.com\b", "the Cursor agent address"),
    (r"@nousresearch\.com\b", "the Nous agent address"),
    (r"codex@openai\.com\b", "the Codex address"),
    (r"copilot@github\.com\b", "the Copilot address"),
    (r"\+copilot@users\.noreply\.github\.com\b", "the Copilot address"),
]

TRAILER = re.compile(r"(?im)^[ \t]*co-authored-by:[ \t]*(.+?)[ \t]*(?:<([^>\s]+)>)?[ \t]*$")

REMEDY = (
    "rewrite the branch and drop the trailer (git rebase -i --exec, or\n"
    "  git filter-repo --message-callback to strip Co-authored-by lines), then force-push.\n"
    "  Attribution already merged cannot be removed by another rewrite: tags, PR refs\n"
    "  and GitHub's commit index keep the old commits reachable — see AGENTS.md."
)


def git(*args: str) -> str:
    proc = subprocess.run(("git",) + args, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(proc.stderr.strip() or f"git {' '.join(args)} failed")
    return proc.stdout


def identities(sha: str) -> list[tuple[str, str]]:
    """(role, "<name> <email>") for the author and every co-author."""
    name = git("show", "-s", "--format=%an", sha).strip()
    email = git("show", "-s", "--format=%ae", sha).strip()
    found = [("author", f"{name} <{email}>")]
    body = git("show", "-s", "--format=%B", sha)
    for match in TRAILER.finditer(body):
        who = match.group(1).strip()
        addr = match.group(2)
        found.append(("co-author", f"{who} <{addr}>" if addr else who))
    return found


def agent(identity: str) -> str | None:
    """The deny-list label matching this identity, or None."""
    for pattern, label in AGENT_PATTERNS:
        if re.search(pattern, identity, re.IGNORECASE):
            return label
    return None


def check(range_: str) -> int:
    try:
        shas = git("log", "--format=%H", range_).split()
    except RuntimeError as exc:
        print(f"error: cannot read the commit range {range_!r}: {exc}", file=sys.stderr)
        print("hint: the base branch needs full history (actions/checkout fetch-depth: 0).", file=sys.stderr)
        return 2

    if not shas:
        print(f"no commits in {range_}; nothing to check")
        return 0

    violations = 0
    for sha in shas:
        for role, identity in identities(sha):
            label = agent(identity)
            if label is None:
                continue
            subject = git("show", "-s", "--format=%s", sha).strip()
            print(
                f"error: {sha[:12]} {role} is {identity} ({label})\n"
                f"  {subject}\n"
                f"  fix: {REMEDY}",
                file=sys.stderr,
            )
            violations += 1

    if violations:
        print(
            f"\n{violations} agent attribution(s) in {len(shas)} commit(s) on {range_}.",
            file=sys.stderr,
        )
        return 1
    print(f"ok: {len(shas)} commit(s) on {range_}, no agent authors or co-authors")
    return 0


CASES: list[tuple[str, bool]] = [
    ("Claude <noreply@anthropic.com>", True),
    ("Claude Opus 5.5 <noreply@anthropic.com>", True),
    ("Copilot <copilot@github.com>", True),
    ("Copilot <175728472+Copilot@users.noreply.github.com>", True),
    ("Codex <codex@openai.com>", True),
    ("cursoragent <cursoragent@cursor.com>", True),
    ("Devin <devin@example.com>", True),
    ("google-labs-jules[bot] <161369871+google-labs-jules[bot]@users.noreply.github.com>", True),
    ("Hermes Agent (deepseek-v4.1-flash) <noreply@nousresearch.com>", True),
    ("aider <aider@example.com>", True),
    ("Claude Sonnet <someone@example.com>", True),  # named after the product; still refused
    ("George Benjamin-Schonberger <george.benjamin@gmail.com>", False),
    ("dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>", False),
    ("GitHub <noreply@github.com>", False),
    ("Claudia Sava <claudia@example.com>", False),  # no word boundary inside a longer name
    ("Mihai Codreanu <mihai@openai.example.com>", False),
]


def selftest() -> int:
    failed = 0
    for identity, expected in CASES:
        got = agent(identity) is not None
        if got != expected:
            failed += 1
            print(f"FAIL {identity!r}: expected agent={expected}, got {got}", file=sys.stderr)

    trailers = [
        ("subject\n\nbody\n\nCo-authored-by: Claude <noreply@anthropic.com>", 1),
        ("subject\n\nCo-authored-By: Claude Opus 5.5 <noreply@anthropic.com>", 1),
        ("subject\n\n  co-authored-by:   Copilot <copilot@github.com>  ", 1),
        ("subject\n\nCo-authored-by: Jane Doe <jane@example.com>", 1),
        ("subject\n\nreviewed with Claude, no trailer", 0),
    ]
    for body, expected in trailers:
        got = len(TRAILER.findall(body))
        if got != expected:
            failed += 1
            print(f"FAIL trailer parse {body!r}: expected {expected}, got {got}", file=sys.stderr)

    print(f"selftest: {len(CASES) + len(trailers) - failed} passed, {failed} failed")
    return 1 if failed else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--range", default="origin/main..HEAD", help="git revision range (default: origin/main..HEAD)")
    parser.add_argument("--selftest", action="store_true", help="check the deny list and trailer parser")
    args = parser.parse_args()
    if args.selftest:
        return selftest()
    return check(args.range)


if __name__ == "__main__":
    sys.exit(main())
