// Package agent runs the loop: user message → model → tool calls → results
// → repeat. It is the only package that imports all the others (§2).
package agent

import (
	_ "embed"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/adeotek/moca/internal/skills"
)

type ServerLine struct{ Name, Description string }

type PromptInput struct {
	Workdir, OS, Arch, Date, Git, Version string
	RTK                                   bool // rtk is on PATH: only then is it advertised
	// Verify/VerifySource: the project's detected check command (DetectVerify)
	// and the file it came from; "" omits the line.
	Verify, VerifySource string
	Skills               []skills.Skill
	Servers              []ServerLine
	Instructions         []skills.Instruction
}

// The fixed text of the system prompt lives in system-prompt.md so it reads
// and reviews as prose (no Go string escaping). {{token}} slots are resolved
// once, at session start, by renderPrompt — the slot set is pinned by
// TestPromptTemplateSlots; a new slot needs an entry there and in renderPrompt.
//
//go:embed system-prompt.md
var embeddedTemplate string

// promptTemplate trims the file's trailing newline: the markdown file ends
// with one (editors, diffs), the prompt text must not.
var promptTemplate = strings.TrimRight(embeddedTemplate, "\n")

// renderPrompt resolves every {{token}} in one NewReplacer pass — replacement
// values containing token-looking text are never re-scanned.
func renderPrompt(in PromptInput, verify, rtk string) string {
	return strings.NewReplacer(
		"{{workdir}}", in.Workdir,
		"{{os}}", in.OS,
		"{{arch}}", in.Arch,
		"{{date}}", in.Date,
		"{{git}}", in.Git,
		"{{verify}}", verify,
		"{{rtk}}", rtk,
		"{{version}}", in.Version,
	).Replace(promptTemplate)
}

func BuildSystemPrompt(in PromptInput) string {
	var sb strings.Builder
	rtk := ""
	if in.RTK {
		rtk = "- Prefer rtk-prefixed variants for shell command output where they exist (e.g. `rtk git status`, `rtk test -- go test ./...`).\n"
	}
	verify := ""
	if in.Verify != "" {
		verify = fmt.Sprintf("- Project checks: `%s` (from %s) — run them after changes\n", in.Verify, in.VerifySource)
	}
	sb.WriteString(renderPrompt(in, verify, rtk))
	if len(in.Skills) > 0 {
		sb.WriteString("\n\n# Skills\nLoad a skill's instructions with read on its path when the task matches.\n")
		for _, s := range in.Skills {
			fmt.Fprintf(&sb, "- %s: %s (%s)\n", s.Name, oneLine(s.Description), s.Path)
		}
	}
	if len(in.Servers) > 0 {
		sb.WriteString("\n# MCP servers (use the mcp tool)\n")
		for _, s := range in.Servers {
			if s.Description != "" {
				fmt.Fprintf(&sb, "- %s: %s\n", s.Name, s.Description)
			} else {
				fmt.Fprintf(&sb, "- %s\n", s.Name)
			}
		}
	}
	for _, i := range in.Instructions {
		fmt.Fprintf(&sb, "\n# Project instructions (%s)\n%s\n", i.Path, strings.TrimSpace(i.Content))
	}
	return sb.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// toolOnPath reports whether an external helper is installed (indirected for
// tests): the prompt only advertises token-savers the shell can actually run.
var toolOnPath = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// builtinTools maps an embedded builtin skill to the external CLI it
// instructs; a host without that CLI must not be told to use it (the first
// live ship-gate run on a host without rtk derailed on `rtk: command not
// found` and never read the failing test).
var builtinTools = map[string]string{"rtk": "rtk"}

// filterSkills drops builtin skills whose external tool is not installed.
// Project/global skills are never filtered: they are the user's own files.
func filterSkills(sk []skills.Skill) []skills.Skill {
	out := make([]skills.Skill, 0, len(sk))
	for _, s := range sk {
		if s.Source == "builtin" {
			if tool, ok := builtinTools[s.Name]; ok && !toolOnPath(tool) {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

func Platform() (string, string) {
	osName := map[string]string{"linux": "Linux", "darwin": "Darwin", "windows": "Windows"}[runtime.GOOS]
	if osName == "" {
		osName = runtime.GOOS
	}
	arch := map[string]string{"amd64": "x86_64", "386": "i686"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	return osName, arch
}

func GitState(dir string) string {
	branch, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "not a git repository"
	}
	s := "branch " + strings.TrimSpace(string(branch))
	if st, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output(); err == nil {
		if n := len(strings.Split(strings.TrimSpace(string(st)), "\n")); strings.TrimSpace(string(st)) != "" {
			s += fmt.Sprintf(", %d uncommitted changes", n)
		}
	}
	return s
}
