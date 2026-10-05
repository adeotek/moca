package compact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

type fakeSum struct {
	calls []string
}

func (f *fakeSum) fn(_ context.Context, system, user string) (string, llm.Usage, error) {
	f.calls = append(f.calls, user)
	return "## Goal\nsummary " + string(rune('A'+len(f.calls)-1)), llm.Usage{Input: 100, Output: 10}, nil
}

func TestCompactSimple(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u1", k), a("a1", k), {ID: "c", Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"x.go"}`)}},
		res("c", "ok"), u("u2", "next"), a("a2", "fine")}
	r, ok, err := Compact(context.Background(), es, Prev{Summary: "old facts", Read: []string{"r.go"}}, 100, 1<<20, f.fn)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !strings.Contains(f.calls[0], "old facts") || !strings.Contains(f.calls[0], "[User]: x") {
		t.Fatalf("calls %q", f.calls)
	}
	if r.FirstKept != "u2" || r.Usage.Input != 100 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.Summary, "<modified-files>\nx.go\n</modified-files>") || !strings.Contains(r.Summary, "<read-files>\nr.go\n</read-files>") {
		t.Fatal(r.Summary)
	}
}

func TestCompactSplitTurnTwoSummaries(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u0", k), a("a0", k), u("u1", "do it"), a("a1", k), call("c1", "ls"), res("c1", k), a("a2", "x")}
	r, ok, _ := Compact(context.Background(), es, Prev{}, 300, 1<<20, f.fn)
	if !ok || len(f.calls) != 2 || !strings.Contains(f.calls[1], "CURRENT turn") || !strings.Contains(f.calls[1], "do it") {
		t.Fatalf("%q", f.calls)
	}
	if !strings.Contains(r.Summary, "summary A") || !strings.Contains(r.Summary, "## Current turn (in progress)\n## Goal\nsummary B") {
		t.Fatal(r.Summary)
	}
	if r.Usage.Input != 200 {
		t.Fatal("usage summed")
	}
}

func TestCompactNothing(t *testing.T) {
	f := &fakeSum{}
	if _, ok, _ := Compact(context.Background(), []Entry{u("u", "hi")}, Prev{}, 100, 1<<20, f.fn); ok || len(f.calls) != 0 {
		t.Fatal("no call when nothing to compact")
	}
	if _, ok, _ := Compact(context.Background(), []Entry{u("u", "hi"), a("a", "yo")}, Prev{}, 100, 1<<20, f.fn); ok || len(f.calls) != 0 {
		t.Fatal("no call while the whole window is recent")
	}
}

func TestCompactCapsInput(t *testing.T) {
	f := &fakeSum{}
	es := []Entry{u("u0", strings.Repeat("q\n", 50000)), a("a0", "x"), u("u1", "y")}
	Compact(context.Background(), es, Prev{}, 1, 1000, f.fn)
	if len(f.calls[0]) > 1100 || !strings.HasPrefix(f.calls[0], "[… earlier history omitted]") {
		t.Fatal(len(f.calls[0]))
	}
}
