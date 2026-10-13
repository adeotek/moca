package acp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// outputCap bounds the tool output copied into a tool call's content: the
// editor shows it, the model already has the full result.
const outputCap = 8 << 10

// toolInput is the union of the eight frozen tools' input fields moca maps.
type toolInput struct {
	Path      string  `json:"path"`
	Content   *string `json:"content"`
	OldString string  `json:"old_string"`
	NewString string  `json:"new_string"`
	Command   string  `json:"command"`
	Pattern   string  `json:"pattern"`
	Op        string  `json:"op"`
	URL       string  `json:"url"`
	Query     string  `json:"query"`
	Action    string  `json:"action"`
	Server    string  `json:"server"`
	Tool      string  `json:"tool"`
}

var toolKinds = map[string]string{
	"read": "read", "write": "edit", "edit": "edit", "shell": "execute",
	"search": "search", "ls": "search", "web": "fetch",
}

func toolKind(name string) string {
	if k, ok := toolKinds[name]; ok {
		return k
	}
	return "other"
}

// parseInput decodes call input; a cut-off or invalid input yields ok=false.
func parseInput(raw json.RawMessage) (toolInput, bool) {
	var in toolInput
	if !json.Valid(raw) || json.Unmarshal(raw, &in) != nil {
		return toolInput{}, false
	}
	return in, true
}

func toolTitle(name string, in toolInput, ok bool) string {
	if !ok {
		return name
	}
	switch name {
	case "read", "write", "edit":
		if in.Path != "" {
			return name + " " + in.Path
		}
	case "shell":
		line, _, _ := strings.Cut(strings.TrimSpace(in.Command), "\n")
		if r := []rune(line); len(r) > 80 {
			line = string(r[:80])
		}
		if line != "" {
			return line
		}
	case "search":
		return "search " + in.Pattern
	case "ls":
		if in.Path == "" {
			return "ls ."
		}
		return "ls " + in.Path
	case "web":
		if in.Op == "search" {
			return "search " + in.Query
		}
		return "fetch " + in.URL
	case "mcp":
		if in.Action == "call" {
			return fmt.Sprintf("mcp %s/%s", in.Server, in.Tool)
		}
		return strings.TrimSpace("mcp " + in.Action + " " + in.Query)
	}
	return name
}

func absPath(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

// startUpdate announces a tool call (status in_progress).
func startUpdate(id, cwd string, call llm.ToolCall) ToolCall {
	in, ok := parseInput(call.Input)
	u := ToolCall{SessionUpdate: updToolCall, ToolCallID: id, Title: toolTitle(call.Name, in, ok),
		Kind: toolKind(call.Name), Status: statusInProgress}
	if ok {
		u.RawInput = call.Input
		if in.Path != "" && call.Name != "web" && call.Name != "mcp" {
			u.Locations = []ToolCallLocation{{Path: absPath(cwd, in.Path)}}
		}
	}
	return u
}

// endUpdate reports a finished call: a diff for write/edit, the summary for
// read (never the file body), capped output text for the rest.
func endUpdate(id, cwd string, call llm.ToolCall, r tools.Result) ToolCall {
	u := ToolCall{SessionUpdate: updToolUpdate, ToolCallID: id, Status: statusCompleted}
	if r.IsError {
		u.Status = statusFailed
		u.Content = textContent(capHead(r.Content, outputCap))
		return u
	}
	in, ok := parseInput(call.Input)
	switch {
	case call.Name == "edit" && ok:
		u.Content = []ToolCallContent{{Type: "diff", Path: absPath(cwd, in.Path), OldText: strp(in.OldString), NewText: strp(in.NewString)}}
	case call.Name == "write" && ok && in.Content != nil:
		u.Content = []ToolCallContent{{Type: "diff", Path: absPath(cwd, in.Path), NewText: strp(*in.Content)}}
	case call.Name == "read":
		u.Content = textContent(r.Summary)
	case call.Name == "shell":
		u.Content = textContent(capTail(r.Content, outputCap))
	default:
		u.Content = textContent(capHead(r.Content, outputCap))
	}
	return u
}

func textContent(s string) []ToolCallContent {
	if s == "" {
		return nil
	}
	return []ToolCallContent{{Type: "content", Content: &ContentBlock{Type: "text", Text: s}}}
}

// capHead keeps the first n bytes (cut back to a rune boundary) and notes the rest.
func capHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + fmt.Sprintf("\n[… %d more bytes not shown]", len(s)-cut)
}

// capTail keeps the last n bytes (moved forward to a rune boundary).
func capTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	start := len(s) - n
	for start < len(s) && !utf8.RuneStart(s[start]) {
		start++
	}
	return fmt.Sprintf("[… %d earlier bytes not shown]\n", start) + s[start:]
}
