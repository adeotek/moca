package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/session"
)

func TestSetModeSwapsCheckers(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)

	a.SetMode(ModeAuto)
	if _, ok := a.opts.Env.Commands.(*permissions.Shell); !ok || a.opts.Env.Judge == nil {
		t.Fatal("auto: shell checker + judge")
	}
	a.SetMode(ModeYolo)
	if _, ok := a.opts.Env.Commands.(permissions.AllowAll); !ok || a.opts.Env.Judge != nil {
		t.Fatal("yolo: AllowAll, no judge")
	}
	a.SetMode(ModeDefault)
	if a.opts.Env.Judge != nil || a.Yolo() {
		t.Fatal("default")
	}
	// session entries record both flags and the mode
	last := a.entries[len(a.entries)-1].PermissionMode
	if last.Mode != "default" || last.Yolo || last.Plan {
		t.Fatalf("entry %+v", last)
	}
}

// TestSetModeAutoAfterYoloKeepsPathChecks: yolo overwrites Env.Paths with
// Unjailed, so an auto swap that derived the path checker from the *current*
// Env would nil the path checks out. The auto path checker must come from
// the cache built at New() (from Env.AutoJail), so after yolo→auto the
// write/check area is a Jail again — the jail widened by the temp roots.
func TestSetModeAutoAfterYoloKeepsPathChecks(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)
	a.SetMode(ModeYolo)
	if _, ok := a.opts.Env.Paths.(*permissions.Unjailed); !ok {
		t.Fatal("yolo must unjail the paths first")
	}
	a.SetMode(ModeAuto)
	j, ok := a.opts.Env.Paths.(*permissions.Jail)
	if !ok {
		t.Fatalf("auto after yolo: Env.Paths is %T, want *permissions.Jail", a.opts.Env.Paths)
	}
	// The jail's write area covers the workdir (path checks armed again)
	// and refuses a write outside it and the temp roots.
	if _, err := j.Resolve(filepath.Join(a.opts.Env.Root, "in-jail.txt"), true); err != nil {
		t.Errorf("jail refuses writes to the workdir: %v", err)
	}
	// sanity check on TempRoots: non-empty; the jail widened by them must
	// accept a write under the first root (which sits outside the workdir).
	roots := permissions.TempRoots()
	if len(roots) == 0 {
		t.Fatal("no temp roots configured")
	}
	if _, err := j.Resolve(filepath.Join(roots[0], "in-auto-writable.txt"), true); err != nil {
		t.Errorf("temp-root write refused in auto: %v", err)
	}
}

// Status().Yolo must follow the mode: the pre-Task-7 yolo bool field is gone,
// so a Status() reading it would silently report false after /yolo on.
func TestStatusYoloFollowsMode(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)
	a.SetMode(ModeYolo)
	if !a.Status().Yolo {
		t.Fatal("Status().Yolo is false after SetMode(ModeYolo)")
	}
	a.SetMode(ModeDefault)
	if a.Status().Yolo {
		t.Fatal("Status().Yolo is true after SetMode(ModeDefault)")
	}
}

// Plan rides only on default: turning plan on in a wider mode leaves it
// for default first (its own permission_mode entry), so an entry never
// records a plan flag beside auto or yolo.
func TestSetPlanKeepsMode(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40)
	a.SetMode(ModeAuto)
	a.SetPlan(true)
	if got := a.Mode(); got != ModeDefault {
		t.Fatalf("SetPlan(auto) must land on default, got %v", got)
	}
	var got []session.PermissionMode
	for _, e := range a.entries {
		if e.Type == session.TypePermissionMode {
			got = append(got, *e.PermissionMode)
		}
	}
	want := []session.PermissionMode{
		{Mode: "auto"},
		{Mode: "default"},             // SetPlan leaves auto first
		{Plan: true, Mode: "default"}, // then the plan entry
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestProjectRulesReachTheShell(t *testing.T) {
	a, _, _ := startTest(t, newScript(t), 40, func(o *StartOptions) {
		o.Trusted = true
		o.Config.Project = &config.ProjectConfig{Permissions: config.PermissionsConfig{Deny: []string{"git push *"}}}
	})
	if _, _, _, err := a.opts.Env.Commands.Check("git push x"); err == nil {
		t.Fatal("project deny ignored")
	}
}

// TestYoloTurnsEveryCheckOff (moved from yolo_test.go): a yolo start runs
// every shell call and writes anywhere, while the read-before-write guard
// stays on; toggling back restores the checks.
func TestYoloTurnsEveryCheckOff(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "out.txt")
	s := newScript(t,
		toolTurn(
			[2]string{"shell", `{"command":"uname"}`},                               // not allowlisted
			[2]string{"shell", `{"command":"eval echo hi"}`},                        // refused by analysis
			[2]string{"shell", `{"command":"chown $(id -u) a.txt"}`},                // hard-deny, harmless here
			[2]string{"write", fmt.Sprintf(`{"path":%q,"content":"x"}`, outside)},   // outside the jail
			[2]string{"edit", `{"path":"a.txt","old_string":"a","new_string":"b"}`}, // never read: guard stays on
		),
		textTurn("done"),
		toolTurn([2]string{"shell", `{"command":"uname"}`}),
		textTurn("refused"),
	)
	a, work, evs := startTest(t, s, 40, func(o *StartOptions) { o.Mode = ModeYolo })
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("a\n"), 0o644)
	if _, err := a.Run(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	var ends []ToolEnd
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok {
			ends = append(ends, te)
		}
	}
	for i, te := range ends[:4] {
		if te.Result.IsError {
			t.Errorf("call %d must run under yolo: %s", i, te.Result.Content)
		}
	}
	if !ends[4].Result.IsError || !strings.Contains(ends[4].Result.Content, "read it first") {
		t.Fatal("read-before-write guard is not a permission; it stays on")
	}
	if b, _ := os.ReadFile(outside); string(b) != "x" {
		t.Fatal("write outside the workdir")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if !entries[0].Session.Yolo {
		t.Fatal("header records yolo")
	}

	a.SetYolo(false)
	*evs = nil
	a.Run(context.Background(), "again")
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && !te.Result.IsError {
			t.Fatal("checks are back after /yolo off")
		}
	}
	entries, _ = session.ReadFile(a.Session().Path())
	var modes int
	for _, e := range entries {
		if e.Type == session.TypePermissionMode {
			modes++
		}
	}
	if modes != 1 {
		t.Fatal("toggle writes one permission_mode entry")
	}
}

// A start with plan and a wider mode together (a caller that bypassed
// cmd's usage check) starts in default + plan: plan rides only on default.
func TestStartModeNeverWiderThanPlan(t *testing.T) {
	cases := []struct {
		o    StartOptions
		want PermMode
	}{
		{StartOptions{}, ModeDefault},
		{StartOptions{Mode: ModeYolo}, ModeYolo},
		{StartOptions{Mode: ModeYolo, Plan: true}, ModeDefault},
		{StartOptions{Mode: ModeAuto, Plan: true}, ModeDefault},
	}
	for _, c := range cases {
		if got := c.o.mode(); got != c.want {
			t.Errorf("%+v → %v, want %v", c.o, got, c.want)
		}
	}
}
