package session

import "github.com/adeotek/moca/internal/llm"

// LatestCompaction returns the newest compaction entry and its index, or
// (nil, -1). Request rebuilds and further compactions start from its
// FirstKeptEntryID.
func LatestCompaction(entries []Entry) (*Compaction, int) {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == TypeCompaction {
			return entries[i].Compaction, i
		}
	}
	return nil, -1
}

// ElidedStub replaces a superseded tool result's content in requests.
const ElidedStub = "[superseded: this call ran again later (or the file was rewritten) — the newer result is below]"

// ElidedIDs collects the tool_result ids named by every elision entry.
func ElidedIDs(entries []Entry) map[string]bool {
	ids := map[string]bool{}
	for _, e := range entries {
		if e.Type == TypeElision && e.Elision != nil {
			for _, id := range e.Elision.IDs {
				ids[id] = true
			}
		}
	}
	return ids
}

// Messages rebuilds request messages: the latest compaction summary (if any),
// then entries from its firstKeptEntryId onward (§8). Without a compaction
// the whole transcript is used, unchanged.
func Messages(entries []Entry) []llm.Message {
	var out []llm.Message
	start := 0
	if c, idx := LatestCompaction(entries); c != nil {
		found := false
		for i, e := range entries {
			if e.ID == c.FirstKeptEntryID {
				start, found = i, true
				break
			}
		}
		if !found {
			// The kept boundary is gone (trimmed file?): keep from the
			// compaction entry itself rather than replaying summarized
			// history as if the summary did not exist.
			start = idx
		}
		appendUser(&out, llm.ContentBlock{Type: llm.BlockText, Text: "[Summary of earlier conversation]\n" + c.Summary})
	}
	assistantIdx := map[string]int{}
	elided := ElidedIDs(entries)
	for _, e := range entries[start:] {
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
			if elided[e.ID] {
				r.Content = ElidedStub
			}
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
