package tui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

// newManagedModel builds a model around a real agent started with (or
// without) the self-managed switch, in a trusted workdir.
func newManagedModel(t *testing.T, selfManaged bool) *model {
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
	top := ""
	if selfManaged {
		top = `,"skills":{"selfManaged":true}`
	}
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"fake/m"%s,"providers":{"fake":{"baseUrl":%q,
		"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY",
		"models":{"m":{"contextWindow":65536}}}}}`, top, srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(cfgPath, []byte("{\"model\":\"fake/m\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return newAgentModelCfg(t, agent.StartOptions{Config: cfg, Workdir: t.TempDir(), Trusted: selfManaged,
		HTTP: srv.Client(), Slug: "t", Ask: func(_ context.Context, _ tools.Question) tools.Answer { return tools.Deny }}, cfgPath)
}

func TestRememberOffNotice(t *testing.T) {
	m := newManagedModel(t, false)
	out := simulateAll(m, tuiCmd(t, m, "/remember use -count=1"))
	if !strings.Contains(out, "self-managed skills are off for this session") || !strings.Contains(out, "restart moca") {
		t.Fatalf("the off notice must name the config and the restart:\n%s", out)
	}
	if m.running {
		t.Fatal("no run may start")
	}
}

func TestRememberUsageAndPlanRefusal(t *testing.T) {
	m := newManagedModel(t, true)
	if out := simulateAll(m, tuiCmd(t, m, "/remember")); !strings.Contains(out, "usage: /remember <text>") {
		t.Fatalf("usage line:\n%s", out)
	}
	m.agent.SetPlan(true)
	out := simulateAll(m, tuiCmd(t, m, "/remember use -count=1"))
	if !strings.Contains(out, "plan mode is on") || m.running {
		t.Fatalf("plan mode must refuse it:\n%s", out)
	}
	m.agent.SetPlan(false)
}

func TestRememberMidRunRefused(t *testing.T) {
	m := newManagedModel(t, true)
	m.running = true
	out := simulateAll(m, tuiCmd(t, m, "/remember use -count=1"))
	if !strings.Contains(out, "finish or interrupt the run first") {
		t.Fatalf("a run in progress refuses it:\n%s", out)
	}
}

func TestHelpListsRememberAndAutoSkills(t *testing.T) {
	h := HelpText(nil, nil)
	if !strings.Contains(h, "/remember") || !strings.Contains(h, "record <text> as a project skill") {
		t.Fatalf("/help must list the built-in:\n%s", h)
	}
	sk := []skills.Skill{
		{Name: "moca-testing", Description: "learned rules", Auto: true},
		{Name: "deploy", Description: "ship it"},
	}
	h = HelpText(nil, sk)
	if !strings.Contains(h, "auto skills: moca-testing") {
		t.Fatalf("auto skills need their own line:\n%s", h)
	}
	if strings.Contains(h, "/moca-testing") {
		t.Fatalf("an auto skill must not be listed as a command:\n%s", h)
	}
	if !strings.Contains(h, "/deploy") {
		t.Fatalf("hand-written skills stay commands:\n%s", h)
	}
}

func TestAutoSkillsNotSlashCommands(t *testing.T) {
	m := newManagedModel(t, true)
	sk := filepath.Join(m.start.Workdir, ".moca", "skills", "moca-testing")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sk, "SKILL.md"),
		[]byte("---\nname: moca-testing\ndescription: d\nauto: true\n---\n- **a**: b (×1, 2026-10-18)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.agent.ReloadSkills()
	if _, err := ParseInput("/moca-testing", m.opts.Prompts, m.agentSkills()); err == nil {
		t.Fatal("an auto skill must not resolve as a slash command")
	}
	if slices.ContainsFunc(m.commandHints(), func(d dropItem) bool { return d.name == "moca-testing" }) {
		t.Fatalf("the dropdown must not offer it: %+v", m.commandHints())
	}
	// /help still lists it, on the auto line.
	if out := simulateAll(m, tuiCmd(t, m, "/help")); !strings.Contains(out, "auto skills: moca-testing") {
		t.Fatalf("/help lists auto skills on their own line:\n%s", out)
	}
	// It stays in the agent's list (the system prompt's listing is how the
	// model finds it).
	if !slices.ContainsFunc(m.allSkills(), func(s skills.Skill) bool { return s.Name == "moca-testing" && s.Auto }) {
		t.Fatal("the agent must still know the auto skill")
	}
}

func TestTypedTextForTemplatesIsArgs(t *testing.T) {
	rows := []struct {
		in   Parsed
		want string
	}{
		{Parsed{Kind: KindText, Text: "fix the deploy skill"}, "fix the deploy skill"},
		{Parsed{Kind: KindPrompt, Name: "tpl", Args: "prod", Text: "do prod now"}, "prod"},
		{Parsed{Kind: KindSkill, Name: "deploy", Args: "prod", Text: "Load and follow the deploy skill"}, "prod"},
		{Parsed{Kind: KindPrompt, Name: "tpl", Text: "do now"}, ""},
	}
	for _, r := range rows {
		if got := typedText(r.in); got != r.want {
			t.Errorf("typedText(%+v) = %q, want %q", r.in, got, r.want)
		}
	}
	// A mid-run submission is steered with the same pair.
	m := newManagedModel(t, true)
	m.running = true
	m.input.Insert("/tpl prod")
	m.syncTextarea()
	m.opts.Prompts = []skills.Prompt{{Name: "tpl", Body: "do $ARGUMENTS"}}
	if cmd := m.submit(); cmd == nil {
		t.Fatal("a queued submission returns a command")
	}
	if m.agent.PendingSteering() != 1 {
		t.Fatalf("queued = %d, want 1", m.agent.PendingSteering())
	}
	if got := m.agent.TakeSteering(); len(got) != 1 || got[0] != "do prod" {
		t.Fatalf("the transcript text is the expansion: %v", got)
	}
}

func TestNoteRendersMuted(t *testing.T) {
	m := newTestModel()
	cmd := m.handleAgent(agent.Note{Text: "self-managed skills: 1 skill in this project (moca-x)"})
	out := drained(m, cmd)
	if !strings.Contains(out, "self-managed skills: 1 skill") {
		t.Fatalf("the note must print:\n%s", out)
	}
	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}); cmd != nil {
		t.Fatal("a resize needs no command")
	}
}
