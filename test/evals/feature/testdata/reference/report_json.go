package main

import "encoding/json"

type jsonItem struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Price float64 `json:"price"`
}

// RenderJSON returns the items as a JSON array.
func RenderJSON(items []Item) (string, error) {
	out := make([]jsonItem, len(items))
	for i, it := range items {
		out[i] = jsonItem{it.Name, it.Qty, it.Price}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	return string(b) + "\n", err
}
