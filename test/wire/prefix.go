// Package wire provides protocol-neutral checks over decoded request bodies.
// Test-only helper: it imports nothing internal, so any package's tests can
// use it without an import cycle.
package wire

import "reflect"

// Strip deep-copies a decoded JSON value and drops every "cache_control" key.
// Moving a cache breakpoint is not an invalidator: the cached bytes are the
// same, so a prefix check must ignore where the breakpoints sit.
func Strip(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			if k == "cache_control" {
				continue
			}
			out[k] = Strip(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = Strip(vv)
		}
		return out
	default:
		return v // scalars are immutable
	}
}

// FirstDivergence reports where next stops being an append-only continuation
// of prev: the first message index and block index that differ. (-1, -1) means
// prev is an append-only prefix of next.
//
// Append-only means: every message of prev except the last is deep-equal in
// next, and the last message of prev has the same role with its content blocks
// a prefix of the same-index message in next. A user message may grow, because
// steering or a nudge merges after its tool_result.
//
// Lists that are not made of role/content messages (the openai-responses
// `input` array interleaves messages, reasoning items and calls) are compared
// item-wise: an item without role/content is one opaque block.
func FirstDivergence(prev, next []any) (int, int) {
	for i := range prev {
		if i == len(prev)-1 {
			if i >= len(next) {
				return i, 0
			}
			if roleOf(prev[i]) != roleOf(next[i]) {
				return i, 0
			}
			pb, nb := blocksOf(prev[i]), blocksOf(next[i])
			n := min(len(pb), len(nb))
			for j := 0; j < n; j++ {
				if !reflect.DeepEqual(pb[j], nb[j]) {
					return i, j
				}
			}
			if len(nb) < len(pb) {
				return i, len(nb) // shrank: not an append
			}
			continue // equal or grew
		}
		if i >= len(next) {
			return i, 0
		}
		if reflect.DeepEqual(prev[i], next[i]) {
			continue
		}
		pb, nb := blocksOf(prev[i]), blocksOf(next[i])
		n := min(len(pb), len(nb))
		for j := 0; j < n; j++ {
			if !reflect.DeepEqual(pb[j], nb[j]) {
				return i, j
			}
		}
		return i, n
	}
	return -1, -1
}

func roleOf(v any) string {
	if m, ok := v.(map[string]any); ok {
		if r, ok := m["role"].(string); ok {
			return r
		}
	}
	return ""
}

// blocksOf returns the comparable blocks of one message: a string content is
// one block, an array content is its elements, and an item without content
// (a responses reasoning item or function call) is one opaque block.
func blocksOf(v any) []any {
	m, ok := v.(map[string]any)
	if !ok {
		return []any{v}
	}
	switch c := m["content"].(type) {
	case []any:
		return c
	case string:
		return []any{c}
	case nil:
		if _, has := m["role"]; has {
			return []any{}
		}
	}
	return []any{v}
}
