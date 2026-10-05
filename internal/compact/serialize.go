package compact

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

const resultCap = 2000

func callLine(c *llm.ToolCall) string {
	var m map[string]json.RawMessage
	json.Unmarshal(c.Input, &m)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v := string(m[k])
		var s string
		if json.Unmarshal(m[k], &s) == nil {
			v = s
		}
		if len(v) > 200 {
			v = v[:200] + "…"
		}
		parts[i] = k + "=" + strings.ReplaceAll(v, "\n", "⏎")
	}
	return fmt.Sprintf("%s(%s)", c.Name, strings.Join(parts, ", "))
}

// Serialize flattens the summarized window into text for the summary request.
// It is lossy by design: tool results are truncated at 2K chars (the
// summarizer needs facts, not outputs) and the header says so.
func Serialize(es []Entry) string {
	var sb strings.Builder
	sb.WriteString("COMPACTION NOTE (lossy): tool results truncated at 2000 chars.\n")
	for _, e := range es {
		switch e.Kind {
		case KindUser:
			for _, c := range e.Msg.Content {
				if c.Type == llm.BlockText {
					fmt.Fprintf(&sb, "[User]: %s\n", c.Text)
				}
			}
		case KindAssistant:
			for _, c := range e.Msg.Content {
				switch c.Type {
				case llm.BlockThinking:
					if c.Text != "" {
						fmt.Fprintf(&sb, "[Assistant thinking]: %s\n", c.Text)
					}
				case llm.BlockText:
					fmt.Fprintf(&sb, "[Assistant]: %s\n", c.Text)
				}
			}
		case KindToolUse:
			fmt.Fprintf(&sb, "[Assistant tool calls]: %s\n", callLine(e.Call))
		case KindToolResult:
			body := e.Result.Content
			if len(body) > resultCap {
				body = body[:resultCap] + "[… truncated]"
			}
			label := "Tool result"
			if e.Result.IsError {
				label = "Tool error"
			}
			fmt.Fprintf(&sb, "[%s]: %s\n", label, body)
		}
	}
	return sb.String()
}

// TrackFiles accumulates the cumulative read/modified file lists across
// compactions: read calls add to read, write/edit move a path to modified.
func TrackFiles(es []Entry, prevRead, prevMod []string) ([]string, []string) {
	read := map[string]bool{}
	mod := map[string]bool{}
	for _, p := range prevRead {
		read[p] = true
	}
	for _, p := range prevMod {
		mod[p] = true
	}
	for _, e := range es {
		if e.Kind != KindToolUse {
			continue
		}
		var a struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(e.Call.Input, &a) != nil || a.Path == "" {
			continue
		}
		switch e.Call.Name {
		case "read":
			if !mod[a.Path] {
				read[a.Path] = true
			}
		case "write", "edit":
			mod[a.Path] = true
			delete(read, a.Path)
		}
	}
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	return keys(read), keys(mod)
}

const omitted = "[… earlier history omitted]\n"

// CapChars bounds a serialized payload to max chars by dropping the oldest
// whole lines, so the summary request itself can never overflow the
// summarizer model's window.
func CapChars(s string, max int) string {
	if len(s) <= max {
		return s
	}
	tail := s[len(s)-max:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	return omitted + tail
}
