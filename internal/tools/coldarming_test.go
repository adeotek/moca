package tools

// Pin for PR C: cold-repo arming needs a *verification-shaped* command.
// `go version` and `npm install` are toolchain setup/inspection — a cold repo's
// first green one used to arm the recording hint with a claim the run never
// established ("this is how the repo is checked", open item 8).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestColdArmingNeedsAVerificationShapedCommand(t *testing.T) {
	env, root := testEnv(t)
	setSkillDir(t, env, root)
	env.SkillGuard = func(abs, rel string, data []byte) error { return nil }
	env.Commands = &fakeCmds{}
	reg := NewRegistry(writeTool{}, shellTool{})

	call := func(id, cmd string) llm.ToolCall {
		return llm.ToolCall{ID: id, Name: "shell", Input: []byte(fmt.Sprintf(`{"command":%q}`, cmd))}
	}

	for i, cmd := range []string{"go version", "go env", "npm install", "cargo fetch"} {
		env.BeginRun()
		env.SkillCold = true
		res := reg.Run(t.Context(), env, call(fmt.Sprint(i), cmd))
		if env.SkillHintArmed {
			t.Errorf("%q armed the cold hint (%q); setup/inspection is not a verification", cmd, res.Content)
		}
	}

	// The verification-shaped control still arms, and names what it proved.
	// `make test` really succeeds here (a Makefile with a no-op test target):
	// a command that fails cannot arm, so the control needs a green one.
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("test:\n	@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.BeginRun()
	env.SkillCold = true
	reg.Run(t.Context(), env, call("ok", "make test"))
	if !env.SkillHintArmed {
		t.Fatal("a verification-shaped command must arm the cold hint")
	}
	if env.SkillProvenCmd != "make test" {
		t.Errorf("SkillProvenCmd = %q, want the command that armed it", env.SkillProvenCmd)
	}

	// First proof wins: a later verifying call does not overwrite it.
	reg.Run(t.Context(), env, call("later", "make build"))
	if env.SkillProvenCmd != "make test" {
		t.Errorf("SkillProvenCmd = %q after a later call; the first proof must win", env.SkillProvenCmd)
	}
	if strings.Contains(env.SkillProvenCmd, "build") {
		t.Errorf("SkillProvenCmd = %q, want the first proof", env.SkillProvenCmd)
	}
}
