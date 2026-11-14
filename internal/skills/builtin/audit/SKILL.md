---
name: audit
description: Audit the whole repository in the same four-bullet shape — what it does, the load it expects, and the findings that matter at scale.
argument-hint: "[path|package|focus]"
---
<!-- built into moca: this copy follows the binary and an edit here does not survive an update. `moca skills fork audit` copies it into your own skills dir, where the copy shadows it. -->

Audit the repository; change no code. Map before you judge: the README, the build and deploy config, the dependencies, the entry points, the test layout. State the load the project actually expects (one user on a laptop, or many processes against one database) — the README is the evidence, not your guess.

Then trace the main flows end to end, and read **in full** the paths that carry user input, money, authentication, data writes, background jobs and shared state. Those are where an audit finds something a diff review cannot.

Look in this order:
1. Bugs: a flow that cannot complete as the README describes, a caller that passes what the callee cannot accept, a fix present in one path and missing in its sibling.
2. Risk: security holes, data loss, an authorization check that a new route skipped.
3. Scale: check-then-write races, work every process repeats, memory or disk that only grows, a query per item, O(n²) on the input the README implies.
4. Risky logic with no test — a branch, a loop, a parser, money or security with nothing covering it.
5. Lean: dead code, a helper the repo already has, stdlib or a platform feature instead of a dependency, an abstraction with one implementation, near-copies to merge. Never split by line count, never judge by file size.

Before calling anything unused, search the whole tree — tests, fixtures, config, string references, templates — and say what you searched. A false "this is dead" finding costs the reader more than it gives them. A `shortcut:` comment that names its limit is a decision, not a finding.

Every finding needs a concrete case: the input, the caller, or the customer flow that breaks. Re-read the code before writing the finding. Cap at 20 findings, most important first — a list longer than that is a rewrite, not an audit. Propose the smallest fix, prefer deleting code, never add a layer.

Reply shape: start with `What this repo does:` in two or three sentences, then the load you assumed and the flows you traced. Then numbered findings, grouped `Must fix` / `Should fix` / `Nice to have`. Each finding: a bold title with `file:lines`, then exactly four bullets — `What this is` / `Problem` / `Fix` / `If we skip it`. End with `Verdict:`, then `Not checked:` naming anything that mattered and you could not check. Nothing found: `Looks solid. Ship.`
