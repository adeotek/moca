package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

func trimZero(s string) string { return strings.TrimSuffix(s, ".0") }

// FmtTokens renders a token count compactly: 999 · 1.2k · 24k · 999k · 1.3M.
func FmtTokens(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprint(n)
	case n < 10_000:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1000)) + "k"
	case n < 1_000_000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return trimZero(fmt.Sprintf("%.1f", float64(n)/1_000_000)) + "M"
	}
}

// FmtWindow renders a context window: binary multiples of 1024 print in k/M.
func FmtWindow(n int) string {
	switch {
	case n%1_048_576 == 0:
		return fmt.Sprintf("%dM", n/1_048_576)
	case n%1_000_000 == 0:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n%1024 == 0:
		return fmt.Sprintf("%dk", n/1024)
	default:
		return fmt.Sprintf("%dk", n/1000)
	}
}

var effortAbbrev = map[llm.Effort]string{llm.EffortOff: "off", llm.EffortMinimal: "min", llm.EffortLow: "low",
	llm.EffortMedium: "med", llm.EffortHigh: "high", llm.EffortXHigh: "xhigh", llm.EffortMax: "max"}

func AbbrevEffort(e llm.Effort) string { return effortAbbrev[e] }

// AbbrevHome shortens a path under home to its ~ form.
func AbbrevHome(p, home string) string {
	if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
		return "~" + p[len(home):]
	}
	return p
}

// gitStatusArgs: --no-optional-locks keeps the status bar's refresh from
// taking index.lock while the agent's own git commands run.
func gitStatusArgs(dir string) []string {
	return []string{"--no-optional-locks", "-C", dir, "status", "--porcelain"}
}

// GitBranch reports the branch, dirty flag and whether dir is a git repo.
// The git calls are bounded: a hanging git (credential prompt, stalled NFS)
// must not leave the status bar stale forever.
func GitBranch(dir string) (string, bool, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", false, false
	}
	st, _ := exec.CommandContext(ctx, "git", gitStatusArgs(dir)...).Output()
	return strings.TrimSpace(string(b)), len(strings.TrimSpace(string(st))) > 0, true
}
