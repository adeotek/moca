package tui

// Markdown-lite for assistant responses. The scrollback is immutable and text
// streams line by line, so this is a per-line renderer with a little state
// across lines (inside a code fence or not) — not a document renderer:
//
//	**bold**  `code`  # headings  - bullets  > quotes  ```fenced code```
//
// Everything else (links, emphasis, tables, nested lists) prints as written.
// Spans never cross lines; an unmatched marker prints literally.
//
// The response band's background must survive the styling, so inline styles
// are raw SGR *toggles* (bold on/off, foreground set/default) — never a full
// reset. (ansi.Wrap re-opens a span on the continuation row of a wrapped
// line and closes it at the break — with a full reset, rewritten below.) The input is sanitized first, so model text cannot smuggle its own
// escape sequences in; the only escapes in a rendered row are ours.

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	sgrBoldOn  = "\x1b[1m"
	sgrBoldOff = "\x1b[22m"
	sgrFgOff   = "\x1b[39m"
)

// codeFg / code band colors: dark and light variants like the other bands.
var (
	codeFgDark, codeFgLight = "\x1b[38;2;242;197;124m", "\x1b[38;2;122;62;0m"
	codeBgDark, codeBgLight = lipgloss.Color("#16303d"), lipgloss.Color("#b6cdee")
)

func codeBandStyle(dark bool) lipgloss.Style {
	if dark {
		return lipgloss.NewStyle().Background(codeBgDark).Foreground(respFgDark)
	}
	return lipgloss.NewStyle().Background(codeBgLight).Foreground(respFgLight)
}

// mdState is what carries from one response line to the next.
type mdState struct{ inFence bool }

var (
	headingRe = regexp.MustCompile(`^\s{0,3}#{1,6}\s+(.*)$`)
	bulletRe  = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	quoteRe   = regexp.MustCompile(`^\s{0,3}>\s?(.*)$`)
)

// fenceLang reports whether a line opens/closes a code fence (``` or ~~~)
// and, for an opener, its info string.
func fenceLang(line string) (lang string, ok bool) {
	t := strings.TrimSpace(line)
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(t, f) {
			return strings.TrimSpace(strings.TrimLeft(t, f[:1])), true
		}
	}
	return "", false
}

// renderInline styles `code` and **bold** spans in one sanitized line.
func renderInline(s string, dark bool) string {
	fg := codeFgDark
	if !dark {
		fg = codeFgLight
	}
	var sb strings.Builder
	for i := 0; i < len(s); {
		switch {
		case s[i] == '`':
			if j := strings.IndexByte(s[i+1:], '`'); j > 0 {
				sb.WriteString(fg + s[i+1:i+1+j] + sgrFgOff)
				i += j + 2
				continue
			}
		case strings.HasPrefix(s[i:], "**"):
			if j := strings.Index(s[i+2:], "**"); j > 0 {
				sb.WriteString(sgrBoldOn + renderInline(s[i+2:i+2+j], dark) + sgrBoldOff)
				i += j + 4
				continue
			}
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

// renderMarkdown styles text (one or more complete response lines) as
// full-width bands of the response style, switching to the code style inside
// fences. st carries the fence state across calls; width ≤ 0 means no wrap
// and no fill. Like styleBlock it sanitizes first and expands tabs.
func renderMarkdown(st *mdState, s string, width int, dark bool) string {
	resp, code := respMsgStyle(dark), codeBandStyle(dark)
	s = strings.ReplaceAll(Sanitize(s), "\t", strings.Repeat(" ", tabWidth))
	var out []string
	band := func(style lipgloss.Style, rows []string) {
		for _, r := range rows {
			row := " " + r
			if pad := width - lipgloss.Width(row); pad > 0 {
				row += strings.Repeat(" ", pad)
			}
			out = append(out, style.Render(row))
		}
	}
	for _, l := range strings.Split(s, "\n") {
		if lang, isFence := fenceLang(l); isFence {
			label := ""
			if !st.inFence { // opener
				label = "─"
				if lang != "" {
					label += " " + lang
				}
			}
			st.inFence = !st.inFence
			band(code, []string{ansi.Strip(label)})
			continue
		}
		if st.inFence {
			// Code is never word-wrapped (a token split across rows cannot be
			// read or copied); only a row wider than the screen is cut at the
			// width so the band survives.
			rows := []string{l}
			if width > 2 {
				rows = strings.Split(ansi.Hardwrap(l, width-2, true), "\n")
			}
			band(code, rows)
			continue
		}
		bold := false
		switch {
		case headingRe.MatchString(l):
			l, bold = headingRe.FindStringSubmatch(l)[1], true
		case bulletRe.MatchString(l):
			m := bulletRe.FindStringSubmatch(l)
			l = m[1] + "• " + m[2]
		case quoteRe.MatchString(l):
			l = "│ " + quoteRe.FindStringSubmatch(l)[1]
		}
		l = renderInline(l, dark)
		if bold {
			l = sgrBoldOn + l + sgrBoldOff
		}
		if width > 2 {
			l = lipgloss.Wrap(l, width-2, "")
			// Wrap closes a span at the break with a full reset, which would
			// also drop the band's background for the padding that follows.
			// Only our toggles can be open, so close exactly those.
			l = strings.NewReplacer("\x1b[m", sgrBoldOff+sgrFgOff, "\x1b[0m", sgrBoldOff+sgrFgOff).Replace(l)
		}
		band(resp, strings.Split(l, "\n"))
	}
	return strings.Join(out, "\n")
}
