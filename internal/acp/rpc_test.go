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
	case "deferred":
		return Deferred{Result: map[string]int{"n": 1}, Then: func() { close(h.then) }}, nil
	case "bad":
		return nil, invalidParams("bad %s", "input")
	}
	return nil, &RPCError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func (h *echoHandler) Notify(ctx context.Context, method string, params json.RawMessage) {
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
