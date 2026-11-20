package compact

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

const resultCap = 2000

// safeTail trims a byte-sliced string back to a rune boundary so a cut can
// never leave a stranded partial rune at the seam (payloads are valid UTF-8;
// a cut lands at most one rune inside it).
func safeTail(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	i := len(s)
	for i > 0 && !utf8.RuneStart(s[i-1]) {
		i--
	}
	if i == 0 {
		return s
	}
	return s[:i-1]
}

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
		limit := 200
		switch {
		case k == "command", k == "old_string", k == "new_string":
			limit = 500
		case c.Name == "write" && k == "content":
			limit = 500
		}
		if len(v) > limit {
			v = safeTail(v[:limit]) + "…"
		}
		parts[i] = k + "=" + strings.ReplaceAll(v, "\n", "⏎")
	}
	return fmt.Sprintf("%s(%s)", c.Name, strings.Join(parts, ", "))
}

// resultHeadRatio chooses how much of a capped tool result to keep from the
// head: reads lead with declarations; command/web/mcp output leads with noise
// and ends with the failure or summary.
func resultHeadRatio(tool string, isError bool) float64 {
	if isError {
		return 0.35
	}
	switch tool {
	case "read":
		return 0.80
	case "shell", "search", "web", "mcp":
		return 0.35
	default:
		return 0.50
	}
}

// capHeadTail bounds s to max bytes, keeping a head and a tail on rune
// boundaries. The middle, not the failure at the end, is what gets dropped.
func capHeadTail(s string, max int, headRatio float64) string {
	if len(s) <= max {
		return s
	}
	headLen := int(float64(max) * headRatio)
	tailLen := max - headLen
	head := runePrefix(s, headLen)
	tail := runeSuffix(s, tailLen)
	return head + fmt.Sprintf("\n[… %d chars omitted …]\n", len(s)-len(head)-len(tail)) + tail
}

func runePrefix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func runeSuffix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return s[i:]
}

// Serialize flattens the summarized window into text for the summary request.
// It is lossy by design: tool results keep a head and a tail within 2K chars,
// tool arguments are bounded, and historical thinking is omitted — the
// summarizer needs facts, not reasoning traces.
func Serialize(es []Entry) string {
	var sb strings.Builder
	sb.WriteString("COMPACTION NOTE (lossy): tool results keep head and tail within 2000 chars; historical thinking omitted.\n")
	tool := map[string]string{}
	for _, e := range es {
		if e.Kind == KindToolUse && e.Call != nil {
			tool[e.Call.ID] = e.Call.Name
		}
	}
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
				case llm.BlockText:
					// Tool-only turns carry an empty text block; a bare
					// "[Assistant]: " line is noise for the summarizer.
					if c.Text != "" {
						fmt.Fprintf(&sb, "[Assistant]: %s\n", c.Text)
					}
				}
			}
		case KindToolUse:
			fmt.Fprintf(&sb, "[Assistant tool calls]: %s\n", callLine(e.Call))
		case KindToolResult:
			body := capHeadTail(e.Result.Content, resultCap, resultHeadRatio(tool[e.Result.CallID], e.Result.IsError))
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
		// Normalize spellings ("a.go", "./a.go", "src/../a.go") so one path
		// cannot sit in both lists at once. Absolute vs relative spellings of
		// the same file stay distinct: compact works on raw tool inputs.
		a.Path = filepath.Clean(a.Path)
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
// summarizer model's window. The byte cut is snapped to a rune boundary.
func CapChars(s string, max int) string {
	if len(s) <= max {
		return s
	}
	tail := s[len(s)-max:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	// The seam (or the newline search) can land mid-rune: skip continuation
	// bytes so the retained tail starts at a complete character.
	for len(tail) > 0 && !utf8.RuneStart(tail[0]) {
		tail = tail[1:]
	}
	return omitted + tail
}
