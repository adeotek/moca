package compact

import (
	"context"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

// SummarySystem instructs the cheap model to produce the structured summary
// the rebuilt context continues from (§6, Pi's proven shape).
const SummarySystem = `You compress a coding-agent conversation into a summary that lets the agent continue the work without the original messages.
Write exactly these sections, in this order, as Markdown headings:
## Goal
## Constraints & Preferences
## Progress
### Done
### In progress
### Blocked
## Key Decisions
## Next Steps
## Critical Context
Rules: keep exact identifiers (file paths, function names, commands, error messages, versions). Prefer facts over narration. Omit pleasantries. If a section is empty write "none". Do not invent anything not in the conversation.`

const currentTurnNote = "This is the beginning of the CURRENT turn, still in progress. Summarize the request and progress so far.\n\n"

// Summarizer runs one summary request; agent injects the cheap model here so
// this package never imports provider.
type Summarizer func(ctx context.Context, system, user string) (string, llm.Usage, error)

// Prev is the previous compaction's state; a repeated compaction updates its
// summary instead of restarting from scratch.
type Prev struct {
	Summary        string
	Read, Modified []string
}

type Result struct {
	Summary, FirstKept string
	Usage              llm.Usage
	Read, Modified     []string
}

// Compact summarizes the part of es before the cut point. ok=false means
// there is nothing to summarize; no summarizer call happens then. A cut
// inside a turn produces two summaries (prior history + the turn prefix)
// merged into one.
func Compact(ctx context.Context, es []Entry, prev Prev, keepRecent, inputCapChars int, sum Summarizer) (Result, bool, error) {
	cut, turnStart, ok := FindCut(es, keepRecent)
	if !ok {
		return Result{}, false, nil
	}
	prevBlock := ""
	if prev.Summary != "" {
		prevBlock = "Previous summary (update it, don't drop facts):\n" + prev.Summary + "\n\n"
	}
	var usage llm.Usage
	call := func(user string) (string, error) {
		s, u, err := sum(ctx, SummarySystem, CapChars(user, inputCapChars))
		usage = usage.Add(u)
		return strings.TrimSpace(s), err
	}
	var summary string
	var err error
	if turnStart < 0 {
		summary, err = call(prevBlock + Serialize(es[:cut]))
	} else {
		var a, b string
		if turnStart > 0 || prevBlock != "" {
			if a, err = call(prevBlock + Serialize(es[:turnStart])); err != nil {
				return Result{}, false, err
			}
		}
		if b, err = call(currentTurnNote + Serialize(es[turnStart:cut])); err != nil {
			return Result{}, false, err
		}
		summary = strings.TrimSpace(a + "\n\n## Current turn (in progress)\n" + b)
	}
	if err != nil {
		return Result{}, false, err
	}
	read, mod := TrackFiles(es[:cut], prev.Read, prev.Modified)
	summary += "\n\n<read-files>\n" + strings.Join(read, "\n") + "\n</read-files>\n<modified-files>\n" +
		strings.Join(mod, "\n") + "\n</modified-files>"
	return Result{Summary: summary, FirstKept: es[cut].ID, Usage: usage, Read: read, Modified: mod}, true, nil
}
