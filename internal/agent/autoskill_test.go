package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

const switchOn = `"skills":{"selfManaged":true}`

func today() string { return time.Now().Format("2006-01-02") }

// managedConfig is startWith's config with the fake provider, built here so a
// test can splice a trusted project layer in (Project is json:"-": extraTop
// cannot carry it).
func managedConfig(t *testing.T, s *scriptServer, extraTop string) config.Config {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")
	if extraTop != "" {
		extraTop = "," + extraTop
	}
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m"%s,"providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}}}`, extraTop, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func startIn(t *testing.T, s *scriptServer, cfg config.Config, workdir string, mods ...func(*StartOptions)) (*Agent, *[]Event) {
	t.Helper()
	var evs []Event
	o := StartOptions{Config: cfg, Workdir: workdir, Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client(), Slug: "t"}
	for _, mod := range mods {
		mod(&o)
	}
	a, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	return a, &evs
}

// startManaged starts a trusted agent with the self-managed switch on.
func startManaged(t *testing.T, s *scriptServer, mods ...func(*StartOptions)) (*Agent, string, *[]Event) {
	t.Helper()
	all := append([]func(*StartOptions){func(o *StartOptions) { o.Trusted = true }}, mods...)
	return startWith(t, s, 40, switchOn, "", all...)
}

// startManagedIn is startManaged with a workdir the caller controls.
func startManagedIn(t *testing.T, s *scriptServer, workdir string, extraTop string, mods ...func(*StartOptions)) (*Agent, *[]Event) {
	t.Helper()
	cfg := managedConfig(t, s, extraTop)
	if p, err := config.LoadProject(workdir); err == nil {
		cfg.Project = p
	} else {
		t.Fatal(err)
	}
	all := append([]func(*StartOptions){func(o *StartOptions) { o.Trusted = true }}, mods...)
	return startIn(t, s, cfg, workdir, all...)
}

func toolCall(name, args string) llm.ToolCall {
	id := args
	if len(id) > 12 {
		id = id[:12]
	}
	return llm.ToolCall{ID: "c-" + name + "-" + id, Name: name, Input: json.RawMessage(args)}
}

func runTool(t *testing.T, a *Agent, name, args string) tools.Result {
	t.Helper()
	return a.opts.Tools.Run(context.Background(), a.opts.Env, toolCall(name, args))
}

func writeArgs(path, content string) string {
	b, _ := json.Marshal(map[string]string{"path": path, "content": content})
	return string(b)
}

func editArgs(path, old, new string) string {
	b, _ := json.Marshal(map[string]string{"path": path, "old_string": old, "new_string": new})
	return string(b)
}

func autoBody(rules ...string) string {
	var sb strings.Builder
	for i, r := range rules {
		sb.WriteString("- **r" + string(rune('a'+i)) + "**: " + r + " (×1, " + today() + ")\n")
	}
	return sb.String()
}

func autoFile(name, desc, body string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\nauto: true\n---\n" + body
}

func writeProjectConfig(t *testing.T, workdir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(workdir, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, ".moca", "config.jsonc"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedAutoSkill(t *testing.T, workdir, name, body string) string {
	t.Helper()
	dir := filepath.Join(workdir, ".moca", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(autoFile(name, "d", body)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hasNote(evs *[]Event, sub string) bool {
	for _, e := range *evs {
		if n, ok := e.(Note); ok && strings.Contains(n.Text, sub) {
			return true
		}
	}
	return false
}

func warningTexts(evs *[]Event) []string {
	var out []string
	for _, e := range *evs {
		if w, ok := e.(Warning); ok {
			out = append(out, w.Text)
		}
	}
	return out
}

// --- the guard -------------------------------------------------------------

func TestGuardInstalledOnlyWhenOnAndTrusted(t *testing.T) {
	// Off (no user opt-in), trusted: no guard, no dir, no block.
	a, _, _ := startWith(t, newScript(t), 40, "", "", func(o *StartOptions) { o.Trusted = true })
	if a.opts.Env.SkillGuard != nil || a.opts.Env.SkillDir != "" || a.SelfManagedSkills() {
		t.Fatal("with the switch off there is no guard")
	}
	if strings.Contains(a.opts.System, "# Project skills you maintain") {
		t.Fatal("no block with the switch off")
	}
	a.Session().Close()

	// On but untrusted: still inert.
	b, _, _ := startWith(t, newScript(t), 40, switchOn, "", nil...)
	if b.opts.Env.SkillGuard != nil || b.SelfManagedSkills() {
		t.Fatal("an untrusted workdir never learns")
	}
	b.Session().Close()

	// On and trusted: the validating guard, the resolved dir and the block.
	c, work, _ := startManaged(t, newScript(t))
	defer c.Session().Close()
	if c.opts.Env.SkillGuard == nil || c.opts.Env.SkillGuardRefusing || !c.SelfManagedSkills() {
		t.Fatal("a trusted workdir with the switch on must install a validating guard")
	}
	realRoot, _ := filepath.EvalSymlinks(work)
	if want := filepath.Join(realRoot, ".moca", "skills"); c.opts.Env.SkillDir != want {
		t.Fatalf("SkillDir = %q, want %q", c.opts.Env.SkillDir, want)
	}
	if !strings.Contains(c.opts.System, selfManagedProbe(c.opts.Env.SkillDir)) {
		t.Fatal("the stored prompt must carry the block")
	}
}

func TestGuardRefusesInvalidSkillAndLandsValidOne(t *testing.T) {
	a, work, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	big := strings.Repeat("x", 2000)
	r := runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", "- **a**: "+big+" (×1, "+today()+")\n")))
	if !r.IsError || !strings.Contains(r.Content, "cap 1500") {
		t.Fatalf("over-cap write must be refused: %q", r.Content)
	}
	if _, err := os.Stat(filepath.Join(work, ".moca", "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("a refused write must leave nothing on disk")
	}
	ok := runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", autoBody("run the suite with -count=1"))))
	if ok.IsError {
		t.Fatalf("a conforming auto skill must land: %q", ok.Content)
	}
	if a.opts.Env.SkillCreations != 1 {
		t.Fatalf("creations=%d, want 1", a.opts.Env.SkillCreations)
	}
}

func TestGuardScopeOnSymlinkedMoca(t *testing.T) {
	// .moca is a symlink INSIDE the workdir holding skills/: the guard must
	// still fire (a lexical SkillDir would fail open — F8/H12).
	var real string
	a, work, _ := startManaged(t, newScript(t), func(o *StartOptions) {
		real = filepath.Join(o.Workdir, "real-moca")
		if err := os.MkdirAll(filepath.Join(real, "skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, filepath.Join(o.Workdir, ".moca")); err != nil {
			t.Fatal(err)
		}
	})
	defer a.Session().Close()
	if a.opts.Env.SkillGuard == nil {
		t.Fatal("a symlinked .moca inside the workdir must not turn the guard off")
	}
	want, err := filepath.EvalSymlinks(filepath.Join(work, ".moca", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if a.opts.Env.SkillDir != want {
		t.Fatalf("SkillDir = %q, want %q", a.opts.Env.SkillDir, want)
	}
	big := strings.Repeat("x", 2000)
	r := runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", "- **a**: "+big+" (×1, "+today()+")\n")))
	if !r.IsError || !strings.Contains(r.Content, "cap 1500") {
		t.Fatalf("the guard must refuse, not the jail: %q", r.Content)
	}
	if _, err := os.Stat(filepath.Join(real, "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("nothing may land")
	}
}

func TestGuardScopeOnSymlinkedWorkdir(t *testing.T) {
	var link string
	a, work, _ := startManaged(t, newScript(t), func(o *StartOptions) {
		link = filepath.Join(filepath.Dir(o.Workdir), "link")
		if err := os.Symlink(o.Workdir, link); err != nil {
			t.Fatal(err)
		}
		o.Workdir = link
	})
	defer a.Session().Close()
	if a.opts.Env.SkillGuard == nil {
		t.Fatal("a symlinked workdir must not turn the guard off (F8)")
	}
	big := strings.Repeat("x", 2000)
	r := runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", "- **a**: "+big+" (×1, "+today()+")\n")))
	if !r.IsError || !strings.Contains(r.Content, "cap 1500") {
		t.Fatalf("%q", r.Content)
	}
	if _, err := os.Stat(filepath.Join(work, ".moca", "skills", "moca-x", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("nothing may land")
	}
	_ = link
}

func TestSymlinkedMocaOutsideWorkdirIsOff(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	a, _, evs := startManaged(t, newScript(t), func(o *StartOptions) {
		if err := os.Symlink(outer, filepath.Join(o.Workdir, ".moca")); err != nil {
			t.Fatal(err)
		}
	})
	defer a.Session().Close()
	if a.opts.Env.SkillGuard != nil || a.opts.Env.SkillDir != "" {
		t.Fatal("an unresolvable skills dir means no guard")
	}
	if !slices.ContainsFunc(warningTexts(evs), func(s string) bool {
		return strings.Contains(s, "self-managed skills are off for this session")
	}) {
		t.Fatalf("one switch-gated warning is expected: %v", warningTexts(evs))
	}
	// The write is refused by the jail — a different mechanism, asserted here
	// separately.
	r := runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", autoBody("x"))))
	if !r.IsError || !strings.Contains(r.Content, "outside the workdir") {
		t.Fatalf("want the jail's confinement refusal: %q", r.Content)
	}
}

func TestGuardResourceFiles(t *testing.T) {
	a, work, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	sk := filepath.Join(work, ".moca", "skills")
	// A hand-written skill with its own reference file.
	if err := os.MkdirAll(filepath.Join(sk, "deploy", "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "deploy", "SKILL.md"), []byte("---\nname: deploy\ndescription: d\n---\nprose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runTool(t, a, "write", writeArgs(".moca/skills/deploy/references/x.md", "notes\n"))
	if !r.IsError || !strings.Contains(r.Content, "hand-written skill") {
		t.Fatalf("an unnamed resource write must be refused: %q", r.Content)
	}
	a.opts.Env.SkillTyped = []string{"extend the deploy skill's notes"}
	r = runTool(t, a, "write", writeArgs(".moca/skills/deploy/references/x.md", "notes\n"))
	if r.IsError {
		t.Fatalf("a named resource write must land: %q", r.Content)
	}
	// An extra file inside an auto skill is refused.
	if err := os.MkdirAll(filepath.Join(sk, "moca-testing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "moca-testing", "SKILL.md"), []byte(autoFile("moca-testing", "d", autoBody("rule"))), 0o644); err != nil {
		t.Fatal(err)
	}
	r = runTool(t, a, "write", writeArgs(".moca/skills/moca-testing/notes.md", "x\n"))
	if !r.IsError || !strings.Contains(r.Content, "single SKILL.md") {
		t.Fatalf("an extra file in an auto skill must be refused: %q", r.Content)
	}
	// A loose file in the skills root passes.
	if r = runTool(t, a, "write", writeArgs(".moca/skills/README.md", "x\n")); r.IsError {
		t.Fatalf("a loose file in the skills root is not a skill: %q", r.Content)
	}
	// A foreign file into a dir with no SKILL.md is refused (write order).
	r = runTool(t, a, "write", writeArgs(".moca/skills/moca-fresh/notes.md", "x\n"))
	if !r.IsError || !strings.Contains(r.Content, "has no SKILL.md yet") {
		t.Fatalf("the write order must be pinned: %q", r.Content)
	}
}

func TestGuardOwnershipEndToEnd(t *testing.T) {
	a, _, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	r := runTool(t, a, "write", writeArgs(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n"))
	if !r.IsError || !strings.Contains(r.Content, "must be an auto skill") {
		t.Fatalf("a new non-auto skill must be refused: %q", r.Content)
	}
	if r = runTool(t, a, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", autoBody("rule")))); r.IsError {
		t.Fatalf("%q", r.Content)
	}
	r = runTool(t, a, "edit", editArgs(".moca/skills/moca-x/SKILL.md", "auto: true\n", ""))
	if !r.IsError || !strings.Contains(r.Content, "keep auto: true") {
		t.Fatalf("dropping the marker must be refused: %q", r.Content)
	}
}

func TestHandWrittenSkillEditNeedsNaming(t *testing.T) {
	a, work, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	sk := filepath.Join(work, ".moca", "skills", "deploy")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "SKILL.md"), []byte("---\nname: deploy\ndescription: d\n---\nstaging step\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	load := func() {
		if r := runTool(t, a, "read", `{"path":".moca/skills/deploy/SKILL.md"}`); r.IsError {
			t.Fatal(r.Content)
		}
	}
	load()
	r := runTool(t, a, "edit", editArgs(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2"))
	if !r.IsError || !strings.Contains(r.Content, "hand-written skill") {
		t.Fatalf("an unnamed edit must be refused: %q", r.Content)
	}
	a.opts.Env.SkillTyped = []string{"fix the redeploy script"}
	load()
	if r = runTool(t, a, "edit", editArgs(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2")); !r.IsError {
		t.Fatal("redeploy must not name deploy")
	}
	a.opts.Env.SkillTyped = []string{"fix the deploy skill's staging step"}
	load()
	if r = runTool(t, a, "edit", editArgs(".moca/skills/deploy/SKILL.md", "staging step", "staging step v2")); r.IsError {
		t.Fatalf("a named edit must land: %q", r.Content)
	}
	if b, _ := os.ReadFile(filepath.Join(sk, "SKILL.md")); !strings.Contains(string(b), "staging step v2") {
		t.Fatal("the named edit must land")
	}
}

func TestSkillCreationCapPerRun(t *testing.T) {
	a, work, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	for i, name := range []string{"moca-a", "moca-b"} {
		if r := runTool(t, a, "write", writeArgs(".moca/skills/"+name+"/SKILL.md", autoFile(name, "d", autoBody("rule")))); r.IsError {
			t.Fatalf("creation %d must land: %q", i+1, r.Content)
		}
	}
	r := runTool(t, a, "write", writeArgs(".moca/skills/moca-c/SKILL.md", autoFile("moca-c", "d", autoBody("rule"))))
	if !r.IsError || !strings.Contains(r.Content, "cap 2") {
		t.Fatalf("the 3rd creation must be refused: %q", r.Content)
	}
	for _, name := range []string{"moca-a", "moca-b"} {
		if _, err := os.Stat(filepath.Join(work, ".moca", "skills", name, "SKILL.md")); err != nil {
			t.Fatalf("%s must be on disk: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(work, ".moca", "skills", "moca-c", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("the refused creation must not exist")
	}
	// A new run resets the counter.
	a.opts.Env.BeginRun()
	if r := runTool(t, a, "write", writeArgs(".moca/skills/moca-c/SKILL.md", autoFile("moca-c", "d", autoBody("rule")))); r.IsError {
		t.Fatalf("the cap is per run: %q", r.Content)
	}
}

// --- hygiene, notices, switch messages -------------------------------------

func TestStartPrunesStaleSkillsOnlyWhenEnabled(t *testing.T) {
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
	body := "- **a**: stale (×1, " + old + ")\n- **b**: keep (×2, " + old + ")\n"
	work := t.TempDir()
	p := seedAutoSkill(t, work, "moca-x", body)
	cfg := managedConfig(t, newScript(t), switchOn)
	a, evs := startIn(t, newScript(t), cfg, work, func(o *StartOptions) { o.Trusted = true })
	a.Session().Close()
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "stale") || !strings.Contains(string(b), "keep") {
		t.Fatalf("the stale ×1 rule must go:\n%s", b)
	}
	if !slices.ContainsFunc(warningTexts(evs), func(s string) bool { return strings.Contains(s, "skill hygiene: removed 1 stale rule") }) {
		t.Fatalf("one hygiene warning: %v", warningTexts(evs))
	}
	// With the switch off, nothing is pruned.
	work2 := t.TempDir()
	q := seedAutoSkill(t, work2, "moca-x", body)
	b2, _ := startIn(t, newScript(t), managedConfig(t, newScript(t), ""), work2, func(o *StartOptions) { o.Trusted = true })
	b2.Session().Close()
	if got, _ := os.ReadFile(q); !strings.Contains(string(got), "stale") {
		t.Fatal("with the switch off hygiene must not run")
	}
}

func TestHeadlessHygieneReportsOnly(t *testing.T) {
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
	work := t.TempDir()
	p := seedAutoSkill(t, work, "moca-x", "- **a**: stale (×1, "+old+")\n- **b**: keep (×2, "+old+")\n")
	a, evs := startIn(t, newScript(t), managedConfig(t, newScript(t), switchOn), work,
		func(o *StartOptions) { o.Trusted, o.Headless = true, true })
	a.Session().Close()
	if b, _ := os.ReadFile(p); !strings.Contains(string(b), "stale") {
		t.Fatal("a headless run must never change a file")
	}
	if !slices.ContainsFunc(warningTexts(evs), func(s string) bool {
		return strings.Contains(s, "would be removed") && strings.Contains(s, "run moca interactively")
	}) {
		t.Fatalf("a headless run must report only: %v", warningTexts(evs))
	}
}

func TestResumeNeverPrunes(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, work, _ := startManaged(t, s)
	path := a.Session().Path()
	cfg := a.opts.Config
	a.Session().Close()
	old := time.Now().AddDate(0, 0, -100).Format("2006-01-02")
	p := seedAutoSkill(t, work, "moca-x", "- **a**: stale (×1, "+old+")\n- **b**: keep (×2, "+old+")\n")
	b, err := Resume(StartOptions{Config: cfg, Workdir: session.WorkdirOf(path), Trusted: true, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	if got, _ := os.ReadFile(p); !strings.Contains(string(got), "stale") {
		t.Fatal("Resume must never prune (a deletion would contradict the frozen prompt)")
	}
}

func TestStartNoticeNamesAutoCount(t *testing.T) {
	work := t.TempDir()
	seedAutoSkill(t, work, "moca-one", autoBody("rule"))
	a, evs := startManagedIn(t, newScript(t), work, switchOn)
	defer a.Session().Close()
	if !hasNote(evs, "self-managed skills: 1 skill in this project (moca-one)") {
		t.Fatalf("the startup note must name the count: %v", *evs)
	}
	// Switch off, auto skills present: the note stays (artifact-keyed).
	work2 := t.TempDir()
	seedAutoSkill(t, work2, "moca-one", autoBody("rule"))
	b, evs2 := startManagedIn(t, newScript(t), work2, "")
	defer b.Session().Close()
	if !hasNote(evs2, "self-managed skills: 1 skill") {
		t.Fatalf("the note rides the artifact, not the switch: %v", *evs2)
	}
	// Untrusted: no note.
	c, _, evs3 := startWith(t, newScript(t), 40, switchOn, "", nil...)
	defer c.Session().Close()
	if hasNote(evs3, "self-managed skills") {
		t.Fatal("an untrusted workdir gets no note")
	}
}

func TestSwitchMessagesEmittedOnce(t *testing.T) {
	// User unset + project true: one warning; the run starts and the feature
	// stays off.
	work := t.TempDir()
	writeProjectConfig(t, work, `{"skills":{"selfManaged":true}}`)
	a, evs := startManagedIn(t, newScript(t), work, "")
	defer a.Session().Close()
	if got := warningTexts(evs); len(got) != 1 || !strings.Contains(got[0], "does not enable it") {
		t.Fatalf("one warning: %v", got)
	}
	if a.opts.Env.SkillGuard != nil || a.SelfManagedSkills() {
		t.Fatal("a project true never enables the feature alone")
	}
	if _, err := os.Stat(filepath.Join(work, ".moca", "skills")); err == nil {
		t.Fatal("the run must not create the skills dir")
	}
	// A scripted invalid auto-skill write lands unvalidated: the feature is
	// off, not half-on.
	if r := runTool(t, a, "write", writeArgs(".moca/skills/hand/SKILL.md", "---\nname: hand\ndescription: d\n---\nprose\n")); r.IsError {
		t.Fatalf("no guard means no validation: %q", r.Content)
	}

	// User false + project true: the "sets it false" wording.
	work2 := t.TempDir()
	writeProjectConfig(t, work2, `{"skills":{"selfManaged":true}}`)
	b, evs2 := startManagedIn(t, newScript(t), work2, `"skills":{"selfManaged":false}`)
	defer b.Session().Close()
	if got := warningTexts(evs2); len(got) != 1 || !strings.Contains(got[0], "sets it false") {
		t.Fatalf("wording: %v", got)
	}

	// User true + project false: off with a note, no warning.
	work3 := t.TempDir()
	writeProjectConfig(t, work3, `{"skills":{"selfManaged":false}}`)
	c, evs3 := startManagedIn(t, newScript(t), work3, switchOn)
	defer c.Session().Close()
	if !hasNote(evs3, "self-managed skills are off in this project") || len(warningTexts(evs3)) != 0 {
		t.Fatalf("a project false opts out with a note: evs=%v", *evs3)
	}
	if c.SelfManagedSkills() {
		t.Fatal("a project false must opt the repository out")
	}

	// User true + project true: on, no message.
	work4 := t.TempDir()
	writeProjectConfig(t, work4, `{"skills":{"selfManaged":true}}`)
	d, evs4 := startManagedIn(t, newScript(t), work4, switchOn)
	defer d.Session().Close()
	if !d.SelfManagedSkills() || len(warningTexts(evs4)) != 0 || hasNote(evs4, "off in this project") {
		t.Fatalf("project true + user true is simply on: %v", *evs4)
	}

	// Untrusted: the project file is never read, so no message at all.
	work5 := t.TempDir()
	writeProjectConfig(t, work5, `{"skills":{"selfManaged":true}}`)
	cfg := managedConfig(t, newScript(t), "")
	e, evs5 := startIn(t, newScript(t), cfg, work5)
	defer e.Session().Close()
	if len(warningTexts(evs5)) != 0 || hasNote(evs5, "self-managed") {
		t.Fatalf("nothing for an untrusted workdir: %v", *evs5)
	}
}

// --- prompt block, resume rule ---------------------------------------------

func TestPromptBlockAndArtifactMarker(t *testing.T) {
	pre := BuildSystemPrompt(PromptInput{Workdir: "/w", OS: "Linux", Arch: "x86_64", Date: "2026-10-18", Version: "v0"})
	if strings.Contains(pre, "Project skills you maintain") {
		t.Fatal("nil SelfManaged must leave the prompt untouched")
	}
	in := PromptInput{Workdir: "/w", Skills: []skills.Skill{{Name: "moca-x", Description: "d", Path: "/w/.moca/skills/moca-x/SKILL.md", Auto: true}}}
	marked := BuildSystemPrompt(in)
	if !strings.Contains(marked, "(auto)") || !strings.Contains(marked, "fallible") {
		t.Fatalf("the artifact-keyed marker and framing line must be present:\n%s", marked)
	}
	if strings.Contains(marked, "correct them") {
		t.Fatal("the framing line must be read-only advice (C5c)")
	}
	in.SelfManaged = &SelfManagedInput{Dir: "/w/.moca/skills"}
	full := BuildSystemPrompt(in)
	if !strings.Contains(full, selfManagedProbe("/w/.moca/skills")) {
		t.Fatal("the probe must match the block")
	}
	for _, want := range []string{"moca-*", "auto: true", "≤ 8 skills", "≤ 12 rules", "(user, <date>)", "(×2, <today>)"} {
		if !strings.Contains(full, want) {
			t.Errorf("the block must name %q", want)
		}
	}
	// The block adds at most 12 lines.
	if d := strings.Count(full, "\n") - strings.Count(marked, "\n"); d > 12 {
		t.Fatalf("the block adds %d lines (≤ 12)", d)
	}
}

func TestSelfManagedHeaderField(t *testing.T) {
	a, _, _ := startManaged(t, newScript(t))
	path := a.Session().Path()
	a.Session().Close()
	head, err := session.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !head[0].Session.SelfManagedSkills {
		t.Fatal("Start must record the block in the session header")
	}
	b, _, _ := startWith(t, newScript(t), 40, "", "", func(o *StartOptions) { o.Trusted = true })
	path2 := b.Session().Path()
	b.Session().Close()
	head2, _ := session.ReadFile(path2)
	if head2[0].Session.SelfManagedSkills {
		t.Fatal("a feature-off header must stay byte-identical (no field)")
	}
	if strings.Contains(string(head2[0].Session.SystemPrompt), "Project skills you maintain") {
		t.Fatal("a feature-off prompt carries no block")
	}
}

func TestResumeKeepsOrRefusesTheGuard(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	path := a.Session().Path()
	cfg := a.opts.Config
	a.Session().Close()

	// Resumed with the switch still on: a validating guard.
	b, err := Resume(StartOptions{Config: cfg, Workdir: session.WorkdirOf(path), Trusted: true, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.SelfManagedSkills() || b.opts.Env.SkillGuardRefusing {
		t.Fatal("a resumed session with the switch on still learns")
	}
	b.Session().Close()

	// Resumed after the switch went off: a refusing guard (C5b).
	off := cfg
	off.Skills.SelfManaged = nil
	var evs []Event
	c, err := Resume(StartOptions{Config: off, Workdir: session.WorkdirOf(path), Trusted: true,
		Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Session().Close()
	if c.SelfManagedSkills() || !c.opts.Env.SkillGuardRefusing || c.opts.Env.SkillGuard == nil {
		t.Fatal("the frozen prompt still invites writes: the guard must refuse, not be nil")
	}
	if !slices.ContainsFunc(warningTexts(&evs), func(x string) bool { return strings.Contains(x, "skills.selfManaged is off") }) {
		t.Fatalf("one warning about the mismatch: %v", warningTexts(&evs))
	}
	r := runTool(t, c, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", autoBody("rule"))))
	if !r.IsError || !strings.Contains(r.Content, "turned off after this session started") {
		t.Fatalf("an auto-skill write must be refused: %q", r.Content)
	}
	// A hand-written skill edit passes (unvalidated, like any write off).
	sk := filepath.Join(c.opts.Env.Root, ".moca", "skills", "deploy")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "SKILL.md"), []byte("---\nname: deploy\ndescription: d\n---\nprose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTool(t, c, "read", `{"path":".moca/skills/deploy/SKILL.md"}`)
	r = runTool(t, c, "write", writeArgs(".moca/skills/deploy/SKILL.md", "---\nname: deploy\ndescription: d\n---\nnew prose\n"))
	if r.IsError {
		t.Fatalf("hand-written edits pass in this state: %q", r.Content)
	}
}

// A session resumed days after it started still carries its start date in
// the frozen prompt: the provenance check must accept that date (the guard's
// SessionDate is the header's StartedAt, not the resume time).
func TestResumeGuardAcceptsTheSessionStartDate(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	path := a.Session().Path()
	cfg := a.opts.Config
	a.Session().Close()

	started := time.Now().AddDate(0, 0, -5)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	head, rest, _ := strings.Cut(string(data), "\n")
	var hdr map[string]any
	if err := json.Unmarshal([]byte(head), &hdr); err != nil {
		t.Fatal(err)
	}
	sess, _ := hdr["session"].(map[string]any)
	if sess == nil {
		t.Fatalf("no session header in %q", head)
	}
	sess["startedAt"] = started.Format(time.RFC3339Nano)
	nh, _ := json.Marshal(hdr)
	if err := os.WriteFile(path, []byte(string(nh)+"\n"+rest), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := Resume(StartOptions{Config: cfg, Workdir: session.WorkdirOf(path), Trusted: true, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	d := started.Format("2006-01-02")
	r := runTool(t, b, "write", writeArgs(".moca/skills/moca-x/SKILL.md",
		autoFile("moca-x", "d", "- **ra**: run the suite with -count=1 (×1, "+d+")\n")))
	if r.IsError {
		t.Fatalf("the frozen prompt's date must be accepted after a resume: %q", r.Content)
	}
}

// Resumed UNTRUSTED (the approval was declined for this process): the switch
// is on, but C5b must still hold — the dir resolves with or without trust,
// the refusing guard exists, and an auto-skill write is REFUSED, not landed
// unvalidated through a dead one.
func TestResumeUntrustedInstallsAReachingRefusingGuard(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	path := a.Session().Path()
	a.Session().Close()

	d, err := Resume(StartOptions{Config: a.opts.Config, Workdir: session.WorkdirOf(path), Trusted: false, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Session().Close()
	if !d.opts.Env.SkillGuardRefusing || d.opts.Env.SkillGuard == nil || d.opts.Env.SkillDir == "" {
		t.Fatalf("an untrusted resume of a blocked session installs a REACHING refusing guard: refusing=%v dir=%q",
			d.opts.Env.SkillGuardRefusing, d.opts.Env.SkillDir)
	}
	r := runTool(t, d, "write", writeArgs(".moca/skills/moca-x/SKILL.md", autoFile("moca-x", "d", autoBody("rule"))))
	if !r.IsError || !strings.Contains(r.Content, "turned off after this session started") {
		t.Fatalf("an auto-skill write must be refused on the untrusted resume: %q", r.Content)
	}
}

func TestResumeOfAFeatureOffSessionStaysOff(t *testing.T) {
	// Started with the switch off (no block in the frozen prompt), resumed
	// after it went on: the session stays off — no guard, no /remember, no
	// shell refusal — and the warning says so (SPECS §12's resume rule).
	s := newScript(t, textTurn("ok"))
	a, _, _ := startWith(t, s, 40, "", "", func(o *StartOptions) { o.Trusted = true })
	path := a.Session().Path()
	on := a.opts.Config
	a.Session().Close()
	yes := true
	on.Skills.SelfManaged = &yes
	var evs []Event
	b, err := Resume(StartOptions{Config: on, Workdir: session.WorkdirOf(path), Trusted: true,
		Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	if b.SelfManagedSkills() || b.opts.Env.SkillGuard != nil || b.opts.Env.SkillDir != "" || b.opts.Env.SkillCold {
		t.Fatal("a session that started without the block must not learn after a resume")
	}
	if !slices.ContainsFunc(warningTexts(&evs), func(x string) bool { return strings.Contains(x, "stay off in this resumed session") }) {
		t.Fatalf("one warning about the mismatch: %v", warningTexts(&evs))
	}
}

func TestRememberRefusesEmptyTextAndPlanMode(t *testing.T) {
	a, _, _ := startManaged(t, newScript(t))
	defer a.Session().Close()
	if _, err := a.Remember(context.Background(), "  "); err == nil || !strings.Contains(err.Error(), "needs the rule") {
		t.Fatalf("an empty /remember must not start a user-signal run: %v", err)
	}
	a.SetPlan(true)
	if _, err := a.Remember(context.Background(), "use make test"); err == nil || !strings.Contains(err.Error(), "plan mode is on") {
		t.Fatalf("/remember in plan mode must refuse before the run: %v", err)
	}
	if a.opts.Env.SkillUserSignal {
		t.Fatal("a refused /remember never arms the signal")
	}
}

func TestStartOmitsBlockOnResolveError(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	a, _, _ := startManaged(t, newScript(t), func(o *StartOptions) {
		if err := os.Symlink(outer, filepath.Join(o.Workdir, ".moca")); err != nil {
			t.Fatal(err)
		}
	})
	defer a.Session().Close()
	if strings.Contains(a.opts.System, "Project skills you maintain") {
		t.Fatal("no block without a resolved dir (C5a)")
	}
	head, err := session.ReadFile(a.Session().Path())
	if err != nil {
		t.Fatal(err)
	}
	if head[0].Session.SelfManagedSkills {
		t.Fatal("the header must record what the prompt carries")
	}
}

// --- run options: typed text and the user signal ---------------------------

func TestRememberSetsSignalAndTyped(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"shell", `{"command":"go version"}`}), textTurn("ok"),
		toolTurn([2]string{"shell", `{"command":"go version"}`}), textTurn("ok"))
	a, _, _ := startManaged(t, s)
	defer a.Session().Close()
	// A plain run carries neither.
	if _, err := a.Run(context.Background(), "remember to do things"); err != nil {
		t.Fatal(err)
	}
	if a.opts.Env.SkillUserSignal {
		t.Fatal("a plain run never sets the user signal (C4)")
	}
	if len(a.opts.Env.SkillTyped) != 0 {
		t.Fatalf("Run carries no typed text: %v", a.opts.Env.SkillTyped)
	}
	// /remember carries both.
	out, err := a.Remember(context.Background(), "always use -count=1")
	if err != nil || out.Text != "ok" {
		t.Fatal(out, err)
	}
	if !a.opts.Env.SkillUserSignal {
		t.Fatal("Agent.Remember is the only setter of SkillUserSignal")
	}
	if len(a.opts.Env.SkillTyped) != 1 || a.opts.Env.SkillTyped[0] != "always use -count=1" {
		t.Fatalf("typed = %v", a.opts.Env.SkillTyped)
	}
	bodies := s.bodies
	if len(bodies) == 0 {
		t.Fatal("no request recorded")
	}
	// The expansion is what the model sees.
	last := bodies[len(bodies)-1]
	if !strings.Contains(fmt.Sprint(last), "Remember this project rule: always use -count=1") {
		t.Fatal("the run must carry the expansion")
	}
}

func TestTypedTextFailClosed(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	defer a.Session().Close()
	if _, err := a.Run(context.Background(), "fix the deploy skill"); err != nil {
		t.Fatal(err)
	}
	if len(a.opts.Env.SkillTyped) != 0 {
		t.Fatal("Run(ctx, prompt) carries no typed text (fail-closed)")
	}
	if _, err := a.RunWith(context.Background(), "fix the deploy skill", RunOpts{Typed: "fix the deploy skill"}); err != nil {
		t.Fatal(err)
	}
	if len(a.opts.Env.SkillTyped) != 1 {
		t.Fatalf("RunWith must carry it: %v", a.opts.Env.SkillTyped)
	}
}

func TestSteeringCarriesTypedAndArmsTheHint(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	defer a.Session().Close()
	a.Steer("fix the deploy skill too", "fix the deploy skill too")
	if _, err := a.Run(context.Background(), "do something"); err != nil {
		t.Fatal(err)
	}
	if len(a.opts.Env.SkillTyped) != 1 || a.opts.Env.SkillTyped[0] != "fix the deploy skill too" {
		t.Fatalf("a steering message's typed part must reach the guard: %v", a.opts.Env.SkillTyped)
	}
	if !a.opts.Env.SkillHintArmed {
		t.Fatal("any steering arms the hint")
	}
	if a.opts.Env.SkillUserSignal {
		t.Fatal("steering never sets the user signal")
	}
}

func TestPromptArmsHintOnlyNotSignal(t *testing.T) {
	s := newScript(t, textTurn("ok"))
	a, _, _ := startManaged(t, s)
	defer a.Session().Close()
	if _, err := a.RunWith(context.Background(), "remember this forever", RunOpts{Typed: "remember this forever"}); err != nil {
		t.Fatal(err)
	}
	if a.opts.Env.SkillUserSignal {
		t.Fatal("word-matching the prompt never sets the signal (C4)")
	}
	if !a.opts.Env.SkillHintArmed {
		t.Fatal("remember-class typed text arms the hint")
	}
}

func TestRememberRequestText(t *testing.T) {
	req := RememberRequest("use -count=1")
	if !strings.HasPrefix(req, "Remember this project rule: use -count=1") || !strings.Contains(req, "(user, <today's date>)") {
		t.Fatalf("%q", req)
	}
	if !mentionsRemembering("please remember this") || !mentionsRemembering("From now on, do X") {
		t.Fatal("the hint trigger must match")
	}
	if mentionsRemembering("I remembered it") {
		t.Fatal("a word-bounded match must not fire on \"remembered\"")
	}
	if mentionsRemembering("") {
		t.Fatal("empty text never fires")
	}
}

// The "no block ⇒ no hint" end-to-end pin lives with the tools that decide it
// (internal/tools: TestArmedHintNeedsAValidatingGuard — armed by hand, silent
// with no guard and with C5b's refusing one, delivered only under a validating
// guard, with that delivery as the non-vacuity control). The predicate itself
// is pinned by tools.TestSkillLearningPredicate; the test that used to sit here
// only re-asserted that predicate on an empty Env and pinned nothing.
