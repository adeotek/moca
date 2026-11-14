package human

import "testing"

// The fixture's own contract, so the three sizes the checker looks for are
// this package's promise rather than the agent's reconstruction of them.
func TestBytes(t *testing.T) {
	for _, c := range []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{1536, "1.5 KiB"},
		{2097152, "2.0 MiB"},
	} {
		if got := Bytes(c.in); got != c.want {
			t.Fatalf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
