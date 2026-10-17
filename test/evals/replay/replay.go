// Command replay re-runs a recorded session against a fresh fixture: the
// model's turns are served from the transcript, the tools execute for real.
// Because the model's output is fixed, harness-side formatting changes are
// measured exactly (request sizes, append-only prefix breaks) with zero model
// variance. Limit: the turns are counterfactual — a real model would react to
// the new bytes — so behavior changes still need the eval corpus (-compare).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/test/wire"
)

// Turn is one recorded assistant turn.
type Turn struct {
	Text     string
	Thinking string
	Calls    []llm.ToolCall
}

// Turns extracts the assistant turns of a transcript in order. Compaction
// summaries are not messages and are excluded; wrap-up and nudge answers are
// ordinary assistant messages and are included.
func Turns(entries []session.Entry) []Turn {
	calls := map[string][]llm.ToolCall{}
	for _, e := range entries {
		if e.Type == session.TypeToolUse && e.ToolUse != nil {
			calls[e.ToolUse.MessageID] = append(calls[e.ToolUse.MessageID], e.ToolUse.Call)
		}
	}
	var out []Turn
	for _, e := range entries {
		if e.Type != session.TypeMessage || e.Message == nil || e.Message.Role != llm.RoleAssistant {
			continue
		}
		t := Turn{Calls: calls[e.ID]}
		for _, b := range e.Message.Content {
			switch b.Type {
			case llm.BlockText:
				t.Text += b.Text
			case llm.BlockThinking:
				t.Thinking += b.Text
			}
		}
		out = append(out, t)
	}
	return out
}

// PromptOf returns the first user message text — the run's original prompt.
func PromptOf(entries []session.Entry) string {
	for _, e := range entries {
		if e.Type == session.TypeMessage && e.Message != nil && e.Message.Role == llm.RoleUser {
			return llm.TextOf(*e.Message)
		}
	}
	return ""
}

// UsesWeb reports a session that calls the web tool: its turns cannot be
// replayed hermetically (the fetch would hit the live network).
func UsesWeb(entries []session.Entry) bool {
	for _, e := range entries {
		if e.Type == session.TypeToolUse && e.ToolUse != nil && e.ToolUse.Call.Name == "web" {
			return true
		}
	}
	return false
}

// SSE renders a turn as one openai-completions stream. Deliberately no usage
// chunk: moca's chars/4 fallback then anchors deterministically.
func SSE(t Turn) string {
	var sb strings.Builder
	if t.Thinking != "" {
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":%s}}]}\n\n", jsonStr(t.Thinking))
	}
	if t.Text != "" {
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", jsonStr(t.Text))
	}
	for i, c := range t.Calls {
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":%s,\"function\":{\"name\":%s,\"arguments\":%s}}]}}]}\n\n",
			i, jsonStr(c.ID), jsonStr(c.Name), jsonStr(string(c.Input)))
	}
	finish := "stop"
	if len(t.Calls) > 0 {
		finish = "tool_calls"
	}
	fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":%q}]}\n\n", finish)
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Options configures one replayed run.
type Options struct {
	Moca    string // absolute path to the binary under test
	Fixture string // scenario fixture to copy into a fresh repo
	Prompt  string
	Flags   []string // the scenario's flags file, verbatim
	// Dirs is a directory this run owns outright (it is wiped first); the
	// replay uses <Dirs>/work, /data and /cfg. The workdir and the XDG paths
	// appear in the system prompt, so a compared pair of runs must get Dirs
	// paths of equal length or the request bytes differ for no reason.
	Dirs          string
	ContextWindow int // the model window the recorded run had
	Turns         []Turn
	Timeout       time.Duration
}

// Result is what one replayed run produced.
type Result struct {
	Requests     int
	Bytes        []int // one per request body
	PrefixBreaks int   // consecutive request pairs that are not append-only
	Diverged     bool  // the binary asked for more turns than recorded
	DivergeAt    int
	Output       string // combined stdout+stderr of the binary
}

// Run replays one recorded session: a local server answers request k with the
// recorded turn k (a `(replay exhausted)` reply sets Diverged), the binary runs
// on a fresh copy of the fixture.
func Run(ctx context.Context, o Options) (Result, error) {
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		mu     sync.Mutex
		bodies []any
		served int
		div    bool
		divAt  int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var decoded any
		_ = json.Unmarshal(raw, &decoded)
		mu.Lock()
		bodies = append(bodies, decoded)
		i := served
		served++
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if i < len(o.Turns) {
			io.WriteString(w, SSE(o.Turns[i]))
			return
		}
		mu.Lock()
		if !div {
			div, divAt = true, i
		}
		mu.Unlock()
		io.WriteString(w, SSE(Turn{Text: "(replay exhausted)"}))
	}))
	defer srv.Close()

	if err := os.RemoveAll(o.Dirs); err != nil {
		return Result{}, err
	}
	work, data, cfg := filepath.Join(o.Dirs, "work"), filepath.Join(o.Dirs, "data"), filepath.Join(o.Dirs, "cfg")
	for _, d := range []string{work, data, cfg} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Result{}, err
		}
	}
	if err := copyTree(o.Fixture, work); err != nil {
		return Result{}, fmt.Errorf("fixture copy: %w", err)
	}
	if err := gitInit(ctx, work); err != nil {
		return Result{}, fmt.Errorf("fixture git init: %w", err)
	}
	conf := fmt.Sprintf(`{"model":"replay/m","providers":{"replay":{"baseUrl":%q,"protocol":"openai-completions",`+
		`"auth":"api_key","apiKey":"env:REPLAY_KEY","models":{"m":{"contextWindow":%d}}}}}`,
		srv.URL+"/v1", o.ContextWindow)
	if err := os.WriteFile(filepath.Join(cfg, "config.jsonc"), []byte(conf), 0o600); err != nil {
		return Result{}, err
	}

	args := append([]string{"--config", filepath.Join(cfg, "config.jsonc")}, o.Flags...)
	args = append(args, "-p", o.Prompt)
	cmd := exec.CommandContext(ctx, o.Moca, args...)
	cmd.Dir = work
	cmd.Env = append(os.Environ(),
		"XDG_DATA_HOME="+data, "XDG_CONFIG_HOME="+cfg, "XDG_STATE_HOME="+data, "REPLAY_KEY=k")
	out, err := cmd.CombinedOutput()

	var res Result
	mu.Lock()
	res.Requests = len(bodies)
	res.Diverged, res.DivergeAt = div, divAt
	for _, b := range bodies {
		raw, _ := json.Marshal(b)
		res.Bytes = append(res.Bytes, len(raw))
	}
	mu.Unlock()
	res.Output = string(out)
	if err != nil {
		return res, fmt.Errorf("replay run: %w", err)
	}
	for i := 1; i < res.Requests; i++ {
		prev, next := messagesOf(bodies[i-1]), messagesOf(bodies[i])
		if prev == nil || next == nil {
			continue
		}
		if m, _ := wire.FirstDivergence(wire.Strip(prev).([]any), wire.Strip(next).([]any)); m != -1 {
			res.PrefixBreaks++
		}
	}
	return res, nil
}

func messagesOf(body any) []any {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	msgs, _ := m["messages"].([]any)
	return msgs
}

func gitInit(ctx context.Context, dir string) error {
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=replay", "-c", "user.email=replay@example.com", "commit", "-qm", "fixture"},
	} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
