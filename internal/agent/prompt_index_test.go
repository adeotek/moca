package agent

import (
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/skills"
)

// The line/byte ceiling in TestBuildSystemPromptLineBudget counts the template
// and the environment skeleton. The **skill index** is appended at runtime, so
// it never shows up there — and the v0.8.1 release A/B priced exactly that:
// systemPromptTokens went 2381 -> 2680 (+299) when three built-ins joined, on
// every request of every session, for everyone.
//
// So the index gets its own ceiling, measured the same way the prompt's is:
// BuildSystemPrompt with the real embedded built-ins, minus BuildSystemPrompt
// with none. Raise it only with a measured reason — a new built-in is a
// permanent tax, and this is where it is declared.
func TestTheSkillIndexFitsItsBudget(t *testing.T) {
	dir, err := skills.ExtractBuiltins(t.TempDir())
	if err != nil {
		t.Fatalf("extract the built-ins: %v", err)
	}
	sk, errs := skills.Discover([]skills.Dir{{Path: dir, Source: "builtin"}})
	if len(errs) > 0 {
		t.Fatalf("discover: %v", errs)
	}
	if len(sk) == 0 {
		t.Fatal("no built-in skills discovered: the index this test prices is empty")
	}

	bare := BuildSystemPrompt(PromptInput{})
	withIndex := BuildSystemPrompt(PromptInput{Skills: sk})
	// Every index line embeds the skill's absolute path, and the extraction dir
	// is as long as TMPDIR says: pricing the raw delta measured the machine, not
	// the tree (a ~133-char TMPDIR failed the ceiling outright, and the recorded
	// figure did not reproduce anywhere else). Each path is normalized to a
	// fixed sentinel, so the pin counts the line's content and stays portable.
	const sentinel = "<path>"
	index := len(withIndex) - len(bare)
	for _, k := range sk {
		index += len(sentinel) - len(k.Path)
	}
	// Six built-ins measure exactly 1 280 bytes: a header, a hint line and one
	// line per skill, each path normalized to <path>. The ceiling adds 120 bytes
	// — enough to reword a description; the cheapest seventh built-in costs 139
	// bytes of line (the shortest of the six), so one does not fit.
	const ceiling = 1400
	if index > ceiling {
		t.Fatalf("the built-in skill index is %d bytes (ceiling %d): %d skills listed — trim a description or move a skill out of the index", index, ceiling, len(sk))
	}

	// Each name appears once: a duplicated line is a bug the byte count would
	// hide inside its headroom. The renderer emits exactly one `- name:` line
	// per skill (prompt.go), so two is not a legal shape.
	for _, k := range sk {
		if n := strings.Count(withIndex, "\n- "+k.Name+":"); n != 1 {
			t.Fatalf("built-in %q appears %d times in the index (expected exactly one entry)", k.Name, n)
		}
	}
}
