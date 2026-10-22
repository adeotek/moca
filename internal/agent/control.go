package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
)

type savedModel struct {
	model  string
	effort llm.Effort
}

// SteeringApplied is emitted when queued steering texts are appended to the
// transcript (after the current tool batch's results).
type SteeringApplied struct{ Texts []string }

func (SteeringApplied) isEvent() {}

// Status is a consistent snapshot for the TUI status bar (§11). It is safe to
// call while a run is in progress.
type Status struct {
	Model         provider.Model
	Effort        llm.Effort
	ContextTokens int
	Window        int
	Usage         llm.Usage
	Cost          float64
	Sub           bool
	Hard          bool
	Yolo          bool
	Plan          bool
}

// Models lists the catalog for /model (sorted by qualified id).
func (a *Agent) Models() []provider.Model { return a.opts.Providers.Models() }

// RefreshModels re-reads a configured Ollama server's models (models pulled
// since startup appear; removed ones go). It returns the reason when the
// server cannot be reached, and nil when no local provider is configured.
func (a *Agent) RefreshModels(ctx context.Context) error {
	_, err := a.opts.Providers.DiscoverOllama(ctx)
	return err
}

// HasCredential reports whether a request to the qualified model could be
// authenticated (the check SetModel makes: an env lookup or the stored
// login, never the network).
func (a *Agent) HasCredential(q string) bool { return a.opts.Providers.CheckCredential(q) == nil }

// History is the conversation as the model sees it (after any compaction
// summary): a copy of the stored messages, thinking blocks included, for a
// front end to replay.
func (a *Agent) History() []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	return session.Messages(a.entries)
}

// NudgeTexts maps each user message a bounded nudge produced (§14) to the
// nudge's name. A front end replaying History() gets plain user messages, so a
// resumed session showed "record what this run proved …" as if the user had
// typed it — the marker lives on the entry, not in llm.Message, because the
// message is what goes on the wire (P3.8).
func (a *Agent) NudgeTexts() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]string)
	for _, e := range a.entries {
		if e.Type != session.TypeMessage || e.Nudge == "" || e.Message == nil {
			continue
		}
		if t := strings.TrimSpace(llm.TextOf(*e.Message)); t != "" {
			out[t] = e.Nudge
		}
	}
	return out
}

// recordModel persists a model_change entry and drops the usage anchor: after
// a switch the estimate falls back to chars/4 of the whole request (§6).
func (a *Agent) recordModel() {
	a.append(session.Entry{Type: session.TypeModelChange,
		ModelChange: &session.ModelChange{Model: a.model.Qualified(), Effort: string(a.effort)}})
	a.mu.Lock()
	a.anchorValid = false
	a.mu.Unlock()
}

// SetModel switches model (and effort; "" = the model's default) for the next
// run/turn. The provider credential is checked first (an env lookup, or a stored
// login for OAuth; never the network), so a missing key refuses the switch and nothing changes. Callers
// must not switch while a run is in progress (the TUI refuses; §11).
func (a *Agent) SetModel(q string, e llm.Effort) error {
	if err := a.opts.Providers.CheckCredential(q); err != nil {
		return err
	}
	prev, prevAd, prevE := a.model, a.adapter, a.effort
	if err := a.setModel(q, e); err != nil {
		a.model, a.adapter, a.effort = prev, prevAd, prevE
		return err
	}
	a.recordModel()
	return nil
}

// SetEffort clamps to the current model's supported levels and records the
// effective level.
func (a *Agent) SetEffort(e llm.Effort) (llm.Effort, error) {
	a.effort = a.model.ClampEffort(e)
	a.recordModel()
	return a.effort, nil
}

func (a *Agent) HardOn() bool { return a.hard != nil }

// restoreHard re-enters hard mode after a resume: the saved pair is not in
// the transcript, so it is the configured default model at its default effort
// (what /hard would have saved). A default that no longer resolves leaves hard
// mode off.
func (a *Agent) restoreHard() {
	m, _, err := a.opts.Providers.Resolve(a.opts.Config.Model)
	if err != nil {
		return
	}
	a.hard = &savedModel{m.Qualified(), m.DefaultEffort()}
}

// Carry is the model and effort a replacement session (/clear) should start
// with: the current pair, or the saved pair while hard mode is on (the new
// session starts outside hard mode, so /hard restores correctly).
func (a *Agent) Carry() (string, llm.Effort) {
	if a.hard != nil {
		return a.hard.model, a.hard.effort
	}
	return a.model.Qualified(), a.effort
}

// ToggleHard switches to modelHard at high effort (clamped) and back to the
// saved pair on the next call (§11).
func (a *Agent) ToggleHard() (bool, error) {
	if a.hard != nil {
		saved := *a.hard
		if err := a.SetModel(saved.model, saved.effort); err != nil {
			return true, err
		}
		a.hard = nil
		return false, nil
	}
	if a.opts.Config.ModelHard == "" {
		return false, errors.New("modelHard is not configured")
	}
	saved := savedModel{a.model.Qualified(), a.effort}
	if err := a.SetModel(a.opts.Config.ModelHard, llm.EffortHigh); err != nil {
		return false, err
	}
	a.hard = &saved
	return true, nil
}

// steerMsg is one queued steering message: what is appended to the
// transcript, and the part of it the user typed ("" for none).
type steerMsg struct{ text, typed string }

// Steer queues a message to be appended after the current tool batch's
// results (thread-safe: the TUI calls it from another goroutine). typed is
// the part the user typed — a mid-run "fix the deploy skill too" names it
// (Decision 15); the text alone is not typed.
func (a *Agent) Steer(text, typed string) {
	a.mu.Lock()
	a.steer = append(a.steer, steerMsg{text: text, typed: typed})
	a.mu.Unlock()
}

// PendingSteering is the number of queued steering messages not yet applied.
func (a *Agent) PendingSteering() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.steer)
}

// TakeSteering drains the queue (an aborted run's texts return to the editor).
func (a *Agent) TakeSteering() []string {
	msgs := a.takeSteering()
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.text
	}
	return out
}

func (a *Agent) takeSteering() []steerMsg {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.steer
	a.steer = nil
	return s
}

// applySteering appends queued texts as user messages; reports whether any.
// A steering message's typed part joins the run's typed text (Decision 15),
// and any steering arms the one-shot skill hint.
func (a *Agent) applySteering() (bool, error) {
	msgs := a.takeSteering()
	for i := 0; i < len(msgs); i++ {
		if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(msgs[i].text)}); err != nil {
			// A failed append must not swallow the rest of the queue:
			// re-queue the messages that did not land — this one included
			// (an aborted run's texts return to the editor through
			// TakeSteering).
			a.mu.Lock()
			a.steer = append(msgs[i:], a.steer...)
			a.mu.Unlock()
			return false, err
		}
	}
	if len(msgs) > 0 {
		texts := make([]string, len(msgs))
		for i, m := range msgs {
			texts[i] = m.text
			if m.typed != "" {
				a.opts.Env.SkillTyped = append(a.opts.Env.SkillTyped, m.typed)
			}
		}
		a.opts.Env.SkillHintArmed = true // any steering arms the hint, never the signal
		a.emit(SteeringApplied{Texts: texts})
	}
	return len(msgs) > 0, nil
}

// AddNote appends a user message without running (the `!` prefix).
func (a *Agent) AddNote(text string) error {
	_, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(text)})
	return err
}

// Undo reverts the most recent snapshotted write/edit of this session (§4).
func (a *Agent) Undo() (string, error) { return a.opts.Snapshots.Undo() }

// ContextTokens is the §6 estimate of the next request:
// usage-anchored when possible, chars/4 of the whole request otherwise.
func (a *Agent) ContextTokens() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.contextTokensLocked()
}

func (a *Agent) contextTokensLocked() int {
	if a.anchorValid {
		n := 0
		for _, m := range session.Messages(a.entries[a.anchorEntries:]) {
			n += compact.MessageChars(m)
		}
		return a.anchorTokens + compact.Tokens(n)
	}
	return compact.Tokens(compact.RequestChars(a.opts.System, a.opts.Tools.Specs(),
		TransformHistory(session.MessagesWith(a.entries, planExtra, a.model.KeepsTailThinking()), a.model.Qualified())))
}

func (a *Agent) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Status{Model: a.model, Effort: a.effort, ContextTokens: a.contextTokensLocked(), Window: a.model.ContextWindow,
		Usage: a.usage, Cost: a.cost, Sub: a.opts.Config.Providers[a.model.Provider].Auth == "oauth", Hard: a.hard != nil, Yolo: a.mode == ModeYolo, Plan: a.plan}
}
