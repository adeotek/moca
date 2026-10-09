package tui

import (
	"os/exec"
	"testing"

	"github.com/adeotek/moca/internal/llm"
)

func TestFormatters(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1234: "1.2k", 24_000: "24k", 999_999: "999k", 1_300_000: "1.3M"}
	for n, want := range cases {
		if got := FmtTokens(n); got != want {
			t.Errorf("FmtTokens(%d)=%s want %s", n, got, want)
		}
	}
	for n, want := range map[int]string{1_000_000: "1M", 200_000: "200k", 131_072: "128k", 1_048_576: "1M", 32_768: "32k"} {
		if got := FmtWindow(n); got != want {
			t.Errorf("FmtWindow(%d)=%s want %s", n, got, want)
		}
	}
	if AbbrevEffort(llm.EffortMedium) != "med" || AbbrevEffort(llm.EffortMinimal) != "min" || AbbrevEffort(llm.EffortXHigh) != "xhigh" {
		t.Fatal("effort abbrev")
	}
	if AbbrevHome("/home/u/projects/moca", "/home/u") != "~/projects/moca" || AbbrevHome("/opt/x", "/home/u") != "/opt/x" {
		t.Fatal("home abbrev")
	}
}

// GitBranch resolves the repository of the test itself.
func TestGitBranch(t *testing.T) {
	b, dirty, isGit := GitBranch(".")
	if !isGit || b == "" {
		t.Fatalf("repo branch %q dirty %v git %v", b, dirty, isGit)
	}
	if _, _, isGit := GitBranch(t.TempDir()); isGit {
		t.Fatal("a plain temp dir is not a git repo")
	}
}

// A fresh `git init` (no commit yet) still reports its unborn branch.
func TestGitBranchUnborn(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q", "-b", "trunk").Run(); err != nil {
		t.Skip("git unavailable:", err)
	}
	if b, _, isGit := GitBranch(dir); !isGit || b != "trunk" {
		t.Fatalf("got %q git=%v", b, isGit)
	}
}
