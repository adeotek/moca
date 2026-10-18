package main

import "fmt"

// Sum returns the total of xs.
func Sum(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

func main() { fmt.Println(Sum([]int{1, 2, 3})) }
