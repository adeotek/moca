package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// scriptServer serves openai-completions SSE responses from a script, one
// per request, and records request bodies.
type scriptServer struct {
	mu     sync.Mutex
	turns  []string
	bodies []map[string]any
	srv    *httptest.Server
}

func textTurn(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

func toolTurn(calls ...[2]string) string {
	var sb strings.Builder
	for i, c := range calls {
		args, _ := json.Marshal(c[1])
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"call_%d\",\"function\":{\"name\":%q,\"arguments\":%s}}]}}]}\n\n", i, i, c[0], args)
	}
	sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
	return sb.String()
}

func newScript(t *testing.T, turns ...string) *scriptServer {
	s := &scriptServer{turns: turns}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		s.bodies = append(s.bodies, m)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(s.turns) == 0 {
			io.WriteString(w, textTurn("(script exhausted)"))
			return
		}
		io.WriteString(w, s.turns[0])
		s.turns = s.turns[1:]
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func startTest(t *testing.T, s *scriptServer, maxSteps int, mods ...func(*StartOptions)) (*Agent, string, *[]Event) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	work := t.TempDir()
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","context":{"maxSteps":%d},
		"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536}}}}}`, maxSteps, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	var evs []Event
	o := StartOptions{Config: cfg, Workdir: work, Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client(), Slug: "t"}
	for _, mod := range mods {
		mod(&o)
	}
	a, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	return a, work, &evs
}

func TestLoopReadEditVerify(t *testing.T) {
	s := newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}, [2]string{"ls", `{}`}),
		toolTurn([2]string{"edit", `{"path":"a.txt","old_string":"bug","new_string":"fix"}`}),
		textTurn("Fixed."),
	)
	a, work, evs := startTest(t, s, 40)
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("bug\n"), 0o644)
	out, err := a.Run(context.Background(), "fix it")
	if err != nil || out.Text != "Fixed." || out.MaxSteps {
		t.Fatal(out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(work, "a.txt")); string(b) != "fix\n" {
		t.Fatal(string(b))
	}
	// third request carries both tool results of turn 1 + the edit result, in order
	msgs := s.bodies[2]["messages"].([]any)
	var roles []string
	for _, m := range msgs {
		roles = append(roles, m.(map[string]any)["role"].(string))
	}
	if strings.Join(roles, ",") != "system,user,assistant,tool,tool,assistant,tool" {
		t.Fatal(roles)
	}
	var starts int
	for _, e := range *evs {
		if _, ok := e.(ToolStart); ok {
			starts++
		}
	}
	if starts != 3 {
		t.Fatal("tool events")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if entries[0].Type != session.TypeSession || !strings.Contains(entries[0].Session.SystemPrompt, "moca") {
		t.Fatal("session header with stored prompt")
	}
	if got := session.Messages(entries); len(got) != 6 {
		t.Fatalf("transcript rebuilds the same conversation: %d", len(got))
	}
}

func TestFailedCallDoesNotCancelRest(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"read", `{"path":"nope"}`}, [2]string{"ls", `{}`}), textTurn("ok"))
	a, _, evs := startTest(t, s, 40)
	a.Run(context.Background(), "go")
	var ends []ToolEnd
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok {
			ends = append(ends, te)
		}
	}
	if len(ends) != 2 || !ends[0].Result.IsError || ends[1].Result.IsError {
		t.Fatal(ends)
	}
}

func TestMaxStepsWrapUp(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}), toolTurn([2]string{"ls", `{}`}), textTurn("Summary: did ls twice."))
	a, _, _ := startTest(t, s, 2)
	out, err := a.Run(context.Background(), "loop")
	if err != nil || !out.MaxSteps || out.Text != "Summary: did ls twice." {
		t.Fatal(out, err)
	}
	last := s.bodies[len(s.bodies)-1]
	if last["tool_choice"] != "none" || last["tools"] == nil {
		t.Fatal("wrap-up: tool_choice none, tools still listed (cache prefix)")
	}
}

func TestShellRefusedInOneShot(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"shell", `{"command":"rm -rf x"}`}), textTurn("couldn't"))
	a, _, evs := startTest(t, s, 40)
	a.Run(context.Background(), "clean")
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && (!te.Result.IsError || !strings.Contains(te.Result.Content, "rm")) {
			t.Fatal(te.Result)
		}
	}
}

func TestAbortWritesSyntheticResults(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"ls", `{}`}, [2]string{"ls", `{}`}))
	a, _, _ := startTest(t, s, 40)
	ctx, cancel := context.WithCancel(context.Background())
	a.opts.Emit = func(e Event) {
		if _, ok := e.(ToolEnd); ok {
			cancel()
		}
	}
	_, err := a.Run(ctx, "x")
	if err == nil {
		t.Fatal("cancelled run returns ctx error")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("every tool_use must have a result after abort")
	}
	last := entries[len(entries)-1]
	if last.ToolResult == nil || last.ToolResult.Content != session.AbortedByUser {
		t.Fatalf("%+v", last)
	}
}

func TestLengthStopTruncatedToolCall(t *testing.T) {
	trunc := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c\",\"function\":{\"name\":\"write\",\"arguments\":\"{\\\"path\\\":\\\"x\\\",\\\"content\\\":\\\"aaa\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"
	s := newScript(t, trunc, textTurn("ok"))
	a, work, evs := startTest(t, s, 40)
	a.Run(context.Background(), "x")
	if _, err := os.Stat(filepath.Join(work, "x")); err == nil {
		t.Fatal("truncated call must not execute")
	}
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && !strings.Contains(strings.ToLower(te.Result.Content), "split") {
			t.Fatal(te.Result.Content)
		}
	}
	_ = llm.StopLength
}
