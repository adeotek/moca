//go:build codemodecarry

package provider

import "sync/atomic"

// evalAttempts counts physical adapter attempts in this process. It exists in
// the codemodecarry build only, like the carry hook: the shipped binary carries
// no measurement code.
//
// The work-turn instrument counts logical requests (one turn() call can make
// several attempts; internal/provider/retry.go retries inside one), so "fewer
// requests" and "the same requests, fewer retries" are different findings and
// the campaign must be able to tell them apart.
var evalAttempts atomic.Int64

// evalAttempt records one physical attempt, called by the retry loop.
func evalAttempt() { evalAttempts.Add(1) }

// EvalAttempts is the cumulative physical-attempt count of this process,
// written into each carry observation (internal/agent/carry_hook.go).
func EvalAttempts() int { return int(evalAttempts.Load()) }
