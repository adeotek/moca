package tools

// The composite-approval contract (v0.8.0): one shell invocation is ONE
// question, whatever the parsed classifier found in it. The question's
// Subject is the deduplicated constituent list, its Detail is the untouched
// original command, and allow-always is offered only when every constituent
// is learnable and none asks every time.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCompositeApprovalMatrix(t *testing.T) {
	cases := []struct {
		name          string
		need, every   []string
		wantAsk       bool
		wantSubject   string
		wantCanAlways bool
		wantAllow     []string
	}{
		{"single need", []string{"python3"}, nil, true, "python3", true, []string{"python3"}},
		{"multiple need", []string{"python3", "perl"}, nil, true, "python3, perl", true, []string{"python3", "perl"}},
		{"multiple every", nil, []string{"rm", "dd"}, true, "rm, dd", false, nil},
		{"mixed need and every", []string{"python3"}, []string{"rm"}, true, "python3, rm", false, []string{"python3"}},
		{"one name in both tiers", []string{"rm"}, []string{"rm"}, true, "rm", false, []string{"rm"}},
		{"duplicates across segments", []string{"python3", "python3"}, nil, true, "python3", true, []string{"python3"}},
		{"empty tiers", nil, nil, false, "", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, _ := testEnv(t)
			env.ShellEnv = nil
			env.Commands = &fakeCmds{need: tc.need, every: tc.every}
			var asked []Question
			env.Ask = func(_ context.Context, q Question) Answer {
				asked = append(asked, q)
				return AllowOnce
			}
			const cmd = "echo hi"
			r := run(t, shellTool{}, env, map[string]any{"command": cmd})
			if r.IsError {
				t.Fatalf("%s", r.Content)
			}
			if !tc.wantAsk {
				if len(asked) != 0 {
					t.Fatalf("asked %d questions for a command with nothing to approve: %+v", len(asked), asked)
				}
				return
			}
			if len(asked) != 1 {
				t.Fatalf("asked %d questions, want exactly one for one invocation: %+v", len(asked), asked)
			}
			q := asked[0]
			if q.Kind != "shell" || q.Subject != tc.wantSubject || q.Detail != cmd {
				t.Fatalf("question = %+v, want kind shell, subject %q, detail %q", q, tc.wantSubject, cmd)
			}
			if q.CanAlways != tc.wantCanAlways {
				t.Fatalf("CanAlways = %v, want %v", q.CanAlways, tc.wantCanAlways)
			}
			if q.SessionExact() == tc.wantCanAlways {
				t.Fatalf("SessionExact = %v with CanAlways = %v", q.SessionExact(), q.CanAlways)
			}
			if !slices.Equal(q.AllowNames, tc.wantAllow) {
				t.Fatalf("AllowNames = %v, want %v", q.AllowNames, tc.wantAllow)
			}
		})
	}
}

// A deny or an unrecognised answer refuses the entire command: no segment
// runs, and the refusal keeps the shell tool's "refused:" prefix (the skill
// hint's arming rule keys on it).
func TestCompositeDenyRunsNoSegment(t *testing.T) {
	for _, tc := range []struct {
		name string
		ans  Answer
		want string
	}{
		{"deny", Deny, "refused"},
		{"unknown answer", Answer(42), "not recognised"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, root := testEnv(t)
			env.ShellEnv = nil
			env.Commands = &fakeCmds{need: []string{"python3"}, every: []string{"rm"}}
			env.Ask = func(context.Context, Question) Answer { return tc.ans }
			cmd := "printf a > a.txt && printf b > b.txt"
			r := run(t, shellTool{}, env, map[string]any{"command": cmd})
			if !r.IsError || !strings.Contains(r.Content, tc.want) {
				t.Fatalf("result %q, want an error containing %q", r.Content, tc.want)
			}
			for _, f := range []string{"a.txt", "b.txt"} {
				if _, err := os.Stat(filepath.Join(root, f)); err == nil {
					t.Fatalf("%s exists: a denied composite ran a segment", f)
				}
			}
		})
	}
}

// No asker (a -p run) denies the whole invocation before anything executes.
func TestCompositeNilAskerDenies(t *testing.T) {
	env, root := testEnv(t)
	env.ShellEnv = nil
	env.Commands = &fakeCmds{need: []string{"python3"}, every: []string{"rm"}}
	env.Ask = nil
	if r := run(t, shellTool{}, env, map[string]any{"command": "printf a > a.txt && printf b > b.txt"}); !r.IsError {
		t.Fatal("no asker must deny")
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("a segment ran without an asker")
	}
}
