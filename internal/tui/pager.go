package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// pagerModel is the alt-screen view of one item's body (ctrl+o / /show n).
// `/` searches the body (case-insensitive), `n`/`N` step through the matches.
type pagerModel struct {
	vp    viewport.Model
	title string
	body  string

	typing bool   // the search query is being typed
	query  string // the query being typed / last applied
	hits   int    // matches of the applied query
	cur    int    // 0-based current match
}

func newPager(it Item, w, h int) *pagerModel {
	vp := viewport.New(viewport.WithWidth(w), viewport.WithHeight(max(3, h-1)))
	// Reasoning and tool output are long prose lines: wrap them instead of
	// cutting them off at the right edge.
	vp.SoftWrap = true
	vp.HighlightStyle = lipgloss.NewStyle().Reverse(true)
	vp.SelectedHighlightStyle = lipgloss.NewStyle().Reverse(true).Bold(true).Foreground(lipgloss.Color("#d97706"))
	vp.SetContent(it.Body)
	return &pagerModel{vp: vp, title: it.Line, body: it.Body}
}

func (p *pagerModel) resize(w, h int) {
	p.vp.SetWidth(w)
	p.vp.SetHeight(max(3, h-1))
}

// matchRanges returns the byte ranges of query in body, ignoring case when
// lowercasing keeps the byte offsets (it does for all but exotic scripts).
func matchRanges(body, query string) [][]int {
	if query == "" {
		return nil
	}
	hay, needle := body, query
	if lb, lq := strings.ToLower(body), strings.ToLower(query); len(lb) == len(body) && len(lq) == len(query) {
		hay, needle = lb, lq
	}
	var out [][]int
	for off := 0; ; {
		i := strings.Index(hay[off:], needle)
		if i < 0 {
			break
		}
		out = append(out, []int{off + i, off + i + len(needle)})
		off += i + len(needle)
	}
	return out
}

// search applies the typed query: highlight, jump to the first match at or
// below the current position.
func (p *pagerModel) search() {
	p.vp.ClearHighlights()
	ranges := matchRanges(p.body, p.query)
	p.hits, p.cur = len(ranges), 0
	if len(ranges) > 0 {
		p.vp.SetHighlights(ranges)
	}
}

// update returns false when the pager should close.
func (p *pagerModel) update(msg tea.Msg) (bool, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if p.typing {
			switch k.String() {
			case "esc":
				p.typing, p.query = false, ""
				p.vp.ClearHighlights()
				p.hits = 0
			case "enter":
				p.typing = false
				p.search()
			case "backspace":
				if r := []rune(p.query); len(r) > 0 {
					p.query = string(r[:len(r)-1])
				}
			default:
				if k.Text != "" {
					p.query += k.Text
				}
			}
			return true, nil
		}
		switch k.String() {
		case "q", "esc":
			return false, nil
		case "/":
			p.typing, p.query = true, ""
			return true, nil
		case "n":
			if p.hits > 0 {
				p.vp.HighlightNext()
				p.cur = (p.cur + 1) % p.hits
			}
			return true, nil
		case "N":
			if p.hits > 0 {
				p.vp.HighlightPrevious()
				p.cur = (p.cur - 1 + p.hits) % p.hits
			}
			return true, nil
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

// view is the title row (clamped to one row, so the body keeps its height)
// with the state on its right — the search being typed, the match position,
// or the scroll position — then the body. pending marks an approval waiting
// behind the pager.
func (p *pagerModel) view(pending bool) string {
	var suffix string
	switch {
	case p.typing:
		suffix = "  /" + Sanitize(p.query) + "▏ enter search · esc cancel"
	case p.query != "" && p.hits == 0:
		suffix = fmt.Sprintf("  /%s: no match · / search · q close", Sanitize(p.query))
	case p.query != "":
		suffix = fmt.Sprintf("  /%s %d/%d · n/N · / search · q close", Sanitize(p.query), p.cur+1, p.hits)
	default:
		suffix = fmt.Sprintf("  %d%% · / search · q close", int(p.vp.ScrollPercent()*100))
	}
	if pending {
		suffix = "  " + "⚠ approval pending — q to answer" + suffix
	}
	title := p.title
	if room := p.vp.Width() - lipgloss.Width(suffix); lipgloss.Width(title) > room {
		title = truncCells(title, max(0, room-1)) + "…"
	}
	return title + dim.Render(suffix) + "\n" + p.vp.View()
}
