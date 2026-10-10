package main

import (
	"fmt"
	"strings"
)

// Render prints the items as an aligned text table with a total line.
func Render(items []Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-10s %5s %8s\n", "NAME", "QTY", "PRICE")
	total := 0.0
	for _, it := range items {
		fmt.Fprintf(&b, "%-10s %5d %8.2f\n", it.Name, it.Qty, it.Price)
		total += float64(it.Qty) * it.Price
	}
	fmt.Fprintf(&b, "%-10s %14.2f\n", "TOTAL", total)
	return b.String()
}
