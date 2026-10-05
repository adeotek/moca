package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// trustModel is the one-time project-trust prompt on first open of an
// untrusted workdir that has project resources (§7).
type trustModel struct {
	dir    string
	answer bool
	done   bool
}

func (m *trustModel) Init() tea.Cmd { return nil }

func (m *trustModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "y", "Y":
			m.answer, m.done = true, true
			return m, tea.Quit
		case "n", "N", "enter", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *trustModel) View() tea.View {
	return tea.NewView(fmt.Sprintf("Trust project resources in %s? (.moca/, AGENTS.md/CLAUDE.md can steer the agent) [y/N] ", m.dir))
}

// RunTrustPrompt asks once; the caller saves the decision (trust.json).
func RunTrustPrompt(dir string) (bool, error) {
	m := &trustModel{dir: dir}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		return false, err
	}
	return m.answer, nil
}
