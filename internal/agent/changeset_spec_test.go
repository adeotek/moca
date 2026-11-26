package agent

import (
	"strings"
	"testing"
)

// The edits variant is selected by the resolved session, not by a hand-built
// spec: a table over resolved configs, because the failure mode this catches is
// a wired holder whose policy and capability set disagree.
func TestCodemodeSpecByResolvedSurface(t *testing.T) {
	cases := []struct {
		name, extra string
		stage       bool
		declared    bool
	}{
		{"off", "", false, false},
		{"additive", `"codemode":{"enabled":true}`, false, true},
		{"transport", `"codemode":{"enabled":true,"exposure":"transport"}`, false, true},
		{"workspace", `"codemode":{"enabled":true,"exposure":"workspace"}`, false, true},
		{"workspace and edits", `"codemode":{"enabled":true,"exposure":"workspace","workspaceEdits":true}`, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newScript(t, textTurn("answer"))
			a, _, _ := startWith(t, s, 1, c.extra, "")
			var desc string
			for _, spec := range a.opts.Tools.Specs() {
				if spec.Name == "codemode" {
					desc = spec.Description
				}
			}
			if !c.declared {
				if desc != "" {
					t.Fatal("codemode is declared in a session that never enabled it")
				}
				return
			}
			if desc == "" {
				t.Fatal("codemode is missing")
			}
			if got := strings.Contains(desc, "stageEdit"); got != c.stage {
				t.Fatalf("stageEdit in the description = %v, want %v: %q", got, c.stage, desc)
			}
		})
	}
}
