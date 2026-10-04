# Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `moca -p 'hi'` streams a reply from anthropic, opencode-go (both protocol families) and openai, driven by a validated JSONC config, with uniform retry/backoff and the §12.5 `-p` output contract.

**Architecture:** A leaf `internal/llm` package holds the wire-neutral conversation types. `internal/config` parses JSONC (string-literal-aware pre-pass → `encoding/json`) into a typed, validated struct. `internal/provider` has three hand-rolled HTTP+SSE protocol codecs behind one `Adapter` interface, a built-in model catalog (context window, max output, cost, thinking mode/level map), a retry wrapper, and a `Registry` that turns a provider-qualified model id into `(Model, Adapter)`. `cmd/moca` parses the CLI and runs the one-shot path.

**Tech Stack:** Go 1.27.1 (mise), stdlib only (`net/http`, `encoding/json`, `flag`, `net/http/httptest` for tests). No SDKs, no third-party deps in this phase.

**Spec:** `docs/specs/DESIGN.md` (rev 11) — §2, §3, §12, §12.5, phase plan item 1.

## Global Constraints

- Module path `github.com/adeotek/moca`; binary `moca`; Go `1.27.1` (go.mod `go 1.27.1`).
- Dependency direction (§2): every package may import `internal/llm` and `internal/config`; `provider` imports nothing else internal. `llm` imports nothing internal.
- No SDKs. Hand-rolled HTTP + SSE (§3).
- Secrets never literal: `apiKey` must be `"env:VAR"`; validation rejects anything else (§12).
- Model ids are provider-qualified `provider/model`, split at the **first** `/` (§3).
- Effort levels, in order: `off|minimal|low|medium|high|xhigh|max` (§3). Default `medium` where supported.
- Retry: 429 / 5xx / anthropic 529 / connection reset before first byte → backoff 1s,2s,4s,8s,16s with jitter, 5 attempts, honour `retry-after`; SSE stall 90s → abort attempt and retry; mid-stream failure → discard partial, retry once; other 4xx → no retry (§3).
- `max_tokens` = catalog max output capped at `reserveTokens` (default 16384); anthropic `budget` mode: `budget_tokens = max_tokens − 4096`, min 1024; `adaptive` models take no budget (§3).
- `-p` contract: stdout = final assistant text only; stderr = one-liners, retries, exit summary (tokens in/out + cost). Exit codes `0` ok · `1` provider/runtime · `2` config/usage · `3` maxSteps · `130` interrupted (§12.5).
- Config path `~/.config/moca/config.jsonc` (honour `$XDG_CONFIG_HOME`); data dir `~/.local/share/moca/` (honour `$XDG_DATA_HOME`).
- Custom-provider models must declare `contextWindow`; any model with `contextWindow < 16384` is refused (§3, §6).
- Config key `yolo` (bool, default false) and flags `--yolo` / `--no-yolo` are parsed in this phase; they take effect in phase 2 (§7.5).
- Build version stamp: `config.Version`, set with `-ldflags "-X github.com/adeotek/moca/internal/config.Version=…"`, default `"0.0.0-dev"`.

## Review Focus

1. **An `env:` variable that is unset or empty** → a clear error naming the variable, raised when the first request to that provider is built (not at startup — an unused provider's missing key must not block other providers), exit code `2`. Tested in Task 10.
2. **SSE framing variants**: CRLF line endings, `data:` without a space, multi-line `data:` fields, comment lines (`: ping`) and blank keep-alives must parse identically. Tested in Task 5.
3. **Tool-call arguments split across many deltas, interleaved with text** (and, for openai-completions, multiple tool calls distinguished only by `index`) → assembled into complete JSON per call in emitted order. Tested in Tasks 6–8.
4. **Ctrl-C during a stream or during a backoff sleep** → no further retry, exit `130`, nothing partial on stdout beyond what was already streamed. Tested in Tasks 9 and 11.
5. **Bare model id (`glm-5.3`) or unknown provider (`foo/bar`) on `--model` or in config** → exit `2` with a message naming the valid form and the known providers. Tested in Tasks 3 and 10.

---

## File Structure

```
cmd/moca/
  main.go            entry: os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
  cli.go             flag parsing → Options; usage errors
  oneshot.go         -p path: build request, stream, print, exit summary
  cli_test.go        flag parsing + exit-code tests (fake provider via httptest)
internal/llm/
  types.go           Role, ContentBlock, ToolCall, ToolResult, Message, ToolSpec,
                     Request, Usage, StopReason, Event, Response, Effort
  types_test.go
internal/config/
  version.go         Version
  paths.go           ConfigDir, DataDir, ConfigFile
  jsonc.go           Standardize (comment/trailing-comma pre-pass)
  jsonc_test.go
  config.go          Config structs, defaults, Parse, Load, Validate, SplitModel
  env.go             ResolveEnv, EnvRefs
  config_test.go
  testdata/example.jsonc   the §12 example, verbatim
internal/provider/
  errors.go          HTTPError, ErrStall, ErrContextOverflow, Retryable
  sse.go             readSSE with stall timeout
  sse_test.go
  catalog.go         Model, Cost, built-in catalog, ClampEffort, MaxTokens, CostOf
  catalog_test.go
  anthropic.go       anthropic-messages codec
  anthropic_test.go
  openai_completions.go
  openai_completions_test.go
  openai_responses.go
  openai_responses_test.go
  retry.go           WithRetry
  retry_test.go
  registry.go        Registry: config → (Model, Adapter); credentials
  registry_test.go
```

---

### Task 1: Wire-neutral conversation types (`internal/llm`)

**Files:**
- Create: `internal/llm/types.go`
- Test: `internal/llm/types_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (used by every later phase — names are final):
  - `type Role string` — `RoleUser = "user"`, `RoleAssistant = "assistant"`.
  - `type BlockType string` — `BlockText`, `BlockThinking`, `BlockToolUse`, `BlockToolResult`.
  - `type ContentBlock struct { Type BlockType; Text string; Signature string; ThinkingID string; Model string; ToolCall *ToolCall; ToolResult *ToolResult }`
  - `type ToolCall struct { ID, Name string; Input json.RawMessage }`
  - `type ToolResult struct { CallID string; Content string; IsError bool }`
  - `type Message struct { Role Role; Content []ContentBlock }`
  - `type ToolSpec struct { Name, Description string; Schema json.RawMessage }`
  - `type ToolChoice string` — `ToolChoiceAuto = "auto"`, `ToolChoiceNone = "none"`.
  - `type Effort string` + `Efforts []Effort` (ordered) + `ParseEffort(string) (Effort, error)` + `(Effort) Rank() int`.
  - `type Request struct { Model string; System string; Messages []Message; Tools []ToolSpec; ToolChoice ToolChoice; MaxTokens int; Effort Effort; NoCacheWrite bool }`
  - `type Usage struct { Input, Output, CacheRead, CacheWrite int }` + `(Usage) Add(Usage) Usage`.
  - `type StopReason string` — `StopEnd`, `StopToolUse`, `StopLength`, `StopRefusal`.
  - `type EventType int` — `EventText`, `EventThinking`, `EventToolCall`, `EventReset`.
  - `type Event struct { Type EventType; Text string; ToolCall *ToolCall }`
  - `type Response struct { Message Message; Usage Usage; Stop StopReason }`

- [ ] **Step 1: Write the failing test**

```go
// internal/llm/types_test.go
package llm

import "testing"

func TestParseEffort(t *testing.T) {
	for _, s := range []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"} {
		e, err := ParseEffort(s)
		if err != nil || string(e) != s {
			t.Fatalf("ParseEffort(%q) = %q, %v", s, e, err)
		}
	}
	if _, err := ParseEffort("med"); err == nil {
		t.Fatal("ParseEffort(med) must fail; status-bar abbreviations are display only")
	}
}

func TestEffortRankOrdered(t *testing.T) {
	for i := 1; i < len(Efforts); i++ {
		if Efforts[i-1].Rank() >= Efforts[i].Rank() {
			t.Fatalf("%s must rank below %s", Efforts[i-1], Efforts[i])
		}
	}
}

func TestUsageAdd(t *testing.T) {
	got := Usage{1, 2, 3, 4}.Add(Usage{10, 20, 30, 40})
	if got != (Usage{11, 22, 33, 44}) {
		t.Fatalf("Add = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/llm/`
Expected: FAIL — `undefined: ParseEffort`.

- [ ] **Step 3: Implement**

```go
// internal/llm/types.go

// Package llm holds the wire-neutral conversation types shared by every
// package. It is a leaf: it imports nothing from moca.
package llm

import (
	"encoding/json"
	"fmt"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type BlockType string

const (
	BlockText       BlockType = "text"
	BlockThinking   BlockType = "thinking"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

// ContentBlock is one block of a message. Thinking blocks keep the opaque
// replay payload (anthropic signature / openai encrypted_content) in
// Signature, the openai reasoning item id in ThinkingID, and the producing
// model id in Model (§3 cross-provider transform, §8 persistence).
type ContentBlock struct {
	Type       BlockType   `json:"type"`
	Text       string      `json:"text,omitempty"`
	Signature  string      `json:"signature,omitempty"`
	ThinkingID string      `json:"thinkingId,omitempty"`
	Model      string      `json:"model,omitempty"`
	ToolCall   *ToolCall   `json:"toolCall,omitempty"`
	ToolResult *ToolResult `json:"toolResult,omitempty"`
}

type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type ToolResult struct {
	CallID  string `json:"callId"`
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

type Effort string

const (
	EffortOff     Effort = "off"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
)

// Efforts lists every level, lowest first.
var Efforts = []Effort{EffortOff, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}

func (e Effort) Rank() int {
	for i, x := range Efforts {
		if x == e {
			return i
		}
	}
	return -1
}

func ParseEffort(s string) (Effort, error) {
	if e := Effort(s); e.Rank() >= 0 {
		return e, nil
	}
	return "", fmt.Errorf("unknown effort %q (want off|minimal|low|medium|high|xhigh|max)", s)
}

// Request is protocol-neutral. Model is the bare model id (no provider
// prefix). Effort must already be clamped for the model (provider.ClampEffort).
type Request struct {
	Model        string
	System       string
	Messages     []Message
	Tools        []ToolSpec
	ToolChoice   ToolChoice
	MaxTokens    int
	Effort       Effort
	NoCacheWrite bool // compaction summaries: never write the prompt cache (§6)
}

// Usage is normalized: Input excludes cache reads and writes on every
// protocol (openai's prompt_tokens includes cached tokens; adapters subtract).
type Usage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
}

func (u Usage) Add(o Usage) Usage {
	return Usage{u.Input + o.Input, u.Output + o.Output, u.CacheRead + o.CacheRead, u.CacheWrite + o.CacheWrite}
}

type StopReason string

const (
	StopEnd     StopReason = "end_turn"
	StopToolUse StopReason = "tool_use"
	StopLength  StopReason = "length"
	StopRefusal StopReason = "refusal"
)

type EventType int

const (
	EventText     EventType = iota // Text = delta
	EventThinking                  // Text = delta
	EventToolCall                  // ToolCall = complete call
	EventReset                     // a mid-stream retry began: discard everything streamed so far
)

type Event struct {
	Type     EventType
	Text     string
	ToolCall *ToolCall
}

type Response struct {
	Message Message
	Usage   Usage
	Stop    StopReason
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/llm/ && go vet ./internal/llm/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/llm
git commit -m "feat(llm): wire-neutral conversation types"
```

---

### Task 2: JSONC pre-pass

**Files:**
- Create: `internal/config/jsonc.go`, `internal/config/version.go`
- Test: `internal/config/jsonc_test.go`

**Interfaces:**
- Produces: `func Standardize(src []byte) ([]byte, error)` — same length as `src`; comments → spaces (newlines kept); trailing commas → space. `var Version = "0.0.0-dev"`.

- [ ] **Step 1: Write the failing test**

```go
// internal/config/jsonc_test.go
package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestStandardizeTable(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain json untouched", `{"a":[1,2],"b":"x"}`, `{"a":[1,2],"b":"x"}`},
		{"line comment", "{\"a\":1 // c\n}", "{\"a\":1     \n}"},
		{"block comment keeps newline", "{/*x\ny*/\"a\":1}", "{   \n   \"a\":1}"},
		{"url in string survives", `{"u":"https://x.io/a//b"}`, `{"u":"https://x.io/a//b"}`},
		{"escaped quote in string", `{"s":"a\"//b"}`, `{"s":"a\"//b"}`},
		{"trailing comma object", `{"a":1,}`, `{"a":1 }`},
		{"trailing comma array with comment", "[1, // x\n]", "[1      \n]"},
		{"comma before comment then value kept", "[1, /*c*/ 2]", "[1,       2]"},
		{"comment chars in string after comment", "{//x\n\"a\":\"/*\"}", "{   \n\"a\":\"/*\"}"},
	}
	for _, c := range cases {
		got, err := Standardize([]byte(c.in))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if len(got) != len(c.in) {
			t.Errorf("%s: length changed — error offsets would drift", c.name)
		}
	}
}

func TestStandardizeUnterminatedBlock(t *testing.T) {
	if _, err := Standardize([]byte(`{"a":1 /* oops`)); err == nil {
		t.Fatal("want error for unterminated block comment")
	}
}

func TestStandardizeNoOpOnPlainJSON(t *testing.T) {
	src := []byte("{\n  \"model\": \"a/b\",\n  \"x\": [1, 2, {\"y\": \"//\"}]\n}\n")
	got, _ := Standardize(src)
	if !bytes.Equal(got, src) {
		t.Fatal("pre-pass must be a no-op on plain JSON")
	}
	var v any
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run Standardize`
Expected: FAIL — `undefined: Standardize`.

- [ ] **Step 3: Implement**

```go
// internal/config/version.go
package config

// Version is stamped at build time via -ldflags -X.
var Version = "0.0.0-dev"
```

```go
// internal/config/jsonc.go
package config

import "errors"

// Standardize turns JSONC into JSON that encoding/json accepts. Comments are
// recognized only outside string literals (escapes honoured) and replaced by
// spaces, keeping newlines, so the output has the same length and line
// structure as the input and decode-error offsets still point at the right
// line:col. A comma followed only by whitespace/comments before } or ] is
// blanked (trailing comma).
func Standardize(src []byte) ([]byte, error) {
	out := append([]byte(nil), src...)
	const (
		code = iota
		str
		lineComment
		blockComment
	)
	state := code
	pendingComma := -1
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch state {
		case str:
			if c == '\\' {
				i++
			} else if c == '"' {
				state = code
			}
		case lineComment:
			if c == '\n' {
				state = code
			} else {
				out[i] = ' '
			}
		case blockComment:
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = code
			} else if c != '\n' && c != '\r' {
				out[i] = ' '
			}
		case code:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				state = lineComment
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				out[i], out[i+1] = ' ', ' '
				i++
				state = blockComment
			case c == '"':
				pendingComma = -1
				state = str
			case c == ',':
				pendingComma = i
			case c == '}' || c == ']':
				if pendingComma >= 0 {
					out[pendingComma] = ' '
				}
				pendingComma = -1
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			default:
				pendingComma = -1
			}
		}
	}
	if state == blockComment {
		return nil, errors.New("unterminated /* comment")
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config/ -run Standardize -v`
Expected: PASS (all subcases).

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): string-literal-aware JSONC pre-pass with trailing-comma removal"
```

---

### Task 3: Typed config — structs, defaults, decode, validate, env indirection, paths

**Files:**
- Create: `internal/config/config.go`, `internal/config/env.go`, `internal/config/paths.go`, `internal/config/testdata/example.jsonc`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `Standardize` (Task 2).
- Produces:
  - `type Config struct { Model, ModelHard string; Providers map[string]ProviderConfig; Shell ShellConfig; MCP MCPConfig; Snapshot SnapshotConfig; Context ContextConfig; Yolo bool }`
  - `type ProviderConfig struct { Auth, APIKey, BaseURL, Protocol string; BaseURLs map[string]string; Models map[string]ModelOverride }`
  - `type ModelOverride struct { Protocol string; ContextWindow, MaxOutputTokens int; Cost *CostConfig }`, `type CostConfig struct { Input, Output, CacheRead, CacheWrite float64 }`
  - `type ShellConfig struct { Allow []string }`; `type MCPConfig struct { IdleTimeout int; Servers map[string]MCPServer }`; `type MCPServer struct { Command string; Args []string; Env map[string]string; URL string; Headers map[string]string; Description string; Approve []string }`; `type SnapshotConfig struct { RetentionDays *int }`; `type ContextConfig struct { ReserveTokens, KeepRecentTokens, MaxSteps int }`
  - `func Parse(data []byte) (Config, error)` — pre-pass, strict decode (unknown keys rejected), defaults applied, structural validation.
  - `func Load(path string) (Config, error)` — missing file → `Default()`.
  - `func Default() Config`
  - `func (c Config) RetentionDays() int`
  - `func SplitModel(q string) (provider, model string, err error)`
  - `func ResolveEnv(ref string) (string, error)`; `func EnvRefs(c Config) []string`
  - `func ConfigDir() string`, `DataDir() string`, `ConfigFile() string`
  - `const MinContextWindow = 16384`
  - `var DefaultShellAllow []string`

- [ ] **Step 1: Add the spec example verbatim**

Copy the `jsonc` block of DESIGN.md §12 (lines starting `{` … `}`) byte-for-byte into `internal/config/testdata/example.jsonc`. Do **not** reformat it — the trailing commas after the `context7` and `servers` entries and the `//` inside URLs are the point of the test.

- [ ] **Step 2: Write the failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSpecExampleDecodesIntact(t *testing.T) {
	data, err := os.ReadFile("testdata/example.jsonc")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse(data)
	if err != nil {
		t.Fatalf("§12 example must decode: %v", err)
	}
	if c.Model != "opencode-go/glm-5.3-flash" || c.ModelHard != "opencode-go/glm-5.3" {
		t.Fatalf("models: %q %q", c.Model, c.ModelHard)
	}
	if got := c.MCP.Servers["context7"].URL; got != "https://mcp.context7.com/mcp" {
		t.Fatalf("URL mangled: %q", got)
	}
	if c.MCP.Servers["context7"].Description != "library/API docs lookup" {
		t.Fatal("description lost")
	}
	if c.Providers["anthropic"].APIKey != "env:ANTHROPIC_API_KEY" {
		t.Fatal("apiKey lost")
	}
	if !slices.Contains(c.Shell.Allow, "graphify") || len(c.Shell.Allow) != 32 {
		t.Fatalf("allow list: %v", c.Shell.Allow)
	}
	if c.MCP.IdleTimeout != 600 || c.RetentionDays() != 30 {
		t.Fatal("scalars lost")
	}
	if c.Context != (ContextConfig{ReserveTokens: 16384, KeepRecentTokens: 20000, MaxSteps: 40}) {
		t.Fatalf("context: %+v", c.Context)
	}
	if c.Yolo {
		t.Fatal("example ships yolo off")
	}
	if y, _ := Parse([]byte(`{"yolo": true}`)); !y.Yolo {
		t.Fatal("yolo key decodes")
	}
}

func TestDefaultsApplied(t *testing.T) {
	c, err := Parse([]byte(`{"model":"anthropic/x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Context.ReserveTokens != 16384 || c.Context.KeepRecentTokens != 20000 || c.Context.MaxSteps != 40 {
		t.Fatalf("context defaults: %+v", c.Context)
	}
	if c.MCP.IdleTimeout != 600 || c.RetentionDays() != 30 {
		t.Fatal("scalar defaults")
	}
	if !slices.Equal(c.Shell.Allow, DefaultShellAllow) {
		t.Fatal("default allowlist")
	}
	if c.Providers["anthropic"].APIKey != "env:ANTHROPIC_API_KEY" ||
		c.Providers["openai"].APIKey != "env:OPENAI_API_KEY" ||
		c.Providers["opencode-go"].APIKey != "env:OPENCODE_API_KEY" {
		t.Fatalf("built-in provider defaults: %+v", c.Providers)
	}
}

func TestRetentionZeroMeansForever(t *testing.T) {
	c, err := Parse([]byte(`{"snapshot":{"retentionDays":0}}`))
	if err != nil || c.RetentionDays() != 0 {
		t.Fatalf("retention 0 must survive defaults: %v %d", err, c.RetentionDays())
	}
}

func TestValidationErrors(t *testing.T) {
	cases := map[string]string{
		`{"model":"glm-5.3"}`:                                         "provider/model",
		`{"model":"foo/bar"}`:                                         "unknown provider \"foo\"",
		`{"providers":{"anthropic":{"auth":"api_key","apiKey":"sk-x"}}}`: "env:",
		`{"providers":{"anthropic":{"auth":"magic"}}}`:                "auth",
		`{"providers":{"vllm":{"auth":"api_key","apiKey":"env:K"}}}`:     "baseUrl",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"openai-completions","auth":"api_key","apiKey":"env:K","models":{"m":{}}}}}`: "contextWindow",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"openai-completions","auth":"api_key","apiKey":"env:K","models":{"m":{"contextWindow":8192}}}}}`: "16384",
		`{"providers":{"vllm":{"baseUrl":"http://x/v1","protocol":"grpc","auth":"api_key","apiKey":"env:K"}}}`: "protocol",
		`{"modle":"x/y"}`: "unknown field",
		`{"context":{"maxSteps":-1}}`: "maxSteps",
	}
	for in, want := range cases {
		_, err := Parse([]byte(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) err = %v, want containing %q", in, err, want)
		}
	}
}

func TestSyntaxErrorReportsLineCol(t *testing.T) {
	_, err := Parse([]byte("{\n  // c\n  \"model\": ,\n}"))
	if err == nil || !strings.Contains(err.Error(), "3:") {
		t.Fatalf("want line 3 in error, got %v", err)
	}
}

func TestSplitModel(t *testing.T) {
	p, m, err := SplitModel("openai/gpt-x/variant")
	if err != nil || p != "openai" || m != "gpt-x/variant" {
		t.Fatalf("%q %q %v", p, m, err)
	}
	for _, bad := range []string{"", "x", "/m", "p/"} {
		if _, _, err := SplitModel(bad); err == nil {
			t.Errorf("SplitModel(%q) must fail", bad)
		}
	}
}

func TestResolveEnv(t *testing.T) {
	t.Setenv("MOCA_T_KEY", "v")
	if v, err := ResolveEnv("env:MOCA_T_KEY"); err != nil || v != "v" {
		t.Fatal(v, err)
	}
	t.Setenv("MOCA_T_EMPTY", "")
	if _, err := ResolveEnv("env:MOCA_T_EMPTY"); err == nil || !strings.Contains(err.Error(), "MOCA_T_EMPTY") {
		t.Fatalf("empty var must name the variable: %v", err)
	}
	if v, err := ResolveEnv("literal"); err != nil || v != "literal" {
		t.Fatal("non-env strings pass through (headers may be literal)")
	}
}

func TestEnvRefs(t *testing.T) {
	c, _ := Parse([]byte(`{"mcp":{"servers":{"gh":{"url":"https://x","headers":{"Authorization":"env:GH_TOK"}},"fs":{"command":"x","env":{"A":"env:FS_KEY","B":"lit"}}}}}`))
	refs := EnvRefs(c)
	for _, want := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENCODE_API_KEY", "GH_TOK", "FS_KEY"} {
		if !slices.Contains(refs, want) {
			t.Errorf("EnvRefs missing %s: %v", want, refs)
		}
	}
}

func TestLoadMissingFileIsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.jsonc"))
	if err != nil || c.Context.MaxSteps != 40 {
		t.Fatal(err)
	}
}

func TestPathsHonourXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_DATA_HOME", "/data")
	if ConfigFile() != "/cfg/moca/config.jsonc" || DataDir() != "/data/moca" {
		t.Fatal(ConfigFile(), DataDir())
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Parse`.

- [ ] **Step 4: Implement `paths.go` and `env.go`**

```go
// internal/config/paths.go
package config

import (
	"os"
	"path/filepath"
)

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// ConfigDir is ~/.config/moca (or $XDG_CONFIG_HOME/moca).
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "moca")
	}
	return filepath.Join(home(), ".config", "moca")
}

// DataDir is ~/.local/share/moca (or $XDG_DATA_HOME/moca): sessions,
// snapshots, auth.json, trust.json, mcp-index.json, builtin-skills.
func DataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "moca")
	}
	return filepath.Join(home(), ".local", "share", "moca")
}

func ConfigFile() string { return filepath.Join(ConfigDir(), "config.jsonc") }
```

```go
// internal/config/env.go
package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

const envPrefix = "env:"

// ResolveEnv resolves an "env:VAR" reference. Any other string is returned
// unchanged (MCP headers/env values may be literal; apiKey is validated to be
// a reference separately).
func ResolveEnv(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, envPrefix)
	if !ok {
		return ref, nil
	}
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("environment variable %s is not set (referenced as %q)", name, ref)
	}
	return v, nil
}

// EnvRefs lists every variable named by an env: reference anywhere in the
// config. The shell tool removes these from the model's environment (§4).
func EnvRefs(c Config) []string {
	var out []string
	add := func(s string) {
		if name, ok := strings.CutPrefix(s, envPrefix); ok && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, p := range c.Providers {
		add(p.APIKey)
	}
	for _, s := range c.MCP.Servers {
		for _, v := range s.Env {
			add(v)
		}
		for _, v := range s.Headers {
			add(v)
		}
	}
	slices.Sort(out)
	return out
}
```

- [ ] **Step 5: Implement `config.go`**

```go
// internal/config/config.go
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

const MinContextWindow = 16384

var DefaultShellAllow = []string{"go", "git", "gh", "grep", "rg", "find", "ls", "cat", "head", "tail",
	"wc", "sort", "uniq", "diff", "which", "jq", "mkdir", "sed", "awk",
	"curl", "make", "mise", "python", "pytest", "node", "npm",
	"docker", "kubectl", "terraform", "ansible", "rtk", "graphify"}

// BuiltinProviders are always known; config entries with these names
// override fields, they don't need baseUrl/protocol.
var BuiltinProviders = map[string]string{
	"anthropic":   "env:ANTHROPIC_API_KEY",
	"opencode-go": "env:OPENCODE_API_KEY",
	"openai":      "env:OPENAI_API_KEY",
}

var Protocols = []string{"anthropic-messages", "openai-completions", "openai-responses"}

type Config struct {
	Model     string                    `json:"model"`
	ModelHard string                    `json:"modelHard"`
	Providers map[string]ProviderConfig `json:"providers"`
	Shell     ShellConfig               `json:"shell"`
	MCP       MCPConfig                 `json:"mcp"`
	Snapshot  SnapshotConfig            `json:"snapshot"`
	Context   ContextConfig             `json:"context"`
	Yolo      bool                      `json:"yolo"` // all permission checks off by default (§7.5)
}

type ProviderConfig struct {
	Auth     string                   `json:"auth"`               // "api_key" | "oauth"
	APIKey   string                   `json:"apiKey,omitempty"`   // must be "env:VAR"
	BaseURL  string                   `json:"baseUrl,omitempty"`  // overrides every protocol's base URL
	BaseURLs map[string]string        `json:"baseUrls,omitempty"` // per-protocol override (opencode-go's dual URLs)
	Protocol string                   `json:"protocol,omitempty"` // custom providers: default protocol of its models
	Models   map[string]ModelOverride `json:"models,omitempty"`
}

type ModelOverride struct {
	Protocol        string      `json:"protocol,omitempty"`
	ContextWindow   int         `json:"contextWindow,omitempty"`
	MaxOutputTokens int         `json:"maxOutputTokens,omitempty"`
	Cost            *CostConfig `json:"cost,omitempty"`
}

type CostConfig struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

type ShellConfig struct {
	Allow []string `json:"allow"`
}

type MCPConfig struct {
	IdleTimeout int                  `json:"idleTimeout"` // seconds
	Servers     map[string]MCPServer `json:"servers"`
}

type MCPServer struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Description string            `json:"description,omitempty"`
	Approve     []string          `json:"approve,omitempty"`
}

type SnapshotConfig struct {
	RetentionDays *int `json:"retentionDays,omitempty"` // nil → 30; 0 → forever
}

type ContextConfig struct {
	ReserveTokens    int `json:"reserveTokens"`
	KeepRecentTokens int `json:"keepRecentTokens"`
	MaxSteps         int `json:"maxSteps"`
}

func (c Config) RetentionDays() int {
	if c.Snapshot.RetentionDays == nil {
		return 30
	}
	return *c.Snapshot.RetentionDays
}

func Default() Config {
	c := Config{}
	c.applyDefaults()
	return c
}

func (c *Config) applyDefaults() {
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
	for name, key := range BuiltinProviders {
		p := c.Providers[name]
		if p.Auth == "" {
			p.Auth = "api_key"
		}
		if p.Auth == "api_key" && p.APIKey == "" {
			p.APIKey = key
		}
		c.Providers[name] = p
	}
	if c.Shell.Allow == nil {
		c.Shell.Allow = slices.Clone(DefaultShellAllow)
	}
	if c.MCP.IdleTimeout == 0 {
		c.MCP.IdleTimeout = 600
	}
	if c.Context.ReserveTokens == 0 {
		c.Context.ReserveTokens = 16384
	}
	if c.Context.KeepRecentTokens == 0 {
		c.Context.KeepRecentTokens = 20000
	}
	if c.Context.MaxSteps == 0 {
		c.Context.MaxSteps = 40
	}
}

// Load reads and parses path. A missing file yields Default().
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	c, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func Parse(data []byte) (Config, error) {
	std, err := Standardize(data)
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(std))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, withPosition(data, err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func withPosition(src []byte, err error) error {
	var off int64 = -1
	var se *json.SyntaxError
	var te *json.UnmarshalTypeError
	switch {
	case errors.As(err, &se):
		off = se.Offset
	case errors.As(err, &te):
		off = te.Offset
	}
	if off < 0 {
		return err
	}
	line, col := 1, 1
	for _, b := range src[:min(int(off), len(src))] {
		if b == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return fmt.Errorf("%d:%d: %w", line, col, err)
}

func SplitModel(q string) (string, string, error) {
	p, m, ok := strings.Cut(q, "/")
	if !ok || p == "" || m == "" {
		return "", "", fmt.Errorf("model %q must be provider-qualified: provider/model (e.g. opencode-go/glm-5.3-flash)", q)
	}
	return p, m, nil
}

// Validate checks structure only. Whether a built-in provider knows a model
// id is checked by provider.Registry (it owns the catalog).
func (c Config) Validate() error {
	for _, q := range []string{c.Model, c.ModelHard} {
		if q == "" {
			continue
		}
		p, _, err := SplitModel(q)
		if err != nil {
			return err
		}
		if _, ok := c.Providers[p]; !ok {
			return fmt.Errorf("model %q: unknown provider %q (known: %s)", q, p, strings.Join(c.providerNames(), ", "))
		}
	}
	for name, p := range c.Providers {
		switch p.Auth {
		case "api_key":
			if !strings.HasPrefix(p.APIKey, envPrefix) {
				return fmt.Errorf("providers.%s.apiKey must be an env: reference (e.g. \"env:MY_KEY\"), never a literal", name)
			}
		case "oauth":
		default:
			return fmt.Errorf("providers.%s.auth must be \"api_key\" or \"oauth\", got %q", name, p.Auth)
		}
		_, builtin := BuiltinProviders[name]
		if !builtin && p.BaseURL == "" && len(p.BaseURLs) == 0 {
			return fmt.Errorf("providers.%s: custom provider needs baseUrl and protocol", name)
		}
		if p.Protocol != "" && !slices.Contains(Protocols, p.Protocol) {
			return fmt.Errorf("providers.%s.protocol %q unknown (want %s)", name, p.Protocol, strings.Join(Protocols, "|"))
		}
		if !builtin && p.Protocol == "" {
			return fmt.Errorf("providers.%s: custom provider needs protocol", name)
		}
		for mid, m := range p.Models {
			if m.Protocol != "" && !slices.Contains(Protocols, m.Protocol) {
				return fmt.Errorf("providers.%s.models.%s.protocol %q unknown", name, mid, m.Protocol)
			}
			if !builtin && m.ContextWindow == 0 {
				return fmt.Errorf("providers.%s.models.%s: custom models must declare contextWindow", name, mid)
			}
			if m.ContextWindow != 0 && m.ContextWindow < MinContextWindow {
				return fmt.Errorf("providers.%s.models.%s: contextWindow %d below the %d minimum", name, mid, m.ContextWindow, MinContextWindow)
			}
		}
	}
	if c.Context.MaxSteps < 1 || c.Context.ReserveTokens < 1 || c.Context.KeepRecentTokens < 1 {
		return fmt.Errorf("context: reserveTokens, keepRecentTokens and maxSteps must be positive")
	}
	if c.Snapshot.RetentionDays != nil && *c.Snapshot.RetentionDays < 0 {
		return fmt.Errorf("snapshot.retentionDays must be >= 0")
	}
	for name, s := range c.MCP.Servers {
		if (s.Command == "") == (s.URL == "") {
			return fmt.Errorf("mcp.servers.%s: exactly one of command (stdio) or url (streamable HTTP)", name)
		}
	}
	return nil
}

func (c Config) providerNames() []string {
	var n []string
	for k := range c.Providers {
		n = append(n, k)
	}
	slices.Sort(n)
	return n
}
```

Note on the negative-`maxSteps` case: `applyDefaults` replaces only `0`, so `-1` survives to `Validate` and fails there.

- [ ] **Step 6: Run tests**

Run: `go test ./internal/config/ -v`
Expected: PASS. If `TestSpecExampleDecodesIntact` fails on a count, recount the §12 allow list in DESIGN.md (it has 32 entries) — do not edit the fixture.

- [ ] **Step 7: Commit**

```bash
git add internal/config
git commit -m "feat(config): typed JSONC config with defaults, strict validation, env indirection"
```

---

### Task 4: Model catalog, effort clamping, max-tokens rule, cost

**Files:**
- Create: `internal/provider/catalog.go`
- Test: `internal/provider/catalog_test.go`

**Interfaces:**
- Consumes: `llm.Effort`, `llm.Usage`, `config.ModelOverride`, `config.CostConfig`.
- Produces:
  - `type Model struct { Provider, ID, Protocol string; ContextWindow, MaxOutput int; Cost config.CostConfig; ThinkingMode string; ThinkingLevelMap map[llm.Effort]string }`
  - `func (m Model) Qualified() string`
  - `func (m Model) ClampEffort(e llm.Effort) llm.Effort`
  - `func (m Model) DefaultEffort() llm.Effort`
  - `func (m Model) MaxTokens(reserve int) int`
  - `func (m Model) BudgetTokens(maxTokens int) int` (only meaningful for `ThinkingMode == "budget"`)
  - `func (m Model) CostOf(u llm.Usage) float64`
  - `var builtinCatalog []Model`
  - Thinking modes: `"none" | "budget" | "adaptive" | "openai"`.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/catalog_test.go
package provider

import (
	"math"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestClampEffort(t *testing.T) {
	m := Model{ThinkingMode: "adaptive", ThinkingLevelMap: map[llm.Effort]string{
		llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max",
	}}
	cases := map[llm.Effort]llm.Effort{
		llm.EffortOff:     llm.EffortLow, // cannot disable → lowest supported
		llm.EffortMinimal: llm.EffortLow,
		llm.EffortMedium:  llm.EffortMedium,
		llm.EffortXHigh:   llm.EffortHigh, // nearest at or below
		llm.EffortMax:     llm.EffortMax,
	}
	for in, want := range cases {
		if got := m.ClampEffort(in); got != want {
			t.Errorf("Clamp(%s) = %s, want %s", in, got, want)
		}
	}
	none := Model{ThinkingMode: "none"}
	if none.ClampEffort(llm.EffortHigh) != llm.EffortOff {
		t.Fatal("non-thinking model clamps everything to off")
	}
}

func TestDefaultEffort(t *testing.T) {
	m := Model{ThinkingMode: "openai", ThinkingLevelMap: map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high"}}
	if m.DefaultEffort() != llm.EffortLow {
		t.Fatal("medium unsupported → clamp(medium)")
	}
}

func TestMaxTokensAndBudget(t *testing.T) {
	m := Model{MaxOutput: 64000, ThinkingMode: "budget"}
	if m.MaxTokens(16384) != 16384 {
		t.Fatal("capped at reserve")
	}
	if (Model{MaxOutput: 8192}).MaxTokens(16384) != 8192 {
		t.Fatal("catalog max wins when smaller")
	}
	if m.BudgetTokens(16384) != 16384-4096 {
		t.Fatal("budget = max − 4K")
	}
	if m.BudgetTokens(4096) != 1024 {
		t.Fatal("budget floor 1024")
	}
}

func TestCostOf(t *testing.T) {
	m := Model{Cost: costM(1.4, 4.4, 0.14, 0)}
	got := m.CostOf(llm.Usage{Input: 1_000_000, Output: 500_000, CacheRead: 2_000_000})
	if math.Abs(got-(1.4+2.2+0.28)) > 1e-9 {
		t.Fatalf("cost %v", got)
	}
}

func TestBuiltinCatalogSane(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range builtinCatalog {
		q := m.Qualified()
		if seen[q] {
			t.Fatalf("duplicate %s", q)
		}
		seen[q] = true
		if m.ContextWindow < 16384 || m.MaxOutput <= 0 || m.Protocol == "" || m.ThinkingMode == "" {
			t.Fatalf("incomplete entry %+v", m)
		}
	}
	for _, q := range []string{"opencode-go/glm-5.3", "opencode-go/glm-5.3-flash"} {
		if !seen[q] {
			t.Fatalf("spec-named model %s missing", q)
		}
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run 'Clamp|Default|MaxTokens|Cost|Catalog'` → FAIL (`undefined: Model`).

- [ ] **Step 3: Implement**

```go
// internal/provider/catalog.go
package provider

import (
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

// Model is one catalog entry. Cost is USD per million tokens.
// ThinkingMode: "none" | "budget" (anthropic budget_tokens) | "adaptive"
// (anthropic thinking:adaptive + output_config.effort) | "openai" (reasoning
// effort). ThinkingLevelMap maps each supported effort to the wire value
// (budget mode: unused — budget comes from MaxTokens); a missing key means
// the level is unsupported. EffortOff present means thinking can be disabled.
type Model struct {
	Provider         string
	ID               string
	Protocol         string
	ContextWindow    int
	MaxOutput        int
	Cost             config.CostConfig
	ThinkingMode     string
	ThinkingLevelMap map[llm.Effort]string
}

func (m Model) Qualified() string { return m.Provider + "/" + m.ID }

func (m Model) ClampEffort(e llm.Effort) llm.Effort {
	if m.ThinkingMode == "none" || len(m.ThinkingLevelMap) == 0 {
		return llm.EffortOff
	}
	best := llm.Effort("")
	for _, lvl := range llm.Efforts {
		if _, ok := m.ThinkingLevelMap[lvl]; !ok {
			continue
		}
		if lvl.Rank() <= e.Rank() {
			best = lvl
		} else if best == "" {
			return lvl // nothing at or below → lowest supported
		}
	}
	return best
}

func (m Model) DefaultEffort() llm.Effort { return m.ClampEffort(llm.EffortMedium) }

func (m Model) MaxTokens(reserve int) int { return min(m.MaxOutput, reserve) }

func (m Model) BudgetTokens(maxTokens int) int { return max(maxTokens-4096, 1024) }

func (m Model) CostOf(u llm.Usage) float64 {
	return (float64(u.Input)*m.Cost.Input + float64(u.Output)*m.Cost.Output +
		float64(u.CacheRead)*m.Cost.CacheRead + float64(u.CacheWrite)*m.Cost.CacheWrite) / 1e6
}

func costM(in, out, cr, cw float64) config.CostConfig {
	return config.CostConfig{Input: in, Output: out, CacheRead: cr, CacheWrite: cw}
}

var (
	anthropicAdaptive = map[llm.Effort]string{llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high", llm.EffortMax: "max"}
	anthropicBudget   = map[llm.Effort]string{llm.EffortOff: "", llm.EffortLow: "", llm.EffortMedium: "", llm.EffortHigh: "", llm.EffortMax: ""}
	openaiReasoning   = map[llm.Effort]string{llm.EffortMinimal: "minimal", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
	glmThinking       = map[llm.Effort]string{llm.EffortOff: "", llm.EffortLow: "low", llm.EffortMedium: "medium", llm.EffortHigh: "high"}
)
```

Then append `builtinCatalog`. Step 4 fills it.

- [ ] **Step 4: Fill `builtinCatalog` from live vendor pages**

Prices, context windows and max-output values change; they are **data, verified at implementation time**, not invented. Open each source and write one `Model{…}` literal per row:

| Row | Source to read | Fields |
|---|---|---|
| `opencode-go/glm-5.3` | DESIGN.md §3 (verified: $1.4/$4.4, 1M ctx) + opencode.ai/zen docs for max output, protocol, cache prices | protocol per the Zen model list |
| `opencode-go/glm-5.3-flash` | DESIGN.md §3 ($0.15/$0.5) + Zen docs | same |
| `opencode-go/minimax-m3`, `kimi-k3`, `gpt-6-luna`, `grok-4.7` | Zen model list | each declares its protocol — the catalog is mixed-protocol |
| `anthropic/<current Opus, Sonnet, Haiku ids>` | docs.anthropic.com models overview + pricing | Opus/Sonnet current gen: `ThinkingMode: "adaptive"`, `anthropicAdaptive`; Haiku 4.5: `"budget"`, `anthropicBudget` |
| `openai/<current flagship + mini>` | platform.openai.com models + pricing | `Protocol: "openai-responses"`, `ThinkingMode: "openai"`, `openaiReasoning` |

Shape of each literal:

```go
var builtinCatalog = []Model{
	{Provider: "opencode-go", ID: "glm-5.3", Protocol: "openai-completions",
		ContextWindow: 1_000_000, MaxOutput: 131072, Cost: costM(1.4, 4.4, 0.14, 0),
		ThinkingMode: "openai", ThinkingLevelMap: glmThinking},
	// … one literal per row of the table above
}
```

The protocol, max-output and cache prices in this example are **illustrative of the shape only**. Replace each with the value from the source. If a source doesn't publish a cache price, use `0`. Record the date you checked in a comment above the slice (`// prices verified 2026-MM-DD`).

- [ ] **Step 5: Run** — `go test ./internal/provider/ -run 'Clamp|Default|MaxTokens|Cost|Catalog' -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/catalog*.go
git commit -m "feat(provider): model catalog with effort clamping, max-tokens rule, cost"
```

---

### Task 5: SSE reader with stall timeout + typed provider errors

**Files:**
- Create: `internal/provider/sse.go`, `internal/provider/errors.go`
- Test: `internal/provider/sse_test.go`

**Interfaces:**
- Produces:
  - `func readSSE(ctx context.Context, body io.Reader, stall time.Duration, fn func(event, data string) error) error` — returns `ErrStall` if no bytes arrive for `stall`.
  - `type HTTPError struct { Status int; RetryAfter time.Duration; Body string }` + `Error()`; `func newHTTPError(resp *http.Response) *HTTPError` (reads ≤ 8 KiB of body).
  - `var ErrStall = errors.New("stream stalled: no data for 90s")`
  - `var ErrContextOverflow = errors.New("context window exceeded")` — adapters wrap: `fmt.Errorf("%w: %s", ErrContextOverflow, body)`.
  - `type streamError struct{ msg string }` for in-stream `error` events (anthropic `overloaded_error` inside the stream is retryable: adapters map it to `&HTTPError{Status: 529}`).
  - `func retryable(err error) bool`
  - `const stallTimeout = 90 * time.Second`

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/sse_test.go
package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type evt struct{ ev, data string }

func collect(t *testing.T, raw string) []evt {
	t.Helper()
	var got []evt
	err := readSSE(context.Background(), strings.NewReader(raw), time.Second, func(e, d string) error {
		got = append(got, evt{e, d})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSSEFramingVariants(t *testing.T) {
	want := []evt{{"a", "1"}, {"", "x\ny"}, {"b", "{}"}}
	lf := "event: a\ndata: 1\n\n: ping\n\ndata: x\ndata: y\n\nevent:b\ndata:{}\n\n"
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	for name, raw := range map[string]string{"lf": lf, "crlf": crlf} {
		got := collect(t, raw)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v", name, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s[%d]: got %v want %v", name, i, got[i], want[i])
			}
		}
	}
}

func TestSSEFinalEventWithoutBlankLine(t *testing.T) {
	if got := collect(t, "data: [DONE]"); len(got) != 1 || got[0].data != "[DONE]" {
		t.Fatalf("%v", got)
	}
}

func TestSSEStall(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()
	go pw.Write([]byte("data: 1\n\n"))
	err := readSSE(context.Background(), pr, 50*time.Millisecond, func(string, string) error { return nil })
	if !errors.Is(err, ErrStall) {
		t.Fatalf("want ErrStall, got %v", err)
	}
}

func TestSSECallbackErrorStops(t *testing.T) {
	boom := errors.New("boom")
	err := readSSE(context.Background(), strings.NewReader("data: 1\n\ndata: 2\n\n"), time.Second,
		func(string, string) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestRetryable(t *testing.T) {
	yes := []error{&HTTPError{Status: 429}, &HTTPError{Status: 500}, &HTTPError{Status: 529}, ErrStall, io.ErrUnexpectedEOF}
	no := []error{&HTTPError{Status: 400}, &HTTPError{Status: 401}, ErrContextOverflow, context.Canceled}
	for _, e := range yes {
		if !retryable(e) {
			t.Errorf("%v should retry", e)
		}
	}
	for _, e := range no {
		if retryable(e) {
			t.Errorf("%v should not retry", e)
		}
	}
}

func TestRetryAfterParsed(t *testing.T) {
	r := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"7"}}, Body: io.NopCloser(strings.NewReader("slow down"))}
	e := newHTTPError(r)
	if e.RetryAfter != 7*time.Second || e.Body != "slow down" {
		t.Fatalf("%+v", e)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run 'SSE|Retryable|RetryAfter'` → FAIL.

- [ ] **Step 3: Implement `errors.go`**

```go
// internal/provider/errors.go
package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"
)

const stallTimeout = 90 * time.Second

var (
	ErrStall           = errors.New("stream stalled: no data for 90s")
	ErrContextOverflow = errors.New("context window exceeded")
)

type HTTPError struct {
	Status     int
	RetryAfter time.Duration
	Body       string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body) }

func newHTTPError(resp *http.Response) *HTTPError {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	e := &HTTPError{Status: resp.StatusCode, Body: string(b)}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			e.RetryAfter = time.Duration(n) * time.Second
		} else if t, err := http.ParseTime(s); err == nil {
			e.RetryAfter = time.Until(t)
		}
	}
	return e
}

func retryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrContextOverflow) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status == 429 || he.Status >= 500
	}
	var ne net.Error
	return errors.Is(err, ErrStall) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || (errors.As(err, &ne) && ne.Timeout())
}
```

- [ ] **Step 4: Implement `sse.go`**

```go
// internal/provider/sse.go
package provider

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"
)

// readSSE parses a text/event-stream. fn receives each dispatched event's
// type ("" when absent) and data (multi-line data joined with "\n"). If no
// line arrives within stall, it returns ErrStall; the caller cancels the
// request so the body read unblocks.
func readSSE(ctx context.Context, body io.Reader, stall time.Duration, fn func(event, data string) error) error {
	lines := make(chan string)
	errc := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(body)
		sc.Buffer(make([]byte, 64<<10), 8<<20)
		for sc.Scan() {
			select {
			case lines <- strings.TrimSuffix(sc.Text(), "\r"):
			case <-ctx.Done():
				return
			}
		}
		errc <- sc.Err()
	}()

	var event string
	var data []string
	dispatch := func() error {
		if len(data) == 0 {
			event = ""
			return nil
		}
		err := fn(event, strings.Join(data, "\n"))
		event, data = "", nil
		return err
	}
	timer := time.NewTimer(stall)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrStall
		case err := <-errc:
			if err != nil {
				return err
			}
			return dispatch()
		case line := <-lines:
			timer.Reset(stall)
			switch {
			case line == "":
				if err := dispatch(); err != nil {
					return err
				}
			case strings.HasPrefix(line, ":"):
			default:
				field, value, _ := strings.Cut(line, ":")
				value = strings.TrimPrefix(value, " ")
				switch field {
				case "event":
					event = value
				case "data":
					data = append(data, value)
				}
			}
		}
	}
}
```

The scanner goroutine exits when the caller closes the response body (every adapter `defer resp.Body.Close()`s) or the context is cancelled.

- [ ] **Step 5: Run** — `go test ./internal/provider/ -run 'SSE|Retryable|RetryAfter' -race -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/sse*.go internal/provider/errors.go
git commit -m "feat(provider): SSE reader with stall timeout, typed retryable errors"
```

---

### Task 6: `anthropic-messages` adapter

**Files:**
- Create: `internal/provider/adapter.go`, `internal/provider/anthropic.go`
- Test: `internal/provider/anthropic_test.go`

**Interfaces:**
- Consumes: `Model` (Task 4), `readSSE`/`HTTPError`/`ErrContextOverflow` (Task 5), `llm.*`.
- Produces:
  - `type Adapter interface { Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) }`
  - `type Credential struct { Token string; OAuth bool }` and `type CredentialFunc func(ctx context.Context) (Credential, error)` (phase 7 plugs OAuth in here).
  - `func newAnthropic(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter`
  - Request mapping (fixed for later phases): system → one text block with `cache_control: {type:"ephemeral"}`; last tool spec gets `cache_control` too (end of tools+system prefix, §3); the last content block of the last message gets `cache_control` unless `req.NoCacheWrite`. With `NoCacheWrite`, no `cache_control` anywhere.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/anthropic_test.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

// sseServer replies with body as an event stream and captures the request.
func sseServer(t *testing.T, status int, body string, got *map[string]any, hdr *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, got)
		}
		if hdr != nil {
			*hdr = r.Header.Clone()
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func keyCred(k string) CredentialFunc {
	return func(context.Context) (Credential, error) { return Credential{Token: k}, nil }
}

const anthropicToolStream = `event: message_start
data: {"type":"message_start","message":{"usage":{"input_tokens":10,"cache_read_input_tokens":100,"cache_creation_input_tokens":5,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"hmm"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"SIG"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Let me "}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"look."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: content_block_start
data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"read","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}

event: ping
data: {"type":"ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"a.go\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":2}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":42}}

event: message_stop
data: {"type":"message_stop"}

`

func TestAnthropicStreamToolCall(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := sseServer(t, 200, anthropicToolStream, &body, &hdr)
	defer srv.Close()
	m := Model{Provider: "anthropic", ID: "claude-x", MaxOutput: 32000, ThinkingMode: "adaptive", ThinkingLevelMap: anthropicAdaptive}
	a := newAnthropic(m, srv.URL, keyCred("K"), srv.Client())

	var text strings.Builder
	var calls []*llm.ToolCall
	resp, err := a.Stream(context.Background(), llm.Request{
		Model: "claude-x", System: "sys", MaxTokens: 16384, Effort: llm.EffortMedium, ToolChoice: llm.ToolChoiceAuto,
		Tools:    []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}}},
	}, func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			text.WriteString(e.Text)
		case llm.EventToolCall:
			calls = append(calls, e.ToolCall)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "Let me look." || len(calls) != 1 || string(calls[0].Input) != `{"path":"a.go"}` {
		t.Fatalf("text %q calls %+v", text.String(), calls)
	}
	if resp.Stop != llm.StopToolUse {
		t.Fatalf("stop %s", resp.Stop)
	}
	if resp.Usage != (llm.Usage{Input: 10, Output: 42, CacheRead: 100, CacheWrite: 5}) {
		t.Fatalf("usage %+v", resp.Usage)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || th.Text != "hmm" || th.Signature != "SIG" || th.Model != "claude-x" {
		t.Fatalf("thinking block %+v", th)
	}
	// request shape
	if hdr.Get("x-api-key") != "K" || hdr.Get("anthropic-version") != "2023-06-01" {
		t.Fatal("auth headers")
	}
	if body["stream"] != true || body["max_tokens"].(float64) != 16384 {
		t.Fatalf("body %v", body)
	}
	th2 := body["thinking"].(map[string]any)
	if th2["type"] != "adaptive" || th2["display"] != "summarized" {
		t.Fatalf("thinking %v", th2)
	}
	if body["output_config"].(map[string]any)["effort"] != "medium" {
		t.Fatal("effort")
	}
	if _, has := th2["budget_tokens"]; has {
		t.Fatal("adaptive must not send budget_tokens")
	}
	sys := body["system"].([]any)[0].(map[string]any)
	if sys["cache_control"] == nil {
		t.Fatal("system breakpoint")
	}
	tools := body["tools"].([]any)
	if tools[len(tools)-1].(map[string]any)["cache_control"] == nil {
		t.Fatal("tools breakpoint")
	}
}

func TestAnthropicBudgetAndNoCache(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n", &body, nil)
	defer srv.Close()
	m := Model{Provider: "anthropic", ID: "haiku", MaxOutput: 64000, ThinkingMode: "budget", ThinkingLevelMap: anthropicBudget}
	a := newAnthropic(m, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "haiku", MaxTokens: 16384, Effort: llm.EffortHigh, NoCacheWrite: true,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "x"}}}}}, func(llm.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	th := body["thinking"].(map[string]any)
	if th["type"] != "enabled" || th["budget_tokens"].(float64) != 16384-4096 {
		t.Fatalf("budget thinking %v", th)
	}
	if strings.Contains(mustJSON(body), "cache_control") {
		t.Fatal("NoCacheWrite must remove every cache_control")
	}
	// effort off on a budget model that supports off → no thinking field
	body = nil
	a.Stream(context.Background(), llm.Request{Model: "haiku", MaxTokens: 1000, Effort: llm.EffortOff,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "x"}}}}}, func(llm.Event) {})
	if _, has := body["thinking"]; has {
		t.Fatal("off → omit thinking")
	}
}

func TestAnthropicHistoryMapping(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "event: message_stop\ndata: {}\n\n", &body, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "t", Signature: "S", Model: "m"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x/", IsError: true}}}},
	}
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 10, Messages: msgs}, func(llm.Event) {})
	got := mustJSON(body["messages"])
	for _, want := range []string{`"type":"thinking"`, `"signature":"S"`, `"type":"tool_use"`, `"id":"c1"`,
		`"type":"tool_result"`, `"tool_use_id":"c1"`, `"is_error":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("messages missing %s: %s", want, got)
		}
	}
}

func TestAnthropicErrors(t *testing.T) {
	srv := sseServer(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 210000 tokens > 200000 maximum"}}`, nil, nil)
	defer srv.Close()
	a := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if !errors.Is(err, ErrContextOverflow) {
		t.Fatalf("want overflow, got %v", err)
	}
	srv2 := sseServer(t, 200, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n", nil, nil)
	defer srv2.Close()
	a2 := newAnthropic(Model{ID: "m", ThinkingMode: "none"}, srv2.URL, keyCred("K"), srv2.Client())
	_, err = a2.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 529 {
		t.Fatalf("in-stream overloaded → 529, got %v", err)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run Anthropic` → FAIL.

- [ ] **Step 3: Implement `adapter.go`**

```go
// internal/provider/adapter.go
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/adeotek/moca/internal/llm"
)

// Adapter streams one model turn. emit receives deltas and completed tool
// calls in order; the returned Response holds the assembled message.
type Adapter interface {
	Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error)
}

type Credential struct {
	Token string
	OAuth bool // bearer token from `moca login` (phase 7)
}

type CredentialFunc func(ctx context.Context) (Credential, error)

// post sends a JSON body and returns the response on 2xx; non-2xx becomes *HTTPError.
func post(ctx context.Context, hc *http.Client, url string, hdr http.Header, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header = hdr
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "text/event-stream")
	resp, err := hc.Do(r)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		return nil, newHTTPError(resp)
	}
	return resp, nil
}
```

- [ ] **Step 4: Implement `anthropic.go`**

```go
// internal/provider/anthropic.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type anthropicAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newAnthropic(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &anthropicAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/v1/messages", cred: cred, hc: hc}
}

var ephemeral = map[string]string{"type": "ephemeral"}

func (a *anthropicAdapter) body(req llm.Request) map[string]any {
	cache := !req.NoCacheWrite
	b := map[string]any{"model": req.Model, "max_tokens": req.MaxTokens, "stream": true}
	if req.System != "" {
		sys := map[string]any{"type": "text", "text": req.System}
		if cache {
			sys["cache_control"] = ephemeral
		}
		b["system"] = []any{sys}
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.Schema}
		}
		if cache {
			tools[len(tools)-1]["cache_control"] = ephemeral
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = map[string]string{"type": string(req.ToolChoice)}
		}
	}
	msgs := make([]map[string]any, len(req.Messages))
	for i, m := range req.Messages {
		blocks := make([]map[string]any, 0, len(m.Content))
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				blocks = append(blocks, map[string]any{"type": "text", "text": c.Text})
			case llm.BlockThinking:
				blocks = append(blocks, map[string]any{"type": "thinking", "thinking": c.Text, "signature": c.Signature})
			case llm.BlockToolUse:
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": c.ToolCall.ID, "name": c.ToolCall.Name, "input": c.ToolCall.Input})
			case llm.BlockToolResult:
				blocks = append(blocks, map[string]any{"type": "tool_result", "tool_use_id": c.ToolResult.CallID,
					"content": c.ToolResult.Content, "is_error": c.ToolResult.IsError})
			}
		}
		if cache && i == len(req.Messages)-1 && len(blocks) > 0 {
			blocks[len(blocks)-1]["cache_control"] = ephemeral
		}
		msgs[i] = map[string]any{"role": string(m.Role), "content": blocks}
	}
	b["messages"] = msgs
	switch a.m.ThinkingMode {
	case "adaptive":
		b["thinking"] = map[string]any{"type": "adaptive", "display": "summarized"}
		b["output_config"] = map[string]any{"effort": a.m.ThinkingLevelMap[req.Effort]}
	case "budget":
		if req.Effort != llm.EffortOff {
			b["thinking"] = map[string]any{"type": "enabled", "budget_tokens": a.m.BudgetTokens(req.MaxTokens)}
		}
	}
	return b
}

type anthropicEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message struct {
		Usage anthropicUsage `json:"usage"`
	} `json:"message"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
	} `json:"content_block"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage anthropicUsage `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type anthropicUsage struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
}

func (a *anthropicAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	cred, err := a.cred(ctx)
	if err != nil {
		return llm.Response{}, err
	}
	hdr := http.Header{}
	hdr.Set("anthropic-version", "2023-06-01")
	if cred.OAuth {
		hdr.Set("Authorization", "Bearer "+cred.Token)
	} else {
		hdr.Set("x-api-key", cred.Token)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := post(ctx, a.hc, a.url, hdr, a.body(req))
	if err != nil {
		return llm.Response{}, anthropicOverflow(err)
	}
	defer resp.Body.Close()

	var out llm.Response
	out.Message.Role = llm.RoleAssistant
	blocks := map[int]*llm.ContentBlock{}
	partial := map[int]*strings.Builder{}
	var order []int
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		var ev anthropicEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return fmt.Errorf("anthropic: bad event: %w", err)
		}
		switch ev.Type {
		case "message_start":
			u := ev.Message.Usage
			out.Usage = llm.Usage{Input: u.Input, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Output: u.Output}
		case "content_block_start":
			cb := &llm.ContentBlock{}
			switch ev.ContentBlock.Type {
			case "text":
				cb.Type = llm.BlockText
			case "thinking", "redacted_thinking":
				cb.Type, cb.Model = llm.BlockThinking, req.Model
			case "tool_use":
				cb.Type = llm.BlockToolUse
				cb.ToolCall = &llm.ToolCall{ID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name}
				partial[ev.Index] = &strings.Builder{}
			default:
				return nil
			}
			blocks[ev.Index] = cb
			order = append(order, ev.Index)
		case "content_block_delta":
			cb := blocks[ev.Index]
			if cb == nil {
				return nil
			}
			switch ev.Delta.Type {
			case "text_delta":
				cb.Text += ev.Delta.Text
				emit(llm.Event{Type: llm.EventText, Text: ev.Delta.Text})
			case "thinking_delta":
				cb.Text += ev.Delta.Thinking
				emit(llm.Event{Type: llm.EventThinking, Text: ev.Delta.Thinking})
			case "signature_delta":
				cb.Signature += ev.Delta.Signature
			case "input_json_delta":
				partial[ev.Index].WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_stop":
			if cb := blocks[ev.Index]; cb != nil && cb.Type == llm.BlockToolUse {
				in := partial[ev.Index].String()
				if in == "" {
					in = "{}"
				}
				cb.ToolCall.Input = json.RawMessage(in)
				emit(llm.Event{Type: llm.EventToolCall, ToolCall: cb.ToolCall})
			}
		case "message_delta":
			out.Usage.Output = ev.Usage.Output
			out.Stop = anthropicStop(ev.Delta.StopReason)
		case "error":
			if ev.Error.Type == "overloaded_error" {
				return &HTTPError{Status: 529, Body: ev.Error.Message}
			}
			return fmt.Errorf("anthropic: %s: %s", ev.Error.Type, ev.Error.Message)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	for _, i := range order {
		out.Message.Content = append(out.Message.Content, *blocks[i])
	}
	return out, nil
}

func anthropicStop(s string) llm.StopReason {
	switch s {
	case "tool_use":
		return llm.StopToolUse
	case "max_tokens", "model_context_window_exceeded":
		return llm.StopLength
	case "refusal":
		return llm.StopRefusal
	}
	return llm.StopEnd
}

func anthropicOverflow(err error) error {
	var he *HTTPError
	if errors.As(err, &he) && he.Status == 400 && strings.Contains(he.Body, "prompt is too long") {
		return fmt.Errorf("%w: %s", ErrContextOverflow, he.Body)
	}
	return err
}
```

- [ ] **Step 5: Run** — `go test ./internal/provider/ -run Anthropic -race -v` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/adapter.go internal/provider/anthropic*.go
git commit -m "feat(provider): anthropic-messages streaming codec with thinking, tools, cache breakpoints"
```

---

### Task 7: `openai-completions` adapter

**Files:**
- Create: `internal/provider/openai_completions.go`
- Test: `internal/provider/openai_completions_test.go`

**Interfaces:**
- Consumes: `Adapter`, `CredentialFunc`, `post`, `readSSE`, `Model`.
- Produces: `func newOpenAICompletions(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter` — URL `baseURL + "/chat/completions"`. Thinking blocks from history are **not** sent on this protocol (it has no slot for them; §3's transform converts other-model thinking to text before the adapter sees it). Streamed `reasoning_content` / `reasoning` deltas become `EventThinking` and a thinking block with `Model = req.Model`, no signature.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/openai_completions_test.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

const completionsStream = `data: {"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"think"}}]}

data: {"choices":[{"index":0,"delta":{"content":"Hi "}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read","arguments":"{\"pa"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"ls","arguments":"{}"}}]}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"x\"}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: {"choices":[],"usage":{"prompt_tokens":120,"completion_tokens":30,"prompt_tokens_details":{"cached_tokens":100}}}

data: [DONE]

`

func TestCompletionsStream(t *testing.T) {
	var body map[string]any
	var hdr http.Header
	srv := sseServer(t, 200, completionsStream, &body, &hdr)
	defer srv.Close()
	m := Model{ID: "glm", ThinkingMode: "openai", ThinkingLevelMap: glmThinking}
	a := newOpenAICompletions(m, srv.URL, keyCred("K"), srv.Client())
	var calls []string
	resp, err := a.Stream(context.Background(), llm.Request{Model: "glm", System: "sys", MaxTokens: 100, Effort: llm.EffortHigh,
		ToolChoice: llm.ToolChoiceNone,
		Tools:      []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}}},
	}, func(e llm.Event) {
		if e.Type == llm.EventToolCall {
			calls = append(calls, e.ToolCall.ID+":"+string(e.ToolCall.Input))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != `call_a:{"path":"x"},call_b:{}` {
		t.Fatalf("calls in index order: %v", calls)
	}
	if resp.Stop != llm.StopToolUse || resp.Usage != (llm.Usage{Input: 20, Output: 30, CacheRead: 100}) {
		t.Fatalf("stop %s usage %+v", resp.Stop, resp.Usage)
	}
	if resp.Message.Content[0].Type != llm.BlockThinking || resp.Message.Content[1].Text != "Hi " {
		t.Fatalf("content %+v", resp.Message.Content)
	}
	if hdr.Get("Authorization") != "Bearer K" {
		t.Fatal("bearer")
	}
	if body["reasoning_effort"] != "high" || body["tool_choice"] != "none" || body["max_tokens"].(float64) != 100 {
		t.Fatalf("body %v", body)
	}
	if body["stream_options"].(map[string]any)["include_usage"] != true {
		t.Fatal("include_usage")
	}
	if body["messages"].([]any)[0].(map[string]any)["role"] != "system" {
		t.Fatal("system message first")
	}
}

func TestCompletionsHistoryMapping(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, "data: [DONE]\n\n", &body, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1, Messages: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "secret"},
			{Type: llm.BlockText, Text: "ok"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{"path":"."}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "a/"}},
			{Type: llm.BlockText, Text: "steer"}}},
	}}, func(llm.Event) {})
	got := mustJSON(body["messages"])
	if strings.Contains(got, "secret") {
		t.Fatal("thinking must not be sent on completions")
	}
	for _, want := range []string{`"tool_calls":[{"function":{"arguments":"{\"path\":\".\"}","name":"ls"},"id":"c1","type":"function"}]`,
		`{"content":"a/","role":"tool","tool_call_id":"c1"}`, `{"content":"steer","role":"user"}`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestCompletionsOverflow(t *testing.T) {
	srv := sseServer(t, 400, `{"error":{"code":"context_length_exceeded","message":"maximum context length is 8192"}}`, nil, nil)
	defer srv.Close()
	a := newOpenAICompletions(Model{ID: "m", ThinkingMode: "none"}, srv.URL, keyCred("K"), srv.Client())
	_, err := a.Stream(context.Background(), llm.Request{Model: "m", MaxTokens: 1}, func(llm.Event) {})
	if !errors.Is(err, ErrContextOverflow) {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run Completions` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/provider/openai_completions.go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type completionsAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newOpenAICompletions(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &completionsAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/chat/completions", cred: cred, hc: hc}
}

func (a *completionsAdapter) body(req llm.Request) map[string]any {
	var msgs []map[string]any
	if req.System != "" {
		msgs = append(msgs, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.Messages {
		var text strings.Builder
		var calls []map[string]any
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				text.WriteString(c.Text)
			case llm.BlockToolUse:
				calls = append(calls, map[string]any{"id": c.ToolCall.ID, "type": "function",
					"function": map[string]any{"name": c.ToolCall.Name, "arguments": string(c.ToolCall.Input)}})
			case llm.BlockToolResult:
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": c.ToolResult.CallID, "content": c.ToolResult.Content})
			}
		}
		if text.Len() == 0 && calls == nil {
			continue
		}
		msg := map[string]any{"role": string(m.Role), "content": text.String()}
		if calls != nil {
			msg["tool_calls"] = calls
		}
		msgs = append(msgs, msg)
	}
	b := map[string]any{"model": req.Model, "messages": msgs, "stream": true, "max_tokens": req.MaxTokens,
		"stream_options": map[string]any{"include_usage": true}}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.Schema}}
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = string(req.ToolChoice)
		}
	}
	if a.m.ThinkingMode == "openai" {
		if v := a.m.ThinkingLevelMap[req.Effort]; v != "" {
			b["reasoning_effort"] = v
		}
	}
	return b
}

type completionsChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		Prompt     int `json:"prompt_tokens"`
		Completion int `json:"completion_tokens"`
		Details    struct {
			Cached int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *completionsAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	cred, err := a.cred(ctx)
	if err != nil {
		return llm.Response{}, err
	}
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+cred.Token)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := post(ctx, a.hc, a.url, hdr, a.body(req))
	if err != nil {
		return llm.Response{}, openaiOverflow(err)
	}
	defer resp.Body.Close()

	var thinking, text strings.Builder
	type pending struct {
		call *llm.ToolCall
		args strings.Builder
	}
	calls := map[int]*pending{}
	out := llm.Response{Stop: llm.StopEnd}
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		if data == "[DONE]" {
			return nil
		}
		var ch completionsChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return fmt.Errorf("openai-completions: bad chunk: %w", err)
		}
		if ch.Error != nil {
			return fmt.Errorf("openai-completions: %s", ch.Error.Message)
		}
		if u := ch.Usage; u != nil {
			out.Usage = llm.Usage{Input: u.Prompt - u.Details.Cached, CacheRead: u.Details.Cached, Output: u.Completion}
		}
		for _, c := range ch.Choices {
			if r := c.Delta.ReasoningContent + c.Delta.Reasoning; r != "" {
				thinking.WriteString(r)
				emit(llm.Event{Type: llm.EventThinking, Text: r})
			}
			if c.Delta.Content != "" {
				text.WriteString(c.Delta.Content)
				emit(llm.Event{Type: llm.EventText, Text: c.Delta.Content})
			}
			for _, tc := range c.Delta.ToolCalls {
				p := calls[tc.Index]
				if p == nil {
					p = &pending{call: &llm.ToolCall{}}
					calls[tc.Index] = p
				}
				if tc.ID != "" {
					p.call.ID = tc.ID
				}
				if tc.Function.Name != "" {
					p.call.Name = tc.Function.Name
				}
				p.args.WriteString(tc.Function.Arguments)
			}
			switch c.FinishReason {
			case "tool_calls":
				out.Stop = llm.StopToolUse
			case "length":
				out.Stop = llm.StopLength
			case "content_filter":
				out.Stop = llm.StopRefusal
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	out.Message.Role = llm.RoleAssistant
	if thinking.Len() > 0 {
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockThinking, Text: thinking.String(), Model: req.Model})
	}
	if text.Len() > 0 {
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockText, Text: text.String()})
	}
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		p := calls[i]
		in := p.args.String()
		if in == "" {
			in = "{}"
		}
		p.call.Input = json.RawMessage(in)
		emit(llm.Event{Type: llm.EventToolCall, ToolCall: p.call})
		out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: p.call})
	}
	if len(calls) > 0 {
		out.Stop = llm.StopToolUse // some servers send finish_reason "stop" with tool calls
	}
	return out, nil
}

func openaiOverflow(err error) error {
	var he *HTTPError
	if errors.As(err, &he) && he.Status == 400 &&
		(strings.Contains(he.Body, "context_length_exceeded") || strings.Contains(he.Body, "maximum context length")) {
		return fmt.Errorf("%w: %s", ErrContextOverflow, he.Body)
	}
	return err
}
```

Tool calls on this protocol are emitted **after** the stream ends. Arguments are only complete once the stream finishes, and completion order across `index` values isn't guaranteed.

- [ ] **Step 4: Run** — `go test ./internal/provider/ -run Completions -race -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/openai_completions*.go
git commit -m "feat(provider): openai-completions streaming codec"
```

---

### Task 8: `openai-responses` adapter

**Files:**
- Create: `internal/provider/openai_responses.go`
- Test: `internal/provider/openai_responses_test.go`

**Interfaces:**
- Produces: `func newOpenAIResponses(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter` — URL `baseURL + "/responses"`; always `store: false` + `include: ["reasoning.encrypted_content"]`, so reasoning replays statelessly. Reasoning item → thinking block with `ThinkingID = item.id`, `Signature = encrypted_content`, `Text = joined summary text`, `Model = req.Model`.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/openai_responses_test.go
package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

const responsesStream = `event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"plan"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"reasoning","id":"rs_1","encrypted_content":"ENC","summary":[{"type":"summary_text","text":"plan"}]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.output_item.added
data: {"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_9","name":"read","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"path\":\"a\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_9","name":"read","arguments":"{\"path\":\"a\"}"}}

event: response.completed
data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":50,"output_tokens":7,"input_tokens_details":{"cached_tokens":40}}}}

`

func TestResponsesStream(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, responsesStream, &body, nil)
	defer srv.Close()
	m := Model{ID: "gpt", ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}
	a := newOpenAIResponses(m, srv.URL, keyCred("K"), srv.Client())
	var calls []*llm.ToolCall
	resp, err := a.Stream(context.Background(), llm.Request{Model: "gpt", System: "sys", MaxTokens: 500, Effort: llm.EffortLow,
		Tools:    []llm.ToolSpec{{Name: "read", Description: "d", Schema: json.RawMessage(`{"type":"object"}`)}},
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "q"}}}},
	}, func(e llm.Event) {
		if e.Type == llm.EventToolCall {
			calls = append(calls, e.ToolCall)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "call_9" || string(calls[0].Input) != `{"path":"a"}` {
		t.Fatalf("calls %+v", calls)
	}
	th := resp.Message.Content[0]
	if th.Type != llm.BlockThinking || th.ThinkingID != "rs_1" || th.Signature != "ENC" || th.Text != "plan" {
		t.Fatalf("reasoning %+v", th)
	}
	if resp.Stop != llm.StopToolUse || resp.Usage != (llm.Usage{Input: 10, Output: 7, CacheRead: 40}) {
		t.Fatalf("stop %s usage %+v", resp.Stop, resp.Usage)
	}
	if body["store"] != false || body["instructions"] != "sys" || body["max_output_tokens"].(float64) != 500 {
		t.Fatalf("body %v", body)
	}
	if body["reasoning"].(map[string]any)["effort"] != "low" {
		t.Fatal("effort")
	}
	if !strings.Contains(mustJSON(body["include"]), "reasoning.encrypted_content") {
		t.Fatal("include encrypted reasoning")
	}
}

func TestResponsesHistoryAndIncomplete(t *testing.T) {
	var body map[string]any
	srv := sseServer(t, 200, `event: response.incomplete
data: {"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{"input_tokens":1,"output_tokens":1}}}

`, &body, nil)
	defer srv.Close()
	a := newOpenAIResponses(Model{ID: "gpt", ThinkingMode: "openai", ThinkingLevelMap: openaiReasoning}, srv.URL, keyCred("K"), srv.Client())
	resp, err := a.Stream(context.Background(), llm.Request{Model: "gpt", MaxTokens: 1, Effort: llm.EffortLow, Messages: []llm.Message{
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, ThinkingID: "rs_1", Signature: "ENC", Model: "gpt"},
			{Type: llm.BlockText, Text: "ok"},
			{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "x"}}}},
	}}, func(llm.Event) {})
	if err != nil || resp.Stop != llm.StopLength {
		t.Fatalf("incomplete → length: %v %s", err, resp.Stop)
	}
	got := mustJSON(body["input"])
	for _, want := range []string{`"type":"reasoning"`, `"encrypted_content":"ENC"`, `"id":"rs_1"`,
		`"type":"output_text"`, `"type":"function_call"`, `"call_id":"c1"`, `"type":"function_call_output"`} {
		if !strings.Contains(got, want) {
			t.Errorf("input missing %s: %s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run Responses` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/provider/openai_responses.go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

type responsesAdapter struct {
	m    Model
	url  string
	cred CredentialFunc
	hc   *http.Client
}

func newOpenAIResponses(m Model, baseURL string, cred CredentialFunc, hc *http.Client) Adapter {
	return &responsesAdapter{m: m, url: strings.TrimRight(baseURL, "/") + "/responses", cred: cred, hc: hc}
}

func (a *responsesAdapter) body(req llm.Request) map[string]any {
	var input []map[string]any
	for _, m := range req.Messages {
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockText:
				typ := "input_text"
				if m.Role == llm.RoleAssistant {
					typ = "output_text"
				}
				input = append(input, map[string]any{"type": "message", "role": string(m.Role),
					"content": []map[string]any{{"type": typ, "text": c.Text}}})
			case llm.BlockThinking:
				if c.Signature == "" {
					continue // cannot replay reasoning without its encrypted payload
				}
				input = append(input, map[string]any{"type": "reasoning", "id": c.ThinkingID,
					"encrypted_content": c.Signature, "summary": []any{}})
			case llm.BlockToolUse:
				input = append(input, map[string]any{"type": "function_call", "call_id": c.ToolCall.ID,
					"name": c.ToolCall.Name, "arguments": string(c.ToolCall.Input)})
			case llm.BlockToolResult:
				input = append(input, map[string]any{"type": "function_call_output",
					"call_id": c.ToolResult.CallID, "output": c.ToolResult.Content})
			}
		}
	}
	b := map[string]any{"model": req.Model, "input": input, "stream": true, "store": false,
		"max_output_tokens": req.MaxTokens, "include": []string{"reasoning.encrypted_content"}}
	if req.System != "" {
		b["instructions"] = req.System
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"type": "function", "name": t.Name, "description": t.Description, "parameters": t.Schema}
		}
		b["tools"] = tools
		if req.ToolChoice != "" {
			b["tool_choice"] = string(req.ToolChoice)
		}
	}
	if a.m.ThinkingMode == "openai" {
		if v := a.m.ThinkingLevelMap[req.Effort]; v != "" {
			b["reasoning"] = map[string]any{"effort": v, "summary": "auto"}
		}
	}
	return b
}

type responsesEvent struct {
	Type   string `json:"type"`
	Delta  string `json:"delta"`
	ItemID string `json:"item_id"`
	Item   struct {
		Type             string `json:"type"`
		ID               string `json:"id"`
		CallID           string `json:"call_id"`
		Name             string `json:"name"`
		Arguments        string `json:"arguments"`
		EncryptedContent string `json:"encrypted_content"`
		Summary          []struct {
			Text string `json:"text"`
		} `json:"summary"`
	} `json:"item"`
	Response struct {
		Status            string `json:"status"`
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
		Usage struct {
			Input   int `json:"input_tokens"`
			Output  int `json:"output_tokens"`
			Details struct {
				Cached int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"response"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (a *responsesAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	cred, err := a.cred(ctx)
	if err != nil {
		return llm.Response{}, err
	}
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+cred.Token)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := post(ctx, a.hc, a.url, hdr, a.body(req))
	if err != nil {
		return llm.Response{}, openaiOverflow(err)
	}
	defer resp.Body.Close()

	out := llm.Response{Stop: llm.StopEnd}
	out.Message.Role = llm.RoleAssistant
	var text strings.Builder
	flushText := func() {
		if text.Len() > 0 {
			out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockText, Text: text.String()})
			text.Reset()
		}
	}
	hasCalls := false
	err = readSSE(ctx, resp.Body, stallTimeout, func(_, data string) error {
		var ev responsesEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return fmt.Errorf("openai-responses: bad event: %w", err)
		}
		switch ev.Type {
		case "response.output_text.delta":
			text.WriteString(ev.Delta)
			emit(llm.Event{Type: llm.EventText, Text: ev.Delta})
		case "response.reasoning_summary_text.delta":
			emit(llm.Event{Type: llm.EventThinking, Text: ev.Delta})
		case "response.output_item.done":
			switch ev.Item.Type {
			case "reasoning":
				flushText()
				var sum []string
				for _, s := range ev.Item.Summary {
					sum = append(sum, s.Text)
				}
				out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockThinking,
					Text: strings.Join(sum, "\n"), ThinkingID: ev.Item.ID, Signature: ev.Item.EncryptedContent, Model: req.Model})
			case "function_call":
				flushText()
				args := ev.Item.Arguments
				if args == "" {
					args = "{}"
				}
				call := &llm.ToolCall{ID: ev.Item.CallID, Name: ev.Item.Name, Input: json.RawMessage(args)}
				emit(llm.Event{Type: llm.EventToolCall, ToolCall: call})
				out.Message.Content = append(out.Message.Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: call})
				hasCalls = true
			case "message":
				flushText()
			}
		case "response.completed", "response.incomplete":
			u := ev.Response.Usage
			out.Usage = llm.Usage{Input: u.Input - u.Details.Cached, CacheRead: u.Details.Cached, Output: u.Output}
			if ev.Response.IncompleteDetails.Reason == "max_output_tokens" {
				out.Stop = llm.StopLength
			}
		case "response.failed":
			if e := ev.Response.Error; e != nil {
				if e.Code == "context_length_exceeded" {
					return fmt.Errorf("%w: %s", ErrContextOverflow, e.Message)
				}
				if e.Code == "server_error" || e.Code == "rate_limit_exceeded" {
					return &HTTPError{Status: 500, Body: e.Message}
				}
				return fmt.Errorf("openai-responses: %s: %s", e.Code, e.Message)
			}
			return fmt.Errorf("openai-responses: response failed")
		case "error":
			return fmt.Errorf("openai-responses: %s: %s", ev.Code, ev.Message)
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	flushText()
	if hasCalls && out.Stop == llm.StopEnd {
		out.Stop = llm.StopToolUse
	}
	return out, nil
}
```

- [ ] **Step 4: Run** — `go test ./internal/provider/ -run Responses -race -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/openai_responses*.go
git commit -m "feat(provider): openai-responses streaming codec with stateless reasoning replay"
```

---

### Task 9: Retry wrapper

**Files:**
- Create: `internal/provider/retry.go`
- Test: `internal/provider/retry_test.go`

**Interfaces:**
- Consumes: `Adapter`, `retryable`, `HTTPError`.
- Produces:
  - `type RetryNotice struct { Attempt, Max int; Wait time.Duration; Err error }`
  - `type RetryPolicy struct { Delays []time.Duration; Sleep func(ctx context.Context, d time.Duration) error; Jitter func(d time.Duration) time.Duration; Notify func(RetryNotice) }`
  - `func DefaultRetryPolicy(notify func(RetryNotice)) RetryPolicy` — delays 1,2,4,8,16s; jitter ±20%; ctx-aware sleep.
  - `func WithRetry(a Adapter, p RetryPolicy) Adapter`
  - Semantics: up to `len(Delays)` retries **before** any event was emitted (`retry-after` overrides the computed delay when larger). Once an event was emitted and the stream fails, emit `EventReset`, retry the same request **once**, then surface the error. Non-retryable errors surface immediately.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/retry_test.go
package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

type scripted struct {
	steps []func(emit func(llm.Event)) error
	n     int
}

func (s *scripted) Stream(_ context.Context, _ llm.Request, emit func(llm.Event)) (llm.Response, error) {
	f := s.steps[s.n]
	s.n++
	return llm.Response{Stop: llm.StopEnd}, f(emit)
}

func testPolicy(slept *[]time.Duration, notes *[]RetryNotice) RetryPolicy {
	return RetryPolicy{
		Delays: []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
		Sleep:  func(_ context.Context, d time.Duration) error { *slept = append(*slept, d); return nil },
		Jitter: func(d time.Duration) time.Duration { return d },
		Notify: func(n RetryNotice) { *notes = append(*notes, n) },
	}
}

func fail(err error) func(func(llm.Event)) error { return func(func(llm.Event)) error { return err } }
func ok(func(llm.Event)) error                   { return nil }

func TestRetryBackoffThenSuccess(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 429}), fail(&HTTPError{Status: 503}), ok}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if err != nil || s.n != 3 {
		t.Fatalf("err %v calls %d", err, s.n)
	}
	if len(slept) != 2 || slept[0] != time.Second || slept[1] != 2*time.Second {
		t.Fatalf("slept %v", slept)
	}
	if notes[1].Attempt != 2 || notes[1].Max != 5 {
		t.Fatalf("notice %+v", notes[1])
	}
}

func TestRetryHonoursRetryAfter(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 429, RetryAfter: 7 * time.Second}), ok}}
	WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if slept[0] != 7*time.Second {
		t.Fatalf("retry-after ignored: %v", slept)
	}
}

func TestRetryGivesUpAfterFive(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	e := &HTTPError{Status: 500}
	s := &scripted{steps: []func(func(llm.Event)) error{fail(e), fail(e), fail(e), fail(e), fail(e), fail(e)}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if !errors.Is(err, e) || s.n != 6 {
		t.Fatalf("err %v calls %d (1 + 5 retries)", err, s.n)
	}
}

func TestNoRetryOn400(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	s := &scripted{steps: []func(func(llm.Event)) error{fail(&HTTPError{Status: 400})}}
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(llm.Event) {})
	if err == nil || s.n != 1 {
		t.Fatal("400 must not retry")
	}
}

func TestMidStreamFailureResetsAndRetriesOnce(t *testing.T) {
	var slept []time.Duration
	var notes []RetryNotice
	partial := func(emit func(llm.Event)) error {
		emit(llm.Event{Type: llm.EventText, Text: "par"})
		return ErrStall
	}
	s := &scripted{steps: []func(func(llm.Event)) error{partial, partial, ok}}
	var evs []llm.EventType
	_, err := WithRetry(s, testPolicy(&slept, &notes)).Stream(context.Background(), llm.Request{}, func(e llm.Event) { evs = append(evs, e.Type) })
	if !errors.Is(err, ErrStall) || s.n != 2 {
		t.Fatalf("second mid-stream failure must surface: err %v calls %d", err, s.n)
	}
	if len(evs) != 3 || evs[1] != llm.EventReset {
		t.Fatalf("events %v: want text, reset, text", evs)
	}
}

func TestRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := DefaultRetryPolicy(nil)
	s := &scripted{steps: []func(func(llm.Event)) error{
		func(func(llm.Event)) error { cancel(); return &HTTPError{Status: 500} }, ok}}
	_, err := WithRetry(s, p).Stream(ctx, llm.Request{}, func(llm.Event) {})
	if !errors.Is(err, context.Canceled) || s.n != 1 {
		t.Fatalf("cancel during backoff: err %v calls %d", err, s.n)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run Retry` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/provider/retry.go
package provider

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

type RetryNotice struct {
	Attempt, Max int
	Wait         time.Duration
	Err          error
}

type RetryPolicy struct {
	Delays []time.Duration
	Sleep  func(ctx context.Context, d time.Duration) error
	Jitter func(d time.Duration) time.Duration
	Notify func(RetryNotice)
}

func DefaultRetryPolicy(notify func(RetryNotice)) RetryPolicy {
	return RetryPolicy{
		Delays: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second},
		Sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
		Jitter: func(d time.Duration) time.Duration { return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64())) },
		Notify: notify,
	}
}

type retrying struct {
	a Adapter
	p RetryPolicy
}

func WithRetry(a Adapter, p RetryPolicy) Adapter { return &retrying{a, p} }

func (r *retrying) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	attempt, midRetried := 0, false
	for {
		streamed := false
		resp, err := r.a.Stream(ctx, req, func(e llm.Event) { streamed = true; emit(e) })
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil {
			return resp, ctx.Err()
		}
		if !retryable(err) {
			return resp, err
		}
		if streamed {
			if midRetried {
				return resp, err
			}
			midRetried = true
			emit(llm.Event{Type: llm.EventReset})
			continue
		}
		if attempt >= len(r.p.Delays) {
			return resp, err
		}
		wait := r.p.Jitter(r.p.Delays[attempt])
		if he, ok := err.(*HTTPError); ok && he.RetryAfter > wait {
			wait = he.RetryAfter
		}
		attempt++
		if r.p.Notify != nil {
			r.p.Notify(RetryNotice{Attempt: attempt, Max: len(r.p.Delays), Wait: wait, Err: err})
		}
		if err := r.p.Sleep(ctx, wait); err != nil {
			return resp, err
		}
	}
}
```

- [ ] **Step 4: Run** — `go test ./internal/provider/ -run Retry -race -v` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/retry*.go
git commit -m "feat(provider): uniform retry with backoff, retry-after, mid-stream reset-and-retry-once"
```

---

### Task 10: Provider registry — config + catalog → `(Model, Adapter)`

**Files:**
- Create: `internal/provider/registry.go`
- Test: `internal/provider/registry_test.go`

**Interfaces:**
- Consumes: `config.Config`, `config.SplitModel`, `config.ResolveEnv`, catalog, adapters, `WithRetry`.
- Produces:
  - `type Registry struct { … }`
  - `func NewRegistry(cfg config.Config, hc *http.Client, notify func(RetryNotice)) (*Registry, error)` — validates that `cfg.Model` / `cfg.ModelHard` resolve.
  - `func (r *Registry) Resolve(qualified string) (Model, Adapter, error)` — adapter already wrapped with retry.
  - `func (r *Registry) Models() []Model` — sorted, for `/model` completion (phase 3).
  - `func (r *Registry) SetOAuth(provider string, fn CredentialFunc)` — phase 7 hook.
  - Default base URLs: `anthropic` → `https://api.anthropic.com`; `openai` → `https://api.openai.com/v1`; `opencode-go` → `{"anthropic-messages": "https://opencode.ai/zen/go", "openai-completions": "https://opencode.ai/zen/go/v1", "openai-responses": "https://opencode.ai/zen/go/v1"}`.
  - URL precedence: `baseUrls[protocol]` > `baseUrl` > built-in default.

- [ ] **Step 1: Write failing tests**

```go
// internal/provider/registry_test.go
package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

func mustCfg(t *testing.T, s string) config.Config {
	t.Helper()
	c, err := config.Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestResolveBuiltin(t *testing.T) {
	r, err := NewRegistry(mustCfg(t, `{"model":"opencode-go/glm-5.3-flash"}`), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, a, err := r.Resolve("opencode-go/glm-5.3-flash")
	if err != nil || a == nil || m.ContextWindow == 0 {
		t.Fatal(m, err)
	}
}

func TestUnknownModel(t *testing.T) {
	_, err := NewRegistry(mustCfg(t, `{"model":"anthropic/nope-9"}`), http.DefaultClient, nil)
	if err == nil || !strings.Contains(err.Error(), "anthropic/nope-9") {
		t.Fatalf("%v", err)
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	r, _ := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY",
		"baseUrl":"http://all","baseUrls":{"anthropic-messages":"http://anth"}}}}`), http.DefaultClient, nil)
	if got := r.baseURL("opencode-go", "anthropic-messages"); got != "http://anth" {
		t.Fatal(got)
	}
	if got := r.baseURL("opencode-go", "openai-completions"); got != "http://all" {
		t.Fatal(got)
	}
	r2, _ := NewRegistry(config.Default(), http.DefaultClient, nil)
	if got := r2.baseURL("opencode-go", "openai-responses"); got != "https://opencode.ai/zen/go/v1" {
		t.Fatal(got)
	}
}

func TestCustomProviderAndLazyMissingKey(t *testing.T) {
	var gotAuth string
	srv := sseServer(t, 200, "data: [DONE]\n\n", nil, nil)
	defer srv.Close()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\n\n"))
	})
	cfg := mustCfg(t, `{"model":"vllm/qwen","providers":{"vllm":{"baseUrl":"`+srv.URL+`/v1","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_T_VLLM","models":{"qwen":{"contextWindow":32768}}}}}`)
	r, err := NewRegistry(cfg, srv.Client(), nil)
	if err != nil {
		t.Fatal("missing key must not fail registry construction:", err)
	}
	_, a, err := r.Resolve("vllm/qwen")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCA_T_VLLM", "")
	_, err = a.Stream(context.Background(), llm.Request{Model: "qwen", MaxTokens: 1}, func(llm.Event) {})
	if err == nil || !strings.Contains(err.Error(), "MOCA_T_VLLM") {
		t.Fatalf("missing key error must name the variable: %v", err)
	}
	t.Setenv("MOCA_T_VLLM", "sekret")
	if _, err = a.Stream(context.Background(), llm.Request{Model: "qwen", MaxTokens: 1}, func(llm.Event) {}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sekret" {
		t.Fatal(gotAuth)
	}
}

func TestOverrideBuiltinModelFields(t *testing.T) {
	r, _ := NewRegistry(mustCfg(t, `{"providers":{"opencode-go":{"auth":"api_key","apiKey":"env:OPENCODE_API_KEY",
		"models":{"glm-5.3":{"contextWindow":200000}}}}}`), http.DefaultClient, nil)
	m, _, _ := r.Resolve("opencode-go/glm-5.3")
	if m.ContextWindow != 200000 || m.Cost.Input != 1.4 {
		t.Fatalf("override merges, not replaces: %+v", m)
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/provider/ -run 'Resolve|Unknown|BaseURL|Custom|Override'` → FAIL.

- [ ] **Step 3: Implement**

```go
// internal/provider/registry.go
package provider

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/config"
)

var defaultBaseURLs = map[string]map[string]string{
	"anthropic": {"anthropic-messages": "https://api.anthropic.com"},
	"openai":    {"openai-responses": "https://api.openai.com/v1", "openai-completions": "https://api.openai.com/v1"},
	"opencode-go": {
		"anthropic-messages": "https://opencode.ai/zen/go",
		"openai-completions": "https://opencode.ai/zen/go/v1",
		"openai-responses":   "https://opencode.ai/zen/go/v1",
	},
}

type Registry struct {
	cfg    config.Config
	hc     *http.Client
	notify func(RetryNotice)
	models map[string]Model
	oauth  map[string]CredentialFunc
}

func NewRegistry(cfg config.Config, hc *http.Client, notify func(RetryNotice)) (*Registry, error) {
	r := &Registry{cfg: cfg, hc: hc, notify: notify, models: map[string]Model{}, oauth: map[string]CredentialFunc{}}
	for _, m := range builtinCatalog {
		r.models[m.Qualified()] = m
	}
	for pname, p := range cfg.Providers {
		for mid, o := range p.Models {
			q := pname + "/" + mid
			m, ok := r.models[q]
			if !ok {
				m = Model{Provider: pname, ID: mid, Protocol: p.Protocol, ThinkingMode: "none", MaxOutput: 8192}
			}
			if o.Protocol != "" {
				m.Protocol = o.Protocol
			}
			if o.ContextWindow != 0 {
				m.ContextWindow = o.ContextWindow
			}
			if o.MaxOutputTokens != 0 {
				m.MaxOutput = o.MaxOutputTokens
			}
			if o.Cost != nil {
				m.Cost = *o.Cost
			}
			r.models[q] = m
		}
	}
	for _, q := range []string{cfg.Model, cfg.ModelHard} {
		if q == "" {
			continue
		}
		if _, ok := r.models[q]; !ok {
			return nil, fmt.Errorf("model %q is not in the catalog; declare it under providers.<name>.models", q)
		}
	}
	return r, nil
}

func (r *Registry) SetOAuth(provider string, fn CredentialFunc) { r.oauth[provider] = fn }

func (r *Registry) Models() []Model {
	out := make([]Model, 0, len(r.models))
	for _, m := range r.models {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b Model) int { return strings.Compare(a.Qualified(), b.Qualified()) })
	return out
}

func (r *Registry) baseURL(provider, protocol string) string {
	p := r.cfg.Providers[provider]
	if u := p.BaseURLs[protocol]; u != "" {
		return u
	}
	if p.BaseURL != "" {
		return p.BaseURL
	}
	return defaultBaseURLs[provider][protocol]
}

func (r *Registry) credential(provider string) CredentialFunc {
	p := r.cfg.Providers[provider]
	if p.Auth == "oauth" {
		if fn := r.oauth[provider]; fn != nil {
			return fn
		}
		return func(context.Context) (Credential, error) {
			return Credential{}, fmt.Errorf("provider %s uses oauth: run `moca login %s`", provider, provider)
		}
	}
	return func(context.Context) (Credential, error) {
		v, err := config.ResolveEnv(p.APIKey)
		if err != nil {
			return Credential{}, fmt.Errorf("provider %s: %w", provider, err)
		}
		return Credential{Token: v}, nil
	}
}

func (r *Registry) Resolve(qualified string) (Model, Adapter, error) {
	pname, _, err := config.SplitModel(qualified)
	if err != nil {
		return Model{}, nil, err
	}
	m, ok := r.models[qualified]
	if !ok {
		return Model{}, nil, fmt.Errorf("unknown model %q", qualified)
	}
	if _, ok := r.cfg.Providers[pname]; !ok {
		return Model{}, nil, fmt.Errorf("unknown provider %q", pname)
	}
	base := r.baseURL(pname, m.Protocol)
	if base == "" {
		return Model{}, nil, fmt.Errorf("provider %s has no base URL for protocol %s", pname, m.Protocol)
	}
	cred := r.credential(pname)
	var a Adapter
	switch m.Protocol {
	case "anthropic-messages":
		a = newAnthropic(m, base, cred, r.hc)
	case "openai-completions":
		a = newOpenAICompletions(m, base, cred, r.hc)
	case "openai-responses":
		a = newOpenAIResponses(m, base, cred, r.hc)
	default:
		return Model{}, nil, fmt.Errorf("model %s: unknown protocol %q", qualified, m.Protocol)
	}
	return m, WithRetry(a, DefaultRetryPolicy(r.notify)), nil
}
```

`ErrMissingKey` isn't a separate sentinel. `cmd/moca` maps errors from the credential path to exit code 2 by checking for `*config.EnvError`. Add that type now. In `internal/config/env.go`, replace the `fmt.Errorf` in `ResolveEnv` with:

```go
// EnvError reports an unset env: reference — a configuration error (exit 2).
type EnvError struct{ Name, Ref string }

func (e *EnvError) Error() string {
	return fmt.Sprintf("environment variable %s is not set (referenced as %q)", e.Name, e.Ref)
}
```

and `return "", &EnvError{Name: name, Ref: ref}`. Re-run `go test ./internal/config/` — `TestResolveEnv` still passes (the message is unchanged).

- [ ] **Step 4: Run** — `go test ./internal/... -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/provider/registry*.go internal/config/env.go
git commit -m "feat(provider): registry resolving qualified models to retrying adapters"
```

---

### Task 11: CLI + `-p` one-shot + exit codes

**Files:**
- Create: `cmd/moca/main.go`, `cmd/moca/cli.go`, `cmd/moca/oneshot.go`
- Test: `cmd/moca/cli_test.go`

**Interfaces:**
- Consumes: `config.Load/ConfigFile/SplitModel/EnvError/Version`, `provider.NewRegistry/Resolve/RetryNotice`, `llm.*`.
- Produces (extended in phases 2–7, names final):
  - `type Options struct { Prompt string; OneShot bool; Model string; Effort string; Approve *bool; Yolo *bool; Resume string; Continue bool; Version bool; Sub []string; ConfigPath string }`
  - `func parseArgs(args []string, stdin io.Reader) (Options, error)` — `-p -` reads the prompt from stdin; `--approve`/`--no-approve` mutually exclusive; `--yolo`/`--no-yolo` mutually exclusive (`nil` = use config `yolo`); subcommands `login|logout|mcp` collected into `Sub`.
  - `func (o Options) YoloOn(cfg config.Config) bool` — `*o.Yolo` if set, else `cfg.Yolo`. Phase 1 only parses it; phase 2 gives it effect.
  - `func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int`
  - Exit constants: `exitOK=0, exitRuntime=1, exitUsage=2, exitMaxSteps=3, exitInterrupted=130`.
  - Phase-1 system prompt: `"You are moca, a coding agent."` (replaced by the real prompt builder in phase 2).
  - Phase-1 TUI path: `moca` with no `-p` prints `TUI lands in phase 3; use -p` to stderr, exit 2.

- [ ] **Step 1: Write failing tests**

```go
// cmd/moca/cli_test.go
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func TestParseArgs(t *testing.T) {
	o, err := parseArgs([]string{"-p", "hi", "--model", "openai/x", "--effort", "high", "--no-approve"}, nil)
	if err != nil || !o.OneShot || o.Prompt != "hi" || o.Model != "openai/x" || o.Effort != "high" || o.Approve == nil || *o.Approve {
		t.Fatalf("%+v %v", o, err)
	}
	o, _ = parseArgs([]string{"-p", "-"}, strings.NewReader("from stdin\n"))
	if o.Prompt != "from stdin" {
		t.Fatalf("stdin prompt %q", o.Prompt)
	}
	if _, err := parseArgs([]string{"--approve", "--no-approve", "-p", "x"}, nil); err == nil {
		t.Fatal("approve flags are exclusive")
	}
	if _, err := parseArgs([]string{"--yolo", "--no-yolo", "-p", "x"}, nil); err == nil {
		t.Fatal("yolo flags are exclusive")
	}
	o, _ = parseArgs([]string{"--no-yolo", "-p", "x"}, nil)
	if o.Yolo == nil || *o.Yolo || o.YoloOn(config.Config{Yolo: true}) {
		t.Fatal("--no-yolo overrides config")
	}
	o, _ = parseArgs([]string{"-p", "x"}, nil)
	if o.YoloOn(config.Config{}) || !o.YoloOn(config.Config{Yolo: true}) {
		t.Fatal("no flag → config decides")
	}
	if _, err := parseArgs([]string{"--effort", "med", "-p", "x"}, nil); err == nil {
		t.Fatal("bad effort is a usage error")
	}
	o, _ = parseArgs([]string{"mcp", "import"}, nil)
	if strings.Join(o.Sub, " ") != "mcp import" {
		t.Fatal(o.Sub)
	}
}

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.jsonc")
	os.WriteFile(p, []byte(body), 0o600)
	return p
}

func fakeCompletions(t *testing.T, status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

func TestOneShotStreamsToStdout(t *testing.T) {
	srv := fakeCompletions(t, 200, "data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	code := run(context.Background(), []string{"--config", cfg, "-p", "hi"}, nil, &out, &errb)
	if code != 0 || out.String() != "hello\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "tokens 3/2") || !strings.Contains(errb.String(), "$") {
		t.Fatalf("exit summary missing: %q", errb.String())
	}
}

func TestExitCodes(t *testing.T) {
	t.Setenv("MOCA_T_KEY", "")
	cases := []struct {
		name string
		cfg  string
		args []string
		want int
	}{
		{"bare model", `{}`, []string{"-p", "x", "--model", "glm"}, 2},
		{"unknown provider", `{}`, []string{"-p", "x", "--model", "foo/bar"}, 2},
		{"bad config", `{"model": }`, []string{"-p", "x"}, 2},
		{"missing key", `{"model":"loc/m","providers":{"loc":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`, []string{"-p", "x"}, 2},
		{"no model at all", `{}`, []string{"-p", "x"}, 2},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		if got := run(context.Background(), append([]string{"--config", writeCfg(t, c.cfg)}, c.args...), nil, &out, &errb); got != c.want {
			t.Errorf("%s: exit %d want %d (stderr %q)", c.name, got, c.want, errb.String())
		}
	}
}

func TestProviderErrorExit1(t *testing.T) {
	srv := fakeCompletions(t, 401, `{"error":{"message":"bad key"}}`)
	defer srv.Close()
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errb.String(), "bad key") {
		t.Fatal(errb.String())
	}
}

func TestInterruptedExit130(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-block
	}))
	defer srv.Close()
	defer close(block)
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, `{"model":"loc/m","providers":{"loc":{"baseUrl":"`+srv.URL+`","protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768}}}}}`)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	var out, errb bytes.Buffer
	if code := run(ctx, []string{"--config", cfg, "-p", "x"}, nil, &out, &errb); code != 130 {
		t.Fatalf("exit %d", code)
	}
}
```

`--config <path>` is a hidden test/dev flag (not in §12.5's user-facing list). It overrides `config.ConfigFile()`.

- [ ] **Step 2: Run** — `go test ./cmd/moca/` → FAIL.

- [ ] **Step 3: Implement `cli.go`**

```go
// cmd/moca/cli.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

const (
	exitOK          = 0
	exitRuntime     = 1
	exitUsage       = 2
	exitMaxSteps    = 3
	exitInterrupted = 130
)

type Options struct {
	Prompt     string
	OneShot    bool
	Model      string
	Effort     string
	Approve    *bool
	Yolo       *bool
	Resume     string
	Continue   bool
	Version    bool
	Sub        []string
	ConfigPath string
}

type usageError struct{ error }

// YoloOn: the flag wins; otherwise config `yolo` decides (§7.5).
func (o Options) YoloOn(cfg config.Config) bool {
	if o.Yolo != nil {
		return *o.Yolo
	}
	return cfg.Yolo
}

func parseArgs(args []string, stdin io.Reader) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("moca", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.Prompt, "p", "", "one-shot prompt (- = stdin)")
	fs.StringVar(&o.Model, "model", "", "provider/model")
	fs.StringVar(&o.Effort, "effort", "", "effort level")
	approve := fs.Bool("approve", false, "trust project resources for this run")
	noApprove := fs.Bool("no-approve", false, "skip project resources for this run")
	yolo := fs.Bool("yolo", false, "turn all permission checks off for this run")
	noYolo := fs.Bool("no-yolo", false, "keep permission checks on (overrides config yolo)")
	fs.StringVar(&o.Resume, "resume", "", "resume session id8 | last")
	fs.BoolVar(&o.Continue, "continue", false, "resume latest session in this workdir")
	fs.BoolVar(&o.Version, "version", false, "print version")
	fs.StringVar(&o.ConfigPath, "config", "", "config file (dev/test)")
	if err := fs.Parse(args); err != nil {
		return o, usageError{err}
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "p" {
			o.OneShot = true
		}
	})
	if *approve && *noApprove {
		return o, usageError{errors.New("--approve and --no-approve are mutually exclusive")}
	}
	if *approve || *noApprove {
		v := *approve
		o.Approve = &v
	}
	if *yolo && *noYolo {
		return o, usageError{errors.New("--yolo and --no-yolo are mutually exclusive")}
	}
	if *yolo || *noYolo {
		v := *yolo
		o.Yolo = &v
	}
	if o.Effort != "" {
		if _, err := llm.ParseEffort(o.Effort); err != nil {
			return o, usageError{err}
		}
	}
	if o.OneShot && o.Prompt == "-" {
		if stdin == nil {
			return o, usageError{errors.New("-p - needs stdin")}
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return o, err
		}
		o.Prompt = strings.TrimRight(string(b), "\n")
	}
	if o.OneShot && strings.TrimSpace(o.Prompt) == "" {
		return o, usageError{errors.New("-p needs a non-empty prompt")}
	}
	o.Sub = fs.Args()
	if len(o.Sub) > 0 && !(o.Sub[0] == "login" || o.Sub[0] == "logout" || o.Sub[0] == "mcp") {
		return o, usageError{fmt.Errorf("unknown command %q", o.Sub[0])}
	}
	return o, nil
}
```

- [ ] **Step 4: Implement `oneshot.go` and `main.go`**

```go
// cmd/moca/oneshot.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

func exitFor(ctx context.Context, err error) int {
	var ee *config.EnvError
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return exitInterrupted
	case errors.As(err, &ee):
		return exitUsage
	default:
		return exitRuntime
	}
}

func runOneShot(ctx context.Context, o Options, cfg config.Config, stdout, stderr io.Writer) int {
	reg, err := provider.NewRegistry(cfg, http.DefaultClient, func(n provider.RetryNotice) {
		fmt.Fprintf(stderr, "retry %d/%d · %s (%v)\n", n.Attempt, n.Max, n.Wait.Round(100_000_000), n.Err)
	})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	m, adapter, err := reg.Resolve(cfg.Model)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	effort := m.DefaultEffort()
	if o.Effort != "" {
		e, _ := llm.ParseEffort(o.Effort)
		effort = m.ClampEffort(e)
	}
	req := llm.Request{
		Model:     m.ID,
		System:    "You are moca, a coding agent.",
		MaxTokens: m.MaxTokens(cfg.Context.ReserveTokens),
		Effort:    effort,
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: o.Prompt}}}},
	}
	resp, err := adapter.Stream(ctx, req, func(e llm.Event) {
		switch e.Type {
		case llm.EventText:
			io.WriteString(stdout, e.Text)
		case llm.EventReset:
			fmt.Fprintln(stderr, "\n[stream interrupted — retrying]")
		}
	})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitFor(ctx, err)
	}
	io.WriteString(stdout, "\n")
	u := resp.Usage
	fmt.Fprintf(stderr, "tokens %d/%d · $%.4f\n", u.Input+u.CacheRead+u.CacheWrite, u.Output, m.CostOf(u))
	return exitOK
}
```

**Known phase-1 gap (resolved in phase 2):** an `EventReset` retry re-streams text that is already on stdout. Phase 2 buffers each turn's text and prints only the final message, which removes the duplication.

```go
// cmd/moca/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/adeotek/moca/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parseArgs(args, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		var ue usageError
		if errors.As(err, &ue) {
			return exitUsage
		}
		return exitRuntime
	}
	if o.Version {
		fmt.Fprintln(stdout, "moca", config.Version)
		return exitOK
	}
	path := o.ConfigPath
	if path == "" {
		path = config.ConfigFile()
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	if o.Model != "" {
		cfg.Model = o.Model
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(stderr, "moca:", err)
			return exitUsage
		}
	}
	if cfg.Model == "" {
		fmt.Fprintf(stderr, "moca: no model configured — set \"model\" in %s or pass --model provider/model\n", path)
		return exitUsage
	}
	if len(o.Sub) > 0 {
		fmt.Fprintf(stderr, "moca: %s lands in a later phase\n", o.Sub[0])
		return exitUsage
	}
	if !o.OneShot {
		fmt.Fprintln(stderr, "moca: TUI lands in phase 3; use -p")
		return exitUsage
	}
	return runOneShot(ctx, o, cfg, stdout, stderr)
}
```

- [ ] **Step 5: Run** — `go test ./... -race && go vet ./...` → PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/moca
git commit -m "feat(cli): moca -p one-shot streaming with exit-code contract"
```

---

### Task 12: Live phase gate + README

**Files:**
- Modify: `README.md` (status line)

- [ ] **Step 1: Build**

Run: `go build -o bin/moca ./cmd/moca && ./bin/moca --version`
Expected: `moca 0.0.0-dev`. Add `bin/` to `.gitignore` if it isn't already listed.

- [ ] **Step 2: Gate — stream via all three providers and both opencode-go protocol families**

With real keys exported (`ANTHROPIC_API_KEY`, `OPENCODE_API_KEY`, `OPENAI_API_KEY`), run each and confirm text streams incrementally on stdout and the exit summary appears on stderr:

```bash
./bin/moca -p 'hi' --model anthropic/<a catalog claude id>
./bin/moca -p 'hi' --model opencode-go/<a catalog model whose protocol is anthropic-messages>
./bin/moca -p 'hi' --model opencode-go/glm-5.3-flash      # openai-* family
./bin/moca -p 'hi' --model openai/<a catalog openai id>
echo 'hi' | ./bin/moca -p - --model opencode-go/glm-5.3-flash --effort off
```

If any provider returns 400 on the thinking/effort fields, fix the catalog row (Task 4), not the adapter. The adapter tests pin the wire shapes.

- [ ] **Step 3: Gate — forced 429 retries with backoff**

```bash
python3 -c '
import http.server
class H(http.server.BaseHTTPRequestHandler):
    n=0
    def do_POST(s):
        H.n+=1
        if H.n<3: s.send_response(429); s.send_header("Retry-After","1"); s.end_headers(); return
        s.send_response(200); s.send_header("Content-Type","text/event-stream"); s.end_headers()
        s.wfile.write(b"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
http.server.HTTPServer(("127.0.0.1",8765),H).serve_forever()' &
cat > /tmp/moca-429.jsonc <<'EOF'
{ "model": "fake/m", "providers": { "fake": { "baseUrl": "http://127.0.0.1:8765", "protocol": "openai-completions",
  "auth": "api_key", "apiKey": "env:FAKE_KEY", "models": { "m": { "contextWindow": 32768 } } } } }
EOF
FAKE_KEY=x ./bin/moca --config /tmp/moca-429.jsonc -p hi; kill %1
```

Expected: stderr shows `retry 1/5 …` and `retry 2/5 …`, stdout `ok`, exit 0.

- [ ] **Step 4: Gate — §12 example decodes intact**

Run: `go test ./internal/config/ -run TestSpecExampleDecodesIntact -v` → PASS.

- [ ] **Step 5: Update README status line**

Replace the `**Status: …**` line with:

```markdown
**Status: phase 1 done — `moca -p` streams via anthropic, opencode-go and openai.** Next: phase 2 (tools + agent loop). Plans: [`docs/plans/`](docs/plans/).
```

- [ ] **Step 6: Commit**

```bash
git add README.md .gitignore
git commit -m "docs: phase 1 gate passed"
```
