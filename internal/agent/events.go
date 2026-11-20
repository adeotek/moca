package agent

import (
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/tools"
)

// Event is what the agent reports to its driver (TUI or -p printer).
type Event interface{ isEvent() }

type TextDelta struct{ Text string }
type ThinkingDelta struct{ Text string }
type StreamReset struct{}
type ToolStart struct{ Call llm.ToolCall }
type ToolEnd struct {
	Call   llm.ToolCall
	Result tools.Result
}
type TurnEnd struct {
	Message llm.Message
	Usage   llm.Usage
	Cost    float64
	Stop    llm.StopReason
}
type Retry struct{ Notice provider.RetryNotice }
type Warning struct{ Text string }

// ContextPressure reports a request accepted between the operating trigger and
// the hard ceiling: compaction was attempted but the request fits only above
// the quality-oriented band.
type ContextPressure struct{ Input, Threshold, Ceiling int }

// Note is a quiet, non-problem notice (the startup auto-skill count, the
// switch table's note): the TUI prints it muted, -p as "note: …", ACP as an
// info notice.
type Note struct{ Text string }

func (TextDelta) isEvent()       {}
func (ThinkingDelta) isEvent()   {}
func (StreamReset) isEvent()     {}
func (ToolStart) isEvent()       {}
func (ToolEnd) isEvent()         {}
func (TurnEnd) isEvent()         {}
func (Retry) isEvent()           {}
func (Warning) isEvent()         {}
func (ContextPressure) isEvent() {}
func (Note) isEvent()            {}
