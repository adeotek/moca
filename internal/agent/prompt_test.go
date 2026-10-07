package agent

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestBuildSystemPrompt(t *testing.T) {
	p := BuildSystemPrompt(PromptInput{
		Workdir: "/w", OS: "Linux", Arch: "x86_64", Date: "2026-10-04", Git: "branch main, 2 uncommitted changes", Version: "0.1.0",
		Skills:       []skills.Skill{{Name: "rtk", Description: "compressed output", Path: "/d/rtk/SKILL.md"}},
		Servers:      []ServerLine{{Name: "context7", Description: "docs lookup"}},
		Instructions: []skills.Instruction{{Path: "/w/AGENTS.md", Content: "Use tabs."}},
	})
	for _, want := range []string{"/w", "Linux x86_64", "2026-10-04", "branch main", "`shell` is stateless",
		"read/search/ls for files, not shell cat/grep/find/ls",
		"read the failing test file with the read tool", "locate the cause with search",
		"commit when the task asks",
		"rtk", "- rtk: compressed output (/d/rtk/SKILL.md)", "- context7: docs lookup", "Use tabs.", "moca 0.1.0"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Count(BuildSystemPrompt(PromptInput{}), "\n") > 60 {
		t.Fatal("core prompt must stay ~40 lines")
	}
	// version stamp sits at the end of the moca text (before appended sections)
	core := BuildSystemPrompt(PromptInput{Version: "9"})
	if !strings.HasSuffix(strings.TrimSpace(core), "moca 9") {
		t.Fatal("version stamp last")
	}
}

// rtk is advertised only when it is installed: a host without the CLI must
// not be told to use it (the first live ship-gate run derailed on
// `rtk: command not found` and never read the failing test).
func TestPromptRTKIsConditional(t *testing.T) {
	on := BuildSystemPrompt(PromptInput{RTK: true})
	if !strings.Contains(on, "rtk-prefixed") {
		t.Fatal("rtk on PATH: the token-discipline line must be present")
	}
	off := BuildSystemPrompt(PromptInput{})
	if strings.Contains(off, "rtk") {
		t.Fatalf("rtk absent: the prompt must not mention rtk at all:\n%s", off)
	}
}

func TestFilterSkillsDropsMissingBuiltins(t *testing.T) {
	sk := []skills.Skill{
		{Name: "rtk", Source: "builtin", Path: "/b/rtk/SKILL.md"},
		{Name: "rtk", Source: "global", Path: "/g/rtk/SKILL.md"}, // the user's own file survives
		{Name: "other", Source: "builtin", Path: "/b/other/SKILL.md"},
	}
	old := toolOnPath
	defer func() { toolOnPath = old }()
	toolOnPath = func(string) bool { return false }
	got := filterSkills(sk)
	if len(got) != 2 || got[0].Source != "global" || got[1].Name != "other" {
		t.Fatalf("missing tool: %+v", got)
	}
	toolOnPath = func(string) bool { return true }
	if got := filterSkills(sk); len(got) != 3 {
		t.Fatalf("installed tool: the builtin must stay: %+v", got)
	}
}
