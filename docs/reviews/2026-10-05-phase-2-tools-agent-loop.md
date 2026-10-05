# Review: phase 2 — seven tools, agent loop, permissions, skills, sessions

Date: 2026-10-05
Branch: phase/2-tools-agent-loop   Range: main...HEAD (merge-base c3a5e56; 19 commits, 63 files, +4907/−58)
Plan: docs/plans/phase-2-tools-agent-loop.md   Spec: DESIGN.md §2, §4, §5, §7, §8, §9, §9.5, §12.5, §13; docs/specs/SPECS.md §10–§15
Follows up: docs/reviews/2026-10-04-phase-1-skeleton-providers.md (only where a phase-1 finding touches phase 2; see "Earlier findings")

## Verdict

**Block.** The test suite is green and the structure follows the plan, but three of the checks this phase exists to provide can be bypassed (path jail, shell hard-deny, shell redirect jail), and a model reply cut off mid tool call ends the run with an error instead of recovering, which is the opposite of what Review Focus item 4 requires. The last three were reproduced by running the real packages; the jail escape was traced through the code and matches files an earlier manual probe left on this machine (finding 1).

## Summary

Phase 2 delivers the seven frozen tools, a symlink-resolving jail, `mvdan.cc/sh`-based command analysis, trust and yolo, skills and AGENTS.md loading, JSONL sessions with snapshots, and the agent loop behind `moca -p`. The package layout and dependency direction match DESIGN.md §2, the tool schemas are frozen by a golden test, and the unit gates named in the plan all exist and pass.

I read every non-test file in the range, read the test files for the gate and Review Focus items, and ran targeted reproductions against the real packages from a throwaway module in `/tmp` (since deleted; the repository was not modified). Several guarantees the code and SPECS.md state are false in specific cases:

- 4 High: a dangling-symlink jail escape, a shell wrapper-parsing bypass of the hard-deny and `rm` rules, redirect-jail bypasses (`cd` then a relative target, and `<>`), and the truncated-tool-call crash.
- 6 Medium: output truncation that returns nothing, an unbounded shell capture and a timeout that can be outrun, quoted and escaped command names that skip hard-deny, `search` ignoring ancestor `.gitignore`, whole-file reads without a size cap, and a session reopen that silently loses entries.
- 5 Low: skill discovery diagnostics and symlinked skills, a wrap-up turn that leaves calls unanswered together with `Start` errors mapped to exit 2, a missed hint in `edit`, builtin extraction races, and a stray untracked probe.

## Checks run

| Check | Result |
|---|---|
| `gofmt -l .` | prints `internal/permissions/probe/main.go` only. That file is untracked and not part of the branch (Low finding 15); every tracked file is clean |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | 156 passed in 10 packages, 0 failed |
| Gate Step 1 unit gates (`TestEditLadder`, `TestWriteNewFileNoReadNeeded`, `TestWriteExistingNeedsReadAndFreshness`, `TestShellAnalysisLadder`, `TestJail`, `TestNoApproveSkipsProjectInstructions`, `TestMaxStepsWrapUp`, `TestSchemasFrozen`, `TestYoloTurnsEveryCheckOff`) | all nine exist and pass inside the run above |
| Targeted reproductions | run for findings 2–7, 8, 10, 11, 12 (details under each finding); finding 1 traced, not re-run |
| `git status --short` after the checks | unchanged from the start: `?? .claude/` and `?? internal/permissions/probe/` (both untracked before the review began) |

Not verified: gate Step 2 and Step 3 (live gates: a global skill body loaded via `read`, and a real fix loop against `opencode-go/glm-5.3-flash`). They need a real provider key and network access; SPECS.md §15 records them as passed on 2026-10-05, and I have not confirmed that. Windows behaviour (`shell_windows_rules.go`, `shellrun_windows.go`, `within` on Windows) was read but not run.

## Plan conformance

| Task | Status |
|---|---|
| 1 Tool interface, registry, schema golden | Done. `Registry.Run` rejects unknown tools and invalid JSON; the golden test freezes the schemas |
| 2 Path jail + read-only roots | Done, but `resolveDeep` has the dangling-symlink hole (finding 1) |
| 3 `read` + ReadTracker | Done. No input size cap (finding 9) |
| 4 `ls` | Done |
| 5 gitignore matcher + `search` | Done, except ancestor ignore files are not applied (finding 8) |
| 6 Edit ladder | Done; `N|` hint present for the fuzzy path, missing for `replace_all` (finding 13) |
| 7 `write` + `edit` with read guard and snapshots | Done; nested new directories work; inherits finding 1 |
| 8 Shell analysis | Done, with the bypasses in findings 2, 3 and 7 |
| 9 Shell runner + `shell` | Done, with findings 5 and 6 |
| 10 `mcp` stub + freeze | Done. Tool order `read write edit shell search ls mcp` matches DESIGN.md §4 |
| 11 Trust store | Implemented and tested. Not consumed anywhere in production code, which matches the plan (`-p` uses flags only; the TUI uses saved decisions in phase 3, SPECS.md §11) |
| 12 Skills, builtins, AGENTS.md | Done, with findings 11 and 14 |
| 13 Session schema, writer, rebuild, repair | Done, with finding 10 |
| 14 Snapshots + undo | Done. Git-clean skip, undo refusal when the file changed, and prune all behave as specified |
| 15 System prompt | Done. Built once, stored in the session entry, no hostname |
| 16 Agent loop + `Start` | Done, with findings 4 and 12 |
| 17 Yolo | Done. Implemented by swapping checkers, not by branching inside tools, as specified |
| 18 `-p` on the agent | Done. Exit 3 for maxSteps, trust rule (flag, then yolo, then no) as planned; `Start` errors map to exit 2 (finding 12) |
| 19 Gate | Unit gates pass; live gates not verified here |

Deviations from the plan are recorded in SPECS.md (new sections 10–15). Two statements there do not hold: §14 "a `length`-stopped turn executes nothing — each call gets the §6 error result" (false for the common truncated-JSON case, finding 4) and §12 "a skill without a description is skipped and reported" (the report is discarded, finding 11).

## Review Focus

1. **`edit` with `N|` prefixes attached** — satisfied for the normal path: `replaceFuzzy` appends the "drop them" hint when `lineNumPrefix` matches (`internal/tools/editapply.go:158`), tested by the "no match with line-number prefixes" subtest in `editapply_test.go:73`. Not satisfied when `replace_all` is set (finding 13).
2. **`write` into a new directory / symlink parent refused before mkdir** — satisfied for new nested directories (`TestWriteNewFileNoReadNeeded` writes `pkg/new/x.go`) and for a parent that is an existing symlink out of the jail (`TestJail`, `escape/new.txt`). **Not satisfied** for a parent or target that is a dangling symlink (finding 1); there is no test for it.
3. **`shell` output with no trailing newline, binary garbage, large stderr** — satisfied for no trailing newline (`TestRunShellBasics`), invalid UTF-8 (`TestRunShellInvalidUTF8`) and the exit code (always reported in `shell.go`). **Not satisfied** when the output is dominated by a long line: `Truncate` returns only `[… 1 lines omitted]` (finding 5). `TestTruncate` uses 10,000 short lines only.
4. **Truncated or unknown tool call → error result, loop continues, transcript stays valid** — unknown tool: satisfied (`registry.Run`). **Not satisfied** for a call truncated at `max_tokens` from a real adapter: the run fails (finding 4). `TestLengthStopTruncatedToolCall` passes without exercising the path.
5. **SKILL.md CRLF, BOM, folded/literal descriptions, unknown keys** — satisfied. `TestFrontmatter` covers BOM, CRLF, `>` folded, `allowed-tools` and a `metadata:` map.

## Findings

### High

#### 1. Dangling symlink lets `write`/`edit` create files outside the workdir — Confirmed
`internal/permissions/jail.go:47` (`resolveDeep`)

`resolveDeep` calls `filepath.EvalSymlinks` and, on "not exist", treats the final path component as a new file by walking up to the parent and re-appending the name. A symlink whose target does not exist also returns "not exist", so the link itself is re-appended as if it were a plain new file. `Resolve` then returns the in-jail link path, and `os.WriteFile` follows the link out of the jail.

Failure scenario (traced through `Resolve`, `write.go` and `ReadTracker.Check`; I did not re-run the probe): the workdir contains `link-dangle -> /tmp/moca-ext/newfile.txt` (for example from a cloned repository). The model calls `write` with `{"path":"link-dangle","content":"x"}`. `Resolve` accepts it, `os.Stat` fails so the directory check is skipped, `ReadTracker.Check` treats "not exist" as a new file, and the write creates `/tmp/moca-ext/newfile.txt`. This is already visible on this machine: the leftover from the untracked probe is `/tmp/moca-ext/newfile.txt` containing `pwned`, reached through `/tmp/moca-int/link-dangle`. The same applies to `edit`'s `write` step and to a dangling link used as a parent directory.

Proposed fix: in `resolveDeep`, when `EvalSymlinks` fails with "not exist", `os.Lstat` the component. If it exists and is a symlink, `Readlink` it, resolve a relative target against the link's directory, and keep resolving (with a hop limit such as 40, then refuse). Re-append the remaining tail only after the link target is canonical. Add jail tests for a dangling link to a missing file, to a missing directory plus a subpath, a chain of dangling links, and a link loop, each for `write=true`.

#### 2. Shell wrapper-option parsing hides the real command, bypassing hard-deny and the `rm` prompt — Confirmed
`internal/permissions/shell.go:105` (`unwrap`, `case "env"` at 113, `case "command"` at 161)

`unwrap` skips wrapper options but not their arguments, and `command` does not skip `-p`. The word after the skipped option is taken as the wrapped command.

Failure scenario (all checked with `Shell.Check` using the default allowlist, `ls` allowlisted):
- `env -u ls sudo id` returns no approval and no error. The analyser classifies `ls`; bash runs `sudo`.
- `env -u ls rm -rf x` returns no approval and no error, so the "ask every time" rule for `rm` is skipped.
- `env -u FOO sudo id` asks to approve a command called `FOO`; `command -p sudo id` asks for `-p`; `env -C /tmp rm -rf x` asks for `tmp`. Approving "always" calls `Commands.Allow("-p")`, which then lets `command -p <anything>` through without a prompt.

DESIGN.md §7 states hard-deny has "no override" and that the wrappers are unwrapped and the wrapped command is checked.

Proposed fix: make `unwrap` consume each wrapper's options and option arguments exactly (`env`: `-u NAME`, `-C DIR`, `-S STR`, `-i`, `-0`, `-v`, `--unset=`, `--chdir=`; `command`: `-p`; `time`: `-f FMT`, `-o FILE`; and so on). For any option it cannot model, fail closed with a refusal such as "cannot analyse options of `env`" instead of guessing. Add ladder cases for each of the commands above, including that the hard-deny and `rm` verdicts survive them.

#### 3. Shell redirect checks can be bypassed with `cd` and with `<>` — Confirmed
`internal/permissions/shell.go:175` (`checkTarget`), `:212` (redirect operator switch)

Two independent gaps:
- Redirect and `tee` targets are resolved against the jail root, but `cd` is an implicit builtin, so a relative target after `cd` points somewhere else at run time.
- `syntax.RdrInOut` (`<>`) is not in the operator list, so it is never checked. `<>` opens read-write and creates the file, so a command's output can be written through it.

Failure scenario: `Check("cd / && echo hi > etc/moca-test")` and `Check("cd .. && echo hi > x")` both return no approval and no error (jail root `/tmp` in the reproduction). In bash the first writes `/etc/moca-test`. `Check("echo hi 1<> /etc/x")` and `Check("echo hi <> /etc/x")` also pass; running `bash -c 'echo hi 1<> file'` created the file containing `hi`. Absolute paths and `..` in a plain `>` are refused correctly (`>`, `>>`, `>|`, `&>`, `>&`, and `tee` all checked).

Proposed fix: add `syntax.RdrInOut` to the checked operators. For `cd`, either resolve targets against a tracked working directory (for simple `cd literal && …` sequences) or fail closed: once a `cd`/`pushd`/`popd` to anything other than a literal in-jail path appears, refuse every later redirect and `tee` target in that command. Add ladder cases for both.

#### 4. A tool call truncated at `max_tokens` ends the run with a JSON marshal error instead of an error result — Confirmed
`internal/agent/agent.go:149` (`turn`), `internal/provider/openai_completions.go:205` (call kept as assembled), `internal/agent/agent_test.go:194` (`TestLengthStopTruncatedToolCall`)

The completions adapter deliberately keeps a call cut off mid-JSON (its comment says the phase-2 loop will attach the §6 error result). `turn` persists each call as a `tool_use` session entry whose `Call.Input` is a `json.RawMessage`; `json.Marshal` of an invalid `RawMessage` fails, `Writer.Append` returns the error, and `turn` returns it. The loop that was supposed to answer the call never gets to run.

Failure scenario: a script server returns `write` with arguments `{"path":"x","content":"aaa` and `finish_reason: "length"` (the shape the test uses), driven through the real `agent.Start`. `Run` returns `Outcome{}` and `json: error calling MarshalJSON for type *jsontext.Value: unexpected end of JSON input` after exactly one request. The session file ends with an assistant message whose `content` is `null` and no tool entries. `moca -p` would exit 1 with that message. SPECS.md §14 and Review Focus item 4 both say the opposite.

The test passes only because it asserts nothing when there are no `ToolEnd` events: it checks that no file was created and loops over events whose count is not asserted, and it discards `Run`'s error.

Proposed fix: before persisting in `turn`, replace any call whose `Input` is not valid JSON with a valid placeholder (for example `{}`) and remember its ID. In `Run`, give every such call, and every call of a `length` turn, the existing "cut off, split the work" error result without executing it. Then rewrite the test to assert that `Run` returns nil, the number of `ToolEnd` events equals the number of calls, each is an error, a second request is sent, and `session.Messages` of the resulting file marshals.

### Medium

#### 5. `Truncate` returns nothing for output dominated by one long line — Confirmed
`internal/tools/shellrun.go:83`

Truncation keeps whole lines from the head and tail up to half the budget each. If the first line is longer than half the budget, nothing is kept at either end.

Failure scenario: `Truncate(strings.Repeat("x", 100000), 30000)` returns exactly `[… 1 lines omitted]\n` (22 bytes). A `curl` of a minified JSON document, or `cat` of a bundle, shows the model no output at all, and the exit code line is all it sees. With 40 normal lines followed by a 100K line, the tail is lost but the head is kept (15,023 bytes).

Proposed fix: after the line loops, if either side is empty and the budget is unused, fall back to slicing characters: the first `max/2` and last `max/2` bytes, snapped to rune boundaries, with a note. Add a `TestTruncate` case for a single long line and for a long line at the end.

#### 6. Shell runner: unbounded capture, and the timeout is not enforced when a descendant leaves the process group — Confirmed (timeout); capture growth confirmed by reading, OOM not reproduced
`internal/tools/shellrun.go:49` (buffer), `:55-62` (wait)

- Output goes to a `bytes.Buffer` with no limit, so a command such as `yes` runs for the full timeout (default 30s, up to 300s) and its output is held in memory, even though only 30K characters are ever returned.
- `exec.Command` is used without `WaitDelay`, so after the group is killed `cmd.Wait` still blocks until every holder of the output pipe exits. A descendant that called `setsid` is not in the killed group.

Failure scenario: `RunShell(..., "setsid sleep 6 & echo hi", 2*time.Second)` returned after 6s with `TimedOut=true`; a 300s timeout against a daemonised process would hang for as long as the daemon lives. Relatedly, `sleep 6 & echo hi` waits the full timeout and reports a timeout and an error status although the shell exited immediately.

Proposed fix: set `cmd.WaitDelay` (a couple of seconds) so `Wait` returns after the kill, and capture through a bounded writer that keeps the head and a rolling tail (a little over 30K characters) and counts dropped lines. Add tests for `setsid` and for a large-output command.

#### 7. Quoted or escaped command names skip the hard-deny list — Confirmed
`internal/permissions/shell.go:80` (`wordLit`), `:56` (`classify`)

`wordLit` returns the raw text of an unquoted word, including backslashes, and the raw body of an ANSI-C `$'…'` string. Hard-deny, `rm` and the refused set are compared against that raw text.

Failure scenario: `\sudo id`, `s\udo id` and `$'s\x75do' id` return `need=[\sudo]`, `need=[s\udo]` and `need=[s\x75do]` and no error. Bash runs `sudo` for each. The result is an approval prompt (or a refusal in `-p`) rather than the "never allowed" refusal; "always allow" would then allowlist the escaped spelling. `"su"do` is handled correctly.

Proposed fix: treat a command-name word that contains a backslash, a `$'` string, or glob characters as non-literal and refuse it, like `$CMD`. Add ladder cases.

#### 8. `search` ignores `.gitignore`/`.ignore` files above the search path — Confirmed
`internal/tools/search.go:60`, `:82`

Ignore files are loaded for the start directory and below, relative to the start directory, never from its ancestors.

Failure scenario: workdir has `.gitignore` containing `ignored.txt`, and `sub/ignored.txt` and `sub/kept.txt` both contain `needle`. `search {"pattern":"needle","path":"."}` returns only `sub/kept.txt:1:needle`. `search {"pattern":"needle","path":"sub"}` returns both `sub/ignored.txt:1:needle` and `sub/kept.txt:1:needle`. A model that scopes a search to `src` will see `node_modules`, `dist` and build output the root ignore file excludes, which costs tokens and returns misleading hits.

Proposed fix: when `start` is inside the workdir, load the `.gitignore` and `.ignore` of every directory from the workdir (or the git root) down to `start` with the right bases before walking. Add a test for a sub-path search.

#### 9. `read`, `write` and `search` load whole files with no size limit — Confirmed by reading; not run
`internal/tools/read.go:65`, `internal/tools/write.go:77`, `internal/tools/readtracker.go:34`, `internal/tools/search.go:109`

All four read the entire file into memory before applying the line, character or hit caps. `ReadTracker.stampOf` reads the file again to hash it, and `search` reads every non-ignored regular file in the tree.

Failure scenario: a multi-gigabyte dump, log or database file in the workdir that is not hidden or ignored. `search` over the workdir, or `read` of that file, allocates the whole file first (the binary check only looks at the first 8K afterwards). A named pipe in the workdir blocks `read` indefinitely.

Proposed fix: `Stat` first and refuse or window files above a limit (for example `read` streams only the requested window; `search` skips files over a few MB with a note; hash with a streaming `io.Copy`). Refuse non-regular files in `read`.

#### 10. `session.Open` appends onto a crash-truncated last line, losing the new entries — Confirmed
`internal/session/writer.go:79`

`ReadFile` tolerates a truncated last line by stopping at the first unparseable line, but `Open` reopens the file for append without terminating or trimming that partial line.

Failure scenario: a session file ends with `{"id":"partial","type":"error","err` (no newline) after a crash. `Open` returns the 2 valid entries; one `Append` writes its JSON onto the same line; `ReadFile` then returns 2 entries again, so the appended entry is gone, and every later entry is hidden behind the corrupt line. Not reachable from a command in this phase (resume is phase 4), but `Open` and the "tolerates a truncated last line" claim ship now.

Proposed fix: in `Open`, if the file does not end in `\n`, truncate it back to the last newline (or write a leading `\n`) before appending. Add a test that appends after a deliberately truncated file.

### Low

#### 11. Skill discovery: diagnostics are dropped and symlinked skill directories are skipped silently — Confirmed
`internal/agent/start.go:67`, `internal/skills/skills.go:30`

`Start` calls `sk, _ := skills.Discover(dirs)` and discards the error list, although SPECS.md §12 says a skill without a description is "skipped and reported". Discovery also uses `e.IsDir()` on `os.ReadDir` entries, which is false for a symlink, so `skills/hello -> ~/dotfiles/skills/hello` yields 0 skills and no error (reproduced). The jail would also refuse to `read` a body that lives outside the skill root.

Proposed fix: surface the discovery errors (a `Warning` event or a stderr line in `-p`), and either follow symlinks to directories or report that they are skipped. Decide the read-jail interaction at the same time.

#### 12. A wrap-up turn that still returns tool calls leaves them unanswered, and `Start` errors map to exit code 2 — Confirmed
`internal/agent/agent.go:186`, `cmd/moca/oneshot.go:76`

- The wrap-up request uses `tool_choice: none`, but if a model or gateway ignores it the response's calls are persisted and never answered. With `maxSteps: 1` and a server that always returns a tool call, `Run` returned `MaxSteps:true`, `err=nil`, empty text, and `session.Repair` found 1 unanswered `tool_use` (SPECS.md §10 says every call gets exactly one result). `-p` then prints an empty answer and exits 3.
- Any `agent.Start` failure (session directory not writable, builtin skill extraction failing) returns `exitUsage` (2), but 2 is reserved for config and usage errors.

Proposed fix: after the wrap-up turn, run `session.Repair` for any calls it produced and report that the model did not comply; return `exitRuntime` from `Start` failures that are not config errors.

#### 13. `edit` with `replace_all` omits the line-number-prefix hint — Confirmed
`internal/tools/editapply.go:57`

The `case replaceAll` branch returns the generic "old_string not found" message and never checks `lineNumPrefix`, so a model that copies `read` output with `N|` prefixes and sets `replace_all` does not get the hint Review Focus item 1 asks for.

Proposed fix: append the same hint in that branch and add a subtest.

#### 14. `ExtractBuiltins` can fail when two processes start together, and dev builds never refresh it — Plausible
`internal/skills/builtin.go:17`

The existence check and the `RemoveAll(tmp)` / write / `Rename` sequence is not safe against a second process doing the same on first use: one process can delete the other's temp directory, or `Rename` onto an existing non-empty directory fails, and either aborts `Start` (exit 2). Separately, every development build uses version `0.0.0-dev`, so an edited embedded skill is never re-extracted.

Proposed fix: use a unique temp directory per process and treat "destination already exists" after the rename as success; key the directory on a content hash of the embedded files rather than the version string.

#### 15. Untracked probe at `internal/permissions/probe/main.go` — Confirmed
`internal/permissions/probe/main.go`

An untracked program that creates symlinks and writes `pwned` under `/tmp/moca-int` and `/tmp/moca-ext`. It is not gofmt-clean (`gofmt -l` lists it), so it would fail the gofmt gate if added, and it writes outside the repository when run. Its leftovers show that finding 1 had already been found by hand.

Proposed fix: turn it into regression tests in `jail_test.go` (finding 1) and delete the program. I did not run it and did not touch it.

## Earlier findings

Phase-1 review (`2026-10-04-phase-1-skeleton-providers.md`), only items that touch this phase:

- **#1 `-p` duplicates text on retry** — still fixed. `oneshot.go` prints only `Outcome.Text` once at the end and resets its per-turn buffer on `StreamReset`.
- **#2 truncated tool calls** — still open end to end. The adapter keeps the cut-off call and keeps `StopLength` (as that review proposed for the stop reason), but the call is not safe to persist: see finding 4.
- **#3, #4, #5, #6, #7, #8, #9, #10** — present in the tree as described (`ErrUnexpectedEOF`, `replaysVerbatim`, effort-aware `BudgetTokens`, `maxRetryAfter`, `http2Transient`, `glmThinking[off] = "none"`, model-level protocol check). Not re-tested beyond the passing suite.

## Suggested order of fixes

1. Finding 1 (jail), then 3 (redirect checks) and 2/7 (command classification); these are the guarantees the phase promises. Fix them with the regression cases listed so the ladders cover wrappers, escapes, `<>`, `cd` and dangling links.
2. Finding 4 and rewrite `TestLengthStopTruncatedToolCall`; it is also the only Review Focus item currently unmet end to end.
3. Findings 5 and 6 (shell output), then 8 and 9 (search and read behaviour).
4. Finding 10 before phase 4 depends on `Open`.
5. The Low findings in any order; 15 can be done with finding 1.
