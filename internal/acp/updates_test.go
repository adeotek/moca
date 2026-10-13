package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
	msession "github.com/adeotek/moca/internal/session"
)

// bareSession is a session without an agent on a Server whose conn writes
// into a buffer: for the event-mapping rules. notices/compaction stand for
// the client's advertised capabilities.
func bareSession(t *testing.T, notices, compaction bool) (*session, func() []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	srv := &Server{sessions: map[string]*session{}, notices: notices, compaction: compaction}
	srv.conn = NewConn(&buf, srv)
	s := newSession(srv, "/w", true)
	s.id = "abcd1234"
	s.markReady()
	return s, func() []map[string]any {
		var out []map[string]any
		sc := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
		for sc.Scan() {
			var m struct {
				Params SessionNotification `json:"params"`
			}
			json.Unmarshal(sc.Bytes(), &m)
			out = append(out, m.Params.Update.(map[string]any))
		}
		return out
	}
}

func TestWarningAndRetryNotices(t *testing.T) {
	s, ups := bareSession(t, true, false)
	s.emit(agent.Warning{Text: "run stopped: the same failing call repeated 3 times"})
	s.emit(agent.Retry{Notice: provider.RetryNotice{Attempt: 2, Max: 5, Wait: 1500 * time.Millisecond}})
	got := ups()
	if len(got) != 2 || got[0]["sessionUpdate"] != updNotice || got[0]["severity"] != "warning" ||
		got[1]["severity"] != "info" || !strings.Contains(got[1]["description"].(string), "attempt 2 of 5") {
		t.Fatalf("%v", got)
	}
	for _, u := range got {
		conform(t, "SessionNotification", SessionNotification{SessionID: s.id, Update: u})
	}
	// Without the capability: no notice at all (spec MUST); the warning
	// stays visible as a thought line, the retry is silent.
	s2, ups2 := bareSession(t, false, false)
	s2.emit(agent.Warning{Text: "careful"})
	s2.emit(agent.Retry{Notice: provider.RetryNotice{Attempt: 1, Max: 5}})
	got = ups2()
	if len(got) != 1 || got[0]["sessionUpdate"] != updThought || !strings.Contains(mustJSON(got[0]), "moca: careful") {
		t.Fatalf("%v", got)
	}
}

func TestCompactedUpdates(t *testing.T) {
	s, ups := bareSession(t, false, true)
	s.compacted("the summary", 90000, 12000)
	got := ups()
	if len(got) != 2 || got[0]["sessionUpdate"] != updCompactionChunk || got[1]["sessionUpdate"] != updCompaction ||
		got[1]["status"] != "completed" || got[0]["compactionId"] != got[1]["compactionId"] ||
		got[0]["content"].(map[string]any)["text"] != "the summary" {
		t.Fatalf("%v", got)
	}
	for _, u := range got {
		conform(t, "SessionNotification", SessionNotification{SessionID: s.id, Update: u})
	}
	s2, ups2 := bareSession(t, false, false)
	s2.compacted("the summary", 90000, 12000)
	if got := ups2(); len(got) != 1 || got[0]["sessionUpdate"] != updThought || !strings.Contains(mustJSON(got[0]), "90000 → 12000") {
		t.Fatalf("without the capability: %v", got)
	}
}

func TestSummaryText(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: msession.SummaryPrefix + "\nWe fixed the parser."}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "ok"}}},
	}
	if got := summaryText(msgs); got != "We fixed the parser." {
		t.Fatalf("%q", got)
	}
	if summaryText(msgs[1:]) != "" {
		t.Fatal("no summary message, no summary")
	}
}

func TestReplayCompactionNeedsCapability(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: msession.SummaryPrefix + "\nS"}}},
		{Role: llm.RoleAssistant, Content: []llm.ContentBlock{{Type: llm.BlockText, Text: "hi"}}},
	}
	s, ups := bareSession(t, false, true)
	s.replayMessages(msgs)
	if got := ups(); len(got) != 3 || got[0]["sessionUpdate"] != updCompactionChunk || got[1]["sessionUpdate"] != updCompaction || got[2]["sessionUpdate"] != updAgentMessage {
		t.Fatalf("with capability: %v", got)
	}
	s2, ups2 := bareSession(t, false, false)
	s2.replayMessages(msgs)
	if got := ups2(); len(got) != 2 || !strings.Contains(mustJSON(got[0]), "earlier conversation was summarized") {
		t.Fatalf("without capability: %v", got)
	}
}

// The client's {} vs null decides: notices on, compaction off. testEmit
// (tests only) fires a warning on the new session as soon as it is ready.
func TestInitializeRecordsClientCapabilities(t *testing.T) {
	o := testOptions(t, newScript(t))
	var notices, compaction bool
	o.testEmit = func(s *session) {
		notices, compaction = s.srv.clientSupports()
		s.emit(agent.Warning{Text: "hello"})
	}
	c := startClient(t, o)
	c.call("initialize", map[string]any{"protocolVersion": 1,
		"clientCapabilities": map[string]any{"session": map[string]any{"notices": map[string]any{}, "compaction": nil}}})
	c.newSession(t.TempDir())
	deadline := time.Now().Add(5 * time.Second)
	for len(c.updatesOf(updNotice)) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n, th := len(c.updatesOf(updNotice)), len(c.updatesOf(updThought)); n != 1 || th != 0 {
		t.Fatalf("notices advertised: want 1 notice and no thought line, got %d / %d", n, th)
	}
	if !notices || compaction {
		t.Fatalf("parsed capabilities: notices %v (want true), compaction %v (want false: null)", notices, compaction)
	}
}

func TestSessionInfoAfterPrompts(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("a"), textTurn("b"))))
	sid := c.newSession(t.TempDir())
	c.prompt(sid, "fix the parser\nit chokes on tabs")
	c.prompt(sid, "and the lexer")
	info := c.updatesOf(updSessionInfo)
	if len(info) != 2 || info[0]["title"] != "fix the parser" || info[1]["title"] != nil {
		t.Fatalf("title once, from the first prompt's first line: %v", info)
	}
	for _, u := range info {
		if _, err := time.Parse(time.RFC3339, u["updatedAt"].(string)); err != nil {
			t.Fatalf("updatedAt: %v", u)
		}
		conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: u})
	}
}

func TestEffortOption(t *testing.T) {
	thinking := provider.Model{Provider: "p", ID: "m", ThinkingMode: "openai",
		ThinkingLevelMap: map[llm.Effort]string{llm.EffortLow: "low", llm.EffortHigh: "high"}}
	if got := effortLevels(thinking); len(got) != 2 || got[0] != llm.EffortLow || got[1] != llm.EffortHigh {
		t.Fatalf("levels %v", got)
	}
	o := effortOption(thinking, llm.EffortHigh)
	if o == nil || o.ID != "effort" || o.Category != "thought_level" || o.CurrentValue != "high" || len(o.Options) != 2 {
		t.Fatalf("%+v", o)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: "abcd1234",
		Update: ConfigOptionUpdate{SessionUpdate: updConfigOptions, ConfigOptions: []ConfigOption{*o}}})
	if effortOption(provider.Model{ThinkingMode: "none"}, llm.EffortOff) != nil {
		t.Fatal("a model without thinking has nothing to pick")
	}
}

func TestConfigOptionUpdateWhenCredentialAppears(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("a"), textTurn("b"))))
	sid := c.newSession(t.TempDir())
	c.prompt(sid, "one")
	if n := len(c.updatesOf(updConfigOptions)); n != 0 {
		t.Fatalf("nothing changed, yet %d config_option_update", n)
	}
	t.Setenv("MOCA_T_OTHER", "k2") // as if the user ran `moca login` in a terminal
	c.prompt(sid, "two")
	ups := c.updatesOf(updConfigOptions)
	if len(ups) != 1 || !strings.Contains(mustJSON(ups[0]), "other/x") {
		t.Fatalf("%v", ups)
	}
}
