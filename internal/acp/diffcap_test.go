package acp

// Pin for F-7: an edit or write update carries the file's text to the editor as
// a diff. Uncapped, a 1 MB file became a 1 MB JSON-RPC frame per tool call — the
// editor parses it, and the session's update queue holds it.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func TestEditUpdateCapsBothDiffSides(t *testing.T) {
	big := strings.Repeat("x", 100<<10)
	call := llm.ToolCall{ID: "1", Name: "write", Input: []byte(`{"path":"big.txt","content":"` + big + `"}`)}
	u := endUpdate("1", "/w", call, tools.Result{})

	if len(u.Content) != 1 || u.Content[0].NewText == nil {
		t.Fatalf("want one diff update, got %+v", u.Content)
	}
	if n := len(*u.Content[0].NewText); n > (8<<10)+64 {
		t.Fatalf("NewText is %d bytes; the update must be bounded", n)
	}
	if !strings.Contains(*u.Content[0].NewText, "more bytes not shown") {
		t.Fatal("a truncated diff must say so")
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 32<<10 {
		t.Fatalf("marshalled update is %d bytes", len(b))
	}

	// Control: a small write is untouched.
	small := llm.ToolCall{ID: "2", Name: "write", Input: []byte(`{"path":"s.txt","content":"hello"}`)}
	su := endUpdate("2", "/w", small, tools.Result{})
	if su.Content[0].NewText == nil || *su.Content[0].NewText != "hello" {
		t.Fatalf("small content must pass through: %+v", su.Content)
	}
}
