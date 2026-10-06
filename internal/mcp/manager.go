package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// defaultHTTPTimeout bounds a whole HTTP MCP request (connect + body). The
// transport has no per-read stall timeout (ctx bounds it), so without this a
// server that accepts a POST, answers text/event-stream and wedges would hang
// a call until the user interrupts; the TUI's run context is unbounded.
const defaultHTTPTimeout = 15 * time.Minute

const (
	// defaultHandshakeTimeout bounds start + initialize + tools/list: a server
	// that never answers (a launcher waiting for a browser login, a wedged
	// process) must not hold the server lock for as long as the run lives.
	defaultHandshakeTimeout = 60 * time.Second
	// defaultCallTimeout bounds one tools/call on either transport.
	defaultCallTimeout = 15 * time.Minute
	// failBackoff is how long `search` remembers a failed start instead of
	// spawning the broken server again on every query.
	failBackoff = 2 * time.Minute
)

type Options struct {
	HTTP    *http.Client
	BaseEnv []string // os.Environ()
	// HandshakeTimeout and CallTimeout default to 60 s and 15 min.
	HandshakeTimeout, CallTimeout time.Duration
}

// state is one configured server's lifecycle: nothing runs until the first
// describe/call (or an index-missing search); the idle timer stops it.
type state struct {
	mu    sync.Mutex
	cfg   config.MCPServer
	cl    *client
	tools []Tool
	timer *time.Timer
	// failErr/failedAt remember the last failed start (set only when the
	// caller's own context was still live) for search's backoff.
	failErr  error
	failedAt time.Time
	// busy counts calls between their ensure and their response. The idle
	// timer only stops a server with busy == 0, so a stop can never kill a
	// call in flight.
	busy int
}

// Manager owns every configured MCP server for one moca session: lazy start,
// idle stop, restart, index refresh, and the persisted Index that makes
// search work with zero servers running.
type Manager struct {
	idle    time.Duration
	ix      *Index
	o       Options
	servers map[string]*state
	// closed is set by Close: a call failing during shutdown must not respawn
	// a server nothing would ever stop.
	closed atomic.Bool
	// saveErr is the last index-persist failure ("" when the last save worked).
	saveMu  sync.Mutex
	saveErr string
}

func NewManager(servers map[string]config.MCPServer, idle time.Duration, ix *Index, o Options) *Manager {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if o.HandshakeTimeout <= 0 {
		o.HandshakeTimeout = defaultHandshakeTimeout
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = defaultCallTimeout
	}
	m := &Manager{idle: idle, ix: ix, o: o, servers: map[string]*state{}}
	for n, s := range servers {
		m.servers[n] = &state{cfg: s}
	}
	return m
}

func (m *Manager) Servers() []string {
	var out []string
	for n := range m.servers {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}

func (m *Manager) get(name string) (*state, error) {
	st, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("unknown MCP server %q (configured: %s)", name, strings.Join(m.Servers(), ", "))
	}
	return st, nil
}

// ensure starts the server if needed: transport (stdio or HTTP), initialize,
// tools/list (every page), index refresh — all bounded by the handshake
// timeout. Caller holds st.mu.
func (m *Manager) ensure(ctx context.Context, name string, st *state) error {
	if st.cl != nil {
		return nil
	}
	if m.closed.Load() {
		return errors.New("mcp manager is closed")
	}
	hctx, cancel := context.WithTimeout(ctx, m.o.HandshakeTimeout)
	defer cancel()
	err := m.start(hctx, name, st)
	if err != nil && ctx.Err() == nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("mcp server %s: no answer within %s while starting", name, m.o.HandshakeTimeout)
		}
		st.failErr, st.failedAt = err, time.Now()
	}
	if err == nil {
		st.failErr = nil
	}
	return err
}

func (m *Manager) start(ctx context.Context, name string, st *state) error {
	var tr transport
	var err error
	if st.cfg.URL != "" {
		tr, err = startHTTP(name, st.cfg, m.o.HTTP)
	} else {
		tr, err = startStdio(ctx, name, st.cfg, m.o.BaseEnv)
	}
	if err != nil {
		return err
	}
	if err := initialize(ctx, tr); err != nil {
		tr.Close()
		return err
	}
	cl := &client{t: tr, name: name}
	tools, err := cl.listTools(ctx)
	if err != nil {
		tr.Close()
		return err
	}
	st.cl, st.tools = cl, tools
	m.ix.Put(name, ConfigHash(st.cfg), tools)
	saveErr := ""
	if err := m.ix.Save(); err != nil {
		saveErr = err.Error()
	}
	m.saveMu.Lock()
	m.saveErr = saveErr
	m.saveMu.Unlock()
	m.armTimer(name, st)
	return nil
}

// armTimer (re)starts the idle timer. Caller holds st.mu.
func (m *Manager) armTimer(_ string, st *state) {
	if st.timer != nil {
		st.timer.Stop()
	}
	st.timer = time.AfterFunc(m.idle, func() {
		st.mu.Lock()
		defer st.mu.Unlock()
		if st.busy == 0 && st.cl != nil {
			st.cl.t.Close()
			st.cl = nil
		}
	})
}

// Running lists the servers with a live transport, sorted.
func (m *Manager) Running() []string {
	var out []string
	for _, n := range m.Servers() {
		st := m.servers[n]
		st.mu.Lock()
		if st.cl != nil {
			out = append(out, n)
		}
		st.mu.Unlock()
	}
	return out
}

// Search ranks the persisted index. Servers missing from the index (or with
// a changed config) are started once to index them; they then follow the
// normal idle timeout. A server that failed to start is not retried for
// failBackoff. The notes name every server that could not be indexed (and an
// index that could not be saved) so the model and user can see why tools are
// missing; an error is returned only when nothing could be searched at all.
func (m *Manager) Search(ctx context.Context, query, server string) ([]Hit, []string, error) {
	names := m.Servers()
	if server != "" {
		if _, err := m.get(server); err != nil {
			return nil, nil, err
		}
		names = []string{server}
	}
	entries := map[string]IndexEntry{}
	var errs, notes []string
	for _, n := range names {
		st := m.servers[n]
		if e, ok := m.ix.Valid(n, ConfigHash(st.cfg)); ok {
			entries[n] = e
			continue
		}
		st.mu.Lock()
		var err error
		if st.failErr != nil && time.Since(st.failedAt) < failBackoff {
			err = st.failErr // known-broken: do not respawn on every query
		} else {
			err = m.ensure(ctx, n, st)
		}
		st.mu.Unlock()
		if err != nil {
			errs = append(errs, err.Error())
			notes = append(notes, fmt.Sprintf("server %s could not be indexed: %v", n, err))
			continue
		}
		entries[n], _ = m.ix.Valid(n, ConfigHash(st.cfg))
	}
	hits := Rank(query, entries, server, 20)
	if len(hits) == 0 && len(errs) > 0 {
		return nil, nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	m.saveMu.Lock()
	if m.saveErr != "" {
		notes = append(notes, "the MCP index could not be saved ("+m.saveErr+"): servers will be started again next session")
	}
	m.saveMu.Unlock()
	return hits, notes, nil
}

func (m *Manager) Describe(ctx context.Context, server, tool string) (Tool, error) {
	st, err := m.get(server)
	if err != nil {
		return Tool{}, err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := m.ensure(ctx, server, st); err != nil {
		return Tool{}, err
	}
	for _, t := range st.tools {
		if t.Name == tool {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("server %s has no tool %q; use action=search", server, tool)
}

// Call runs one tools/call, bounded by the call timeout. A dead transport
// (the stdio process exited, an HTTP session expired or its response ended
// without our id) is dropped so the next use restarts the server. The call is
// replayed once on the fresh server only when that cannot execute it twice:
// the request never left (errNotSent), the session had expired (the server
// rejected it before processing), or the tool is read-only. Otherwise the
// error says the call may have executed.
func (m *Manager) Call(ctx context.Context, server, tool string, args json.RawMessage) (CallResult, Tool, error) {
	for attempt := 0; ; attempt++ {
		// One critical section: ensure + tool lookup + busy++ + timer stop.
		// A lookup released the lock and re-locked in an earlier revision,
		// letting the idle timer nil st.cl in the window — a call landing
		// exactly at idle expiry then panicked on the nil client.
		st, err := m.get(server)
		if err != nil {
			return CallResult{}, Tool{}, err
		}
		st.mu.Lock()
		if err := m.ensure(ctx, server, st); err != nil {
			st.mu.Unlock()
			return CallResult{}, Tool{}, err
		}
		var t Tool
		for _, x := range st.tools {
			if x.Name == tool {
				t = x
				break
			}
		}
		if t.Name == "" {
			st.mu.Unlock()
			return CallResult{}, Tool{}, fmt.Errorf("server %s has no tool %q; use action=search", server, tool)
		}
		st.busy++
		if st.timer != nil {
			st.timer.Stop()
		}
		cl := st.cl
		st.mu.Unlock()
		cctx, cancel := context.WithTimeout(ctx, m.o.CallTimeout)
		res, err := cl.callTool(cctx, tool, args)
		cancel()
		st.mu.Lock()
		st.busy--
		// The classification keys on the transports' own sentinels, never on
		// server-supplied error text (a server error containing "exited" must
		// not restart a healthy server).
		expired := errors.Is(err, errSessionExpired)
		dead := errors.Is(err, errTransportDead)
		timedOut := err != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded)
		// A dead or wedged transport is shared: closing it here also fails any
		// other in-flight call on this server (each then reports its own
		// error). Errors, not panics — acceptable for v1.
		if (expired || dead || timedOut) && st.cl == cl {
			cl.t.Close()
			st.cl = nil
		}
		readOnly := isTrue(t.Annotations.ReadOnlyHint) && !isTrue(t.Annotations.DestructiveHint)
		replaySafe := expired || errors.Is(err, errNotSent) || readOnly
		retry := attempt == 0 && (expired || dead) && replaySafe && !m.closed.Load()
		if st.busy == 0 && st.cl != nil {
			m.armTimer(server, st)
		}
		st.mu.Unlock()
		if retry {
			continue
		}
		switch {
		case timedOut:
			err = fmt.Errorf("mcp server %s: %s did not answer within %s; the server was stopped and may have executed the call", server, tool, m.o.CallTimeout)
		case dead && !replaySafe:
			err = fmt.Errorf("%w (the call may have executed before the connection failed; check before retrying)", err)
		}
		return res, t, err
	}
}

// IndexAll starts every configured server once (used by `moca mcp index`).
func (m *Manager) IndexAll(ctx context.Context) (map[string]int, map[string]error) {
	counts, errs := map[string]int{}, map[string]error{}
	for _, n := range m.Servers() {
		st := m.servers[n]
		st.mu.Lock()
		err := m.ensure(ctx, n, st)
		if err == nil {
			counts[n] = len(st.tools)
		}
		st.mu.Unlock()
		if err != nil {
			errs[n] = err
		}
	}
	return counts, errs
}

func (m *Manager) Close() {
	m.closed.Store(true)
	for _, st := range m.servers {
		st.mu.Lock()
		if st.timer != nil {
			st.timer.Stop()
		}
		if st.cl != nil {
			st.cl.t.Close()
			st.cl = nil
		}
		st.mu.Unlock()
	}
}
