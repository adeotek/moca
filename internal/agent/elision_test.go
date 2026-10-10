package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

func TestStaleResults(t *testing.T) {
	use := func(id, name, in string) compact.Entry {
		return compact.Entry{ID: "u" + id, Kind: compact.KindToolUse, Call: &llm.ToolCall{ID: id, Name: name, Input: json.RawMessage(in)}}
	}
	res := func(id, content string, isErr bool) compact.Entry {
		return compact.Entry{ID: "r" + id, Kind: compact.KindToolResult, Result: &llm.ToolResult{CallID: id, Content: content, IsError: isErr}}
	}
	big := strings.Repeat("x", 4000)
	live := []compact.Entry{
		use("1", "read", `{"path":"a.go"}`), res("1", big, false), // superseded by 3 (same call)
		use("2", "read", `{"path":"b.go"}`), res("2", big, false), // superseded by the write of ./b.go
		use("3", "read", `{"path": "a.go"}`), res("3", big, false), // newest: kept
		use("4", "write", `{"path":"./b.go","content":"x"}`), res("4", "ok", false),
		use("5", "read", `{"path":"c.go"}`), res("5", big, false), // a failed write does not supersede
		use("6", "write", `{"path":"c.go","content":"x"}`), res("6", "refused", true),
		use("7", "shell", `{"command":"go test ./..."}`), res("7", big, true),
		use("8", "shell", `{"command":"go test ./..."}`), res("8", "ok", false),
		use("9", "read", `{"path":"d.go"}`), res("9", big, false), // a later errored repeat is not a replacement
		use("10", "read", `{"path":"d.go"}`), res("10", "boom", true),
		res("97", big, false), // orphan results: their call left the window — they match nothing
		res("98", big, false),
	}
	ids, saved := staleResults(live)
	if strings.Join(ids, ",") != "r1,r2,r7" || saved < 2900 {
		t.Fatalf("ids=%v saved=%d", ids, saved)
	}
}

// TestElisionStubsRepeatedReadsInRequests: once the stale volume crosses
// the threshold, the next request carries the stub for the superseded result
// and the newest result in full; an elision entry records it.
func TestElisionStubsRepeatedReadsInRequests(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"big.txt","limit":2000}`}),
		toolTurn([2]string{"read", `{"path":"big.txt","limit":2000}`}),
		textTurn("done"),
	)
	a, work, _ := startTest(t, s, 40)
	writeFile(t, work, "big.txt", strings.Repeat(strings.Repeat("y", 99)+"\n", 400)) // ≈ 10k tokens per read
	if _, err := a.Run(context.Background(), "read twice"); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(s.bodies[2]["messages"])
	body := string(b)
	if strings.Count(body, "superseded: this call ran again") != 1 || strings.Count(body, strings.Repeat("y", 99)) != 400 {
		t.Fatalf("third request: stubs=%d", strings.Count(body, "superseded"))
	}
	n := 0
	for _, e := range a.entries {
		if e.Type == session.TypeElision {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("elision entries: %d", n)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestOverflowIsPerSession: each session spills into its own directory and
// its jail can read only that one — another session's (or project's) saved
// tool output in the shared overflow area, which may hold secrets, stays out
// of reach.
func TestOverflowIsPerSession(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)
	own := filepath.Join(a.opts.Env.SpillDir, "shell-1.txt")
	os.MkdirAll(a.opts.Env.SpillDir, 0o700)
	writeFile(t, a.opts.Env.SpillDir, "shell-1.txt", "mine\n")
	if _, err := a.opts.Env.Paths.Resolve(own, false); err != nil {
		t.Fatalf("own spill file must be readable: %v", err)
	}
	area := filepath.Join(config.DataDir(), "overflow")
	for _, other := range []string{filepath.Join(area, "deadbeef-shell-1.txt"), filepath.Join(area, "0123456789abcdef", "shell-1.txt")} {
		os.MkdirAll(filepath.Dir(other), 0o700)
		os.WriteFile(other, []byte("TOKEN=secret\n"), 0o600)
		if _, err := a.opts.Env.Paths.Resolve(other, false); err == nil {
			t.Fatalf("another session's spill file %s must be outside the jail", other)
		}
	}
	// The path a real tools.Spill note names resolves in the same jail.
	note := tools.Spill(a.opts.Env, "shell", "full text\n")
	i, j := strings.Index(note, "saved to "), strings.LastIndex(note, " — read it")
	if i < 0 || j < i {
		t.Fatalf("spill note shape: %q", note)
	}
	p := note[i+len("saved to ") : j]
	if !strings.HasPrefix(p, a.opts.Env.SpillDir) {
		t.Fatalf("spill path %s not under %s", p, a.opts.Env.SpillDir)
	}
	if _, err := a.opts.Env.Paths.Resolve(p, false); err != nil {
		t.Fatalf("spilled file must resolve in the jail: %v", err)
	}
}
