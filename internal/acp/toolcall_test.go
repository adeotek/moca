package acp

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

func call(name, input string) llm.ToolCall {
	return llm.ToolCall{ID: "call_0", Name: name, Input: json.RawMessage(input)}
}

func TestStartUpdateTable(t *testing.T) {
	cases := []struct {
		name, input, title, kind, loc string
	}{
		{"read", `{"path":"a.go"}`, "read a.go", "read", "/w/a.go"},
		{"write", `{"path":"/abs/b.go","content":"x"}`, "write /abs/b.go", "edit", "/abs/b.go"},
		{"edit", `{"path":"a.go","old_string":"a","new_string":"b"}`, "edit a.go", "edit", "/w/a.go"},
		{"shell", `{"command":"go test ./...\necho hi"}`, "go test ./...", "execute", ""},
		{"search", `{"pattern":"TODO","path":"internal"}`, "search TODO", "search", "/w/internal"},
		{"ls", `{}`, "ls .", "search", ""},
		{"web", `{"op":"search","query":"acp spec"}`, "search acp spec", "fetch", ""},
		{"web", `{"op":"fetch","url":"https://x.dev"}`, "fetch https://x.dev", "fetch", ""},
		{"mcp", `{"action":"call","server":"gh","tool":"issues"}`, "mcp gh/issues", "other", ""},
		{"mcp", `{"action":"search","query":"pr"}`, "mcp search pr", "other", ""},
		{"read", `{"path":`, "read", "read", ""}, // cut-off input: name only, no rawInput
	}
	for _, c := range cases {
		u := startUpdate("1-call_0", "/w", call(c.name, c.input))
		if u.SessionUpdate != updToolCall || u.Status != statusInProgress || u.Title != c.title || u.Kind != c.kind {
			t.Errorf("%s %s: got %+v", c.name, c.input, u)
		}
		if c.loc == "" && len(u.Locations) != 0 || c.loc != "" && (len(u.Locations) != 1 || u.Locations[0].Path != c.loc) {
			t.Errorf("%s: locations %+v, want %q", c.name, u.Locations, c.loc)
		}
		if json.Valid([]byte(c.input)) != (u.RawInput != nil) {
			t.Errorf("%s: rawInput %s", c.name, u.RawInput)
		}
		conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: u})
	}
}

func TestShellTitleCappedAt80(t *testing.T) {
	u := startUpdate("1", "/w", call("shell", `{"command":"`+strings.Repeat("é", 100)+`"}`))
	if utf8.RuneCountInString(u.Title) != 80 || !utf8.ValidString(u.Title) {
		t.Fatalf("title %q", u.Title)
	}
}

func TestEndUpdateContent(t *testing.T) {
	u := endUpdate("1", "/w", call("edit", `{"path":"a.go","old_string":"bug","new_string":"fix"}`), tools.Result{Summary: "a.go [+1 −1]"})
	if u.Status != statusCompleted || len(u.Content) != 1 || u.Content[0].Type != "diff" ||
		u.Content[0].Path != "/w/a.go" || *u.Content[0].OldText != "bug" || *u.Content[0].NewText != "fix" {
		t.Errorf("edit: %+v", u)
	}
	u = endUpdate("1", "/w", call("write", `{"path":"n.go","content":""}`), tools.Result{})
	if c := u.Content[0]; c.Type != "diff" || c.OldText != nil || c.NewText == nil || *c.NewText != "" {
		t.Errorf("write: %+v", u.Content)
	}
	u = endUpdate("1", "/w", call("read", `{"path":"a.go"}`), tools.Result{Content: "1|SECRET BODY", Summary: "a.go 1 line"})
	if b, _ := json.Marshal(u); strings.Contains(string(b), "SECRET BODY") || !strings.Contains(string(b), "a.go 1 line") {
		t.Errorf("read leaked its body or lost its summary: %s", b)
	}
	u = endUpdate("1", "/w", call("shell", `{"command":"false"}`), tools.Result{Content: "exit 1", IsError: true})
	if u.Status != statusFailed || u.Content[0].Content.Text != "exit 1" {
		t.Errorf("failed shell: %+v", u)
	}
	u = endUpdate("1", "/w", call("shell", `{"command":"yes"}`), tools.Result{Content: strings.Repeat("a", outputCap) + "TAIL"})
	if txt := u.Content[0].Content.Text; !strings.HasSuffix(txt, "TAIL") || len(txt) > outputCap+64 {
		t.Errorf("shell output not tail-capped: len %d", len(txt))
	}
	u = endUpdate("1", "/w", call("search", `{"pattern":"x"}`), tools.Result{Content: "HEAD" + strings.Repeat("a", outputCap)})
	if txt := u.Content[0].Content.Text; !strings.HasPrefix(txt, "HEAD") || len(txt) > outputCap+64 {
		t.Errorf("search output not head-capped: len %d", len(txt))
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234", Update: u})
}

func TestCapKeepsUTF8(t *testing.T) {
	s := strings.Repeat("é", outputCap) // 2 bytes per rune: the cap lands mid-rune
	for _, got := range []string{capHead(s, outputCap), capTail(s, outputCap)} {
		if !utf8.ValidString(got) {
			t.Fatal("cap split a rune")
		}
	}
	if capHead("short", outputCap) != "short" || capTail("short", outputCap) != "short" {
		t.Fatal("short strings must pass unchanged")
	}
}
