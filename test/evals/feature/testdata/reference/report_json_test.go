package main

import (
	"encoding/json"
	"testing"
)

func TestRenderJSON(t *testing.T) {
	s, err := RenderJSON(Items)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(s), &got); err != nil || len(got) != 3 || got[0]["name"] != "widget" {
		t.Fatalf("%v %v", got, err)
	}
}
