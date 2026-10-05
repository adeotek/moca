package agent

import (
	"fmt"
	"path/filepath"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// Resumed reports a session opened by --resume/--continue: the TUI prints it
// once so the user knows the scrollback continues from a file.
type Resumed struct {
	ID8      string
	Messages int
}

func (Resumed) isEvent() {}

// Resume reopens a session file: the stored system prompt is reused verbatim
// (the same cache prefix and reproducible requests), the workdir from the
// header becomes the jail root, and the model/effort come from the latest
// model_change (else the header). Unanswered tool calls from a crash get
// Interrupted results before anything else. Yolo is never restored from the
// file — it comes from the resuming process's flag/config (§7.5).
func Resume(o StartOptions, path string) (*Agent, error) {
	w, entries, err := session.Open(path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 || entries[0].Session == nil {
		w.Close()
		return nil, fmt.Errorf("%s: not a moca session", path)
	}
	for _, fix := range session.Repair(entries, session.Interrupted) {
		e, err := w.Append(fix)
		if err != nil {
			w.Close()
			return nil, err
		}
		entries = append(entries, e)
	}
	h := entries[0].Session
	model, effort := h.Model, llm.Effort(h.Effort)
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			model, effort = e.ModelChange.Model, llm.Effort(e.ModelChange.Effort)
		}
	}
	a, err := build(o, h.Workdir, w, h.SystemPrompt, model, effort, entries,
		filepath.Join(config.DataDir(), "snapshot"))
	if err != nil {
		w.Close()
		return nil, err
	}
	if o.Model != "" && o.Model != model {
		if err := a.SetModel(o.Model, effort); err != nil {
			w.Close()
			return nil, err
		}
	}
	a.emit(Resumed{ID8: w.ID8(), Messages: len(session.Messages(entries))})
	return a, nil
}
