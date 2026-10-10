package agent

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/session"
)

// Elision (§6): tool results superseded later in the live window — the same
// call ran again (a re-read, a re-run test, a re-fetched page), or a file
// read earlier was rewritten whole by write (its new content is in the write
// call) — are replaced with a one-line stub in requests. An elision entry
// persists the choice, so resume rebuilds byte-identical requests.

// elideMinTokens is the stale volume that justifies an elision between
// compactions: stubbing rewrites earlier messages and forfeits the prompt
// cache from the first one on, so small savings are not worth it.
// Compaction resets the cache anyway and elides whatever is stale.
const elideMinTokens = 8000

// staleResults returns the ids of live tool results superseded by a later
// call, and the tokens their stubs save.
func staleResults(live []compact.Entry) ([]string, int) {
	type callInfo struct{ key, readPath, writePath string }
	// Results bind to the call that preceded them: ids may repeat across
	// turns (some servers number calls per turn), so a map of all calls
	// would pair an old result with a newer call.
	type result struct {
		e    compact.Entry
		call callInfo
	}
	calls := map[string]callInfo{}
	var order []result
	for _, e := range live {
		switch e.Kind {
		case compact.KindToolUse:
			ci := callInfo{key: callKey(e.Call.Name, e.Call.Input)}
			var in struct {
				Path string `json:"path"`
			}
			if json.Unmarshal(e.Call.Input, &in) == nil && in.Path != "" {
				switch e.Call.Name {
				case "read":
					ci.readPath = filepath.Clean(in.Path)
				case "write":
					ci.writePath = filepath.Clean(in.Path)
				}
			}
			calls[e.Call.ID] = ci
		case compact.KindToolResult:
			order = append(order, result{e, calls[e.Result.CallID]})
		}
	}
	stub := compact.Tokens(len(session.ElidedStub))
	var ids []string
	saved := 0
	for i, r := range order {
		if r.e.Result.Content == session.ElidedStub {
			continue
		}
		for _, later := range order[i+1:] {
			sameCall := later.call.key == r.call.key
			rewritten := r.call.readPath != "" && later.call.writePath == r.call.readPath && !later.e.Result.IsError
			if sameCall || rewritten {
				ids = append(ids, r.e.ID)
				saved += max(0, compact.EntryTokens(r.e)-stub)
				break
			}
		}
	}
	return ids, saved
}

func callKey(name string, input json.RawMessage) string {
	var buf bytes.Buffer
	if json.Compact(&buf, input) != nil {
		return name + "\x00" + string(input)
	}
	return name + "\x00" + buf.String()
}

// elide persists an elision of the live window's stale results when they
// are worth it (force: whatever is stale — right after a compaction).
func (a *Agent) elide(force bool) error {
	live, _ := a.liveEntries()
	ids, saved := staleResults(live)
	if len(ids) == 0 || (!force && saved < elideMinTokens) {
		return nil
	}
	if _, err := a.append(session.Entry{Type: session.TypeElision, Elision: &session.Elision{IDs: ids, Tokens: saved}}); err != nil {
		return err
	}
	a.logger().Info("elided", "results", len(ids), "saved", saved)
	a.mu.Lock()
	a.anchorValid = false // the next request differs from what the anchor measured
	a.mu.Unlock()
	return nil
}
