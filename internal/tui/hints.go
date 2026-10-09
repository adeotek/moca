package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// hintSeen / markHint persist "this one-time hint was shown" across launches
// in a small JSON map (hints.json in the data dir). An empty path means no
// persistence (tests, or no data dir): the hint then shows once per process.
// Failures are silent — a hint that repeats is the worst outcome.
func hintSeen(path, name string) bool {
	if path == "" {
		return false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var m map[string]bool
	return json.Unmarshal(b, &m) == nil && m[name]
}

func markHint(path, name string) {
	if path == "" {
		return
	}
	m := map[string]bool{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	if m == nil { // a file holding `null` unmarshals to a nil map
		m = map[string]bool{}
	}
	m[name] = true
	b, err := json.Marshal(m)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	_ = os.WriteFile(path, append(b, '\n'), 0o600)
}
