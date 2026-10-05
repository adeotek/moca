// Package compact owns token accounting and (phase 4) compaction. It never
// imports provider: summarization is injected by agent (§2).
package compact

import "github.com/adeotek/moca/internal/llm"

// Tokens estimates tokens as chars/4, rounded up.
func Tokens(chars int) int { return (chars + 3) / 4 }

// MessageChars counts the characters of one message's text, thinking text,
// tool-call name/input and tool-result content.
func MessageChars(m llm.Message) int {
	n := 0
	for _, c := range m.Content {
		n += len(c.Text)
		if c.ToolCall != nil {
			n += len(c.ToolCall.Name) + len(c.ToolCall.Input)
		}
		if c.ToolResult != nil {
			n += len(c.ToolResult.Content)
		}
	}
	return n
}

// RequestChars counts the characters of everything a request carries.
func RequestChars(system string, tools []llm.ToolSpec, msgs []llm.Message) int {
	n := len(system)
	for _, t := range tools {
		n += len(t.Name) + len(t.Description) + len(t.Schema)
	}
	for _, m := range msgs {
		n += MessageChars(m)
	}
	return n
}

// UsageTokens is the usage-anchored token count of one response.
func UsageTokens(u llm.Usage) int { return u.Input + u.CacheRead + u.CacheWrite + u.Output }
