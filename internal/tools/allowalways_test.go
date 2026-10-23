package tools

// Pin for T-3: `AllowAlways` is only offered when the rules will honour it
// (CanAlways). An asker that returns it anyway — a non-conforming UI, an editor
// integration, a future ACP client — must not persist a permanent allow the
// checker ignores on the next call. `every` names ask every time by design.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowAlwaysIsIgnoredWhenTheQuestionForbadeIt(t *testing.T) {
	env, root := testEnv(t)
	if err := os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.ShellEnv = nil
	cmds := &fakeCmds{every: []string{"rm"}}
	env.Commands = cmds
	env.Ask = func(_ context.Context, q Question) Answer {
		if q.CanAlways {
			t.Errorf("an ask-every-time name must be asked with CanAlways=false")
		}
		return AllowAlways // non-conforming: the option was not offered
	}

	if r := run(t, shellTool{}, env, map[string]any{"command": "rm x"}); r.IsError {
		t.Fatalf("the call itself still runs: %s", r.Content)
	}
	if len(cmds.allowed) != 0 {
		t.Fatalf("AllowAlways persisted for a name asked with CanAlways=false: %v", cmds.allowed)
	}

	// Control: CanAlways=true still persists.
	env2, _ := testEnv(t)
	env2.ShellEnv = nil
	cmds2 := &fakeCmds{need: []string{"python"}}
	env2.Commands = cmds2
	env2.Ask = func(context.Context, Question) Answer { return AllowAlways }
	if r := run(t, shellTool{}, env2, map[string]any{"command": "python -c pass"}); r.IsError {
		t.Fatalf("%s", strings.TrimSpace(r.Content))
	}
	if len(cmds2.allowed) != 1 {
		t.Fatalf("an offered AllowAlways must persist, got %v", cmds2.allowed)
	}
}

// TestAllowSessionLearnsOnlyWhereTheRulesCan: the session answer rides the
// same `always` gate as allow-always — on an ask-every-time name it runs the
// call and learns nothing (the wrapper's exact-line key is the only memory),
// and on an offered name it learns the name in the in-memory rules. It must
// also never panic the answer-formatting path (the old fixed-size array).
func TestAllowSessionLearnsOnlyWhereTheRulesCan(t *testing.T) {
	env, root := testEnv(t)
	if err := os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.ShellEnv = nil
	cmds := &fakeCmds{every: []string{"rm"}}
	env.Commands = cmds
	env.Ask = func(context.Context, Question) Answer { return AllowSession }
	if r := run(t, shellTool{}, env, map[string]any{"command": "rm x"}); r.IsError {
		t.Fatalf("the call itself runs: %s", r.Content)
	}
	if len(cmds.allowed) != 0 {
		t.Fatalf("AllowSession learned an ask-every-time name: %v", cmds.allowed)
	}

	env2, _ := testEnv(t)
	env2.ShellEnv = nil
	cmds2 := &fakeCmds{need: []string{"python"}}
	env2.Commands = cmds2
	env2.Ask = func(context.Context, Question) Answer { return AllowSession }
	if r := run(t, shellTool{}, env2, map[string]any{"command": "python -c pass"}); r.IsError {
		t.Fatalf("%s", strings.TrimSpace(r.Content))
	}
	if len(cmds2.allowed) != 1 || cmds2.allowed[0] != "python" {
		t.Fatalf("an offered AllowSession must learn the name in memory, got %v", cmds2.allowed)
	}
}
