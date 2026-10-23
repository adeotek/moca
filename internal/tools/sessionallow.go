package tools

// RememberSession wraps an Asker with the in-memory "allow for this session"
// set (rev 35). Scope is one Agent — one session: the map lives here, so
// /clear and /resume (a new Agent) start empty, /mode, /model and /effort
// keep it, and each ACP session is isolated by construction. Nothing is ever
// written to disk; only AllowAlways persists, and only the TUI does that.

import (
	"context"
	"sync"
)

// RememberSession returns an Asker that answers a remembered question with
// AllowSession without consulting inner, and records every AllowSession the
// inner asker returns. A nil inner stays nil — a -p run without an asker must
// keep denying at every `env.Ask != nil` site rather than gain a silent
// allow-by-memory.
//
// A NoSession question (moca's own config file, a web redirect hop) is never
// answered from memory and never recorded; an AllowSession answer to one —
// only a non-conforming UI can produce it — is downgraded to AllowOnce, the
// same pin as allow-always on a question that forbade it.
func RememberSession(inner Asker) Asker {
	if inner == nil {
		return nil
	}
	var mu sync.Mutex
	remembered := map[string]bool{}
	return func(ctx context.Context, q Question) Answer {
		// A cancelled run must not spend a remembered allow: the call is
		// being abandoned, so deny (the same answer every wait returns).
		if ctx.Err() != nil {
			return Deny
		}
		if !q.NoSession {
			mu.Lock()
			ok := remembered[q.SessionKey()]
			mu.Unlock()
			if ok {
				return AllowSession
			}
		}
		ans := inner(ctx, q)
		if ans != AllowSession {
			return ans
		}
		if q.NoSession {
			return AllowOnce
		}
		mu.Lock()
		remembered[q.SessionKey()] = true
		mu.Unlock()
		return AllowSession
	}
}
