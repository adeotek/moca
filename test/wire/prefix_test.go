package wire

import (
	"encoding/json"
	"reflect"
	"testing"
)

func msgs(t *testing.T, s string) []any {
	t.Helper()
	var out []any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("bad message list %q: %v", s, err)
	}
	return out
}

func TestFirstDivergence(t *testing.T) {
	const (
		a  = `{"role":"user","content":[{"type":"text","text":"a"}]}`
		b  = `{"role":"user","content":[{"type":"text","text":"b"}]}`
		tr = `{"role":"user","content":[{"type":"tool_result","content":"x"}]}`
	)
	cases := []struct {
		name       string
		prev, next string
		wantMsg    int
		wantBlock  int
	}{
		{"identical", `[` + a + `]`, `[` + a + `]`, -1, -1},
		{"appended message", `[` + a + `]`, `[` + a + `,` + tr + `]`, -1, -1},
		{"last user msg grows", `[` + tr + `]`, `[{"role":"user","content":[{"type":"tool_result","content":"x"},{"type":"text","text":"nudge"}]}]`, -1, -1},
		{"earlier msg edited", `[` + a + `,` + b + `]`, `[` + b + `,` + b + `]`, 0, 0},
		{"last msg shrinks", `[{"role":"user","content":[{"type":"tool_result","content":"x"},{"type":"text","text":"nudge"}]}]`, `[` + tr + `]`, 0, 1},
		{"role changed", `[` + a + `]`, `[{"role":"assistant","content":[{"type":"text","text":"a"}]}]`, 0, 0},
		{"message removed", `[` + a + `,` + b + `]`, `[` + a + `]`, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, blk := FirstDivergence(msgs(t, tc.prev), msgs(t, tc.next))
			if m != tc.wantMsg || blk != tc.wantBlock {
				t.Fatalf("got (%d,%d), want (%d,%d)", m, blk, tc.wantMsg, tc.wantBlock)
			}
		})
	}
}

func TestStripIgnoresCacheBreakpoints(t *testing.T) {
	prev := msgs(t, `[{"role":"user","content":[{"type":"text","text":"a","cache_control":{"type":"ephemeral"}}]}]`)
	next := msgs(t, `[{"role":"user","content":[{"type":"text","text":"a"}]}]`)
	if m, _ := FirstDivergence(prev, next); m == -1 {
		t.Fatalf("unstripped bodies must diverge (sanity)")
	}
	if m, blk := FirstDivergence(Strip(prev).([]any), Strip(next).([]any)); m != -1 {
		t.Fatalf("stripped bodies must match: (%d,%d)", m, blk)
	}
}

func TestStripDeepCopiesAndDrops(t *testing.T) {
	in := map[string]any{
		"system": []any{
			map[string]any{"type": "text", "text": "s", "cache_control": map[string]any{"type": "ephemeral"}},
		},
		"tools": []any{map[string]any{"name": "x", "cache_control": map[string]any{"type": "ephemeral"}}},
	}
	out := Strip(in).(map[string]any)
	sys := out["system"].([]any)[0].(map[string]any)
	if _, ok := sys["cache_control"]; ok {
		t.Fatal("cache_control survived at depth")
	}
	tool := out["tools"].([]any)[0].(map[string]any)
	if _, ok := tool["cache_control"]; ok {
		t.Fatal("cache_control survived in tools")
	}
	if _, ok := in["system"].([]any)[0].(map[string]any)["cache_control"]; !ok {
		t.Fatal("Strip mutated its input")
	}
	if !reflect.DeepEqual(Strip(in), Strip(out)) {
		t.Fatal("Strip is not idempotent")
	}
}
