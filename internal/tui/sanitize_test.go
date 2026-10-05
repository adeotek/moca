package tui

import "testing"

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\x1b[31mred\x1b[201~\tb\nc\x7f"); got != "a^[[31mred^[[201~\tb\nc^?" {
		t.Fatalf("%q", got)
	}
}

// C1 controls (U+0080–U+009F) are interpreted by some xterm-class terminals
// even when UTF-8 encoded: CSI = U+009B, OSC = U+009D.
func TestSanitizeC1(t *testing.T) {
	if got := Sanitize("a\u009b2Jb\u009d52;c;x\u009cz"); got != "a^[[2Jb^[]52;c;x^[\\z" {
		t.Fatalf("%q", got)
	}
	if !hasControl("x\u009by") {
		t.Fatal("a C1 byte in a paste makes it an unsafe chip")
	}
}
