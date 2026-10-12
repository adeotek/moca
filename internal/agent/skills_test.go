package agent

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/skills"
)

// writeSkillFile writes a minimal valid SKILL.md under dir/<name>/.
func writeSkillFile(t *testing.T, dir, name, desc string) {
	t.Helper()
	d := filepath.Join(dir, name)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: " + desc + "\n---\n" + name + " body\n"
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func skillNames(sk []skills.Skill) []string {
	var out []string
	for _, s := range sk {
		out = append(out, s.Name)
	}
	return out
}

// TestStartedAgentExposesFilteredSkills: a started agent carries the
// discovered skill list (the TUI's picker and /help read it) — and the
// built-in rtk skill is listed only when its CLI is on PATH.
func TestStartedAgentExposesFilteredSkills(t *testing.T) {
	defer func(orig func(string) bool) { toolOnPath = orig }(toolOnPath)

	toolOnPath = func(name string) bool { return name == "rtk" }
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	if !slices.Contains(skillNames(a.Skills()), "rtk") {
		t.Fatalf("rtk must be listed when its CLI is on PATH: %v", skillNames(a.Skills()))
	}
	a.Session().Close()

	toolOnPath = func(string) bool { return false }
	b, _, _ := startTestWith(t, s, "", "")
	if slices.Contains(skillNames(b.Skills()), "rtk") {
		t.Fatalf("rtk must be filtered without its CLI: %v", skillNames(b.Skills()))
	}
	b.Session().Close()
}

// TestResumedAgentExposesSkills: Resume shares prepare's discovery, so a
// resumed agent's picker list is populated too (it used to be built by
// Start only).
func TestResumedAgentExposesSkills(t *testing.T) {
	defer func(orig func(string) bool) { toolOnPath = orig }(toolOnPath)
	toolOnPath = func(name string) bool { return name == "rtk" }

	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	path := a.Session().Path()
	a.Session().Close()
	b, err := Resume(StartOptions{Config: a.opts.Config, Workdir: session.WorkdirOf(path), HTTP: s.srv.Client()}, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Session().Close()
	if !slices.Contains(skillNames(b.Skills()), "rtk") {
		t.Fatalf("a resumed agent must carry the skill list: %v", skillNames(b.Skills()))
	}
}

// TestProjectSkillsNeedTrustAndReloadSkillsPicksUpNewDirs: the project
// skills dir joins only for a trusted workdir, and ReloadSkills sees a
// directory created after start (the self-configure / create-command
// reload path), global dirs included.
func TestProjectSkillsNeedTrustAndReloadSkillsPicksUpNewDirs(t *testing.T) {
	defer func(orig func(string) bool) { toolOnPath = orig }(toolOnPath)
	toolOnPath = func(string) bool { return false }

	s := newScript(t)
	// Untrusted: the workdir's .moca/skills is not in the dir list.
	u, uwork, _ := startTestWith(t, s, "", "")
	writeSkillFile(t, filepath.Join(uwork, ".moca", "skills"), "proj", "project skill")
	u.ReloadSkills()
	if slices.Contains(skillNames(u.Skills()), "proj") {
		t.Fatalf("an untrusted workdir's project skills must not join: %v", skillNames(u.Skills()))
	}
	u.Session().Close()

	// Trusted: discovered at start and after a mid-session addition.
	tr, twork, _ := startWith(t, s, 40, "", "", func(o *StartOptions) { o.Trusted = true })
	writeSkillFile(t, filepath.Join(twork, ".moca", "skills"), "proj", "project skill")
	tr.ReloadSkills()
	if !slices.Contains(skillNames(tr.Skills()), "proj") {
		t.Fatalf("a trusted workdir's project skill must reload: %v", skillNames(tr.Skills()))
	}
	writeSkillFile(t, filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moca", "skills"), "glob", "global skill")
	tr.ReloadSkills()
	if !slices.Contains(skillNames(tr.Skills()), "glob") {
		t.Fatalf("a global skill added after start must reload: %v", skillNames(tr.Skills()))
	}
	tr.Session().Close()
}
