package stats

import (
	"math"
	"slices"
)

// Summary returns the mean, the median and the population standard deviation
// of xs. An empty slice returns three zeroes.
func Summary(xs []float64) (mean, med, sd float64) {
	if len(xs) == 0 {
		return 0, 0, 0
	}
	mean = average(xs)
	med = medianOf(xs)
	return mean, med, stdDev(xs, mean)
}

func average(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// median sorts a copy: Summary may not reorder the caller's slice.
func medianOf(xs []float64) float64 {
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// stdDev is the population standard deviation around a known mean.
func stdDev(xs []float64, mean float64) float64 {
	var sumSquares float64
	for _, x := range xs {
		d := x - mean
		sumSquares += d * d
	}
	return math.Sqrt(sumSquares / float64(len(xs)))
}
