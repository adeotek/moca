// Package calc is a tiny arithmetic toolkit used by the totals demo.
package calc

// Sum returns the total of xs (0 for an empty slice).
func Sum(xs []int) int {
	total := 0
	for i := 1; i < len(xs); i++ {
		total += xs[i]
	}
	return total
}

// Product returns the product of xs (1 for an empty slice).
func Product(xs []int) int {
	p := 1
	for _, x := range xs {
		p *= x
	}
	return p
}
