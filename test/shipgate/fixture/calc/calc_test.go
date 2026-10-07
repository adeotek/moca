package calc

import "testing"

func TestTotals(t *testing.T) {
	cases := []struct {
		in   []int
		want int
	}{
		{[]int{1, 2, 3}, 6},
		{[]int{5}, 5},
		{[]int{}, 0},
		{[]int{-1, 4}, 3},
	}
	for _, c := range cases {
		if got := Sum(c.in); got != c.want {
			t.Errorf("Sum(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
