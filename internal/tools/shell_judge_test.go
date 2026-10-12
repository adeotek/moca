//go:build !windows

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestShellJudgeStep(t *testing.T) {
	cases := map[string]struct {
		judge func(context.Context, string) (bool, string)
		asked int
		allow bool // the command runs
	}{
		"allow": {judge: func(context.Context, string) (bool, string) { return true, "ok" }, asked: 0, allow: true},
		"ask":   {judge: func(context.Context, string) (bool, string) { return false, "no" }, asked: 1, allow: false},
		"nil":   {asked: 1, allow: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			env, _ := testEnv(t)
			env.Commands = &fakeCmds{unsure: []string{"npm"}}
			var asked []Question
			env.Ask = func(_ context.Context, q Question) Answer {
				asked = append(asked, q)
				return Deny // stop at the first ask
			}
			env.Judge = c.judge
			r := run(t, shellTool{}, env, map[string]any{"command": "npm install left-pad"})
			// The uncertain command goes through the ladder: the judge
			// allow short-circuits it, an ask (or a nil judge) lands in it.
			if c.allow != !r.IsError {
				t.Fatalf("allow=%v result=%v", c.allow, r.Content)
			}
			if len(asked) != c.asked {
				t.Fatalf("asked=%d, want %d: %+v (result %v)", len(asked), c.asked, asked, r.Content)
			}
			for _, q := range asked {
				if q.Kind != "shell" || q.Subject != "npm" || !q.CanAlways {
					t.Fatalf("question %+v, want Kind shell Subject npm CanAlways true", q)
				}
			}
			if !c.allow && !strings.Contains(r.Content, "refused") {
				t.Fatalf("the denied ask must refuse: %v", r.Content)
			}
		})
	}
}

// learnableCmds is a checker that refuses to learn some names, like
// permissions.Shell does for rm.
type learnableCmds struct {
	fakeCmds
	never map[string]bool
}

func (l *learnableCmds) Learnable(n string) bool { return !l.never[n] }

// An unsure name the checker can never learn (auto mode's `rm *.o`) is asked
// without the allow-always offer: the answer could not take effect.
func TestShellUnsureNotLearnableNoAlways(t *testing.T) {
	env, _ := testEnv(t)
	env.Commands = &learnableCmds{fakeCmds: fakeCmds{unsure: []string{"rm", "npm"}}, never: map[string]bool{"rm": true}}
	got := map[string]bool{}
	env.Ask = func(_ context.Context, q Question) Answer {
		got[q.Subject] = q.CanAlways
		return AllowOnce
	}
	run(t, shellTool{}, env, map[string]any{"command": "true"})
	if always, ok := got["rm"]; !ok || always {
		t.Fatalf("rm asked=%v CanAlways=%v, want asked without allow-always", ok, always)
	}
	if always, ok := got["npm"]; !ok || !always {
		t.Fatalf("npm asked=%v CanAlways=%v, want allow-always offered", ok, always)
	}
}
