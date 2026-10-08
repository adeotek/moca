package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func TestCompactedAndResumedLines(t *testing.T) {
	m := newAgentModel(t)
	_, cmd := m.Update(agentEventMsg{agent.Compacted{TokensBefore: 26896, TokensAfter: 14500}})
	if got := printed(cmd); !strings.Contains(got, "compacted: 26896 → 14500 tokens") {
		t.Fatalf("compacted line: %q", got)
	}
	ack(m)
	_, cmd = m.Update(agentEventMsg{agent.Resumed{ID8: "deadbeef", Messages: 12}})
	if got := printed(cmd); !strings.Contains(got, "resumed deadbeef (12 messages)") {
		t.Fatalf("resumed line: %q", got)
	}
}

// A thinking block's item line prints when the block ENDS — before the
// response text or tool item that follows — and carries the block's
// duration. It used to wait for TurnEnd and appear after the response.
func TestThinkingLinePrintsBeforeTheResponse(t *testing.T) {
	m := newAgentModel(t)
	clock := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return clock }

	m.Update(agentEventMsg{agent.ThinkingDelta{Text: "step one\n"}})
	clock = clock.Add(9 * time.Second)
	m.Update(agentEventMsg{agent.ThinkingDelta{Text: "step two\n"}})

	// The first response text ends the block: its line lands first.
	_, cmd := m.Update(agentEventMsg{agent.TextDelta{Text: "the answer\n"}})
	out := drained(m, cmd)
	iThink := strings.Index(out, "thinking 2 lines · 9s")
	iResp := strings.Index(out, "the answer")
	if iThink < 0 {
		t.Fatalf("thinking line missing or missing its duration:\n%q", out)
	}
	if iResp < 0 || iResp < iThink {
		t.Fatalf("the thinking line must precede the response:\n%q", out)
	}
	if it, ok := m.items.LastOfKind("thinking"); !ok || it.Line != "⋯ #1 thinking 2 lines · 9s" {
		t.Fatalf("item: %+v", it)
	}

	// A tool call also ends an open block — before the tool's own item.
	m.Update(agentEventMsg{agent.ThinkingDelta{Text: "more\n"}})
	clock = clock.Add(3 * time.Second)
	_, cmd = m.Update(agentEventMsg{agent.ToolStart{Call: llm.ToolCall{Name: "ls"}}})
	out = drained(m, cmd)
	if !strings.Contains(out, "thinking 1 lines · 3s") {
		t.Fatalf("tool-start flush missing:\n%q", out)
	}
	_, cmd = m.Update(agentEventMsg{agent.ToolEnd{Call: llm.ToolCall{Name: "ls"}, Result: tools.Result{Summary: "2 entries"}}})
	out = drained(m, cmd)
	if iThink, iTool := strings.Index(out, "thinking 1 lines · 3s"), strings.Index(out, "ls 2 entries"); iThink >= 0 || iTool < 0 {
		t.Fatalf("the tool item must follow its block's flush:\n%q", out)
	}
}
