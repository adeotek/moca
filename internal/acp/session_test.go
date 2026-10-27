package acp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// §9.5: the last model turn's stop reason maps onto ACP's StopReason. A
// truncated turn is retried once (rev 35), so a run that *ends* on a truncation
// scripts two of them.
func TestStopReasonMaxTokens(t *testing.T) {
	cut := `data: {"choices":[{"delta":{"content":"cut off"},"finish_reason":"length"}]}` + "\n\ndata: [DONE]\n\n"
	s := newScript(t, cut, cut)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "go")
	if err != nil {
		t.Fatalf("err %+v", err)
	}
	if stop != stopMaxTokens {
		t.Fatalf("stop %q, want %q", stop, stopMaxTokens)
	}
}

// A truncated turn that the retry recovers from reports the retry's stop reason:
// the run finished, and calling it max_tokens would mark a rescued run failed.
func TestRescuedTruncationEndsClean(t *testing.T) {
	cut := `data: {"choices":[{"delta":{"content":"cut off"},"finish_reason":"length"}]}` + "\n\ndata: [DONE]\n\n"
	ok := `data: {"choices":[{"delta":{"content":"here it is"},"finish_reason":"stop"}]}` + "\n\ndata: [DONE]\n\n"
	c := startClient(t, testOptions(t, newScript(t, cut, ok)))
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "go")
	if err != nil {
		t.Fatalf("err %+v", err)
	}
	if stop != "end_turn" {
		t.Fatalf("stop %q, want end_turn after a rescued truncation", stop)
	}
}

// The openai-completions codec maps finish_reason content_filter onto
// llm.StopRefusal.
func TestStopReasonRefusal(t *testing.T) {
	s := newScript(t, `data: {"choices":[{"delta":{"content":"no"},"finish_reason":"content_filter"}]}`+"\n\ndata: [DONE]\n\n")
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "go")
	if err != nil {
		t.Fatalf("err %+v", err)
	}
	if stop != stopRefusal {
		t.Fatalf("stop %q, want %q", stop, stopRefusal)
	}
}

// The step limit is the run's outcome, not the last turn's stop reason: the
// tool-less wrap-up turn's own "stop" must not win over it.
func TestStopReasonMaxSteps(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"ls", `{}`}),
		textTurn("out of steps"), // the wrap-up request
	)
	o := testOptions(t, s)
	o.Config.Context.MaxSteps = 1
	c := startClient(t, o)
	sid := c.newSession(t.TempDir())
	stop, err := c.prompt(sid, "go")
	if err != nil {
		t.Fatalf("err %+v", err)
	}
	if stop != stopMaxTurns {
		t.Fatalf("stop %q, want %q", stop, stopMaxTurns)
	}
}

// A run that fails answers -32603 carrying the error, never a stop reason.
func TestPromptFailureIsInternal(t *testing.T) {
	// A 200 that is not an SSE stream is not retryable (§8): the run fails at
	// once, so this needs no retry budget.
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"error":"nope"}`)
	}))
	t.Cleanup(bad.Close)
	o := testOptions(t, newScript(t))
	p := o.Config.Providers["fake"]
	p.BaseURL = bad.URL
	o.Config.Providers["fake"] = p
	c := startClient(t, o)
	sid := c.newSession(t.TempDir())
	if stop, err := c.prompt(sid, "go"); err == nil || err.Code != codeInternal || stop != "" ||
		!strings.Contains(err.Message, "want text/event-stream") {
		t.Fatalf("stop %q err %+v", stop, err)
	}
}

// A client may send its next request the moment it has the session/new
// response: the response's follow-up (available_commands_update) must not
// overlap a prompt already rewriting the same state — the command list and
// the agent's skill list are between-runs state, written by one goroutine at
// a time. testEmit widens the window deterministically (it runs inside the
// follow-up, after the response was written).
func TestPromptDoesNotRaceTheNewSessionFollowUp(t *testing.T) {
	o := testOptions(t, newScript(t, textTurn("ok")))
	release := make(chan struct{})
	o.testEmit = func(*session) { <-release }
	c := startClient(t, o)
	sid := c.newSession(t.TempDir())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if stop, err := c.prompt(sid, "hi"); err != nil || stop != stopEndTurn {
			t.Errorf("prompt: %q %+v", stop, err)
		}
	}()
	time.Sleep(150 * time.Millisecond) // the prompt starts and rewrites the command list
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the prompt never ran")
	}
}
