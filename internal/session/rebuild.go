package session

import "github.com/adeotek/moca/internal/llm"

// Messages rebuilds request messages from transcript entries (no
// compaction handling yet — phase 4 adds it).
func Messages(entries []Entry) []llm.Message {
	var out []llm.Message
	assistantIdx := map[string]int{}
	for _, e := range entries {
		switch e.Type {
		case TypeMessage:
			if e.Message.Role == llm.RoleAssistant {
				out = append(out, llm.Message{Role: llm.RoleAssistant, Content: append([]llm.ContentBlock{}, e.Message.Content...)})
				assistantIdx[e.ID] = len(out) - 1
				continue
			}
			appendUser(&out, e.Message.Content...)
		case TypeToolUse:
			if i, ok := assistantIdx[e.ToolUse.MessageID]; ok {
				call := e.ToolUse.Call
				out[i].Content = append(out[i].Content, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: &call})
			}
		case TypeToolResult:
			r := *e.ToolResult
			appendUser(&out, llm.ContentBlock{Type: llm.BlockToolResult, ToolResult: &r})
		}
	}
	return out
}

func appendUser(out *[]llm.Message, blocks ...llm.ContentBlock) {
	if n := len(*out); n > 0 && (*out)[n-1].Role == llm.RoleUser {
		(*out)[n-1].Content = append((*out)[n-1].Content, blocks...)
		return
	}
	*out = append(*out, llm.Message{Role: llm.RoleUser, Content: append([]llm.ContentBlock{}, blocks...)})
}

// Repair returns synthetic error results for every tool_use without a
// tool_result, in call order. The caller appends them before the next
// request (abort: AbortedByUser; resume after crash: Interrupted).
func Repair(entries []Entry, reason string) []Entry {
	answered := map[string]bool{}
	for _, e := range entries {
		if e.Type == TypeToolResult {
			answered[e.ToolResult.CallID] = true
		}
	}
	var fix []Entry
	for _, e := range entries {
		if e.Type == TypeToolUse && !answered[e.ToolUse.Call.ID] {
			fix = append(fix, Entry{Type: TypeToolResult, ToolResult: &llm.ToolResult{CallID: e.ToolUse.Call.ID, Content: reason, IsError: true}})
		}
	}
	return fix
}
