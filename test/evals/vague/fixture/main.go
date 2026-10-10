// Command temp converts temperatures: temp -to f 100 → 212.0
package main

import (
	"flag"
	"fmt"
	"strconv"
)

func main() {
	to := flag.String("to", "f", "target unit: f (from Celsius) or c (from Fahrenheit)")
	flag.Parse()
	v, _ := strconv.ParseFloat(flag.Arg(0), 64)
	if *to == "c" {
		fmt.Printf("%.1f\n", FToC(v))
		return
	}
	fmt.Printf("%.1f\n", CToF(v))
}
