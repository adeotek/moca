package stats

import "math"

// Summary returns the mean, the median and the population standard deviation.
func Summary(xs []float64) (mean, med, sd float64) {
	n := float64(len(xs))
	if n == 0 {
		return 0, 0, 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	mean = s / n
	c := append([]float64(nil), xs...)
	for i := 0; i < len(c); i++ {
		for j := i + 1; j < len(c); j++ {
			if c[j] < c[i] {
				c[i], c[j] = c[j], c[i]
			}
		}
	}
	if len(c)%2 == 1 {
		med = c[len(c)/2]
	} else {
		med = (c[len(c)/2-1] + c[len(c)/2]) / 2
	}
	var q float64
	for _, x := range xs {
		d := x - mean
		q += d * d
	}
	sd = math.Sqrt(q / n)
	return
}
