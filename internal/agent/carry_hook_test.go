//go:build codemodecarry

package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// carryTokens is the hook's whole measurement: tool-result content only,
// whatever tool produced it. A text or thinking block is not carried output;
// a summary arrives as a user text message and is therefore never counted.
func TestCarryTokensCountsToolResultsOnly(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "a", Content: repeat("x", 400)}}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{
			{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "b", Content: repeat("y", 100)}},
			{Type: llm.BlockText, Text: repeat("z", 40000)},
		}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: repeat("t", 4000)},
			{Type: llm.BlockText, Text: "done"},
		}},
	}
	if got := carryTokens(msgs); got != 125 {
		t.Fatalf("carry = %d, want 125 (100 + 25)", got)
	}
	if got := carryTokens(nil); got != 0 {
		t.Fatalf("an empty request carries 0, got %d", got)
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// carryObserve appends one line per request, numbered from 1, to
// $MOCA_CARRY_OUT — the file the trial harness reads and the "missing
// observation" failure is defined against. With the variable unset it writes
// nothing at all (a normal run of a tagged binary must stay quiet).
func TestCarryObserveWritesOneLinePerRequest(t *testing.T) {
	w, err := session.Create(t.TempDir(), session.Header{Workdir: "/tmp", StartedAt: time.Now()}, "carry")
	if err != nil {
		t.Fatal(err)
	}
	a := &Agent{opts: Options{Session: w}}
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c", Content: repeat("x", 400)}}}},
	}}

	if err := os.Unsetenv("MOCA_CARRY_OUT"); err != nil {
		t.Fatal(err)
	}
	a.carryObserve(req) // nothing to assert but the absence of a panic

	path := filepath.Join(t.TempDir(), "carry.jsonl")
	t.Setenv("MOCA_CARRY_OUT", path)
	t.Setenv("MOCA_TRIAL", "3")
	carryReqNo.Store(0) // the counter is process-wide by design; pin a fresh process
	a.carryObserve(req)
	a.carryObserve(req)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no observation file: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	obs, err := readObs(t, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 {
		t.Fatalf("observations = %d, want 2 (one per request)", len(obs))
	}
	if obs[0].Turn != 1 || obs[1].Turn != 2 {
		t.Fatalf("turns = %d,%d, want 1,2", obs[0].Turn, obs[1].Turn)
	}
	if obs[0].Carry != 100 || obs[0].Session != w.ID8() || obs[0].Trial != 3 {
		t.Fatalf("observation = %+v, want carry 100, session %s, trial 3", obs[0], w.ID8())
	}
}

// A torn or unparseable environment value is not an error: the instrument
// must never fail the run it measures.
func TestEnvIntIsForgiving(t *testing.T) {
	t.Setenv("MOCA_TRIAL", "")
	if envInt("MOCA_TRIAL") != 0 {
		t.Fatal("an empty value must read 0")
	}
	t.Setenv("MOCA_TRIAL", "7")
	if envInt("MOCA_TRIAL") != 7 {
		t.Fatal("a plain integer must read back")
	}
	t.Setenv("MOCA_TRIAL", "seven")
	if envInt("MOCA_TRIAL") != 0 {
		t.Fatal("garbage must read 0, not panic")
	}
}

type obsLine struct {
	Session string `json:"session"`
	Trial   int    `json:"trial"`
	Turn    int    `json:"turn"`
	Carry   int    `json:"carry"`
}

func readObs(t *testing.T, b []byte) ([]obsLine, error) {
	t.Helper()
	var out []obsLine
	for _, line := range splitLines(string(b)) {
		var o obsLine
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
