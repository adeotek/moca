package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// moca asks the terminal for *all* keys as escape codes — that is what makes
// shift+enter distinguishable — and for alternate keys on top of it: with the
// first flag set the terminal reports the *unshifted* codepoint plus a shift
// modifier, so without the second one a shifted symbol loses its shift
// (shift+/ arrives as "/").
func TestViewRequestsAlternateKeys(t *testing.T) {
	m := newTestModel()
	ke := m.View().KeyboardEnhancements
	if !ke.ReportAllKeysAsEscapeCodes || !ke.ReportAlternateKeys {
		t.Fatalf("keyboard enhancements = %+v, want all-keys + alternate keys", ke)
	}
}

// decodeKey is one raw byte sequence through the decoder Bubble Tea itself
// uses, so the test covers the wire form rather than a hand-built message.
func decodeKey(t *testing.T, seq string) tea.KeyPressMsg {
	t.Helper()
	var d uv.EventDecoder
	n, ev := d.Decode([]byte(seq))
	if n != len(seq) {
		t.Fatalf("%q: decoded %d of %d bytes (%T)", seq, n, len(seq), ev)
	}
	kp, ok := ev.(uv.KeyPressEvent)
	if !ok {
		t.Fatalf("%q: %T, want a key press", seq, ev)
	}
	return tea.KeyPressMsg(kp)
}

// A shifted symbol reaches the draft with its shift applied, for both wire
// forms a terminal may send. The alternate-key form (`CSI 47:63;2u`) is what a
// terminal answering rev 28's request sends; the plain form (`CSI 47;2u`) is
// the degradation the request exists to avoid, kept here so the difference is
// pinned rather than rediscovered.
func TestShiftedSymbolReachesTheDraft(t *testing.T) {
	for _, tc := range []struct {
		name, seq, want string
	}{
		{"alternate keys", "\x1b[47:63;2u", "?"},
		{"plain csi-u (no alternate keys)", "\x1b[47;2u", "/"},
		{"shift+a", "\x1b[97:65;2u", "A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel()
			k := decodeKey(t, tc.seq)
			m.Update(k)
			got := m.input.Buffer()
			if got != tc.want {
				t.Fatalf("%q → buffer %q, want %q (key %+v)", tc.seq, got, tc.want, k)
			}
			if strings.Contains(got, "\x1b") {
				t.Fatalf("escape bytes leaked into the draft: %q", got)
			}
		})
	}
}
