package compact

import "github.com/adeotek/moca/internal/llm"

type Kind int

const (
	KindUser Kind = iota
	KindAssistant
	KindToolUse
	KindToolResult
)

// Entry is compact's provider- and session-free view of one transcript
// entry; agent converts session entries into it.
type Entry struct {
	ID     string
	Kind   Kind
	Msg    *llm.Message
	Call   *llm.ToolCall
	Result *llm.ToolResult
}

func EntryTokens(e Entry) int {
	n := 0
	switch {
	case e.Msg != nil:
		n = MessageChars(*e.Msg)
	case e.Call != nil:
		n = len(e.Call.Name) + len(e.Call.Input)
	case e.Result != nil:
		n = len(e.Result.Content)
	}
	return Tokens(n)
}
