package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

// newAgentModelCfg builds an agent model around a real started agent from a
// caller-supplied StartOptions (the config JSON is still written to cfgPath
// for the model's Files/dropdown paths). The helper is the general shape;
// newAgentModel is the common case (one fake provider, two models).
func newAgentModelCfg(t *testing.T, so agent.StartOptions, cfgPath string) *model {
	t.Helper()
	a, err := agent.Start(so)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Session().Close() })
	m := newModel(AppOptions{Start: so, ConfigPath: cfgPath, Home: "/home/u"}, a)
	m.start = so
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// newAgentModel builds a model around a real agent (openai-completions mock
// server answering "ok" to every request), so glue paths that need an agent —
// /clear, steering return, `!` notes, allow-always — run against real code.
func newAgentModel(t *testing.T) *model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n"+
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,
		"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536},"m2":{"contextWindow":65536}}}}}`, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{\"model\":\"fake/m\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return newAgentModelCfg(t, agent.StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: srv.Client(),
		Slug: "t", Ask: func(_ context.Context, _ tools.Question) tools.Answer { return tools.Deny }}, cfgPath)
}

// newThinkingAgentModel builds an agent whose model has effort levels to
// cycle, without touching the catalog or needing an env key: a fake ollama
// server reports a model with the "thinking" capability, and the registry's
// discovery grants it a ThinkingLevelMap (modelFromShow).
func newThinkingAgentModel(t *testing.T) *model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"name": "glm:t"}}})
		case r.URL.Path == "/api/show":
			json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []string{"completion", "tools", "thinking"},
				"parameters":   "num_ctx 65536",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"ollama/glm:t","providers":{"ollama":{"baseUrl":%q}}}`, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte(`{"model":"ollama/glm:t"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return newAgentModelCfg(t, agent.StartOptions{Config: cfg, Workdir: t.TempDir(),
		Slug: "t", Ask: func(_ context.Context, _ tools.Question) tools.Answer { return tools.Deny }}, cfgPath)
}

// printed renders what a print cmd would put in the scrollback: the scrollback
// body of a flush ({body}), or the concatenated bodies of a sequence of them
// (the acknowledgement that follows a flush prints nothing).
func printed(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	return flatten(cmd())
}

var cmdType = reflect.TypeOf((tea.Cmd)(nil))

func flatten(msg tea.Msg) string {
	if msg == nil {
		return ""
	}
	if _, ok := msg.(printedMsg); ok {
		return ""
	}
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == cmdType {
		var sb strings.Builder
		for i := 0; i < v.Len(); i++ {
			if c, _ := v.Index(i).Interface().(tea.Cmd); c != nil {
				sb.WriteString(flatten(c()))
			}
		}
		return sb.String()
	}
	return fmt.Sprint(msg)
}

// ack lets the next queued scrollback flush out, as the renderer's
// acknowledgement does in the running program.
func ack(m *model) tea.Cmd {
	_, cmd := m.Update(printedMsg{})
	return cmd
}

// settle acknowledges until the scrollback queue is idle.
func settle(m *model) {
	for m.outBusy {
		ack(m)
	}
}

// printLineType is the message type tea.Println commands produce. The runtime
// intercepts it before Update (it goes to the renderer); simulate mirrors
// that.
var printLineType = reflect.TypeOf(tea.Println("")())

// writeSkill writes a minimal valid SKILL.md under dir/<name>/ (picker and
// reload tests exercise skill discovery without loading real skills).
func writeSkill(t *testing.T, dir, name, desc string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n%s body\n", name, desc, name)
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// simulate drives a submitted command like the running event loop: it runs
// the command, expands sequences/batches, feeds every other message back
// through Update (following the commands Update returns) and collects the
// scrollback prints. Use it when a submission's flow includes messages the
// model must see (printedMsg, compactDoneMsg, runDoneMsg, …). The bool
// reports whether the flow ended in tea.Quit.
func simulate(m *model, cmd tea.Cmd) (out string, quit bool) {
	var sb strings.Builder
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
		if msg == nil {
			continue
		}
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == cmdType {
			for i := 0; i < v.Len(); i++ {
				if cc, _ := v.Index(i).Interface().(tea.Cmd); cc != nil {
					queue = append(queue, cc)
				}
			}
			continue
		}
		if reflect.TypeOf(msg) == printLineType {
			sb.WriteString(fmt.Sprint(msg))
			continue
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			quit = true
			continue
		}
		_, next := m.Update(msg)
		queue = append(queue, next)
	}
	return sb.String(), quit
}

func agentEvent(text string) agent.Event { return agent.TextDelta{Text: text} }

// secondLineOf is the second line of s ("" when s has at most one line) —
// the status bar's second row, after firstLineOf's first.
func secondLineOf(s string) string {
	_, rest, ok := strings.Cut(s, "\n")
	if !ok {
		return ""
	}
	l, _, _ := strings.Cut(rest, "\n")
	return l
}

// TestMain isolates the package from this host's real environment: an ambient
// provider key makes a built-in provider ready, and discovery then calls its
// real endpoint from a unit test (slow, flaky, and it changes the model lists
// these tests assert). The temp XDG root keeps a real config, auth store and
// sessions out of the run.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "moca-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	os.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	for _, k := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENCODE_API_KEY", "OPENROUTER_API_KEY"} {
		os.Setenv(k, "")
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
