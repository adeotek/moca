package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// trustModel is the one-time project-trust prompt on first open of an
// untrusted workdir that has project resources (§7).
type trustModel struct {
	dir       string
	answer    bool
	done      bool
	cancelled bool // ctrl+c / esc: abort startup, decide nothing
}

func (m *trustModel) Init() tea.Cmd { return nil }

func (m *trustModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "y", "Y":
			m.answer, m.done = true, true
			return m, tea.Quit
		case "n", "N", "enter":
			m.done = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.done, m.cancelled = true, true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *trustModel) View() tea.View {
	return tea.NewView(fmt.Sprintf("Trust project resources in %s? (.moca/, AGENTS.md/CLAUDE.md can steer the agent) [y/N] ", Sanitize(m.dir)))
}

// result maps the finished prompt to the decision; a cancel is reported as
// context.Canceled (exit 130) and must not be saved as an answer.
func (m *trustModel) result() (bool, error) {
	if m.cancelled {
		return false, fmt.Errorf("trust prompt cancelled: %w", context.Canceled)
	}
	return m.answer, nil
}

// RunTrustPrompt asks once; the caller saves the decision (trust.json). The
// prompt runs on the caller's signal context with Bubble Tea's own signal
// handler off — the same shutdown rule as the main program (a second handler
// races the first and can hang the exit). ctrl+c/esc cancel, as does the
// context; every cancel is an error wrapping context.Canceled (quiet 130)
// and nothing is saved.
func RunTrustPrompt(ctx context.Context, dir string) (bool, error) {
	m := &trustModel{dir: dir}
	if _, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithoutSignalHandler()).Run(); err != nil {
		if ctx.Err() != nil {
			return false, fmt.Errorf("trust prompt interrupted: %w", context.Canceled)
		}
		return false, err
	}
	return m.result()
}
