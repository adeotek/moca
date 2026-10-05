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

type Options struct {
	HTTP    *http.Client
	BaseEnv []string // os.Environ()
}

// state is one configured server's lifecycle: nothing runs until the first
// describe/call (or an index-missing search); the idle timer stops it.
type state struct {
	mu    sync.Mutex
	cfg   config.MCPServer
	cl    *client
	tools []Tool
	timer *time.Timer
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
}

func NewManager(servers map[string]config.MCPServer, idle time.Duration, ix *Index, o Options) *Manager {
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: defaultHTTPTimeout}
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
// tools/list (every page), index refresh. Caller holds st.mu.
func (m *Manager) ensure(ctx context.Context, name string, st *state) error {
	if st.cl != nil {
		return nil
	}
	if m.closed.Load() {
		return errors.New("mcp manager is closed")
	}
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
	m.ix.Save()
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
// normal idle timeout. Errors from individual servers surface only when no
// hits could be produced at all.
func (m *Manager) Search(ctx context.Context, query, server string) ([]Hit, error) {
	names := m.Servers()
	if server != "" {
		if _, err := m.get(server); err != nil {
			return nil, err
		}
		names = []string{server}
	}
	entries := map[string]IndexEntry{}
	var errs []string
	for _, n := range names {
		st := m.servers[n]
		if e, ok := m.ix.Valid(n, ConfigHash(st.cfg)); ok {
			entries[n] = e
			continue
		}
		st.mu.Lock()
		err := m.ensure(ctx, n, st)
		st.mu.Unlock()
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		entries[n], _ = m.ix.Valid(n, ConfigHash(st.cfg))
	}
	hits := Rank(query, entries, server, 20)
	if len(hits) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return hits, nil
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

// Call runs one tools/call. On a dead transport (the stdio process exited,
// or an HTTP session expired) it restarts the server and retries exactly
// once.
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
		res, err := cl.callTool(ctx, tool, args)
		st.mu.Lock()
		st.busy--
		// A dead transport is shared: closing it here also fails any other
		// in-flight call on this server, which then spends its own single
		// retry. Errors, not panics — acceptable for v1.
		// The retry keys on the transport's own sentinels, never on
		// server-supplied error text (a server error containing "exited"
		// must not restart a healthy server), and never during shutdown.
		retry := err != nil && attempt == 0 && (errors.Is(err, errSessionExpired) || errors.Is(err, errTransportDead))
		if retry && m.closed.Load() {
			retry = false
		}
		if retry && st.cl == cl {
			cl.t.Close()
			st.cl = nil
		}
		if st.busy == 0 && st.cl != nil {
			m.armTimer(server, st)
		}
		st.mu.Unlock()
		if retry {
			continue
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
