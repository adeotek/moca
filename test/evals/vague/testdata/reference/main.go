// Command temp converts temperatures: temp -to f 100 → 212.0
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

func main() {
	to := flag.String("to", "f", "target unit: f (from Celsius) or c (from Fahrenheit)")
	flag.Parse()
	if flag.NArg() != 1 {
		fail("usage: temp -to f|c <value>")
	}
	v, err := strconv.ParseFloat(flag.Arg(0), 64)
	if err != nil {
		fail(fmt.Sprintf("not a number: %q", flag.Arg(0)))
	}
	switch *to {
	case "f":
		if v < -273.15 {
			fail("below absolute zero (-273.15 °C)")
		}
		fmt.Printf("%.1f\n", CToF(v))
	case "c":
		if v < -459.67 {
			fail("below absolute zero (-459.67 °F)")
		}
		fmt.Printf("%.1f\n", FToC(v))
	default:
		fail(fmt.Sprintf("unknown unit %q (want f or c)", *to))
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "temp:", msg)
	os.Exit(2)
}
