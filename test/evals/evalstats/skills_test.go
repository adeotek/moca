package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// TestCollectSkillMetrics pins the two self-managed-skills metrics: a
// write/edit under .moca/skills/ counts, an ordinary write does not, and a
// stored prompt that lists a moca-* skill reports the listing.
func TestCollectSkillMetrics(t *testing.T) {
	empty := Collect([]session.Entry{{Type: session.TypeSession, Session: &session.Header{SystemPrompt: "# Skills\n- rtk: x (/s/rtk/SKILL.md)\n"}}})
	if empty.SkillWrites != 0 || empty.SkillListed {
		t.Fatalf("a prompt without auto skills must report neither: %+v", empty)
	}

	prompt := "# Skills\n- moca-testing: how tests run (/w/.moca/skills/moca-testing/SKILL.md) (auto)\n"
	write := func(id, path string) session.Entry {
		b, _ := json.Marshal(map[string]string{"path": path, "content": "x"})
		return session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{Call: llm.ToolCall{ID: id, Name: "write", Input: b}}}
	}
	edit := func(id, path string) session.Entry {
		b, _ := json.Marshal(map[string]string{"path": path, "old_string": "a", "new_string": "b"})
		return session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{Call: llm.ToolCall{ID: id, Name: "edit", Input: b}}}
	}
	entries := []session.Entry{
		{Type: session.TypeSession, Session: &session.Header{SystemPrompt: prompt}},
		write("c1", ".moca/skills/moca-testing/SKILL.md"),
		write("c2", "pkg/x.go"),
		edit("c3", ".moca/skills/moca-testing/SKILL.md"),
		edit("c4", ".moca/skills/deploy/references/x.md"),
	}
	m := Collect(entries)
	if m.SkillWrites != 3 {
		t.Fatalf("skill writes = %d, want 3", m.SkillWrites)
	}
	if !m.SkillListed {
		t.Fatal("the stored prompt lists a moca-* skill")
	}
	// The listing is the prompt, not a tool path: a session that wrote one but
	// started before it existed reports listed=false.
	m2 := Collect(entries[1:])
	if m2.SkillListed {
		t.Fatal("no header, no listing")
	}
	if !strings.Contains(Summarize([]Metrics{m}), "skill writes") {
		t.Fatal("the summary table must carry the skill columns")
	}
}
