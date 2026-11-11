//go:build !windows

package tools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Allow once executes the original command exactly once, whole.
func TestCompositeAllowOnceRunsWholeCommand(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = nil
	env.Commands = &fakeCmds{need: []string{"python3"}, every: []string{"rm"}}
	var asked []Question
	env.Ask = func(_ context.Context, q Question) Answer { asked = append(asked, q); return AllowOnce }
	r := run(t, shellTool{}, env, map[string]any{"command": "printf a > a.txt && printf b > b.txt"})
	if r.IsError {
		t.Fatalf("%s", r.Content)
	}
	if len(asked) != 1 {
		t.Fatalf("asked %d questions, want 1", len(asked))
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("%s missing: an approved composite must run every segment", f)
		}
	}
}

// Allow always on a learnable composite teaches every need name at once.
func TestCompositeAllowAlwaysLearnsEveryNeedName(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = nil
	cmds := &fakeCmds{need: []string{"python3", "perl"}}
	env.Commands = cmds
	env.Ask = func(context.Context, Question) Answer { return AllowAlways }
	r := run(t, shellTool{}, env, map[string]any{"command": "printf a > a.txt && printf b > b.txt"})
	if r.IsError {
		t.Fatalf("%s", r.Content)
	}
	if !slices.Equal(cmds.allowed, []string{"python3", "perl"}) {
		t.Fatalf("allowed = %v, want both need names", cmds.allowed)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal("the whole command must run")
	}
}

// A mixed composite (any ask-every-time constituent) can only be remembered
// by its exact command line: the identical invocation is not asked again, a
// changed second segment is.
func TestCompositeSessionMatchesExactMixedCommand(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = nil
	env.Commands = &fakeCmds{need: []string{"python3"}, every: []string{"rm"}}
	asks := 0
	env.Ask = RememberSession(func(_ context.Context, q Question) Answer {
		asks++
		if q.CanAlways {
			t.Fatalf("a composite with an ask-every-time constituent must not offer always: %+v", q)
		}
		return AllowSession
	})
	first := "printf a > a.txt && printf b > b.txt"
	if r := run(t, shellTool{}, env, map[string]any{"command": first}); r.IsError {
		t.Fatalf("%s", r.Content)
	}
	if asks != 1 {
		t.Fatalf("asks = %d after the first invocation", asks)
	}
	if r := run(t, shellTool{}, env, map[string]any{"command": first}); r.IsError {
		t.Fatalf("repeat: %s", r.Content)
	}
	if asks != 1 {
		t.Fatalf("asks = %d, want the exact command remembered for the session", asks)
	}
	changed := "printf a > a.txt && printf c > c.txt"
	if r := run(t, shellTool{}, env, map[string]any{"command": changed}); r.IsError {
		t.Fatalf("%s", r.Content)
	}
	if asks != 2 {
		t.Fatalf("asks = %d, want a changed second segment to ask again", asks)
	}
}
