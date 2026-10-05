package tui

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/agent"
)

func TestCompactedAndResumedLines(t *testing.T) {
	m := newAgentModel(t)
	_, cmd := m.Update(agentEventMsg{agent.Compacted{TokensBefore: 26896, TokensAfter: 14500}})
	if got := printed(cmd); !strings.Contains(got, "compacted: 26896 → 14500 tokens") {
		t.Fatalf("compacted line: %q", got)
	}
	_, cmd = m.Update(agentEventMsg{agent.Resumed{ID8: "deadbeef", Messages: 12}})
	if got := printed(cmd); !strings.Contains(got, "resumed deadbeef (12 messages)") {
		t.Fatalf("resumed line: %q", got)
	}
}
