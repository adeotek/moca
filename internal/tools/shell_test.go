//go:build !windows

package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

type fakeCmds struct {
	need, every []string
	err         error
	allowed     []string
}

func (f *fakeCmds) Check(string) ([]string, []string, error) { return f.need, f.every, f.err }
func (f *fakeCmds) Allow(n string)                           { f.allowed = append(f.allowed, n) }

func TestShellToolApprovals(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = os.Environ()
	cmds := &fakeCmds{need: []string{"python"}}
	env.Commands = cmds
	var asked []Question
	env.Ask = func(_ context.Context, q Question) Answer { asked = append(asked, q); return AllowAlways }
	r := run(t, shellTool{}, env, map[string]any{"command": "echo hi"})
	if r.IsError || !strings.Contains(r.Content, "hi") || !strings.Contains(r.Content, "[exit 0]") {
		t.Fatal(r.Content)
	}
	if len(asked) != 1 || !asked[0].CanAlways || len(cmds.allowed) != 1 {
		t.Fatalf("asked %+v allowed %v", asked, cmds.allowed)
	}
	cmds.need, cmds.every = nil, []string{"rm"}
	env.Ask = func(context.Context, Question) Answer { return Deny }
	r = run(t, shellTool{}, env, map[string]any{"command": "rm x"})
	if !r.IsError || !strings.Contains(r.Content, "rm") || !strings.Contains(r.Content, "refused") {
		t.Fatal(r.Content)
	}
	env.Ask = nil
	if r := run(t, shellTool{}, env, map[string]any{"command": "rm x"}); !r.IsError {
		t.Fatal("no asker = deny (one-shot mode)")
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": "x", "timeout": 301}); !r.IsError {
		t.Fatal("timeout max 300")
	}
}

func TestShellToolRunsAtRoot(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = os.Environ()
	env.Commands = &fakeCmds{}
	os.Mkdir(root+"/sub", 0o755)
	run(t, shellTool{}, env, map[string]any{"command": "cd sub"})
	r := run(t, shellTool{}, env, map[string]any{"command": "pwd -P"})
	if !strings.HasPrefix(strings.TrimSpace(r.Content), root+"\n") && !strings.Contains(r.Content, root+"\n[exit") {
		t.Fatalf("stateless: every call starts at the root, got %q", r.Content)
	}
}
