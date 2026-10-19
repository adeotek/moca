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

// SummaryPrefix opens the user message that carries a compaction summary;
// front ends recognize it when replaying history.
const SummaryPrefix = "[Summary of earlier conversation]"

// Messages rebuilds request messages: the latest compaction summary (if any),
// then entries from its firstKeptEntryId onward (§8). Without a compaction
// the whole transcript is used, unchanged.
func Messages(entries []Entry) []llm.Message { return MessagesWith(entries, nil, false) }

// MessagesWith rebuilds request messages with the two render-time hooks the
// agent needs:
//
//   - extra: run for every user message entry; the blocks it returns are
//     appended immediately after that entry's own blocks (before any later
//     entry merges in). The plan envelope rides this hook per entry — the flag
//     lives on the entry, so an earlier message renders identically in every
//     later request and a /plan toggle never rewrites history (§14). Per-entry
//     placement is deliberate: one envelope anchored at the end of the merged
//     message would move as soon as a second flagged entry merged in.
//   - historyBound: a preserved-thinking route binds the history to the exact
//     prefix it produced, so the kept tail — assistant entries positioned
//     before the latest compaction — replays verbatim, thinking blocks
//     included. Everywhere else the kept tail's thinking is stripped: the
//     summary already stands in for that stretch, and replaying its thinking
//     buys no reasoning. The one exception is the tail's last assistant turn
//     when it made tool calls (openToolTurn). Post-compaction turns keep
//     theirs either way (the strip is scoped to entries before the
//     compaction boundary).
func MessagesWith(entries []Entry, extra func(Entry) []llm.ContentBlock, historyBound bool) []llm.Message {
	var out []llm.Message
	start, compactionIdx := 0, -1
	if c, idx := LatestCompaction(entries); c != nil {
		compactionIdx = idx
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
		appendUser(&out, llm.ContentBlock{Type: llm.BlockText, Text: SummaryPrefix + "\n" + c.Summary})
	}
	stripThinking := !historyBound && compactionIdx >= 0
	keepIdx := -1
	if stripThinking {
		keepIdx = openToolTurn(entries, start, compactionIdx)
	}
	assistantIdx := map[string]int{}
	elided := ElidedIDs(entries)
	for i := start; i < len(entries); i++ {
		e := entries[i]
		switch e.Type {
		case TypeMessage:
			if e.Message.Role == llm.RoleAssistant {
				content := e.Message.Content
				if stripThinking && i < compactionIdx && i != keepIdx {
					content = withoutThinking(content)
				}
				out = append(out, llm.Message{Role: llm.RoleAssistant, Content: append([]llm.ContentBlock{}, content...)})
				assistantIdx[e.ID] = len(out) - 1
				continue
			}
			content := e.Message.Content
			if extra != nil {
				if ex := extra(e); len(ex) > 0 {
					content = append(append([]llm.ContentBlock{}, content...), ex...)
				}
			}
			appendUser(&out, content...)
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

// openToolTurn returns the index of the last assistant message before the
// compaction boundary when it made tool calls, else -1. A compaction that runs
// mid-run (after a tool batch, or on overflow) leaves that message as the one
// the next request's tool results answer, and a thinking-enabled anthropic
// route rejects a final assistant message that does not start with its
// thinking block — so it keeps its thinking. The index is fixed once the
// compaction exists, so every later request renders it identically.
func openToolTurn(entries []Entry, start, compactionIdx int) int {
	for j := compactionIdx - 1; j >= start; j-- {
		e := entries[j]
		if e.Type != TypeMessage || e.Message == nil || e.Message.Role != llm.RoleAssistant {
			continue
		}
		for k := j + 1; k < compactionIdx; k++ {
			if entries[k].Type == TypeToolUse && entries[k].ToolUse != nil && entries[k].ToolUse.MessageID == e.ID {
				return j
			}
		}
		return -1
	}
	return -1
}

// withoutThinking copies the blocks minus thinking: the kept tail's reasoning
// is already replaced by the compaction summary.
func withoutThinking(blocks []llm.ContentBlock) []llm.ContentBlock {
	out := make([]llm.ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type == llm.BlockThinking {
			continue
		}
		out = append(out, b)
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
