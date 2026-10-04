---
description: Review a completed moca phase against its plan, DESIGN.md and SPECS.md, and write a review document to docs/reviews/. Read-only apart from that one file.
argument-hint: <phase number or name> [base-ref, default main]
allowed-tools: Read, Grep, Glob, Write(docs/reviews/*), Bash(git diff:*), Bash(git log:*), Bash(git show:*), Bash(git status:*), Bash(git branch:*), Bash(git merge-base:*), Bash(git rev-parse:*), Bash(go vet:*), Bash(go test:*), Bash(gofmt -l:*), Bash(ls:*), Bash(date:*), Bash(mkdir -p docs/reviews)
---

# Review a completed phase

Review phase **$ARGUMENTS** of moca and write the result to a review document in `docs/reviews/`.

You are a reviewer, not an implementer. **The review document is the only thing you may create or change.**

- Do not edit, format, or delete any other file, including source, tests, plans, specs, README and earlier reviews.
- Do not run `gofmt -w`, `go mod tidy`, `go generate`, or anything else that rewrites files.
- Do not stage, commit, push, check out, stash, or otherwise change git state.
- Do not fix what you find. Propose the fix in the document and stop.

Current branch: !`git branch --show-current`
Working tree: !`git status --short`

## 1. Resolve the scope

1. Parse `$ARGUMENTS`: the first token is the phase (`2`, `phase-2`, or part of the name such as `tools`). An optional second token is the base ref to diff against (default `main`).
2. If no phase is given, infer it from the branch name (`phase/<N>-<name>`). If you still cannot tell, stop and ask the user which phase.
3. Find the plan: `docs/plans/phase-<N>-*.md`. If there is none, stop and say so.
4. Work out the review range: `git merge-base <base> HEAD`..`HEAD`. If the phase has already been merged into the base (the range is empty), say so and ask for a base ref or commit range instead of guessing.
5. Record the range, the commit list (`git log --oneline`), and `git diff --stat`.

## 2. Read the context first

Read these before reading the diff, so you know what "correct" means for this phase:

- The phase plan: Goal, Architecture, Global Constraints, **Review Focus**, every Task, and the phase gate task. Also any "Implementation notes" or deviation notes added after the plan was written.
- The sections of `docs/specs/DESIGN.md` the plan cites (`§` references), and the phase's entry in its phase list.
- `docs/specs/SPECS.md` (the current-state spec) and the README status line, if the phase claims to be done.
- Earlier documents in `docs/reviews/`. Check whether any of their findings recur in this phase, and whether findings marked fixed really are.

## 3. Review

Read the changed code in full (not just hunks) for every package the phase touches. Cover these areas, skipping any that do not apply:

1. **Plan conformance**
   - Is every Task implemented?
   - Does the phase gate actually pass? Is each Global Constraint honoured?
   - Are deviations from the plan recorded, and justified?
   - Is anything outside the phase's scope or on the non-goals list?
2. **Review Focus**
   - Take each item in the plan's Review Focus list and verify it yourself: find the code that satisfies it and the test that proves it.
   - A focus item with no test, or a test that cannot fail, is a finding.
3. **Correctness**
   - Error and cancellation paths, partial or truncated input and streams, retries, off-by-one and boundary values.
   - Resource leaks, goroutine lifetime, data races, context handling, and zero-value or nil handling.
   - State that survives a failure it should not.
4. **Security**
   - Secrets handling, path jail and symlink escapes, shell analysis and permission bypasses, trust gating, untrusted input reaching a shell, file path, or log.
   - Apply the project's honest threat model (DESIGN.md §7): permissions are guard rails, not a sandbox.
5. **Contract and spec conformance**
   - The CLI contract (stdout, stderr, exit codes), the config schema, the wire formats, and the data and session formats against DESIGN.md.
   - Anything in SPECS.md that no longer matches the code.
6. **Tests**
   - Do tests exercise real behaviour rather than mocks?
   - Are error paths and edge cases covered?
   - Are there tests that pass for the wrong reason, or leave out the case the plan called out?
7. **Structure**
   - Dependency direction between packages (DESIGN.md §2), exported surface, dead code, duplicated logic, and comments that no longer match the code.

Rules for findings:

- **Verify before you report.** Re-read the code behind every finding and trace the failure scenario through it. Drop anything you cannot ground in a concrete input and a concrete wrong outcome.
- Mark each finding **Confirmed** (you traced it, or reproduced it with a command) or **Plausible** (you could not fully trace it). Never present a guess as confirmed.
- Do not report style preferences, or problems that `gofmt` or `go vet` would flag, unless they hide a bug.
- Do not pad. A short review of a sound phase is a good outcome. Say what you checked and found fine.

## 4. Run the checks

Run these from the repository root and record the **actual** result of each, verbatim where it fails:

- `gofmt -l .`, which must print nothing.
- `go vet ./...`
- `go test -race -count=1 ./...`

If `go` is not on `PATH`, look for it under `/usr/local/go/bin` and call it by full path. If a command cannot run at all, write that in the document. Do not infer a result you did not see. Also run any phase-specific gate commands in the plan's gate task that are read-only and need no network, credentials, or live providers. List the live or manual gate steps you did **not** run as "not verified".

When you are done running checks, run `git status --short` and confirm the working tree is unchanged from the start of the review. If a check left files behind, report that as a finding and do not clean it up.

## 5. Write the review document

Write exactly one file: `docs/reviews/<YYYY-MM-DD>-phase-<N>-<slug>.md`.

- Take the date from `date +%F`, and `<slug>` from the plan filename (`phase-2-tools-agent-loop.md` gives `tools-agent-loop`).
- If that file already exists, do **not** overwrite it. Use the suffix `-pass-2`, `-pass-3` and so on, and say in the document which earlier review it follows up.
- Create `docs/reviews/` with `mkdir -p` if it is missing.

Use this structure. Keep each finding to what a maintainer needs to act on it:

```markdown
# Review: phase <N> — <name>

Date: <YYYY-MM-DD>
Branch: <branch>   Range: <base>..<head> (<n> commits, <files> files changed)
Plan: docs/plans/<file>   Spec: DESIGN.md <§ list>
Follows up: <earlier review file, or "none">

## Verdict

<Approve | Approve with fixes | Block> — one or two sentences on why.

## Summary

<What the phase delivers, what was checked, overall quality. Counts by severity.>

## Checks run

| Check | Result |
|---|---|
| gofmt -l | clean / <output> |
| go vet ./... | ... |
| go test -race -count=1 ./... | <passed/failed counts, failing test names> |
| <phase gate command> | ... |

Not verified: <live/manual gate steps, anything that could not run, and why>

## Plan conformance

<Task-by-task or gate-by-gate status. Deviations and whether they are recorded.>

## Review Focus

<One line per focus item: satisfied / not satisfied, with the code location and the test that proves it.>

## Findings

### High
#### 1. <short title> — Confirmed|Plausible
`path/to/file.go:LINE`

<What is wrong, in one or two sentences.>

Failure scenario: <concrete input or state and the wrong outcome.>

Proposed fix: <the smallest change that fixes it.>

### Medium
...
### Low
...

(Omit a severity heading that has no findings. If there are none, say "No findings.")

## Earlier findings

<Status of each finding from earlier reviews that touches this phase: still fixed / regressed / still open.>

## Suggested order of fixes

<Only if there are findings.>
```

Severity: **High** means data loss, a security bypass, a crash, a contract violation, or a broken phase gate. **Medium** means wrong behaviour in a realistic case. **Low** means an edge case, a missing test, or a maintainability risk.

## 6. Finish

Confirm with `git status --short` that the only change is the new review file. Then reply in chat with a short summary and not the document:

- the verdict,
- finding counts by severity,
- the path of the review file,
- anything you could not verify.

Do not apply fixes, and do not offer to commit. If the user asks for the fixes afterwards, that is a separate task.
