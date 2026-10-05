package tui

import "testing"

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\x1b[31mred\x1b[201~\tb\nc\x7f"); got != "a^[[31mred^[[201~\tb\nc^?" {
		t.Fatalf("%q", got)
	}
}
