package tui

// /goal pins (v0.8.0): the command lifecycle, pause/clear during a run, and
// truthful replay of host continuations.

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
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

// goalServer is a scripted openai-completions provider that can hold the
// first request until released (the pause/clear-mid-run pins).
type goalServer struct {
	mu      sync.Mutex
	turns   []string
	bodies  []map[string]any
	blocked chan struct{} // closed when the first request arrives (nil: never block)
	release chan struct{}
	srv     *httptest.Server
}

func newGoalServer(t *testing.T, turns ...string) *goalServer {
	s := &goalServer{turns: turns}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"m"}]}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		s.mu.Lock()
		s.bodies = append(s.bodies, m)
		n := len(s.bodies)
		blocked, release := s.blocked, s.release
		var turn string
		if len(s.turns) > 0 {
			turn, s.turns = s.turns[0], s.turns[1:]
		}
		s.mu.Unlock()
		if n == 1 && blocked != nil {
			close(blocked)
			<-release
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if turn == "" {
			turn = goalTextTurn("(script exhausted)")
		}
		io.WriteString(w, turn)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *goalServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

// goalTextTurn is one scripted text response (the agent package's textTurn
// equivalent, local to this suite).
func goalTextTurn(text string) string {
	b, _ := json.Marshal(text)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

// newGoalModel builds a TUI model around a goalServer-backed agent and
// collects every emitted event (the unit-test stand-in for the run pipe).
func newGoalModel(t *testing.T, s *goalServer, work string) (*model, *[]agent.Event) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,
		"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536}}}}}`, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{\"model\":\"fake/m\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var evs []agent.Event
	so := agent.StartOptions{Config: cfg, Workdir: work, HTTP: s.srv.Client(), Slug: "t",
		Ask: func(context.Context, tools.Question) tools.Answer { return tools.Deny },
		Emit: func(e agent.Event) {
			mu.Lock()
			evs = append(evs, e)
			mu.Unlock()
		}}
	return newAgentModelCfg(t, so, cfgPath), &evs
}

// drainEvents replays collected events through the TUI's handler.
func drainEvents(m *model, evs *[]agent.Event) string {
	var sb strings.Builder
	for _, e := range *evs {
		sb.WriteString(simulateAll(m, m.handleAgent(e)))
	}
	return sb.String()
}

func TestGoalCommandLifecycle(t *testing.T) {
	s := newGoalServer(t, goalTextTurn("worked"), goalTextTurn("COMPLETE"))
	work := t.TempDir()
	m, evs := newGoalModel(t, s, work)

	// Bare /goal before anything: the hint, not an error.
	if out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "goal"})); !strings.Contains(out, "no goal") {
		t.Fatalf("bare /goal: %q", out)
	}
	out := simulateAll(m, tuiCmd(t, m, "/goal make the fixture green")) + drainEvents(m, evs)
	if !strings.Contains(out, "› /goal make the fixture green") {
		t.Fatalf("the run must echo the command: %q", out)
	}
	if !strings.Contains(out, "goal started") || !strings.Contains(out, "goal complete") {
		t.Fatalf("start/complete notices: %q", out)
	}
	g, ok := m.agent.Goal()
	if !ok || g.Status != session.GoalComplete {
		t.Fatalf("goal %+v", g)
	}
	// Preview and first message are the objective itself.
	entries, _ := session.ReadFile(m.agent.Session().Path())
	var first string
	for _, e := range entries {
		if e.Type == session.TypeMessage && e.Message != nil && e.Message.Role == llm.RoleUser {
			first = llm.TextOf(*e.Message)
			break
		}
	}
	if first != "make the fixture green" {
		t.Fatalf("first stored text %q", first)
	}
	infos := session.List(filepath.Dir(m.agent.Session().Path()), work, "", 10)
	if len(infos) != 1 || infos[0].Preview != "make the fixture green" {
		t.Fatalf("preview %+v", infos)
	}
	// Bare status names the objective and the budget.
	if out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "goal"})); !strings.Contains(out, "goal complete · rounds 0/8") || !strings.Contains(out, "objective: make the fixture green") {
		t.Fatalf("status: %q", out)
	}
	// Clear, then bare is the hint again.
	out = simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "goal", Args: "clear"})) + drainEvents(m, evs)
	if !strings.Contains(out, "goal cleared") {
		t.Fatalf("clear: %q", out)
	}
	if g, ok := m.agent.Goal(); !ok || g.Status != session.GoalCleared {
		t.Fatalf("cleared goal state %+v ok=%v", g, ok)
	}
	if out := simulateAll(m, m.runCommand(Parsed{Kind: KindCommand, Name: "goal"})); !strings.Contains(out, "goal cleared") {
		t.Fatalf("status after clear: %q", out)
	}
}

// Pause and clear work during a run, never cancel it, and the driver starts
// no further round.
func TestGoalPauseClearWhileRunning(t *testing.T) {
	for _, tc := range []struct {
		name, control string
		status        session.GoalStatus
	}{
		{"pause", "pause", session.GoalPaused},
		{"clear", "clear", session.GoalCleared},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newGoalServer(t, goalTextTurn("round one"), goalTextTurn("CONTINUE"))
			s.blocked, s.release = make(chan struct{}), make(chan struct{})
			m, _ := newGoalModel(t, s, t.TempDir())
			// The goal runs on the agent's goroutine (as it does in the TUI),
			// holding on its first request so the control lands mid-run.
			done := make(chan struct{})
			go func() { defer close(done); m.agent.StartGoal(context.Background(), "objective") }()
			<-s.blocked
			m.running = true // the TUI's in-flight flag
			if _, ok := m.agent.Goal(); !ok {
				t.Fatal("the goal must be set before the first request completes")
			}
			// The control runs on the UI goroutine mid-run: allowed.
			if ctrl := m.runCommand(Parsed{Kind: KindCommand, Name: "goal", Args: tc.control}); ctrl != nil {
				simulateAll(m, ctrl)
			}
			close(s.release)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("the run did not finish")
			}
			g, ok := m.agent.Goal()
			if !ok || g.Status != tc.status {
				t.Fatalf("goal %+v", g)
			}
			if n := s.count(); n != 1 {
				t.Fatalf("%s mid-run started another round: %d requests", tc.name, n)
			}
		})
	}
}

// Replay never attributes a host goal continuation to the user.
func TestGoalReplayDoesNotInventUserText(t *testing.T) {
	m := newTestModel()
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "make it green"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "working"}}},
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: agentHostContinuation()}}},
	}
	out := drained(m, m.replay(msgs, 3, map[string]string{agentHostContinuation(): "goal_continue"}))
	if strings.Contains(out, userMarker+agentHostContinuation()) {
		t.Fatalf("the host continuation was replayed as the user: %q", out)
	}
	if !strings.Contains(out, "not sent by you") || !strings.Contains(out, "goal continuation") {
		t.Fatalf("replay note: %q", out)
	}
}

// agentHostContinuation is the stored text of a goal continuation round (the
// agent's own constant, restated here because it is unexported).
func agentHostContinuation() string { return "continue working toward the session goal" }
