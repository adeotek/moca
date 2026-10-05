package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
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
// model_change (else the header). The resuming process's --model/--effort win
// over the stored pair and are recorded as a model_change. Unanswered tool
// calls from a crash get Interrupted results before anything else. Yolo is
// never restored from the file — it comes from the resuming process's
// flag/config (§7.5). A session another moca process has open is refused
// (session.ErrInUse): repairing its in-flight calls would corrupt it.
func Resume(o StartOptions, path string) (*Agent, error) {
	// Everything that can fail without touching the file runs before the
	// writer opens: a resume that cannot work must not append repair entries.
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
	stored, storedEffort := h.Model, llm.Effort(h.Effort)
	for _, e := range head {
		if e.Type == session.TypeModelChange {
			stored, storedEffort = e.ModelChange.Model, llm.Effort(e.ModelChange.Effort)
		}
	}
	// The resuming process's flags win (§12.5): --model replaces the stored
	// model (which may no longer be configured at all) and --effort the stored
	// level.
	model, effort := stored, storedEffort
	if o.Model != "" {
		model = o.Model
	}
	if o.Effort != "" {
		e, err := llm.ParseEffort(o.Effort)
		if err != nil {
			return nil, err
		}
		effort = e
	}
	st, err := prepare(o, h.Workdir)
	if err != nil {
		var se *StartError
		if errors.As(err, &se) {
			return nil, fmt.Errorf("resume: session workdir %s: %w", h.Workdir, err)
		}
		return nil, err
	}
	if _, _, err := st.reg.Resolve(model); err != nil {
		if errors.Is(err, provider.ErrUnknownModel) && o.Model == "" {
			return nil, fmt.Errorf("resume: the session's model is not configured (%w); pass --model <provider/model> to continue with another", err)
		}
		return nil, fmt.Errorf("resume: %w", err)
	}
	if model != stored {
		if err := st.reg.CheckCredential(model); err != nil {
			return nil, err
		}
	}

	w, entries, err := session.Open(path)
	if err != nil {
		if errors.Is(err, session.ErrInUse) {
			return nil, &StartError{err}
		}
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
	a, err := build(o, st, w, h.SystemPrompt, model, effort, entries, filepath.Join(config.DataDir(), "snapshot"))
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("resume: %w", err)
	}
	if model != stored || a.effort != storedEffort {
		a.recordModel()
	}
	if o.Model == "" && stored == o.Config.ModelHard && o.Config.Model != stored {
		a.restoreHard()
	}
	a.emit(Resumed{ID8: w.ID8(), Messages: len(session.Messages(entries))})
	return a, nil
}
