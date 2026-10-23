# Plan: "Allow for this session" approval answer

> Status: **implemented in v0.7.7 (2026-10-23)**. The in-progress v0.7.7 this plan expected to land
> first shipped as v0.7.6, so this pass took the v0.7.7 number.

## Context
moca's approval prompts offer Allow once / Allow always / Deny. There is no "always" for `rm`, the
user's `permissions.ask` rules, private-host web fetches or writes outside the workdir. The user wants
a fourth answer, **Allow for this session**:
- it is kept in memory only;
- the same command is never asked about again for the rest of the session;
- it is offered even where "always" is not.

## Decisions (user)
- **Match key:**
  - A shell question with `CanAlways:true` matches by command name, the same as Allow always:
    `go` covers every `go …`.
  - A shell question with `CanAlways:false` (rm, rule-Ask, auto ask-every-time, non-learnable names)
    matches only the exact same full command line.
  - MCP matches `server/tool`, web matches `fetch <host>`, and write matches the resolved absolute path.
- **Scope:** one Agent is one session.
  - /clear and /resume start with an empty set.
  - /mode, /model and /effort keep it.
  - Each ACP session is isolated automatically.
- **Kinds:** all four (shell, mcp, web, write).
- **ACP:** replace "Always allow" with `allow-session`. ACP never persisted `allow-always` anyway.
  - The new option is named "Allow for this session" and uses kind `allow_always`, the closest kind
    the ACP schema has.
  - It is offered on every question, `rm` included.
- **Exclusions:** these always ask and never offer the session option:
  1. writes to moca's own config file (`~/.config/moca/config.jsonc`), which holds the permission
     policy itself;
  2. web redirect hops. Only a fetch that starts at a session-allowed host skips the prompt; a public
     page that redirects to localhost still asks.
- **Keys (amended 2026-10-22):**
  - Allow once moves from `a` to `o`.
  - Allow always moves from `ctrl+a` to plain `a`.
  - Allow for this session is `s`.
  - Deny stays `d · esc`.

## Builder's calls (can be vetoed)
- **TUI rows** (the keys are the user's; the order and labels are builder's calls):
  - `Allow once o`
  - `Allow for this session s`, labelled "Allow this exact command for this session" when the key is
    exact-line
  - `Allow always a` (only when CanAlways)
  - `Deny d · esc`
  - The default stays on Allow once.
- **Letter keys:** `o`, `s`, `a` and `d` all share today's 700 ms typing-pause guard (`approvalIdle`).
  - A key typed mid-sentence goes to the draft. Capitals (`O`, `S`, `A`) always go to the draft.
  - `a` on a question without the always row (rm, rule-Ask, web, write, `NoSession`) is swallowed:
    handled, no answer, nothing typed. This is today's `ctrl+a` behaviour.
  - `ctrl+a` no longer answers. It falls through to the input box's "line start".
- **Safety trade-off of the key change:**
  - Today a plain letter can never persist anything (`approvalKey` comment, `TestAllowAlwaysRequiresCtrl`).
  - After the change, `a` after the typing pause writes `.moca/config.jsonc`, even with a
    half-typed draft (as `a`/`d` answer today, `TestApprovalPreservesDraft`).
  - Users used to `a` = once will now save a permanent allow, so the CHANGELOG must lead with the
    key change.
- The `(asks every time)` prompt suffix becomes `(never saved)`.
  - Excluded questions (`NoSession`) keep `(asks every time)`.
  - Choosing the session row prints `allowing \`x\` for this session`.
- Version **v0.7.8**, assuming the in-progress v0.7.7 lands first. Feature PRs here are patch bumps.
  Re-check at implementation time.

## Design
1. `internal/tools/tool.go`
   - Append `AllowSession` to `Answer`.
   - Add `Answer.String()` (deny/once/always/session).
   - Add `Question.NoSession bool`.
   - Add `Question.SessionExact()` (shell && !CanAlways).
   - Add `Question.SessionKey()`: Kind+NUL+Subject, plus NUL+Detail when exact. The key separates the
     tiers, so a session `git` never answers a rule-Ask `git push`.
2. New `internal/tools/sessionallow.go`: `RememberSession(inner Asker) Asker`.
   - nil in returns nil, so `-p` still denies at every `env.Ask != nil` site.
   - A mutex-guarded `map[string]bool`.
   - A remembered key returns `AllowSession` without asking; a cancelled ctx returns Deny.
   - Only an `AllowSession` answer is recorded.
   - A `NoSession` question is never answered from memory or recorded. An `AllowSession` answer to one
     is downgraded to `AllowOnce`, the same pin as `TestAllowAlwaysIsIgnoredWhenTheQuestionForbadeIt`.
3. `internal/agent/agent.go:169`: `a.checkers.ask = tools.RememberSession(o.Env.Ask)`.
   - Ask and auto mode share it through `applyMode` (`mode.go:83`); yolo never asks.
   - Every Agent goes through `build()` → `New` (TUI start, /clear, /resume, ACP new/reopen, `-p`).
   - The in-progress v0.7.7 work touches agent.go:336 only, so the two merge cleanly.
4. `internal/tools/shell.go`
   - Line 44: replace `[...]string{…}[ans]` with `ans.String()`; indexing that array panics on a
     fourth answer value.
   - Line 51: `case AllowAlways, AllowSession: if always { env.Commands.Allow(n) }`.
     - `Rules.Learn` only changes memory; only the TUI writes to disk, and only on AllowAlways.
     - This keeps the `make` guard, which runs only when an allow rule matched
       (`permissions/shell.go:201`).
     - It also skips auto mode's classifier call.
     - /clear rebuilds Rules from `m.start.Config`, so the session allow disappears with it.
5. Exclusion wiring:
   - **Config file:** `build()` (`internal/agent/start.go` ~142) sets `env.ConfigFile`, resolved like
     the write target. `askOutsideWrite` (`tool.go:103`) sets `NoSession` when `abs == env.ConfigFile`.
   - **Web hops:** `askPrivate` (`web.go:125`) takes a `hop bool`. The `CheckRedirect` call
     (`web.go:167`) passes true, which sets `NoSession`.
   - Same-host hops still skip the prompt through the per-call `approved` map.
6. `internal/mcp/tool.go:148`: `case tools.AllowAlways, tools.AllowSession:` sets `p.session`, for parity.
   - `web.go:134` and `tool.go:105` only check `== Deny`.
   - A remembered host still fills `okIPs` (`web.go:137-144`), so the dialer coupling holds.
7. `internal/tui/approval.go`:
   - `options()` (the session row is hidden when NoSession);
   - `approvalPrompt`;
   - `approvalKey` (lines 101-146):
     - the guarded letter case becomes `"o", "s", "a", "d"`;
     - `o` answers AllowOnce;
     - `s` answers AllowSession, and is swallowed when NoSession;
     - `a` answers AllowAlways, and is swallowed when !CanAlways;
     - the `ctrl+a` case is removed;
   - the session note in `answerApproval`;
   - rewrite the doc comments on `approvalKey` and `approvalState`, which justify `ctrl+a` by
     persistence.
8. ACP:
   - `internal/acp/protocol.go:51-56`: change the option id.
   - `session.go:278-308`: change the answer mapping. Don't offer the option when NoSession; reject
     an `allow-session` pick then.
9. Docs (AGENTS.md: SPECS is the source of truth, same PR):
   - `docs/specs/SPECS.md`:
     - header (line 7)
     - §3.5 (~144)
     - §9.5 (~323)
     - §10 (~353)
     - §10.5 (~367)
     - §11.0 (~382)
     - §11 web and ask-write roots (~413-418)
     - §13.5 answer values (~471)
     - §15 test count (~506)
   - `README.md` ~199/201/232. The keys table row becomes
     `` `o` / `↑↓` `enter` / `s` / `a` / `d` ``.
   - SPECS §3.5 (~144): the picker rows and the key list.
   - `CHANGELOG.md`: a v0.7.8 entry. Its first bullet is the key change (`a` now means always and
     saves; once is `o`).
   - Re-copy specs/readme/changelog byte-identical into
     `internal/skills/builtin/self-knowledge/references/`.
   - The self-configure `SKILL.md:8` promise ("asks every time") stays true thanks to the exclusion.

## Tests (TDD: write each failing test first)
- **tools:**
  - `TestAnswerString`
  - `TestQuestionSessionKey` (tiers and kinds never mix)
  - `sessionallow_test.go`:
    - nil stays nil
    - only session answers are recorded
    - name vs exact line
    - NoSession is never remembered and AllowSession is downgraded
    - a cancelled ctx returns Deny
  - `allowalways_test.go`:
    - AllowSession on `rm` learns nothing
    - AllowSession on `python` learns it, with no panic
  - Web:
    - a session host fetched twice asks once and both fetches succeed
    - a redirect hop to a session host still asks
  - writeedit: a session allow on a prompts file sticks; a config-file write still asks.
- **mcp:** extend `TestProxyGating` with AllowSession.
- **agent:** `TestSessionAllowScope`:
  - `uname` is asked once across ask→yolo→ask
  - `rm -f x` twice asks once; `rm -f y` asks again
  - a fresh agent asks again
  - `Ask == nil` stays nil
- **tui:**
  - Update:
    - `TestApprovalPickerArrowsEnter` (three downs to deny)
    - `TestApprovalPickerAlwaysPersists` (two downs)
    - `TestApprovalPickerNoAlwaysRow` (rows once/session/deny)
    - `TestApprovalShowsItsOptionsOnce`
  - Key-change updates:
    - `TestApprovalKeysAnswerAfterPause` (`fixes_test.go:71`): `a` → `o` for once; add `a` →
      AllowAlways.
    - `TestApprovalKeysIgnoredWhileTyping` (`:46`): the typed prose must also contain an `o`
      ("Add tests" already has `a` and `s`), and the config must stay unwritten.
    - `TestAllowAlwaysRequiresCtrl` (`:87`) becomes `TestAllowAlwaysIsPlainA`:
      - a capital `A` still goes to the draft;
      - `a` after the pause answers AllowAlways and persists;
      - `ctrl+a` answers nothing.
    - `TestApprovalPreservesDraft` (`app_test.go:71`): `a` → `o`; the `ctrl+a` on `rm` becomes an
      `a` that is swallowed.
    - Other `ctrl+a` presses switch to `a` with the typing pause elapsed:
      - `fixes_test.go:256` (`TestRestartSessionKeepsModelAndAllowed`)
      - `:309` (`TestAllowAlwaysPersists`)
      - `:336`, `:351`, `:374` (the untrusted-workdir and MCP persists)
    - Wording in `approval_test.go:63/72`. Add `"[o] once"` to the stale-string check at `:96`.
  - New:
    - `TestApprovalSessionKey`: `s` after the pause → AllowSession; a note is printed; no project file
      is written
    - `TestApprovalSessionRowLabels`
    - `TestApprovalNoSessionRow`
- **acp:**
  - Update:
    - `protocol_test.go:70` (the constant is removed)
    - `permission_test.go:34` (`rm` now offers the session option, kind allow_always)
  - New: `TestPermissionAllowSessionRemembersExactCommand`.

## Verification (at implementation time)
- `gofmt -l .` must print nothing.
- `make vet` and `make test` (race) must pass, including `TestSelfKnowledgeReferencesAreByteIdentical`.
- Manual TUI run:
  - `rm -f /tmp/x` twice with `s`: the second identical call doesn't prompt; `rm -f /tmp/y` does.
  - `go version` + `s`, then `go env GOPATH`: no prompt.
  - /clear: prompts again.
  - A config-file edit offers no `s` row.
  - On a `python3` prompt:
    - `o` allows once;
    - `a` saves to `.moca/config.jsonc`;
    - `ctrl+a` moves the cursor to the line start and doesn't answer.
  - On an `rm` prompt, `a` does nothing.
  - `.moca/config.jsonc` is unchanged.
- Then `graphify update .`
- Commit/PR only with the user's approval. Branch `chore/session-allow`, subject `[feat:core] v0.7.8: …`.
