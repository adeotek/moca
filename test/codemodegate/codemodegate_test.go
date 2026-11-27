//go:build codemodegate

// Package codemodegate drives the built moca binary against a scripted provider
// and checks the nested-JavaScript tool end to end: one outer result per script,
// the capability allowlist, zero approvals in any mode, the budget limits, slot
// reuse after a timeout, the resumed refusal-only tool, and no key in any output.
// Hermetic: no key, no network beyond the local test server.
package codemodegate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	// script and turns are touched by the test goroutine and by the provider
	// handler, so they go through mu: without it -race reports the harness
	// itself (a test that sets its script after newHarness) as a data race,
	// which is what the gate's own -race run caught.
	mu     sync.Mutex
	script []string // tool calls, one per work turn; a text turn ends the run
	turns  int
}

// setScript replaces the tool-call script and rewinds the turn counter.
func (h *harness) setScript(lines ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.script = lines
	h.turns = 0
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
		h.mu.Lock()
		i, script := h.turns, h.script
		h.turns++
		h.mu.Unlock()
		if i < len(script) {
			sseTool(w, fmt.Sprintf("c%d", i), "codemode", script[i])
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

// smallBudget pins the inspection ceiling so the cap leg does not depend on the
// product default.
const smallBudget = `"codemode":{"enabled":true,"inspectAttempts":50}`

// One script does the work of many calls: read, search and ls in a single outer
// result, with the tool present as the ninth entry.
func TestWorkspaceScript(t *testing.T) {
	h := newHarness(t, `{"code":"const a = await tools.read({path: 'a.txt'}); const s = await tools.search({pattern: 'needle'}); const l = await tools.ls({path: '.'}); text(a.lines.map(x => x.text).join('|')); text(s.matches.length + ' hits'); return {entries: l.entries.length}"}`)
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

// A script that names the paths itself and loops over them: one call, one
// result. This is the shape rounds 1-3 of the pilot could not get out of the
// model with a prompt, so the primitive it depends on is pinned here.
func TestGlobThenReadInOneProgram(t *testing.T) {
	h := newHarness(t)
	h.cfg(enabled)
	for _, svc := range []string{"api", "jobs", "ledger"} {
		h.file("services/"+svc+"/service.yaml", "cpu: 500m\n")
	}
	h.file("services/README.md", "not a manifest\n")
	h.setScript(`{"code":"const g = await tools.glob({pattern: 'services/*/service.yaml'}); const out = []; for (const p of g.paths) { const r = await tools.read({path: p}); out.push(p + ':' + r.lines[0].text); } return out;"}`)
	h.run("-p", "Read every service manifest and report them.")

	res := h.lastResult()
	for _, want := range []string{"services/api/service.yaml:cpu: 500m", "services/jobs/service.yaml", "services/ledger/service.yaml"} {
		if !strings.Contains(res, want) {
			t.Fatalf("the one-program result lost %q: %q", want, res)
		}
	}
	if strings.Contains(res, "README.md") {
		t.Fatalf("the glob matched a non-manifest: %q", res)
	}
}

func TestCapabilityAllowlist(t *testing.T) {
	h := newHarness(t, `{"code":"await tools.write({path: 'pwned.txt', content: 'x'})"}`)
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
			h := newHarness(t, `{"code":"const r = await tools.read({path: 'a.txt'}); return r.lines.length"}`)
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

// The configured attempt cap latches at the next inspection and the ledger says
// so. Pinned small (50) rather than left to the default: the default is a
// product number, and this leg is about the latch.
func TestAttemptCap(t *testing.T) {
	h := newHarness(t, `{"code":"for (let i = 0; i < 51; i++) { await tools.read({path: 'f' + i + '.txt'}); }"}`)
	h.cfg(smallBudget)
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

// Grant scope (SPECS §10.6, P0): a script reaches the jail's directory read
// roots on the transport surface only, and a file is never a grant. The config
// file is the regression — it used to be granted as its parent directory, which
// admitted auth.json beside it, on additive too, because grants were applied
// whenever a budget was set.
func TestGrantScope(t *testing.T) {
	const transportCfg = `"codemode":{"enabled":true,"exposure":"transport"}`
	const additiveCfg = `"codemode":{"enabled":true}`

	// fixture writes the prompts probe (a directory read root) and a credential
	// store beside the config file (not a grant, and not reachable either way).
	fixture := func(t *testing.T, mode string) (h *harness, probe, auth string) {
		t.Helper()
		h = newHarness(t)
		h.cfg(mode)
		dir := filepath.Join(h.cfgDir, "moca")
		prompts := filepath.Join(dir, "prompts")
		if err := os.MkdirAll(prompts, 0o700); err != nil {
			t.Fatal(err)
		}
		probe = filepath.Join(prompts, "probe.md")
		if err := os.WriteFile(probe, []byte("PROMPT-CANARY\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		auth = filepath.Join(dir, "auth.json")
		if err := os.WriteFile(auth, []byte(`{"apiKeys":{"fake":"sk-gate-must-not-leave"}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		return h, probe, auth
	}
	read := func(path string) string {
		// Two layers: the JS source needs a quoted path, and the script itself
		// is a JSON string argument to the tool.
		src := fmt.Sprintf("const r = await tools.read({path: %s}); return r.lines[0].text", strconv.Quote(path))
		b, err := json.Marshal(map[string]string{"code": src})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("transport reads a directory grant", func(t *testing.T) {
		h, probe, _ := fixture(t, transportCfg)
		h.setScript(read(probe))
		h.run("-p", "read the prompt reference")
		if res := h.lastResult(); !strings.Contains(res, "PROMPT-CANARY") {
			t.Fatalf("a granted prompts read did not reach the file: %q", res)
		}
	})

	t.Run("transport denies a file grant and its sibling", func(t *testing.T) {
		h, _, auth := fixture(t, transportCfg)
		cfgFile := filepath.Join(h.cfgDir, "moca", "config.jsonc")
		for _, p := range []string{auth, cfgFile} {
			h.setScript(read(p))
			h.run("-p", "read a path outside the workdir")
			res := h.lastResult()
			if strings.Contains(res, "sk-gate-must-not-leave") || strings.Contains(res, "PROMPT-CANARY") {
				t.Fatalf("a non-granted read returned content: %q", res)
			}
			if !strings.Contains(res, "not inside a granted directory") {
				t.Fatalf("refusal did not name the reason: %q", res)
			}
		}
	})

	t.Run("additive has no external roots", func(t *testing.T) {
		h, probe, _ := fixture(t, additiveCfg)
		h.setScript(read(probe))
		h.run("-p", "read the prompt reference")
		res := h.lastResult()
		if strings.Contains(res, "PROMPT-CANARY") {
			t.Fatalf("additive reached a read root outside the workdir: %q", res)
		}
		if !strings.Contains(res, "not inside a granted directory") {
			t.Fatalf("refusal did not name the reason: %q", res)
		}
	})
}

// A timed-out script must not poison the slot: the next call in the same run
// still executes and returns its value.
func TestTimeoutThenReuse(t *testing.T) {
	h := newHarness(t,
		`{"code":"while (true) {}","timeoutSeconds":1}`,
		`{"code":"return 1 + 1"}`,
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
	h := newHarness(t, `{"code":"const r = await tools.read({path: 'a.txt'}); return r.lines.length"}`)
	h.cfg(enabled)
	h.file("a.txt", "alpha\n")
	h.run("-p", "first run", "--approve")

	// Second run of the same workdir: codemode is off now (the config loses the
	// block), and the run resumes the session whose header promised the tool.
	h.setScript(`{"code":"return 1"}`)
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
	h := newHarness(t, `{"code":"const r = await tools.read({path: 'a.txt'}); return r.lines.length"}`)
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

// TestChangesetConsent is the writable surface's consent leg (§4.10): a real
// binary stages one edit and either applies it or refuses, driven by the
// effective mode alone. Under -p there is no one to ask, so only yolo applies;
// ask and plan discard with the remedy named and the file untouched. The tool
// result is paired in both directions — a refused batch is still a result the
// model sees, never a dropped call.
func TestChangesetConsent(t *testing.T) {
	const editsCfg = `"codemode":{"enabled":true,"exposure":"workspace","workspaceEdits":true}`
	script := `{"code":"const r = await tools.stageEdit({path: 'a.txt', edits: [{old: 'alpha', new: 'ALPHA'}]}); return r"}`

	cases := []struct {
		name, mode string
		applied    bool
	}{
		{"yolo-applies", "yolo", true},
		{"ask-refuses", "ask", false},
		{"plan-refuses", "plan", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, script)
			h.cfg(editsCfg + `,"permissions":{"defaultMode":"` + c.mode + `"}`)
			h.file("a.txt", "alpha\n")
			out := h.run("-p", "rotate the marker")
			got, err := os.ReadFile(filepath.Join(h.work, "a.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if c.applied {
				if string(got) != "ALPHA\n" {
					t.Fatalf("yolo did not apply: disk=%q\n%s", got, out)
				}
				if res := h.lastResult(); !strings.Contains(res, "changeset applied") {
					t.Fatalf("no applied receipt: %q", res)
				}
			} else {
				if string(got) != "alpha\n" {
					t.Fatalf("%s wrote the file: %q", c.mode, got)
				}
				res := h.lastResult()
				if !strings.Contains(res, "discarded") {
					t.Fatalf("no discard receipt: %q", res)
				}
				if c.mode != "plan" && !strings.Contains(res, "-p") && !strings.Contains(res, "not trusted") {
					t.Fatalf("the refusal names no remedy: %q", res)
				}
			}
			// Every leg still declares nine tools: the surface adds no tool.
			if names := h.toolNames(); len(names) != 9 || names[8] != "codemode" {
				t.Fatalf("tools = %v", names)
			}
		})
	}
}

// The staging result carries the image the host will write, so a program can
// check its own work before apply instead of verifying through a shell in a
// later turn. This is the only leg that proves the returned content and digest
// describe the bytes that land on disk, not the edit engine's intent.
func TestStagedContentMatchesTheAppliedBytes(t *testing.T) {
	const editsCfg = `"codemode":{"enabled":true,"exposure":"workspace","workspaceEdits":true}`
	script := `{"code":"const r = await tools.stageEdit({path: 'a.txt', edits: [{old: 'alpha', new: 'ALPHA'}]}); return {sha256: r.sha256, bytes: r.bytes, content: r.content, truncated: r.truncated}"}`
	h := newHarness(t, script)
	h.cfg(editsCfg + `,"permissions":{"defaultMode":"yolo"}`)
	h.file("a.txt", "alpha\n")
	out := h.run("-p", "rotate the marker")

	disk, err := os.ReadFile(filepath.Join(h.work, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(disk) != "ALPHA\n" {
		t.Fatalf("the batch did not apply: disk=%q\n%s", disk, out)
	}
	want := "ALPHA\n"
	sum := sha256.Sum256(disk)

	res := h.lastResult()
	if !strings.Contains(res, hex.EncodeToString(sum[:])) {
		t.Fatalf("the digest the program saw does not match the applied bytes (%s): %q\n%s",
			hex.EncodeToString(sum[:]), res, out)
	}
	if !strings.Contains(res, `"bytes":`+strconv.Itoa(len(want))) {
		t.Fatalf("the reported size is not the applied size (%d B): %q", len(want), res)
	}
	// content must be the post image, never the pre image.
	if !strings.Contains(res, "ALPHA") || strings.Contains(res, `"content":"alpha`) {
		t.Fatalf("content is not the post-edit image: %q", res)
	}
	if !strings.Contains(res, `"truncated":false`) {
		t.Fatalf("a small file was reported truncated: %q", res)
	}
}
