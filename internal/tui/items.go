package tui

import (
	"fmt"
	"strings"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// Item is one numbered collapsible scrollback entry (tool call or thinking
// block); its Body opens in the pager.
type Item struct {
	N          int
	Kind       string // "tool" | "thinking"
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

// LastOfKind returns the most recent item of the given kind ("tool" or
// "thinking").
func (s *Items) LastOfKind(kind string) (Item, bool) {
	for i := len(s.list) - 1; i >= 0; i-- {
		if s.list[i].Kind == kind {
			return s.list[i], true
		}
	}
	return Item{}, false
}
