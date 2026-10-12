package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestParseJudge(t *testing.T) {
	cases := map[string]bool{
		`{"decision":"allow","reason":"build in workdir"}`:                true,
		"Sure!\n```json\n{\"decision\":\"allow\",\"reason\":\"ok\"}\n```": true,
		`{"decision":"ask","reason":"pushes"}`:                            false,
		`{"decision":"deny"}`:                                             false,
		`{"decision":"ALLOW"}`:                                            false,
		`allow`:                                                           false,
		``:                                                                false,
		// Pins for the first-{@}-wins rule (Task 6 review C1): only the
		// first balanced object in the reply counts, strings are skipped.
		`{"decision":"allow","reason":"ok"} {"decision":"ask"}`:                      true,
		`{"decision":"ask","reason":"hmm"} {"decision":"allow"}`:                     false,
		`{"decision":"allow","reason":"a } b"}`:                                      true,
		`{"decision":"allow","reason":"run it: {\"decision\":\"ask\"} is in there"}`: true,
	}
	for in, want := range cases {
		if got, _ := parseJudge(in); got != want {
			t.Errorf("parseJudge(%q) = %v, want %v", in, got, want)
		}
	}
}

// judgeAgent builds an Agent exactly like the loop tests (startTest) and
// appends a live-window skeleton: one user message, one tool call and one
// tool result. `scripts` are the SSE turns the fake provider replays for
// each judge call; a later call replays the last turn again.
func judgeAgent(t *testing.T, scripts ...string) (*Agent, *scriptServer) {
	t.Helper()
	s := newScript(t, scripts...)
	a, _, _ := startTest(t, s, 40, func(o *StartOptions) {
		o.Model, o.Effort = "fake/m", "minimal"
	})
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText("please run npm install left-pad")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{Call: llm.ToolCall{ID: "c1", Name: "shell"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.append(session.Entry{Type: session.TypeToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "SECRET-OUTPUT"}}); err != nil {
		t.Fatal(err)
	}
	return a, s
}

func TestJudgeFailsClosed(t *testing.T) {
	cases := map[string]string{
		"no-json":    `sure thing, allow it`,
		"truncated":  `{"decision":"allow"`,
		"ask":        `{"decision":"ask","reason":"hmm"}`,
		"deny":       `{"decision":"deny","reason":"x"}`,
		"wrong-case": `{"decision":"ALLOW","reason":"x"}`,
	}
	for name, turn := range cases {
		t.Run(name, func(t *testing.T) {
			a, _ := judgeAgent(t, textTurn(turn))
			if ok, _ := a.judge(context.Background(), "npm install left-pad"); ok {
				t.Errorf("%s: judge allowed", name)
			}
		})
	}
}

func TestJudgeTimeoutFailsClosed(t *testing.T) {
	// A turn that sends text but never a [DONE]: the 50ms judge budget
	// (judgeTimeout, vs the 20s production value) expires first.
	hang := "data: {\"choices\":[{\"delta\":{\"content\":\" \"}}]}\n\n"
	a, _ := judgeAgent(t, hang)
	was := judgeTimeout
	judgeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { judgeTimeout = was })
	if ok, _ := a.judge(context.Background(), "npm install left-pad"); ok {
		t.Fatal("an unfinished stream must not count as allow")
	}
}

// TestJudgeRequestCarriesNoToolOutput: the judge request's user text
// includes the last user message and the fenced command, never any tool
// result of the live window. The request is read back from the fake
// provider's captured bodies (negative indices reach the no-op turn the
// loop sends at Start, so the last body is the judge request).
func TestJudgeRequestCarriesNoToolOutput(t *testing.T) {
	a, s := judgeAgent(t, textTurn(`{"decision":"allow","reason":"ok"}`))
	if ok, _ := a.judge(context.Background(), "npm install left-pad"); !ok {
		t.Fatal("scripted allow lost")
	}
	s.mu.Lock()
	var usr string
	var hit bool
	for i := len(s.bodies) - 1; i >= 0; i-- {
		msgs, ok := s.bodies[i]["messages"].([]any)
		if !ok {
			continue
		}
		for _, m := range msgs {
			mm, o := m.(map[string]any)
			if !o || mm["role"] != "user" {
				continue
			}
			tx, o := mm["content"].(string)
			if !o {
				continue
			}
			// The judge request is the one that fences the command and
			// carries the workdir/temp dir header.
			if strings.Contains(tx, "<<<CMD") && strings.Contains(tx, "Temp dirs:") {
				usr, hit = tx, true
			}
		}
	}
	s.mu.Unlock()
	if !hit {
		t.Fatal("no judge request found among the captured bodies")
	}
	if strings.Contains(usr, "SECRET-OUTPUT") {
		t.Fatalf("the judge request carries a tool result: %q", usr)
	}
	if !strings.Contains(usr, "please run npm install left-pad") {
		t.Fatalf("the judge request must carry the user request: %q", usr)
	}
	if !strings.Contains(usr, "npm install left-pad") {
		t.Fatalf("the judge request must carry the command: %q", usr)
	}
	if strings.Contains(usr, "SECRET") {
		t.Fatal("unreachable: the tool result must be absent")
	}
}

// TestJudgeNoCacheForAskOrError: an ask or error verdict re-judges next
// time (context may have changed). Provider-call counts come from the fake
// server's captured request bodies.
func TestJudgeNoCacheForAskOrError(t *testing.T) {
	t.Run("ask re-judges", func(t *testing.T) {
		a, s := judgeAgent(t, textTurn(`{"decision":"ask","reason":"hmm"}`), textTurn(`{"decision":"allow","reason":"ok"}`))
		if ok, _ := a.judge(context.Background(), "npm install left-pad"); ok {
			t.Fatal("first call must be ask")
		}
		if ok, _ := a.judge(context.Background(), "npm install left-pad"); !ok {
			t.Fatal("second call after allow must be allow")
		}
		if got := s.bodyCount(); got != 2 {
			t.Fatalf("an ask verdict must re-judge next time: %d provider calls", got)
		}
	})
	t.Run("error re-judges", func(t *testing.T) {
		// A stall (network drop: io.EOF — retryable, and the retry loop
		// gives up after the attempt budget) surfaces as an error.
		a, s := judgeAgent(t, "data: bad chunk\n\n")
		_, _, err := a.judgeCmd(context.Background(), "npm install left-pad")
		if err == nil {
			t.Fatal("the first judge call must surface the provider error")
		}
		// The failed verdict is not cached: the same command re-judges
		// (the second SSE turn answers allow).
		{
			s.mu.Lock()
			s.turns = []string{textTurn(`{"decision":"allow","reason":"ok"}`)}
			s.mu.Unlock()
		}
		if ok, _ := a.judge(context.Background(), "npm install left-pad"); !ok {
			t.Fatal("the retried judge call must carry the allow")
		}
	})
}

func TestJudgeCachesAllowOnly(t *testing.T) {
	a, s := judgeAgent(t, textTurn(`{"decision":"allow","reason":"ok"}`))
	j1, _ := a.judge(context.Background(), "npm install left-pad")
	j2, _ := a.judge(context.Background(), "npm install left-pad")
	if !j1 || !j2 {
		t.Fatalf("j1=%v j2=%v", j1, j2)
	}
	// The Start probe, then exactly one judge call: the second verdict
	// came from the cache.
	if got := s.bodyCount(); got != 1 {
		t.Fatalf("an allow verdict must be cached: %d provider calls", got)
	}
	// A different command is not cached.
	if _, _ = a.judge(context.Background(), "go test ./..."); s.bodyCount() != 2 {
		t.Fatal("the cache is keyed by the exact command")
	}
}

func TestJudgeAccountsCost(t *testing.T) {
	// A fake provider model with default costs prices at zero, so the
	// accounting check asserts on usage; the cost path is the shared
	// CostOf formula (provider/catalog.go), exercised elsewhere.
	a, _ := judgeAgent(t, textTurn(`{"decision":"allow","reason":"ok"}`))
	u0, _ := a.Totals()
	if _, _, err := a.judgeCmd(context.Background(), "npm install left-pad"); err != nil {
		t.Fatal(err)
	}
	u1, _ := a.Totals()
	if u1.Input <= u0.Input || u1.Output <= u0.Output {
		t.Fatalf("the judge request must be accounted: usage %v -> %v", u0, u1)
	}
}
