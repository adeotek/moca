package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

func mustJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestResumeRebuildsExactContext(t *testing.T) {
	s := newScript(t, textTurn("first answer"), textTurn("second answer"))
	a, work, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "first question")
	a.SetEffort(llm.EffortLow)
	path := a.Session().Path()
	sys := a.opts.System
	a.Session().Close()

	b, err := Resume(StartOptions{Config: a.opts.Config, Workdir: "/ignored", HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	if b.opts.System != sys || b.Effort() != a.Effort() || b.opts.Env.Root != a.opts.Env.Root {
		t.Fatal("system prompt verbatim, effort restored, workdir from header")
	}
	if b.Workdir() != a.Workdir() {
		t.Fatal("workdir from header, not the resuming process")
	}
	b.Run(context.Background(), "second question")
	msgs := s.bodies[1]["messages"].([]any)
	if msgs[0].(map[string]any)["content"] != sys || len(msgs) != 4 {
		t.Fatalf("resumed request = stored system + full history + new prompt: %v", msgs)
	}
	_ = work
}

func TestResumeAfterCrashMidTool(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	// simulate a crash: assistant message + tool_use persisted, no result
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := session.ReadFile(b.Session().Path())
	last := entries[len(entries)-1]
	if last.ToolResult == nil || last.ToolResult.Content != session.Interrupted {
		t.Fatalf("%+v", last)
	}
}

func TestResumeNeverRestoresYolo(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	a.SetYolo(true)
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil || b.Yolo() {
		t.Fatal("resume starts with checks on unless the resuming process asks for yolo", err)
	}
	b.Session().Close()
	c, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client(), Yolo: true}, path)
	if !c.Yolo() {
		t.Fatal("--yolo on resume applies")
	}
	c.Session().Close()
}

func TestResumeHonoursCompaction(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startTestWith(t, s, "", "")
	u1, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("old")})
	a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "old answer"}}}})
	u2, _ := a.append(session.Entry{Type: session.TypeMessage, Message: userText("kept")})
	a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{Summary: "SUMMARY", FirstKeptEntryID: u2.ID}})
	_ = u1
	path := a.Session().Path()
	a.Session().Close()
	b, _ := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	b.Run(context.Background(), "new")
	body := mustJSONString(s.bodies[0])
	if !strings.Contains(body, "SUMMARY") || strings.Contains(body, "old answer") {
		t.Fatal(body)
	}
}

func TestResumeNotASession(t *testing.T) {
	if _, err := Resume(StartOptions{}, t.TempDir()+"/nope.jsonl"); err == nil {
		t.Fatal("missing file must fail")
	}
}

// A session whose workdir no longer exists must fail with a clear message
// and must not append repair entries for a resume that never became usable.
func TestResumeMissingWorkdir(t *testing.T) {
	s := newScript(t)
	a, work, _ := startTestWith(t, s, "", "")
	// Make it a crash: an unanswered tool_use that a resume would repair.
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	before, _ := session.ReadFile(path)
	if err := os.RemoveAll(work); err != nil {
		t.Fatal(err)
	}
	_, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err == nil || !strings.Contains(err.Error(), "workdir") || !strings.Contains(err.Error(), work) {
		t.Fatalf("%v", err)
	}
	after, _ := session.ReadFile(path)
	if len(after) != len(before) {
		t.Fatalf("no repair entries may be appended: before=%d after=%d", len(before), len(after))
	}
}

// catalogConfig declares two catalog models (which support efforts, unlike
// custom ones that clamp everything to "off") against the script server.
func catalogConfig(t *testing.T, s *scriptServer, top string) config.Config {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"opencode-go/glm-5.3-flash"%s,"providers":{"opencode-go":{
		"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"glm-5.3-flash":{},"glm-5.3":{}}}}}`, top, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func startCatalog(t *testing.T, s *scriptServer, cfg config.Config) *Agent {
	t.Helper()
	a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: s.srv.Client(), Slug: "t"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func lastModelChange(t *testing.T, path string) *session.ModelChange {
	t.Helper()
	entries, err := session.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var mc *session.ModelChange
	for _, e := range entries {
		if e.Type == session.TypeModelChange {
			mc = e.ModelChange
		}
	}
	return mc
}

// §8: "model_change … resume restores both". The custom test models clamp
// every effort to "off", so this runs on catalog models where the restored
// values can differ from the defaults.
func TestResumeRestoresModelAndEffortFromModelChange(t *testing.T) {
	s := newScript(t)
	cfg := catalogConfig(t, s, "")
	a := startCatalog(t, s, cfg)
	if err := a.SetModel("opencode-go/glm-5.3", llm.EffortHigh); err != nil {
		t.Fatal(err)
	}
	a.SetEffort(llm.EffortLow)
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: cfg, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	if b.Model().Qualified() != "opencode-go/glm-5.3" || b.Effort() != llm.EffortLow {
		t.Fatalf("restored %s/%s, want opencode-go/glm-5.3/low", b.Model().Qualified(), b.Effort())
	}
}

// §12.5: --effort overrides the effort for this session — on resume too.
func TestResumeEffortFlagOverridesStored(t *testing.T) {
	s := newScript(t)
	cfg := catalogConfig(t, s, "")
	a := startCatalog(t, s, cfg)
	a.SetEffort(llm.EffortLow)
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: cfg, Effort: "high", HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Effort() != llm.EffortHigh {
		t.Fatalf("effort %s, want high", b.Effort())
	}
	b.Session().Close()
	if mc := lastModelChange(t, path); mc == nil || mc.Effort != "high" {
		t.Fatalf("the override must be recorded: %+v", mc)
	}
	c, err := Resume(StartOptions{Config: cfg, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Session().Close()
	if c.Effort() != llm.EffortHigh {
		t.Fatalf("a later plain resume restores the recorded override, got %s", c.Effort())
	}
	if _, err := Resume(StartOptions{Config: cfg, Effort: "bogus", HTTP: s.srv.Client()}, path); err == nil {
		t.Fatal("a bad effort is an error, not ignored")
	}
}

// A stored model that is no longer configured must not strand the session:
// --model replaces it, and without --model the error names the cause and the
// way out — before any repair entry is written.
func TestResumeStoredModelNoLongerConfigured(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	a.Session().Close()
	before, _ := session.ReadFile(path)
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"other/x","providers":{"other":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"x":{"contextWindow":65536}}}}}`, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resume(StartOptions{Config: cfg, HTTP: s.srv.Client()}, path)
	if err == nil || !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), "--model") || strings.Contains(err.Error(), "workdir") {
		t.Fatalf("want a clear model error, got: %v", err)
	}
	if after, _ := session.ReadFile(path); len(after) != len(before) {
		t.Fatalf("a failed resume must not append repair entries: %d → %d", len(before), len(after))
	}
	b, err := Resume(StartOptions{Config: cfg, Model: "other/x", HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatalf("--model must rescue the session: %v", err)
	}
	defer b.Session().Close()
	if b.Model().Qualified() != "other/x" {
		t.Fatalf("model %s", b.Model().Qualified())
	}
	if mc := lastModelChange(t, path); mc == nil || mc.Model != "other/x" {
		t.Fatalf("the override must be recorded: %+v", mc)
	}
}

// Two writers on one session would interleave histories, and the second
// process would "repair" the first one's in-flight tool call, leaving two
// results for one call.
func TestResumeRefusesSessionHeldByAnotherProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no advisory file locks on windows")
	}
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	m, _ := a.append(session.Entry{Type: session.TypeMessage, Message: &llm.Message{Role: llm.RoleAssistant}})
	a.append(session.Entry{Type: session.TypeToolUse, ToolUse: &session.ToolUse{MessageID: m.ID, Call: llm.ToolCall{ID: "c1", Name: "shell"}}})
	path := a.Session().Path()
	before, _ := session.ReadFile(path)
	_, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	var se *StartError
	if !errors.Is(err, session.ErrInUse) || !errors.As(err, &se) {
		t.Fatalf("resume of a live session must be refused as a runtime error: %v", err)
	}
	if after, _ := session.ReadFile(path); len(after) != len(before) {
		t.Fatal("a refused resume must not repair the other process's in-flight call")
	}
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatalf("once the holder is gone the session resumes (and is repaired): %v", err)
	}
	b.Session().Close()
}

// Hard mode is not in the transcript as a pair: a session that ended on
// modelHard must resume with /hard able to leave it again.
func TestResumeRestoresHardMode(t *testing.T) {
	s := newScript(t)
	cfg := catalogConfig(t, s, `,"modelHard":"opencode-go/glm-5.3"`)
	a := startCatalog(t, s, cfg)
	if on, err := a.ToggleHard(); err != nil || !on {
		t.Fatal(on, err)
	}
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: cfg, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	if !b.HardOn() || b.Model().Qualified() != "opencode-go/glm-5.3" {
		t.Fatalf("hard mode must survive resume: hard=%v model=%s", b.HardOn(), b.Model().Qualified())
	}
	if on, err := b.ToggleHard(); err != nil || on || b.Model().Qualified() != "opencode-go/glm-5.3-flash" {
		t.Fatalf("/hard must return to the default model: on=%v model=%s err=%v", on, b.Model().Qualified(), err)
	}
	// A session that never used hard mode stays out of it.
	c := startCatalog(t, s, cfg)
	cpath := c.Session().Path()
	c.Session().Close()
	d, err := Resume(StartOptions{Config: cfg, HTTP: s.srv.Client()}, cpath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Session().Close()
	if d.HardOn() {
		t.Fatal("a normal session must not resume in hard mode")
	}
}
