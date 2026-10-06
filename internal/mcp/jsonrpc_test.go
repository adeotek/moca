package mcp

import (
	"encoding/json"
	"testing"
)

// responseID: int64 and quoted-int64 ids parse; null, fractions and garbage
// do not (helpers/echoed-id handling, pass-2 L6).
func TestResponseID(t *testing.T) {
	ok := map[string]int64{"12": 12, `"12"`: 12, `"-3"`: -3, `12.0`: 12}
	for raw, want := range ok {
		r := json.RawMessage(raw)
		if got, good := responseID(&r); !good || got != want {
			t.Errorf("%s → %d, %v (want %d)", raw, got, good, want)
		}
	}
	for _, raw := range []string{`"abc"`, `1.5`, `null`, `[1]`} {
		r := json.RawMessage(raw)
		if _, good := responseID(&r); good {
			t.Errorf("%s must not parse", raw)
		}
	}
	if _, good := responseID(nil); good {
		t.Error("nil id must not parse")
	}
}
