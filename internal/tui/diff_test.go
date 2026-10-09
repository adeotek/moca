package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

const sampleDiff = "--- a.go\n+++ a.go\n@@ -1,4 +1,4 @@\n ctx\n-old one\n-old two\n+new one\n+new\x1b[2J two\n ctx2\n"

func TestDiffPreview(t *testing.T) {
	got := ansi.Strip(diffPreview(sampleDiff, 80, 8))
	want := "    -old one\n    -old two\n    +new one\n    +new^[[2J two"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	capped := ansi.Strip(diffPreview(sampleDiff, 80, 3))
	if !strings.Contains(capped, "… 2 more changed lines · ctrl+o") || strings.Count(capped, "\n") != 2 {
		t.Fatalf("overflow: %q", capped)
	}
	// Inside a hunk, "----" is a removed "---" line, not a file header.
	if got := ansi.Strip(diffPreview("--- a.md\n+++ a.md\n@@ -1,1 +0,0 @@\n----\n", 80, 8)); got != "    ----" {
		t.Fatalf("removed --- line: %q", got)
	}
	if diffPreview("--- a\n+++ a\n", 80, 8) != "" || diffPreview("", 80, 8) != "" {
		t.Fatal("no changes, no preview")
	}
}

func TestEditItemShowsDiff(t *testing.T) {
	m := newTestModel()
	out, _ := simulate(m, m.handleAgent(agent.ToolEnd{
		Call:   llm.ToolCall{Name: "edit"},
		Result: tools.Result{Summary: "a.go [+2 −2]", Detail: sampleDiff},
	}))
	out = ansi.Strip(out)
	if !strings.Contains(out, "▸ #1 edit a.go [+2 −2]") || !strings.Contains(out, "    +new one") {
		t.Fatalf("edit item: %q", out)
	}
	out, _ = simulate(m, m.handleAgent(agent.ToolEnd{
		Call:   llm.ToolCall{Name: "read"},
		Result: tools.Result{Summary: "a.go", Detail: sampleDiff},
	}))
	out = ansi.Strip(out)
	if strings.Contains(out, "+new one") {
		t.Fatalf("only edit items preview a diff: %q", out)
	}
}
