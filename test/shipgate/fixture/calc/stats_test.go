package calc

import "testing"

func TestMaxAndAverage(t *testing.T) {
	if got := Max([]int{3, 9, 2}); got != 9 {
		t.Errorf("Max = %d, want 9", got)
	}
	if got := Average([]int{2, 4}); got != 3 {
		t.Errorf("Average = %v, want 3", got)
	}
}
