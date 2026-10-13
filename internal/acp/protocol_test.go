package acp

import (
	"encoding/json"
	"os"
	"testing"
)

// Every method moca serves or calls must exist in the pinned v1 method table
// (meta.json): a typo or a renamed method fails here, not in an editor.
func TestMethodsInPinnedMeta(t *testing.T) {
	b, err := os.ReadFile("testdata/meta.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		Version         int               `json:"version"`
		AgentMethods    map[string]string `json:"agentMethods"`
		ClientMethods   map[string]string `json:"clientMethods"`
		ProtocolMethods map[string]string `json:"protocolMethods"`
	}
	if err := json.Unmarshal(b, &meta); err != nil || meta.Version != 1 {
		t.Fatalf("meta.json: version %d, err %v (want a v1 schema)", meta.Version, err)
	}
	known := map[string]bool{}
	for _, m := range []map[string]string{meta.AgentMethods, meta.ClientMethods, meta.ProtocolMethods} {
		for _, v := range m {
			known[v] = true
		}
	}
	for _, m := range acpMethods {
		if !known[m] {
			t.Errorf("method %q is not in the pinned ACP v1 schema", m)
		}
	}
}

func TestProtocolShapesConform(t *testing.T) {
	conform(t, "InitializeResponse", InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{LoadSession: true,
			PromptCapabilities:  PromptCapabilities{EmbeddedContext: true},
			SessionCapabilities: SessionCapabilities{List: &struct{}{}}},
		AuthMethods: []AuthMethod{}, AgentInfo: Implementation{Name: "moca", Title: "moca", Version: "1.0.0"},
	})
	conform(t, "PromptResponse", PromptResponse{StopReason: stopEndTurn})
	for _, kind := range []string{updAgentMessage, updThought, updUserMessage} {
		conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
			Update: ContentChunk{SessionUpdate: kind, Content: ContentBlock{Type: "text", Text: "hi"}, MessageID: newMessageID()}})
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: ToolCall{
		SessionUpdate: updToolCall, ToolCallID: "1-call_0", Title: "read a.go", Kind: "read", Status: statusInProgress,
		RawInput: []byte(`{"path":"a.go"}`), Locations: []ToolCallLocation{{Path: "/w/a.go"}}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: ToolCall{
		SessionUpdate: updToolUpdate, ToolCallID: "1-call_0", Status: statusCompleted,
		Content: []ToolCallContent{
			{Type: "content", Content: &ContentBlock{Type: "text", Text: "ok"}},
			{Type: "diff", Path: "/w/a.go", OldText: strp("a"), NewText: strp("b")},
			{Type: "diff", Path: "/w/new.go", NewText: strp("")},
		}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: CurrentModeUpdate{SessionUpdate: updCurrentMode, CurrentModeID: "plan"}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: AvailableCommandsUpdate{
		SessionUpdate: updCommands, AvailableCommands: []AvailableCommand{{Name: "review", Description: "d", Input: &AvailableCommandInput{Hint: "file"}}}}})
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: UsageUpdate{SessionUpdate: updUsage, Used: 1200, Size: 65536, Cost: &Cost{Amount: 0.0123, Currency: "USD"}}})
	conform(t, "RequestPermissionRequest", RequestPermissionRequest{SessionID: "abcd1234",
		ToolCall: ToolCall{ToolCallID: "1-call_0", Title: "shell: rm"},
		Options: []PermissionOption{{OptionID: optAllowOnce, Name: "Allow once", Kind: "allow_once"},
			{OptionID: optAllowAlways, Name: "Always allow", Kind: "allow_always"},
			{OptionID: optReject, Name: "Deny", Kind: "reject_once"}}})
	conform(t, "InitializeResponse", InitializeResponse{ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{LoadSession: true, SessionCapabilities: SessionCapabilities{
			List: &struct{}{}, Delete: &struct{}{}, Resume: &struct{}{}, Close: &struct{}{}}},
		AuthMethods: []AuthMethod{}, AgentInfo: Implementation{Name: "moca", Version: "1"}})
	conform(t, "ResumeSessionResponse", ResumeSessionResponse{})
}
