//go:build !windows

package agent

// TestSessionAllowScope pins the "allow for this session" scope: one Agent is
// one session. The set survives /mode (ask → yolo/auto → ask) and dies with the
// Agent (/clear, /resume, a new ACP session); the name tier covers every
// invocation of a command, the exact tier only the same command line.

import (
	"context"
	"testing"

	"github.com/adeotek/moca/internal/tools"
)

func TestSessionAllowScope(t *testing.T) {
	var asked []tools.Question
	ask := func(_ context.Context, q tools.Question) tools.Answer {
		asked = append(asked, q)
		return tools.AllowSession
	}
	s := newScript(t,
		toolTurn([2]string{"shell", `{"command":"uname"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"uname"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"uname"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"rm -f x"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"rm -f x"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"rm -f y"}`}), textTurn("ok"),
	)
	a, _, _ := startWith(t, s, 40, "", "", func(o *StartOptions) { o.Ask = ask })
	run := func() {
		t.Helper()
		if out, err := a.Run(context.Background(), "go"); err != nil || out.Text != "ok" {
			t.Fatalf("%+v %v", out, err)
		}
	}

	// uname is learnable: asked once, then answered from memory. yolo never
	// asks, and switching back to ask must not ask again — the set is the
	// Agent's, not the mode's.
	run()
	if len(asked) != 1 || asked[0].Subject != "uname" || asked[0].SessionExact() {
		t.Fatalf("uname in ask: %+v", asked)
	}
	a.SetMode(ModeYolo)
	run()
	a.SetMode(ModeAsk)
	run()
	if len(asked) != 1 {
		t.Fatalf("the set must survive ask→yolo→ask, asked %d times: %+v", len(asked), asked)
	}

	// rm is ask-every-time: the exact line is remembered, another line is not.
	// The name tier above cannot pin the /mode survival on its own — the
	// session answer also learned uname in the Rules, which would hide a set
	// rebuilt per mode. rm learns nothing there, so only the wrapper's set
	// can keep the repeat quiet across ask → auto → ask.
	asked = nil
	run()
	a.SetMode(ModeAuto)
	a.SetMode(ModeAsk)
	run()
	if len(asked) != 1 || !asked[0].SessionExact() {
		t.Fatalf("rm -f x across /mode: %+v", asked)
	}
	run()
	if len(asked) != 2 {
		t.Fatalf("rm -f y must ask: %+v", asked)
	}

	// A fresh Agent is a new session: the same question asks again.
	asked = nil
	s2 := newScript(t, toolTurn([2]string{"shell", `{"command":"uname"}`}), textTurn("ok"))
	a2, _, _ := startWith(t, s2, 40, "", "", func(o *StartOptions) { o.Ask = ask })
	if out, err := a2.Run(context.Background(), "go"); err != nil || out.Text != "ok" {
		t.Fatalf("%+v %v", out, err)
	}
	if len(asked) != 1 {
		t.Fatalf("a fresh agent must ask again, asked %d", len(asked))
	}

	// No asker stays no asker: wrapping nil must not invent an allow-by-memory
	// in a -p run.
	s3 := newScript(t)
	a3, _, _ := startWith(t, s3, 40, "", "")
	if a3.opts.Env.Ask != nil {
		t.Fatalf("Ask = %p, want nil", a3.opts.Env.Ask)
	}
	a3.SetMode(ModeAuto)
	if a3.opts.Env.Ask != nil {
		t.Fatalf("auto: Ask = %p, want nil", a3.opts.Env.Ask)
	}
}
