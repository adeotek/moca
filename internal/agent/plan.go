package agent

import (
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// PlanChanged is emitted when plan mode is toggled.
type PlanChanged struct{ On bool }

func (PlanChanged) isEvent() {}

// Plan reports whether plan mode is on: runs analyze the request and write
// an implementation plan to docs/plans/<slug>.md — nothing else changes.
func (a *Agent) Plan() bool { return a.plan }

func (a *Agent) applyPlan(on bool) {
	a.plan = on
	a.opts.Env.Plan = on
}

// SetPlan toggles plan mode between runs (never during Run). The tools layer
// enforces the write confinement (write/edit only reach docs/plans/*.md);
// shell, mcp, web and the read tools stay available under their normal
// permission rules; the envelope rides every request while it is on.
func (a *Agent) SetPlan(on bool) {
	a.applyPlan(on)
	a.append(session.Entry{Type: session.TypePermissionMode, PermissionMode: &session.PermissionMode{Yolo: a.yolo, Plan: on}})
	a.emit(PlanChanged{On: on})
}

// planEnvelope is appended to every user text message of every request
// built while plan mode is on — a request-only transform (like the
// cross-provider thinking transform): the transcript keeps exactly what the
// user wrote. Every message, not just the last: a moving envelope would
// rewrite an earlier message on each new prompt and forfeit the prompt cache
// from there (B4); toggling the mode costs one cache miss instead.
const planEnvelope = "[plan mode] You are producing an implementation plan, not making changes. " +
	"Investigate with read/search/ls/shell/web/mcp as needed — shell and mcp run under the normal permission rules, and you must use them only to inspect, never to change anything. " +
	"The write and edit tools refuse any path outside docs/plans. " +
	"Then write the plan to docs/plans/<kebab-case-slug>.md with the write tool (edit to update an existing one) and stop with a 3-5 line summary naming the file. " +
	"Plan format: a one-paragraph goal; current state (file paths); numbered steps as `- [ ]` checkboxes (each: what changes, which files, how it is verified); then verification, out-of-scope and risks/decisions sections. " +
	"Do not modify any other file."

// planNudge is persisted and answered when a plan-mode run is about to end
// without its deliverable: one bounded retry, then the run ends with a
// warning (§14, mirroring the maxSteps wrap-up).
const planNudge = "You are in plan mode and this run has not written a plan file under docs/plans/ yet. " +
	"Write the deliverable now (write for a new plan, edit to update one), then summarize in 3-5 lines."

// planReserveText is persisted two steps before a plan run's tool-less
// wrap-up when its file is still missing (§14).
const planReserveText = "You are running out of steps for this plan-mode run. Write the plan file under docs/plans/ now " +
	"with what you know (mark open questions in its risks section), then summarize in 3-5 lines."

// warnPlanMissing flags a plan run ending without its deliverable — after
// the nudge, or at the tool-less maxSteps wrap-up.
func (a *Agent) warnPlanMissing() {
	if a.plan && !a.opts.Env.PlanWrote {
		a.emit(Warning{"plan mode: the run finished without a plan file under docs/plans/ — the deliverable is missing"})
	}
}

// withPlanEnvelope appends the envelope to every user message that carries
// text (never a tool-result-only message).
func withPlanEnvelope(msgs []llm.Message, on bool) []llm.Message {
	if !on {
		return msgs
	}
	for i := range msgs {
		if msgs[i].Role != llm.RoleUser {
			continue
		}
		hasText := false
		for _, b := range msgs[i].Content {
			if b.Type == llm.BlockText {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}
		content := make([]llm.ContentBlock, 0, len(msgs[i].Content)+1)
		content = append(content, msgs[i].Content...)
		content = append(content, llm.ContentBlock{Type: llm.BlockText, Text: planEnvelope})
		msgs[i].Content = content
	}
	return msgs
}
