package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"

	"github.com/adeotek/moca/internal/llm"
)

var validToolID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)

// NormalizeToolID maps a tool-call id to [a-zA-Z0-9_-]{1,40} deterministically:
// valid ids are unchanged, anything else becomes "c" + the first 16 hex chars
// of its sha256 (§3).
func NormalizeToolID(id string) string {
	if validToolID.MatchString(id) {
		return id
	}
	h := sha256.Sum256([]byte(id))
	return "c" + hex.EncodeToString(h[:])[:16]
}

// TransformHistory adapts stored history for the target model (§3): thinking
// blocks replay unchanged only to the exact (qualified) model that produced
// them, everything else becomes a "[prior reasoning]" text block (empty and
// redacted ones are dropped); tool-call ids are normalized consistently on both sides. The
// input is never mutated — the transcript keeps the originals.
func TransformHistory(msgs []llm.Message, target string) []llm.Message {
	var out []llm.Message
	for _, m := range msgs {
		var blocks []llm.ContentBlock
		for _, c := range m.Content {
			switch c.Type {
			case llm.BlockThinking:
				if c.Model == target {
					blocks = append(blocks, c)
				} else if c.Text != "" && !c.Redacted {
					blocks = append(blocks, llm.ContentBlock{Type: llm.BlockText, Text: "[prior reasoning]\n" + c.Text})
				}
			case llm.BlockToolUse:
				call := *c.ToolCall
				call.ID = NormalizeToolID(call.ID)
				blocks = append(blocks, llm.ContentBlock{Type: llm.BlockToolUse, ToolCall: &call})
			case llm.BlockToolResult:
				r := *c.ToolResult
				r.CallID = NormalizeToolID(r.CallID)
				blocks = append(blocks, llm.ContentBlock{Type: llm.BlockToolResult, ToolResult: &r})
			default:
				blocks = append(blocks, c)
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Role == m.Role {
			out[n-1].Content = append(out[n-1].Content, blocks...)
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: blocks})
	}
	return out
}
