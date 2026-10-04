package provider

import (
	"encoding/json"

	"github.com/adeotek/moca/internal/llm"
)

const priorReasoning = "[prior reasoning]\n"

// replaysVerbatim reports whether a stored thinking block may go back to the
// model as a thinking block. Signatures and encrypted payloads are bound to the
// model that produced them (§3), and a block without one is unreplayable.
func replaysVerbatim(c llm.ContentBlock, model string) bool {
	return c.Model == model && c.Signature != ""
}

// priorReasoningText is the fallback for a thinking block that cannot replay
// verbatim: its text under a label, or "" when there is nothing worth keeping
// (redacted or empty reasoning).
func priorReasoningText(c llm.ContentBlock) string {
	if c.Redacted || c.Text == "" {
		return ""
	}
	return priorReasoning + c.Text
}

// toolInput guards history replay: a nil or malformed Input would marshal as
// `null` or fail json.Marshal outright, breaking every later turn.
func toolInput(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage("{}")
	}
	return raw
}
