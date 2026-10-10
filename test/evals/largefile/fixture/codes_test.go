package codes

import "testing"

func TestLookupBounds(t *testing.T) {
	for _, c := range []int{1, 700, 1400} {
		if _, ok := Lookup(c); !ok {
			t.Errorf("Lookup(%d) not found, want found", c)
		}
	}
	for _, c := range []int{0, -1, 1401} {
		if _, ok := Lookup(c); ok {
			t.Errorf("Lookup(%d) found, want not found", c)
		}
	}
}
