package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeAdvertisesLifecycle(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, _ := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	var r InitializeResponse
	json.Unmarshal(res, &r)
	sc := r.AgentCapabilities.SessionCapabilities
	if sc.Close == nil || sc.Resume == nil || sc.Delete == nil || sc.List == nil {
		t.Fatalf("%s", res)
	}
}

// session/close mid-prompt: the prompt ends as cancelled, the session is gone.
func TestCloseCancelsAndFrees(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	closed := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/close", CloseSessionRequest{SessionID: sid})
		closed <- err
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if e := <-closed; e != nil {
		t.Fatalf("close: %+v", e)
	}
	if _, err := c.prompt(sid, "again"); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("prompt after close: %+v", err)
	}
	if _, err := c.call("session/close", CloseSessionRequest{SessionID: sid}); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("second close: %+v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("rm ran after close")
	}
}

// A stored session carries its own model: a config without "model" must not
// block load/resume — the same rule as the CLI, where the check gates -p and
// not --resume. (session/new still refuses: it has no stored model.)
func TestReopenWithoutConfiguredModel(t *testing.T) {
	work := t.TempDir()
	s := newScript(t, textTurn("first"), textTurn("second"))
	o := testOptions(t, s)
	c := startClient(t, o)
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "hello"); err != nil || stop != stopEndTurn {
		t.Fatalf("first prompt: %q %+v", stop, err)
	}
	c.stop()             // releases the single-writer session lock
	o2 := o              // same data dir (testOptions re-isolates XDG_DATA_HOME per call)
	o2.Config.Model = "" // no configured model: the session's own model wins
	c2 := startClient(t, o2)
	for _, m := range []string{"session/resume", "session/load"} {
		if _, err := c2.call(m, ResumeSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}}); err != nil {
			t.Fatalf("%s without a configured model: %+v", m, err)
		}
	}
	if stop, err := c2.prompt(sid, "again"); err != nil || stop != stopEndTurn {
		t.Fatalf("prompt after reopen: %q %+v", stop, err)
	}
}

func TestResumeWithoutReplay(t *testing.T) {
	work := t.TempDir()
	s := newScript(t, textTurn("first"))
	o := testOptions(t, s)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "remember the word kumquat")
	c.stop() // releases the single-writer session lock
	c2 := startClient(t, o)
	res, err := c2.call("session/resume", ResumeSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r ResumeSessionResponse
	json.Unmarshal(res, &r)
	conform(t, "ResumeSessionResponse", r)
	if r.Modes == nil || len(r.ConfigOptions) == 0 {
		t.Fatalf("%s", res)
	}
	if n := len(c2.updatesOf(updUserMessage)) + len(c2.updatesOf(updAgentMessage)); n != 0 {
		t.Fatalf("resume replayed %d chunks", n)
	}
	s.mu.Lock()
	s.turns = append(s.turns, textTurn("kumquat"))
	s.mu.Unlock()
	if stop, err := c2.prompt(sid, "which word?"); err != nil || stop != stopEndTurn {
		t.Fatalf("prompt after resume: %q %+v", stop, err)
	}
	if !strings.Contains(lastUserText(t, s), "which word?") {
		t.Fatal("the follow-up prompt did not reach the model")
	}
	s.mu.Lock()
	last := mustJSON(s.bodies[len(s.bodies)-1])
	s.mu.Unlock()
	if !strings.Contains(last, "remember the word kumquat") {
		t.Fatal("the resumed history did not reach the model")
	}
}

// An id that is open here must not be reused for another workdir.
func TestReopenOpenSessionOtherCwd(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := c.newSession(t.TempDir())
	for _, m := range []string{"session/resume", "session/load"} {
		_, err := c.call(m, ResumeSessionRequest{SessionID: sid, Cwd: t.TempDir(), MCPServers: []json.RawMessage{}})
		if err == nil || err.Code != codeInvalidParams || !strings.Contains(err.Message, "another directory") {
			t.Fatalf("%s from another cwd: %+v", m, err)
		}
	}
}

func TestDeleteSession(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("ok")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	// Open and idle in this process: delete closes it first (our own writer
	// holds the file lock), then removes the file.
	if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: sid}); err != nil {
		t.Fatalf("delete: %+v", err)
	}
	res, _ := c.call("session/list", ListSessionsRequest{Cwd: work})
	var l ListSessionsResponse
	json.Unmarshal(res, &l)
	if len(l.Sessions) != 0 {
		t.Fatalf("deleted session still listed: %+v", l)
	}
	// Deleting an already-deleted or never-created id succeeds silently
	// (session-delete spec, SHOULD; ACP TCK ACP-DELETE-002).
	for _, id := range []string{sid, "deadbeef"} {
		if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: id}); err != nil {
			t.Fatalf("delete of unknown %s: %+v", id, err)
		}
	}
	// A non-id8 id names no moca session: silent success, and "last" must
	// never reach session.Find (it would resolve to the newest session).
	keep := c.newSession(work)
	c.prompt(keep, "hello")
	for _, id := range []string{"last", "tck-never-created-session"} {
		if _, err := c.call("session/delete", DeleteSessionRequest{SessionID: id}); err != nil {
			t.Fatalf("delete %q: %+v", id, err)
		}
	}
	res, _ = c.call("session/list", ListSessionsRequest{Cwd: work})
	json.Unmarshal(res, &l)
	if len(l.Sessions) != 1 || l.Sessions[0].SessionID != keep {
		t.Fatalf("delete of a non-id must not remove %s: %+v", keep, l)
	}
}

func TestDeleteRunningSessionRefused(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	got := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/delete", DeleteSessionRequest{SessionID: sid})
		got <- err
		return selected(optReject)
	}
	c.prompt(sid, "delete x")
	if e := <-got; e == nil || e.Code != codeInvalidRequest {
		t.Fatalf("delete mid-prompt: %+v", e)
	}
}
