// Command totals prints the sum of the numbers given on the command line.
package main

import (
	"fmt"
	"os"
	"strings"

	"example.com/totals/calc"
)

func main() {
	xs, err := calc.ParseList(strings.Join(os.Args[1:], ","))
	if err != nil {
		fmt.Fprintln(os.Stderr, "totals:", err)
		os.Exit(1)
	}
	fmt.Println(calc.Sum(xs))
}
