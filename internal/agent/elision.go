package agent

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
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

// agedCand is one older completed tool result eligible for pruning with a
// spill pointer.
type agedCand struct {
	id, content string
	tokens      int
}

// agedResults returns older completed results beyond the protected suffix:
// the newest min(20K, keepRecent, 40K) tool-output tokens, the last two user
// turns, errors, and the latest result per path stay. The saving estimate
// reserves the spill stub's own text.
func agedResults(live []compact.Entry, keepRecent int) ([]agedCand, int) {
	protect := max(20_000, min(keepRecent, 40_000))
	paths := map[string]string{} // call id -> canonical path
	for _, e := range live {
		if e.Kind != compact.KindToolUse || e.Call == nil {
			continue
		}
		var in struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(e.Call.Input, &in) == nil && in.Path != "" {
			paths[e.Call.ID] = filepath.Clean(in.Path)
		}
	}
	latest := map[string]bool{}
	tokens, userTurns := 0, 0
	var out []agedCand
	saved := 0
	for i := len(live) - 1; i >= 0; i-- {
		e := live[i]
		if e.Kind == compact.KindUser {
			userTurns++
		}
		if e.Kind != compact.KindToolResult || e.Result == nil || e.Result.Content == session.ElidedStub {
			continue
		}
		tokens += compact.EntryTokens(e)
		if userTurns < 2 || tokens <= protect || e.Result.IsError {
			continue
		}
		path := paths[e.Result.CallID]
		if path != "" {
			if !latest[path] {
				latest[path] = true
				continue // the newest result for this path is the working set
			}
		}
		out = append(out, agedCand{id: e.ID, content: e.Result.Content, tokens: compact.EntryTokens(e)})
		saved += max(0, compact.EntryTokens(e)-80) // spill stub headroom
	}
	return out, saved
}

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
			// A later error is not a replacement for a good result; a result
			// whose call left the window (empty key) matches nothing.
			sameCall := r.call.key != "" && later.call.key == r.call.key && !later.e.Result.IsError
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

// shouldElide decides whether stubbing the stale results is worth rewriting
// earlier messages at all. force: right after a compaction, which resets the
// cache anyway — whatever is stale goes. historyBound: a preserved-thinking
// route binds the history to the exact prefix its thinking was produced
// against, so an unforced elision (a rewrite) is never worth it there (§14).
// The caller reads the active model at call time, so a /model switch takes
// effect immediately.
func shouldElide(saved int, force, historyBound bool) bool {
	if force {
		return saved > 0
	}
	if historyBound {
		return false
	}
	return saved >= elideMinTokens
}

// elide persists an elision of the live window's stale results when they
// are worth it (force: whatever is stale — right before or after a
// compaction, which resets the cache anyway). Older completed results are
// pruned only when forced and only with a verified spill pointer.
func (a *Agent) elide(force bool) error {
	live, _ := a.liveEntries()
	ids, saved := staleResults(live)
	stubs := map[string]string{}
	if a.opts.Env != nil {
		cands, agedSaved := agedResults(live, a.Budget().KeepRecent)
		// Mid-run pruning rewrites the prompt cache: only cross the 25K/25%
		// gate when a summary will not reset it anyway. A forced pass (at a
		// compaction boundary) takes everything eligible.
		if force || agedSaved >= max(25_000, a.ContextTokens()/4) {
			for _, c := range cands {
				note, err := tools.Spill(a.opts.Env, "prune", c.content)
				if err != nil || note == "" {
					continue
				}
				stub := "[pruned: " + note + "]"
				stubs[c.id] = stub
				ids = append(ids, c.id)
				saved += max(0, c.tokens-compact.Tokens(len(stub)))
			}
		}
	}
	if len(ids) == 0 || !shouldElide(saved, force, a.model.HistoryBound()) {
		return nil
	}
	el := &session.Elision{IDs: ids, Tokens: saved}
	if len(stubs) > 0 {
		el.Stubs = stubs
	}
	if _, err := a.append(session.Entry{Type: session.TypeElision, Elision: el}); err != nil {
		return err
	}
	a.logger().Info("elided", "results", len(ids), "saved", saved, "spilled", len(stubs))
	a.mu.Lock()
	a.anchorValid = false // the next request differs from what the anchor measured
	a.mu.Unlock()
	return nil
}

// pruneProtected is the last-resort reduction (§5.2 rule 4): when the hard
// ceiling is otherwise unreachable, even the working set and the latest
// results are stubbed, but only with a verified spill pointer. Errors stay.
func (a *Agent) pruneProtected() error {
	if a.opts.Env == nil {
		return nil
	}
	live, _ := a.liveEntries()
	var ids []string
	stubs := map[string]string{}
	saved := 0
	for _, e := range live {
		if e.Kind != compact.KindToolResult || e.Result == nil || e.Result.IsError ||
			e.Result.Content == session.ElidedStub || strings.HasPrefix(e.Result.Content, "[pruned:") {
			continue
		}
		note, err := tools.Spill(a.opts.Env, "prune", e.Result.Content)
		if err != nil || note == "" {
			continue
		}
		stub := "[pruned: " + note + "]"
		ids = append(ids, e.ID)
		stubs[e.ID] = stub
		saved += max(0, compact.EntryTokens(e)-compact.Tokens(len(stub)))
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := a.append(session.Entry{Type: session.TypeElision, Elision: &session.Elision{IDs: ids, Tokens: saved, Stubs: stubs}}); err != nil {
		return err
	}
	a.logger().Info("pruned protected results", "results", len(ids), "saved", saved)
	a.mu.Lock()
	a.anchorValid = false
	a.mu.Unlock()
	return nil
}
