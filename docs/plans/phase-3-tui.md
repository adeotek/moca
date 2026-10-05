# Phase 3 — TUI Shell, Slash Commands, Status Bar, Steering, Model Switching — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `moca` (no `-p`) opens an inline Bubble Tea TUI: immutable scrollback, a multi-line input with native paste, a one-line status bar below it from the first frame, numbered collapsible tool/thinking items with a pager, steering during runs, interactive trust/approval prompts, `!`/`!!`, and slash commands (`/model /effort /hard /clear /compact /cost /undo /copy /show /help` plus user prompt templates). Switching model mid-session works across providers.

**Architecture:** The behaviour that matters lives in **pure, unit-tested code**: agent additions (history transform, model switching, steering, estimates), the JSONC array append, prompt templates, status formatting, command parsing, the input state machine, and the item store. A thin Bubble Tea glue layer (`internal/tui/app.go`) maps terminal messages to those functions. The agent runs in a goroutine; its events reach the program via `Program.Send`, and approval prompts block the agent on a reply channel.

**Tech Stack:** Go 1.27.1; Bubble Tea **v2** (`charm.land/bubbletea/v2`; keyboard enhancements = Kitty protocol / modifyOtherKeys, bracketed paste, `Println` for inline committed output), `charm.land/bubbles/v2` (textarea, viewport), `charm.land/lipgloss/v2`. These are UI deps; `mvdan.cc/sh` stays the only non-UI third-party dep.

**Spec:** `docs/specs/DESIGN.md` (rev 11) — §3 (cross-provider history transform, effort clamp), §4 (`/undo`), §6 (token estimate, used by the status bar), §7 (interactive trust + approvals), §8 (`model_change`, `/clear`), §10 (slash commands), §11 (all), phase plan item 3.

**Builds on:** Phases 1–2. Uses `agent.Agent/Start/StartOptions/Event*`, `session.*`, `tools.Asker/Question/Answer/RunShell/ShellEnv/Truncate`, `provider.Registry/Model`, `config.*`, `skills.ParseFrontmatter/Dir`, `permissions.LoadTrust`.

## Global Constraints

- Inline rendering, **no alt-screen** for the main view; finished output goes to terminal scrollback via `tea.Println` and is never redrawn. Only the live region re-renders: streaming text, approval prompt, input box, status bar (§11).
- Layout top→bottom: scrollback · input box · status bar (1 line, below the input).
- Status bar contains exactly: `cwd` (home-abbreviated session workdir, constant) · `branch` (`*` if dirty; non-git = dimmed `-`) · `provider/model · effort` (effective: `off min low med high xhigh max`) · `ctx <window> · <pct>%` · `<in>/<out>` tokens · `$<cost>` (4 decimals) or `sub` for OAuth providers. Transient states (`retry 2/5 · 4s`) replace the cost field. The branch updates at every turn end. Updates are event-driven.
- Keys: `enter` send · `shift+enter` newline (fallback `alt+enter`, `ctrl+j`; one-time hint if the terminal can't report it) · `↑/↓` history at buffer edges · `esc` interrupt run · `ctrl+c` clears non-empty input, double `ctrl+c` within 1s on empty input quits · `ctrl+o` pager on latest item · `alt+p` toggles paste chips.
- Pastes: verbatim, never auto-sent; >50 lines (or any control bytes) render as a `[paste N lines #K]` chip, with full content kept until send.
- Tool items: `▸ #7 edit main.go [+3 −1]`; thinking items: `⋯ #6 thinking N lines`. Thinking is never shown expanded outside the pager.
- Steering: a message sent during a run is appended **after the current tool batch's results**; if the turn had no tool calls it becomes the next user message; `esc` returns queued steering to the editor.
- `/hard` toggles: save `{model, effort}` → `modelHard` + effort `high` (clamped); second use restores. Both directions write `model_change`.
- Switching model forfeits the prompt cache; `/model` and `/hard` say so.
- Cross-provider transform: thinking replayed unchanged only to the exact producing model (qualified id); otherwise non-empty thinking → text block `[prior reasoning]\n…`, empty thinking dropped; tool-call ids normalized to `[a-zA-Z0-9_-]{1,40}` deterministically; the transcript keeps originals.
- `!cmd`: output (truncated 30K) enters the conversation as a user message. `!!cmd`: shown locally only. Both bypass the model-facing analyser.
- Built-in command names win over prompt templates.
- Yolo (§7.5): `/yolo` toggles it **between runs only** (refused while a run is in progress). While on, a red `YOLO` field leads the status bar and is never dropped for width. With yolo on and no explicit `--approve`/`--no-approve`, the workdir is trusted without the trust prompt.
- The interactive trust prompt appears on first open of an untrusted workdir **that has project resources** (`.moca/`, `AGENTS.md` or `CLAUDE.md`). The decision is saved in `trust.json`.

## Review Focus

1. **A paste containing ANSI escape sequences or a literal `\x1b[201~`** (bracketed-paste end marker) → inserted as text, and the escape bytes are rendered harmlessly (shown as `^[`) in chips, scrollback and pager, never interpreted by the terminal. Tested in Task 9.
2. **A terminal narrower than the status bar's content (e.g. 60 columns)** → the status bar truncates by dropping/shortening fields in a fixed priority (cwd first, then branch, then tokens) and never wraps to a second line. Tested in Task 7.
3. **Approval prompt arrives while the user is mid-typing** → the typed draft is preserved; approval keys (`a`/`A`/`d`) go to the prompt only while it is shown; `esc` on the prompt = deny. Tested in Task 11.
4. **`/model` to a model whose provider key is unset** → the switch is refused with the env-var error, and the current model stays active with no `model_change` written. Tested in Task 3.
5. **Steering text sent, then `esc` before the batch ends** → the steering text returns to the editor (prepended to any draft) and is NOT in the transcript. Tested in Task 4.

---

## File Structure

```
internal/compact/
  estimate.go estimate_test.go        chars/4 helpers (phase 4 grows this package)
internal/agent/
  transform.go transform_test.go      cross-provider history transform + id normalization
  control.go control_test.go          SetModel/SetEffort/ToggleHard/Steer/AddNote/Undo/ContextTokens/Status
  agent.go                            (modified) steering drain points, estimate anchor, transform in request()
internal/config/
  edit.go edit_test.go                AppendString(path, keyPath, value, init)
internal/skills/
  prompts.go prompts_test.go          slash-command templates
internal/tui/
  format.go format_test.go            tokens/window/effort/home formatting
  status.go status_test.go            RenderStatus
  commands.go commands_test.go        ParseInput → Command | Prompt | Shell | Text
  input.go input_test.go              input state machine (history, chips, ctrl+c)
  items.go items_test.go              numbered items + one-liners
  pager.go                            viewport pager (alt-screen while open)
  sanitize.go sanitize_test.go        escape neutralization for display
  app.go app_test.go                  Bubble Tea glue
  bridge.go                           agent events → tea.Msg, blocking Asker
  trust.go                            startup trust prompt model
cmd/moca/
  tui.go                              runTUI
  main.go                             (modified) TUI path
```

---

### Task 1: Pin Bubble Tea v2 and lock the APIs this plan uses

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/tui/teaapi_test.go`

**Interfaces:**
- Produces: a compile-checked list of the v2 symbols the glue layer depends on. If any name differs in the pinned version, fix it **here and in `app.go`/`bridge.go`/`pager.go` only**. The pure layers don't import Bubble Tea.

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get charm.land/bubbletea/v2@latest charm.land/bubbles/v2@latest charm.land/lipgloss/v2@latest
go mod tidy
```

- [ ] **Step 2: Read the API you are pinning**

Run: `go doc charm.land/bubbletea/v2 | head -150` and `go doc charm.land/bubbletea/v2.View`, `go doc charm.land/bubbletea/v2.KeyPressMsg`, `go doc charm.land/bubbletea/v2.PasteMsg`, `go doc charm.land/bubbletea/v2.KeyboardEnhancementsMsg`, `go doc charm.land/bubbletea/v2.Println`, `go doc charm.land/bubbletea/v2.SetClipboard`, `go doc charm.land/bubbles/v2/textarea`, `go doc charm.land/bubbles/v2/viewport`.

Write down the exact names for:
- the key-press message type and how to get `"shift+enter"`, `"alt+p"`, `"ctrl+c"`, `"esc"` strings;
- the paste message and its content field;
- the keyboard-enhancement capability message;
- how `View()` requests keyboard enhancements and alt-screen (v2 uses fields on the returned `tea.View`);
- `Println` and the clipboard command.

- [ ] **Step 3: Write the compile-lock test**

```go
// internal/tui/teaapi_test.go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
)

// TestTeaAPI fails to compile if a v2 symbol the glue relies on moved.
func TestTeaAPI(t *testing.T) {
	var _ tea.KeyPressMsg
	var _ tea.PasteMsg
	var _ tea.KeyboardEnhancementsMsg
	var _ tea.WindowSizeMsg
	var _ tea.Cmd = tea.Println("x")
	var _ tea.Cmd = tea.SetClipboard("x")
	var _ tea.Cmd = tea.Quit
	var v tea.View = tea.NewView("x")
	v.AltScreen = true
	_ = v
	_ = textarea.New()
	_ = viewport.New()
}
```

Run: `go test ./internal/tui/ -run TeaAPI`
Expected: PASS. If a symbol is missing, use the name from Step 2 and record the substitution in a comment at the top of this test file. Later tasks cite these names.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/tui/teaapi_test.go
git commit -m "build: pin Bubble Tea v2, bubbles, lipgloss; lock used APIs"
```

---

### Task 2: Token estimate helpers (`internal/compact`)

**Files:**
- Create: `internal/compact/estimate.go`
- Test: `internal/compact/estimate_test.go`

**Interfaces:**
- Produces (phase 4 reuses these):
  - `func Tokens(chars int) int` — `(chars+3)/4`.
  - `func MessageChars(m llm.Message) int` — text + thinking text + tool-call name/input + tool-result content.
  - `func RequestChars(system string, tools []llm.ToolSpec, msgs []llm.Message) int`
  - `func UsageTokens(u llm.Usage) int` — `Input + CacheRead + CacheWrite + Output`.

- [ ] **Step 1: Write failing test**

```go
// internal/compact/estimate_test.go
package compact

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestEstimate(t *testing.T) {
	if Tokens(0) != 0 || Tokens(1) != 1 || Tokens(8) != 2 {
		t.Fatal("chars/4 rounded up")
	}
	m := llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
		{Type: llm.BlockText, Text: "abcd"},
		{Type: llm.BlockThinking, Text: "efgh"},
		{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{Name: "ls", Input: json.RawMessage(`{}`)}},
		{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{Content: "123456"}},
	}}
	if MessageChars(m) != 4+4+2+2+6 {
		t.Fatal(MessageChars(m))
	}
	if RequestChars("sys", []llm.ToolSpec{{Name: "a", Description: "bb", Schema: json.RawMessage(`{}`)}}, []llm.Message{m}) != 3+1+2+2+18 {
		t.Fatal("request chars")
	}
	if UsageTokens(llm.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4}) != 10 {
		t.Fatal("usage")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/compact/` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/compact/estimate.go

// Package compact owns token accounting and (phase 4) compaction. It never
// imports provider: summarization is injected by agent (§2).
package compact

import "github.com/adeotek/moca/internal/llm"

func Tokens(chars int) int { return (chars + 3) / 4 }

func MessageChars(m llm.Message) int {
	n := 0
	for _, c := range m.Content {
		n += len(c.Text)
		if c.ToolCall != nil {
			n += len(c.ToolCall.Name) + len(c.ToolCall.Input)
		}
		if c.ToolResult != nil {
			n += len(c.ToolResult.Content)
		}
	}
	return n
}

func RequestChars(system string, tools []llm.ToolSpec, msgs []llm.Message) int {
	n := len(system)
	for _, t := range tools {
		n += len(t.Name) + len(t.Description) + len(t.Schema)
	}
	for _, m := range msgs {
		n += MessageChars(m)
	}
	return n
}

func UsageTokens(u llm.Usage) int { return u.Input + u.CacheRead + u.CacheWrite + u.Output }
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/compact && git commit -m "feat(compact): chars/4 token estimate helpers"`

---

### Task 3: Cross-provider history transform + model switching

**Files:**
- Create: `internal/agent/transform.go`, `internal/agent/control.go`
- Modify: `internal/agent/agent.go` (`request()` applies the transform; `turn()` stamps thinking blocks with the qualified model)
- Test: `internal/agent/transform_test.go`, `internal/agent/control_test.go`

**Interfaces:**
- Produces:
  - `func TransformHistory(msgs []llm.Message, target string) []llm.Message` — `target` = qualified model id.
  - `func NormalizeToolID(id string) string` — valid ids unchanged; otherwise `"c" + hex(sha256(id))[:16]`.
  - `func (a *Agent) SetModel(qualified string, effort llm.Effort) error` — `effort == ""` → the target model's default. Resolves first (provider key unset → error, nothing changes), then sets and appends `model_change{model, effort}`.
  - `func (a *Agent) SetEffort(e llm.Effort) (llm.Effort, error)` — clamps; appends `model_change`; returns the effective level.
  - `func (a *Agent) ToggleHard() (on bool, err error)`
  - `func (a *Agent) HardOn() bool`
  - `func (a *Agent) Models() []provider.Model` (from the registry, for `/model` listing)
- `turn()` change: every thinking block persisted gets `Model = a.model.Qualified()` (adapters set the bare id; the agent owns qualification).
- `request()` change: `Messages: TransformHistory(session.Messages(a.entries), a.model.Qualified())`.
- Key-resolution check for `SetModel`: `Registry.Resolve` resolves keys lazily, so `SetModel` additionally calls a new `provider.(*Registry).CheckCredential(qualified string) error` that runs the credential func once (env lookup only; no network). Add that method to `internal/provider/registry.go` in this task.

- [ ] **Step 1: Write failing tests**

```go
// internal/agent/transform_test.go
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestTransformHistory(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "deep thought", Signature: "SIG", Model: "anthropic/claude-x"},
			{Type: llm.BlockThinking, Text: "", Signature: "OMITTED", Model: "anthropic/claude-x"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "toolu_01:weird/id", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "toolu_01:weird/id", Content: "x"}}}},
	}
	same := TransformHistory(msgs, "anthropic/claude-x")
	if same[1].Content[0].Signature != "SIG" || len(same[1].Content) != 3 {
		t.Fatal("same model: thinking replayed unchanged")
	}
	other := TransformHistory(msgs, "opencode-go/glm-5.3")
	c := other[1].Content
	if len(c) != 2 || c[0].Type != llm.BlockText || c[0].Text != "[prior reasoning]\ndeep thought" {
		t.Fatalf("other model: %+v", c)
	}
	id := c[1].ToolCall.ID
	if id != other[2].Content[0].ToolResult.CallID || strings.ContainsAny(id, ":/") || len(id) > 40 {
		t.Fatalf("ids normalized consistently: %q", id)
	}
	if msgs[1].Content[0].Signature != "SIG" || msgs[1].Content[2].ToolCall.ID != "toolu_01:weird/id" {
		t.Fatal("input must not be mutated (transcript keeps originals)")
	}
}

func TestTransformDropsEmptiedAssistantAndMerges(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockThinking, Model: "x/y"}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "b"}}},
	}
	out := TransformHistory(msgs, "p/q")
	if len(out) != 1 || len(out[0].Content) != 2 {
		t.Fatalf("empty assistant dropped, users merged: %+v", out)
	}
}

func TestNormalizeToolID(t *testing.T) {
	if NormalizeToolID("call_abc-1") != "call_abc-1" {
		t.Fatal("valid ids unchanged")
	}
	long := strings.Repeat("a", 41)
	if n := NormalizeToolID(long); len(n) > 40 || n != NormalizeToolID(long) {
		t.Fatal("deterministic and bounded")
	}
}
```

```go
// internal/agent/control_test.go
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestSetModelAndEffortWriteModelChange(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, `"modelHard":"fake/big"`, `"big":{"contextWindow":131072}`)
	if err := a.SetModel("fake/big", ""); err != nil {
		t.Fatal(err)
	}
	if a.Model().Qualified() != "fake/big" {
		t.Fatal("switched")
	}
	if _, err := a.SetEffort(llm.EffortHigh); err != nil {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var changes int
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			changes++
		}
	}
	if changes != 2 {
		t.Fatal("model_change per switch")
	}
}

func TestSetModelRefusedWhenKeyMissing(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	t.Setenv("MOCA_T_KEY", "")
	before := a.Model().Qualified()
	err := a.SetModel("fake/m", "")
	if err == nil || !strings.Contains(err.Error(), "MOCA_T_KEY") || a.Model().Qualified() != before {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			t.Fatal("no model_change on failure")
		}
	}
}

func TestToggleHard(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, `"modelHard":"fake/big"`, `"big":{"contextWindow":131072}`)
	orig, origE := a.Model().Qualified(), a.Effort()
	on, err := a.ToggleHard()
	if err != nil || !on || a.Model().Qualified() != "fake/big" {
		t.Fatal(on, err)
	}
	on, _ = a.ToggleHard()
	if on || a.Model().Qualified() != orig || a.Effort() != origE {
		t.Fatal("restores saved pair")
	}
	_ = context.Background
}
```

`startTestWith(t, s, extraTop, extraModels string)` generalizes phase 2's `startTest`. It splices `extraTop` into the top-level config object and `extraModels` into the `fake` provider's `models` map. Refactor `startTest` to call it with empty strings, so existing tests keep working.

- [ ] **Step 2: Run** — `go test ./internal/agent/ -run 'Transform|Normalize|SetModel|Toggle'` → FAIL.

- [ ] **Step 3: Implement `transform.go`**

```go
// internal/agent/transform.go
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/adeotek/moca/internal/llm"
)

var validToolID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)

func NormalizeToolID(id string) string {
	if validToolID.MatchString(id) {
		return id
	}
	h := sha256.Sum256([]byte(id))
	return "c" + hex.EncodeToString(h[:])[:16]
}

// TransformHistory adapts stored history for the target model (§3). The
// input is never mutated; the transcript keeps originals.
func TransformHistory(msgs []llm.Message, target string) []llm.Message {
	var out []llm.Message
	for _, m := range msgs {
		var blocks []llm.ContentBlock
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockThinking:
				if c.Model == target {
					blocks = append(blocks, c)
				} else if c.Text != "" {
					blocks = append(blocks, llm.ContentBlock{Type: llm.BlockText, Text: "[prior reasoning]\n" + c.Text})
				}
			case llm.BlockToolUse:
				call := *c.ToolCall
				call.ID = NormalizeToolID(call.ID)
				blocks = append(blocks, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: &call})
			case llm.BlockToolResult:
				r := *c.ToolResult
				r.CallID = NormalizeToolID(r.CallID)
				blocks = append(blocks, llm.ContentBlock{Type: llm.BlockToolResult, ToolResult: &r})
			default:
				blocks = append(blocks, c)
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Content = append(out[n-1].Content, blocks...)
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: blocks})
	}
	return out
}
```

- [ ] **Step 4: Implement `control.go` + registry `CheckCredential` + agent changes**

```go
// internal/provider/registry.go — add:
// CheckCredential resolves the provider's credential once (env lookup or
// token store read; no network) so a model switch fails fast.
func (r *Registry) CheckCredential(qualified string) error {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return err
	}
	_, err = r.credential(pname)(context.Background())
	return err
}
```

```go
// internal/agent/control.go
package agent

import (
	"errors"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
)

type savedModel struct {
	model  string
	effort llm.Effort
}

func (a *Agent) Models() []provider.Model { return a.opts.Providers.Models() }

func (a *Agent) recordModel() {
	a.append(session.Entry{Type: session.TypeModelChange,
		ModelChange: &session.ModelChange{Model: a.model.Qualified(), Effort: string(a.effort)}})
	a.anchorValid = false // estimate falls back to chars/4 after a switch (§6)
}

func (a *Agent) SetModel(q string, e llm.Effort) error {
	if err := a.opts.Providers.CheckCredential(q); err != nil {
		return err
	}
	prev, prevAd, prevE := a.model, a.adapter, a.effort
	if err := a.setModel(q, e); err != nil {
		a.model, a.adapter, a.effort = prev, prevAd, prevE
		return err
	}
	a.recordModel()
	return nil
}

func (a *Agent) SetEffort(e llm.Effort) (llm.Effort, error) {
	a.effort = a.model.ClampEffort(e)
	a.recordModel()
	return a.effort, nil
}

func (a *Agent) HardOn() bool { return a.hard != nil }

func (a *Agent) ToggleHard() (bool, error) {
	if a.hard != nil {
		saved := *a.hard
		if err := a.SetModel(saved.model, saved.effort); err != nil {
			return true, err
		}
		a.hard = nil
		return false, nil
	}
	if a.opts.Config.ModelHard == "" {
		return false, errors.New("modelHard is not configured")
	}
	saved := savedModel{a.model.Qualified(), a.effort}
	if err := a.SetModel(a.opts.Config.ModelHard, llm.EffortHigh); err != nil {
		return false, err
	}
	a.hard = &saved
	return true, nil
}
```

In `agent.go`:
- Add fields `hard *savedModel`, `anchorTokens int`, `anchorEntries int`, `anchorValid bool` to `Agent`. The anchor fields are used in Task 4.
- In `turn()`, when building `body`, set `c.Model = a.model.Qualified()` for thinking blocks before appending.
- In `request()`, replace `Messages: session.Messages(a.entries)` with `Messages: TransformHistory(session.Messages(a.entries), a.model.Qualified())`.

- [ ] **Step 5: Run** — `go test ./internal/... -race` → PASS (phase-2 tests included).

- [ ] **Step 6: Commit**

```bash
git add internal/agent internal/provider/registry.go
git commit -m "feat(agent): cross-provider history transform, model/effort switching, /hard toggle"
```

---

### Task 4: Steering, `!` notes, undo, context estimate, status snapshot

**Files:**
- Modify: `internal/agent/agent.go`, `internal/agent/control.go`
- Test: `internal/agent/control_test.go` (extend)

**Interfaces:**
- Produces:
  - `func (a *Agent) Steer(text string)` — thread-safe queue.
  - `func (a *Agent) TakeSteering() []string` — drains (the TUI calls it after an aborted run to return texts to the editor).
  - `func (a *Agent) AddNote(text string) error` — appends a user `message` entry without running (the `!` prefix).
  - `func (a *Agent) Undo() (string, error)` — `a.opts.Snapshots.Undo()`.
  - `func (a *Agent) ContextTokens() int` — usage-anchored estimate (§6): if `anchorValid`, `anchorTokens + Tokens(chars of entries appended after anchorEntries)`; else `Tokens(RequestChars(system, specs, transformed messages))`.
  - `func (a *Agent) Status() Status`, with `type Status struct { Model provider.Model; Effort llm.Effort; ContextTokens, Window int; Usage llm.Usage; Cost float64; Sub bool; Hard bool; Yolo bool }`. `Sub` = the provider's `auth == "oauth"`.
  - `type SteeringApplied struct{ Texts []string }` event, emitted when queued texts are appended.
- Loop changes in `Run`:
  - After every complete tool batch (before the next request), drain the queue and append each text as a user `message` entry. Rebuild merges them after the tool results.
  - When a turn has **no** tool calls and the queue is non-empty, drain, append, and **continue the loop** instead of returning.
  - On abort (`ctx` done), the queue is left untouched for `TakeSteering`.
  - After each successful `turn()`: `anchorTokens = compact.UsageTokens(resp.Usage)`, `anchorEntries = len(a.entries)`, `anchorValid = true`.

- [ ] **Step 1: Write failing tests**

```go
// append to internal/agent/control_test.go

func TestSteeringLandsAfterToolResults(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}), textTurn("done"))
	a, _, _ := startTestWith(t, s, "", "")
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolStart); ok {
			a.Steer("also check README")
		}
	}
	a.Run(context.Background(), "go")
	msgs := s.bodies[1]["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	prev := msgs[len(msgs)-2].(map[string]any)
	if prev["role"] != "tool" || last["role"] != "user" || last["content"] != "also check README" {
		t.Fatalf("steering must follow the tool result: %v / %v", prev, last)
	}
}

func TestSteeringWithoutToolsBecomesNextMessage(t *testing.T) {
	s := newScript(t, textTurn("first"), textTurn("second"))
	a, _, _ := startTestWith(t, s, "", "")
	a.opts.Emit = func(e Event) {
		if _, ok := e.(TextDelta); ok && len(s.bodies) == 1 {
			a.Steer("follow-up")
		}
	}
	out, _ := a.Run(context.Background(), "go")
	if out.Text != "second" || len(s.bodies) != 2 {
		t.Fatal(out, len(s.bodies))
	}
}

func TestAbortReturnsSteering(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}, [2]string{"ls", `{}`}))
	a, _, _ := startTestWith(t, s, "", "")
	ctx, cancel := context.WithCancel(context.Background())
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolEnd); ok {
			a.Steer("draft")
			cancel()
		}
	}
	a.Run(ctx, "go")
	if got := a.TakeSteering(); len(got) != 1 || got[0] != "draft" {
		t.Fatal(got)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	for _, e := range entries {
		if e.Message != nil && len(e.Message.Content) > 0 && e.Message.Content[0].Text == "draft" {
			t.Fatal("aborted steering must not reach the transcript")
		}
	}
}

func TestContextTokensAnchored(t *testing.T) {
	s := newScript(t, textTurn("ok")) // usage: prompt 10 + completion 5
	a, _, _ := startTestWith(t, s, "", "")
	before := a.ContextTokens()
	if before == 0 {
		t.Fatal("chars/4 fallback before the first response")
	}
	a.Run(context.Background(), "hi")
	if a.ContextTokens() != 15 {
		t.Fatalf("anchored at usage: %d", a.ContextTokens())
	}
	a.AddNote(strings.Repeat("x", 40))
	if a.ContextTokens() != 15+10 {
		t.Fatalf("anchor + delta: %d", a.ContextTokens())
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

In `control.go`:

```go
type SteeringApplied struct{ Texts []string }

func (SteeringApplied) isEvent() {}

type Status struct {
	Model         provider.Model
	Effort        llm.Effort
	ContextTokens int
	Window        int
	Usage         llm.Usage
	Cost          float64
	Sub           bool
	Hard          bool
	Yolo          bool
}

func (a *Agent) Steer(text string) {
	a.mu.Lock()
	a.steer = append(a.steer, text)
	a.mu.Unlock()
}

func (a *Agent) TakeSteering() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.steer
	a.steer = nil
	return s
}

// applySteering appends queued texts as user messages; true if any.
func (a *Agent) applySteering() bool {
	texts := a.TakeSteering()
	for _, t := range texts {
		a.append(session.Entry{Type: session.TypeMessage, Message: userText(t)})
	}
	if len(texts) > 0 {
		a.emit(SteeringApplied{texts})
	}
	return len(texts) > 0
}

func (a *Agent) AddNote(text string) error {
	_, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(text)})
	return err
}

func (a *Agent) Undo() (string, error) { return a.opts.Snapshots.Undo() }

func (a *Agent) ContextTokens() int {
	if a.anchorValid {
		n := 0
		for _, m := range session.Messages(a.entries[a.anchorEntries:]) {
			n += compact.MessageChars(m)
		}
		return a.anchorTokens + compact.Tokens(n)
	}
	return compact.Tokens(compact.RequestChars(a.opts.System, a.opts.Tools.Specs(),
		TransformHistory(session.Messages(a.entries), a.model.Qualified())))
}

func (a *Agent) Status() Status {
	return Status{Model: a.model, Effort: a.effort, ContextTokens: a.ContextTokens(), Window: a.model.ContextWindow,
		Usage: a.usage, Cost: a.cost, Sub: a.opts.Config.Providers[a.model.Provider].Auth == "oauth", Hard: a.hard != nil, Yolo: a.yolo}
}
```

(Add `mu sync.Mutex` and `steer []string` fields to `Agent`; import `sync`, `compact`.)

`session.Messages(a.entries[a.anchorEntries:])` on a slice that starts mid-turn (e.g. with tool results) still produces the right character count: rebuild only groups, it doesn't validate.

In `Run`:
- At the end of each successful `turn()` call site, set `a.anchorTokens, a.anchorEntries, a.anchorValid = compact.UsageTokens(resp.Usage), len(a.entries), true`. Put this inside `turn()` right after persisting.
- Replace the no-tool-calls branch with:

```go
		if len(calls) == 0 {
			if a.applySteering() {
				continue
			}
			return Outcome{Text: textOf(resp.Message)}, nil
		}
```

- After the sequential tool loop (and its `ctx.Err()` check), add `a.applySteering()`.

- [ ] **Step 4: Run** — `go test ./internal/agent/ -race -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent
git commit -m "feat(agent): steering after tool results, notes, undo, anchored context estimate, status"
```

---

### Task 5: JSONC-preserving array append (`allow-always` persistence)

**Files:**
- Create: `internal/config/edit.go`
- Test: `internal/config/edit_test.go`

**Interfaces:**
- Produces: `func AppendString(path string, keyPath []string, value string, init []string) error`.
  - Phase 7's `config.SetString` and phase 5's `config.SetObjectEntry` build on this task's scanner.
  - Missing file → create `{ "<k1>": { … "<kn>": [init…, value] } }` (mode 0600, dirs 0700).
  - Array exists → insert `, "value"` before its closing `]` (or `"value"` if empty); no-op if already present.
  - Some key on the path missing → insert `"<missing>": <nested object/array>` right after the deepest existing object's `{`, with `init` + value as the array.
  - Comments, formatting and everything else stay byte-identical. Writes `<path>.bak` first, then replaces atomically.
- Uses `Standardize` (same offsets) and a small offset-tracking JSON scanner over the standardized bytes.
- Callers: TUI allow-always for shell → `AppendString(file, []string{"shell","allow"}, name, DefaultShellAllow)`. MCP (phase 5) → `[]string{"mcp","servers",server,"approve"}` with `init = nil`.

- [ ] **Step 1: Write failing tests**

```go
// internal/config/edit_test.go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendStringExistingArray(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	src := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\",], // trailing\n  },\n}\n"
	os.WriteFile(p, []byte(src), 0o600)
	if err := AppendString(p, []string{"shell", "allow"}, "python", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := "{\n  // keep me\n  \"shell\": {\n    \"allow\": [\"go\", \"git\", \"python\"], // trailing\n  },\n}\n"
	if string(got) != want {
		t.Fatalf("got\n%s", got)
	}
	AppendString(p, []string{"shell", "allow"}, "python", nil)
	if got2, _ := os.ReadFile(p); string(got2) != want {
		t.Fatal("idempotent")
	}
	if _, err := os.Stat(p + ".bak"); err != nil {
		t.Fatal("backup")
	}
	if _, err := Parse(got); err != nil {
		t.Fatal("result must stay valid:", err)
	}
}

func TestAppendStringMissingKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.jsonc")
	os.WriteFile(p, []byte("{\n  \"model\": \"a/b\" // m\n}\n"), 0o600)
	if err := AppendString(p, []string{"shell", "allow"}, "python", []string{"go"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	c, err := Parse(got)
	if err != nil || strings.Join(c.Shell.Allow, ",") != "go,python" || c.Model != "a/b" || !strings.Contains(string(got), "// m") {
		t.Fatalf("%s %v", got, err)
	}
}

func TestAppendStringEmptyArrayAndNoFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "c.jsonc")
	if err := AppendString(p, []string{"mcp", "servers", "gh", "approve"}, "create_issue", nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), `"approve": ["create_issue"]`) {
		t.Fatal(string(b))
	}
	p2 := filepath.Join(dir, "e.jsonc")
	os.WriteFile(p2, []byte(`{"shell":{"allow":[ ]}}`), 0o600)
	AppendString(p2, []string{"shell", "allow"}, "x", nil)
	if b, _ := os.ReadFile(p2); string(b) != `{"shell":{"allow":[ "x"]}}` {
		t.Fatal(string(b))
	}
}

func TestAppendStringCommentWithComma(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		// no trailing comma; the comma lives in the comment and must survive
		"{\"shell\":{\"allow\":[\"go\" // keep, ok\n]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" // keep, ok\n]}}",
		// real trailing comma followed by a comment that also has one
		"{\"shell\":{\"allow\":[\"go\", // a, b\n]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" // a, b\n]}}",
		// block comment with a comma, no trailing comma
		"{\"shell\":{\"allow\":[\"go\" /* x, y */]}}": "{\"shell\":{\"allow\":[\"go\", \"x\" /* x, y */]}}",
	}
	for in, want := range cases {
		p := filepath.Join(dir, "c.jsonc")
		os.WriteFile(p, []byte(in), 0o600)
		if err := AppendString(p, []string{"shell", "allow"}, "x", nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(p); string(got) != want {
			t.Errorf("in  %q\ngot %q\nwant %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/config/edit.go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// span is a JSON value's byte range in the (standardized) document.
type span struct{ start, end int } // end exclusive

type scanner struct {
	b []byte
	i int
}

func (s *scanner) ws() {
	for s.i < len(s.b) && strings.IndexByte(" \t\r\n", s.b[s.i]) >= 0 {
		s.i++
	}
}

func (s *scanner) str() (string, error) {
	start := s.i
	s.i++
	for s.i < len(s.b) {
		switch s.b[s.i] {
		case '\\':
			s.i += 2
			continue
		case '"':
			s.i++
			var v string
			err := json.Unmarshal(s.b[start:s.i], &v)
			return v, err
		}
		s.i++
	}
	return "", errors.New("unterminated string")
}

// value skips one value; if it is an object and path is non-empty it
// descends, returning the span of the deepest existing value on path and
// how many path elements were matched.
func (s *scanner) value(path []string) (span, int, error) {
	s.ws()
	start := s.i
	if s.i >= len(s.b) {
		return span{}, 0, errors.New("unexpected end")
	}
	switch s.b[s.i] {
	case '{':
		s.i++
		best, depth := span{start, -1}, 0
		for {
			s.ws()
			if s.b[s.i] == '}' {
				s.i++
				if depth == 0 {
					best.end = s.i
				}
				return best, depth, nil
			}
			if s.b[s.i] == ',' {
				s.i++
				continue
			}
			k, err := s.str()
			if err != nil {
				return span{}, 0, err
			}
			s.ws()
			s.i++ // ':'
			if len(path) > 0 && k == path[0] {
				sp, d, err := s.value(path[1:])
				if err != nil {
					return span{}, 0, err
				}
				best, depth = sp, d+1
			} else if _, _, err := s.value(nil); err != nil {
				return span{}, 0, err
			}
		}
	case '[':
		s.i++
		for {
			s.ws()
			if s.b[s.i] == ']' {
				s.i++
				return span{start, s.i}, 0, nil
			}
			if s.b[s.i] == ',' {
				s.i++
				continue
			}
			if _, _, err := s.value(nil); err != nil {
				return span{}, 0, err
			}
		}
	case '"':
		_, err := s.str()
		return span{start, s.i}, 0, err
	default:
		for s.i < len(s.b) && strings.IndexByte(",}] \t\r\n", s.b[s.i]) < 0 {
			s.i++
		}
		return span{start, s.i}, 0, nil
	}
}

func nested(keys []string, arr []string) string {
	b, _ := json.Marshal(arr)
	v := strings.ReplaceAll(string(b), `","`, `", "`)
	for i := len(keys) - 1; i >= 1; i-- {
		k, _ := json.Marshal(keys[i])
		v = fmt.Sprintf("{ %s: %s }", k, v)
	}
	k, _ := json.Marshal(keys[0])
	return fmt.Sprintf("%s: %s", k, v)
}

func concat(parts ...[]byte) []byte { return slices.Concat(parts...) }

// rawComma returns the index of the first ',' in gap that is outside a
// comment, or -1. gap holds only whitespace, comments and commas.
func rawComma(gap []byte) int {
	for i := 0; i < len(gap); i++ {
		switch {
		case gap[i] == ',':
			return i
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '/':
			for i < len(gap) && gap[i] != '\n' {
				i++
			}
		case gap[i] == '/' && i+1 < len(gap) && gap[i+1] == '*':
			end := strings.Index(string(gap[i+2:]), "*/")
			if end < 0 {
				return -1
			}
			i += 2 + end + 1
		}
	}
	return -1
}

func AppendString(path string, keyPath []string, value string, init []string) error {
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("{\n  "+nested(keyPath, slices.Concat(init, []string{value}))+"\n}\n"), 0o600)
	}
	if err != nil {
		return err
	}
	std, err := Standardize(src)
	if err != nil {
		return err
	}
	sc := &scanner{b: std}
	sp, depth, err := sc.value(keyPath)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var out []byte
	if depth == len(keyPath) {
		var cur []string
		if err := json.Unmarshal(std[sp.start:sp.end], &cur); err != nil {
			return fmt.Errorf("%s: %s is not a string array", path, strings.Join(keyPath, "."))
		}
		if slices.Contains(cur, value) {
			return nil
		}
		q, _ := json.Marshal(value)
		closeAt := sp.end - 1 // index of ']'
		last := closeAt - 1   // last non-space before ']' (comments are spaces in std)
		for last > sp.start && strings.IndexByte(" \t\r\n", std[last]) >= 0 {
			last--
		}
		// A trailing comma sits in the RAW gap between the last element and ']'.
		// Standardize blanked it to a space in std, so it is invisible there — and
		// `last` skips whitespace, so it never points at it either way. Look for the
		// comma in src (byte indexes match: Standardize preserves length), skipping
		// comments: a ',' inside `// a, b` is the user's text, not a trailing comma.
		gapComma := rawComma(src[last+1 : closeAt])
		switch {
		case len(cur) == 0:
			out = concat(src[:closeAt], q, src[closeAt:])
		case gapComma >= 0:
			// trailing comma: insert after the last element, drop the old comma,
			// keep whatever else follows it (whitespace, comments) up to ']'.
			commaAt := last + 1 + gapComma
			out = concat(src[:last+1], []byte(", "+string(q)), src[commaAt+1:])
		default:
			out = concat(src[:last+1], []byte(", "+string(q)), src[last+1:])
		}
	} else {
		if std[sp.start] != '{' {
			return fmt.Errorf("%s: %s is not an object", path, strings.Join(keyPath[:depth], "."))
		}
		ins := "\n  " + nested(keyPath[depth:], slices.Concat(init, []string{value}))
		if strings.TrimSpace(string(std[sp.start+1:sp.end-1])) != "" {
			ins += ","
		}
		out = concat(src[:sp.start+1], []byte(ins), src[sp.start+1:])
	}
	if err := os.WriteFile(path+".bak", src, 0o600); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

`value()` returns the deepest existing value on `keyPath` and how many keys matched. When a key is missing, that is the enclosing object, whose `end` is set when its `}` is reached.

- [ ] **Step 4: Run** — `go test ./internal/config/ -run AppendString -v` → PASS (fixtures match byte-for-byte).

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): comment-preserving JSONC array append for allow-always"
```

---

### Task 6: Prompt templates (slash commands from files)

**Files:**
- Create: `internal/skills/prompts.go`
- Test: `internal/skills/prompts_test.go`

**Interfaces:**
- Produces:
  - `type Prompt struct { Name, Description, ArgumentHint, Path, Body, Source string }`
  - `func LoadPrompts(dirs []Dir) []Prompt` — `<dir>/<name>.md`; frontmatter optional; first dir wins on a name collision; sorted.
  - `func ExpandPrompt(body, args string) string` — `$ARGUMENTS` and `$@` → full args; `$1…$9` → whitespace-split args (missing → ""); `$$` → `$`.

- [ ] **Step 1: Write failing tests**

```go
// internal/skills/prompts_test.go
package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrompts(t *testing.T) {
	proj, glob := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(proj, "review.md"), []byte("---\ndescription: review code\nargument-hint: <path>\n---\nReview $1 carefully. All: $ARGUMENTS"), 0o644)
	os.WriteFile(filepath.Join(glob, "review.md"), []byte("global"), 0o644)
	os.WriteFile(filepath.Join(glob, "plain.md"), []byte("No frontmatter $@"), 0o644)
	os.WriteFile(filepath.Join(glob, "skip.txt"), []byte("x"), 0o644)
	ps := LoadPrompts([]Dir{{proj, "project"}, {glob, "global"}})
	if len(ps) != 2 || ps[1].Name != "review" || ps[1].Description != "review code" || ps[0].Body != "No frontmatter $@" {
		t.Fatalf("%+v", ps)
	}
}

func TestExpandPrompt(t *testing.T) {
	got := ExpandPrompt("Review $1 and $2; all=$ARGUMENTS; at=$@; none=$3; cost $$5", "a.go b.go")
	if got != "Review a.go and b.go; all=a.go b.go; at=a.go b.go; none=; cost $5" {
		t.Fatal(got)
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/skills/prompts.go
package skills

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type Prompt struct {
	Name, Description, ArgumentHint, Path, Body, Source string
}

func LoadPrompts(dirs []Dir) []Prompt {
	seen := map[string]bool{}
	var out []Prompt
	for _, d := range dirs {
		ents, err := os.ReadDir(d.Path)
		if err != nil {
			continue
		}
		for _, e := range ents {
			name, ok := strings.CutSuffix(e.Name(), ".md")
			if !ok || e.IsDir() || seen[name] {
				continue
			}
			p := filepath.Join(d.Path, e.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			pr := Prompt{Name: name, Path: p, Source: d.Source, Body: string(data)}
			if fm, body, err := ParseFrontmatter(data); err == nil {
				pr.Description, pr.ArgumentHint, pr.Body = fm["description"], fm["argument-hint"], string(body)
			}
			seen[name] = true
			out = append(out, pr)
		}
	}
	slices.SortFunc(out, func(a, b Prompt) int { return strings.Compare(a.Name, b.Name) })
	return out
}

var promptVar = regexp.MustCompile(`\$(\$|ARGUMENTS|@|[1-9])`)

func ExpandPrompt(body, args string) string {
	fields := strings.Fields(args)
	return promptVar.ReplaceAllStringFunc(body, func(m string) string {
		switch m {
		case "$$":
			return "$"
		case "$ARGUMENTS", "$@":
			return args
		}
		i := int(m[1] - '1')
		if i < len(fields) {
			return fields[i]
		}
		return ""
	})
}
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/skills && git commit -m "feat(skills): prompt templates for slash commands"`

---

### Task 7: Status bar formatting

**Files:**
- Create: `internal/tui/format.go`, `internal/tui/status.go`
- Test: `internal/tui/format_test.go`, `internal/tui/status_test.go`

**Interfaces:**
- Produces:
  - `func FmtTokens(n int) string` — `<1000` → `"999"`; `<1M` → `"24k"` (`"1.2k"` below 10k); else `"1.3M"`.
  - `func FmtWindow(n int) string` — `1_000_000` → `"1M"`, `200_000` → `"200k"`, `131072` → `"128k"` (binary multiples of 1024 print in k).
  - `func AbbrevEffort(e llm.Effort) string` — `off min low med high xhigh max`.
  - `func AbbrevHome(path, home string) string`
  - `type StatusInfo struct { Cwd, Branch string; Git, Dirty bool; Model string; Effort llm.Effort; Window, Used, In, Out int; Cost float64; Sub bool; Transient string }`
  - `func RenderStatus(s StatusInfo, width int) string` — plain text (styling applied by the caller), never wider than `width`. Field order: `cwd · branch · model · effort · ctx W · P% · in/out · cost`. Shrink order when too wide: drop `cwd`, then `branch`, then `in/out`, then truncate the model with `…`.
  - `func GitBranch(dir string) (branch string, dirty, isGit bool)`

- [ ] **Step 1: Write failing tests**

```go
// internal/tui/format_test.go
package tui

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestFormatters(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1234: "1.2k", 24_000: "24k", 999_999: "999k", 1_300_000: "1.3M"}
	for n, want := range cases {
		if got := FmtTokens(n); got != want {
			t.Errorf("FmtTokens(%d)=%s want %s", n, got, want)
		}
	}
	for n, want := range map[int]string{1_000_000: "1M", 200_000: "200k", 131_072: "128k", 1_048_576: "1M", 32_768: "32k"} {
		if got := FmtWindow(n); got != want {
			t.Errorf("FmtWindow(%d)=%s want %s", n, got, want)
		}
	}
	if AbbrevEffort(llm.EffortMedium) != "med" || AbbrevEffort(llm.EffortMinimal) != "min" || AbbrevEffort(llm.EffortXHigh) != "xhigh" {
		t.Fatal("effort abbrev")
	}
	if AbbrevHome("/home/u/projects/moca", "/home/u") != "~/projects/moca" || AbbrevHome("/opt/x", "/home/u") != "/opt/x" {
		t.Fatal("home abbrev")
	}
}
```

```go
// internal/tui/status_test.go
package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

var full = StatusInfo{Cwd: "~/projects/moca", Branch: "main", Git: true, Dirty: true, Model: "opencode-go/glm-5.3-flash",
	Effort: llm.EffortMedium, Window: 1_000_000, Used: 230_000, In: 24_000, Out: 6_000, Cost: 0.0412}

func TestRenderStatusFull(t *testing.T) {
	got := RenderStatus(full, 200)
	want := "~/projects/moca · main* · opencode-go/glm-5.3-flash · med · ctx 1M · 23% · 24k/6k · $0.0412"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestRenderStatusVariants(t *testing.T) {
	s := full
	s.Git, s.Branch, s.Sub = false, "", true
	if got := RenderStatus(s, 200); !strings.Contains(got, " · - · ") || !strings.HasSuffix(got, "· sub") {
		t.Fatal(got)
	}
	s = full
	s.Transient = "retry 2/5 · 4s"
	if !strings.HasSuffix(RenderStatus(s, 200), "retry 2/5 · 4s") {
		t.Fatal("transient replaces cost")
	}
}

func TestRenderStatusNarrow(t *testing.T) {
	for _, w := range []int{100, 80, 60, 40, 20} {
		got := RenderStatus(full, w)
		if utf8.RuneCountInString(got) > w || strings.Contains(got, "\n") {
			t.Errorf("width %d: %q (%d)", w, got, utf8.RuneCountInString(got))
		}
	}
	if strings.Contains(RenderStatus(full, 80), "~/projects") {
		t.Fatal("cwd dropped first")
	}
	if !strings.Contains(RenderStatus(full, 60), "23%") {
		t.Fatal("ctx % survives at 60 cols")
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tui/format.go
package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

func trimZero(s string) string { return strings.TrimSuffix(s, ".0") }

func FmtTokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 10_000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1000)) + "k"
	case n < 1_000_000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "M"
	}
}

func FmtWindow(n int) string {
	switch {
	case n%1_048_576 == 0:
		return fmt.Sprintf("%dM", n/1_048_576)
	case n%1_000_000 == 0:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n%1024 == 0:
		return fmt.Sprintf("%dk", n/1024)
	default:
		return fmt.Sprintf("%dk", n/1000)
	}
}

var effortAbbrev = map[llm.Effort]string{llm.EffortOff: "off", llm.EffortMinimal: "min", llm.EffortLow: "low",
	llm.EffortMedium: "med", llm.EffortHigh: "high", llm.EffortXHigh: "xhigh", llm.EffortMax: "max"}

func AbbrevEffort(e llm.Effort) string { return effortAbbrev[e] }

func AbbrevHome(p, home string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + p[len(home):]
	}
	return p
}

func GitBranch(dir string) (string, bool, bool) {
	b, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", false, false
	}
	st, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	return strings.TrimSpace(string(b)), len(strings.TrimSpace(string(st))) > 0, true
}
```

```go
// internal/tui/status.go
package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

type StatusInfo struct {
	Cwd, Branch      string
	Git, Dirty       bool
	Model            string
	Effort           llm.Effort
	Window, Used     int
	In, Out          int
	Cost             float64
	Sub              bool
	Transient        string
}

func RenderStatus(s StatusInfo, width int) string {
	branch := "-"
	if s.Git {
		branch = s.Branch
		if s.Dirty {
			branch += "*"
		}
	}
	pct := 0
	if s.Window > 0 {
		pct = s.Used * 100 / s.Window
	}
	cost := fmt.Sprintf("$%.4f", s.Cost)
	if s.Sub {
		cost = "sub"
	}
	if s.Transient != "" {
		cost = s.Transient
	}
	type field struct {
		text string
		drop int // drop order: 1 first; 0 never
	}
	model := s.Model
	fields := []field{
		{s.Cwd, 1}, {branch, 2}, {model, 0}, {AbbrevEffort(s.Effort), 0},
		{"ctx " + FmtWindow(s.Window), 0}, {fmt.Sprintf("%d%%", pct), 0},
		{FmtTokens(s.In) + "/" + FmtTokens(s.Out), 3}, {cost, 0},
	}
	render := func() string {
		var parts []string
		for _, f := range fields {
			if f.text != "" {
				parts = append(parts, f.text)
			}
		}
		return strings.Join(parts, " · ")
	}
	for drop := 1; drop <= 3 && utf8.RuneCountInString(render()) > width; drop++ {
		for i := range fields {
			if fields[i].drop == drop {
				fields[i].text = ""
			}
		}
	}
	if over := utf8.RuneCountInString(render()) - width; over > 0 {
		r := []rune(model)
		keep := max(1, len(r)-over-1)
		fields[2].text = string(r[:keep]) + "…"
	}
	out := render()
	if r := []rune(out); len(r) > width {
		out = string(r[:max(0, width-1)]) + "…"
	}
	return out
}
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/tui && git commit -m "feat(tui): status bar formatting with width-aware field dropping"`

---

### Task 8: Input classification + slash-command parsing

**Files:**
- Create: `internal/tui/commands.go`
- Test: `internal/tui/commands_test.go`

**Interfaces:**
- Produces:
  - `type InputKind int` — `KindText, KindCommand, KindPrompt, KindShell, KindShellLocal`.
  - `type Parsed struct { Kind InputKind; Name, Args, Text string }`
  - `var BuiltinCommands = []string{"model","effort","hard","yolo","clear","compact","cost","undo","copy","show","help"}`
  - `func ParseInput(s string, prompts []skills.Prompt) (Parsed, error)`
  - Rules: `!!x` → ShellLocal; `!x` → Shell; `/name args` → builtin first, then a prompt template (`Text` = expanded body); unknown → error `unknown command /name (try /help)`; a leading `//` escapes to text (`/` + rest); anything else → Text.
  - `func HelpText(prompts []skills.Prompt) string`

- [ ] **Step 1: Write failing tests**

```go
// internal/tui/commands_test.go
package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestParseInput(t *testing.T) {
	prompts := []skills.Prompt{{Name: "review", Body: "Review $1"}, {Name: "model", Body: "shadowed"}}
	cases := []struct {
		in   string
		want Parsed
	}{
		{"hello", Parsed{Kind: KindText, Text: "hello"}},
		{"/model opencode-go/glm-5.3", Parsed{Kind: KindCommand, Name: "model", Args: "opencode-go/glm-5.3"}},
		{"/hard", Parsed{Kind: KindCommand, Name: "hard"}},
		{"/review a.go", Parsed{Kind: KindPrompt, Name: "review", Args: "a.go", Text: "Review a.go"}},
		{"!go test ./...", Parsed{Kind: KindShell, Text: "go test ./..."}},
		{"!!ls", Parsed{Kind: KindShellLocal, Text: "ls"}},
		{"//etc/hosts is odd", Parsed{Kind: KindText, Text: "/etc/hosts is odd"}},
		{"  /show 7 ", Parsed{Kind: KindCommand, Name: "show", Args: "7"}},
	}
	for _, c := range cases {
		got, err := ParseInput(c.in, prompts)
		if err != nil || got != c.want {
			t.Errorf("ParseInput(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	if _, err := ParseInput("/nope", prompts); err == nil || !strings.Contains(err.Error(), "/help") {
		t.Fatal(err)
	}
	if p, _ := ParseInput("/model", prompts); p.Kind != KindCommand {
		t.Fatal("built-ins win on collision")
	}
	if !strings.Contains(HelpText(prompts), "/review") {
		t.Fatal("help lists prompts")
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tui/commands.go
package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/skills"
)

type InputKind int

const (
	KindText InputKind = iota
	KindCommand
	KindPrompt
	KindShell
	KindShellLocal
)

type Parsed struct {
	Kind             InputKind
	Name, Args, Text string
}

var BuiltinCommands = []string{"model", "effort", "hard", "yolo", "clear", "compact", "cost", "undo", "copy", "show", "help"}

var builtinHelp = map[string]string{
	"model":   "[provider/model]  list models or switch (forfeits prompt cache)",
	"effort":  "[level]  show or set effort (off|minimal|low|medium|high|xhigh|max)",
	"hard":    "toggle modelHard + high effort",
	"yolo":    "toggle yolo mode: ALL permission checks off (between runs only)",
	"clear":   "start a new session (the old one stays resumable)",
	"compact": "summarize older context now",
	"cost":    "session token and cost detail",
	"undo":    "revert the last write/edit of this session",
	"copy":    "copy the last assistant message (OSC 52)",
	"show":    "<n>  open item #n in the pager",
	"help":    "this help",
}

func ParseInput(s string, prompts []skills.Prompt) (Parsed, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "!!"):
		return Parsed{Kind: KindShellLocal, Text: strings.TrimSpace(s[2:])}, nil
	case strings.HasPrefix(s, "!"):
		return Parsed{Kind: KindShell, Text: strings.TrimSpace(s[1:])}, nil
	case strings.HasPrefix(s, "//"):
		return Parsed{Kind: KindText, Text: s[1:]}, nil
	case strings.HasPrefix(s, "/"):
		name, args, _ := strings.Cut(s[1:], " ")
		args = strings.TrimSpace(args)
		if slices.Contains(BuiltinCommands, name) {
			return Parsed{Kind: KindCommand, Name: name, Args: args}, nil
		}
		for _, p := range prompts {
			if p.Name == name {
				return Parsed{Kind: KindPrompt, Name: name, Args: args, Text: skills.ExpandPrompt(p.Body, args)}, nil
			}
		}
		return Parsed{}, fmt.Errorf("unknown command /%s (try /help)", name)
	}
	return Parsed{Kind: KindText, Text: s}, nil
}

func HelpText(prompts []skills.Prompt) string {
	var sb strings.Builder
	for _, c := range BuiltinCommands {
		fmt.Fprintf(&sb, "/%-8s %s\n", c, builtinHelp[c])
	}
	for _, p := range prompts {
		if slices.Contains(BuiltinCommands, p.Name) {
			continue
		}
		fmt.Fprintf(&sb, "/%-8s %s %s\n", p.Name, p.ArgumentHint, p.Description)
	}
	sb.WriteString("!cmd     run cmd, output goes to the model   !!cmd  run cmd locally only\n")
	sb.WriteString("enter send · shift+enter newline · esc interrupt · ctrl+o pager · alt+p paste chips · ctrl+c×2 quit")
	return sb.String()
}
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/tui && git commit -m "feat(tui): input classification and slash-command parsing"`

---

### Task 9: Display sanitizing + input state machine

**Files:**
- Create: `internal/tui/sanitize.go`, `internal/tui/input.go`
- Test: `internal/tui/sanitize_test.go`, `internal/tui/input_test.go`

**Interfaces:**
- Produces:
  - `func Sanitize(s string) string` — replaces C0 controls except `\n`/`\t` with caret notation (`\x1b` → `^[`), and DEL → `^?`.
  - `type Input struct{…}` — pure state, no Bubble Tea types. **The buffer is exactly what the textarea shows.** A paste with more than 50 lines, or one containing control characters other than `\n`/`\t`, is stored aside, and the buffer gets a plain-text **chip marker** `[paste N lines #K]` instead. The textarea renders and edits chips like normal text: deleting the marker drops the paste, and a partially edited marker is sent as literal text. Control bytes therefore never reach the terminal, and the full content stays intact until send.
    - `func NewInput() *Input`
    - `func (in *Input) SetBuffer(s string)` / `Buffer() string` — sync with `textarea.Value()`.
    - `func (in *Input) Insert(s string)`, `Paste(s string)`
    - `func (in *Input) Text() string` — buffer with intact markers replaced by their content (what gets sent).
    - `func (in *Input) Display() string` — `Sanitize(buffer)`.
    - `func (in *Input) ToggleChips()` — `alt+p`: expand every intact marker to its content in the buffer (only for pastes without control chars), or collapse the expanded contents back to markers.
    - `func (in *Input) Submit() string` — `Text()`, pushed to history, then clears.
    - `func (in *Input) HistoryPrev() bool`, `HistoryNext() bool` — refuse (return false) when the buffer has a newline; the glue only calls them when the textarea cursor is on the first/last line.
    - `func (in *Input) CtrlC(now time.Time) (quit bool)` — non-empty → clear, false; empty → quit if the previous empty ctrl+c was ≤1s ago.
    - `func (in *Input) Prepend(texts []string)` — returned steering goes above the draft.
  - The DESIGN.md chip text is `[paste N lines]`; the `#K` suffix makes each marker unique when two pastes have the same length.

- [ ] **Step 1: Write failing tests**

```go
// internal/tui/sanitize_test.go
package tui

import "testing"

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\x1b[31mred\x1b[201~\tb\nc\x7f"); got != "a^[[31mred^[[201~\tb\nc^?" {
		t.Fatalf("%q", got)
	}
}
```

```go
// internal/tui/input_test.go
package tui

import (
	"strings"
	"testing"
	"time"
)

func TestPasteChipKeepsContent(t *testing.T) {
	in := NewInput()
	in.Insert("see: ")
	big := strings.Repeat("line\n", 120)
	in.Paste(big)
	if in.Text() != "see: "+big {
		t.Fatal("full content sent verbatim")
	}
	if in.Display() != "see: [paste 120 lines #1]" {
		t.Fatalf("%q", in.Display())
	}
	in.ToggleChips()
	if in.Buffer() != "see: "+big {
		t.Fatal("alt+p expands into the buffer")
	}
	in.ToggleChips()
	if in.Display() != "see: [paste 120 lines #1]" {
		t.Fatal("alt+p collapses again")
	}
	small := NewInput()
	small.Paste("a\nb\n")
	if small.Display() != "a\nb\n" {
		t.Fatal("small clean pastes go inline")
	}
}

func TestPasteWithControlBytesIsChipped(t *testing.T) {
	in := NewInput()
	in.Paste("x\x1b[201~y")
	if in.Text() != "x\x1b[201~y" || in.Display() != "[paste 1 line #1]" {
		t.Fatalf("text %q display %q", in.Text(), in.Display())
	}
	in.ToggleChips()
	if in.Display() != "[paste 1 line #1]" {
		t.Fatal("control-byte pastes never expand into the buffer")
	}
}

func TestEditedMarkerSentLiterally(t *testing.T) {
	in := NewInput()
	in.Paste(strings.Repeat("z\n", 60))
	in.SetBuffer(strings.Replace(in.Buffer(), "#1]", "#1", 1)) // user broke the marker
	if strings.Contains(in.Text(), "z\nz") {
		t.Fatal("a broken marker no longer expands")
	}
}

func TestHistory(t *testing.T) {
	in := NewInput()
	in.Insert("one")
	in.Submit()
	in.Insert("two")
	in.Submit()
	in.HistoryPrev()
	if in.Text() != "two" {
		t.Fatal(in.Text())
	}
	in.HistoryPrev()
	if in.Text() != "one" {
		t.Fatal(in.Text())
	}
	in.HistoryNext()
	in.HistoryNext()
	if in.Text() != "" {
		t.Fatal("past the newest → the saved draft (empty)")
	}
	in.Insert("multi\nline")
	if in.HistoryPrev() {
		t.Fatal("multi-line buffer: textarea handles ↑")
	}
}

func TestCtrlC(t *testing.T) {
	in := NewInput()
	now := time.Now()
	in.Insert("draft")
	if in.CtrlC(now) || in.Text() != "" {
		t.Fatal("non-empty: clear")
	}
	if in.CtrlC(now.Add(100 * time.Millisecond)) {
		t.Fatal("first empty ctrl+c arms")
	}
	if !in.CtrlC(now.Add(600 * time.Millisecond)) {
		t.Fatal("second within 1s quits")
	}
	in2 := NewInput()
	in2.CtrlC(now)
	if in2.CtrlC(now.Add(1500 * time.Millisecond)) {
		t.Fatal("outside 1s re-arms")
	}
}

func TestPrependSteering(t *testing.T) {
	in := NewInput()
	in.Insert("draft")
	in.Prepend([]string{"s1", "s2"})
	if in.Text() != "s1\ns2\ndraft" {
		t.Fatal(in.Text())
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/tui/ -run 'Sanitize|Paste|Marker|History|CtrlC|Prepend'` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tui/sanitize.go
package tui

import "strings"

// Sanitize neutralizes control characters so pasted or tool-produced bytes
// are displayed, never interpreted by the terminal.
func Sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			sb.WriteRune(r)
		case r < 0x20:
			sb.WriteByte('^')
			sb.WriteRune(r + '@')
		case r == 0x7f:
			sb.WriteString("^?")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func hasControl(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return true
		}
	}
	return false
}
```

```go
// internal/tui/input.go
package tui

import (
	"fmt"
	"strings"
	"time"
)

const chipThreshold = 50

type paste struct {
	marker, content string
	unsafe          bool // contains control bytes: never expanded into the buffer
	expanded        bool
}

type Input struct {
	buf         string
	pastes      []*paste
	history     []string
	hpos        int
	draft       string
	lastEmptyCC time.Time
}

func NewInput() *Input { return &Input{} }

func (in *Input) SetBuffer(s string) { in.buf = s }
func (in *Input) Buffer() string     { return in.buf }
func (in *Input) Insert(s string)    { in.buf += s }

func lineCount(s string) int { return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1 }

func (in *Input) Paste(s string) {
	unsafe := hasControl(s)
	if !unsafe && lineCount(s) <= chipThreshold {
		in.buf += s
		return
	}
	n := lineCount(s)
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	p := &paste{marker: fmt.Sprintf("[paste %d %s #%d]", n, unit, len(in.pastes)+1), content: s, unsafe: unsafe}
	in.pastes = append(in.pastes, p)
	in.buf += p.marker
}

func (in *Input) Text() string {
	t := in.buf
	for _, p := range in.pastes {
		if !p.expanded {
			t = strings.Replace(t, p.marker, p.content, 1)
		}
	}
	return t
}

func (in *Input) Display() string { return Sanitize(in.buf) }

func (in *Input) ToggleChips() {
	anyCollapsed := false
	for _, p := range in.pastes {
		if !p.expanded && !p.unsafe && strings.Contains(in.buf, p.marker) {
			anyCollapsed = true
		}
	}
	for _, p := range in.pastes {
		switch {
		case p.unsafe:
		case anyCollapsed && !p.expanded && strings.Contains(in.buf, p.marker):
			in.buf, p.expanded = strings.Replace(in.buf, p.marker, p.content, 1), true
		case !anyCollapsed && p.expanded && strings.Contains(in.buf, p.content):
			in.buf, p.expanded = strings.Replace(in.buf, p.content, p.marker, 1), false
		}
	}
}

func (in *Input) clear() { in.buf, in.pastes = "", nil }

func (in *Input) Submit() string {
	t := in.Text()
	if strings.TrimSpace(t) != "" {
		in.history = append(in.history, t)
	}
	in.hpos, in.draft = len(in.history), ""
	in.clear()
	return t
}

func (in *Input) HistoryPrev() bool {
	if strings.Contains(in.buf, "\n") || in.hpos == 0 {
		return false
	}
	if in.hpos == len(in.history) {
		in.draft = in.Text()
	}
	in.hpos--
	in.clear()
	in.buf = in.history[in.hpos]
	return true
}

func (in *Input) HistoryNext() bool {
	if strings.Contains(in.buf, "\n") || in.hpos >= len(in.history) {
		return false
	}
	in.hpos++
	in.clear()
	if in.hpos == len(in.history) {
		in.buf = in.draft
	} else {
		in.buf = in.history[in.hpos]
	}
	return true
}

func (in *Input) CtrlC(now time.Time) bool {
	if in.Text() != "" {
		in.clear()
		in.lastEmptyCC = time.Time{}
		return false
	}
	if !in.lastEmptyCC.IsZero() && now.Sub(in.lastEmptyCC) <= time.Second {
		return true
	}
	in.lastEmptyCC = now
	return false
}

func (in *Input) Prepend(texts []string) {
	if len(texts) == 0 {
		return
	}
	pre := strings.Join(texts, "\n")
	if in.buf != "" {
		pre += "\n"
	}
	in.buf = pre + in.buf
}
```

History entries are stored expanded (`Text()`), so recalling an old multi-hundred-line paste puts its content into the buffer. That is acceptable, because recall is explicit.

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/tui && git commit -m "feat(tui): input state — paste chips, history, ctrl+c, steering return, sanitizing"`

---

### Task 10: Numbered items

**Files:**
- Create: `internal/tui/items.go`
- Test: `internal/tui/items_test.go`

**Interfaces:**
- Produces:
  - `type Item struct { N int; Kind string /*"tool"|"thinking"*/; Line, Body string }`
  - `type Items struct{…}`; `func (s *Items) AddTool(call llm.ToolCall, r tools.Result) Item`; `func (s *Items) AddThinking(text string) Item`; `func (s *Items) Get(n int) (Item, bool)`; `func (s *Items) Last() (Item, bool)`.
  - Lines: tool ok → `▸ #7 edit main.go [+3 −1]` (`Summary` if set, else first arg-ish line); tool error → `✗ #7 shell go test [exit 1]` (Summary) or `✗ #7 read <first line of error>`; thinking → `⋯ #6 thinking N lines`.
  - Body: tool → `Detail` if set, else `Content`; thinking → the text. All bodies are sanitized.
  - Numbering is shared across both kinds, starting at 1 per TUI session (it resets on `/clear`).

- [ ] **Step 1: Write failing test**

```go
// internal/tui/items_test.go
package tui

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func TestItems(t *testing.T) {
	var s Items
	th := s.AddThinking("a\nb\nc")
	if th.N != 1 || th.Line != "⋯ #1 thinking 3 lines" {
		t.Fatal(th.Line)
	}
	ok := s.AddTool(llm.ToolCall{Name: "edit"}, tools.Result{Summary: "main.go [+3 −1]", Detail: "DIFF", Content: "c"})
	if ok.Line != "▸ #2 edit main.go [+3 −1]" || ok.Body != "DIFF" {
		t.Fatal(ok)
	}
	bad := s.AddTool(llm.ToolCall{Name: "read"}, tools.Result{IsError: true, Content: "no such file\nmore"})
	if bad.Line != "✗ #3 read no such file" || bad.Body != "no such file\nmore" {
		t.Fatal(bad)
	}
	if last, _ := s.Last(); last.N != 3 {
		t.Fatal("last")
	}
	if got, ok := s.Get(2); !ok || got.Kind != "tool" {
		t.Fatal("get")
	}
	if _, ok := s.Get(9); ok {
		t.Fatal("missing")
	}
	pct := s.AddTool(llm.ToolCall{Name: "shell"}, tools.Result{Summary: "echo 100% [exit 0]"})
	if pct.Line != "▸ #4 shell echo 100% [exit 0]" {
		t.Fatal("summaries are never format strings:", pct.Line)
	}
}
```

- [ ] **Step 2: Run** — FAIL.

- [ ] **Step 3: Implement**

```go
// internal/tui/items.go
package tui

import (
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

type Item struct {
	N          int
	Kind       string
	Line, Body string
}

type Items struct{ list []Item }

// add numbers the item; mark is "▸", "✗" or "⋯". Text is never used as a
// format string (summaries may contain %).
func (s *Items) add(kind, mark, text, body string) Item {
	n := len(s.list) + 1
	it := Item{N: n, Kind: kind, Line: fmt.Sprintf("%s #%d %s", mark, n, Sanitize(text)), Body: Sanitize(body)}
	s.list = append(s.list, it)
	return it
}

func (s *Items) AddThinking(text string) Item {
	n := strings.Count(strings.TrimRight(text, "\n"), "\n") + 1
	return s.add("thinking", "⋯", fmt.Sprintf("thinking %d lines", n), text)
}

func (s *Items) AddTool(call llm.ToolCall, r tools.Result) Item {
	body := r.Detail
	if body == "" {
		body = r.Content
	}
	if r.IsError {
		desc := r.Summary
		if desc == "" {
			desc, _, _ = strings.Cut(r.Content, "\n")
		}
		return s.add("tool", "✗", call.Name+" "+desc, body)
	}
	return s.add("tool", "▸", call.Name+" "+r.Summary, body)
}

func (s *Items) Get(n int) (Item, bool) {
	if n < 1 || n > len(s.list) {
		return Item{}, false
	}
	return s.list[n-1], true
}

func (s *Items) Last() (Item, bool) { return s.Get(len(s.list)) }
```

- [ ] **Step 4: Run** — PASS. **Step 5: Commit** — `git add internal/tui && git commit -m "feat(tui): numbered collapsible items"`

---

### Task 11: Bubble Tea glue — app, bridge, pager, trust prompt

**Files:**
- Create: `internal/tui/bridge.go`, `internal/tui/pager.go`, `internal/tui/trust.go`, `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: Tasks 1–10, `agent.*`, `tools.RunShell/ShellEnv/Truncate`, `config.AppendString/ConfigFile/DefaultShellAllow`, `skills.LoadPrompts`.
- Produces:
  - `type AppOptions struct { Start agent.StartOptions; ConfigPath string; Prompts []skills.Prompt; Home string }`
  - `func Run(ctx context.Context, o AppOptions) error` — creates the program in inline mode and runs it.
  - `func RunTrustPrompt(dir string) (bool, error)` — small standalone program: `Trust project resources in <dir>? (.moca/, AGENTS.md) [y/N]`.
  - Internal messages: `agentEventMsg{agent.Event}`, `runDoneMsg{outcome agent.Outcome; err error}`, `approvalMsg{q tools.Question; reply chan tools.Answer}`, `branchMsg{branch string; dirty, git bool}`, `shellDoneMsg{local bool; cmd string; out tools.ShellOutput; err error}`.
  - `bridge.go`: `func newAsker(send func(tea.Msg)) tools.Asker` blocks on `reply` or `ctx.Done()` (→ `Deny`).
  - Model state: `agent *agent.Agent`, `input *Input`, `ta textarea.Model`, `items Items`, `live strings.Builder` (streaming text not yet committed), `thinking strings.Builder`, `running bool`, `cancel context.CancelFunc`, `approval *approvalMsg`, `pager *pagerModel`, `status StatusInfo`, `width int`, `kbdEnhanced bool`, `hintShown bool`, `lastAssistant string`.
- Behaviour (each line is one `Update` branch):
  1. `KeyPressMsg "enter"` (no approval/pager): parse input. Text → if running, `agent.Steer` and print `↳ queued: …`; else start a run. Command → `runCommand`. Prompt → like text with the expanded body. Shell/ShellLocal → `tea.Cmd` running `tools.RunShell` in the workdir with `ShellEnv`.
  2. `"shift+enter"`, `"alt+enter"`, `"ctrl+j"` → `ta.InsertString("\n")`.
  3. `"esc"`: if an approval is shown → Deny. Else if running → `cancel()`.
  4. `"ctrl+c"` → `input.CtrlC(time.Now())` → `tea.Quit` if true.
  5. `"ctrl+o"` → open the pager on `items.Last()`.
  6. `"alt+p"` → `input.ToggleChips()`.
  7. `"up"`/`"down"` when the textarea cursor is on the first/last line → history.
  8. `PasteMsg` → `input.Paste(content)`; sync the textarea value.
  9. `KeyboardEnhancementsMsg` → `kbdEnhanced = true`. On the first frame, `tea.Tick(500ms)` → if not enhanced and the hint isn't shown yet: print `hint: this terminal can't report shift+enter; use alt+enter or ctrl+j for a newline` once.
  10. `agentEventMsg`:
      - `TextDelta`: append to `live`; commit every completed line with `tea.Println` (keep the partial line live).
      - `ThinkingDelta`: append to `thinking` (not shown; the live region shows `⋯ thinking…`).
      - `StreamReset`: drop `live` and `thinking`, print `[stream interrupted — retrying]`.
      - `TurnEnd`: flush the `live` remainder; if `thinking` is non-empty, `items.AddThinking` + `Println(line)`; set `lastAssistant`; refresh status; run `GitBranch` async → `branchMsg`.
      - `ToolStart`: live shows `… <name>`.
      - `ToolEnd`: `items.AddTool` + `Println(line)`; refresh status.
      - `Retry`: `status.Transient = "retry n/max · wait"` (cleared at the next `TurnEnd`).
      - `SteeringApplied`: print `↳ sent: …`.
      - `YoloChanged`: refresh status; print `yolo mode on: all permission checks are off` (red) or `yolo mode off: permission checks restored`.
  11. `runDoneMsg`: `running = false`. On a cancelled err: `input.Prepend(agent.TakeSteering())`, print `[interrupted]`. On another err: print `error: …`. Refresh status and branch.
  12. `approvalMsg`: store it and show the live prompt. Shell: `allow '<subject>'? [a] once  [A] always  [d] deny` (no `[A]` when `!CanAlways`). Keys `a`/`A`/`d`/`esc`. On `A` for shell → `config.AppendString(cfgPath, {"shell","allow"}, subject, DefaultShellAllow)`; on failure print the error but still allow.
  13. `shellDoneMsg`: Print `$ cmd` + sanitized output (`Truncate` 30K) + `[exit N]`. For `!` (not local), also `agent.AddNote("$ cmd\n" + output + "\n[exit N]")`.
  14. Pager open → all keys go to the pager (`q`/`esc` closes; `↑↓ pgup pgdn g G` scroll). The view sets `AltScreen = true` while it is open.
  15. `View()`: `live` (sanitized) + approval prompt + textarea (showing `input.Display()`) + `RenderStatus(status, width)` styled dim. Request keyboard enhancements on the view (per Task 1's API notes).
- Commands (`runCommand`):
  - `/model` (no args): print `Models()` qualified ids, marking the current one. With args: `agent.SetModel(arg, "")` → print `switched to X · effort Y (prompt cache forfeited)`.
  - `/effort` (no args): print current + supported levels. With args: `ParseEffort` → `SetEffort` → print the effective level (note when clamped).
  - `/hard` → `ToggleHard` → print `hard mode on: X · high` / `hard mode off: back to X · Y`, plus the cache note.
  - `/yolo` → while running: print `finish or interrupt the run first (esc)`. Otherwise `agent.SetYolo(!agent.Yolo())`; the `YoloChanged` event prints the line. (Never during a run: `SetYolo` swaps `Env` checkers and is not synchronized with the running agent.)
  - `/clear` (refused while running) → close the session, `agent.Start(opts)` again, reset items, print `new session <id8> (previous stays resumable)`.
  - `/compact` → phase 3 prints `compaction lands in phase 4`; phase 4 replaces it.
  - `/cost` → print in/out/cacheRead/cacheWrite and cost.
  - `/undo` → `agent.Undo()` → print the message or error.
  - `/copy` → `tea.SetClipboard(lastAssistant)` + print `copied N chars`.
  - `/show n` → pager.
  - `/help` → `HelpText`.

- [ ] **Step 1: Write failing glue tests (drive `Update` directly; no terminal)**

```go
// internal/tui/app_test.go
package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// newTestModel builds a model without an agent (agent-dependent commands are
// tested in agent's own tests).
func newTestModel() *model {
	m := newModel(AppOptions{Home: "/home/u"}, nil)
	m.width = 120
	return m
}

func key(s string) tea.Msg { return keyMsg(s) } // helper from app.go test hooks

func TestStreamingCommitsCompletedLines(t *testing.T) {
	m := newTestModel()
	m.running = true
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "line one\nline t"}})
	if cmd == nil || !strings.Contains(m.live.String(), "line t") || strings.Contains(m.live.String(), "line one") {
		t.Fatalf("completed line printed, partial kept live: %q", m.live.String())
	}
}

func TestApprovalPreservesDraft(t *testing.T) {
	m := newTestModel()
	m.input.Insert("half-typed")
	m.syncTextarea()
	reply := make(chan tools.Answer, 1)
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "python", CanAlways: true}, reply: reply})
	m.Update(key("a"))
	if got := <-reply; got != tools.AllowOnce {
		t.Fatal(got)
	}
	if m.input.Text() != "half-typed" {
		t.Fatal("draft untouched by approval keys")
	}
	m.Update(approvalMsg{q: tools.Question{Kind: "shell", Subject: "rm"}, reply: reply})
	m.Update(key("A"))
	select {
	case <-reply:
		t.Fatal("A is not offered for ask-every-time commands")
	default:
	}
	m.Update(key("esc"))
	if got := <-reply; got != tools.Deny {
		t.Fatal("esc on prompt = deny")
	}
}

func TestThinkingCollapsedOnTurnEnd(t *testing.T) {
	m := newTestModel()
	m.Update(agentEventMsg{agent.ThinkingDelta{Text: "secret\nreasoning"}})
	if strings.Contains(m.View().Content, "secret") {
		t.Fatal("thinking never shown expanded outside the pager")
	}
	m.Update(agentEventMsg{agent.TurnEnd{Message: llm.Message{}}})
	it, ok := m.items.Last()
	if !ok || it.Kind != "thinking" || it.Line != "⋯ #1 thinking 2 lines" {
		t.Fatal(it)
	}
}

func TestPasteDoesNotSend(t *testing.T) {
	m := newTestModel()
	m.Update(tea.PasteMsg{Content: strings.Repeat("x\n", 80)})
	if m.running || m.input.Display() != "[paste 80 lines #1]" {
		t.Fatal(m.input.Display())
	}
	_ = context.Background
}
```

`newModel(opts, agent)`, `syncTextarea()` and the test hook `keyMsg(s string) tea.KeyPressMsg` (builds a key-press message whose `String()` is `s`) live in `app.go`. Build `keyMsg` from the struct fields that Task 1 recorded.

- [ ] **Step 2: Run** — `go test ./internal/tui/` → FAIL.

- [ ] **Step 3: Implement `bridge.go`**

```go
// internal/tui/bridge.go
package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/tools"
)

type agentEventMsg struct{ e agent.Event }
type runDoneMsg struct {
	out agent.Outcome
	err error
}
type approvalMsg struct {
	q     tools.Question
	reply chan tools.Answer
}
type branchMsg struct {
	branch     string
	dirty, git bool
}
type shellDoneMsg struct {
	local bool
	cmd   string
	out   tools.ShellOutput
	err   error
}

func newAsker(send func(tea.Msg)) tools.Asker {
	return func(ctx context.Context, q tools.Question) tools.Answer {
		reply := make(chan tools.Answer, 1)
		send(approvalMsg{q: q, reply: reply})
		select {
		case a := <-reply:
			return a
		case <-ctx.Done():
			return tools.Deny
		}
	}
}
```

- [ ] **Step 4: Implement `pager.go` and `trust.go`**

```go
// internal/tui/pager.go
package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
)

type pagerModel struct {
	vp    viewport.Model
	title string
}

func newPager(it Item, w, h int) *pagerModel {
	vp := viewport.New()
	vp.SetWidth(w)
	vp.SetHeight(max(3, h-1))
	vp.SetContent(it.Body)
	return &pagerModel{vp: vp, title: it.Line}
}

// update returns false when the pager should close.
func (p *pagerModel) update(msg tea.Msg) (bool, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "q", "esc":
			return false, nil
		case "g":
			p.vp.GotoTop()
			return true, nil
		case "G":
			p.vp.GotoBottom()
			return true, nil
		}
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return true, cmd
}

func (p *pagerModel) view() string { return p.title + "  (q to close)\n" + p.vp.View() }
```

`viewport.New()`'s v2 signature and the `SetWidth`/`SetHeight` names come from Task 1's API notes. Adjust them there if they differ.

```go
// internal/tui/trust.go
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

type trustModel struct {
	dir    string
	answer bool
	done   bool
}

func (m *trustModel) Init() tea.Cmd { return nil }

func (m *trustModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "y", "Y":
			m.answer, m.done = true, true
			return m, tea.Quit
		case "n", "N", "enter", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *trustModel) View() tea.View {
	return tea.NewView(fmt.Sprintf("Trust project resources in %s? (.moca/, AGENTS.md/CLAUDE.md can steer the agent) [y/N] ", m.dir))
}

// RunTrustPrompt asks once; the caller saves the decision.
func RunTrustPrompt(dir string) (bool, error) {
	m := &trustModel{dir: dir}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		return false, err
	}
	return m.answer, nil
}
```

- [ ] **Step 5: Implement `app.go`**

Write `app.go` per the behaviour list above. Its skeleton:

```go
// internal/tui/app.go
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

type AppOptions struct {
	Start      agent.StartOptions
	ConfigPath string
	Prompts    []skills.Prompt
	Home       string
}

type model struct {
	opts          AppOptions
	agent         *agent.Agent
	send          func(tea.Msg)
	input         *Input
	ta            textarea.Model
	items         Items
	live          strings.Builder
	thinking      strings.Builder
	running       bool
	cancel        context.CancelFunc
	approval      *approvalMsg
	pager         *pagerModel
	status        StatusInfo
	width, height int
	kbdEnhanced   bool
	hintShown     bool
	lastAssistant string
}

var dim = lipgloss.NewStyle().Faint(true)

func newModel(o AppOptions, a *agent.Agent) *model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	ta.SetHeight(3)
	ta.Focus()
	m := &model{opts: o, agent: a, input: NewInput(), ta: ta}
	m.refreshStatus()
	return m
}

func (m *model) syncTextarea()    { m.ta.SetValue(m.input.Buffer()) }
func (m *model) pullTextarea()    { m.input.SetBuffer(m.ta.Value()) }
func println(s string) tea.Cmd    { return tea.Println(Sanitize(s)) }

func (m *model) refreshStatus() {
	if m.agent == nil {
		return
	}
	st := m.agent.Status()
	m.status.Cwd = AbbrevHome(m.opts.Start.Workdir, m.opts.Home)
	m.status.Model, m.status.Effort = st.Model.Qualified(), st.Effort
	m.status.Window, m.status.Used = st.Window, st.ContextTokens
	m.status.In = st.Usage.Input + st.Usage.CacheRead + st.Usage.CacheWrite
	m.status.Out, m.status.Cost, m.status.Sub = st.Usage.Output, st.Cost, st.Sub
}

func (m *model) branchCmd() tea.Cmd {
	dir := m.opts.Start.Workdir
	return func() tea.Msg { b, d, g := GitBranch(dir); return branchMsg{b, d, g} }
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(m.branchCmd(), tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return hintCheckMsg{} }))
}

type hintCheckMsg struct{}

func (m *model) startRun(text string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.running, m.cancel = true, cancel
	a := m.agent
	return tea.Sequence(println("› "+text), func() tea.Msg {
		out, err := a.Run(ctx, text)
		return runDoneMsg{out, err}
	})
}

// Update, handleKey, handleAgent, runCommand, View: implement per the
// numbered behaviour list in this task's Interfaces block.
```

Implement `Update` as a type switch over: `tea.WindowSizeMsg` (set width/height, `ta.SetWidth`), `tea.KeyboardEnhancementsMsg`, `hintCheckMsg`, `tea.PasteMsg`, `tea.KeyPressMsg` (→ `handleKey`), `agentEventMsg` (→ `handleAgent`), `runDoneMsg`, `approvalMsg`, `branchMsg`, `shellDoneMsg`. Forward unhandled key presses to `m.ta.Update(msg)` and then `m.pullTextarea()`.

`View()`:

```go
func (m *model) View() tea.View {
	if m.pager != nil {
		v := tea.NewView(m.pager.view())
		v.AltScreen = true
		return v
	}
	var sb strings.Builder
	if m.live.Len() > 0 {
		sb.WriteString(Sanitize(m.live.String()) + "\n")
	}
	if m.thinking.Len() > 0 && m.running {
		sb.WriteString(dim.Render("⋯ thinking…") + "\n")
	}
	if m.approval != nil {
		sb.WriteString(approvalPrompt(m.approval.q) + "\n")
	}
	sb.WriteString(m.ta.View() + "\n")
	sb.WriteString(m.statusLine())
	v := tea.NewView(sb.String())
	// request keyboard enhancements here (exact field from Task 1 notes)
	return v
}

var red = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)

// statusLine renders the bar; in yolo mode a red YOLO field leads it and is
// never dropped — the rest gets the remaining width.
func (m *model) statusLine() string {
	w := max(20, m.width)
	if m.agent != nil && m.agent.Yolo() {
		return red.Render("YOLO") + dim.Render(" · "+RenderStatus(m.status, w-7))
	}
	return dim.Render(RenderStatus(m.status, w))
}

func approvalPrompt(q tools.Question) string {
	if q.CanAlways {
		return fmt.Sprintf("allow `%s`?  [a] once  [A] always  [d] deny   — %s", q.Subject, firstLineOf(q.Detail))
	}
	return fmt.Sprintf("allow `%s` (asks every time)?  [a] once  [d] deny   — %s", q.Subject, firstLineOf(q.Detail))
}

func firstLineOf(s string) string { l, _, _ := strings.Cut(s, "\n"); return l }
```

The textarea value **is** `input.Buffer()`; chips are plain-text markers, so the textarea renders them with no special hook. After every pure `Input` operation call `m.syncTextarea()`, and before every one call `m.pullTextarea()`.

`Run`:

```go
func Run(ctx context.Context, o AppOptions) error {
	var p *tea.Program
	m := newModel(o, nil)
	o.Start.Emit = func(e agent.Event) { p.Send(agentEventMsg{e}) }
	o.Start.Ask = newAsker(func(msg tea.Msg) { p.Send(msg) })
	a, err := agent.Start(o.Start)
	if err != nil {
		return err
	}
	m.agent, m.opts = a, o
	m.refreshStatus()
	p = tea.NewProgram(m, tea.WithContext(ctx))
	_, err = p.Run()
	a.Session().Close()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}
```

`p` is captured by the closures before assignment. Events can't fire before `p.Run()` starts, because the agent only runs on user input. Note this in a comment.

- [ ] **Step 6: Run** — `go test ./internal/tui/ -race -v` → PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui
git commit -m "feat(tui): Bubble Tea app — inline scrollback, streaming commit, approvals, pager, commands"
```

---

### Task 12: CLI — TUI entry + trust decision

**Files:**
- Create: `cmd/moca/tui.go`
- Modify: `cmd/moca/main.go`

**Interfaces:**
- Produces: `func runTUI(ctx context.Context, o Options, cfg config.Config, cfgPath string, stderr io.Writer) int`.
  - Trust: `--approve`/`--no-approve` if given; else the saved decision from `permissions.LoadTrust(DataDir()/trust.json)`; else, when `.moca/`, `AGENTS.md` or `CLAUDE.md` exists, `tui.RunTrustPrompt` → `Set`. Otherwise untrusted, with nothing to load.
  - Prompts: `skills.LoadPrompts([]skills.Dir{{workdir/.moca/prompts (trusted only)}, {ConfigDir()/prompts}})`.
  - Exit codes: `0` normal quit; `1` runtime error; `2` config error.

- [ ] **Step 1: Implement**

```go
// cmd/moca/tui.go
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tui"
)

func hasProjectResources(dir string) bool {
	for _, n := range []string{".moca", "AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

func decideTrust(o Options, wd string, yolo bool) (bool, error) {
	if o.Approve != nil {
		return *o.Approve, nil
	}
	if yolo {
		return true, nil // §7.5: yolo trusts project resources unless --no-approve is explicit
	}
	ts, err := permissions.LoadTrust(filepath.Join(config.DataDir(), "trust.json"))
	if err != nil {
		return false, err
	}
	if v, known := ts.Lookup(wd); known {
		return v, nil
	}
	if !hasProjectResources(wd) {
		return false, nil
	}
	ans, err := tui.RunTrustPrompt(wd)
	if err != nil {
		return false, err
	}
	return ans, ts.Set(wd, ans)
}

func runTUI(ctx context.Context, o Options, cfg config.Config, cfgPath string, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	yolo := o.YoloOn(cfg)
	trusted, err := decideTrust(o, wd, yolo)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	var pdirs []skills.Dir
	if trusted {
		pdirs = append(pdirs, skills.Dir{Path: filepath.Join(wd, ".moca", "prompts"), Source: "project"})
	}
	pdirs = append(pdirs, skills.Dir{Path: filepath.Join(config.ConfigDir(), "prompts"), Source: "global"})
	home, _ := os.UserHomeDir()
	err = tui.Run(ctx, tui.AppOptions{
		Start:      agent.StartOptions{Config: cfg, Workdir: wd, Effort: o.Effort, Trusted: trusted, Yolo: yolo, Slug: "tui"},
		ConfigPath: cfgPath, Prompts: skills.LoadPrompts(pdirs), Home: home,
	})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	return exitOK
}
```

The slug: TUI sessions start before the first prompt exists, so use `"tui"`. Phase 4's `--continue` finds sessions by the header's workdir, not by slug.

In `main.go`, replace the `!o.OneShot` early exit with `return runTUI(ctx, o, cfg, path, stderr)`.

- [ ] **Step 2: Run** — `go build ./... && go test ./... -race` → PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/moca
git commit -m "feat(cli): moca opens the TUI with trust decision and prompt templates"
```

---

### Task 13: Phase gate (manual, in real terminals)

Build: `go build -o bin/moca ./cmd/moca`. Run each check and tick it only when observed.

- [ ] **Full session**: in a scratch repo, ask moca to fix a failing test. Streaming text appears and commits to scrollback. Tool calls show as `▸ #n …` and thinking as `⋯ #n thinking N lines`. `ctrl+o` opens the latest item in the pager and `q` returns. `/show 2` works.
- [ ] **Status bar from the first frame**: shows cwd · branch · model · effort · ctx · % · tokens · cost, below the input. Ask the agent to `git checkout -b gate-test` via shell; the bar shows the new branch at the turn end.
- [ ] **`/model` cross-provider**: start on `anthropic/<claude>` with effort `high` and run a turn with thinking + a tool call. Then `/model opencode-go/glm-5.3-flash` and continue the conversation: no 400, prior reasoning appears as `[prior reasoning]` text in the request (verify in the session file + a debug dump of one request body). `/effort low` updates the bar. `/hard` switches to `modelHard · high`, and `/hard` again restores.
- [ ] **Missing key**: `unset OPENAI_API_KEY` and `/model openai/<x>` → error naming `OPENAI_API_KEY`; the model is unchanged.
- [ ] **Multi-line paste**: paste 120 lines → `[paste 120 lines]` chip, not sent. `alt+p` expands. `enter` sends; the session file has all 120 lines intact.
- [ ] **shift+enter**: in kitty/WezTerm/Ghostty, `shift+enter` inserts a newline. In a terminal without enhanced keyboard reporting (e.g. plain xterm or tmux without `extended-keys`), the one-time hint appears and `alt+enter`/`ctrl+j` work.
- [ ] **Steering**: during a multi-tool run, type `also update the README` + enter → `↳ queued`. Then `↳ sent` appears after the current tool results; the session file shows the user message right after the `tool_result` entries.
- [ ] **esc**: interrupt a run mid-tool → `[interrupted]`, queued steering returns to the editor, and the session file ends with an `aborted by user` tool_result.
- [ ] **`/undo`**: after an `edit` of an untracked or dirty file, `/undo` restores it. On a clean tracked file it prints the `git restore` hint.
- [ ] **Approvals**: ask for `python3 -c 'print(1)'` → prompt. `A` → it runs, and `~/.config/moca/config.jsonc` gains `"python3"` in `shell.allow` with comments intact. Ask for `rm tmpfile` → prompt without `[A]`.
- [ ] **Trust**: open moca in a fresh clone that has an `AGENTS.md` → trust prompt. `n` → the AGENTS.md content is not in the session's stored system prompt. The second launch doesn't ask.
- [ ] **Yolo**: start with `moca --yolo` → the bar leads with red `YOLO`, and `python3 -c 'print(1)'` runs with no prompt. `/yolo` → `yolo mode off`; the same request prompts again. `/yolo` during a run → refused with the esc hint. At 60 columns `YOLO` is still shown. With `"yolo": true` in config, `moca --no-yolo` starts with checks on.
- [ ] **`!` / `!!`**: `!git status` → output shown, and the next request includes it as a user message. `!!ls` → shown only, not in the session file.
- [ ] **`/copy`**: over SSH into a terminal that supports OSC 52, paste the clipboard locally and it matches the last answer.

- [ ] **Commit**: update the README status (`phase 3 done — TUI`) and commit `docs: phase 3 gate passed`.

---

## Implementation notes (added after implementation + gate runs)

Deviations from the task snippets, all deliberate; the plan's intent is kept.

1. **Adapter thinking-replay comparison is qualified (`m.Qualified()`)** — the plan says "adapters set the bare id; the agent owns qualification", but its `replaysVerbatim(c, req.Model)` still compared the *bare* id while `turn()` stamps the *qualified* one, which would degrade every same-model thinking block to `[prior reasoning]` on the next request. The agent still stamps qualified at persist time (belt and braces); the adapters compare against their own qualified model. Pinned by `TestAnthropicForeignThinking` / `TestResponsesForeignThinking` ("same bare id, other provider" must not replay) and `TestTransformSameBareIDDifferentProvider`.
2. **`/model`, `/effort`, `/hard` are refused while a run is in progress** (same "finish or interrupt the run first (esc)" message as `/yolo`/`/clear`): they mutate model/adapter/effort that the run goroutine reads, and a mid-run switch would silently change the model between turns of one run.
3. **The agent has a mutex** (`mu`) around the steering queue, the transcript mirror, usage/cost and the estimate anchor; `append`, `turn`, `request`, `abort`, `Status` and `ContextTokens` use it. The TUI polls `Status()` from the event-loop goroutine while the agent runs in another goroutine — without this, `-race` in a real session is a data race.
4. **Bridge: agent events are delivered through a FIFO pipe (`eventPipe`), not a direct `Program.Send`** — found by the phase gate: `/yolo` calls `agent.SetYolo` from inside `Update`, which emitted `YoloChanged` synchronously; `Program.Send` is a blocking send on an unbuffered channel and `Update` runs on the event-loop goroutine, so the program deadlocked (reproduced in tmux, fixed in `8abca34`). `emit` now only enqueues (order preserved, `TestEventPipeNonBlockingAndFIFO`); the asker still uses a direct send because it is only called from the agent's run goroutine.
5. **`println` no longer sanitizes; `printlnContent` stays for untrusted text** — also found by the gate: sanitizing a lipgloss-styled line (`red.Render(...)`) printed the escape codes as literal `^[[1;31m` text. Styled/trusted lines use `println`, content (model output, user input, tool output, error strings) uses `printlnContent`; the status bar sanitizes its content before styling. Also found: recalling a submission that contained control bytes put them raw into the textarea — history recall now re-chips such entries (`setFromHistory`, tested).
6. **Small adaptations:** `startTestWith(t, s, extraTop, extraModels)` wraps the phase-2 `startTest` (which takes `maxSteps` + modifiers); the `TestAppendStringMissingKeys` fixture uses a real provider's model id (`anthropic/claude-x`) because `config.Parse` validates providers; the multi-call abort test steers once (abort's synthetic ToolEnds re-fire any Emit hook); `handleRunDone` flushes a clean run's trailing live line (the pipe decouples delivery order from `TurnEnd`).

Gate evidence (2026-10-05): mock-driven session in tmux — status bar from the first frame, streaming commit, `⋯`/`▸` items, `/help /cost /model /effort /hard /copy /show /compact`, missing-key refusal, no-enhanced-keyboard hint + `alt+enter`, 120-line paste chip → expand → send (transcript intact), steering after tool results (verified in the captured request payload), approvals (`a`, `A` persisting `python3` with comments + trailing comma intact, `rm` without `[A]` → deny), `/yolo` on/off + 60-col bar, `!`/`!!`, `/undo`, esc-interrupt, `/clear`. Live (opencode-go key): minimax-m3 (anthropic-messages, thinking + tool calls) → glm-5.3-flash (completions) → back to minimax-m3, both continuations clean (no 400), qualified thinking stamps + signatures in the transcript, 2 `model_change` entries, real usage/cost in the bar.

## Review fixes (2026-10-05, two independent subagent reviews)

Verdicts: pass 1 (`docs/reviews/2026-10-05-phase-3-tui.md`) Approve with fixes (0H/1M/3L); pass 2 (`docs/reviews/2026-10-05-phase-3-tui-pass-2.md`) Approve with fixes (1H/2M/6L). All fixed in `640be8e` with regression tests, except the three noted below.

- **H1 (pass 2, reproduced)** — `eventPipe` gates its forwarder until `start()`: events emitted before the program exists (agent.Start skill warnings) are buffered, not sent through a nil `*Program` (a malformed `SKILL.md` in `~/.config/moca/skills/` panicked the TUI). `TestEventPipeBuffersUntilStarted`.
- **M1 (pass 1; folded in by pass 2)** — `/clear` starts the replacement session before closing the old one and re-resolves the branch: the bar no longer regresses to `-`, and a failed restart leaves the old session usable.
- **M2 (pass 2, reproduced)** — the `AppendString` scanner is bounds-total; truncated configs return `unexpected end of input` / `expected ':'` instead of an index-out-of-range panic (`TestAppendStringTruncatedConfig`).
- **L2 (pass 1) / L8 (pass 2)** — chip markers skip collisions with typed text (`TestMarkerNeverCollidesWithTypedText`); collapse re-chips the remembered expansion offset, falling back to string matching (`TestCollapseUsesExactOffset`).
- **L4 (both)** — `handleRunDone` flushes the trailing live line unconditionally (the visible tail of an interrupted stream) and clears the retry transient (`TestRunDoneClearsTransient`).
- **L5 (pass 2)** — `!` is refused while a run is in progress (a note appended between a batch's tool results would diverge from the rebuild order); `!!` (local-only) still runs.
- **L6/L7 (pass 2)** — the approval prompt sanitizes its subject/detail (`TestApprovalPromptSanitized`); `GitBranch` calls are bounded to 3s each.
- Not changed, recorded: **M3 (pass 2)** — allow-always still answers before the config write (per plan; the failure path prints and the command still runs); **L3 (pass 1)** — `eventPipe` keeps deliberate backpressure at a full 1024-slot queue (dropping streamed deltas corrupts the answer); the comment now states the bound; **L9 (pass 2)** — `textOf`/`messageText` dedupe deferred to phase 4, comments corrected.

## Review fixes, pass 3 (2026-10-05, `docs/reviews/2026-10-05-phase-3-tui-pass-3.md`)

Verdict was Approve with fixes (0H/9M/7L); all sixteen are fixed with regression tests (219 top-level tests, race-clean). New test scaffolding: `internal/tui/helpers_test.go` builds a `model` around a real agent (mock completions server) so glue paths — `/clear`, steering return, `!`, allow-always — run against real code.

- **M1** output printed while the pager is open is held (`hold`/`release`) and flushed on close. **M2** `a`/`A`/`d` need a 700 ms typing pause; `enter`/`ctrl+c` fall through during a prompt. **M3** `shellBusy` blocks runs and `/clear` while a `!` executes.
- **M4/M5** `runResult` maps cancellation to the context error (exit 130) and keeps panics as errors; the program runs with `tea.WithoutSignalHandler()` (verified: 20/20 SIGTERM runs exit 130, previously ~40% hung). **L13** quit cancels the run and waits (2 s) for its abort before closing the *current* session.
- **M6/M7** `Input.Prepare`: CR/CRLF → LF, insertion at the cursor via the textarea. **M8** C1 controls sanitised. **M9** `AppendString` resolves symlinks and keeps the file mode.
- **L10** redacted foreign thinking dropped by the transform. **L11** ctrl+c/esc at the trust prompt cancels (130) instead of saving "no". **L12** `git --no-optional-locks status`. **L14** `/clear` carries model/effort (`Agent.Carry`) and `[A]` approvals. **L15** the missing tests. **L16** status width in terminal cells; a response with zero usage no longer anchors the estimate.
