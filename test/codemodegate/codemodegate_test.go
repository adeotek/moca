//go:build codemodegate

// Package codemodegate drives the built moca binary against a scripted provider
// and checks the nested-JavaScript tool end to end: one outer result per script,
// the capability allowlist, zero approvals in any mode, the budget limits, slot
// reuse after a timeout, the resumed refusal-only tool, and no key in any output.
// Hermetic: no key, no network beyond the local test server.
package codemodegate

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
	"strings"
	"sync"
	"testing"
)

var mocaBin = flag.String("moca", "", "path to the built moca binary")

type recorder struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (r *recorder) add(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodies = append(r.bodies, append([]byte(nil), b...))
}

func (r *recorder) all() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]byte(nil), r.bodies...)
}

func sseText(w http.ResponseWriter, text string) {
	b, _ := json.Marshal(text)
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
}

func sseTool(w http.ResponseWriter, id, name, args string) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":%q,\"function\":{\"name\":%q,\"arguments\":%q}}]}}]}\n\n"+
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", id, name, args)
}

// harness is one moca run: its own config/data dirs, workdir and script.
type harness struct {
	t      *testing.T
	srv    *httptest.Server
	rec    *recorder
	cfgDir string
	data   string
	work   string
	script []string // tool calls, one per work turn; a text turn ends the run
	turns  int
}

func newHarness(t *testing.T, script ...string) *harness {
	t.Helper()
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags codemodegate ./test/codemodegate -args -moca <binary>")
	}
	h := &harness{t: t, rec: &recorder{}, cfgDir: t.TempDir(), data: t.TempDir(), work: t.TempDir(), script: script}
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"m","limit":{"context":200000,"output":8192}}]}`)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			http.Error(w, "body", http.StatusBadRequest)
			return
		}
		h.rec.add(body)
		var req struct {
			Tools []json.RawMessage `json:"tools"`
		}
		json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(req.Tools) == 0 { // a summarizer or nudge call
			sseText(w, "ok")
			return
		}
		i := h.turns
		h.turns++
		if i < len(h.script) {
			sseTool(w, fmt.Sprintf("c%d", i), "codemode", h.script[i])
			return
		}
		sseText(w, "done")
	}))
	t.Cleanup(h.srv.Close)
	return h
}

// cfg writes the user config; extra is spliced in as top-level JSON members.
func (h *harness) cfg(extra string) {
	h.t.Helper()
	dir := filepath.Join(h.cfgDir, "moca")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		h.t.Fatal(err)
	}
	providers := fmt.Sprintf(`"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key",`+
		`"apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":200000,"maxOutputTokens":8192}}}}`, h.srv.URL)
	parts := []string{`"model":"fake/m"`, providers}
	if extra != "" {
		parts = append(parts, extra)
	}
	cfg := "{" + strings.Join(parts, ",") + "}"
	if err := os.WriteFile(filepath.Join(dir, "config.jsonc"), []byte(cfg), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) file(rel, content string) {
	h.t.Helper()
	p := filepath.Join(h.work, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) run(args ...string) string {
	h.t.Helper()
	cmd := exec.Command(*mocaBin, args...)
	cmd.Dir = h.work
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+h.cfgDir,
		"XDG_DATA_HOME="+h.data,
		"MOCA_T_KEY=gate-key-must-not-leak",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		h.t.Fatalf("moca %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// results returns every tool-result message the model was shown, in order.
func (h *harness) results() []string {
	h.t.Helper()
	var out []string
	for _, body := range h.rec.all() {
		var req struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if json.Unmarshal(body, &req) != nil {
			continue
		}
		for _, m := range req.Messages {
			if m.Role != "tool" {
				continue
			}
			var s string
			if json.Unmarshal(m.Content, &s) == nil {
				out = append(out, s)
			}
		}
	}
	return out
}

func (h *harness) lastResult() string {
	h.t.Helper()
	r := h.results()
	if len(r) == 0 {
		h.t.Fatal("no tool result reached the model")
	}
	return r[len(r)-1]
}

func (h *harness) toolNames() []string {
	h.t.Helper()
	bodies := h.rec.all()
	if len(bodies) == 0 {
		h.t.Fatal("no request recorded")
	}
	var req struct {
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(bodies[0], &req); err != nil {
		h.t.Fatal(err)
	}
	names := make([]string, len(req.Tools))
	for i, t := range req.Tools {
		names[i] = t.Function.Name
	}
	return names
}

const enabled = `"codemode":{"enabled":true}`

// One script does the work of many calls: read, search and ls in a single outer
// result, with the tool present as the ninth entry.
func TestWorkspaceScript(t *testing.T) {
	h := newHarness(t, `{"code":"var a = tools.read({path: 'a.txt'}); var s = tools.search({pattern: 'needle'}); var l = tools.ls({path: '.'}); text(a.lines.map(function(x){return x.text}).join('|')); text(s.matches.length + ' hits'); ({entries: l.entries.length})"}`)
	h.cfg(enabled)
	h.file("a.txt", "alpha\n")
	h.file("b.txt", "beta needle\n")
	h.file("sub/c.txt", "gamma\n")
	h.run("-p", "read the workspace", "--approve")

	names := h.toolNames()
	if len(names) != 9 || names[8] != "codemode" {
		t.Fatalf("tools = %v, want nine with codemode last", names)
	}
	res := h.lastResult()
	if !strings.Contains(res, "alpha") {
		t.Fatalf("read output lost: %q", res)
	}
	if !strings.Contains(res, "1 hits") {
		t.Fatalf("search output lost: %q", res)
	}
	if !strings.Contains(res, `"entries":3`) {
		t.Fatalf("final value lost (want 3 entries: a.txt, b.txt, sub): %q", res)
	}
}

// The capability table is the boundary: write and shell do not exist inside a
// script, the refusal says so, and nothing lands on disk.
func TestCapabilityAllowlist(t *testing.T) {
	h := newHarness(t, `{"code":"tools.write({path: 'pwned.txt', content: 'x'})"}`)
	h.cfg(enabled)
	h.run("-p", "try to write", "--approve")

	res := h.lastResult()
	if !strings.Contains(res, "failed: script") || !strings.Contains(res, "unknown tools.write") {
		t.Fatalf("refusal not reported: %q", res)
	}
	if _, err := os.Stat(filepath.Join(h.work, "pwned.txt")); err == nil {
		t.Fatal("a script wrote a file through a capability that does not exist")
	}
}

// No mode asks: the workspace primitives never need approval, and codemode adds
// no question path of its own.
func TestZeroApprovalsInEveryMode(t *testing.T) {
	for _, mode := range []string{"default", "auto", "yolo", "plan"} {
		t.Run(mode, func(t *testing.T) {
			h := newHarness(t, `{"code":"tools.read({path: 'a.txt'}).value.lines.length"}`)
			h.cfg(enabled)
			h.file("a.txt", "alpha\n")
			out := h.run("-p", "read", "--approve", "--mode", mode)
			if strings.Contains(strings.ToLower(out), "allow?") || strings.Contains(out, "[y/n]") {
				t.Fatalf("mode %s asked for approval:\n%s", mode, out)
			}
			if !strings.Contains(h.lastResult(), "1") {
				t.Fatalf("mode %s: script did not run: %q", mode, h.lastResult())
			}
		})
	}
}

// The 50-attempt cap latches at the 51st inspection and the ledger says so.
func TestAttemptCap(t *testing.T) {
	h := newHarness(t, `{"code":"for (var i = 0; i < 51; i++) { tools.read({path: 'f' + i + '.txt'}); }"}`)
	h.cfg(enabled)
	for i := 0; i < 51; i++ {
		h.file(fmt.Sprintf("f%d.txt", i), "line\n")
	}
	h.run("-p", "read every file", "--approve")
	res := h.lastResult()
	if !strings.Contains(res, "failed: limit") {
		t.Fatalf("the attempt cap was not reported: %q", res)
	}
	if !strings.Contains(res, "[codemode: 51 calls") {
		t.Fatalf("ledger is missing the call count: %q", res)
	}
}

// A timed-out script must not poison the slot: the next call in the same run
// still executes and returns its value.
func TestTimeoutThenReuse(t *testing.T) {
	h := newHarness(t,
		`{"code":"while (true) {}","timeoutSeconds":1}`,
		`{"code":"1 + 1"}`,
	)
	h.cfg(enabled)
	h.run("-p", "time out then work", "--approve")
	// Messages accumulate in every request, so the same result is shown
	// repeatedly; the last one is the second call's.
	all := h.results()
	if len(all) == 0 {
		t.Fatal("no tool result reached the model")
	}
	if !strings.Contains(all[0], "failed: timeout") {
		t.Fatalf("first call did not time out: %q", all[0])
	}
	last := all[len(all)-1]
	if strings.Contains(last, "failed") || !strings.Contains(last, "2") {
		t.Fatalf("the slot did not recover: %q", last)
	}
}

// A resumed session keeps the tool its frozen prompt promised: with the switch
// off now it is the refusal-only factory, not a missing tool.
func TestResumeKeepsRefusalOnly(t *testing.T) {
	h := newHarness(t, `{"code":"tools.read({path: 'a.txt'}).value.lines.length"}`)
	h.cfg(enabled)
	h.file("a.txt", "alpha\n")
	h.run("-p", "first run", "--approve")

	// Second run of the same workdir: codemode is off now (the config loses the
	// block), and the run resumes the session whose header promised the tool.
	h.script = []string{`{"code":"1"}`}
	h.turns = 0
	h.rec = &recorder{}
	h.cfg("")
	h.run("-p", "again", "--approve", "--resume", "last")

	names := h.toolNames()
	if len(names) != 9 || names[8] != "codemode" {
		t.Fatalf("resumed tools = %v, want the promised ninth", names)
	}
	res := h.lastResult()
	if !strings.Contains(res, "codemode is not enabled in this session") {
		t.Fatalf("resumed call was not refused: %q", res)
	}
}

// The provider key must not reach stdout, stderr or any log file.
func TestNoKeyLeak(t *testing.T) {
	h := newHarness(t, `{"code":"tools.read({path: 'a.txt'}).value.lines.length"}`)
	h.cfg(enabled)
	h.file("a.txt", "alpha\n")
	out := h.run("-p", "read", "--approve")
	if strings.Contains(out, "gate-key-must-not-leak") {
		t.Fatalf("the provider key leaked into the output:\n%s", out)
	}
	err := filepath.Walk(h.data, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), "gate-key-must-not-leak") {
			t.Fatalf("the provider key leaked into %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
