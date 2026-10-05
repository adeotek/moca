package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// Small-window model (32K): reserve 8192, keepRecent 8192, trigger 24576 tokens.
const smallModel = `"m":{"contextWindow":32768,"maxOutputTokens":4096}`

// The config minimum window (16K): reserve 4096, keepRecent 4096, trigger
// 12288 — small enough that one big read busts it.
const minWindowModel = `"m":{"contextWindow":16384,"maxOutputTokens":4096}`

// bigToolTurns scripts n shell turns each returning 20000 chars (~5000
// tokens); with smallModel the trigger (24576) fires exactly after the 5th
// batch. (The script server answers every agent request in order, including
// summary requests, so the follow-up turns must be planned with that.)
func bigToolTurns(n int) []string {
	var turns []string
	for range n {
		turns = append(turns, toolTurn([2]string{"shell", "{\"command\":\"head -c 20000 /dev/zero | tr '\\\\0' x\"}"}))
	}
	return turns
}

// textTurnUsage is textTurn with an explicit usage report: tests use it to
// plant a chosen anchored estimate.
func textTurnUsage(text string, prompt, completion int) string {
	b, _ := json.Marshal(text)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d}}\n\ndata: [DONE]\n\n", b, prompt, completion)
}

func summaryBodies(s *scriptServer) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []map[string]any
	for _, b := range s.bodies {
		msgs, _ := b["messages"].([]any)
		if len(msgs) == 0 {
			continue
		}
		m0, _ := msgs[0].(map[string]any)
		if sys, _ := m0["content"].(string); strings.Contains(sys, "## Constraints & Preferences") {
			out = append(out, b)
		}
	}
	return out
}

func TestAutoCompactionShrinksContext(t *testing.T) {
	turns := bigToolTurns(5)
	turns = append(turns, textTurn("## Goal\nsummary"), textTurn("done"))
	s := newScript(t, turns...)
	a, _, evs := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	out, err := a.Run(context.Background(), "make output")
	if err != nil || out.Text != "done" {
		t.Fatal(out, err)
	}
	var comp *Compacted
	for _, e := range *evs {
		if c, ok := e.(Compacted); ok {
			comp = &c
		}
	}
	if comp == nil || comp.TokensAfter >= comp.TokensBefore {
		t.Fatalf("compaction must shrink context: %+v", comp)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if len(session.Repair(entries, "")) != 0 {
		t.Fatal("no orphaned tool calls")
	}
	msgs := session.Messages(entries)
	if !strings.HasPrefix(msgs[0].Content[0].Text, "[Summary of earlier conversation]") {
		t.Fatal("request starts from the summary")
	}
	// The summary request: no tools, max_tokens ≤ 4096, one structured ask.
	sums := summaryBodies(s)
	if len(sums) != 1 || sums[0]["tools"] != nil {
		t.Fatalf("summary request shape: %v", sums)
	}
	if mt, _ := sums[0]["max_tokens"].(float64); mt == 0 || mt > 4096 {
		t.Fatalf("summary max_tokens: %v", sums[0]["max_tokens"])
	}
}

func TestLoopGuardNamesOversizedEntry(t *testing.T) {
	s := newScript(t, toolTurn([2]string{"read", `{"path":"dist/app.min.js"}`}), textTurn("## Goal\ns"), textTurn("never"))
	a, work, _ := startTestWith(t, s, "", minWindowModel)
	// One read of a big file is capped at 50K chars ≈ 12k tokens: larger
	// than keepRecent (4096) and, kept verbatim, still over the trigger
	// (12288) — the loop guard must name it instead of compacting forever.
	var sb strings.Builder
	for sb.Len() < 300_000 {
		sb.WriteString("const v000000000000000000000000 = 1;\n")
	}
	if err := os.MkdirAll(filepath.Join(work, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "dist", "app.min.js"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := a.Run(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "still over budget") ||
		!strings.Contains(err.Error(), "read") || !strings.Contains(err.Error(), "12k tokens") {
		t.Fatalf("%v", err)
	}
}

func TestOverflowCompactsAndRetriesOnce(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	// Requests: q-prompt, r-prompt, overflow; summary A (history), summary B
	// (split turn part), retry. 80K-char prompts put ~20k tokens per turn, so
	// the cut splits the turn (keepRecent 16384) and two summaries merge.
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflow, textTurn("## Goal\ns"), textTurn("## Turn\ns"), textTurn("recovered"))
	a, _, _ := startTestWith(t, s, "", "")
	if _, err := a.Run(context.Background(), strings.Repeat("q", 80000)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), strings.Repeat("r", 80000)); err != nil {
		t.Fatal(err)
	}
	out, err := a.Run(context.Background(), "third")
	if err != nil || out.Text != "recovered" {
		t.Fatal(out, err)
	}
	if sums := summaryBodies(s); len(sums) != 2 {
		t.Fatalf("split-turn overflow recovery issues exactly two summaries, got %d", len(sums))
	}
	// Exactly-once: 2 runs × 1 turn + overflow turn + 2 summaries + 1 retry.
	if n := len(s.bodies); n != 6 {
		t.Fatalf("request count %d != 6: the compact-and-retry must fire exactly once", n)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var comps, recovered int
	for _, e := range entries {
		if e.Type == session.TypeCompaction {
			comps++
		}
		if e.Type == session.TypeMessage && e.Message.Role == llm.RoleAssistant &&
			len(e.Message.Content) > 0 && e.Message.Content[0].Text == "recovered" {
			recovered++
		}
	}
	if comps != 1 || recovered != 1 {
		t.Fatalf("compactions=%d recovered=%d", comps, recovered)
	}
}

func TestOverflowRecoveryFailsWithoutHistory(t *testing.T) {
	overflow := "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`
	s := newScript(t, overflow, textTurn("unused"))
	a, _, _ := startTestWith(t, s, "", "")
	_, err := a.Run(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("overflow with nothing to compact must surface the original error: %v", err)
	}
	// Nothing was persisted beyond the user prompt: no error entry, no
	// compaction entry, no assistant response.
	entries, _ := session.ReadFile(a.Session().Path())
	if len(entries) != 2 || entries[1].Type != session.TypeMessage {
		t.Fatalf("nothing but the prompt must be persisted: %d entries", len(entries))
	}
}

func TestManualCompactNothing(t *testing.T) {
	s := newScript(t, textTurn("hi"))
	a, _, _ := startTestWith(t, s, "", "")
	a.Run(context.Background(), "hello")
	n := len(s.bodies)
	if err := a.Compact(context.Background()); err != ErrNothingToCompact || len(s.bodies) != n {
		t.Fatal(err)
	}
}

// A cheap model whose credential is missing must not kill a run the active
// model handles fine: the automatic checkpoint warns and skips; a real
// overflow would still surface it (turnWithRecovery path).
func TestAutoCompactionSkipsWhenSummarizerUnavailable(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOCA_T_KEY", "k")   // working provider
	t.Setenv("MOCA_T_BROKEN", "") // missing key for the configured cheap model
	s := newScript(t, append(bigToolTurns(5), textTurn("done"))...)
	cfg, err := config.Parse([]byte(fmt.Sprintf(`{"model":"broken/x","context":{"maxSteps":40},
		"providers":{
		  "fake":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":32768,"maxOutputTokens":4096}}},
		  "broken":{"baseUrl":%q,"protocol":"openai-completions","auth":"api_key","apiKey":"env:MOCA_T_BROKEN","models":{"x":{"contextWindow":32768,"maxOutputTokens":4096}}}}}`,
		s.srv.URL, s.srv.URL)))
	if err != nil {
		t.Fatal(err)
	}
	var evs []Event
	a, err := Start(StartOptions{Config: cfg, Workdir: t.TempDir(), Emit: func(e Event) { evs = append(evs, e) }, HTTP: s.srv.Client(), Slug: "t"})
	if err != nil {
		t.Fatal(err)
	}
	a.opts.Env.Commands = allowAll{}
	if err := a.SetModel("fake/m", ""); err != nil { // active model: the keyed one
		t.Fatal(err)
	}
	out, err := a.Run(context.Background(), "make output")
	if err != nil || out.Text != "done" {
		t.Fatalf("the run must continue when the summarizer is unusable: %v %q", err, out.Text)
	}
	var warned bool
	for _, e := range evs {
		if w, ok := e.(Warning); ok && strings.Contains(w.Text, "auto-compaction skipped") && strings.Contains(w.Text, "MOCA_T_BROKEN") {
			warned = true
		}
	}
	if !warned {
		t.Fatal("a warning must report the skipped auto-compaction")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if c, _ := session.LatestCompaction(entries); c != nil {
		t.Fatal("no compaction entry may be written when the summarizer is unusable")
	}
}

// When the estimate is over the trigger but the transcript is tiny, the loop
// guard must blame the fixed overhead, not near-empty entries.
func TestLoopGuardReportsFixedOverhead(t *testing.T) {
	s := newScript(t, textTurnUsage("hi", 26000, 15))
	a, _, _ := startTestWith(t, s, "", smallModel)
	if out, err := a.Run(context.Background(), "one"); err != nil || out.Text != "hi" {
		t.Fatal(out, err)
	}
	_, err := a.Run(context.Background(), "two")
	if err == nil || !strings.Contains(err.Error(), "still over budget") ||
		!strings.Contains(err.Error(), "fixed overhead") || !strings.Contains(err.Error(), "26k tokens") {
		t.Fatalf("%v", err)
	}
}

// A compaction whose firstKeptEntryID is gone (trimmed file) must not make
// the next compaction re-serialize already-summarized history.
func TestLiveEntriesMissingFirstKeptFallback(t *testing.T) {
	s := newScript(t)
	a, _, _ := startTestWith(t, s, "", "")
	a.append(session.Entry{Type: session.TypeMessage, Message: userText("old")})
	a.append(session.Entry{Type: session.TypeCompaction, Compaction: &session.Compaction{Summary: "S", FirstKeptEntryID: "gone"}})
	a.append(session.Entry{Type: session.TypeMessage, Message: userText("kept")})
	live, prev := a.liveEntries()
	if prev.Summary != "S" {
		t.Fatalf("%+v", prev)
	}
	if len(live) != 1 || live[0].Msg == nil || live[0].Msg.Content[0].Text != "kept" {
		t.Fatalf("live window must start after the compaction entry: %+v", live)
	}
}

func TestManualCompactWritesEntryUsageAndRebuild(t *testing.T) {
	turns := bigToolTurns(2)
	turns = append(turns, textTurn("early done"), textTurn("## Goal\nmanual"), textTurn("after"))
	s := newScript(t, turns...)
	a, _, _ := startTestWith(t, s, "", smallModel)
	a.opts.Env.Commands = allowAll{}
	if _, err := a.Run(context.Background(), "make output"); err != nil {
		t.Fatal(err)
	}
	u0, c0 := a.Totals()
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	u1, c1 := a.Totals()
	if u1.Input <= u0.Input || u1.Output <= u0.Output || c1 < c0 {
		t.Fatalf("summary usage must count towards session totals: %+v/%v → %+v/%v", u0, c0, u1, c1)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	c, _ := session.LatestCompaction(entries)
	if c == nil || !strings.Contains(c.Summary, "manual") || c.FirstKeptEntryID == "" {
		t.Fatalf("compaction entry: %+v", c)
	}
	out, err := a.Run(context.Background(), "now")
	if err != nil || out.Text != "after" {
		t.Fatal(out, err)
	}
	last, _ := json.Marshal(s.bodies[len(s.bodies)-1])
	if !strings.Contains(string(last), "[Summary of earlier conversation]") {
		t.Fatalf("the next request must start from the summary: %s", last)
	}
}

const overflowBody = "HTTP400:" + `{"error":{"code":"context_length_exceeded","message":"maximum context length"}}`

// esc during the recovery compaction is an interrupt, not the provider error
// the recovery was trying to get past.
func TestOverflowRecoveryCancelSurfacesCancellation(t *testing.T) {
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflowBody, textTurn("## Goal\ns"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _, _ := startWith(t, s, 40, "", "", func(o *StartOptions) {
		// StreamReset is emitted right as the overflow is detected, before
		// the recovery compaction starts.
		o.Emit = func(e Event) {
			if _, ok := e.(StreamReset); ok {
				cancel()
			}
		}
	})
	for _, p := range []string{strings.Repeat("q", 80000), strings.Repeat("r", 80000)} {
		if _, err := a.Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	_, err := a.Run(ctx, "third")
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "maximum context length") {
		t.Fatalf("a cancelled recovery must surface the cancellation, got: %v", err)
	}
}

// Tokens a summary spent before the compaction failed were billed: the first
// of the two split-turn calls succeeds, the second fails.
func TestFailedCompactionStillCountsSpentUsage(t *testing.T) {
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflowBody,
		textTurnUsage("## Goal\nA", 1234, 56), "HTTP400:"+`{"error":{"message":"nope"}}`)
	a, _, _ := startTestWith(t, s, "", "")
	for _, p := range []string{strings.Repeat("q", 80000), strings.Repeat("r", 80000)} {
		if _, err := a.Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	u0, _ := a.Totals()
	if _, err := a.Run(context.Background(), "third"); err == nil {
		t.Fatal("recovery fails: the original overflow error surfaces")
	}
	u1, c1 := a.Totals()
	if u1.Input-u0.Input != 1234 || u1.Output-u0.Output != 56 {
		t.Fatalf("the first summary call's usage must be counted: %+v → %+v", u0, u1)
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var booked bool
	for _, e := range entries {
		if e.Type == session.TypeError && e.Usage != nil && e.Usage.Input == 1234 {
			booked = true
		}
	}
	if !booked {
		t.Fatal("the spent usage must be in the session file so a resume restores it")
	}
	_ = c1
}

// The input cap is an estimate: a summarizer that still overflows gets the
// newer half of the payload once instead of failing the compaction.
func TestSummarizerOverflowRetriesWithHalfThePayload(t *testing.T) {
	s := newScript(t, textTurn("a1"), textTurn("a2"), overflowBody,
		overflowBody, textTurn("## Goal\nA"), textTurn("## Turn\nB"), textTurn("recovered"))
	a, _, _ := startTestWith(t, s, "", "")
	for _, p := range []string{strings.Repeat("q", 80000), strings.Repeat("r", 80000)} {
		if _, err := a.Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	out, err := a.Run(context.Background(), "third")
	if err != nil || out.Text != "recovered" {
		t.Fatalf("%q %v", out.Text, err)
	}
	sums := summaryBodies(s)
	if len(sums) != 3 {
		t.Fatalf("overflowed attempt + halved retry + split-turn part = 3 summary requests, got %d", len(sums))
	}
	userLen := func(b map[string]any) int {
		msgs := b["messages"].([]any)
		c, _ := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		return len(c)
	}
	if userLen(sums[1]) >= userLen(sums[0]) {
		t.Fatalf("the retry must send less: %d then %d", userLen(sums[0]), userLen(sums[1]))
	}
}

// Repeated compactions (Global Constraint): the second one summarizes from the
// first one's kept boundary, is fed the first summary, and the file lists
// accumulate even though the call that read the file is long out of the window.
func TestRepeatedCompactionBuildsOnThePreviousOne(t *testing.T) {
	const bigModel = `"m":{"contextWindow":100000,"maxOutputTokens":4096}`
	var turns []string
	turns = append(turns, toolTurn([2]string{"read", `{"path":"a.txt"}`}))
	turns = append(turns, bigToolTurns(5)...)
	turns = append(turns, textTurn("r1 done"), textTurn("## Goal\nS1marker"))
	turns = append(turns, bigToolTurns(5)...)
	turns = append(turns, textTurn("r2 done"), textTurn("## Goal\nS2marker"), textTurn("## Turn\nS2b"))
	s := newScript(t, turns...)
	a, work, _ := startTestWith(t, s, "", bigModel)
	a.opts.Env.Commands = allowAll{}
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "FIRSTPROMPT read a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Run(context.Background(), "SECONDPROMPT"); err != nil {
		t.Fatal(err)
	}
	if err := a.Compact(context.Background()); err != nil {
		t.Fatal(err)
	}
	sums := summaryBodies(s)
	// The first compaction cuts between turns (one request); the second cuts
	// inside the SECONDPROMPT turn (history + turn prefix = two requests).
	if len(sums) != 3 {
		t.Fatalf("1 + 2 summary requests expected, got %d", len(sums))
	}
	payload := func(b map[string]any) string {
		msgs := b["messages"].([]any)
		c, _ := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		return c
	}
	if !strings.Contains(payload(sums[0]), "FIRSTPROMPT") {
		t.Fatal("the first compaction summarizes the opening of the session")
	}
	second := payload(sums[1])
	if !strings.Contains(second, "Previous summary") || !strings.Contains(second, "S1marker") {
		t.Fatal("the second compaction must be fed the first summary")
	}
	if strings.Contains(second, "FIRSTPROMPT") {
		t.Fatal("the second compaction must start at the first one's kept boundary, not re-serialize old history")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	var comps []*session.Compaction
	var idx = map[string]int{}
	for i, e := range entries {
		idx[e.ID] = i
		if e.Type == session.TypeCompaction {
			comps = append(comps, e.Compaction)
		}
	}
	if len(comps) != 2 || idx[comps[1].FirstKeptEntryID] <= idx[comps[0].FirstKeptEntryID] {
		t.Fatalf("the kept boundary must advance: %+v", comps)
	}
	if !slices.Contains(comps[1].ReadFiles, "a.txt") {
		t.Fatalf("read-file list must accumulate across compactions: %v", comps[1].ReadFiles)
	}
}
