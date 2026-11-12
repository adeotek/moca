//go:build goalgate

// Package goalgate drives the built `moca` TUI in tmux against a scripted
// openai-completions provider and asserts the /goal contract end to end:
// work → CONTINUE → work → COMPLETE, the controls, pause mid-review, a
// process restart that pauses, an explicit resume, and no key leak. Run
// through test/goalgate/run.sh.
package goalgate

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	mocaBin = flag.String("moca", "", "path to the built moca binary")
	gateDir = flag.String("gatedir", "", "scratch dir kept for artifacts on failure")
)

const secret = "goalgate-not-a-real-key-91c2"

// provider serves scripted SSE turns, split by kind: a request whose system
// prompt carries the review marker consumes a review answer, everything else
// a work answer. blockReview(n) holds the n-th upcoming review until its
// release is closed (or the request is cancelled) — the mid-review legs.
type provider struct {
	mu        sync.Mutex
	work      []string
	reviews   []string
	bodies    []map[string]any
	reviewsN  int
	blockAt   int
	ready     chan struct{}
	release   chan struct{}
	readyOnce sync.Once
	srv       *httptest.Server
}

func newProvider(t *testing.T, work, reviews []string) *provider {
	p := &provider{work: work, reviews: reviews, srv: nil}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"m"}]}`)
			return
		}
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		review := strings.Contains(fmt.Sprint(m["messages"]), "coding session's evidence")
		p.mu.Lock()
		p.bodies = append(p.bodies, m)
		idx := 0
		if review {
			p.reviewsN++
			idx = p.reviewsN
		}
		blocked := review && p.blockAt > 0 && idx == p.blockAt
		release, ready := p.release, p.ready
		var turn string
		if review {
			turn = pop(&p.reviews, "(review script exhausted)")
		} else {
			turn = pop(&p.work, "(work script exhausted)")
		}
		p.mu.Unlock()
		if blocked {
			p.readyOnce.Do(func() { close(ready) })
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, text(turn))
	}))
	t.Cleanup(p.srv.Close)
	return p
}

// blockReview holds the n-th upcoming reviewer call; it returns the ready and
// release channels for that hold. Each call gets fresh channels, so legs do
// not leak state into each other.
func (p *provider) blockReview(n int) (<-chan struct{}, chan<- struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockAt = p.reviewsN + n
	p.ready, p.release = make(chan struct{}), make(chan struct{})
	p.readyOnce = sync.Once{}
	return p.ready, p.release
}

func pop(list *[]string, fallback string) string {
	if len(*list) == 0 {
		return fallback
	}
	v := (*list)[0]
	*list = (*list)[1:]
	return v
}

func (p *provider) counts() (work, review int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(p.bodies)
	return n - p.reviewsN, p.reviewsN
}

func text(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\n"+
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

// --- tmux ------------------------------------------------------------------

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

type pane struct {
	t    *testing.T
	name string
	id   string // the pane id: capture-pane/send-keys target THIS, never the session name
	log  *os.File
}

func tmux(t *testing.T, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("tmux", args...).CombinedOutput()
	return string(out), err
}

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatal("goalgate requires tmux — install it (the gate never skips silently)")
	}
}

// start launches the TUI in a detached tmux session through a script (the
// tmux server would otherwise own the environment). The pane id is resolved
// once and every capture/send targets it: a session-name target can resolve
// through the window table and land on another pane under a custom tmux
// configuration.
func start(t *testing.T, name, work, script string) *pane {
	t.Helper()
	tmux(t, "kill-session", "-t", name)
	if out, err := exec.Command("tmux", "new-session", "-d", "-s", name, "-x", "120", "-y", "40",
		"-c", work, "bash", script).CombinedOutput(); err != nil {
		t.Fatalf("tmux new-session: %v %s", err, out)
	}
	t.Cleanup(func() { tmux(t, "kill-session", "-t", name) })
	out, err := exec.Command("tmux", "list-panes", "-t", name, "-F", "#{pane_id}").Output()
	id := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if err != nil || id == "" {
		t.Fatalf("tmux list-panes %s: %v %s", name, err, out)
	}
	return &pane{t: t, name: name, id: id}
}

func (p *pane) capture() string {
	out, _ := exec.Command("tmux", "capture-pane", "-p", "-t", p.id).Output()
	return ansi.ReplaceAllString(string(out), "")
}

func (p *pane) waitFor(want string, timeout time.Duration) string {
	p.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if out := p.capture(); strings.Contains(out, want) {
			return out
		}
		time.Sleep(100 * time.Millisecond)
	}
	return p.capture()
}

func (p *pane) send(line string) {
	p.t.Helper()
	exec.Command("tmux", "send-keys", "-t", p.id, "-l", line).Run()
	exec.Command("tmux", "send-keys", "-t", p.id, "Enter").Run()
}

func (p *pane) kill() { tmux(p.t, "kill-session", "-t", p.name) }

// --- test ------------------------------------------------------------------

func TestGoalGate(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags goalgate ./test/goalgate -args -moca <binary>")
	}
	requireTmux(t)
	base := *gateDir
	if base == "" {
		base = t.TempDir()
	}
	work, data, cfg, state := filepath.Join(base, "work"), filepath.Join(base, "data"),
		filepath.Join(base, "config"), filepath.Join(base, "state")
	for _, d := range []string{work, data, cfg, state} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Artifacts survive a failure: the scratch tree is printed by run.sh and
	// kept unless every leg passed.
	if *gateDir != "" {
		t.Cleanup(func() {
			if !t.Failed() {
				os.RemoveAll(base)
			}
		})
	}

	p := newProvider(t,
		[]string{"working", "working more", "third round", "fourth round", "fifth round"},
		[]string{"CONTINUE", "COMPLETE", "BLOCKED", "BLOCKED", "COMPLETE"},
	)
	os.MkdirAll(filepath.Join(cfg, "moca"), 0o700)
	cfgJSON := fmt.Sprintf(`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions",
		"auth":"api_key","apiKey":"env:GATE_KEY","models":{"m":{"contextWindow":65536}}}}}`+"\n", p.srv.URL)
	if err := os.WriteFile(filepath.Join(cfg, "moca", "config.jsonc"), []byte(cfgJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(base, "run-tui.sh")
	writeScript(t, script, fmt.Sprintf(`#!/usr/bin/env bash
export XDG_DATA_HOME=%q XDG_CONFIG_HOME=%q XDG_STATE_HOME=%q
export GATE_KEY=%s MOCA_LOG=debug
exec %q --approve
`, data, cfg, state, secret, *mocaBin), 0o700)

	// Leg 1: work → CONTINUE → work → COMPLETE.
	s := start(t, "goalgate-a", work, script)
	ready := s.waitFor("How can I help you today?", 30*time.Second)
	if !strings.Contains(ready, "How can I help you today?") {
		saveArtifacts(t, base, ready, "", "")
		t.Fatalf("the TUI did not start:\n%s", ready)
	}
	s.send("/goal make the fixture green")
	out := s.waitFor("goal complete", 60*time.Second)
	if !strings.Contains(out, "goal complete") {
		saveArtifacts(t, base, s.capture(), "", "")
		t.Fatalf("no completion:\n%s", out)
	}
	if w, r := p.counts(); w != 2 || r != 2 {
		saveArtifacts(t, base, s.capture(), "", "")
		t.Fatalf("requests: work %d (want 2), review %d (want 2)", w, r)
	}
	if !strings.Contains(out, "goal complete") || !strings.Contains(out, "goal started") {
		t.Fatalf("completion notices:\n%s", out)
	}

	// Leg 2: bare /goal shows the state, /goal clear removes it.
	s.send("/goal")
	if out := s.waitFor("goal complete · rounds 1/8", 10*time.Second); !strings.Contains(out, "objective: make the fixture green") {
		t.Fatalf("bare /goal:\n%s", out)
	}
	s.send("/goal clear")
	if out := s.waitFor("goal cleared", 10*time.Second); !strings.Contains(out, "goal cleared") {
		t.Fatalf("/goal clear:\n%s", out)
	}

	// Leg 3: pause lands while the reviewer is in flight; no further round.
	readyCh, release := p.blockReview(1)
	s.send("/goal third objective")
	select {
	case <-readyCh:
	case <-time.After(30 * time.Second):
		t.Fatalf("the reviewer was never called:\n%s", s.capture())
	}
	s.send("/goal pause")
	time.Sleep(300 * time.Millisecond)
	close(release)
	if out := s.waitFor("goal paused", 20*time.Second); !strings.Contains(out, "goal paused") || strings.Contains(out, "blocked") {
		t.Fatalf("pause mid-review (reason must be the pause, not the verdict):\n%s", out)
	}
	if w, r := p.counts(); w != 3 || r != 3 {
		t.Fatalf("pause mid-review: work %d review %d (want 3/3)", w, r)
	}

	// Leg 4: a killed process leaves an active goal; the next process pauses
	// it and makes zero provider calls until /goal resume.
	s.send("/goal clear")
	s.waitFor("goal cleared", 10*time.Second)
	readyCh, _ = p.blockReview(1)
	s.send("/goal fourth objective")
	select {
	case <-readyCh:
	case <-time.After(30 * time.Second):
		t.Fatalf("the second reviewer was never called:\n%s", s.capture())
	}
	sid := sessionID(t, data, work)
	s.kill()
	time.Sleep(300 * time.Millisecond)
	if status := goalStatusOnDisk(t, data, sid); status != "active" {
		t.Fatalf("killed session goal status %q, want active", status)
	}
	resumeScript := filepath.Join(base, "resume-tui.sh")
	writeScript(t, resumeScript, fmt.Sprintf(`#!/usr/bin/env bash
export XDG_DATA_HOME=%q XDG_CONFIG_HOME=%q XDG_STATE_HOME=%q
export GATE_KEY=%s MOCA_LOG=debug
exec %q --approve --resume %s
`, data, cfg, state, secret, *mocaBin, sid), 0o700)
	q := start(t, "goalgate-b", work, resumeScript)
	out = q.waitFor("goal paused", 30*time.Second)
	if !strings.Contains(out, "goal paused") || !strings.Contains(out, "restarted") {
		saveArtifacts(t, base, out, "", "")
		t.Fatalf("a resumed goal must pause with the restart reason:\n%s", out)
	}
	before, br := p.counts()
	time.Sleep(time.Second)
	if after, ar := p.counts(); after != before || ar != br {
		t.Fatalf("resume spent provider calls: %d/%d → %d/%d", before, br, after, ar)
	}
	q.send("/goal resume")
	out = q.waitFor("goal complete", 60*time.Second)
	if !strings.Contains(out, "goal complete") {
		saveArtifacts(t, base, q.capture(), "", "")
		t.Fatalf("explicit resume did not complete:\n%s", out)
	}

	// Leg 5: the key never appears in a capture, the log, or the state tree.
	captured := s.capture() + q.capture()
	logs, _ := filepath.Glob(filepath.Join(state, "moca", "logs", "*"))
	for _, l := range logs {
		if b, err := os.ReadFile(l); err == nil {
			captured += string(b)
		}
	}
	if strings.Contains(captured, secret) {
		t.Fatal("the API key leaked into captured output or the log")
	}
}

func writeScript(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// sessionID finds the one session file under data and returns its id8.
func sessionID(t *testing.T, data, work string) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(data, "moca", "sessions", "*.jsonl"))
	if len(files) == 0 {
		t.Fatalf("no session file under %s", data)
	}
	base := strings.TrimSuffix(filepath.Base(files[len(files)-1]), ".jsonl")
	return base[len(base)-8:]
}

// goalStatusOnDisk reads the newest goal snapshot of the session.
func goalStatusOnDisk(t *testing.T, data, id8 string) string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(data, "moca", "sessions", "*-"+id8+".jsonl"))
	if len(files) == 0 {
		t.Fatalf("no session %s", id8)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	status := ""
	for _, line := range strings.Split(string(b), "\n") {
		var e struct {
			Type string `json:"type"`
			Goal *struct {
				Status string `json:"status"`
			} `json:"goal"`
		}
		if json.Unmarshal([]byte(line), &e) == nil && e.Type == "goal" && e.Goal != nil {
			status = e.Goal.Status
		}
	}
	return status
}

func saveArtifacts(t *testing.T, base, pane, stdout, stderr string) {
	t.Helper()
	os.WriteFile(filepath.Join(base, "pane.txt"), []byte(pane), 0o600)
	os.WriteFile(filepath.Join(base, "stdout.txt"), []byte(stdout), 0o600)
	os.WriteFile(filepath.Join(base, "stderr.txt"), []byte(stderr), 0o600)
	t.Logf("artifacts kept under %s", base)
}
