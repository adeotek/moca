// Package acp serves the Agent Client Protocol (ACP v1) over stdio: editors
// such as Zed launch `moca acp` as a child process and drive moca sessions
// through it. JSON-RPC 2.0 is hand-rolled (as in internal/mcp); the peer is
// bidirectional — it serves the client's requests and calls the client back
// (session/request_permission).
package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

const (
	codeParse            = -32700
	codeInvalidRequest   = -32600
	codeMethodNotFound   = -32601
	codeInvalidParams    = -32602
	codeInternal         = -32603
	codeAuthRequired     = -32000 // ACP: authentication required
	codeResourceNotFound = -32002 // ACP: e.g. an unknown sessionId
	codeRequestCancelled = -32800 // ACP: answered after $/cancel_request
)

// RPCError is a JSON-RPC error object; handlers return it to choose the code.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

func invalidParams(format string, a ...any) error {
	return &RPCError{Code: codeInvalidParams, Message: fmt.Sprintf(format, a...)}
}

func notFound(format string, a ...any) error {
	return &RPCError{Code: codeResourceNotFound, Message: fmt.Sprintf(format, a...)}
}

// busy reports a request the session cannot take while a prompt runs.
func busy(msg string) error { return &RPCError{Code: codeInvalidRequest, Message: msg} }

var errClosed = errors.New("acp: connection closed")

// Handler serves incoming messages. Request runs on its own goroutine;
// Notify runs on the read loop, in arrival order, and must not block.
type Handler interface {
	Request(ctx context.Context, method string, params json.RawMessage) (any, error)
	Notify(ctx context.Context, method string, params json.RawMessage)
}

// Deferred lets a handler run follow-up work after its response is written
// (updates the client can only route once it has the response, e.g.
// available_commands_update after session/new).
type Deferred struct {
	Result any
	Then   func()
}

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Conn is one JSON-RPC peer over newline-delimited JSON.
type Conn struct {
	w        io.Writer
	wmu      sync.Mutex
	h        Handler
	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan message
	inflight map[string]context.CancelFunc // incoming request id → its cancel (guarded by mu)
	closed   bool
	wg       sync.WaitGroup
}

func NewConn(w io.Writer, h Handler) *Conn {
	return &Conn{w: w, h: h, pending: map[int64]chan message{}, inflight: map[string]context.CancelFunc{}}
}

// Serve reads messages until r ends or ctx is done, then cancels in-flight
// handlers' context, fails pending outgoing calls, and waits for the
// handlers to return. EOF is a normal end (nil error).
func (c *Conn) Serve(ctx context.Context, r io.Reader) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 64<<20)
		for sc.Scan() {
			lines <- bytes.Clone(sc.Bytes())
		}
		readErr <- sc.Err()
		close(lines)
	}()
	var err error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case line, ok := <-lines:
			if !ok {
				err = <-readErr
				break loop
			}
			c.dispatch(ctx, line)
		}
	}
	cancel()
	c.failPending()
	c.wg.Wait()
	return err
}

func (c *Conn) dispatch(ctx context.Context, line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var m message
	if err := json.Unmarshal(line, &m); err != nil {
		c.write(message{ID: json.RawMessage("null"), Error: &RPCError{Code: codeParse, Message: "parse error"}})
		return
	}
	switch {
	case m.Method == "$/cancel_request" && len(m.ID) == 0:
		c.cancelInflight(m.Params)
	case m.Method != "" && len(m.ID) > 0:
		// Register the cancel func here, before the goroutine starts: a
		// $/cancel_request right behind this request runs inline on the read
		// loop and would otherwise find nothing to cancel.
		reqCtx, cancel := context.WithCancel(ctx)
		key := string(bytes.TrimSpace(m.ID))
		c.mu.Lock()
		c.inflight[key] = cancel
		c.mu.Unlock()
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			defer func() {
				c.mu.Lock()
				delete(c.inflight, key)
				c.mu.Unlock()
				cancel()
			}()
			c.serveRequest(reqCtx, m)
		}()
	case m.Method != "":
		c.h.Notify(ctx, m.Method, m.Params)
	case len(m.ID) > 0:
		c.deliver(m)
	}
}

func (c *Conn) serveRequest(ctx context.Context, m message) {
	res, err := c.h.Request(ctx, m.Method, m.Params)
	var then func()
	if d, ok := res.(Deferred); ok {
		res, then = d.Result, d.Then
	}
	out := message{ID: m.ID}
	if err != nil {
		var re *RPCError
		switch {
		case errors.As(err, &re):
		case ctx.Err() != nil && errors.Is(err, context.Canceled):
			// $/cancel_request or shutdown: ACP's "request cancelled".
			re = &RPCError{Code: codeRequestCancelled, Message: "request cancelled"}
		default:
			re = &RPCError{Code: codeInternal, Message: err.Error()}
		}
		out.Error = re
	} else if b, merr := json.Marshal(res); merr != nil {
		out.Error = &RPCError{Code: codeInternal, Message: merr.Error()}
	} else {
		out.Result = b
	}
	c.write(out)
	if then != nil && out.Error == nil {
		then()
	}
}

// cancelInflight serves $/cancel_request: the named request's context is
// cancelled and its handler still answers — a result such as stopReason
// "cancelled", or -32800. Unknown and finished ids are ignored.
func (c *Conn) cancelInflight(params json.RawMessage) {
	var p struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	c.mu.Lock()
	cancel := c.inflight[string(bytes.TrimSpace(p.RequestID))]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *Conn) deliver(m message) {
	id, err := strconv.ParseInt(string(m.ID), 10, 64)
	if err != nil {
		return
	}
	c.mu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ch != nil {
		ch <- m
	}
}

func (c *Conn) failPending() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
}

func (c *Conn) write(m message) error {
	m.JSONRPC = "2.0"
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.w.Write(append(b, '\n'))
	return err
}

// Notify sends a notification to the client.
func (c *Conn) Notify(method string, params any) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(message{Method: method, Params: p})
}

// Call sends a request to the client and waits for its response; result
// (if non-nil) receives the decoded result. A closed connection is errClosed.
func (c *Conn) Call(ctx context.Context, method string, params, result any) error {
	p, err := json.Marshal(params)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errClosed
	}
	c.nextID++
	id := c.nextID
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.write(message{ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: p}); err != nil {
		return err
	}
	select {
	case m, ok := <-ch:
		if !ok {
			return errClosed
		}
		if m.Error != nil {
			return m.Error
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-ctx.Done():
		// Cascade the cancellation (ACP): the client may drop its UI for this
		// request; a late answer finds no pending entry and is ignored.
		c.Notify("$/cancel_request", map[string]int64{"requestId": id})
		return ctx.Err()
	}
}
