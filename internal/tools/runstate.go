package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

// Per-run state (§10/§14): what one agent run changed, whether it verified
// the change since, and which calls keep failing identically. The agent calls
// BeginRun at the start of every run; the tools update the rest.

// repeatHintAt is the identical-failure count from which a result carries
// the change-approach hint; the agent stops a run at RepeatStopAt.
const (
	repeatHintAt = 3
	RepeatStopAt = 5
)

// BeginRun resets the per-run state.
func (e *Env) BeginRun() {
	e.PlanWrote, e.Edited, e.Unverified = false, false, ""
	e.failures, e.MaxRepeat = nil, 0
	e.copied = nil
	e.SkillCreations, e.SkillUserSignal, e.SkillTyped = 0, false, nil
	e.SkillHintArmed, e.SkillHintDone, e.lastFailedVerify = false, false, ""
}

// markCopied records a successful byte copy of abs (the SQLite sidecar guard
// then stops refusing opens of it, §10).
func (e *Env) markCopied(abs ...string) {
	if len(abs) == 0 {
		return
	}
	if e.copied == nil {
		e.copied = map[string]struct{}{}
	}
	for _, p := range abs {
		e.copied[p] = struct{}{}
	}
}

func (e *Env) copiedPath(abs string) bool {
	_, ok := e.copied[abs]
	return ok
}

// noteChange records a successful write/edit of abs: the run has changed
// files (a later red test run is its own work in progress, not a failure to
// investigate), a code file now needs verification, and earlier identical
// failures no longer count as repeats (the state they ran against is gone).
func noteChange(env *Env, abs string) {
	env.Edited = true
	env.failures = nil
	// An edit between a failing verification and the next green run means the
	// fix was in the code, not in how it is run: no recovery trigger.
	env.lastFailedVerify = ""
	if !isDocPath(abs) {
		rel, err := filepath.Rel(env.Root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = abs
		}
		env.Unverified = filepath.ToSlash(rel)
	}
}

func isDocPath(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".md", ".markdown", ".txt", ".rst", ".adoc":
		return true
	}
	return false
}

// inspectOnly are commands that look at the tree without exercising the
// code: running only these after an edit is not verification.
var inspectOnly = map[string]bool{"cat": true, "ls": true, "head": true, "tail": true, "grep": true, "rg": true,
	"find": true, "echo": true, "printf": true, "wc": true, "git": true, "pwd": true, "which": true, "sed": true,
	"awk": true, "diff": true, "stat": true, "file": true, "tree": true, "cd": true, "true": true, "sort": true,
	"uniq": true, "cut": true, "tr": true, "less": true, "more": true, "type": true, "Get-Content": true,
	"Select-String": true, "Get-ChildItem": true}

// verifies reports whether a shell command line exercises the code (build,
// test, lint, run): any pipeline segment whose command is not inspect-only.
// Generous on purpose — the nudge it feeds must not fire on real checks.
func verifies(command string) bool {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	for _, seg := range strings.Split(r.Replace(command), "\n") {
		f := strings.Fields(seg)
		for len(f) > 0 && (strings.Contains(f[0], "=") || f[0] == "rtk" || f[0] == "sudo" || f[0] == "time" || f[0] == "--") {
			f = f[1:]
		}
		if len(f) > 0 && !inspectOnly[f[0]] {
			return true
		}
	}
	return false
}

// spillPathRe matches the overflow file named in a Spill note: its name is
// random per call, so it must not make identical failures look different.
var spillPathRe = regexp.MustCompile(`saved to \S+ — `)

// normCommand is the whitespace-collapsed form of a command line — the
// comparison form for "did the command change?" (verifies() applies the same
// collapsing per pipeline segment).
func normCommand(cmd string) string { return strings.Join(strings.Fields(cmd), " ") }

// mutatesOnly reports a command made only of state changes that exercise no
// code (mkdir/cp/mv/rm/touch/chmod/ln): not a verification, so a successful
// one after a failure is not a recovery.
func mutatesOnly(cmd string) bool {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	seen := false
	for _, seg := range strings.Split(r.Replace(cmd), "\n") {
		f := strings.Fields(seg)
		for len(f) > 0 && (strings.Contains(f[0], "=") || f[0] == "rtk" || f[0] == "sudo" || f[0] == "time" || f[0] == "--") {
			f = f[1:]
		}
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "mkdir", "cp", "mv", "rm", "touch", "chmod", "ln":
			seen = true
		default:
			return false
		}
	}
	return seen
}

// cmdOf extracts the shell command from a tool call's arguments.
func cmdOf(call llm.ToolCall) string {
	var a struct {
		Command string `json:"command"`
	}
	json.Unmarshal(call.Input, &a)
	return a.Command
}

// verifyingCall reports a shell call that exercises the code — not a purely
// mutating one (mkdir/cp/rm/…): the hint's arming and its delivery share this
// one predicate.
func verifyingCall(call llm.ToolCall) bool {
	if call.Name != "shell" {
		return false
	}
	c := cmdOf(call)
	return verifies(c) && !mutatesOnly(c)
}

// trackFailure counts identical failing calls (tool, normalized input,
// output) and returns the hint for a repeat.
func trackFailure(env *Env, name string, input json.RawMessage, content string) string {
	content = spillPathRe.ReplaceAllLiteralString(content, "saved to <overflow file> — ")
	var buf bytes.Buffer
	if json.Compact(&buf, input) != nil {
		buf.Reset()
		buf.Write(input)
	}
	key := name + "\x00" + buf.String() + "\x00" + content
	if env.failures == nil {
		env.failures = map[string]int{}
	}
	env.failures[key]++
	n := env.failures[key]
	env.MaxRepeat = max(env.MaxRepeat, n)
	if n >= repeatHintAt {
		return fmt.Sprintf("\n[hint: this exact call has failed %d times with the same result — retrying will not help; change approach, or stop and explain what is blocking you]", n)
	}
	return ""
}
