package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// pagerModel is the alt-screen view of one item's body (ctrl+o / /show n).
type pagerModel struct {
	vp    viewport.Model
	title string
}

func newPager(it Item, w, h int) *pagerModel {
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(max(3, h-1)))
	// Reasoning and tool output are long prose lines: wrap them instead of
	// cutting them off at the right edge.
	vp.SoftWrap = true
	vp.SetContent(it.Body)
	return &pagerModel{vp: vp, title: it.Line}
}

func (p *pagerModel) resize(w, h int) {
	p.vp.SetWidth(w)
	p.vp.SetHeight(max(3, h-1))
}

// update returns false when the pager should close.
func (p *pagerModel) update(msg tea.Msg) (bool, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "q", "esc":
			return false, nil
		case "g":
			p.vp.GotoTop()
			return true, nil
		case "G":
			p.vp.GotoBottom()
			return true, nil
		}
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return true, cmd
}

func (p *pagerModel) view() string { return p.title + "  (q to close)\n" + p.vp.View() }
