package stats

import (
	"math"
	"testing"
)

func close(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The behaviour the refactor must not change.
func TestSummary(t *testing.T) {
	for _, c := range []struct {
		in            []float64
		mean, med, sd float64
	}{
		{[]float64{}, 0, 0, 0},
		{[]float64{5}, 5, 5, 0},
		{[]float64{1, 2, 3, 4}, 2.5, 2.5, 1.118033988749895},
		{[]float64{4, 1, 3, 2, 10}, 4, 3, 3.16227766016838},
	} {
		mean, med, sd := Summary(c.in)
		if !close(mean, c.mean) || !close(med, c.med) || !close(sd, c.sd) {
			t.Fatalf("Summary(%v) = %v, %v, %v; want %v, %v, %v", c.in, mean, med, sd, c.mean, c.med, c.sd)
		}
	}
}

// Summary must not reorder the caller's slice.
func TestSummaryLeavesInputAlone(t *testing.T) {
	in := []float64{4, 1, 3, 2}
	Summary(in)
	if in[0] != 4 || in[3] != 2 {
		t.Fatalf("Summary mutated its input: %v", in)
	}
}
