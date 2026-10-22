package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// echoHandler answers "echo" with its params, "slow" after release is
// closed, and records notifications in order.
type echoHandler struct {
	mu      sync.Mutex
	notes   []string
	release chan struct{}
	then    chan struct{}
}

func (h *echoHandler) Request(ctx context.Context, method string, params json.RawMessage) (any, error) {
	switch method {
	case "echo":
		return params, nil
	case "slow":
		<-h.release
		return map[string]string{"ok": "slow"}, nil
	case "wait":
		<-ctx.Done()
		return nil, ctx.Err()
	case "deferred":
		return Deferred{Result: map[string]int{"n": 1}, Then: func() { close(h.then) }}, nil
	case "bad":
		return nil, invalidParams("bad %s", "input")
	case "panic":
		// A handler bug must not take the process (and the client's transport)
		// down: the Conn answers -32603 and keeps serving.
		panic("boom")
	case "panic-then":
		return Deferred{Result: map[string]int{"n": 2}, Then: func() { panic("then boom") }}, nil
	}
	return nil, &RPCError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func (h *echoHandler) Notify(ctx context.Context, method string, params json.RawMessage) {
	if method == "npanic" {
		panic("notify boom")
	}
	h.mu.Lock()
	h.notes = append(h.notes, method)
	h.mu.Unlock()
}

// pipePeer runs a Conn on pipes and returns the client's ends.
func pipePeer(t *testing.T, h Handler) (*Conn, io.WriteCloser, *bufio.Scanner, chan error) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := NewConn(outW, h)
	done := make(chan error, 1)
	go func() { done <- c.Serve(context.Background(), inR); outW.Close() }()
	sc := bufio.NewScanner(outR)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	t.Cleanup(func() { inW.Close() })
	return c, inW, sc, done
}

func readMsg(t *testing.T, sc *bufio.Scanner) map[string]any {
	t.Helper()
	if !sc.Scan() {
		t.Fatalf("no message: %v", sc.Err())
	}
	var m map[string]any
	if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
		t.Fatalf("bad JSON %q: %v", sc.Text(), err)
	}
	return m
}

func TestRequestResponse(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":7,"method":"echo","params":{"a":1}}`+"\n")
	m := readMsg(t, sc)
	if m["id"] != float64(7) || m["result"].(map[string]any)["a"] != float64(1) || m["jsonrpc"] != "2.0" {
		t.Fatalf("got %v", m)
	}
}

func TestErrors(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, "{not json\n")
	if m := readMsg(t, sc); m["error"].(map[string]any)["code"] != float64(codeParse) || m["id"] != nil {
		t.Fatalf("parse error: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":"x","method":"bad","params":{}}`+"\n")
	if m := readMsg(t, sc); m["id"] != "x" || m["error"].(map[string]any)["code"] != float64(codeInvalidParams) {
		t.Fatalf("invalid params: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"nope"}`+"\n")
	if m := readMsg(t, sc); m["error"].(map[string]any)["code"] != float64(codeMethodNotFound) {
		t.Fatalf("method not found: %v", m)
	}
}

func TestConcurrentDispatch(t *testing.T) {
	h := &echoHandler{release: make(chan struct{})}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"slow"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"echo","params":1}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(2) {
		t.Fatalf("a blocked request held up the next one: %v", m)
	}
	close(h.release)
	if m := readMsg(t, sc); m["id"] != float64(1) {
		t.Fatalf("got %v", m)
	}
}

func TestNotificationsInOrder(t *testing.T) {
	h := &echoHandler{}
	_, in, sc, _ := pipePeer(t, h)
	for _, m := range []string{"a", "b", "c"} {
		io.WriteString(in, `{"jsonrpc":"2.0","method":"`+m+`"}`+"\n")
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"echo","params":0}`+"\n")
	readMsg(t, sc)
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.Join(h.notes, "") != "abc" {
		t.Fatalf("notes %v", h.notes)
	}
}

func TestDeferredRunsAfterResponse(t *testing.T) {
	h := &echoHandler{then: make(chan struct{})}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"deferred"}`+"\n")
	m := readMsg(t, sc)
	if m["result"].(map[string]any)["n"] != float64(1) {
		t.Fatalf("got %v", m)
	}
	select {
	case <-h.then:
	case <-time.After(5 * time.Second):
		t.Fatal("Then never ran")
	}
}

func TestOutgoingCall(t *testing.T) {
	c, in, sc, _ := pipePeer(t, &echoHandler{})
	got := make(chan error, 1)
	var res struct{ V string }
	go func() { got <- c.Call(context.Background(), "client/thing", map[string]int{"q": 1}, &res) }()
	m := readMsg(t, sc)
	if m["method"] != "client/thing" {
		t.Fatalf("got %v", m)
	}
	id, _ := json.Marshal(m["id"])
	io.WriteString(in, `{"jsonrpc":"2.0","id":`+string(id)+`,"result":{"V":"yes"}}`+"\n")
	if err := <-got; err != nil || res.V != "yes" {
		t.Fatalf("err %v res %+v", err, res)
	}
}

func TestCallFailsOnEOF(t *testing.T) {
	c, in, sc, done := pipePeer(t, &echoHandler{})
	got := make(chan error, 1)
	go func() { got <- c.Call(context.Background(), "client/thing", nil, nil) }()
	readMsg(t, sc)
	in.Close()
	select {
	case err := <-got:
		if !errors.Is(err, errClosed) {
			t.Fatalf("err %v, want errClosed", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Call hung after EOF")
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve: %v", err)
	}
}

func TestLargeLine(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	big := strings.Repeat("x", 3<<20)
	go io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"echo","params":"`+big+`"}`+"\n")
	if m := readMsg(t, sc); len(m["result"].(string)) != len(big) {
		t.Fatal("large line lost")
	}
}

// Updates, responses and outgoing calls share one writer: under -race,
// concurrent Notify calls must never interleave bytes within a line.
func TestConcurrentWritesAreWholeLines(t *testing.T) {
	c, _, sc, _ := pipePeer(t, &echoHandler{})
	const g, n = 16, 50
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				c.Notify("session/update", map[string]string{"pad": strings.Repeat("y", 512)})
			}
		}()
	}
	for i := 0; i < g*n; i++ {
		if m := readMsg(t, sc); m["method"] != "session/update" {
			t.Fatalf("line %d: %v", i, m)
		}
	}
	wg.Wait()
}

func TestUnknownNotificationGetsNoReply(t *testing.T) {
	h := &echoHandler{}
	_, in, sc, _ := pipePeer(t, h)
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/whatever"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","id":9,"method":"echo","params":0}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(9) {
		t.Fatalf("a notification was answered: %v", m)
	}
}

func TestCancelRequest(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":5,"method":"wait"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":99}}`+"\n") // unknown: ignored
	io.WriteString(in, `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":5}}`+"\n")
	m := readMsg(t, sc)
	if m["id"] != float64(5) || m["error"].(map[string]any)["code"] != float64(codeRequestCancelled) {
		t.Fatalf("a cancelled request must answer -32800: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":"s","method":"echo","params":1}`+"\n")
	if m := readMsg(t, sc); m["id"] != "s" {
		t.Fatalf("connection unusable after a cancel: %v", m)
	}
}

func TestCallCascadesCancel(t *testing.T) {
	c, _, sc, _ := pipePeer(t, &echoHandler{})
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan error, 1)
	go func() { got <- c.Call(ctx, "client/thing", nil, nil) }()
	req := readMsg(t, sc)
	cancel()
	m := readMsg(t, sc)
	if m["method"] != "$/cancel_request" || m["params"].(map[string]any)["requestId"] != req["id"] {
		t.Fatalf("want $/cancel_request for id %v, got %v", req["id"], m)
	}
	if err := <-got; !errors.Is(err, context.Canceled) {
		t.Fatalf("Call: %v", err)
	}
}

// A handler panic is a bug, not a reason to kill the client's transport: the
// request is answered -32603 and the connection keeps serving. This is the
// shape an editor sees as "Incoming transport closed" when it is missing, and
// an external agent has no stderr to explain itself on (Zed on Windows).
func TestHandlerPanicAnswersAndKeepsServing(t *testing.T) {
	_, in, sc, done := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"panic"}`+"\n")
	m := readMsg(t, sc)
	code, _ := m["error"].(map[string]any)["code"].(float64)
	if m["id"] != float64(1) || int(code) != codeInternal {
		t.Fatalf("a panicking handler must answer -32603: %v", m)
	}
	if msg, _ := m["error"].(map[string]any)["message"].(string); !strings.Contains(msg, "boom") {
		t.Fatalf("the panic value belongs in the message: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"echo","params":3}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(2) || m["result"] != float64(3) {
		t.Fatalf("connection unusable after a panic: %v", m)
	}
	select {
	case err := <-done:
		t.Fatalf("serve ended after a recovered panic: %v", err)
	default:
	}
}

// A panic in a notification handler runs on the read loop: recover it there
// too, or one bad notification stops every later request.
func TestNotificationPanicKeepsTheLoop(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","method":"npanic"}`+"\n")
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"echo","params":"after"}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(1) || m["result"] != "after" {
		t.Fatalf("notification panic took the loop down: %v", m)
	}
}

// A deferred follow-up runs after its response is written; a panic there is
// logged, not fatal — the client already has its result.
func TestDeferredPanicKeepsServing(t *testing.T) {
	_, in, sc, _ := pipePeer(t, &echoHandler{})
	io.WriteString(in, `{"jsonrpc":"2.0","id":1,"method":"panic-then"}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(1) || m["result"].(map[string]any)["n"] != float64(2) {
		t.Fatalf("result before the panic: %v", m)
	}
	io.WriteString(in, `{"jsonrpc":"2.0","id":2,"method":"echo","params":4}`+"\n")
	if m := readMsg(t, sc); m["id"] != float64(2) || m["result"] != float64(4) {
		t.Fatalf("connection unusable after a deferred panic: %v", m)
	}
}
