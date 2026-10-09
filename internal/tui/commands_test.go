package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

// A command written during a run (the create-command workflow) becomes usable
// when the run finishes: the prompt dirs are re-read, and the dropdown and the
// parser both see it.
func TestPromptDirsReloadAtRunEnd(t *testing.T) {
	m := newTestModel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "deploy.md"),
		[]byte("---\ndescription: ship it\nargument-hint: <env>\n---\nDeploy $1 now"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.opts.PromptDirs = []skills.Dir{{Path: dir, Source: "global"}}
	if _, err := ParseInput("/deploy prod", m.opts.Prompts); err == nil {
		t.Fatal("not loaded before the run ends")
	}
	m.handleRunDone(runDoneMsg{})
	p, err := ParseInput("/deploy prod", m.opts.Prompts)
	if err != nil || p.Kind != KindPrompt || p.Text != "Deploy prod now" {
		t.Fatalf("after the run: %+v %v", p, err)
	}
	if !slices.ContainsFunc(m.commandHints(), func(d dropItem) bool { return d.name == "deploy" }) {
		t.Fatalf("the dropdown must list it: %+v", m.commandHints())
	}
	// Without dirs (callers passing literal prompts) nothing changes.
	m2 := newTestModel()
	m2.opts.Prompts = []skills.Prompt{{Name: "fixed", Body: "x"}}
	m2.handleRunDone(runDoneMsg{})
	if len(m2.opts.Prompts) != 1 || m2.opts.Prompts[0].Name != "fixed" {
		t.Fatalf("no dirs, no reload: %+v", m2.opts.Prompts)
	}
}

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
		{"/exit", Parsed{Kind: KindCommand, Name: "exit"}},
		{"/q", Parsed{Kind: KindCommand, Name: "exit"}},
		{"/quit now", Parsed{Kind: KindCommand, Name: "exit", Args: "now"}},
	}
	for _, c := range cases {
		got, err := ParseInput(c.in, prompts)
		if err != nil || got != c.want {
			t.Errorf("ParseInput(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	// An alias never hides a prompt template that owns the name.
	owned := []skills.Prompt{{Name: "q", Body: "Question $1"}}
	if got, _ := ParseInput("/q why", owned); got.Kind != KindPrompt || got.Text != "Question why" {
		t.Fatalf("a prompt named like an alias must win: %+v", got)
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
	if !strings.Contains(HelpText(prompts), "/exit") {
		t.Fatal("help lists /exit")
	}
}
