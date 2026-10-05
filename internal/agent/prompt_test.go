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
