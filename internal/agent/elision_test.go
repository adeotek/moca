package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
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
