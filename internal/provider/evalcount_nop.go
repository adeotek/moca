//go:build !codemodecarry

package provider

// evalAttempt is a no-op in a normal build, so the retry loop carries no
// counter; the call compiles away.
func evalAttempt() {}

// EvalAttempts is zero in a normal build: nothing counts attempts, and the
// carry hook that would read the number does not exist either.
func EvalAttempts() int { return 0 }
