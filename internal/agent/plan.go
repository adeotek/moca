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
// permission rules; the envelope rides every user message *sent while plan
// mode is on* (§14). Plan rides only on default: turning plan on leaves a
// wider mode (auto, yolo) for default first (its own ModeChanged) —
// previously only yolo did.
func (a *Agent) SetPlan(on bool) {
	was := a.plan
	if on && a.mode != ModeDefault {
		a.SetMode(ModeDefault)
	}
	a.applyPlan(on)
	a.append(session.Entry{Type: session.TypePermissionMode,
		PermissionMode: &session.PermissionMode{Yolo: a.mode == ModeYolo, Plan: on, Mode: string(a.mode)}})
	if was && !on {
		// The envelopes stay in the history — they were part of what the
		// model received, and the history is append-only. This note tells the
		// model they no longer apply (the next request carries it once).
		a.append(session.Entry{Type: session.TypeMessage, Message: userText(planOffNote)})
	}
	a.emit(PlanChanged{On: on})
}

// planEnvelope rides user text messages appended while plan mode was on — a
// request-only render (like the cross-provider thinking transform): the
// transcript keeps exactly what the user wrote, plus the entry's plan flag.
// The flag lives on the entry, so an earlier message renders byte-identically
// in every later request and toggling the mode costs one cache miss instead
// of a rewrite of the whole history (B1.1, §14).
const planEnvelope = "[plan mode] You are producing an implementation plan, not making changes. " +
	"Investigate with read/search/ls/shell/web/mcp as needed — shell and mcp run under the normal permission rules, and you must use them only to inspect, never to change anything. " +
	"The write and edit tools refuse any path outside docs/plans. " +
	"Then write the plan to docs/plans/<kebab-case-slug>.md with the write tool (edit to update an existing one) and stop with a 3-5 line summary naming the file. " +
	"Plan format: a one-paragraph goal; current state (file paths); numbered steps as `- [ ]` checkboxes (each: what changes, which files, how it is verified); then verification, out-of-scope and risks/decisions sections. " +
	"Do not modify any other file."

// planOffNote is appended once when plan mode turns off. The envelopes
// attached to earlier messages cannot be un-sent, so the note says they no
// longer apply.
const planOffNote = "[plan mode is off] The plan-mode instructions attached to earlier messages no longer apply: you may change files again under the normal permission rules."

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

// planExtra is the per-entry hook session.MessagesWith runs for each user
// message entry: a flagged entry (appended while plan mode was on, §13) gets
// the envelope appended immediately after its own blocks — before any later
// entry merges in, so an entry appended later can never move or drop the
// envelope an earlier request already carried.
func planExtra(e session.Entry) []llm.ContentBlock {
	if !e.Plan || e.Message == nil || e.Message.Role != llm.RoleUser {
		return nil
	}
	for _, b := range e.Message.Content {
		if b.Type == llm.BlockText {
			return []llm.ContentBlock{{Type: llm.BlockText, Text: planEnvelope}}
		}
	}
	return nil
}
