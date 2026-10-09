# TUI review & polish plan

Scope: `internal/tui` (+ `cmd/moca/tui.go`). Reviewed by reading the code against SPECS §3.5 and by driving the
built binary in tmux (fresh `git init` repo, live runs on `opencode-go/deepseek-v4.1-flash`, approvals, `!`
commands, pager, resize, quit). Compared against Claude Code, OpenCode and Pi.

**Lens.** moca's pitch is *~10% of the surface area, cheap per task, readable in an hour*. Every item below was
filtered through: does it remove friction a user hits **every session**, and can it be built from pieces the TUI
already has (the dropdown, the picker rows of the login wizard, the pager, `Items`, `styleBlock`) without a new
dependency? Items that fail both are listed under *Deliberately out of scope*.

**Status (this branch):** everything below is implemented except **2.6** (investigated; deliberately not changed —
see its entry) and **3.2** (skipped). Each item carries a *Status* line recording what shipped and where it
differs from the proposal. The authoritative behavior description is SPECS §3.5 / §3.5.1.

---

## 0. Fixed in this pass (already on `fix/polish`)

| # | Issue found | Fix |
|---|---|---|
| F1 | **Approval prompt showed only the first line of the command.** Approving `python3` for `python3 - <<'EOF' … EOF` hid the script that would run, so you approved something you couldn't see. | `approvalPanel`: the question, then the command under a dim `│` gutter, up to 6 lines (fewer on short terminals), each clamped to the width, with the overflow counted (`… +N lines · ctrl+o shows the whole command`). `ctrl+o` while a question is pending pages the full command. |
| F2 | `!cmd` had no feedback while running (up to 30 s of silence) and could not be stopped. `/compact` only changed the cost field. | The activity row now covers runs, `/compact` (`compacting… · esc to cancel`) and `!` (`$ cmd…`). `esc` kills a running `!` (process group); it prints `[interrupted — process group killed]`, and the interrupted command is not added to the transcript. |
| F3 | Double `ctrl+c` to quit gave no hint after the first press. Inside the pager, `ctrl+c` cleared `Input` but not the textarea, so the draft came back on the next key. | A dim `press ctrl+c again to quit` row for the 1 s window, removed by a tick. A shared `ctrlC()` path (also used in the pager) re-syncs the textarea. It uses `m.now`, so tests can inject the clock. |
| F4 | On exit the shell prompt landed on top of the last frame, and there was no way back into the session except `--continue`. | The final frame keeps the rule and the status bar (final cost and context) and drops the input box. `session <id8> · resume with: moca --resume <id8>` prints below it, but only when the session holds something. |
| F5 | The status bar showed branch `-` in a fresh `git init` (unborn `HEAD`). | `GitBranch` falls back to `git symbolic-ref --short HEAD`. |
| F6 | `/cost` printed raw integers (`in 2377`) that didn't match the bar (`4.6k`, which includes cache). | Same compact format as the bar: `in 4.6k (fresh 201 · cache read 2.2k · cache write 0) · out 129 · $0.0004` (`subscription` for OAuth). |
| F7 | Small things: `thinking 1 lines`; `/copy` lost the answer when the final turn had no text; failed tool items looked like successful ones (all muted); a long pager title wrapped and pushed the body off by a row. | `1 line`; `lastAssistant` only updates on non-empty text; a red `✗` mark; the pager title is clamped to one row and shows `N% · q to close`. |

Tests: `internal/tui/polish_test.go` plus updated `items/app/activity/events/format` tests. `make vet` and
`go test ./... -race` are green. SPECS §3.5, README keys and CHANGELOG (`Unreleased`) are updated.

---

## 1. Proposed improvements (priority order)

Size: **S** ≤ ½ day · **M** ≈ 1–2 days · **L** > 2 days. Every item ends with *SPECS §3.5 + README + CHANGELOG
updated, tests in `internal/tui`* (AGENTS.md rule). That isn't repeated below.

### P1 — high value, every session

#### 1.1 Markdown-lite rendering of responses — **M**

- **Status: ✅ done** (`markdown.go`). Differences: no `tui.markdown` switch (kept the config surface small; `NO_COLOR` strips the colors, `/copy` copies raw text). `ansi.Wrap` already re-opens spans on wrapped rows but closes them with a full `\x1b[m`, which would cut the band background short — rewritten to targeted toggles (bold off / fg default). The live partial line is previewed with a copy of the fence state, so a fence opener does not flicker. **A bug found on the way:** the fence state was reset by `commitLive` on every batch; covered by `TestFenceSurvivesCommitLive**
- **What.** Style the assistant band instead of printing raw markdown: `**bold**`, `` `code` `` (distinct
  foreground), `#` headings (bold, no `#`), list bullets (`-`/`*` → `•`), and fenced code blocks (no wrapping
  inside, a dimmer band, the fence lines replaced by a thin `─ go` label). Links stay as text.
- **Why.** Every answer is markdown. Raw `**Package:**` and triple backticks (seen live) are the most visible gap
  next to Claude Code and Pi, which both render it. Code in fences is also what users copy, so it must not be
  word-wrapped mid-token.
- **How.**
  - New `internal/tui/markdown.go` (~150 lines, no dependency: glamour is heavy, re-renders whole documents and
    is not streaming-friendly). A `mdState{inFence bool; lang string}` lives on the model. `renderMDLine(st *mdState, line string) (styled string)`
    is applied per **committed** line in `handleAgent(TextDelta)` before `printlnResponse`, so streaming keeps
    working line by line. Fence state survives across deltas and resets on `TurnEnd`/`StreamReset`/run end.
  - Inline spans: a small scanner for `**…**` and `` `…` `` on the already-sanitized line. It skips unbalanced
    markers (prints them literally) and never crosses lines.
  - `styleBlock` gains a variant that takes pre-styled spans. Wrapping must measure with `lipgloss.Width`
    (ANSI-aware) and wrap **before** styling the spans, or the band padding miscounts.
  - The live (partial) line renders plain. It gets styled when it commits, which matches the "completing a line
    does not change its look" rule, except for inline spans. That's acceptable; document it.
  - Off switch: config `tui.markdown: false`. Strict config means defaults, validation and SPECS §4 are needed.
    Alternative: no switch.
  - Tests: per-construct table tests, a fence spanning deltas, an unbalanced `**`, a wide-rune line in a fence,
    and that `/copy` still copies the raw markdown.

#### 1.2 Inline diff preview for `edit`/`write` items — **S/M**

- **Status: ✅ done** for `edit` (`diff.go`). `write` results carry no diff (`Result.Detail` is empty), so it is not covered; giving `write` a diff is a tool-result change, not TUI work. Only changed lines are shown (context dropped) so 8 rows hold more signal**
- **What.** Under `▸ #4 edit main.go [+3 −1]`, print the first ~8 hunk lines with `+` green / `-` red, then
  `… ctrl+o for the full diff`.
- **Why.** Reviewing what the agent changed is the core loop of a coding agent. Today it takes one `ctrl+o` per
  edit. Claude Code's inline diffs are its most praised UI element. `tools.Result.Detail` already *is* the diff
  for `edit`, so this is pure rendering.
- **How.** In `handleAgent(ToolEnd)`, when `e.Call.Name` is `edit` or `write` and `Detail` is non-empty, call a new
  `diffPreview(detail string, width, maxLines int) string` (items.go). It skips `---`/`+++` headers, keeps `@@`
  dim, clamps lines to the width and colors by first rune. The cap is a constant (8). Consider a config knob
  only if asked. Sanitize before styling.

#### 1.3 Context-pressure color in the status bar — **S**

- **Status: ✅ done** — red threshold is `compact.Budget.Trigger()` (window − reserve, itself capped at window/4 for small windows), so the color predicts auto-compaction exactly; the `/compact` hint is a droppable field (rank 2)**
- **What.** The `P%` field turns yellow at ≥ 70% of the window and red at ≥ 90%, with `· /compact` appended when
  red.
- **Why.** moca's selling point is cost. A context near full means slow, expensive turns and imminent
  auto-compaction, and a dim number among dim numbers is easy to miss.
- **How.** `RenderStatus` returns plain text by design (styling is the caller's job). Have it also return the
  byte span of the percent field, or render line 2 as fields in `statusLine()` and style that field there. Keep
  the existing shrink ranks. Thresholds: yellow at 70%; red when `Used ≥ Window − context.reserveTokens`, i.e. where
  auto-compaction fires, so the color predicts it rather than guessing. Pass `reserveTokens` into `StatusInfo`.

#### 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**

- **Status: ✅ done** (`dropdown.go`, `picker.go`). Also covers `/resume`. Bare `/model` now opens a modal picker (current preselected, `no key` marked); during a run it still prints the plain list (a switch is refused then). The login wizard keeps its own state machine — only `/model` and `/resume` use the new `pickState`, so no working code was disturbed; its rows share `choiceRow`**
- **What.**
  - (a) Once the draft is `/model ␠…`, `/effort ␠…`, `/login ␠…`, `/logout ␠…` or `/show ␠…`, the dropdown lists
    the valid arguments (models, effort levels the current model supports, configured providers, item numbers
    with their lines), prefix/substring-filtered.
  - (b) The `/model` list marks models whose provider has no credential as dim `(no key)` instead of listing them
    as if switchable. It refused the switch only after you picked one (seen live: anthropic and openai models
    listed with only `opencode-go` configured).
- **Why.** `/model` printed a 21-line wall you then had to retype from. Pi and OpenCode both have a model picker.
  Reusing the dropdown keeps one interaction pattern for everything.
- **How.**
  - Generalize `slashQuery` to `(cmd, argPrefix, ok)`. `commandHints` becomes `completions(cmd, arg)`, with a
    switch per command over `agent.Models()`, `supportedEfforts`, `providerNames`, `m.items`.
  - `completeFromDrop` writes `/cmd value`. Enter on an exact argument submits (same rule as today).
  - Credential check: `Providers.CheckCredential(q)` already exists (env/store lookup, no network). Cache the
    result per dropdown build.
  - Matching: substring for models (`glm` should find `opencode-go/glm-5.3`), prefix for the rest.

#### 1.5 `@path` file mentions — **M**

- **Status: ✅ done** (`mention.go`, `tools.ListFiles`). The mention is the draft's **last word** rather than the word under the cursor (the textarea does not expose a cheap cursor column); `enter` inserts instead of sending while the list is open; indexed lazily on the first `@` and re-indexed after a turn**
- **What.** Typing `@` followed by a path fragment opens the dropdown with matching workdir files (gitignore-aware).
  `tab` completes to `@internal/tui/app.go`. The text is sent as-is, and the model reads the file itself with its
  windowed `read`.
- **Why.** Pointing at files is the most common prompt pattern. Typing exact paths is the friction. All three
  reference TUIs have it. Not inlining contents keeps it token-cheap and frozen-tool-compatible: no new
  behavior on the agent side.
- **How.** Reuse `tools/ignore.go` + a bounded walk (cap ~5k entries, cached per session, refreshed on
  `TurnEnd`) off the event loop via a `tea.Cmd`. The dropdown trigger is the token under the cursor matching
  `@\S*` (needs the cursor column from `m.ta`). Fuzzy-ish ranking: basename prefix > path segment prefix >
  substring.

#### 1.6 Resume shows where you left off; `/resume` picker — **M**

- **Status: ✅ done** (`replay.go`, `session.List`, `agent.History`). Replay runs off the `Resumed` event so startup `--resume` and `/resume` share one path; replayed tool items summarize the call operand since the live summary is not stored**
- **What.**
  - (a) On `--resume`/`--continue`, replay the last few exchanges (default: last 3 user turns) into the
    scrollback with the normal bands and item lines, instead of `resumed 16b8f49a (4 messages) — earlier output
    is in the session file`.
  - (b) A `/resume` command opens a picker of recent sessions for this workdir (id8 · age · first user line), and
    picking one swaps the session like `/clear` does.
- **Why.** Resuming blind means re-asking "where were we". Every reference TUI shows history on resume and has a
  session picker. moca already has `session.FindForWorkdir`, `listNewestFirst` and `agent.Resume`.
- **How.**
  - (a) `agent.Resume` already rebuilds messages. Expose `agent.History(n)` (read-only copy of the last n
    messages) and have the TUI render them through the same `printlnUser`/`printlnResponse`/`Items` paths
    (thinking collapsed, tool items numbered).
  - (b) Reuse the login wizard's picker rendering (`choiceRow`) by extracting a tiny `picker` type
    (`items, cursor, up/down/enter/esc`) that login, `/model` (1.4 fallback) and `/resume` share. Swapping
    sessions follows `restartSession` with `agent.Resume` instead of `agent.Start`. It is refused while busy.

#### 1.7 Persistent input history + `ctrl+r` search — **S/M**

- **Status: ✅ done** (`history.go`; search lives in the dropdown with the draft as the query). Entries over 8 KiB or containing control bytes are not stored**
- **What.** `↑` history survives restarts (per workdir, capped at 500, consecutive duplicates collapsed).
  `ctrl+r` opens a reverse-incremental search in the dropdown slot.
- **Why.** Re-sending yesterday's "run the tests and fix failures" is common, and shells taught everyone
  `ctrl+r`.
- **How.** `Input.history` loads from and appends to `$XDG_DATA_HOME/moca/history.jsonl` (`{"wd":…, "text":…}`;
  the file is 0600). Entries containing control bytes are skipped (they're re-chipped today, so don't persist
  them). Write on `Submit` (append, no rewrite). The search UI is a `dropState` whose items come from
  `history` filtered by substring. Enter puts the match in the draft and doesn't send it.

### P2 — readability & feedback polish

#### 2.1 Turn spacing and hanging indent — **S**

- **Status: ✅ done**
- **What.** A blank row before each assistant response block that follows tool or thinking items. Wrapped user
  lines indent under the text after `› ` instead of under the `›`.
- **Why.** Live output reads as one undifferentiated stack: response text, then `▸ #1 read…`, then
  response text. Claude Code separates turns with bullets and whitespace.
- **How.** Track `lastPrintKind` (user/response/item/notice) in `emit`'s callers. `printlnResponse` prepends
  `"\n"` when the previous kind was `item`. `styleBlock` gets a `hang int` parameter for continuation rows
  (`printlnUser` passes 2).

#### 2.2 One-row item lines — **S**

- **Status: ✅ done**
- **What.** `▸`/`✗`/`⋯` item lines are clamped to the terminal width with `…`.
- **Why.** Long error summaries (`✗ #4 shell refused: … ask the user to allow it…`) wrap into ragged multi-row
  lines. The full text is one `ctrl+o` away, and items are meant to be one line each.
- **How.** `truncCells` in `handleAgent(ToolEnd)` / `flushThinking` at print time, so `Item.Line` stays full for
  the pager title.

#### 2.3 Richer activity row — **S**

- **Status: ✅ done** — token figure is a chars/4 estimate and shown for runs only; queued count via `agent.PendingSteering`**
- **What.** `⠋ shell go test ./... 12s · ↓1.2k tok · 1 queued · esc to interrupt`: the running tool's argument
  summary, live output tokens, and the number of queued steering messages.
- **Why.** "shell…" for 40 s says nothing about *what* is running. Token flow shows the model is alive during
  long thinking. Queued steering is otherwise invisible after the `↳ queued` line scrolls away (Pi shows queued
  messages).
- **How.** `ToolStart` carries `e.Call.Input`. Add a `toolArgSummary(call)` (command for shell, path for
  read/edit/write/ls, pattern for search, `server/tool` for mcp) clamped to the remaining width. Count
  `TextDelta`/`ThinkingDelta` runes /4 as an estimate (reset per run). Steering count comes from a new
  `agent.PendingSteering() int`.

#### 2.4 Pager: search and approval awareness — **S/M**

- **Status: ✅ done** — the viewport's native highlight API did the work (no hand-rolled match mapping)**
- **What.**
  - `/` search with `n`/`N`, matches highlighted.
  - When an approval arrives while the pager is open, the title row shows `⚠ approval pending — q to answer`.
- **Why.** Tool outputs and diffs are long, and search is the pager's reason to exist. An approval that arrives
  while you're reading blocks the agent invisibly (the prompt only renders in the inline view).
- **How.** The viewport has no search. Keep `[]int` line offsets of matches over the soft-wrapped content
  (search the raw body, map to wrapped rows via `lipgloss.Wrap` widths), `SetYOffset`, and re-render
  highlighted spans. `pagerModel.view(pending bool)`.

#### 2.5 Notify when you're not looking — **S**

- **Status: ✅ done** — new config key `tui.notify` (`osc9`|`bell`|`off`, default `osc9`; SPECS §4)**
- **What.** Ring the bell / send OSC 9 (`\x1b]9;moca: done\x07`, which most modern terminals turn into a desktop
  notification) when a run longer than 30 s ends or an approval is waiting, **only while the terminal is
  unfocused**.
- **Why.** Long agent runs mean you switch windows, and an approval prompt silently stalls the run. Claude Code
  and others notify. It is nearly free.
- **How.** Bubble Tea v2 reports focus (`tea.FocusMsg`/`tea.BlurMsg`, enable via `View.ReportFocus`). Track
  `focused`. On `runDoneMsg` (elapsed > 30 s) / `approvalMsg` while unfocused, return a `tea.Raw` with the OSC
  sequence. Config `tui.notify: "bell"|"osc9"|"off"`, default `osc9`.

#### 2.6 Resize artifacts — **M (investigation first)**

- **Status: 🔎 investigated, not changed.** Reproduced in tmux (100→50 columns leaves one duplicate rule row above the input box). Cause: the terminal reflows the old full-width frame rows before Bubble Tea erases them (the renderer already erases and redraws on resize; `ClearScreen` would blank the visible history). The only workable fix is moving the cursor up by the number of extra rows the old frame wrapped into before the next render — correct on reflowing terminals but it would erase scrollback on xterm-style terminals that truncate instead, and only tmux could be tested here. Documented as a known limitation (SPECS §3.5.1); revisit with a terminal matrix (kitty, iTerm2, WezTerm, Windows Terminal, xterm) and a reflow probe (query cursor position before/after a resize)**
- **What.** Shrinking the terminal width leaves duplicated rules and fragments of the old status bar in the
  scrollback (seen live at 100→50 columns).
- **Why.** The terminal reflows the old frame's full-width lines, so the inline renderer's "move up N rows and
  redraw" lands short. The same class of bug as the shrink guard.
- **How.** On a `WindowSizeMsg` whose width *decreases*, erase the frame region before the next render
  (`tea.ClearScreen` clears only the visible screen in inline mode, so scrollback is unaffected; verify on
  tmux, kitty, iTerm, Windows Terminal). Alternative: render rules one cell shorter than the width, so a reflow
  of the *frame* is less likely to split lines. That helps only for small shrinks. Add a manual check to the
  ship gate doc. Document the residual: scrollback bands printed at the old width reflow, which is
  unavoidable inline.

#### 2.7 Quieter first run — **S**

- **Status: ✅ done** — `hints.json` in the data dir; the placeholder names `ctrl+j` whenever key disambiguation is unavailable**
- **What.** The `hint: this terminal can't report shift+enter…` line prints on every launch in
  tmux/screen/older terminals. Show it once per machine, and keep it in `/help`.
- **Why.** A permanent warning on every start trains users to ignore the welcome area.
- **How.** A marker file `$XDG_DATA_HOME/moca/hints.json` (`{"shiftEnter": true}`) is checked in `hintCheckMsg`.
  Also mention `ctrl+j` in the placeholder when `!kbdEnhanced`: `Message moca…  (ctrl+j newline · /help)`.

#### 2.8 Echo `!cmd` like other input — **S**

- **Status: ✅ done**
- **What.** A submitted `!cmd`/`!!cmd` prints in the `›` user band before its output, like `/` commands.
- **Why.** Consistency: "the transcript reads as everything the user sent" (SPECS §3.5) currently excludes shell
  input.
- **How.** In `submit()`, `tea.Sequence(m.printlnUser("› "+text), …)` for `KindShell`/`KindShellLocal`, and drop the
  then-redundant `$ cmd` header line in `handleShellDone` (keep it in the transcript note).

### P3 — nice to have

#### 3.1 `shift+tab` cycles effort — **S**

- **Status: ✅ done**
- **What.** Cycles through the current model's supported effort levels, with a transient `effort: high` notice.
- **Why.** Pi cycles thinking level with one key. `/effort high` is 12 keystrokes for the knob most worth
  tuning per task in a cost-focused agent.
- **How.** Same refusal rules as `/effort`. It reuses `supportedEfforts` and `agent.SetEffort`.

#### 3.2 Welcome line with context — **S**

- **Status: ⏭ skipped** — on reflection it duplicates the status bar (model, cwd and branch are one line below the input from the first frame); the greeting costs one row and tells a first-time user where to type**
- **What.** `moca v0.1.1 · opencode-go/deepseek-v4.1-flash · ~/proj (main)` as the second line, replacing "How can
  I help you today?", or only when the status bar is narrower than its content.
- **Why.** The greeting carries no information, and the bar already shows this. Low priority: it's mostly
  taste.

#### 3.3 `NO_COLOR` audit — **S**

- **Status: ✅ done, and it found a real bug:** Bubble Tea detects `NO_COLOR` but `tea.Println` bypasses its downsampling, so scrollback kept raw truecolor — on `NO_COLOR` *and* on 256-color terminals (e.g. Terminal.app). Scrollback is now converted through `colorprofile.Writer` by the detected profile (detected at startup so the welcome line is covered)**
- **What.** Verify `NO_COLOR=1` drops every color (bands included) and the TUI stays legible (`›` and markers
  still distinguish roles).
- **Why.** Accessibility and convention. The hex colors are hard-coded.
- **How.** Bubble Tea v2 downsamples by color profile. Check it honors `NO_COLOR`, and if not, force the ASCII
  profile. Add a test rendering `styleBlock` under the ASCII profile.

---

## 2. Deliberately out of scope

| Idea (seen in) | Why not for moca |
|---|---|
| Full-screen layout, sidebar, panes (OpenCode) | It breaks the inline/scrollback model that keeps moca small and lets the terminal own history, search and copy. |
| Theme system / custom status line scripts (OpenCode, Claude Code) | Light/dark auto-pick + `NO_COLOR` covers the need. Themes are surface area to maintain. |
| Todo/plan panel (Claude Code) | It needs a todo tool, and the seven tools are frozen for v1. |
| Esc-Esc rewind / edit an earlier message (Claude Code, Pi `/tree`) | It needs session branching in `agent`/`session`, which is a design change, not TUI polish. Revisit with session v2. |
| Image paste | Multimodal input is a provider-layer feature first. |
| Vim mode, custom keybindings | Niche. The textarea's emacs-style keys suffice. |
| Inlining `@file` contents into the prompt | Defeats windowed reads and caching. 1.5 sends the path only. |

---

## 3. Suggested sequencing (as planned; all but 2.6 and 3.2 landed together on this branch)

1. **PR A (S items, low risk):** 1.3, 2.1, 2.2, 2.7, 2.8, 3.3.
2. **PR B:** 1.2 diff preview + 1.1 markdown-lite. They share the styled-span `styleBlock` refactor.
3. **PR C:** extract the `picker`, then 1.4, 1.6 (b) and 1.7. Dropdown/picker generalization is the common work.
4. **PR D:** 1.5 `@path`, 2.3 activity row, 2.4 pager search, 2.5 notifications, 1.6 (a) resume replay.
5. **Investigation:** 2.6 resize, with a manual matrix across terminals before changing code.
