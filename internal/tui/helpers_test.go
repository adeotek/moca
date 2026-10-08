package tui

import (
	"context"
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
	so := agent.StartOptions{Config: cfg, Workdir: t.TempDir(), HTTP: srv.Client(), Slug: "t",
		Ask: func(_ context.Context, _ tools.Question) tools.Answer { return tools.Deny }}
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
