//go:build !codemodecarry

package agent

import "github.com/adeotek/moca/internal/llm"

// carryObserve is the shipped arm of the carry instrument: a normal build
// records nothing and imports no measurement code (the tagged arm is
// carry_hook.go). The call site is unconditional so the two builds differ by
// one file, not by a build condition scattered through the loop.
func (a *Agent) carryObserve(llm.Request) {}
