package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
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
}

// noteChange records a successful write/edit of abs: the run has changed
// files (a later red test run is its own work in progress, not a failure to
// investigate), a code file now needs verification, and earlier identical
// failures no longer count as repeats (the state they ran against is gone).
func noteChange(env *Env, abs string) {
	env.Edited = true
	env.failures = nil
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

// trackFailure counts identical failing calls (tool, normalized input,
// output) and returns the hint for a repeat.
func trackFailure(env *Env, name string, input json.RawMessage, content string) string {
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
