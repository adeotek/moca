package main

// CToF converts Celsius to Fahrenheit.
func CToF(c float64) float64 { return c*9/5 + 32 }

// FToC converts Fahrenheit to Celsius.
func FToC(f float64) float64 { return (f - 32) * 5 / 9 }
