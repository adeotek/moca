// Package llm holds the wire-neutral conversation types shared by every
// package. It is a leaf: it imports nothing from moca.
package llm

import (
	"encoding/json"
	"fmt"
	"strings"
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
// model id in Model (§3 cross-provider transform, §8 persistence). Anthropic
// redacted_thinking blocks store their opaque `data` payload in Signature
// with Redacted set, so they replay verbatim as redacted_thinking.
type ContentBlock struct {
	Type       BlockType   `json:"type"`
	Text       string      `json:"text,omitempty"`
	Signature  string      `json:"signature,omitempty"`
	ThinkingID string      `json:"thinkingId,omitempty"`
	Model      string      `json:"model,omitempty"`
	Redacted   bool        `json:"redacted,omitempty"`
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

// TextOf concatenates a message's text blocks (no thinking, no tool data).
func TextOf(m Message) string {
	var sb strings.Builder
	for _, c := range m.Content {
		if c.Type == BlockText {
			sb.WriteString(c.Text)
		}
	}
	return sb.String()
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
