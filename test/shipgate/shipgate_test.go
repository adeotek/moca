//go:build shipgate

// Package shipgate is the automated §14 ship-gate checker: it asserts the
// recorded session transcript and the fixture repo show the required
// behaviour, then the process exit code. Driven by test/shipgate/run.sh:
//
//	go test -tags shipgate ./test/shipgate -args -session <jsonl> -repo <dir> -exit <code>
//
// The checker runs `go test ./...` in -repo itself, so `go` must be on PATH.
package shipgate

import (
	"encoding/json"
	"flag"
	"os/exec"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

var (
	sessionPath = flag.String("session", "", "session jsonl")
	repo        = flag.String("repo", "", "demo repo")
	exitCode    = flag.Int("exit", -1, "moca exit code")
)

type use struct {
	name   string
	input  map[string]any
	result string
	isErr  bool
}

func TestShipGate(t *testing.T) {
	if *sessionPath == "" || *repo == "" {
		t.Fatal("usage: -session <jsonl> -repo <dir> [-exit <code>]")
	}
	entries, err := session.ReadFile(*sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 || entries[0].Session == nil {
		t.Fatal("session has no header entry")
	}
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("every tool_use must have a tool_result")
	}
	results := map[string]*use{}
	var uses []*use
	for _, e := range entries {
		switch e.Type {
		case session.TypeToolUse:
			u := &use{name: e.ToolUse.Call.Name}
			if err := json.Unmarshal(e.ToolUse.Call.Input, &u.input); err != nil {
				t.Fatalf("tool_use %s: bad input: %v", u.name, err)
			}
			uses = append(uses, u)
			results[e.ToolUse.Call.ID] = u
		case session.TypeToolResult:
			if u := results[e.ToolResult.CallID]; u != nil {
				u.result, u.isErr = e.ToolResult.Content, e.ToolResult.IsError
			}
		}
	}
	idx := func(pred func(*use) bool) int {
		for i, u := range uses {
			if pred(u) {
				return i
			}
		}
		return -1
	}
	str := func(u *use, k string) string { s, _ := u.input[k].(string); return s }
	firstEdit := idx(func(u *use) bool { return u.name == "edit" && strings.HasSuffix(str(u, "path"), "calc.go") && !u.isErr })
	readTest := idx(func(u *use) bool { return u.name == "read" && strings.HasSuffix(str(u, "path"), "_test.go") })
	search := idx(func(u *use) bool { return u.name == "search" })
	testRun := -1
	for i, u := range uses {
		c := str(u, "command")
		if u.name == "shell" && strings.Contains(c, "go test ./...") && strings.Contains(u.result, "[exit 0]") && i > firstEdit {
			testRun = i
		}
	}
	switch {
	case firstEdit < 0:
		t.Fatal("3: no successful edit of calc.go")
	case readTest < 0 || readTest > firstEdit:
		t.Fatal("1: the failing test was not read before the fix")
	case search < 0 || search > firstEdit:
		t.Fatal("2: the bug was not located with search before the fix")
	case testRun < 0:
		t.Fatal("4: no green `go test ./...` after the edit")
	}
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", *repo}, args...)...).Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return strings.TrimSpace(string(out))
	}
	if git("rev-parse", "--abbrev-ref", "HEAD") != "fix/sum" ||
		git("log", "--oneline", "main..fix/sum") == "" ||
		git("status", "--porcelain") != "" {
		t.Fatal("5: the fix is not committed on branch fix/sum with a clean tree")
	}
	// Gate 4 above reads the transcript, where "[exit 0]" is the exit status of
	// the whole shell command: `go test ./... | tail` or `|| true` reports 0
	// with the suite red. The authoritative check is the suite itself, run in
	// the repo the session left behind.
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = *repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4: `go test ./...` in the final repo state fails: %v\n%s", err, out)
	}
	sys := entries[0].Session.SystemPrompt
	rtkRead := idx(func(u *use) bool { return u.name == "read" && strings.Contains(str(u, "path"), "/rtk/SKILL.md") }) >= 0
	rtkUsed := idx(func(u *use) bool {
		return u.name == "shell" && strings.HasPrefix(strings.TrimSpace(str(u, "command")), "rtk ")
	}) >= 0
	if !strings.Contains(sys, "- rtk:") || !(rtkRead || rtkUsed) {
		// The literal "- rtk:" is phase-2's frozen skill-list line shape
		// ("- <name>: <one-line description> (<absolute path>)"); if the shape
		// ever changes, this assertion changes with it, in the same revision.
		t.Fatal("6: the rtk skill was not discoverable or its guidance was not followed")
	}
	if *exitCode != 0 {
		t.Fatalf("7: moca exit code %d", *exitCode)
	}
}
