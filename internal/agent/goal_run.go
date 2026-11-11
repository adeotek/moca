package agent

// The goal driver (v0.8.0): RunWith runs the ordinary work round, then — only
// while a goal is active — asks the evidence reviewer and continues within
// the caps. Each round is a full a.run, so the nudge, verification,
// permission, compaction, cancellation and step-budget behaviour is the same
// as any other run; the driver only decides whether another round starts.

import (
	"context"
	"errors"
	"fmt"

	"github.com/adeotek/moca/internal/session"
)

// StartGoal starts a goal and works its first round.
func (a *Agent) StartGoal(ctx context.Context, objective string) (Outcome, error) {
	g, err := a.setGoal(objective)
	if err != nil {
		return Outcome{}, err
	}
	return a.RunWith(ctx, g.Objective, RunOpts{Typed: g.Objective, goalID: g.ID})
}

// ResumeGoal re-activates a paused goal with a fresh round/stall budget and
// works one round. Only this explicit call restarts goal work after a pause
// or a process restart.
func (a *Agent) ResumeGoal(ctx context.Context) (Outcome, error) {
	g, err := a.resumeGoalState()
	if err != nil {
		return Outcome{}, err
	}
	return a.RunWith(ctx, goalResumeText, RunOpts{goalID: g.ID, goalNudge: goalNudgeResume})
}

// runGoalLoop calls a.run for the first round and, while the session's goal
// is active, reviews the evidence and continues within the caps. It stops
// honestly at every abnormal outcome: a provider error, cancellation,
// MaxSteps, Stuck, a stall, the round budget, or a review failure pauses the
// goal instead of spending more, and the original work outcome/error is what
// the caller sees.
func (a *Agent) runGoalLoop(ctx context.Context, prompt string, o RunOpts) (out Outcome, err error) {
	total := 0
	round := func(p string, ro RunOpts) (Outcome, error) {
		a.steps = 0
		out, err := a.run(ctx, p, ro)
		total += a.steps
		return out, err
	}
	// The run-end log reports the whole goal run's steps, not the last round's.
	defer func() { a.steps = total }()

	out, err = round(prompt, o)
	for {
		if err != nil {
			// A provider failure or a cancellation pauses the goal (Esc stays
			// immediate cancellation): no automatic spend after an abnormal
			// end, and the fixed reason carries no provider text.
			if g, ok := a.Goal(); ok && g.Status == session.GoalActive {
				reason := "the run ended with a provider error — /goal resume continues from the current state"
				if errors.Is(err, context.Canceled) {
					reason = "the run was cancelled — /goal resume continues from the current state"
				}
				return out, a.pauseJoined(g.ID, reason, err)
			}
			return out, err
		}
		g, ok := a.Goal()
		if !ok || g.Status != session.GoalActive {
			return out, nil // no goal (or the user already paused/cleared): nothing to review
		}
		if out.MaxSteps || out.Stuck {
			return out, a.pauseJoined(g.ID, "the run ended at its step or stuck limit — /goal resume continues from the current state", err)
		}
		// Review BEFORE enforcing the round cap: the eighth continuation
		// still gets its completion check.
		verdict, rerr := a.reviewGoal(ctx, g)
		// Re-read after the review: a pause/clear/replace meanwhile wins.
		now, ok := a.Goal()
		if !ok || now.Status != session.GoalActive || now.ID != g.ID {
			return out, err
		}
		if rerr != nil {
			reason := "the goal reviewer failed — /goal resume continues from the current state"
			if errors.Is(rerr, errReviewOverflow) {
				reason = "the review request overflowed the context — run /compact, then /goal resume"
			}
			return out, a.pauseJoined(g.ID, reason, err)
		}
		switch verdict {
		case verdictComplete:
			if cerr := a.completeGoal(g.ID); cerr != nil && !errors.Is(cerr, errGoalChanged) {
				return out, errors.Join(err, cerr)
			}
			return out, err
		case verdictBlocked:
			return out, a.pauseJoined(g.ID, "the reviewer found the goal blocked on a human decision or missing access", err)
		}
		// CONTINUE. The round budget is checked after the review and against
		// the state the review saw.
		if g.Rounds >= goalMaxRounds {
			return out, a.pauseJoined(g.ID, fmt.Sprintf("the goal used its %d automatic continuation rounds — /goal resume starts a fresh budget", goalMaxRounds), err)
		}
		ng, cerr := a.continueGoal(g.ID, a.runBatches == 0)
		if cerr != nil {
			if errors.Is(cerr, errGoalChanged) {
				return out, err
			}
			return out, errors.Join(err, cerr)
		}
		if ng.Stalls >= goalMaxStalls {
			return out, a.pauseJoined(g.ID, fmt.Sprintf("the goal made no tool call in %d consecutive rounds — /goal resume continues from the current state", goalMaxStalls), err)
		}
		out, err = round(goalContinueText, RunOpts{goalID: g.ID, goalNudge: goalNudgeContinue})
	}
}

// pauseJoined pauses the goal and surfaces a failed transition without
// disguising the primary cause (workErr): errGoalChanged is not an error —
// the user's own transition won.
func (a *Agent) pauseJoined(id, reason string, workErr error) error {
	if perr := a.pauseWithReason(id, reason); perr != nil && !errors.Is(perr, errGoalChanged) {
		return errors.Join(workErr, perr)
	}
	return workErr
}
