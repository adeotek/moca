//go:build acpgate

// Package acpgate drives the built `moca acp` binary as an editor would:
// a child process on pipes, a scripted openai-completions provider, two
// processes sharing one data dir. Run through test/acpgate/run.sh.
package acpgate

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var mocaBin = flag.String("moca", "", "path to the built moca binary")

const secret = "acpgate-not-a-real-key-7f3a"

// provider serves scripted SSE turns; a turn equal to "BLOCK" waits until
// the request is cancelled (the cancel leg).
func provider(t *testing.T, turns ...string) *httptest.Server {
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		mu.Lock()
		turn := "data: {\"choices\":[{\"delta\":{\"content\":\"(exhausted)\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
		if len(turns) > 0 {
			turn, turns = turns[0], turns[1:]
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if turn == "BLOCK" {
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		io.WriteString(w, turn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func text(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

func shell(cmd string) string {
	args, _ := json.Marshal(fmt.Sprintf(`{"command":%q}`, cmd))
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_0\",\"function\":{\"name\":\"shell\",\"arguments\":%s}}]}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", args)
}

// proc is one `moca acp` child with a line reader over its stdout.
type proc struct {
	t        *testing.T
	cmd      *exec.Cmd
	in       io.WriteCloser
	lines    chan map[string]any
	stderr   strings.Builder
	raw      strings.Builder // every stdout byte, for the leak scan (read after readDone)
	wmu      sync.Mutex      // send runs from the test and from the cancel leg's goroutine
	next     int
	readDone chan struct{} // closed when stdout hit EOF
}

func start(t *testing.T, env []string) *proc {
	p := &proc{t: t, lines: make(chan map[string]any, 1024), readDone: make(chan struct{})}
	// --mode ask: the approval leg needs a command that ASKS, and v0.7.2 made
	// auto the product default (auto runs a confined `rm` without a prompt, so
	// the permission request this gate exists to see never arrived).
	p.cmd = exec.Command(*mocaBin, "acp", "--mode", "ask")
	p.cmd.Env = env
	p.cmd.Stderr = &p.stderr
	p.in, _ = p.cmd.StdinPipe()
	out, _ := p.cmd.StdoutPipe()
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			p.raw.WriteString(sc.Text() + "\n")
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m["jsonrpc"] != "2.0" {
				t.Errorf("stdout carried a non-JSON-RPC line: %q", sc.Text())
				continue
			}
			p.lines <- m
		}
		close(p.lines)
		close(p.readDone)
	}()
	return p
}

// exit closes stdin and waits for the process: stdout must reach EOF first
// (os/exec: Wait must not run before the pipe reads complete).
func (p *proc) exit() error {
	p.t.Helper()
	p.in.Close()
	select {
	case <-p.readDone:
	case <-time.After(10 * time.Second):
		p.t.Fatal("moca acp did not close stdout after stdin EOF")
	}
	return p.cmd.Wait()
}

func (p *proc) send(v map[string]any) {
	v["jsonrpc"] = "2.0"
	b, _ := json.Marshal(v)
	p.wmu.Lock()
	defer p.wmu.Unlock()
	p.in.Write(append(b, '\n'))
}

// call sends a request and collects everything up to its response.
// onPerm answers session/request_permission requests met on the way.
func (p *proc) call(method string, params any, onPerm func(map[string]any) map[string]any) (resp map[string]any, updates []map[string]any) {
	p.t.Helper()
	p.next++
	id := float64(p.next)
	p.send(map[string]any{"id": id, "method": method, "params": params})
	timeout := time.After(30 * time.Second)
	for {
		select {
		case m, ok := <-p.lines:
			if !ok {
				p.t.Fatalf("%s: stdout closed; stderr %q", method, p.stderr.String())
			}
			switch {
			case m["method"] == "session/update":
				updates = append(updates, m["params"].(map[string]any)["update"].(map[string]any))
			case m["method"] == "session/request_permission":
				p.send(map[string]any{"id": m["id"], "result": onPerm(m["params"].(map[string]any))})
			case m["id"] == id:
				return m, updates
			}
		case <-timeout:
			p.t.Fatalf("%s: no response in 30s", method)
		}
	}
}

func kinds(ups []map[string]any) []string {
	var out []string
	for _, u := range ups {
		out = append(out, u["sessionUpdate"].(string))
	}
	return out
}

func TestACPGate(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags acpgate ./test/acpgate -args -moca <binary>")
	}
	work, data, cfgDir, state := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(work, "victim.txt"), []byte("x"), 0o644)
	llm := provider(t, text("Hello from the gate."), shell("rm victim.txt"), text("Deleted."), "BLOCK")
	os.MkdirAll(filepath.Join(cfgDir, "moca"), 0o700)
	os.WriteFile(filepath.Join(cfgDir, "moca", "config.jsonc"), []byte(fmt.Sprintf(
		`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:ACPGATE_KEY","models":{"m":{"contextWindow":65536}}}}}`,
		llm.URL)), 0o600)
	env := append(os.Environ(), "XDG_DATA_HOME="+data, "XDG_CONFIG_HOME="+cfgDir, "XDG_STATE_HOME="+state, "ACPGATE_KEY="+secret,
		"MOCA_LOG=debug") // the most verbose log is the strictest leak scan
	allow := func(map[string]any) map[string]any {
		return map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow-once"}}
	}

	p := start(t, env)
	if r, _ := p.call("initialize", map[string]any{"protocolVersion": 1}, allow); r["result"].(map[string]any)["agentInfo"].(map[string]any)["name"] != "moca" {
		t.Fatalf("initialize: %v", r)
	}
	r, _ := p.call("session/new", map[string]any{"cwd": work, "mcpServers": []any{}}, allow)
	sid := r["result"].(map[string]any)["sessionId"].(string)

	// 1. A text turn streams and ends the turn.
	prompt := func(s string) map[string]any {
		return map[string]any{"sessionId": sid, "prompt": []any{map[string]any{"type": "text", "text": s}}}
	}
	r, ups := p.call("session/prompt", prompt("say hello"), allow)
	if r["result"].(map[string]any)["stopReason"] != "end_turn" || !strings.Contains(fmt.Sprint(ups), "Hello from the gate.") {
		t.Fatalf("text turn: %v %v", r, kinds(ups))
	}

	// 2. A shell call asks; allow-once runs it.
	perms := 0
	r, ups = p.call("session/prompt", prompt("delete victim.txt"), func(m map[string]any) map[string]any { perms++; return allow(m) })
	if perms != 1 || r["result"].(map[string]any)["stopReason"] != "end_turn" {
		t.Fatalf("approval turn: perms %d resp %v updates %v", perms, r, kinds(ups))
	}
	if _, err := os.Stat(filepath.Join(work, "victim.txt")); !os.IsNotExist(err) {
		t.Fatal("approved rm did not run")
	}

	// 3. Cancel mid-turn.
	done := make(chan map[string]any, 1)
	go func() { r, _ := p.call("session/prompt", prompt("take forever"), allow); done <- r }()
	time.Sleep(500 * time.Millisecond)
	p.send(map[string]any{"method": "session/cancel", "params": map[string]any{"sessionId": sid}})
	select {
	case r := <-done:
		if r["result"].(map[string]any)["stopReason"] != "cancelled" {
			t.Fatalf("cancel: %v", r)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("cancel did not end the turn")
	}

	// 4. stdin EOF → exit 0, quickly, with a silent stderr.
	if err := p.exit(); err != nil {
		t.Fatalf("exit: %v; stderr %q", err, p.stderr.String())
	}
	if p.stderr.Len() != 0 {
		t.Fatalf("stderr must stay empty: %q", p.stderr.String())
	}

	// 5. A second process reopens the session; replay comes before the response.
	q := start(t, env)
	q.call("initialize", map[string]any{"protocolVersion": 1}, allow)
	r, ups = q.call("session/load", map[string]any{"sessionId": sid, "cwd": work, "mcpServers": []any{}}, allow)
	if r["error"] != nil || !strings.Contains(fmt.Sprint(ups), "say hello") || !strings.Contains(fmt.Sprint(ups), "Hello from the gate.") {
		t.Fatalf("load: %v updates %v", r, kinds(ups))
	}
	// 5b. Close, then delete: the session leaves session/list.
	if r, _ := q.call("session/close", map[string]any{"sessionId": sid}, allow); r["error"] != nil {
		t.Fatalf("close: %v", r)
	}
	if r, _ := q.call("session/delete", map[string]any{"sessionId": sid}, allow); r["error"] != nil {
		t.Fatalf("delete: %v", r)
	}
	r, _ = q.call("session/list", map[string]any{"cwd": work}, allow)
	if n := len(r["result"].(map[string]any)["sessions"].([]any)); n != 0 {
		t.Fatalf("deleted session still listed (%d)", n)
	}
	if err := q.exit(); err != nil {
		t.Fatalf("second process exit: %v", err)
	}

	// 6. Leak scan: the key never appears in stdout, stderr or the log.
	logs, _ := filepath.Glob(filepath.Join(state, "moca", "logs", "*"))
	if len(logs) == 0 {
		t.Fatal("no log file to scan")
	}
	captured := p.raw.String() + p.stderr.String() + q.raw.String() + q.stderr.String()
	for _, l := range logs {
		b, _ := os.ReadFile(l)
		captured += string(b)
	}
	if strings.Contains(captured, secret) {
		t.Fatal("the API key leaked into captured output")
	}
}
