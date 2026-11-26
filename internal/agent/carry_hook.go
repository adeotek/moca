//go:build codemodecarry

package agent

import (
	"encoding/json"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

// This file is the carry instrument (codemode plan v13, Task 1). It exists in
// the `codemodecarry` build only: the shipped binary carries no observation
// code at all (carry_nop.go is the untagged arm), because the measurement is
// for the eval harness and must cost a normal run nothing.
//
// The metric: moca's own token estimate for the UTF-8 content of every
// retained tool-result block in an outbound request. It is measured on the
// REQUEST, after history reconstruction, trim, elision and compaction have
// run, so it counts exactly the bytes a provider would have to re-read:
// summaries are text (not tool results), the classifier never goes through
// turn(), and a retry re-sends the same request — the first observation of a
// process is unaffected by either.

// carryReqNo numbers the outbound requests of this process. The eval trial
// reads turn 1 (the first request after --continue), which is the only one
// that is the "carried history" figure; later turns merely make a compaction
// or a retry visible.
var carryReqNo atomic.Int64

// carryObservation is the JSONL line the harness joins to its row.
type carryObservation struct {
	Session string `json:"session"`
	Trial   int    `json:"trial"`
	Turn    int    `json:"turn"`
	Carry   int    `json:"carry"`
	// Attempts is the process's cumulative physical-attempt count at this
	// turn (internal/provider/evalcount.go). A diagnostic beside the work
	// count: the two are never summed, and this one cannot be split between
	// work and auxiliary requests because a compaction may run mid-turn.
	Attempts int `json:"attempts,omitempty"`
}

// carryTokens sums the estimate over every retained tool-result block of the
// request, whatever tool produced it (read, search, ls, shell, web, mcp).
func carryTokens(msgs []llm.Message) int {
	var n int
	for _, m := range msgs {
		for _, c := range m.Content {
			if c.Type == llm.BlockToolResult && c.ToolResult != nil {
				n += compact.Tokens(len(c.ToolResult.Content))
			}
		}
	}
	return n
}

// carryObserve appends one observation to $MOCA_CARRY_OUT. It is best-effort
// on purpose: an instrument must never change the outcome of the run it is
// measuring. A missing or unwritable file shows up as the harness's "missing
// carry" failure, never as a failed run.
func (a *Agent) carryObserve(req llm.Request) {
	path := os.Getenv("MOCA_CARRY_OUT")
	if path == "" {
		return
	}
	turn := int(carryReqNo.Add(1))
	session := ""
	if a.opts.Session != nil {
		session = a.opts.Session.ID8()
	}
	obs := carryObservation{Session: session, Trial: envInt("MOCA_TRIAL"), Turn: turn,
		Carry: carryTokens(req.Messages), Attempts: provider.EvalAttempts()}
	b, err := json.Marshal(obs)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// envInt reads an optional integer env var; unset or malformed is 0, which the
// harness treats as "no trial label" rather than as an error.
func envInt(name string) int {
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0
	}
	return n
}
