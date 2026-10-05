package tui

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// API notes — verified against charm.land/bubbletea v2.0.10, bubbles v2.2.1,
// lipgloss v2.0.6 (2026-10-05). The glue layer (app.go, bridge.go, pager.go,
// trust.go) may rely only on symbols checked here:
//
//   - key presses: tea.KeyPressMsg (alias of tea.Key); .String() yields
//     "enter", "shift+enter", "alt+enter", "ctrl+j", "esc", "ctrl+c", "alt+p",
//     "up"/"down"/"pgup"/"pgdown", "g"/"G".
//   - paste: tea.PasteMsg{Content string}.
//   - enhancements: tea.KeyboardEnhancementsMsg{Flags int},
//     .SupportsKeyDisambiguation() — the shift+enter signal.
//   - request on the view: v.KeyboardEnhancements =
//     tea.KeyboardEnhancements{ReportAllKeysAsEscapeCodes: true}.
//   - tea.Println(args ...any) tea.Cmd · tea.SetClipboard(s string) tea.Cmd ·
//     tea.Quit is func() Msg (assignable to tea.Cmd).
//   - program: tea.NewProgram(m, tea.WithContext(ctx)) · p.Send(msg) ·
//     p.Run() (Model, error) · p.Kill().
//   - textarea.New() Model · SetHeight/SetWidth/SetValue/Value/Focus/InsertString.
//   - viewport.New(viewport.WithWidth(w), viewport.WithHeight(h)) ·
//     SetContent/GotoTop/GotoBottom/Update/View.
func TestTeaAPI(t *testing.T) {
	var _ tea.KeyPressMsg
	var _ tea.PasteMsg
	var _ tea.KeyboardEnhancementsMsg
	var _ tea.WindowSizeMsg
	var _ tea.Cmd = tea.Println("x")
	var _ tea.Cmd = tea.SetClipboard("x")
	var _ tea.Cmd = tea.Quit
	v := tea.NewView("x")
	v.AltScreen = true
	v.KeyboardEnhancements = tea.KeyboardEnhancements{ReportAllKeysAsEscapeCodes: true}
	_ = v
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	ta.SetHeight(3)
	ta.SetWidth(40)
	ta.SetValue("x")
	_ = ta.Value()
	_ = ta.Focus()
	ta.InsertString("\n")
	vp := viewport.New(viewport.WithWidth(10), viewport.WithHeight(5))
	vp.SetContent("x")
	vp.GotoTop()
	vp.GotoBottom()
	_ = vp.View()
}
