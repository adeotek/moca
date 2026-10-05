package agent

import (
	"errors"
	"fmt"
	"os"
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
	// Validate before opening the writer: a session whose workdir is gone
	// must fail with a clear message and without appending repair entries
	// for a resume that never became usable.
	head, err := session.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(head) == 0 || head[0].Session == nil {
		return nil, fmt.Errorf("%s: not a moca session", path)
	}
	h := head[0].Session
	if fi, err := os.Stat(h.Workdir); err != nil || !fi.IsDir() {
		if err == nil {
			err = errors.New("not a directory")
		}
		return nil, fmt.Errorf("session workdir %s is not available: %w", h.Workdir, err)
	}
	w, entries, err := session.Open(path)
	if err != nil {
		return nil, err
	}
	for _, fix := range session.Repair(entries, session.Interrupted) {
		e, err := w.Append(fix)
		if err != nil {
			w.Close()
			return nil, err
		}
		entries = append(entries, e)
	}
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
		return nil, fmt.Errorf("resume: session workdir %s: %w", h.Workdir, err)
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
