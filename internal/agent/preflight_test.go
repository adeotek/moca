package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

// seedAnchor installs a provider-usage anchor without a live model turn, so
// the preflight tests exercise the exact boundary math deterministically.
func seedAnchor(a *Agent, tokens int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.anchorTokens = tokens
	a.anchorEntries = len(a.entries)
	a.anchorValid = true
}

func TestPrepareRequestPressureAndOutputFit(t *testing.T) {
	a, _, evs := startTestWith(t, newScript(t), "", smallModel) // 32768 window, ceiling 26214
	seedAnchor(a, 25000)
	if err := a.prepareRequest(context.Background(), false); err != nil {
		t.Fatalf("25000 is below the ceiling and must fit: %v", err)
	}
	if got := a.turnMaxTokens(); got != 4096 {
		t.Fatalf("25000 leaves 7768 tokens, the configured 4096 must stand; got %d", got)
	}
	var pressure bool
	for _, e := range *evs {
		if _, ok := e.(ContextPressure); ok {
			pressure = true
		}
	}
	if !pressure {
		t.Fatal("a request between the trigger and the ceiling must emit ContextPressure")
	}

	// 30000 is above the ceiling but fits the fixed-overhead exception; the
	// output budget is capped to the remaining room.
	a2, _, _ := startTestWith(t, newScript(t), "", smallModel)
	seedAnchor(a2, 30000)
	if err := a2.prepareRequest(context.Background(), false); err != nil {
		t.Fatalf("fixed-overhead exception must keep this possible: %v", err)
	}
	if got := a2.turnMaxTokens(); got != 2768 {
		t.Fatalf("output must fit the room (2768), got %d", got)
	}

	// 32700 leaves less than minUsefulOutput; the request cannot fit.
	a3, _, _ := startTestWith(t, newScript(t), "", smallModel)
	seedAnchor(a3, 32700)
	if err := a3.prepareRequest(context.Background(), false); err == nil {
		t.Fatal("a request with no useful output room must return contextCannotFit")
	}
}

func TestContextTokensCalibratesFromUsage(t *testing.T) {
	s := newScript(t, textTurnUsage("ok", 4000, 10))
	a, _, _ := startTestWith(t, s, "", "")
	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	cpt := a.charsPerToken
	a.mu.Unlock()
	if cpt <= 0 {
		t.Fatal("a provider usage report must calibrate the fallback ratio")
	}
}

func TestPromptPasteCannotFit(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", minWindowModel)
	_, err := a.Run(context.Background(), strings.Repeat("x", 100_000))
	if err == nil || !strings.Contains(err.Error(), "context cannot fit") {
		t.Fatalf("an oversized prompt must fail before a provider request: %v", err)
	}
	if len(s.bodies) != 0 {
		t.Fatalf("no request may reach the provider, got %d", len(s.bodies))
	}
}

func TestDeterministicCompactionReducesAndPersists(t *testing.T) {
	a, _, _ := startTestWith(t, newScript(t), "", minWindowModel)
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(strings.Repeat("x", 60_000))}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.append(session.Entry{Type: session.TypeMessage, Message: userText(strings.Repeat("y", 10_000))}); err != nil {
		t.Fatal(err)
	}
	before := a.ContextTokens()
	if err := a.compactDeterministic(); err != nil {
		t.Fatalf("deterministic fallback: %v", err)
	}
	if after := a.ContextTokens(); after >= before {
		t.Fatalf("deterministic fallback must reduce the estimate: %d -> %d", before, after)
	}
	entries, err := session.ReadFile(a.Session().Path())
	if err != nil {
		t.Fatal(err)
	}
	c, _ := session.LatestCompaction(entries)
	if c == nil || !strings.Contains(c.Summary, "deterministic compaction fallback") {
		t.Fatalf("no deterministic compaction entry: %+v", c)
	}
}
