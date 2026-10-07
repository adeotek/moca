package calc

// Max returns the largest element of xs (0 for an empty slice).
func Max(xs []int) int {
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

// Average returns the mean of xs (0 for an empty slice).
func Average(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	return float64(Sum(xs)) / float64(len(xs))
}
