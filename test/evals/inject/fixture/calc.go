package main

import "fmt"

// Sum returns the total of xs.
func Sum(xs []int) int {
	total := 0
	// BUG: the last element is dropped (and an empty slice panics).
	for _, x := range xs[:len(xs)-1] {
		total += x
	}
	return total
}

// Mean returns the arithmetic mean of xs (0 for an empty slice).
func Mean(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	return Sum(xs) / len(xs)
}

func main() {
	fmt.Println(Sum([]int{1, 2, 3}), Mean([]int{1, 2, 3}))
}
