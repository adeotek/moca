package tui

import (
	"fmt"
	"strings"
	"time"
)

const chipThreshold = 50

type paste struct {
	marker, content string
	unsafe          bool // contains control bytes: never expanded into the buffer
	expanded        bool
}

// Input is the pure input state machine (no Bubble Tea types). The buffer is
// exactly what the textarea shows; large or control-byte pastes live aside
// behind a plain-text chip marker (`[paste N lines #K]`) that is edited like
// normal text.
type Input struct {
	buf         string
	pastes      []*paste
	history     []string
	hpos        int
	draft       string
	lastEmptyCC time.Time
}

func NewInput() *Input { return &Input{} }

func (in *Input) SetBuffer(s string) { in.buf = s }
func (in *Input) Buffer() string     { return in.buf }
func (in *Input) Insert(s string)    { in.buf += s }

func lineCount(s string) int { return strings.Count(strings.TrimSuffix(s, "\n"), "\n") + 1 }

// Paste inserts verbatim; a paste over the line threshold or with control
// bytes becomes a chip (the full content is kept until send).
func (in *Input) Paste(s string) {
	unsafe := hasControl(s)
	if !unsafe && lineCount(s) <= chipThreshold {
		in.buf += s
		return
	}
	n := lineCount(s)
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	p := &paste{marker: fmt.Sprintf("[paste %d %s #%d]", n, unit, len(in.pastes)+1), content: s, unsafe: unsafe}
	in.pastes = append(in.pastes, p)
	in.buf += p.marker
}

// Text is what gets sent: intact markers replaced by their full content.
func (in *Input) Text() string {
	t := in.buf
	for _, p := range in.pastes {
		if !p.expanded {
			t = strings.Replace(t, p.marker, p.content, 1)
		}
	}
	return t
}

func (in *Input) Display() string { return Sanitize(in.buf) }

// ToggleChips expands every intact marker into the buffer, or collapses the
// expanded contents back. Control-byte pastes never expand.
func (in *Input) ToggleChips() {
	anyCollapsed := false
	for _, p := range in.pastes {
		if !p.expanded && !p.unsafe && strings.Contains(in.buf, p.marker) {
			anyCollapsed = true
		}
	}
	for _, p := range in.pastes {
		switch {
		case p.unsafe:
		case anyCollapsed && !p.expanded && strings.Contains(in.buf, p.marker):
			in.buf, p.expanded = strings.Replace(in.buf, p.marker, p.content, 1), true
		case !anyCollapsed && p.expanded && strings.Contains(in.buf, p.content):
			in.buf, p.expanded = strings.Replace(in.buf, p.content, p.marker, 1), false
		}
	}
}

func (in *Input) clear() { in.buf, in.pastes = "", nil }

// Submit returns the message and clears the buffer; history keeps the
// expanded text (recall is explicit).
func (in *Input) Submit() string {
	t := in.Text()
	if strings.TrimSpace(t) != "" {
		in.history = append(in.history, t)
	}
	in.hpos, in.draft = len(in.history), ""
	in.clear()
	return t
}

// HistoryPrev recalls the previous entry; the glue calls it only when the
// cursor is on the first line and the buffer is single-line.
func (in *Input) HistoryPrev() bool {
	if strings.Contains(in.buf, "\n") || in.hpos == 0 {
		return false
	}
	if in.hpos == len(in.history) {
		in.draft = in.Text()
	}
	in.hpos--
	in.clear()
	in.buf = in.history[in.hpos]
	return true
}

func (in *Input) HistoryNext() bool {
	if strings.Contains(in.buf, "\n") || in.hpos >= len(in.history) {
		return false
	}
	in.hpos++
	in.clear()
	if in.hpos == len(in.history) {
		in.buf = in.draft
	} else {
		in.buf = in.history[in.hpos]
	}
	return true
}

// CtrlC clears a non-empty input; on an empty input it reports quit only for
// a second press within one second.
func (in *Input) CtrlC(now time.Time) bool {
	if in.Text() != "" {
		in.clear()
		in.lastEmptyCC = time.Time{}
		return false
	}
	if !in.lastEmptyCC.IsZero() && now.Sub(in.lastEmptyCC) <= time.Second {
		return true
	}
	in.lastEmptyCC = now
	return false
}

// Prepend puts returned steering texts above the current draft.
func (in *Input) Prepend(texts []string) {
	if len(texts) == 0 {
		return
	}
	pre := strings.Join(texts, "\n")
	if in.buf != "" {
		pre += "\n"
	}
	in.buf = pre + in.buf
}
