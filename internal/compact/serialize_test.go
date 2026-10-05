package compact

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/adeotek/moca/internal/llm"
)

func TestSerialize(t *testing.T) {
	es := []Entry{
		u("u1", "fix the bug"),
		{ID: "a1", Kind: KindAssistant, Msg: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "hmm"}, {Type: llm.BlockText, Text: "looking"}}}},
		{ID: "c1", Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"a.go","offset":10}`)}},
		{ID: "r1", Kind: KindToolResult, Result: &llm.ToolResult{Content: strings.Repeat("z", 3000)}},
		{ID: "r2", Kind: KindToolResult, Result: &llm.ToolResult{Content: "boom", IsError: true}},
	}
	s := Serialize(es)
	for _, want := range []string{"COMPACTION NOTE (lossy)", "[User]: fix the bug", "[Assistant thinking]: hmm",
		"[Assistant]: looking", "[Assistant tool calls]: read(offset=10, path=a.go)", "[… truncated]", "[Tool error]: boom"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(s, "z") != 2000 {
		t.Fatal("2K truncation")
	}
}

func TestTrackFiles(t *testing.T) {
	es := []Entry{
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"b.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"c.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"c.go","old_string":"a","new_string":"b"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "write", Input: json.RawMessage(`{"path":"d.go","content":""}`)}},
	}
	read, mod := TrackFiles(es, []string{"a.go"}, []string{"z.go"})
	if !slices.Equal(read, []string{"a.go", "b.go"}) || !slices.Equal(mod, []string{"c.go", "d.go", "z.go"}) {
		t.Fatal(read, mod)
	}
}

// Path spellings are normalized: a file read as "./c.go" and edited as
// "c.go" must not end up in both lists.
func TestTrackFilesNormalizesPaths(t *testing.T) {
	es := []Entry{
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"./c.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "read", Input: json.RawMessage(`{"path":"src/x.go"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"c.go","old_string":"a","new_string":"b"}`)}},
		{Kind: KindToolUse, Call: &llm.ToolCall{Name: "edit", Input: json.RawMessage(`{"path":"src/../src/x.go","old_string":"a","new_string":"b"}`)}},
	}
	read, mod := TrackFiles(es, nil, nil)
	if len(read) != 0 || !slices.Equal(mod, []string{"c.go", "src/x.go"}) {
		t.Fatal(read, mod)
	}
}

func TestCapChars(t *testing.T) {
	s := strings.Repeat("old line\n", 100) + "newest"
	got := CapChars(s, 100)
	if len(got) > 100+len("[… earlier history omitted]\n") || !strings.HasPrefix(got, "[… earlier history omitted]\n") || !strings.HasSuffix(got, "newest") {
		t.Fatalf("%q", got)
	}
	if CapChars("short", 100) != "short" {
		t.Fatal("no-op")
	}
}

// A byte cut must never strand a partial rune at the seam (newline-free CJK).
func TestCapCharsRuneSafe(t *testing.T) {
	got := CapChars(strings.Repeat("あ", 50), 10)
	if !strings.HasPrefix(got, "[… earlier history omitted]\n") || !utf8.ValidString(got) {
		t.Fatalf("%q", got)
	}
	got = CapChars("prefix\n"+strings.Repeat("é", 200), 101)
	if !utf8.ValidString(got) {
		t.Fatalf("newline-snapped tail splits a rune: %q", got)
	}
}

// Truncation of tool results and tool-call argument values is rune-safe too,
// and empty assistant text blocks do not emit bare lines.
func TestSerializeRuneSafeAndEmptyText(t *testing.T) {
	es := []Entry{
		{ID: "a1", Kind: KindAssistant, Msg: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockText, Text: ""}, {Type: llm.BlockText, Text: "kept"}}}},
		{ID: "c1", Kind: KindToolUse, Call: &llm.ToolCall{Name: "shell", Input: json.RawMessage(`{"command":"` + strings.Repeat("あ", 150) + `"}`)}},
		{ID: "r1", Kind: KindToolResult, Result: &llm.ToolResult{Content: strings.Repeat("い", 3000)}},
	}
	s := Serialize(es)
	if strings.Contains(s, "[Assistant]: \n") {
		t.Fatal("empty assistant text must be skipped")
	}
	if !utf8.ValidString(s) {
		t.Fatal("serialized payload must stay valid UTF-8")
	}
	if strings.Count(s, "い") != 2000/3 {
		t.Fatalf("2K byte cap kept %d chars", strings.Count(s, "い"))
	}
}
