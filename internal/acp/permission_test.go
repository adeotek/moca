package acp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	c := startClient(t, askOptions(t, s))
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
	kinds := map[string]string{}
	for _, o := range p.Options {
		kinds[o.Kind] = o.OptionID
	}
	// rm now offers the session answer too — ACP has no persisted allow, so
	// the always-kind option is the in-memory one and is offered everywhere.
	if kinds["allow_once"] != optAllowOnce || kinds["reject_once"] != optReject || kinds["allow_always"] != optAllowSession {
		t.Errorf("options %+v", p.Options)
	}
	conform(t, "RequestPermissionRequest", p)
	if _, err := os.Stat(filepath.Join(work, "x.txt")); !os.IsNotExist(err) {
		t.Fatal("approved rm did not run")
	}
}

// TestPermissionAllowSessionRemembersExactCommand: an rm approved "for this
// session" is not asked about again when the exact same line repeats, and a
// different rm still asks (the exact tier, keyed by the full command line).
func TestPermissionAllowSessionRemembersExactCommand(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "x.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(work, "y.txt"), []byte("y"), 0o644)
	rm := func(f string) [2]string { return [2]string{"shell", `{"command":"rm ` + f + `"}`} }
	s := newScript(t, toolTurn(rm("x.txt")), toolTurn(rm("x.txt")), toolTurn(rm("y.txt")), textTurn("done"))
	c := startClient(t, askOptions(t, s))
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse { return selected(optAllowSession) }
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "delete them"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if len(c.perms) != 2 {
		t.Fatalf("want 2 questions (x.txt once, y.txt once), got %d", len(c.perms))
	}
	if p := c.perms[0]; p.ToolCall.ToolCallID == "" || len(p.Options) != 3 {
		t.Fatalf("first question %+v", p)
	}
	// The editor's row must say what it allows: only this exact line, as the
	// TUI's row does — "Allow for this session" read as every rm.
	for _, o := range c.perms[0].Options {
		if o.OptionID == optAllowSession && o.Name != "Allow this exact command for this session" {
			t.Errorf("exact-tier session option is labelled %q", o.Name)
		}
	}
}

// TestPermissionRedirectHopOffersNoSessionOption: a web fetch's redirect hop is
// an excluded question (NoSession) — it offers once/deny only, and an editor
// that picks allow-session anyway is answered with deny (no fetch of the
// target). The fetch's own host is an ordinary question and keeps the session
// row.
func TestPermissionRedirectHopOffersNoSessionOption(t *testing.T) {
	var hit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		w.Write([]byte("internal"))
	}))
	defer target.Close()
	_, port, err := net.SplitHostPort(strings.TrimPrefix(target.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	local := "http://localhost:" + port + "/"
	hop := httptest.NewServer(http.RedirectHandler(local, http.StatusFound))
	defer hop.Close()

	s := newScript(t,
		toolTurn([2]string{"web", `{"op":"fetch","url":"` + hop.URL + `/"}`}),
		textTurn("done"))
	c := startClient(t, askOptions(t, s))
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse { return selected(optAllowSession) }
	sid := c.newSession(t.TempDir())
	if stop, err := c.prompt(sid, "fetch it"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if len(c.perms) != 2 {
		t.Fatalf("want a question for the start host and one for the hop, got %d", len(c.perms))
	}
	if p := c.perms[0]; len(p.Options) != 3 {
		t.Fatalf("the fetch's own host must offer the session answer: %+v", p.Options)
	}
	hopQ := c.perms[1]
	if len(hopQ.Options) != 2 {
		t.Fatalf("an excluded question must offer only once/deny: %+v", hopQ.Options)
	}
	for _, o := range hopQ.Options {
		if o.OptionID == optAllowSession {
			t.Fatalf("the session option must not be offered for a redirect hop: %+v", hopQ.Options)
		}
	}
	if hit.Load() {
		t.Fatal("a denied hop still fetched the target")
	}
	if ends := c.updatesOf(updToolUpdate); len(ends) != 1 || ends[0]["status"] != statusFailed {
		t.Fatalf("the fetch should have failed: %v", ends)
	}
}

func TestPermissionDeny(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, askOptions(t, s))
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
	c := startClient(t, askOptions(t, s))
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
	c := startClient(t, askOptions(t, s))
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

// session/cancel while the editor sits on a permission prompt: the agent
// must not wait for the answer, and must tell the client to drop the prompt.
func TestSessionCancelCascadesToPermission(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, askOptions(t, s))
	sid := c.newSession(work)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		c.notify("session/cancel", CancelNotification{SessionID: sid})
		<-release // the editor never answers
		return cancelledOutcome()
	}
	stop, err := c.prompt(sid, "delete x")
	if err != nil || stop != stopCancelled {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	c.mu.Lock()
	n := len(c.cancels)
	c.mu.Unlock()
	if n != 1 {
		t.Fatalf("want one $/cancel_request for the pending permission, got %d", n)
	}
	if _, err := os.Stat(filepath.Join(work, "x.txt")); err != nil {
		t.Fatal("cancelled rm ran")
	}
}
