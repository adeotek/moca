package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

func typeText(m *model, s string) {
	for _, r := range s {
		m.Update(keyMsg(string(r)))
	}
}

func TestParseSlash(t *testing.T) {
	cases := []struct {
		buf, cmd, arg string
		inArg, ok     bool
	}{
		{"/mod", "mod", "", false, true},
		{"/model ", "model", "", true, true},
		{"/model glm", "model", "glm", true, true},
		{"/model a b", "", "", false, false},
		{"/clear x", "", "", false, false}, // not an argument command
		{"//x", "", "", false, false},
		{"/model\tx", "", "", false, false},
	}
	for _, c := range cases {
		cmd, arg, inArg, ok := parseSlash(c.buf)
		if cmd != c.cmd || arg != c.arg || inArg != c.inArg || ok != c.ok {
			t.Errorf("parseSlash(%q) = %q %q %v %v", c.buf, cmd, arg, inArg, ok)
		}
	}
}

func TestModelArgCompletion(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("/model fake/")
	m.syncTextarea()
	d := m.dropdown()
	if d == nil || len(d.items) != 2 {
		t.Fatalf("both fake models listed: %+v", d)
	}
	for _, it := range d.items {
		if it.name == "fake/m" && it.hint != "current" {
			t.Fatalf("current model marked: %+v", it)
		}
	}
	// A provider with no credential is flagged (the catalog's built-ins have none here).
	m.input.SetBuffer("/model anthropic/")
	m.syncTextarea()
	if d = m.dropdown(); d == nil || !strings.Contains(d.items[0].hint, "no key") {
		t.Fatalf("models without a credential are flagged: %+v", d)
	}
	m.input.SetBuffer("/model m2") // substring, not prefix
	m.syncTextarea()
	if d = m.dropdown(); d == nil || len(d.items) != 1 || d.items[0].name != "fake/m2" {
		t.Fatalf("substring match: %+v", d)
	}
	m.Update(key("tab"))
	if m.input.Buffer() != "/model fake/m2" {
		t.Fatalf("tab completes the argument: %q", m.input.Buffer())
	}
	// Enter on an exact argument runs the command.
	_, cmd := m.Update(key("enter"))
	out, _ := simulate(m, cmd)
	if !strings.Contains(out, "switched to fake/m2") {
		t.Fatalf("switch: %q", out)
	}
}

func TestShowCompletionNewestFirst(t *testing.T) {
	m := newTestModel()
	m.items.AddThinking("a", 0)
	m.items.AddThinking("b", 0)
	m.items.AddThinking("c", 0)
	m.input.SetBuffer("/show ")
	m.syncTextarea()
	d := m.dropdown()
	if d == nil || dropNames(d)[0] != "3" || len(d.items) != 3 {
		t.Fatalf("items newest first: %+v", d)
	}
	m.input.SetBuffer("/show 2")
	m.syncTextarea()
	if d = m.dropdown(); d == nil || len(d.items) != 1 {
		t.Fatalf("prefix filter: %+v", d)
	}
}

func TestModelPicker(t *testing.T) {
	m := newAgentModel(t)
	m.input.SetBuffer("/model")
	m.syncTextarea()
	_, cmd := m.Update(key("enter"))
	simulate(m, cmd)
	if m.pick == nil || !strings.Contains(m.pick.items[m.pick.cursor].label, "fake/m  — current") {
		t.Fatalf("picker open on the current model: %+v", m.pick)
	}
	if !strings.Contains(m.View().Content, "switch to which model?") {
		t.Fatal("picker panel rendered")
	}
	m.Update(key("down"))
	_, cmd = m.Update(key("enter"))
	out, _ := simulate(m, cmd)
	if m.pick != nil || !strings.Contains(out, "switched to fake/m2") {
		t.Fatalf("choice: pick=%v out=%q", m.pick, out)
	}
	// esc cancels with a note.
	m.input.SetBuffer("/model")
	m.syncTextarea()
	_, cmd = m.Update(key("enter"))
	simulate(m, cmd)
	_, cmd = m.Update(key("esc"))
	if out, _ := simulate(m, cmd); m.pick != nil || !strings.Contains(out, "model unchanged") {
		t.Fatalf("cancel: %q", out)
	}
}

func TestWindowRange(t *testing.T) {
	for _, c := range []struct{ cur, n, limit, s, e int }{
		{0, 5, 8, 0, 5}, {0, 20, 8, 0, 8}, {7, 20, 8, 0, 8}, {8, 20, 8, 1, 9}, {19, 20, 8, 12, 20},
	} {
		if s, e := windowRange(c.cur, c.n, c.limit); s != c.s || e != c.e {
			t.Errorf("windowRange%v = %d,%d", c, s, e)
		}
	}
}

func TestToolArgSummary(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"shell", `{"command":"go test ./...\nexit 1"}`, "go test ./..."},
		{"read", `{"path":"a/b.go","offset":3}`, "a/b.go"},
		{"ls", `{}`, "."},
		{"search", `{"pattern":"TODO","path":"src"}`, "TODO in src"},
		{"mcp", `{"action":"call","server":"gh","tool":"issues"}`, "gh/issues"},
		{"edit", `not json`, ""},
	}
	for _, c := range cases {
		if got := toolArgSummary(llm.ToolCall{Name: c.name, Input: []byte(c.in)}); got != c.want {
			t.Errorf("%s: %q want %q", c.name, got, c.want)
		}
	}
}

func TestHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "history.jsonl")
	appendHistory(path, "/a", "one")
	appendHistory(path, "/a", "one") // consecutive duplicate
	appendHistory(path, "/b", "other project")
	appendHistory(path, "/a", "two\nlines")
	appendHistory(path, "/a", "bad\x1b[2J") // control bytes are never persisted
	appendHistory(path, "/a", strings.Repeat("x", historyMaxLine+1))
	got := loadHistory(path, "/a")
	if len(got) != 2 || got[0] != "one" || got[1] != "two\nlines" {
		t.Fatalf("history: %q", got)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("history file must be 0600: %v %v", fi, err)
	}
	if loadHistory("", "/a") != nil || loadHistory(filepath.Join(t.TempDir(), "none"), "/a") != nil {
		t.Fatal("missing file → no history")
	}
	for i := 0; i < historyTrimAt+10; i++ {
		appendHistory(path, "/c", "m"+string(rune('a'+i%26)))
	}
	loadHistory(path, "/c")
	b, _ := os.ReadFile(path)
	if n := strings.Count(string(b), "\n"); n > historyTrimTo {
		t.Fatalf("oversized history not trimmed: %d lines", n)
	}
}

func TestHistoryRecallAcrossLaunches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	m := newTestModel()
	m.opts.HistoryPath, m.opts.Start.Workdir = path, "/w"
	m.input.SetBuffer("remember me")
	m.syncTextarea()
	m.submit()
	m2 := newModel(AppOptions{HistoryPath: path, Home: "/home/u", Start: agent.StartOptions{Workdir: "/w"}}, nil)
	if !m2.input.HistoryPrev() || m2.input.Buffer() != "remember me" {
		t.Fatalf("a new launch recalls %q", m2.input.Buffer())
	}
}

func TestCtrlRHistorySearch(t *testing.T) {
	m := newTestModel()
	m.input.LoadHistory([]string{"alpha", "beta one\nsecond line", "gamma", "beta two"})
	m.Update(key("ctrl+r"))
	d := m.dropdown()
	if d == nil || !d.hist || len(d.items) != 4 || d.items[0].name != "beta two" {
		t.Fatalf("all entries, newest first: %+v", d)
	}
	typeText(m, "beta")
	d = m.dropdown()
	if d == nil || len(d.items) != 2 || !strings.HasSuffix(d.items[1].name, "⏎…") {
		t.Fatalf("filtered by the draft: %+v", d)
	}
	m.Update(key("down"))
	m.Update(key("enter"))
	if m.histSearch || m.input.Buffer() != "beta one\nsecond line" {
		t.Fatalf("enter puts the match in the draft (not sent): search=%v buf=%q", m.histSearch, m.input.Buffer())
	}
	if m.running {
		t.Fatal("history search must not send")
	}
	// esc leaves the draft alone.
	m.input.SetBuffer("")
	m.syncTextarea()
	m.Update(key("ctrl+r"))
	typeText(m, "zzz")
	if d = m.dropdown(); d == nil || len(d.items) != 0 || !strings.Contains(ansi.Strip(m.dropdownPanel(d)), "no history matches") {
		t.Fatalf("no match is said so: %+v", d)
	}
	m.Update(key("enter")) // swallowed: nothing to send mid-search
	if !m.histSearch {
		t.Fatal("enter with no match keeps searching")
	}
	m.Update(key("esc"))
	if m.histSearch || m.input.Buffer() != "zzz" {
		t.Fatalf("esc cancels, draft kept: search=%v buf=%q", m.histSearch, m.input.Buffer())
	}
}

func TestResumeFlowAndReplay(t *testing.T) {
	m := newAgentModel(t)
	if _, err := m.agent.Run(context.Background(), "first question"); err != nil {
		t.Fatal(err)
	}
	old := m.agent.Session().ID8()
	simulate(m, m.restartSession())
	if m.agent.Session().ID8() == old {
		t.Fatal("restart made a new session")
	}
	cmd := m.runResume("")
	simulate(m, cmd)
	if m.pick == nil || len(m.pick.items) != 1 || !strings.Contains(m.pick.items[0].label, "first question") {
		t.Fatalf("picker lists the earlier session: %+v", m.pick)
	}
	_, cmd = m.Update(key("enter"))
	simulate(m, cmd)
	if m.agent.Session().ID8() != old {
		t.Fatalf("resumed %s, want %s", m.agent.Session().ID8(), old)
	}
	out, _ := simulate(m, m.handleAgent(agent.Resumed{ID8: old, Messages: 2}))
	out = ansi.Strip(out)
	if !strings.Contains(out, "resumed "+old) || !strings.Contains(out, "› first question") || !strings.Contains(out, "ok") {
		t.Fatalf("replay: %q", out)
	}
	// The open session is not offered (the abandoned one has no messages);
	// an unknown id is an error line.
	if len(m.otherSessions()) != 0 {
		t.Fatalf("other sessions: %+v", m.otherSessions())
	}
	out, _ = simulate(m, m.runResume("deadbeef"))
	if !strings.Contains(out, "no session deadbeef") {
		t.Fatalf("unknown id: %q", out)
	}
}

func TestReplayLimitsTurnsAndShowsItems(t *testing.T) {
	m := newTestModel()
	user := func(s string) llm.Message {
		return llm.Message{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: s}}}
	}
	call := llm.ToolCall{ID: "c1", Name: "read", Input: []byte(`{"path":"x.go"}`)}
	msgs := []llm.Message{
		user("turn one"), {Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a1"}}},
		user("turn two"), {Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a2"}}},
		user("turn three"),
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{
			{Type: llm.BlockThinking, Text: "hmm"},
			{Type: llm.BlockToolUse, ToolCall: &call},
		}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockToolResult, ToolResult: &llm.ToolResult{CallID: "c1", Content: "file body"}}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "a3"}}},
		user("turn four"),
	}
	out, _ := simulate(m, m.replay(msgs, 2))
	out = ansi.Strip(out)
	for _, want := range []string{"2 earlier turns not shown", "› turn three", "⋯ #1 thinking 1 line", "▸ #2 read x.go", " a3", "› turn four"} {
		if !strings.Contains(out, want) {
			t.Errorf("replay lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "turn one") || strings.Contains(out, "turn two") {
		t.Fatalf("older turns must not be replayed:\n%s", out)
	}
	if it, ok := m.items.Get(2); !ok || it.Body != "file body" {
		t.Fatalf("replayed items open in the pager: %+v", it)
	}
}

func TestMatchRanges(t *testing.T) {
	got := matchRanges("Foo foo FOO bar", "foo")
	if len(got) != 3 || got[0][0] != 0 || got[1][0] != 4 || got[2][0] != 8 || got[2][1] != 11 {
		t.Fatalf("case-insensitive ranges: %v", got)
	}
	if matchRanges("abc", "") != nil || matchRanges("abc", "zzz") != nil {
		t.Fatal("no query / no match")
	}
}

func TestPagerSearch(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 60; i++ {
		body.WriteString("filler line\n")
	}
	body.WriteString("the NEEDLE is here\n")
	for i := 0; i < 20; i++ {
		body.WriteString("more filler\n")
	}
	body.WriteString("second needle\n")
	m := newTestModel()
	m.openPager(Item{N: 1, Kind: "tool", Line: "▸ #1 x", Body: body.String()})
	m.Update(key("/"))
	if !m.pager.typing {
		t.Fatal("/ starts typing the query")
	}
	typeText(m, "needle")
	if !strings.Contains(ansi.Strip(m.pager.view(false)), "/needle▏") {
		t.Fatalf("query shown while typing: %q", ansi.Strip(m.pager.view(false)))
	}
	m.Update(key("enter"))
	if m.pager.typing || m.pager.hits != 2 || m.pager.cur != 0 {
		t.Fatalf("applied: typing=%v hits=%d cur=%d", m.pager.typing, m.pager.hits, m.pager.cur)
	}
	v := ansi.Strip(m.pager.view(false))
	if !strings.Contains(v, "the NEEDLE is here") || !strings.Contains(v, "1/2") {
		t.Fatalf("scrolled to the first match: %q", v)
	}
	m.Update(key("n"))
	if m.pager.cur != 1 || !strings.Contains(ansi.Strip(m.pager.view(false)), "second needle") {
		t.Fatalf("n steps to the next match: cur=%d", m.pager.cur)
	}
	m.Update(key("N"))
	if m.pager.cur != 0 {
		t.Fatal("N steps back")
	}
	// A typed 'q' inside the query does not close the pager; esc cancels typing first.
	m.Update(key("/"))
	typeText(m, "q")
	if m.pager == nil || m.pager.query != "q" {
		t.Fatal("keys go to the query while typing")
	}
	m.Update(key("esc"))
	if m.pager == nil || m.pager.typing {
		t.Fatal("esc cancels the query, not the pager")
	}
	m.Update(key("q"))
	if m.pager != nil {
		t.Fatal("q closes")
	}
}

func TestPagerShowsPendingApproval(t *testing.T) {
	m := newTestModel()
	m.openPager(Item{N: 1, Line: "▸ #1 x", Body: "b"})
	if strings.Contains(ansi.Strip(m.pager.view(false)), "approval pending") {
		t.Fatal("no marker without a question")
	}
	if !strings.Contains(ansi.Strip(m.pager.view(true)), "approval pending") {
		t.Fatal("marker with a question waiting")
	}
}

// With an ollama server configured, /model re-reads its models before the
// picker opens, so a model pulled meanwhile is there.
func TestModelPickerRefreshesOllama(t *testing.T) {
	models := []string{"llama3.1:8b"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		for i, n := range models {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"name":%q}`, n)
		}
		fmt.Fprintf(w, `{"models":[%s]}`, b.String())
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"capabilities":["completion","tools"],"parameters":"num_ctx 32768"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Parse([]byte(`{"model":"ollama/llama3.1:8b","providers":{"ollama":{"baseUrl":"` + srv.URL + `"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	so := agent.StartOptions{Config: cfg, Workdir: t.TempDir(), Slug: "t"}
	a, err := agent.Start(so)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	m := newModel(AppOptions{Start: so, Home: "/home/u"}, a)
	m.start = so
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	models = append(models, "qwen3:8b") // pulled after moca started
	cmd := m.runCommand(Parsed{Kind: KindCommand, Name: "model"})
	if m.pick != nil || m.status.Transient != "reading models…" || cmd == nil {
		t.Fatalf("the picker opens after the refresh, not before: pick=%v %q", m.pick, m.status.Transient)
	}
	// The runtime runs the command off the loop and delivers its message.
	msg, ok := cmd().(modelsRefreshedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("refresh: %#v", msg)
	}
	_, cmd = m.Update(msg)
	simulate(m, cmd)
	if m.pick == nil || m.status.Transient != "" {
		t.Fatalf("picker open, transient cleared: pick=%v %q", m.pick, m.status.Transient)
	}
	var labels []string
	for _, it := range m.pick.items {
		labels = append(labels, it.key)
	}
	if !contains(labels, "ollama/qwen3:8b") || !strings.Contains(m.pick.items[m.pick.cursor].label, "ollama/llama3.1:8b  — current") {
		t.Fatalf("picker lacks the new model or the current marker: %v", labels)
	}
	for _, it := range m.pick.items {
		if strings.HasPrefix(it.key, "ollama/") && strings.Contains(it.label, "no key") {
			t.Fatalf("a local model is never 'no key': %q", it.label)
		}
	}
	// An unreachable server is a warning, and the picker still opens.
	m.pick = nil
	_, cmd = m.Update(modelsRefreshedMsg{err: fmt.Errorf("cannot reach ollama at http://x")})
	if out, _ := simulate(m, cmd); !strings.Contains(out, "cannot reach ollama") || m.pick == nil {
		t.Fatalf("warning + picker: %q pick=%v", out, m.pick)
	}
}

func TestLoginRowForOllama(t *testing.T) {
	if r := providerRow("ollama"); !strings.Contains(r, "local server") {
		t.Fatal(r)
	}
}
