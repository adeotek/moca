package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `moca skills` is the discovery-order view: a user has no other way to learn
// that a built-in exists, or that their own copy of the same name wins.
func TestRunSkillsListsProvenance(t *testing.T) {
	dir := skillsConfigDir(t)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"skills"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{
		"resolution order: project, global, built-in",
		"review",
		"builtin",
		"`moca skills fork <name>` copies one into " + dir,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("stdout missing %q:\n%s", want, s)
		}
	}
}

func TestRunSkillsForkThenTheCopyWins(t *testing.T) {
	dir := skillsConfigDir(t)
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"skills", "fork", "review"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	want := "forked review -> " + filepath.Join(dir, "review")
	if !strings.Contains(out.String(), want) {
		t.Fatalf("stdout missing %q:\n%s", want, out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "review", "SKILL.md")); err != nil {
		t.Fatalf("the fork was not written: %v", err)
	}

	// The listing now shows the user's copy as global and no builtin row for it.
	out.Reset()
	errb.Reset()
	if code := run(context.Background(), []string{"skills", "list"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("list exit %d, stderr: %s", code, errb.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "review ") && !strings.Contains(line, "global") {
			t.Fatalf("after a fork, review must resolve to the global copy:\n%s", line)
		}
	}
}

func TestRunSkillsForkRefusals(t *testing.T) {
	skillsConfigDir(t)
	for _, args := range [][]string{
		{"skills", "fork"},         // no name
		{"skills", "fork", "nope"}, // not a built-in
		{"skills", "fork", "a/b"},  // not a name
		{"skills", "bogus"},        // unknown action
	} {
		var out, errb bytes.Buffer
		if code := run(context.Background(), args, nil, &out, &errb); code == exitOK {
			t.Fatalf("%v must fail, stdout:\n%s", args, out.String())
		}
		if !strings.Contains(errb.String(), "usage:") && !strings.Contains(errb.String(), "not a built-in skill") {
			t.Fatalf("%v: the refusal must say what to do:\n%s", args, errb.String())
		}
	}
}
