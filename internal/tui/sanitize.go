package tui

import "strings"

// Sanitize neutralizes control characters so pasted or tool-produced bytes
// are displayed, never interpreted by the terminal (\x1b → ^[, \x7f → ^?).
// C1 controls (U+0080–U+009F) are shown as their 7-bit ESC form (U+009B CSI →
// ^[[): xterm-class terminals act on them even UTF-8 encoded.
func Sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			sb.WriteRune(r)
		case r < 0x20:
			sb.WriteByte('^')
			sb.WriteRune(r + '@')
		case r == 0x7f:
			sb.WriteString("^?")
		case r >= 0x80 && r <= 0x9f:
			sb.WriteString("^[")
			sb.WriteRune(r - 0x40)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func hasControl(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return false
}
