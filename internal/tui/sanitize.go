package tui

import "strings"

// Sanitize neutralizes control characters so pasted or tool-produced bytes
// are displayed, never interpreted by the terminal (\x1b → ^[, \x7f → ^?).
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
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func hasControl(s string) bool {
	for _, r := range s {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return true
		}
	}
	return false
}
