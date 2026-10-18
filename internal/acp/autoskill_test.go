package acp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

func writeAutoSkillFile(t *testing.T, cwd, name string) {
	t.Helper()
	dir := filepath.Join(cwd, ".moca", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: d\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// managedOptions is testOptions with the self-managed switch on (ACP's trust
// default makes the feature live without a flag — Decision 14).
func managedOptions(t *testing.T, s *scriptServer) Options {
	t.Helper()
	o := testOptions(t, s)
	on := true
	o.Config.Skills.SelfManaged = &on
	return o
}

func TestACPCommandsReserveRememberOnlyWhenOn(t *testing.T) {
	// Off: nothing reserved, even for a user template named remember.
	off := buildCommands([]skills.Prompt{{Name: "remember", Description: "my own"}}, nil, false)
	if len(off) != 1 || off[0].Description != "my own" {
		t.Fatalf("off: %+v", off)
	}
	// On: reserved first, before prompts and skills.
	on := buildCommands(
		[]skills.Prompt{{Name: "remember", Description: "shadowed"}, {Name: "greet", Description: "hi"}},
		[]skills.Skill{{Name: "deploy", Description: "d"}}, true)
	if len(on) != 3 || on[0].Name != "remember" || on[0].Description != "record this as a project skill" ||
		on[0].Input == nil || on[0].Input.Hint != "rule text" {
		t.Fatalf("on: %+v", on)
	}
	if on[1].Name != "greet" || on[2].Name != "deploy" {
		t.Fatalf("order: %+v", on)
	}
}

func TestACPAutoSkillsAreNotCommands(t *testing.T) {
	got := buildCommands(nil, []skills.Skill{
		{Name: "moca-testing", Description: "learned", Auto: true},
		{Name: "deploy", Description: "hand-written"},
	}, true)
	for _, c := range got {
		if c.Name == "moca-testing" {
			t.Fatalf("an auto skill must never be advertised: %+v", got)
		}
	}
	if len(got) != 2 || got[0].Name != "remember" || got[1].Name != "deploy" {
		t.Fatalf("%+v", got)
	}
}

func TestACPTypedExcludesResources(t *testing.T) {
	body := "fix the deploy skill's staging step"
	got := textBlocks([]ContentBlock{
		{Type: "text", Text: "please"},
		{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/deploy.md", Text: &body}},
		{Type: "text", Text: "review this"},
	})
	if got != "please review this" {
		t.Fatalf("typed = %q", got)
	}
	// A resource-only prompt has no typed text at all: an attachment cannot
	// authorize a write (Decision 15).
	if got := textBlocks([]ContentBlock{{Type: "resource", Resource: &EmbeddedResource{URI: "file:///w/x.md", Text: &body}}}); got != "" {
		t.Fatalf("attachment-only typed = %q", got)
	}
}

func TestACPRememberIntercepted(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	sid := c.newSession(t.TempDir())
	if _, err := c.prompt(sid, "/remember use -count=1"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	if txt := lastUserText(t, s); !strings.Contains(txt, "Remember this project rule: use -count=1") {
		t.Fatalf("the run must carry the expansion, got %s", txt)
	}
	// Advertised while the feature is on.
	advertised := false
	for _, u := range c.updatesOf(updCommands) {
		for _, cm := range u["availableCommands"].([]any) {
			if cm.(map[string]any)["name"] == "remember" {
				advertised = true
			}
		}
	}
	if !advertised {
		t.Fatal("remember must be advertised while the feature is on")
	}
}

func TestACPRememberFallsThroughWhenOff(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(t.TempDir())
	if _, err := c.prompt(sid, "/remember use -count=1"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	if txt := lastUserText(t, s); !strings.Contains(txt, "/remember use -count=1") {
		t.Fatalf("with the feature off the text falls through verbatim, got %s", txt)
	}
	for _, u := range c.updatesOf(updCommands) {
		for _, cm := range u["availableCommands"].([]any) {
			if cm.(map[string]any)["name"] == "remember" {
				t.Fatal("remember must not be advertised with the feature off")
			}
		}
	}
}

func TestACPStartupNoteArrivesAsAnUpdate(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	c := startClient(t, managedOptions(t, s))
	cwd := t.TempDir()
	writeAutoSkillFile(t, cwd, "moca-one")
	sid := c.newSession(cwd)
	if _, err := c.prompt(sid, "hi"); err != nil {
		t.Fatalf("prompt: %v", err.Message)
	}
	// The startup note arrives as a session update (a notice when the client
	// advertises the extension, a thought line otherwise).
	found := false
	for _, u := range c.updates {
		if strings.Contains(mustJSON(u), "self-managed skills: 1 skill") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the startup note must reach the client: %v", c.updates)
	}
}
