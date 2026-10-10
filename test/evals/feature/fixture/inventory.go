package main

// Item is one stock line.
type Item struct {
	Name  string
	Qty   int
	Price float64 // unit price in EUR
}

// Items is the current stock (a real tool would load it from storage).
var Items = []Item{
	{"widget", 12, 2.5},
	{"gadget", 3, 19.99},
	{"doohickey", 40, 0.75},
}
