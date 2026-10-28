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
	// The executed set is per run: a resumed session's earlier commands are not
	// in this process's history, and claiming otherwise would let a rule cite
	// a command nobody ran here.
	e.Executed = nil
	e.SkillHintArmed, e.SkillHintDone, e.lastFailedVerify = false, false, ""
	e.SkillProvenCmd = ""
	// T-1: the investigation hints are per run. A search or a test read in run
	// 1 used to satisfy run 2's "you have not looked yet" gate, so the hint
	// never fired in a resumed session.
	e.TestSeen, e.TestFailed, e.Searched = false, false, false
	e.FailingTest = ""
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
	"xxd": true, "od": true, "hexdump": true, "strings": true,
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

// executedCap bounds the per-run executed list: the evidence only has to cover
// commands a rule is likely to cite, and a long run must not grow a slice of
// every shell line it ever ran.
const executedCap = 256

// rtkRunVerbs are rtk's command-running subcommands (permissions/rtk.go's
// rtkRun table, unchanged through rtk 1.1.0 — re-probed 2026-10-27):
// `rtk test -- make test` runs `make test`, so the
// evidence must carry the wrapped command, not the wrapper's verb — otherwise
// a true rule citing `make test` is refused for a command the session ran.
var rtkRunVerbs = map[string]bool{"test": true, "err": true, "summary": true, "proxy": true, "run": true}

// ExecutedSegments splits a shell command line into normalized segments — the
// evidence form for the command-claim rule: `cd x && make test CALC_STRICT=1`
// yields "make test CALC_STRICT=1", so a rule's `make test` matches it.
func ExecutedSegments(command string) []string {
	r := strings.NewReplacer("&&", "\n", "||", "\n", ";", "\n", "|", "\n")
	var out []string
	for _, seg := range strings.Split(r.Replace(command), "\n") {
		f := strings.Fields(seg)
		for len(f) > 0 {
			if strings.Contains(f[0], "=") || f[0] == "sudo" || f[0] == "time" || f[0] == "--" {
				f = f[1:]
				continue
			}
			// `rtk <verb> …` for a command-running verb: consume the wrapper
			// and its verb together (the analyser's own unwrap, §10) — the
			// eval checkers normalize the same way (§15), so the evidence
			// matches what they judge.
			if f[0] == "rtk" && len(f) > 1 && rtkRunVerbs[strings.ToLower(f[1])] {
				f = f[2:]
				continue
			}
			if f[0] == "rtk" {
				f = f[1:]
				continue
			}
			break
		}
		// `cd <dir>` is navigation, not a command a rule would cite — the eval
		// checkers normalize it away too (§15), so the evidence matches.
		if len(f) > 1 && f[0] == "cd" {
			f = f[2:]
		}
		if len(f) > 0 {
			out = append(out, strings.Join(f, " "))
		}
	}
	return out
}

// ExecutedFromCall returns a shell call's command split into evidence
// segments — nil for any other tool, and for input that does not parse. The
// agent seeds a resumed session's evidence with it; Registry.Run appends the
// same shape for every call that ran.
func ExecutedFromCall(call llm.ToolCall) []string {
	if call.Name != "shell" {
		return nil
	}
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(call.Input, &in); err != nil {
		return nil
	}
	return ExecutedSegments(in.Command)
}

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

// setupShaped are the first sub-words that install, inspect or explain a
// toolchain rather than verify a repository. A cold repo's first `go version`
// or `npm install` used to count as "how this repo is checked" and armed the
// hint with a fact the run never established (open item 8).
var setupShaped = map[string]bool{
	"version": true, "--version": true, "help": true, "--help": true,
	"install": true, "i": true, "ci": true, "add": true, "init": true, "create": true,
	"env": true, "mod": true, "get": true, "fetch": true, "download": true,
	"update": true, "upgrade": true, "list": true, "ls": true, "info": true,
	"show": true, "config": true, "doc": true, "clean": true, "prune": true, "cache": true,
}

// verificationShaped reports whether a call verifies *and* names work rather
// than a toolchain's own setup or inspection sub-word (`go version`,
// `npm install`, `cargo fetch`). Cold-repo arming uses this stricter predicate;
// the recovery path keeps the original one.
func verificationShaped(call llm.ToolCall) bool {
	if !verifyingCall(call) {
		return false
	}
	fields := strings.Fields(cmdOf(call))
	for _, w := range fields[1:] {
		if strings.HasPrefix(w, "-") {
			continue // a flag is not the sub-word
		}
		return !setupShaped[strings.ToLower(w)]
	}
	return true
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
