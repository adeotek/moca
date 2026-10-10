package agent

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func TestBuildSystemPrompt(t *testing.T) {
	p := BuildSystemPrompt(PromptInput{
		Workdir: "/w", OS: "Linux", Arch: "x86_64", Date: "2026-10-04", Git: "branch main, 2 uncommitted changes", Version: "0.1.0",
		Verify: "make vet && make test", VerifySource: "Makefile",
		Skills:       []skills.Skill{{Name: "rtk", Description: "compressed output", Path: "/d/rtk/SKILL.md"}},
		Servers:      []ServerLine{{Name: "context7", Description: "docs lookup"}},
		Instructions: []skills.Instruction{{Path: "/w/AGENTS.md", Content: "Use tabs."}},
	})
	for _, want := range []string{"/w", "Linux x86_64", "2026-10-04", "branch main", "`shell` is stateless",
		"read/search/ls for files, not shell cat/grep/find/ls",
		"read the failing test file with the read tool", "Locate the cause with the search tool before editing",
		"commit when the task asks", "- Project checks: `make vet && make test` (from Makefile)",
		"never change files through shell", "Never weaken or delete a test", "edge cases, invalid inputs",
		"read or search that file instead of re-running",
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
	if strings.Contains(off, "Project checks") {
		t.Fatal("no detected checks: no line")
	}
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

// TestPromptTemplateSlots: the fixed prompt text is the embedded markdown
// file — every {{slot}} it uses must be one renderPrompt resolves (a typo
// would ship the raw token to the model), and every resolvable slot must be
// used (a dead slot is silent debt).
func TestPromptTemplateSlots(t *testing.T) {
	known := []string{"{{workdir}}", "{{os}}", "{{arch}}", "{{date}}", "{{git}}", "{{verify}}", "{{rtk}}", "{{version}}"}
	re := regexp.MustCompile(`\{\{[a-z-]+\}\}`)
	used := map[string]bool{}
	for _, tok := range re.FindAllString(promptTemplate, -1) {
		used[tok] = true
	}
	if len(used) == 0 {
		t.Fatal("no slots found — embed broke or tokens were renamed")
	}
	for tok := range used {
		found := false
		for _, k := range known {
			if tok == k {
				found = true
			}
		}
		if !found {
			t.Errorf("unknown template slot %s", tok)
		}
	}
	for _, k := range known {
		if !used[k] {
			t.Errorf("slot %s is resolvable but unused in the template", k)
		}
	}
	// Rendering must consume every slot, with and without optional lines.
	for name, in := range map[string]PromptInput{
		"full": {Workdir: "/w", OS: "o", Arch: "a", Date: "d", Git: "g", Version: "v", Verify: "vk", RTK: true},
		"bare": {},
	} {
		if out := BuildSystemPrompt(in); strings.Contains(out, "{{") {
			t.Errorf("%s render leaves a raw slot:\n%s", name, out)
		}
	}
}
