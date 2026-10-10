package agent

import (
	"context"
	"errors"
	"log/slog"

	"github.com/adeotek/moca/internal/session"
)

// logger is the slog default tagged with this session's id8 (SPECS §13.5),
// resolved per call: cmd/moca installs the default before any agent exists,
// and tests swap it.
func (a *Agent) logger() *slog.Logger {
	if a.opts.Session == nil {
		return slog.Default()
	}
	return slog.Default().With("session", a.opts.Session.ID8())
}

// nudge persists one bounded nudge (§14) and logs its kind.
func (a *Agent) nudge(kind, text string) error {
	a.logger().Info("nudge", "kind", kind)
	_, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(text)})
	return err
}

// outcomeKind names how a run ended, for the run-end line.
func outcomeKind(out Outcome, err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "interrupted"
	case err != nil:
		return "error"
	case out.MaxSteps:
		return "max_steps"
	case out.Stuck:
		return "stuck"
	}
	return "done"
}
