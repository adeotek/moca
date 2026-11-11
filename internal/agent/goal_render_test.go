package agent

// Goal rendering pins (v0.8.0): previews keep the objective, envelopes are
// request-only and per entry, old goals never mutate, and a goal-off run is
// byte-for-byte a normal run.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// bodyMessages returns one recorded request's messages, as JSON text.
func bodyMessages(s *scriptServer, i int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.bodies[i]["messages"])
	return string(b)
}

// bodyText returns every text block of one recorded request, decoded (JSON
// escapes `<` as \u003c, so raw-body substring checks would lie).
func bodyText(s *scriptServer, i int) string {
	s.mu.Lock()
	raw, _ := json.Marshal(s.bodies[i]["messages"])
	s.mu.Unlock()
	var msgs []struct {
		Content any `json:"content"`
	}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return string(raw)
	}
	var sb strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			sb.WriteString(x)
			sb.WriteString("\n")
		case []any:
			for _, it := range x {
				if m, ok := it.(map[string]any); ok {
					walk(m["text"])
				}
			}
		}
	}
	for _, m := range msgs {
		walk(m.Content)
	}
	return sb.String()
}

func TestGoalHistoryPreviewReplayAndCache(t *testing.T) {
	obj := "make the fixture green: go test ./... passes"
	s := newScript(t,
		textTurn("round one"), textTurn("CONTINUE"),
		textTurn("round two"), textTurn("COMPLETE"),
		textTurn("after the goal"),
	)
	a, work, _ := startTest(t, s, 40)
	if _, err := a.StartGoal(context.Background(), obj); err != nil {
		t.Fatal(err)
	}

	// 1. The first stored user message is the objective itself.
	entries, _ := session.ReadFile(a.Session().Path())
	var first string
	for _, e := range entries {
		if e.Type == session.TypeMessage && e.Message != nil && e.Message.Role == llm.RoleUser {
			first = llm.TextOf(*e.Message)
			break
		}
	}
	if first != obj {
		t.Fatalf("first stored text = %q", first)
	}
	// 2. The session preview shows the objective (not a kickoff template).
	infos := session.List(filepath.Dir(a.Session().Path()), work, "", 10)
	if len(infos) != 1 || infos[0].Preview != obj {
		t.Fatalf("preview %+v", infos)
	}
	// 3. The request carries the request-only kickoff envelope, fenced.
	firstBody := bodyText(s, 0)
	if !strings.Contains(firstBody, "<untrusted_objective>") || !strings.Contains(firstBody, obj) {
		t.Fatalf("kickoff request missing its envelope:\n%s", firstBody)
	}
	// 4. The host continuation appears in NudgeTexts (ACP replay drops it).
	if got := a.NudgeTexts()[goalContinueText]; got != goalNudgeContinue {
		t.Fatalf("continuation nudge map %v", a.NudgeTexts())
	}
	// 5. Goal metadata never reaches the wire.
	for i := range s.bodies {
		if m := bodyMessages(s, i); strings.Contains(m, "goalId") || strings.Contains(m, "goalNudge") {
			t.Fatalf("goal metadata on the wire:\n%s", m)
		}
	}

	// 6. Clearing the goal never rewrites the earlier rendering.
	if err := a.ClearGoal(); err != nil {
		t.Fatal(err)
	}
	beforeClear := bodyMessages(s, 2) // the round-two work request
	if _, err := a.Run(context.Background(), "after the goal"); err != nil {
		t.Fatal(err)
	}
	after := bodyMessages(s, 4)
	// Every message before the new prompt is unchanged (the new user message
	// is appended, never merged into an old one).
	if !strings.HasPrefix(after, strings.TrimSuffix(beforeClear, "]")+",") {
		t.Fatalf("clearing rewrote history:\nbefore %s\nafter  %s", beforeClear, after)
	}
	if !strings.Contains(bodyText(s, 4), "<untrusted_objective>") {
		t.Fatal("an old goal entry must keep its envelope after a clear")
	}
}

// Goal-off runs are ordinary runs: one request per turn, no envelopes.
func TestGoalOffRequestsAreUnchanged(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTest(t, s, 40)
	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if s.bodyCount() != 1 {
		t.Fatalf("a goal-off run made %d requests", s.bodyCount())
	}
	body := bodyText(s, 0)
	if strings.Contains(body, "<untrusted_objective>") || strings.Contains(body, "[goal]") {
		t.Fatalf("a goal-off request carries goal text:\n%s", body)
	}
	var msgs []map[string]any
	if err := json.Unmarshal([]byte(bodyMessages(s, 0)), &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0]["role"] != "system" || msgs[1]["role"] != "user" {
		t.Fatalf("messages %s", bodyMessages(s, 0))
	}
}

// Plan and goal envelopes compose per entry: the plan envelope first, then
// the goal kickoff.
func TestGoalEnvelopeComposesWithPlan(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	if _, err := a.setGoal("compose the envelopes"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.plan = true
	a.mu.Unlock()
	if _, err := a.append(session.Entry{Type: session.TypeMessage,
		Message: userText("plan this and work the goal"), GoalID: a.goal.ID}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	msgs := a.messagesLocked()
	a.mu.Unlock()
	var sb strings.Builder
	for _, m := range msgs {
		if m.Role != llm.RoleUser {
			continue
		}
		joined := ""
		for _, b := range m.Content {
			if b.Type == llm.BlockText {
				joined += b.Text + "\n"
			}
		}
		if strings.Contains(joined, "plan this and work the goal") {
			sb.WriteString(joined)
		}
	}
	text := sb.String()
	if text == "" {
		t.Fatal("message not rendered")
	}
	pi := strings.Index(text, "[plan mode]")
	gi := strings.Index(text, "<untrusted_objective>")
	if pi < 0 || gi < 0 || pi > gi {
		t.Fatalf("envelope order: plan %d goal %d\n%s", pi, gi, text)
	}
	if !strings.Contains(text, "compose the envelopes") {
		t.Fatalf("goal envelope lacks the objective: %s", text)
	}
}

// A compacted goal gets its request-only reminder on a clone; stored history
// is untouched.
func TestGoalReminderAfterCompaction(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTest(t, s, 40)
	obj := "survive compaction"
	if _, err := a.setGoal(obj); err != nil {
		t.Fatal(err)
	}
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(obj), GoalID: a.goal.ID}); err != nil {
		t.Fatal(err)
	}
	// A compaction that keeps nothing of the goal-tagged entry.
	if _, err := a.append(session.Entry{Type: session.TypeMessage,
		Message: userText(session.SummaryPrefix + "\nsummary of earlier work")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{
		Summary: "summary", FirstKeptEntryID: a.entries[len(a.entries)-1].ID}}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	msgs := a.messagesLocked()
	a.mu.Unlock()
	flat := fmt.Sprint(msgs)
	if !strings.Contains(flat, "Reminder: this session is still working toward its goal") || !strings.Contains(flat, obj) {
		t.Fatalf("no reminder after compaction:\n%s", flat)
	}
	// Stored history has no reminder: the entries themselves are unchanged.
	entries, _ := session.ReadFile(a.Session().Path())
	if strings.Contains(fmt.Sprint(entries), "Reminder: this session is still working") {
		t.Fatal("the reminder was persisted")
	}
}
