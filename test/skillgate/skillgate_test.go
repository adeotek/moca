//go:build skillgate

// Package skillgate drives the built moca binary against a scripted
// openai-completions provider and asserts the self-managed skills contract
// end to end (SPECS §12). Run through test/skillgate/run.sh.
package skillgate

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

const providerKey = "skillgate-not-a-real-key-4c1d"

// --- scripted provider -----------------------------------------------------

type provider struct {
	mu    sync.Mutex
	turns []string
	url   string
}

func newProvider(t *testing.T, turns ...string) *provider {
	p := &provider{turns: turns}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		p.mu.Lock()
		turn := textTurn("(script exhausted)")
		if len(p.turns) > 0 {
			turn, p.turns = p.turns[0], p.turns[1:]
		}
		p.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, turn)
	}))
	t.Cleanup(srv.Close)
	p.url = srv.URL
	return p
}

func textTurn(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

// toolTurn scripts calls as name+JSON arguments, built through json.Marshal so
// a control byte in the content can never produce invalid JSON.
func toolTurn(calls ...[2]string) string {
	var sb strings.Builder
	for i, c := range calls {
		args, _ := json.Marshal(c[1])
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"call_%d\",\"function\":{\"name\":%q,\"arguments\":%s}}]}}]}\n\n", i, i, c[0], args)
	}
	sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
	return sb.String()
}

func writeCall(path, content string) [2]string {
	b, _ := json.Marshal(map[string]string{"path": path, "content": content})
	return [2]string{"write", string(b)}
}

func editCall(path, old, new string) [2]string {
	b, _ := json.Marshal(map[string]string{"path": path, "old_string": old, "new_string": new})
	return [2]string{"edit", string(b)}
}

func readCall(path string) [2]string {
	b, _ := json.Marshal(map[string]string{"path": path})
	return [2]string{"read", string(b)}
}

func shellCall(cmd string) [2]string {
	b, _ := json.Marshal(map[string]string{"command": cmd})
	return [2]string{"shell", string(b)}
}

// --- fixtures --------------------------------------------------------------

// gate is one hermetic child world: a workdir, the XDG dirs, a provider.
type gate struct {
	t                               *testing.T
	work, data, cfgDir, state, home string
	provider                        *provider
}

func newGate(t *testing.T, turns ...string) *gate {
	t.Helper()
	g := &gate{t: t, work: t.TempDir(), data: t.TempDir(), cfgDir: t.TempDir(), state: t.TempDir(), home: t.TempDir()}
	g.provider = newProvider(t, turns...)
	if err := os.WriteFile(filepath.Join(g.work, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, g.work, "init", "-q", "-b", "main")
	git(t, g.work, "config", "user.name", "skillgate")
	git(t, g.work, "config", "user.email", "gate@example.com")
	git(t, g.work, "add", "-A")
	git(t, g.work, "commit", "-qm", "fixture")
	return g
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// user writes the user config; extraTop is spliced into the root object.
func (g *gate) user(extraTop string) {
	g.t.Helper()
	if extraTop != "" {
		extraTop = "," + extraTop
	}
	body := fmt.Sprintf(`{"model":"fake/m"%s,
	  "providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:SKILLGATE_KEY",
	    "models":{"m":{"contextWindow":65536}}}},
	  "shell":{"allow":["go","git","python3","tee","cp","mv","sed","rm","truncate","install","ln","printf","echo"]}}`, extraTop, g.provider.url)
	if err := os.MkdirAll(filepath.Join(g.cfgDir, "moca"), 0o755); err != nil {
		g.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.cfgDir, "moca", "config.jsonc"), []byte(body), 0o600); err != nil {
		g.t.Fatal(err)
	}
}

func (g *gate) project(body string) {
	g.t.Helper()
	if err := os.MkdirAll(filepath.Join(g.work, ".moca"), 0o755); err != nil {
		g.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g.work, ".moca", "config.jsonc"), []byte(body), 0o644); err != nil {
		g.t.Fatal(err)
	}
}

func (g *gate) childEnv(extra ...string) []string {
	return append(append(os.Environ(),
		"XDG_DATA_HOME="+g.data, "XDG_CONFIG_HOME="+g.cfgDir, "XDG_STATE_HOME="+g.state,
		"HOME="+g.home, "SKILLGATE_KEY="+providerKey), extra...)
}

// run executes `moca <args...> -p <prompt>`; flags go before -p.
func (g *gate) run(prompt string, args ...string) (stdout, stderr string, code int) {
	g.t.Helper()
	return g.runIn(g.work, prompt, args...)
}

func (g *gate) runIn(dir, prompt string, args ...string) (stdout, stderr string, code int) {
	g.t.Helper()
	cmdArgs := append(append([]string{}, args...), "-p", prompt)
	c := exec.Command(*mocaBin, cmdArgs...)
	c.Dir = dir
	c.Env = g.childEnv()
	var so, se strings.Builder
	c.Stdout, c.Stderr = &so, &se
	_ = c.Run()
	return so.String(), se.String(), c.ProcessState.ExitCode()
}

// prompt returns the stored system prompt of the newest session.
func (g *gate) prompt() string {
	g.t.Helper()
	paths, _ := filepath.Glob(filepath.Join(g.data, "moca", "sessions", "*.jsonl"))
	if len(paths) == 0 {
		return ""
	}
	newest := paths[0]
	var newestT time.Time
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err == nil && fi.ModTime().After(newestT) {
			newest, newestT = p, fi.ModTime()
		}
	}
	data, _ := os.ReadFile(newest)
	for _, line := range strings.Split(string(data), "\n") {
		var e struct {
			Type    string `json:"type"`
			Session *struct {
				SystemPrompt      string `json:"systemPrompt"`
				SelfManagedSkills bool   `json:"selfManagedSkills"`
			} `json:"session"`
		}
		if json.Unmarshal([]byte(line), &e) == nil && e.Type == "session" && e.Session != nil {
			return e.Session.SystemPrompt
		}
	}
	return ""
}

func (g *gate) headerHasBlock() bool {
	g.t.Helper()
	paths, _ := filepath.Glob(filepath.Join(g.data, "moca", "sessions", "*.jsonl"))
	for _, p := range paths {
		data, _ := os.ReadFile(p)
		if strings.Contains(string(data), `"selfManagedSkills":true`) {
			return true
		}
	}
	return false
}

func (g *gate) read(rel string) string {
	g.t.Helper()
	b, err := os.ReadFile(filepath.Join(g.work, rel))
	if err != nil {
		return ""
	}
	return string(b)
}

func (g *gate) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(g.work, rel))
	return err == nil
}

func (g *gate) status() string { return git(g.t, g.work, "status", "--porcelain", "-uall") }

// mask replaces every scratch path with a stable placeholder, so two runs in
// different dirs compare byte for byte.
func (g *gate) mask(s string) string {
	r := strings.NewReplacer(g.work, "<WORK>", g.data, "<DATA>", g.cfgDir, "<CFG>", g.state, "<STATE>", g.home, "<HOME>")
	return r.Replace(s)
}

func today() string       { return time.Now().Format("2006-01-02") }
func dayAgo(n int) string { return time.Now().AddDate(0, 0, -n).Format("2006-01-02") }

func autoSkillFile(name, desc, body string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\nauto: true\n---\n" + body
}

func rule(when, text, prov string) string { return "- **" + when + "**: " + text + " (" + prov + ")\n" }

func seedSkill(t *testing.T, work, name, content string) {
	t.Helper()
	dir := filepath.Join(work, ".moca", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- ACP child -------------------------------------------------------------

type proc struct {
	t        *testing.T
	cmd      *exec.Cmd
	in       io.WriteCloser
	lines    chan map[string]any
	stderr   strings.Builder
	readDone chan struct{}
	next     int
	wmu      sync.Mutex
}

func startACP(t *testing.T, env []string, dir string) *proc {
	t.Helper()
	p := &proc{t: t, lines: make(chan map[string]any, 1024), readDone: make(chan struct{})}
	p.cmd = exec.Command(*mocaBin, "acp")
	p.cmd.Dir = dir
	p.cmd.Env = env
	p.cmd.Stderr = &p.stderr
	p.in, _ = p.cmd.StdinPipe()
	out, _ := p.cmd.StdoutPipe()
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer close(p.lines)
		defer close(p.readDone)
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				p.lines <- m
			}
		}
	}()
	return p
}

func (p *proc) send(v map[string]any) {
	v["jsonrpc"] = "2.0"
	b, _ := json.Marshal(v)
	p.wmu.Lock()
	defer p.wmu.Unlock()
	p.in.Write(append(b, '\n'))
}

func (p *proc) call(method string, params any) (map[string]any, []map[string]any) {
	p.t.Helper()
	p.next++
	id := float64(p.next)
	p.send(map[string]any{"id": id, "method": method, "params": params})
	timeout := time.After(60 * time.Second)
	var updates []map[string]any
	for {
		select {
		case m, ok := <-p.lines:
			if !ok {
				p.t.Fatalf("%s: stdout closed; stderr %q", method, p.stderr.String())
			}
			if m["method"] == "session/update" {
				updates = append(updates, m["params"].(map[string]any)["update"].(map[string]any))
				continue
			}
			if m["id"] == id {
				return m, updates
			}
		case <-timeout:
			p.t.Fatalf("%s: no response in 60s", method)
		}
	}
}

func (p *proc) close() {
	p.in.Close()
	select {
	case <-p.readDone:
	case <-time.After(10 * time.Second):
	}
	p.cmd.Wait()
}

func (p *proc) promptText(updates []map[string]any) string {
	var sb strings.Builder
	for _, u := range updates {
		if c, ok := u["content"].(map[string]any); ok {
			if t, ok := c["text"].(string); ok {
				sb.WriteString(t)
			}
		}
	}
	return sb.String()
}

// drain collects updates that arrive after a response (the deferred
// commands/notice flush).
func (p *proc) drain(d time.Duration) []map[string]any {
	var out []map[string]any
	deadline := time.After(d)
	for {
		select {
		case m, ok := <-p.lines:
			if !ok {
				return out
			}
			if m["method"] == "session/update" {
				out = append(out, m["params"].(map[string]any)["update"].(map[string]any))
			}
		case <-deadline:
			return out
		}
	}
}

func commandNames(updates []map[string]any) []string {
	var out []string
	for _, u := range updates {
		if u["sessionUpdate"] != "available_commands_update" {
			continue
		}
		for _, c := range u["availableCommands"].([]any) {
			out = append(out, c.(map[string]any)["name"].(string))
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// --- legs ------------------------------------------------------------------

func TestSkillGate(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags skillgate ./test/skillgate -args -moca <binary>")
	}
	t.Run("1_off_is_inert", legOffIsInert)
	t.Run("2_on_conforming_write", legOnConformingWrite)
	t.Run("3_caps_and_ownership", legCapsAndOwnership)
	t.Run("4_secret_scan", legSecretScan)
	t.Run("5_per_run_cap", legPerRunCap)
	t.Run("6_switch_table", legSwitchTable)
	t.Run("7_hygiene", legHygiene)
	t.Run("8_no_files_we_did_not_ask_for", legNoExtraFiles)
	t.Run("9_p_without_approve_is_inert", legPWithoutApprove)
	t.Run("10_plan_mode_never_learns", legPlanMode)
	t.Run("11_symlinks", legSymlinks)
	t.Run("12_acp_parity", legACPParity)
	t.Run("13_shell_and_interpreters", legShellAndInterpreters)
	t.Run("14_case_variant", legCaseVariant)
	t.Run("15_caps_check_changes", legCapsCheckChanges)
	t.Run("16_naming_and_pins", legNamingAndPins)
}

func legOffIsInert(t *testing.T) {
	// No user opt-in, --approve: ordinary workdir behaviour, no block, no hint.
	g := newGate(t, toolTurn(writeCall(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n")), textTurn("done"))
	g.user("")
	out, errOut, code := g.run("record what you learned", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !g.exists(".moca/skills/hand/SKILL.md") {
		t.Fatalf("off: the write must land unvalidated (stdout %q stderr %q)", out, errOut)
	}
	if strings.Contains(g.prompt(), "# Project skills you maintain") {
		t.Fatal("off: no block")
	}
	if g.headerHasBlock() {
		t.Fatal("off: the header must not record a block")
	}
	if strings.Contains(out+errOut, "[hint: something durable") {
		t.Fatal("off: no hint")
	}
	// Two different "off" paths — no opt-in, and opted in but opted out by a
	// trusted project file — produce the same prompt bytes.
	a := newGate(t, textTurn("one"))
	a.user("")
	a.run("hello", "--approve")
	b := newGate(t, textTurn("one"))
	b.user(`"skills":{"selfManaged":true}`)
	b.project(`{"skills":{"selfManaged":false}}`)
	git(t, b.work, "add", "-A")
	git(t, b.work, "commit", "-qm", "project config")
	b.run("hello", "--approve")
	pa, pb := a.mask(a.prompt()), b.mask(b.prompt())
	if pa == "" || pa != pb {
		t.Fatalf("the two off paths must be byte-identical:\n--- A ---\n%s\n--- B ---\n%s", pa, pb)
	}
	t.Logf("leg 1: off is inert; both off paths prompt-identical")
}

func legOnConformingWrite(t *testing.T) {
	g := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-testing/SKILL.md", autoSkillFile("moca-testing", "How the suite runs here.", rule("Running the suite", "`go test ./... -count=1`", "×1, "+today())))),
		textTurn("recorded"),
		textTurn("nothing new"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	if _, errOut, code := g.run("record the rule", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	body := g.read(".moca/skills/moca-testing/SKILL.md")
	if !strings.Contains(body, "go test ./... -count=1") {
		t.Fatalf("the conforming skill must land: %q", body)
	}
	p := g.prompt()
	if !strings.Contains(p, "# Project skills you maintain") || !strings.Contains(p, ".moca/skills") {
		t.Fatalf("the stored prompt must carry the block:\n%s", p)
	}
	// A second run lists it with the marker, the framing line and the note.
	_, errOut, code := g.run("anything else", "--approve")
	if code != 0 {
		t.Fatalf("second run exit %d: %s", code, errOut)
	}
	p2 := g.prompt()
	if !strings.Contains(p2, "moca-testing") || !strings.Contains(p2, "(auto)") || !strings.Contains(p2, "fallible") {
		t.Fatalf("the listing must mark the auto skill and add the framing line:\n%s", p2)
	}
	if !strings.Contains(errOut, "note: self-managed skills: 1 skill in this project (moca-testing)") {
		t.Fatalf("the startup note must name the count: %q", errOut)
	}
	t.Logf("leg 2: a conforming write lands, is listed (auto), and the note arrives")
}

func legCapsAndOwnership(t *testing.T) {
	big := strings.Repeat("x", 2000)
	// One run with several bad writes, then good ones.
	g := newGate(t,
		toolTurn(
			writeCall(".moca/skills/moca-big/SKILL.md", autoSkillFile("moca-big", "d", rule("a", big, "×1, "+today()))),
			writeCall(".moca/skills/moca-rules/SKILL.md", autoSkillFile("moca-rules", "d", manyRules(13))),
			writeCall(".moca/skills/moca-noprov/SKILL.md", autoSkillFile("moca-noprov", "d", "- **a**: b\n")),
			writeCall(".moca/skills/testing/SKILL.md", autoSkillFile("testing", "d", rule("a", "b", "×1, "+today()))),
			writeCall(".moca/skills/moca-wrong/SKILL.md", autoSkillFile("moca-other", "d", rule("a", "b", "×1, "+today()))),
			writeCall(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n"),
			readCall(".moca/skills/moca-ok/SKILL.md"),
			editCall(".moca/skills/moca-ok/SKILL.md", "auto: true\n", ""),
			writeCall(".moca/skills/moca-ok/references/x.md", "x\n"),
			writeCall(".moca/skills/moca-fresh/notes.md", "x\n"),
		),
		textTurn("done"),
	)
	seedSkill(t, g.work, "moca-ok", autoSkillFile("moca-ok", "d", rule("a", "b", "×1, "+today())))
	g.user(`"skills":{"selfManaged":true}`)
	git(t, g.work, "add", "-A")
	git(t, g.work, "commit", "-qm", "seed")
	_, errOut, code := g.run("learn", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, bad := range []string{".moca/skills/moca-big/SKILL.md", ".moca/skills/moca-rules/SKILL.md",
		".moca/skills/moca-noprov/SKILL.md", ".moca/skills/testing/SKILL.md", ".moca/skills/moca-wrong/SKILL.md",
		".moca/skills/hand/SKILL.md", ".moca/skills/moca-ok/references/x.md", ".moca/skills/moca-fresh/notes.md"} {
		if g.exists(bad) {
			t.Errorf("%s must not exist after its refusal", bad)
		}
	}
	if !strings.Contains(g.read(".moca/skills/moca-ok/SKILL.md"), "auto: true") {
		t.Fatal("the marker must not be dropped")
	}
	// The count cap: 8 seeded skills, the 9th refused with the names listed;
	// emptying one frees the slot.
	h := newGate(t,
		toolTurn(
			writeCall(".moca/skills/moca-new/SKILL.md", autoSkillFile("moca-new", "d", rule("a", "b", "×1, "+today()))),
		),
		textTurn("done"),
	)
	for i := 0; i < 8; i++ {
		seedSkill(t, h.work, fmt.Sprintf("moca-s%d", i), autoSkillFile(fmt.Sprintf("moca-s%d", i), "d", rule("a", "b", "×2, "+dayAgo(10))))
	}
	h.user(`"skills":{"selfManaged":true}`)
	git(t, h.work, "add", "-A")
	git(t, h.work, "commit", "-qm", "seed")
	if _, errOut, code := h.run("learn", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if h.exists(".moca/skills/moca-new/SKILL.md") {
		t.Fatal("the 9th auto skill must be refused")
	}
	k := newGate(t,
		toolTurn(
			readCall(".moca/skills/moca-s0/SKILL.md"),
			writeCall(".moca/skills/moca-s0/SKILL.md", autoSkillFile("moca-s0", "d", "# emptied\n")),
			writeCall(".moca/skills/moca-new/SKILL.md", autoSkillFile("moca-new", "d", rule("a", "b", "×1, "+today()))),
		),
		textTurn("done"),
	)
	for i := 0; i < 8; i++ {
		seedSkill(t, k.work, fmt.Sprintf("moca-s%d", i), autoSkillFile(fmt.Sprintf("moca-s%d", i), "d", rule("a", "b", "×2, "+dayAgo(10))))
	}
	k.user(`"skills":{"selfManaged":true}`)
	git(t, k.work, "add", "-A")
	git(t, k.work, "commit", "-qm", "seed")
	if _, errOut, code := k.run("learn", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !k.exists(".moca/skills/moca-new/SKILL.md") {
		t.Fatal("emptying an auto skill must free its slot")
	}
	t.Logf("leg 3: caps, ownership, single-file and write-order refusals all hold; the 9th is refused and an emptied skill frees the slot")
}

func manyRules(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(rule(fmt.Sprintf("r%d", i), "x", "×2, "+dayAgo(10)))
	}
	return sb.String()
}

func legSecretScan(t *testing.T) {
	const token = "ghp_abcdefgh12345678"
	g := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("token", "use "+token, "×1, "+today())))),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	_, errOut, code := g.run("learn", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if g.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("a credential-shaped rule must be refused")
	}
	if strings.Contains(g.status(), token) {
		t.Fatal("the token must not appear in the workdir's git status")
	}
	var found []string
	filepath.Walk(g.work, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() && !strings.Contains(p, "/.git/") {
			if b, rerr := os.ReadFile(p); rerr == nil && strings.Contains(string(b), token) {
				found = append(found, p)
			}
		}
		return nil
	})
	if len(found) > 0 {
		t.Fatalf("the token reached %v", found)
	}
	// The transcript does store tool input verbatim by construction; the key
	// scan is the provider key (acpgate shape).
	var captured strings.Builder
	captured.WriteString(g.readSessions())
	captured.WriteString(errOut)
	captured.WriteString(g.readLogs())
	if strings.Contains(captured.String(), providerKey) {
		t.Fatal("the provider key leaked into the transcript, stderr or the log")
	}
	t.Logf("leg 4: the secret scan refuses the rule; nothing reached the workdir; the provider key never leaked")
}

func (g *gate) readSessions() string {
	var sb strings.Builder
	paths, _ := filepath.Glob(filepath.Join(g.data, "moca", "sessions", "*.jsonl"))
	for _, p := range paths {
		b, _ := os.ReadFile(p)
		sb.Write(b)
	}
	return sb.String()
}

func (g *gate) readLogs() string {
	var sb strings.Builder
	paths, _ := filepath.Glob(filepath.Join(g.state, "moca", "logs", "*"))
	for _, p := range paths {
		b, _ := os.ReadFile(p)
		sb.Write(b)
	}
	return sb.String()
}

func legPerRunCap(t *testing.T) {
	g := newGate(t,
		toolTurn(
			writeCall(".moca/skills/moca-a/SKILL.md", autoSkillFile("moca-a", "d", rule("a", "b", "×1, "+today()))),
			writeCall(".moca/skills/moca-b/SKILL.md", autoSkillFile("moca-b", "d", rule("a", "b", "×1, "+today()))),
			writeCall(".moca/skills/moca-c/SKILL.md", autoSkillFile("moca-c", "d", rule("a", "b", "×1, "+today()))),
		),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	if _, errOut, code := g.run("learn", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !g.exists(".moca/skills/moca-a/SKILL.md") || !g.exists(".moca/skills/moca-b/SKILL.md") {
		t.Fatal("the first two creations must land")
	}
	if g.exists(".moca/skills/moca-c/SKILL.md") {
		t.Fatal("the 3rd creation in one run must be refused")
	}
	t.Logf("leg 5: the per-run creation cap holds")
}

func legSwitchTable(t *testing.T) {
	// (a) project true, no user opt-in: starts, warns, no block, no guard.
	a := newGate(t, toolTurn(writeCall(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n")), textTurn("done"))
	a.user("")
	a.project(`{"skills":{"selfManaged":true}}`)
	_, errOut, code := a.run("learn", "--approve")
	if code != 0 {
		t.Fatalf("a project true must never stop moca: exit %d %s", code, errOut)
	}
	if !strings.Contains(errOut, "warning: this project's .moca/config.jsonc sets skills.selfManaged true") ||
		!strings.Contains(errOut, "does not enable it") {
		t.Fatalf("warning missing: %q", errOut)
	}
	if strings.Contains(a.prompt(), "# Project skills you maintain") || !a.exists(".moca/skills/hand/SKILL.md") {
		t.Fatal("the feature must be off (no block, unvalidated write lands)")
	}
	// (b) user false + project true: the "sets it false" wording.
	b := newGate(t, textTurn("ok"))
	b.user(`"skills":{"selfManaged":false}`)
	b.project(`{"skills":{"selfManaged":true}}`)
	_, errOut, _ = b.run("hello", "--approve")
	if !strings.Contains(errOut, "your user config sets it false") {
		t.Fatalf("wording: %q", errOut)
	}
	// (c) user true + project false: off with a note.
	c := newGate(t, textTurn("ok"))
	c.user(`"skills":{"selfManaged":true}`)
	c.project(`{"skills":{"selfManaged":false}}`)
	_, errOut, _ = c.run("hello", "--approve")
	if !strings.Contains(errOut, "note: self-managed skills are off in this project") {
		t.Fatalf("note: %q", errOut)
	}
	if c.headerHasBlock() {
		t.Fatal("project false: no block")
	}
	// (d) user true + project true: on, no message.
	d := newGate(t, textTurn("ok"))
	d.user(`"skills":{"selfManaged":true}`)
	d.project(`{"skills":{"selfManaged":true}}`)
	_, errOut, _ = d.run("hello", "--approve")
	if strings.Contains(errOut, "skills.selfManaged") {
		t.Fatalf("no message expected: %q", errOut)
	}
	if !d.headerHasBlock() {
		t.Fatal("on: the header must record the block")
	}
	t.Logf("leg 6: every row of the switch table behaves, and none exits non-zero")
}

func legHygiene(t *testing.T) {
	body := rule("old", "stale", "×1, "+dayAgo(91)) + rule("old2", "pinned", "user, "+dayAgo(400)) +
		rule("old3", "confirmed", "×2, "+dayAgo(91)) + "- **undated**: no provenance\n"
	// -p reports only.
	a := newGate(t, textTurn("ok"))
	seedSkill(t, a.work, "moca-x", autoSkillFile("moca-x", "d", body))
	a.user(`"skills":{"selfManaged":true}`)
	git(t, a.work, "add", "-A")
	git(t, a.work, "commit", "-qm", "seed")
	_, errOut, code := a.run("hello", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if got := a.read(".moca/skills/moca-x/SKILL.md"); got != autoSkillFile("moca-x", "d", body) {
		t.Fatalf("a headless run must not change the file:\n%s", got)
	}
	if !strings.Contains(errOut, "would be removed") || !strings.Contains(errOut, "run moca interactively") {
		t.Fatalf("the headless report: %q", errOut)
	}
	if !strings.Contains(errOut, "pinned rule in moca-x is over a year old") {
		t.Fatalf("the aged pin must be named: %q", errOut)
	}
	// The interactive front end (ACP) applies it.
	b := newGate(t, textTurn("ok"))
	seedSkill(t, b.work, "moca-x", autoSkillFile("moca-x", "d", body))
	seedSkill(t, b.work, "hand", "---\nname: hand\ndescription: d\n---\n"+rule("old", "stale", "×1, "+dayAgo(91)))
	b.user(`"skills":{"selfManaged":true}`)
	git(t, b.work, "add", "-A")
	git(t, b.work, "commit", "-qm", "seed")
	p := startACP(t, b.childEnv(), b.work)
	defer p.close()
	p.call("initialize", map[string]any{"protocolVersion": 1})
	res, ups := p.call("session/new", map[string]any{"cwd": b.work, "mcpServers": []any{}})
	if res["error"] != nil {
		t.Fatalf("session/new: %v", res)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !strings.Contains(b.read(".moca/skills/moca-x/SKILL.md"), "stale") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	got := b.read(".moca/skills/moca-x/SKILL.md")
	if strings.Contains(got, "stale") || !strings.Contains(got, "confirmed") || !strings.Contains(got, "pinned") || !strings.Contains(got, "undated") {
		t.Fatalf("interactive hygiene wrong:\n%s", got)
	}
	ups = append(ups, p.drain(2*time.Second)...)
	if !strings.Contains(p.promptText(ups), "skill hygiene") {
		t.Fatalf("the interactive start must report the removal: %v", ups)
	}
	if !strings.Contains(b.read(".moca/skills/hand/SKILL.md"), "stale") {
		t.Fatal("a hand-written skill must stay byte-identical")
	}
	t.Logf("leg 7: headless reports only; interactive applies; ×2, pins and undated lines survive; hand-written untouched")
}

func legNoExtraFiles(t *testing.T) {
	g := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", "b", "×1, "+today())))),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	if _, errOut, code := g.run("learn", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	st := g.status()
	for _, line := range strings.Split(strings.TrimSpace(st), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(line, ".moca/skills/moca-x/SKILL.md") {
			t.Fatalf("unexpected workdir change: %q\n%s", line, st)
		}
	}
	for _, bad := range []string{".moca/skills/.gitignore", ".moca/skills/index.json", ".moca/skills/manifest.yml"} {
		if g.exists(bad) {
			t.Fatalf("%s must not be created", bad)
		}
	}
	if baks, _ := filepath.Glob(filepath.Join(g.work, ".moca", "skills", "*", "*.bak")); len(baks) > 0 {
		t.Fatalf("no .bak files: %v", baks)
	}
	t.Logf("leg 8: the only workdir change is the skill file the run wrote")
}

func legPWithoutApprove(t *testing.T) {
	g := newGate(t,
		toolTurn(writeCall(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n")),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	if _, errOut, code := g.run("learn"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if strings.Contains(g.prompt(), "# Project skills you maintain") {
		t.Fatal("untrusted -p: no block")
	}
	if !g.exists(".moca/skills/hand/SKILL.md") {
		t.Fatal("untrusted -p: the guard is nil, so an invalid write lands unvalidated")
	}
	t.Logf("leg 9: -p without --approve is inert (the trust gate)")
}

func legPlanMode(t *testing.T) {
	g := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", "b", "×1, "+today())))),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	_, errOut, code := g.run("plan something", "--approve", "--mode", "plan")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(g.prompt(), "# Project skills you maintain") {
		t.Fatal("plan mode still carries the block (it follows switch + trust)")
	}
	if g.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("plan mode: the write must be confined")
	}
	if strings.Contains(errOut, "[hint: something durable") {
		t.Fatal("no hint in plan mode")
	}
	t.Logf("leg 10: plan mode never learns")
}

func legSymlinks(t *testing.T) {
	// (a) .moca → <workdir>/real-moca (inside): the guard, not the jail.
	a := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", strings.Repeat("x", 2000), "×1, "+today())))),
		textTurn("done"),
	)
	if err := os.MkdirAll(filepath.Join(a.work, "real-moca", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(a.work, "real-moca"), filepath.Join(a.work, ".moca")); err != nil {
		t.Fatal(err)
	}
	a.user(`"skills":{"selfManaged":true}`)
	_, errOut, code := a.run("learn", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(a.work, "real-moca", "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the guard must refuse the over-cap write through the symlink")
	}
	// (b) .moca → a sibling outside the workdir: the switch warning and the
	// jail's confinement, a different mechanism.
	b := newGate(t, toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", "b", "×1, "+today())))), textTurn("done"))
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outer, filepath.Join(b.work, ".moca")); err != nil {
		t.Fatal(err)
	}
	b.user(`"skills":{"selfManaged":true}`)
	_, errOut, code = b.run("learn", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(errOut, "self-managed skills are off for this session") {
		t.Fatalf("the switch-gated warning: %q", errOut)
	}
	if _, err := os.Stat(filepath.Join(outer, "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the jail must refuse the write outside the workdir")
	}
	t.Logf("leg 11: neither a symlinked .moca inside nor outside the workdir opens the guard")
}

func legACPParity(t *testing.T) {
	// On: remember advertised, no moca-* command, the note arrives, and an
	// over-cap write is refused by the same guard — with no --approve.
	g := newGate(t, toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", strings.Repeat("x", 2000), "×1, "+today())))), textTurn("done"))
	seedSkill(t, g.work, "moca-one", autoSkillFile("moca-one", "d", rule("a", "b", "×2, "+dayAgo(5))))
	g.user(`"skills":{"selfManaged":true}`)
	git(t, g.work, "add", "-A")
	git(t, g.work, "commit", "-qm", "seed")
	p := startACP(t, g.childEnv(), g.work)
	defer p.close()
	p.call("initialize", map[string]any{"protocolVersion": 1})
	res, ups := p.call("session/new", map[string]any{"cwd": g.work, "mcpServers": []any{}})
	if res["error"] != nil {
		t.Fatalf("session/new: %v", res)
	}
	sid := res["result"].(map[string]any)["sessionId"].(string)
	ups = append(ups, p.drain(2*time.Second)...)
	cmds := commandNames(ups)
	if !contains(cmds, "remember") {
		t.Fatalf("remember must be advertised: %v", cmds)
	}
	for _, c := range cmds {
		if strings.HasPrefix(c, "moca-") {
			t.Fatalf("an auto skill must never be advertised: %v", cmds)
		}
	}
	all := p.promptText(ups)
	if !strings.Contains(all, "self-managed skills: 1 skill") {
		t.Fatalf("the startup note must arrive: %q", all)
	}
	r, _ := p.call("session/prompt", map[string]any{"sessionId": sid, "prompt": []any{map[string]any{"type": "text", "text": "learn"}}})
	if r["error"] != nil {
		t.Fatalf("prompt: %v", r)
	}
	if _, err := os.Stat(filepath.Join(g.work, ".moca", "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("ACP's guard must refuse the over-cap write")
	}
	// Off: remember is not advertised.
	off := newGate(t, textTurn("ok"))
	off.user("")
	q := startACP(t, off.childEnv(), off.work)
	defer q.close()
	q.call("initialize", map[string]any{"protocolVersion": 1})
	_, ups = q.call("session/new", map[string]any{"cwd": off.work, "mcpServers": []any{}})
	for _, c := range commandNames(ups) {
		if c == "remember" {
			t.Fatal("remember must not be advertised with the feature off")
		}
	}
	t.Logf("leg 12: ACP parity — remember advertised only when on, no moca-* command, the note delivered, the guard live without a flag")
}

func legShellAndInterpreters(t *testing.T) {
	g := newGate(t,
		toolTurn(
			shellCall("printf x > .moca/skills/moca-x/SKILL.md"),
			shellCall("printf x >> .moca/skills/moca-x/SKILL.md"),
			shellCall("echo x | tee .moca/skills/moca-x/SKILL.md"),
			shellCall("cp a.md .moca/skills/moca-x/SKILL.md"),
			shellCall("mv a.md .moca/skills/moca-x/SKILL.md"),
			shellCall("sed -i s/a/b/ .moca/skills/moca-x/SKILL.md"),
			// Any other workdir path behaves as before.
			shellCall("printf x > .moca/prompts/keep.md"),
		),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	if err := os.MkdirAll(filepath.Join(g.work, ".moca", "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := g.run("try the shell", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if g.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("no shell write may land")
	}
	if !g.exists(".moca/prompts/keep.md") {
		t.Fatalf("the refusal must not cover unrelated workdir paths: stderr %s", errOut)
	}
	// Auto mode swaps in AutoView's copy of the Shell: the refusal travels.
	a := newGate(t,
		toolTurn(shellCall("printf x > .moca/skills/moca-x/SKILL.md")),
		textTurn("done"),
	)
	a.user(`"skills":{"selfManaged":true}`)
	_, errOut, code = a.run("try it in auto", "--approve", "--mode", "auto")
	if code != 0 {
		t.Fatalf("auto exit %d: %s", code, errOut)
	}
	if a.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("auto mode must keep the refusal (AutoView carries skillDir)")
	}
	// An interpreter still writes (pattern-level only) — and the next
	// session's listing drops the broken skill with one warning.
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatal("the interpreter leg needs python3 on PATH")
	}
	py := `import pathlib; pathlib.Path(".moca/skills/moca-hack").mkdir(parents=True, exist_ok=True); pathlib.Path(".moca/skills/moca-hack/SKILL.md").write_text("---" + chr(10) + "name: moca-hack" + chr(10) + "description: d" + chr(10) + "auto: true" + chr(10) + "---" + chr(10) + "- **a**: " + "x"*2000 + chr(10))`
	b := newGate(t,
		toolTurn(shellCall("python3 -c '"+py+"'")),
		textTurn("done"),
		textTurn("second"),
	)
	b.user(`"skills":{"selfManaged":true}`)
	_, errOut, code = b.run("try python", "--approve")
	if code != 0 {
		t.Fatalf("python exit %d: %s", code, errOut)
	}
	if !b.exists(".moca/skills/moca-hack/SKILL.md") {
		t.Fatalf("the interpreter write must land (the shell check is pattern-level): %s", errOut)
	}
	_, errOut, code = b.run("second session", "--approve")
	if code != 0 {
		t.Fatalf("second exit %d: %s", code, errOut)
	}
	if !strings.Contains(errOut, "moca-hack") || !strings.Contains(errOut, "cap 1500") || !strings.Contains(errOut, "warning:") {
		t.Fatalf("the load-time check must drop it with a warning: %q", errOut)
	}
	t.Logf("leg 13: shell writes refused (default and auto); an interpreter write lands and the next session drops it")
}

func legCaseVariant(t *testing.T) {
	g := newGate(t,
		toolTurn(
			writeCall(".moca/Skills/moca-x/skill.md", autoSkillFile("moca-x", "d", rule("a", "b", "×1, "+today()))),
			writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("a", "b", "×1, "+today()))),
		),
		textTurn("done"),
	)
	g.user(`"skills":{"selfManaged":true}`)
	c := exec.Command(*mocaBin, "--approve", "-p", "learn")
	c.Dir = g.work
	c.Env = g.childEnv("MOCA_CASEFOLD_FS=1")
	if err := c.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(g.work, ".moca", "Skills", "moca-x", "skill.md")); !os.IsNotExist(err) {
		t.Fatal("the case variant must be refused")
	}
	if !g.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("the canonical spelling must land")
	}
	// Without the hook (Linux semantics) the variant is outside the skills
	// dir and behaves like any other path.
	h := newGate(t,
		toolTurn(writeCall(".moca/Skills/other/skill.md", "plain\n")),
		textTurn("done"),
	)
	h.user(`"skills":{"selfManaged":true}`)
	if _, errOut, code := h.run("write elsewhere", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	t.Logf("leg 14: a case-variant spelling is refused under MOCA_CASEFOLD_FS=1; the canonical path lands")
}

func legCapsCheckChanges(t *testing.T) {
	// 9 seeded skills: an edit that deletes a rule lands; one that adds is
	// refused; a 2000-rune body accepts a shrinking edit and refuses growth.
	keep := rule("keep", "me", "user, "+dayAgo(5))
	g := newGate(t,
		toolTurn(
			readCall(".moca/skills/moca-s0/SKILL.md"),
			editCall(".moca/skills/moca-s0/SKILL.md", rule("drop", "me", "user, "+dayAgo(5)), ""),
			readCall(".moca/skills/moca-s1/SKILL.md"),
			editCall(".moca/skills/moca-s1/SKILL.md", "- **grow**: x (user, "+dayAgo(5)+")\n", "- **grow**: x (user, "+dayAgo(5)+") and more (user, "+dayAgo(5)+")\n"+rule("new", "rule", "×1, "+today())),
			readCall(".moca/skills/moca-s2/SKILL.md"),
			editCall(".moca/skills/moca-s2/SKILL.md", strings.Repeat("y", 1500), strings.Repeat("y", 1600)),
		),
		textTurn("done"),
	)
	for i := 0; i < 9; i++ {
		seedSkill(t, g.work, fmt.Sprintf("moca-s%d", i), autoSkillFile(fmt.Sprintf("moca-s%d", i), "d",
			rule("drop", "me", "user, "+dayAgo(5))+keep+rule("grow", "x", "user, "+dayAgo(5))))
	}
	// moca-s2 gets an over-cap body.
	seedSkill(t, g.work, "moca-s2", autoSkillFile("moca-s2", "d", rule("big", strings.Repeat("y", 1500), "user, "+dayAgo(5))))
	g.user(`"skills":{"selfManaged":true}`)
	git(t, g.work, "add", "-A")
	git(t, g.work, "commit", "-qm", "seed")
	if _, errOut, code := g.run("fix the rules", "--approve"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if strings.Contains(g.read(".moca/skills/moca-s0/SKILL.md"), "- **drop**") {
		t.Fatal("an edit that deletes a rule must land despite the count cap")
	}
	if strings.Contains(g.read(".moca/skills/moca-s1/SKILL.md"), "- **new**") {
		t.Fatal("an edit that adds a rule at the cap must be refused")
	}
	if strings.Contains(g.read(".moca/skills/moca-s2/SKILL.md"), strings.Repeat("y", 1600)) {
		t.Fatal("growth of an over-cap body must be refused")
	}
	// An over-cap <when> on an untouched line passes.
	h := newGate(t, textTurn("ok"))
	longWhen := strings.Repeat("w", 100)
	seedSkill(t, h.work, "moca-x", autoSkillFile("moca-x", "d",
		rule(longWhen, "old", "user, "+dayAgo(5))+rule("new", "rule", "×1, "+today())))
	h.user(`"skills":{"selfManaged":true}`)
	git(t, h.work, "add", "-A")
	git(t, h.work, "commit", "-qm", "seed")
	// (The write is exercised through a second run: the untouched long line
	// must not block it.)
	_, _, _ = h.run("hello", "--approve")
	if !strings.Contains(h.read(".moca/skills/moca-x/SKILL.md"), longWhen) {
		t.Fatal("an untouched over-cap line must survive")
	}
	t.Logf("leg 15: caps check changes, not states")
}

func legNamingAndPins(t *testing.T) {
	// A committed hand-written skill: -p can never change it, even when the
	// prompt says its name (a -p prompt is not typed text).
	g := newGate(t,
		toolTurn(readCall(".moca/skills/deploy/SKILL.md"), editCall(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2")),
		textTurn("done"),
	)
	seedSkill(t, g.work, "deploy", "---\nname: deploy\ndescription: ship it\n---\nstaging step\n")
	g.user(`"skills":{"selfManaged":true}`)
	git(t, g.work, "add", "-A")
	git(t, g.work, "commit", "-qm", "seed")
	_, errOut, code := g.run("fix the deploy skill's staging step", "--approve")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if got := g.read(".moca/skills/deploy/SKILL.md"); !strings.Contains(got, "staging step\n") || strings.Contains(got, "v2") {
		t.Fatalf("a -p prompt must never edit a hand-written skill:\n%s", got)
	}
	// Over ACP a text block that names it lands; a resource block does not.
	a := newGate(t,
		toolTurn(readCall(".moca/skills/deploy/SKILL.md"), editCall(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2")), textTurn("done"),
		toolTurn(editCall(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2")), textTurn("done"))
	seedSkill(t, a.work, "deploy", "---\nname: deploy\ndescription: ship it\n---\nstaging step\n")
	a.user(`"skills":{"selfManaged":true}`)
	git(t, a.work, "add", "-A")
	git(t, a.work, "commit", "-qm", "seed")
	p := startACP(t, a.childEnv(), a.work)
	defer p.close()
	p.call("initialize", map[string]any{"protocolVersion": 1})
	res, _ := p.call("session/new", map[string]any{"cwd": a.work, "mcpServers": []any{}})
	sid := res["result"].(map[string]any)["sessionId"].(string)
	// (c) a resource block that names deploy only inside the attachment.
	body := "fix the deploy skill's staging step"
	r, _ := p.call("session/prompt", map[string]any{"sessionId": sid, "prompt": []any{
		map[string]any{"type": "resource", "resource": map[string]any{"uri": "file://" + a.work + "/NOTES.md", "text": body}},
		map[string]any{"type": "text", "text": "review this"},
	}})
	if r["error"] != nil {
		t.Fatalf("prompt: %v", r)
	}
	if strings.Contains(a.read(".moca/skills/deploy/SKILL.md"), "v2") {
		t.Fatal("an attachment must not authorize a hand-written edit")
	}
	// (b) the same request in a text block lands.
	r, _ = p.call("session/prompt", map[string]any{"sessionId": sid, "prompt": []any{
		map[string]any{"type": "text", "text": "fix the deploy skill's staging step"},
	}})
	if r["error"] != nil {
		t.Fatalf("prompt: %v", r)
	}
	if !strings.Contains(a.read(".moca/skills/deploy/SKILL.md"), "v2") {
		t.Fatal("a typed request that names the skill must land")
	}
	// (d) a new (user, …) rule in a plain prompt is refused; /remember lands.
	b := newGate(t,
		toolTurn(writeCall(".moca/skills/moca-x/SKILL.md", autoSkillFile("moca-x", "d", rule("pin", "use it", "user, "+today())))),
		textTurn("done"),
		toolTurn(writeCall(".moca/skills/moca-y/SKILL.md", autoSkillFile("moca-y", "d", rule("pin", "use it", "user, "+today())))),
		textTurn("done"),
	)
	b.user(`"skills":{"selfManaged":true}`)
	q := startACP(t, b.childEnv(), b.work)
	defer q.close()
	q.call("initialize", map[string]any{"protocolVersion": 1})
	res, _ = q.call("session/new", map[string]any{"cwd": b.work, "mcpServers": []any{}})
	sid = res["result"].(map[string]any)["sessionId"].(string)
	q.call("session/prompt", map[string]any{"sessionId": sid, "prompt": []any{map[string]any{"type": "text", "text": "record the rule"}}})
	if b.exists(".moca/skills/moca-x/SKILL.md") {
		t.Fatal("a (user, …) rule needs /remember")
	}
	q.call("session/prompt", map[string]any{"sessionId": sid, "prompt": []any{map[string]any{"type": "text", "text": "/remember always use -count=1"}}})
	if !b.exists(".moca/skills/moca-y/SKILL.md") {
		t.Fatal("/remember must let the pin land")
	}
	t.Logf("leg 16: hand-written skills need the typed naming request; pins need /remember")
}
