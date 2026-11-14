---
name: review
description: Review the current changes like the senior dev who gets paged when they break — bugs, risk, scale, missing tests, speed, and what to cut.
argument-hint: "[uncommitted|staged|branch|pr <url>|files…]"
---
<!-- built into moca: this copy follows the binary and an edit here does not survive an update. `moca skills fork review` copies it into your own skills dir, where the copy shadows it. -->

Review the change; change no code. The user names the target in plain words; with none, review the uncommitted change, or the last commit when the tree is clean. Get the diff with `shell` (`git diff`, `git diff --staged`, `git diff <base>...HEAD`, `git log -1 -p`).

Read first, review second. For every changed function, read its callers — a change can break code it does not touch — and the tests that cover it. Read the project instructions and the README for the load the project expects (one user, or many processes).

Then look, in this order:
1. Bugs: wrong result, crash, a missed edge case, a caller broken by the change, a fix applied in one caller while the shared function stays wrong.
2. Risk: security holes, data loss, an authorization check that moved.
3. Scale: check-then-write races, work every process repeats, memory that only grows, a query per item, O(n²) on the input the README implies.
4. Risky logic with no test — logic that has a branch, a loop, a parser, money or security in it.
5. Speed: big problems as problems, small ones as suggestions.
6. Lean: dead code, a helper the repo already has, stdlib or a platform feature instead of a new dependency, an abstraction with one implementation, near-copies to merge, one function doing several unrelated jobs to split. Never split by line count.

Every finding needs a concrete case — the input that triggers it, or the caller that breaks. Re-read the lines before you write the finding; a plausible-sounding finding that does not survive a re-read is worse than no finding. A `shortcut:` comment that names its limit is a decision, not a finding. Propose the smallest fix, prefer deleting code, never add a layer. No style taste, no vague worries.

Reply shape: start with `What this change does:` in two or three sentences, then the load you assumed. Then numbered findings, grouped `Must fix` / `Should fix` / `Nice to have`, most important first. Each finding: a bold title with `file:lines`, then exactly four bullets — `What this is` / `Problem` / `Fix` / `If we skip it`. End with `Verdict:`, then `Lean: -N lines possible.` when the lean findings allow a count, and `Not checked:` naming anything that mattered and you could not check. Nothing found: `Looks good. Ship.`
