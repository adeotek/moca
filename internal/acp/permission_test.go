package acp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func rmScript(t *testing.T) (*scriptServer, string) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.txt"), []byte("x"), 0o644)
	return newScript(t, toolTurn([2]string{"shell", `{"command":"rm x.txt"}`}), textTurn("done")), work
}

func TestPermissionAllowOnce(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "delete x"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if len(c.perms) != 1 {
		t.Fatalf("permission requests: %d", len(c.perms))
	}
	p := c.perms[0]
	if p.ToolCall.ToolCallID != c.updatesOf(updToolCall)[0]["toolCallId"] {
		t.Errorf("permission not tied to the running tool call: %+v", p.ToolCall)
	}
	kinds := map[string]bool{}
	for _, o := range p.Options {
		kinds[o.Kind] = true
	}
	if !kinds["allow_once"] || !kinds["reject_once"] || kinds["allow_always"] {
		t.Errorf("options %+v (rm must not offer allow_always)", p.Options)
	}
	conform(t, "RequestPermissionRequest", p)
	if _, err := os.Stat(filepath.Join(work, "x.txt")); !os.IsNotExist(err) {
		t.Fatal("approved rm did not run")
	}
}

func TestPermissionDeny(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse { return selected(optReject) }
	sid := c.newSession(work)
	c.prompt(sid, "delete x")
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("denied rm ran")
	}
	if ends := c.updatesOf(updToolUpdate); len(ends) != 1 || ends[0]["status"] != statusFailed {
		t.Fatalf("denied call should fail: %v", ends)
	}
}

func TestCancelDuringPermission(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	second := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.prompt(sid, "again") // a concurrent prompt is refused
		second <- err
		c.notify("session/cancel", CancelNotification{SessionID: sid})
		time.Sleep(50 * time.Millisecond)
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if e := <-second; e == nil || e.Code != codeInvalidRequest {
		t.Fatalf("concurrent prompt: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("cancelled rm ran")
	}
}

func TestEOFDuringPermissionDenies(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, testOptions(t, s))
	sid := c.newSession(work)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		c.in.Close() // the editor goes away mid-approval
		select {}
	}
	go c.prompt(sid, "delete x")
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
		t.Fatal("Serve hung after EOF during a permission request")
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("rm ran without an answer")
	}
}
