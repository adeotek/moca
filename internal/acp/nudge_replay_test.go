package acp

// P3.8: a nudge is moca's own text (§14). History() is []llm.Message and
// carries no marker, so the replay used to stream "record what this run
// proved …" to the client as a user_message_chunk — words in the user's mouth.

import (
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestReplaySkipsNudgeMessages(t *testing.T) {
	nudge := "Record it now as a project skill: what this run proved, the exact command."
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "fix the test"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "done"}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: nudge}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "all green"}}},
	}
	s, ups := bareSession(t, false, true)
	s.replayMessages(msgs, map[string]string{nudge: "record"})
	for _, u := range ups() {
		if u["sessionUpdate"] != updUserMessage {
			continue
		}
		if txt, _ := u["content"].(map[string]any)["text"].(string); txt == nudge {
			t.Fatalf("the nudge was replayed as a user message: %v", u)
		}
	}
	// The real user turn must still be there, and the nudge's absence must not
	// take the answers with it.
	var sawUser, sawAnswer bool
	for _, u := range ups() {
		txt, _ := u["content"].(map[string]any)["text"].(string)
		switch {
		case u["sessionUpdate"] == updUserMessage && txt == "fix the test":
			sawUser = true
		case u["sessionUpdate"] == updAgentMessage && txt == "all green":
			sawAnswer = true
		}
	}
	if !sawUser || !sawAnswer {
		t.Fatalf("replay lost content: user=%v answer=%v (%v)", sawUser, sawAnswer, ups())
	}

	// Control: with no marker the same text is the user's own and is replayed.
	s2, ups2 := bareSession(t, false, true)
	s2.replayMessages(msgs, nil)
	var replayed bool
	for _, u := range ups2() {
		if txt, _ := u["content"].(map[string]any)["text"].(string); u["sessionUpdate"] == updUserMessage && txt == nudge {
			replayed = true
		}
	}
	if !replayed {
		t.Fatal("a user message that is not a nudge must still replay as one")
	}
}
