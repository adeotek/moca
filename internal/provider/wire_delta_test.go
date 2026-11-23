package provider

// The ninth tool must be a pure append on the wire: the eight existing
// serialized entries stay byte-identical, and with the default cache the single
// Anthropic breakpoint moves from entry eight to entry nine. Anything else is a
// prompt-cache reset for every user, which is why this is a test and not a
// review note.

import (
	"encoding/json"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// builtins returns the eight shipped specs, in registry order.
func builtins(t *testing.T) []llm.ToolSpec {
	t.Helper()
	var specs []llm.ToolSpec
	for _, tool := range tools.Builtins() {
		specs = append(specs, tool.Spec())
	}
	if len(specs) != 8 {
		t.Fatalf("Builtins() has %d specs, want 8", len(specs))
	}
	return specs
}

func wireTools(t *testing.T, req llm.Request) []string {
	t.Helper()
	a := &anthropicAdapter{}
	raw, ok := a.body(req)["tools"]
	if !ok {
		t.Fatal("no tools in the body")
	}
	list, ok := raw.([]map[string]any)
	if !ok {
		t.Fatalf("tools is %T", raw)
	}
	out := make([]string, len(list))
	for i, m := range list {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("tool %d: %v", i, err)
		}
		out[i] = string(b)
	}
	return out
}

func TestAnthropicWireDeltaNoCache(t *testing.T) {
	for _, variant := range []struct {
		name string
		net  bool
	}{{"workspace", false}, {"network", true}} {
		t.Run(variant.name, func(t *testing.T) {
			testAnthropicWireDeltaNoCache(t, variant.net)
		})
	}
}

func testAnthropicWireDeltaNoCache(t *testing.T, network bool) {
	base := builtins(t)
	nine := append(append([]llm.ToolSpec(nil), base...), tools.NewCodemodeRefusal(network).Spec())

	with8 := wireTools(t, llm.Request{Model: "m", MaxTokens: 1, Tools: base, NoCacheWrite: true})
	with9 := wireTools(t, llm.Request{Model: "m", MaxTokens: 1, Tools: nine, NoCacheWrite: true})

	if len(with8) != 8 || len(with9) != 9 {
		t.Fatalf("tool counts %d and %d", len(with8), len(with9))
	}
	for i := 0; i < 8; i++ {
		if with8[i] != with9[i] {
			t.Fatalf("entry %d changed:\n 8: %s\n 9: %s", i+1, with8[i], with9[i])
		}
	}
	delta := len(with9[8])
	if delta <= 0 {
		t.Fatal("the ninth entry is empty")
	}
	// The whole ninth entry, not just the schema: the description counts too.
	if delta >= 1024 {
		t.Fatalf("the ninth entry is %d bytes, budget is <1 KiB", delta)
	}
}

func TestAnthropicWireDeltaMovesTheBreakpoint(t *testing.T) {
	for _, variant := range []struct {
		name string
		net  bool
	}{{"workspace", false}, {"network", true}} {
		t.Run(variant.name, func(t *testing.T) {
			testAnthropicWireDeltaMovesTheBreakpoint(t, variant.net)
		})
	}
}

func testAnthropicWireDeltaMovesTheBreakpoint(t *testing.T, network bool) {
	base := builtins(t)
	nine := append(append([]llm.ToolSpec(nil), base...), tools.NewCodemodeRefusal(network).Spec())

	with8 := wireTools(t, llm.Request{Model: "m", MaxTokens: 1, Tools: base})
	with9 := wireTools(t, llm.Request{Model: "m", MaxTokens: 1, Tools: nine})

	has := func(entry string) bool {
		var m map[string]any
		if err := json.Unmarshal([]byte(entry), &m); err != nil {
			t.Fatalf("entry: %v", err)
		}
		return m["cache_control"] != nil
	}
	if !has(with8[7]) {
		t.Fatal("entry eight has no cache breakpoint")
	}
	if has(with8[0]) {
		t.Fatal("the breakpoint is not on the last entry")
	}
	// With nine, the breakpoint moves to entry nine and leaves entry eight.
	if !has(with9[8]) {
		t.Fatal("entry nine has no cache breakpoint")
	}
	if has(with9[7]) {
		t.Fatal("entry eight kept its breakpoint after the ninth was appended")
	}
	// Normalizing only cache_control, the first eight are unchanged.
	for i := 0; i < 8; i++ {
		a, b := with8[i], with9[i]
		if i == 7 {
			a, b = stripCache(a), stripCache(b)
		}
		if a != b {
			t.Fatalf("entry %d changed beyond the breakpoint:\n a: %s\n b: %s", i+1, a, b)
		}
	}
}

func stripCache(entry string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(entry), &m); err != nil {
		return entry
	}
	delete(m, "cache_control")
	out, err := json.Marshal(m)
	if err != nil {
		return entry
	}
	return string(out)
}
