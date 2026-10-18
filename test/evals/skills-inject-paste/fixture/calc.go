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

func main() { fmt.Println(Sum([]int{1, 2, 3})) }
