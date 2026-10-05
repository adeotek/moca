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
	at              int // buffer offset of the expanded content; -1 when collapsed
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

// Paste appends the paste at the end of the buffer (see Prepare for the
// cursor-relative form the glue uses).
func (in *Input) Paste(s string) { in.buf += in.Prepare(s) }

// Prepare registers a paste and returns the text to insert at the cursor:
// the content verbatim, or a chip marker for a paste over the line threshold
// or with control bytes (the full content is kept until send). CR and CRLF
// newlines (Windows sources, some terminals) are normalized to LF first —
// they are text, not control bytes.
func (in *Input) Prepare(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	unsafe := hasControl(s)
	if !unsafe && lineCount(s) <= chipThreshold {
		return s
	}
	n := lineCount(s)
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	// The marker must not collide with text the user typed (or quoted) or
	// with another paste's marker: Text()/ToggleChips map markers to content
	// by string matching, and a collision would expand the wrong occurrence.
	var marker string
	for k := len(in.pastes) + 1; ; k++ {
		marker = fmt.Sprintf("[paste %d %s #%d]", n, unit, k)
		if !strings.Contains(in.buf, marker) && !in.markerExists(marker) {
			break
		}
	}
	p := &paste{marker: marker, content: s, unsafe: unsafe, at: -1}
	in.pastes = append(in.pastes, p)
	return p.marker
}

func (in *Input) markerExists(m string) bool {
	for _, p := range in.pastes {
		if p.marker == m {
			return true
		}
	}
	return false
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
// expanded contents back. Control-byte pastes never expand. Collapse prefers
// the remembered expansion offset (the first string occurrence may be
// identical typed text), falling back to string matching when the buffer was
// edited under it.
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
			idx := strings.Index(in.buf, p.marker)
			in.buf = strings.Replace(in.buf, p.marker, p.content, 1)
			p.expanded, p.at = true, idx
		case !anyCollapsed && p.expanded:
			if p.at >= 0 && p.at+len(p.content) <= len(in.buf) && in.buf[p.at:p.at+len(p.content)] == p.content {
				in.buf = in.buf[:p.at] + p.marker + in.buf[p.at+len(p.content):]
				p.expanded, p.at = false, -1
			} else if strings.Contains(in.buf, p.content) {
				in.buf, p.expanded, p.at = strings.Replace(in.buf, p.content, p.marker, 1), false, -1
			}
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

// setFromHistory puts a recalled entry into the buffer. Entries are stored
// expanded; one carrying control bytes is re-chipped rather than put into the
// editable buffer raw (control bytes must never reach the terminal).
func (in *Input) setFromHistory(s string) {
	in.clear()
	if hasControl(s) {
		in.Paste(s)
		return
	}
	in.buf = s
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
	in.setFromHistory(in.history[in.hpos])
	return true
}

func (in *Input) HistoryNext() bool {
	if strings.Contains(in.buf, "\n") || in.hpos >= len(in.history) {
		return false
	}
	in.hpos++
	if in.hpos == len(in.history) {
		in.clear()
		in.buf = in.draft
	} else {
		in.setFromHistory(in.history[in.hpos])
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
