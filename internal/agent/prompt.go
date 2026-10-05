// Package agent runs the loop: user message → model → tool calls → results
// → repeat. It is the only package that imports all the others (§2).
package agent

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/adeotek/moca/internal/skills"
)

type ServerLine struct{ Name, Description string }

type PromptInput struct {
	Workdir, OS, Arch, Date, Git, Version string
	Skills                                []skills.Skill
	Servers                               []ServerLine
	Instructions                          []skills.Instruction
}

const coreTemplate = `You are moca, a coding agent working in a user's repository through tools.

# Environment
- Workdir: %s (all relative paths resolve here; file tools cannot leave it)
- Platform: %s %s
- Date: %s
- Git at session start: %s (may be stale; check with git when it matters)

# Tools
- read before you write or edit an existing file; edit refuses files you have not read or that changed since.
- Prefer edit (small exact replacements) over write for existing files. Copy old_string without read's N| prefixes.
- read pages large files: use offset/limit instead of re-reading whole files.
- search finds code (RE2 regex, respects .gitignore); ls lists one directory.
- ` + "`shell` is stateless" + `: every call starts in the workdir. Use ` + "`cd dir && cmd`" + ` in one call.
- shell commands are checked against an allowlist. If one is refused, do not retry variants that
  do the same thing (find -delete, python -c …); explain what you need and ask the user.
- Tool calls in one turn run in order; a failed call does not stop the rest.
- mcp gives access to the MCP servers listed below: search, then describe, then call.

# Working style
- Do the task end to end: understand, change, verify (build/tests), then report briefly.
- Keep changes minimal and in the style of the surrounding code. Don't add unrequested features.
- When something fails, read the error and fix the cause; don't loop on the same failing call.
- Final answer: what changed, how it was verified, anything left open. No filler.

# Token discipline
- Every tool result costs tokens on every later turn. Read windows, not whole files; search before reading.
- Prefer rtk-prefixed variants where they exist (e.g. ` + "`rtk git status`, `rtk test -- go test ./...`" + `).
- Don't echo file contents or tool output back to the user; summarize.

moca %s`

func BuildSystemPrompt(in PromptInput) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, coreTemplate, in.Workdir, in.OS, in.Arch, in.Date, in.Git, in.Version)
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
