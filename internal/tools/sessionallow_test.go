package tools

// Pins for the in-memory "allow for this session" answer (rev 35): one Agent
// is one session, so the set lives in the wrapper the agent installs once and
// nothing is ever written to disk. The matching tiers are a Question concern
// (tool_test.go); this file pins the wrapper's own rules.

import (
	"context"
	"testing"
)

func TestRememberSessionNilStaysNil(t *testing.T) {
	if RememberSession(nil) != nil {
		t.Fatal("a nil asker must stay nil: a -p run denies at every env.Ask site")
	}
}

// TestRememberSessionRecordsOnlySessionAnswers: once, always and deny are
// passed through and never remembered — only "for this session" is.
func TestRememberSessionRecordsOnlySessionAnswers(t *testing.T) {
	q := Question{Kind: "shell", Subject: "python3", Detail: "python3 -c 1", CanAlways: true}
	for _, ans := range []Answer{Deny, AllowOnce, AllowAlways} {
		asked := 0
		ask := RememberSession(func(context.Context, Question) Answer { asked++; return ans })
		for i := 0; i < 2; i++ {
			if got := ask(context.Background(), q); got != ans {
				t.Fatalf("%s: answer %v, want %v", ans, got, ans)
			}
		}
		if asked != 2 {
			t.Errorf("%s: remembered (%d asks for 2 calls)", ans, asked)
		}
	}
	asked := 0
	ask := RememberSession(func(context.Context, Question) Answer { asked++; return AllowSession })
	for i := 0; i < 3; i++ {
		if got := ask(context.Background(), q); got != AllowSession {
			t.Fatalf("session answer: %v", got)
		}
	}
	if asked != 1 {
		t.Fatalf("a session allow must be asked once, asked %d times", asked)
	}
}

// TestRememberSessionTiers: a remembered name never answers a question asked at
// the exact tier, and a different exact command line is a different key.
func TestRememberSessionTiers(t *testing.T) {
	var asked []string
	ask := RememberSession(func(_ context.Context, q Question) Answer {
		asked = append(asked, q.Subject+"|"+q.Detail)
		return AllowSession
	})
	name := Question{Kind: "shell", Subject: "git", Detail: "git status", CanAlways: true}
	if got := ask(context.Background(), name); got != AllowSession {
		t.Fatal(got)
	}
	// Same name, exact tier (a rule-Ask): must ask again.
	exact := Question{Kind: "shell", Subject: "git", Detail: "git push origin main"}
	if got := ask(context.Background(), exact); got != AllowSession {
		t.Fatal(got)
	}
	if len(asked) != 2 {
		t.Fatalf("the name tier answered a rule-Ask: %v", asked)
	}
	// The same exact line is remembered; a different one asks.
	if got := ask(context.Background(), exact); got != AllowSession {
		t.Fatal(got)
	}
	if len(asked) != 2 {
		t.Fatalf("the exact line was not remembered: %v", asked)
	}
	other := Question{Kind: "shell", Subject: "rm", Detail: "rm -f y"}
	if got := ask(context.Background(), other); got != AllowSession || len(asked) != 3 {
		t.Fatalf("a different command line must ask: %v", asked)
	}
}

// TestRememberSessionNoSession: an excluded question (moca's own config file,
// a web redirect hop) is never answered from memory, is never recorded, and an
// AllowSession answer to one is downgraded to AllowOnce — the same pin as
// allow-always on a question that forbade it.
func TestRememberSessionNoSession(t *testing.T) {
	asked := 0
	ask := RememberSession(func(_ context.Context, q Question) Answer { asked++; return AllowSession })
	q := Question{Kind: "write", Subject: "/home/u/.config/moca/config.jsonc", NoSession: true}
	for i := 0; i < 2; i++ {
		if got := ask(context.Background(), q); got != AllowOnce {
			t.Fatalf("call %d: %v, want AllowOnce (downgraded)", i, got)
		}
	}
	if asked != 2 {
		t.Fatalf("a NoSession question must be asked every time, asked %d", asked)
	}
	// The downgrade is not recorded either: a normal question with the same
	// key (a hand-built one) must still ask.
	normal := Question{Kind: "write", Subject: q.Subject}
	if got := ask(context.Background(), normal); got != AllowSession {
		t.Fatal(got)
	}
	if asked != 3 {
		t.Fatalf("a downgraded answer was recorded: %d asks", asked)
	}
}

func TestRememberSessionCancelledContextDenies(t *testing.T) {
	ask := RememberSession(func(context.Context, Question) Answer { return AllowSession })
	q := Question{Kind: "shell", Subject: "make", CanAlways: true}
	if got := ask(context.Background(), q); got != AllowSession {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := ask(ctx, q); got != Deny {
		t.Fatalf("a cancelled run must not spend a remembered allow: %v", got)
	}
}
