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
