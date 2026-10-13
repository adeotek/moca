package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAndLoad(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("x\n"), 0o644)
	s := newScript(t, toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("All good."))
	o := testOptions(t, s)
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "check a.txt please")

	res, err := c.call("session/list", ListSessionsRequest{Cwd: work})
	if err != nil {
		t.Fatal(err.Message)
	}
	var l ListSessionsResponse
	json.Unmarshal(res, &l)
	conform(t, "ListSessionsResponse", l)
	if len(l.Sessions) != 1 || l.Sessions[0].SessionID != sid || !strings.Contains(l.Sessions[0].Title, "check a.txt") {
		t.Fatalf("list %+v", l)
	}

	// A fresh process (the first one exited, releasing the single-writer
	// session lock; same data dir) reopens it.
	c.stop()
	c2 := startClient(t, o)
	res, err = c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var lr LoadSessionResponse
	json.Unmarshal(res, &lr)
	conform(t, "LoadSessionResponse", lr)
	users := c2.updatesOf(updUserMessage)
	if len(users) != 1 || !strings.Contains(mustJSON(users[0]), "check a.txt please") {
		t.Fatalf("user replay %v", users)
	}
	if c2.agentText() != "All good." {
		t.Fatalf("agent replay %q", c2.agentText())
	}
	if starts, ends := c2.updatesOf(updToolCall), c2.updatesOf(updToolUpdate); len(starts) != 1 || len(ends) != 1 || ends[0]["status"] != statusCompleted {
		t.Fatalf("tool replay %v / %v", starts, ends)
	}
	// The loaded session takes prompts.
	s.mu.Lock()
	s.turns = append(s.turns, textTurn("again"))
	s.mu.Unlock()
	if stop, err := c2.prompt(sid, "more"); err != nil || stop != stopEndTurn {
		t.Fatalf("prompt after load: %q %+v", stop, err)
	}
	// The resumed conversation reaches the model: the follow-up request
	// carries the first prompt.
	s.mu.Lock()
	last := mustJSON(s.bodies[len(s.bodies)-1]["messages"])
	s.mu.Unlock()
	if !strings.Contains(last, "check a.txt please") {
		t.Fatalf("follow-up request lost the loaded history: %s", last)
	}
}

// The load response must come after the last replayed update: a client
// that renders on the response would otherwise miss the tail.
func TestLoadRespondsAfterReplay(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("first answer")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	c.stop()
	c2 := startClient(t, o)
	if _, err := c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: work, MCPServers: []json.RawMessage{}}); err != nil {
		t.Fatal(err.Message)
	}
	// call returns when the response arrives; reading is sequential, so any
	// update recorded by then was written before the response.
	if c2.agentText() != "first answer" {
		t.Fatalf("replay incomplete at response time: %q", c2.agentText())
	}
}

func TestLoadRejectsNonID8(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	for _, id := range []string{"last", "abc", "../../etc"} {
		if _, err := c.call("session/load", LoadSessionRequest{SessionID: id, Cwd: t.TempDir()}); err == nil || err.Code != codeInvalidParams {
			t.Errorf("%q: %+v", id, err)
		}
	}
}

func TestLoadFromOtherWorkdirRefused(t *testing.T) {
	work := t.TempDir()
	o := testOptions(t, newScript(t, textTurn("ok")))
	c := startClient(t, o)
	sid := c.newSession(work)
	c.prompt(sid, "hello")
	c.stop()
	c2 := startClient(t, o)
	if _, err := c2.call("session/load", LoadSessionRequest{SessionID: sid, Cwd: t.TempDir()}); err == nil {
		t.Fatal("loaded a session into another workdir")
	}
}

func TestInitializeAdvertisesLoadAndList(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, _ := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	var r InitializeResponse
	json.Unmarshal(res, &r)
	if !r.AgentCapabilities.LoadSession || r.AgentCapabilities.SessionCapabilities.List == nil {
		t.Fatalf("%s", res)
	}
}
