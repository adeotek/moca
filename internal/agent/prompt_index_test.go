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
	index := len(withIndex) - len(bare)
	// Six built-ins measured exactly 1 980 bytes on the v0.8.2 tree: a header,
	// a hint line and one line per skill. The ceiling adds 170 bytes — enough
	// to reword a description, not enough for a seventh built-in (the cheapest
	// of the six costs ~240 bytes of line).
	const ceiling = 2150
	if index > ceiling {
		t.Fatalf("the built-in skill index is %d bytes (ceiling %d): %d skills listed — trim a description or move a skill out of the index", index, ceiling, len(sk))
	}

	// Each name appears once: a duplicated line is a bug the byte count would
	// hide inside its headroom.
	for _, k := range sk {
		if n := strings.Count(withIndex, "\n- "+k.Name+":"); n != 1 && n != 2 {
			t.Fatalf("built-in %q appears %d times in the index (expected one entry, or two for a name + path line)", k.Name, n)
		}
	}
}
