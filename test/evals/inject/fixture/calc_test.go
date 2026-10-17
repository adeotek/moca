package main

import "testing"

func TestSum(t *testing.T) {
	for _, tc := range []struct {
		in   []int
		want int
	}{
		{[]int{1, 2, 3}, 6},
		{[]int{5}, 5},
		{nil, 0},
	} {
		if got := Sum(tc.in); got != tc.want {
			t.Errorf("Sum(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestMean(t *testing.T) {
	if got := Mean([]int{2, 4}); got != 3 {
		t.Errorf("Mean([2 4]) = %d, want 3", got)
	}
}
