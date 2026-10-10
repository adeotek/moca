package main

import "testing"

func TestRoundTrip(t *testing.T) {
	if CToF(100) != 212 || FToC(212) != 100 {
		t.Fatal("conversion broken")
	}
}
