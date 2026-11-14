package skills

import (
	"strings"
	"testing"
)

func expandFixture() ([]Prompt, []Skill) {
	return []Prompt{
			{Name: "ship", Body: "Release it. Notes: $ARGUMENTS", Description: "release"},
		},
		[]Skill{
			{Name: "review", Description: "read a target", Path: "/x/review/SKILL.md", Source: "builtin"},
			{Name: "moca-testing", Description: "how this repo is checked", Path: "/x/moca-testing/SKILL.md", Source: "project", Auto: true},
		}
}

// `moca -p` and an ACP session/prompt do not run ParseInput, so the expansion
// rule has to live outside the TUI — and both surfaces have to use it.
func TestExpandCommand(t *testing.T) {
	prompts, sk := expandFixture()
	cases := []struct {
		in, want, kind, name string
	}{
		{"/review internal/store", "Load and follow the review skill (/x/review/SKILL.md) for this request: internal/store", ExpandSkill, "review"},
		{"/review", "Load and follow the review skill (/x/review/SKILL.md).", ExpandSkill, "review"},
		{"/ship 1.2.0", "Release it. Notes: 1.2.0", ExpandTemplate, "ship"},
		{"/ship", "Release it. Notes: ", ExpandTemplate, "ship"},
		// An auto skill is not a command: it rides the artifacts (§12).
		{"/moca-testing", "/moca-testing", ExpandNone, ""},
		// A message that starts with a path is prose, not a typo.
		{"/etc/hosts is wrong", "/etc/hosts is wrong", ExpandNone, ""},
		{"//review is the word", "/review is the word", ExpandEscape, "review"},
		{"plain words", "plain words", ExpandNone, ""},
	}
	for _, c := range cases {
		out, name, kind := ExpandCommand(c.in, prompts, sk)
		if out != c.want || kind != c.kind {
			t.Fatalf("ExpandCommand(%q) = (%q, %q, %q), want (%q, %q, %q)", c.in, out, name, kind, c.want, c.name, c.kind)
		}
		if name != c.name && kind != ExpandNone {
			t.Fatalf("ExpandCommand(%q) named %q, want %q", c.in, name, c.name)
		}
	}
}

// The escape names the word it unescaped, so a caller can log it.
func TestExpandCommandEscapeReportsTheWord(t *testing.T) {
	_, name, kind := ExpandCommand("//remember this", nil, nil)
	if kind != ExpandEscape || name != "remember" || !strings.HasPrefix(name, "remember") {
		t.Fatalf("escape = (%q, %q), want the escaped word", name, kind)
	}
}
