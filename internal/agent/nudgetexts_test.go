package agent

// P3.8: the marker a front end needs in order not to replay a nudge as the
// user's own words. It lives on the session entry (metadata, never on the wire
// — llm.Message is the wire), so the agent exposes the nudge texts beside
// History().

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func TestNudgeTextsMapsOnlyNudges(t *testing.T) {
	msg := func(s string) *llm.Message {
		return &llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
	}
	a := &Agent{}
	a.entries = []session.Entry{
		{Type: session.TypeMessage, Message: msg("run the tests")},
		{Type: session.TypeMessage, Nudge: "record", Message: msg("Record it now as a project skill.\n")},
		{Type: session.TypeMessage, Nudge: "action", Message: msg("You replied without acting.")},
		{Type: session.TypeToolUse},
		{Type: session.TypeMessage, Message: msg("thanks")},
	}
	got := a.NudgeTexts()
	if len(got) != 2 {
		t.Fatalf("NudgeTexts = %v, want the two nudges only", got)
	}
	if got["Record it now as a project skill."] != "record" {
		t.Fatalf("record nudge missing or mislabelled: %v", got)
	}
	if got["You replied without acting."] != "action" {
		t.Fatalf("action nudge missing or mislabelled: %v", got)
	}
	if _, ok := got["run the tests"]; ok {
		t.Fatal("a user message was labelled a nudge")
	}
}
