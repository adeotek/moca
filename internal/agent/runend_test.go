package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// headlessRun marks the start options as a one-shot (-p) task run: only such
// runs get the action nudge.
func headlessRun(o *StartOptions) { o.Headless = true }

// TestRunEndingWithoutAnyToolCallNudgesOnce (TB2 round 3 lever): a text-only
// final reply with no tool call in the whole run gets exactly one persisted
// "action" nudge on a headless run; the next text-only reply ends the run.
func TestRunEndingWithoutAnyToolCallNudgesOnce(t *testing.T) {
	s := newScript(t,
		textTurn("Let me look at the environment first."),
		textTurn("nothing to change"),
	)
	a, _, _ := startTest(t, s, 40, headlessRun)
	out, err := a.Run(context.Background(), "fix the failing test")
	if err != nil {
		t.Fatal(err)
	}
	if s.bodyCount() != 2 {
		t.Fatalf("requests = %d, want exactly 2 (reply + one nudged reply)", s.bodyCount())
	}
	if out.Text != "nothing to change" {
		t.Fatalf("the run must end with the second reply: %q", out.Text)
	}
	var kinds []string
	for _, e := range a.entries {
		if e.Type == session.TypeMessage && e.Nudge != "" {
			kinds = append(kinds, e.Nudge)
		}
	}
	if len(kinds) != 1 || kinds[0] != "action" {
		t.Fatalf("nudge kinds = %v, want exactly one action nudge", kinds)
	}
	found := false
	for _, m := range msgJSON(t, snapshotBodies(s)[1]) {
		if strings.Contains(m, "without a single tool call") {
			found = true
		}
	}
	if !found {
		t.Fatal("the second request must carry the action nudge")
	}
}

// TestRunWithAToolCallGetsNoActionNudge: a run that executed a batch ends
// without the nudge — the trigger is "never acted", not "ended with text".
func TestRunWithAToolCallGetsNoActionNudge(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}),
		textTurn("done"),
	)
	a, _, _ := startTest(t, s, 40)
	if _, err := a.Run(context.Background(), "look at a.txt"); err != nil {
		t.Fatal(err)
	}
	if s.bodyCount() != 2 {
		t.Fatalf("requests = %d, want 2 (no extra nudge round)", s.bodyCount())
	}
	for _, e := range a.entries {
		if e.Nudge == "action" {
			t.Fatal("a run that executed a tool call must not get the action nudge")
		}
	}
}

// dropAdapter reports a thinking-drop count on every response: the first call
// asks for a tool (so the run acts and the action nudge stays out of the way),
// the second answers with text.
type dropAdapter struct {
	n     int
	calls int
}

func (d *dropAdapter) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) (llm.Response, error) {
	d.calls++
	if d.calls == 1 {
		return llm.Response{
			Message: llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
				{Type: llm.BlockText, Text: "checking"},
				{Type: llm.BlockToolUse, ToolCall: &llm.ToolCall{ID: "c1", Name: "ls", Input: json.RawMessage(`{}`)}},
			}},
			Stop: llm.StopToolUse, ThinkingDropped: d.n,
		}, nil
	}
	return llm.Response{
		Message:         llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "done"}}},
		Stop:            llm.StopEnd,
		ThinkingDropped: d.n,
	}, nil
}

// TestThinkingDroppedWarnsOncePerRun (B1.2): the count rides every message
// entry, and the run warns exactly once.
func TestThinkingDroppedWarnsOncePerRun(t *testing.T) {
	a, _, evs := startTest(t, newScript(t), 40)
	a.adapter = &dropAdapter{n: 2}
	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	warns := 0
	for _, e := range *evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "dropped thinking blocks") {
			warns++
		}
	}
	if warns != 1 {
		t.Fatalf("thinking-drop warnings = %d, want exactly one per run", warns)
	}
	var drops []int
	for _, e := range a.entries {
		if e.Type == session.TypeMessage && e.ThinkingDropped > 0 {
			drops = append(drops, e.ThinkingDropped)
		}
	}
	if len(drops) != 2 || drops[0] != 2 || drops[1] != 2 {
		t.Fatalf("message entries must carry the count on both turns: %v", drops)
	}
}
