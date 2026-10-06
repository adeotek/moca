package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/tools"
)

// keepEnv is the environment a stdio server inherits from moca (§10.5):
// enough to run and resolve binaries, nothing that carries credentials.
var keepEnv = []string{"PATH", "HOME", "USER", "LANG", "TERM", "TMPDIR"}

// keepEnvWindows adds what a Windows process needs to run at all (Windows
// reports names in mixed case, so the comparison there is case-insensitive).
var keepEnvWindows = []string{"SYSTEMROOT", "SYSTEMDRIVE", "WINDIR", "USERPROFILE", "USERNAME", "APPDATA",
	"LOCALAPPDATA", "PROGRAMDATA", "PROGRAMFILES", "TEMP", "TMP", "PATHEXT", "COMSPEC"}

// keepName reports whether an inherited variable survives the filter on goos.
func keepName(goos, k string) bool {
	if goos == "windows" {
		k = strings.ToUpper(k)
		return strings.HasPrefix(k, "XDG_") || slices.Contains(keepEnv, k) || slices.Contains(keepEnvWindows, k)
	}
	return strings.HasPrefix(k, "XDG_") || slices.Contains(keepEnv, k)
}

func FilterEnv(base []string) []string {
	var out []string
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if keepName(runtime.GOOS, k) {
			out = append(out, kv)
		}
	}
	return out
}

func serverEnv(base []string, extra map[string]string) ([]string, error) {
	env := FilterEnv(base)
	for k, v := range extra {
		r, err := config.ResolveEnv(v)
		if err != nil {
			return nil, err
		}
		// An explicit entry replaces the inherited one of the same name:
		// duplicate names resolve platform-dependently (glibc keeps the
		// first), so appending an override would be silently ignored.
		env = slices.DeleteFunc(env, func(kv string) bool {
			bk, _, _ := strings.Cut(kv, "=")
			if runtime.GOOS == "windows" {
				return strings.EqualFold(bk, k)
			}
			return bk == k
		})
		env = append(env, k+"="+r)
	}
	return env, nil
}

// ring keeps the last 4 KiB of a server's stderr (and any non-JSON stdout
// lines) for error reporting.
type ring struct {
	mu  sync.Mutex
	buf []byte
}

func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > 4096 {
		r.buf = r.buf[len(r.buf)-4096:]
	}
	r.mu.Unlock()
	return len(p), nil
}

func (r *ring) tail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(r.buf)), "\n")
	return strings.Join(lines[max(0, len(lines)-5):], " | ")
}

// callResult is what a pending call receives: the server's response, or the
// transport's own failure (kept as an error so errors.Is sees its sentinel).
type callResult struct {
	resp response
	err  error
}

// stdioTransport speaks line-delimited JSON-RPC over a server subprocess.
// One reader goroutine routes responses to pending calls by id, answers
// server→client requests (ping only) and logs everything else that is not
// JSON to the stderr ring. A process exit fails every pending call with the
// ring's tail instead of hanging them.
type stdioTransport struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	wmu     sync.Mutex
	nextID  atomic.Int64
	mu      sync.Mutex
	pending map[int64]chan callResult
	dead    error
	logs    *ring
	done    chan struct{}
}

// exitGrace bounds how long the exit handler waits for the stdout reader to
// drain after the process is reaped (a grandchild holding the pipe open would
// otherwise keep it waiting forever).
const exitGrace = 500 * time.Millisecond

func startStdio(_ context.Context, name string, s config.MCPServer, baseEnv []string) (transport, error) {
	env, err := serverEnv(baseEnv, s.Env)
	if err != nil {
		return nil, fmt.Errorf("mcp server %s: %w", name, err)
	}
	cmd := exec.Command(s.Command, s.Args...)
	cmd.Env = env
	cmd.WaitDelay = exitGrace // a grandchild holding stderr open must not stall Wait
	logs := &ring{}
	cmd.Stderr = logs
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	// The stdout pipe is ours, not cmd.StdoutPipe's: Wait closes that one the
	// moment the process is reaped, discarding output the reader has not
	// consumed yet (a reply followed by an exit, or a fatal message on stdout).
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = pw
	tools.SetProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, fmt.Errorf("mcp server %s: %w", name, err)
	}
	pw.Close() // the child holds the write end now
	t := &stdioTransport{name: name, cmd: cmd, stdin: stdin, pending: map[int64]chan callResult{}, logs: logs, done: make(chan struct{})}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		t.readLoop(pr)
	}()
	go func() {
		werr := cmd.Wait()
		select { // let the reader drain what the server wrote before exiting
		case <-readDone:
		case <-time.After(exitGrace):
		}
		pr.Close()
		t.fail(fmt.Errorf("%w: server %s exited (%v): %s", errTransportDead, name, werr, logs.tail()))
		close(t.done)
	}()
	return t, nil
}

func (t *stdioTransport) fail(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dead == nil {
		t.dead = err
	}
	for id, ch := range t.pending {
		ch <- callResult{err: t.dead}
		delete(t.pending, id)
	}
}

func (t *stdioTransport) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var m response
		if json.Unmarshal(line, &m) != nil || m.JSONRPC != "2.0" {
			t.logs.Write(append(append([]byte{}, line...), '\n'))
			continue
		}
		if m.Method != "" { // server→client request or notification
			if m.ID != nil {
				var reply any = map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": -32601, "message": "not supported by moca"}}
				if m.Method == "ping" {
					reply = map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{}}
				}
				t.write(reply)
			}
			continue
		}
		if m.ID == nil {
			continue
		}
		id, ok := responseID(m.ID)
		if !ok {
			continue
		}
		t.mu.Lock()
		ch := t.pending[id]
		delete(t.pending, id)
		t.mu.Unlock()
		if ch != nil {
			ch <- callResult{resp: m}
		}
	}
}

func (t *stdioTransport) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	_, err = t.stdin.Write(append(b, '\n'))
	return err
}

func (t *stdioTransport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := t.nextID.Add(1)
	ch := make(chan callResult, 1)
	t.mu.Lock()
	dead := t.dead
	if dead != nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("%w: %w", errNotSent, dead)
	}
	t.pending[id] = ch
	t.mu.Unlock()
	if err := t.write(request{JSONRPC: "2.0", ID: &id, Method: method, Params: params}); err != nil {
		t.mu.Lock()
		delete(t.pending, id)
		dead := t.dead
		t.mu.Unlock()
		if dead == nil {
			// The pipe is gone: the server died. Wait for the exit to be
			// observed so the caller gets the recorded error (with the stderr
			// tail) rather than a bare EPIPE or a context error.
			select {
			case <-t.done:
			case <-ctx.Done():
			}
			t.mu.Lock()
			dead = t.dead
			t.mu.Unlock()
		}
		if dead != nil {
			return nil, fmt.Errorf("%w: %w", errNotSent, dead)
		}
		return nil, ctx.Err()
	}
	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		if r.resp.Error != nil {
			return nil, r.resp.Error
		}
		return r.resp.Result, nil
	}
}

func (t *stdioTransport) Notify(_ context.Context, method string, params any) error {
	return t.write(request{JSONRPC: "2.0", Method: method, Params: params})
}

// Close kills the whole process group (a server may spawn children) and
// waits for the exit to be observed. It is idempotent, and the wait is
// bounded: a child stuck in uninterruptible sleep must not hang shutdown
// (Agent.Close runs it at session end).
func (t *stdioTransport) Close() error {
	t.stdin.Close()
	tools.KillProcessGroup(t.cmd)
	select {
	case <-t.done:
	case <-time.After(5 * time.Second):
	}
	return nil
}
