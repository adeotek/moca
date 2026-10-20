package tools

// Pins for T-1 and T-5, both about the investigation hints' state:
//   T-1: the flags are per run — run 1's search or test read must not satisfy
//        run 2's "you have not looked yet" gate.
//   T-5: a search that never ran (a bad regex) must not count as a search.

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestBeginRunResetsInvestigationState(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.Commands = &fakeCmds{}

	env.Searched, env.TestSeen, env.TestFailed = true, true, true
	env.FailingTest = "TestX"
	env.BeginRun()
	if env.Searched || env.TestSeen || env.TestFailed || env.FailingTest != "" {
		t.Fatalf("BeginRun kept run 1's investigation state: searched=%v seen=%v failed=%v %q",
			env.Searched, env.TestSeen, env.TestFailed, env.FailingTest)
	}
}

func TestSearchWithABadPatternIsNotASearch(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.Commands = &fakeCmds{}
	env.BeginRun()
	reg := NewRegistry(searchTool{})

	res := reg.Run(t.Context(), env, llm.ToolCall{ID: "1", Name: "search",
		Input: []byte(`{"pattern":"("}`)})
	if !res.IsError {
		t.Fatalf("an invalid pattern must be an error, got %q", res.Content)
	}
	if env.Searched {
		t.Fatal("a failed compile counted as a search — it clears the look-before-you-read gate")
	}

	// Control: a real search sets it.
	reg.Run(t.Context(), env, llm.ToolCall{ID: "2", Name: "search", Input: []byte(`{"pattern":"x"}`)})
	if !env.Searched {
		t.Fatal("a real search must set Searched")
	}
}
