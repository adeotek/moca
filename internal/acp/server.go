package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
)

// Options configures Serve; cmd/moca fills it from the config and flags.
type Options struct {
	Config     config.Config
	ConfigPath string
	Model      string // --model override ("" = config)
	Effort     string // --effort override
	Mode       string // --mode value ("" = resolved per session)
	// Trust decides project trust for a session's workdir (cmd/moca's
	// resolver: flag > yolo > saved trust.json decision > trusted). nil
	// trusts every workdir (tests).
	Trust   func(cwd string) (bool, error)
	Version string
	HTTP    *http.Client // tests
}

// Server maps ACP sessions onto moca agents.
type Server struct {
	opts     Options
	conn     *Conn
	mu       sync.Mutex
	sessions map[string]*session
}

// Serve speaks ACP on r/w until r ends or ctx is done, then closes every
// session's agent (which also stops its MCP servers).
func Serve(ctx context.Context, r io.Reader, w io.Writer, o Options) error {
	srv := &Server{opts: o, sessions: map[string]*session{}}
	srv.conn = NewConn(w, srv)
	err := srv.conn.Serve(ctx, r)
	srv.closeAll()
	return err
}

func (srv *Server) closeAll() {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	for id, s := range srv.sessions {
		s.agent.Close()
		delete(srv.sessions, id)
	}
}

func decode[T any](params json.RawMessage) (T, error) {
	var v T
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}
	if err := json.Unmarshal(params, &v); err != nil {
		return v, invalidParams("invalid params: %v", err)
	}
	return v, nil
}

func (srv *Server) Request(ctx context.Context, method string, params json.RawMessage) (any, error) {
	slog.Debug("acp request", "method", method)
	switch method {
	case "initialize":
		return srv.initialize(params)
	case "authenticate":
		return nil, &RPCError{Code: codeAuthRequired, Message: "moca uses its own credentials: run `moca login <provider>` in a terminal"}
	case "session/new":
		return srv.newSession(params)
	case "session/prompt":
		return srv.prompt(ctx, params)
	case "session/set_mode":
		return srv.setMode(params)
	case "session/set_config_option":
		return srv.setConfigOption(params)
	case "session/list":
		return srv.listSessions(params)
	case "session/load":
		return srv.loadSession(params)
	case "session/close":
		return srv.closeSession(params)
	case "session/resume":
		return srv.resumeSession(params)
	case "session/delete":
		return srv.deleteSession(params)
	}
	return nil, &RPCError{Code: codeMethodNotFound, Message: "method not found: " + method}
}

func (srv *Server) Notify(ctx context.Context, method string, params json.RawMessage) {
	if method != "session/cancel" {
		slog.Debug("acp: ignored notification", "method", method)
		return
	}
	n, err := decode[CancelNotification](params)
	if err != nil {
		return
	}
	if s, ok := srv.get(n.SessionID); ok {
		s.cancelPrompt()
	}
}

func (srv *Server) initialize(params json.RawMessage) (any, error) {
	if _, err := decode[InitializeRequest](params); err != nil {
		return nil, err
	}
	return InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			LoadSession:         true,
			PromptCapabilities:  PromptCapabilities{EmbeddedContext: true},
			SessionCapabilities: SessionCapabilities{List: &struct{}{}, Delete: &struct{}{}, Resume: &struct{}{}, Close: &struct{}{}},
		},
		AuthMethods: []AuthMethod{},
		AgentInfo:   Implementation{Name: "moca", Title: "moca", Version: srv.opts.Version},
	}, nil
}

func (srv *Server) add(s *session) {
	srv.mu.Lock()
	srv.sessions[s.id] = s
	srv.mu.Unlock()
}

func (srv *Server) remove(id string) {
	srv.mu.Lock()
	delete(srv.sessions, id)
	srv.mu.Unlock()
}

func (srv *Server) get(id string) (*session, bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	s, ok := srv.sessions[id]
	return s, ok
}

func (srv *Server) lookup(id string) (*session, error) {
	if s, ok := srv.get(id); ok {
		return s, nil
	}
	return nil, notFound("unknown session %q", id)
}

// sessionConfig validates cwd and returns the config for a session there
// (the --model override, and the project file when the workdir is trusted)
// plus the trust decision.
func (srv *Server) sessionConfig(cwd string) (config.Config, bool, error) {
	cfg := srv.opts.Config
	if !filepath.IsAbs(cwd) {
		return cfg, false, invalidParams("cwd must be an absolute path, got %q", cwd)
	}
	if fi, err := os.Stat(cwd); err != nil || !fi.IsDir() {
		return cfg, false, invalidParams("cwd %s is not a directory", cwd)
	}
	if srv.opts.Model != "" {
		cfg.Model = srv.opts.Model
	}
	if cfg.Model == "" {
		return cfg, false, &RPCError{Code: codeAuthRequired, Message: fmt.Sprintf(
			"no model configured: run `moca login <provider>` in a terminal, then set \"model\" in %s (or pass --model to `moca acp`)", srv.opts.ConfigPath)}
	}
	trusted := true
	if srv.opts.Trust != nil {
		t, err := srv.opts.Trust(cwd)
		if err != nil {
			return cfg, false, &RPCError{Code: codeInternal, Message: err.Error()}
		}
		trusted = t
	}
	if trusted {
		p, err := config.LoadProject(cwd)
		if err != nil {
			return cfg, false, &RPCError{Code: codeInternal, Message: err.Error()}
		}
		cfg.Project = p
	}
	return cfg, trusted, nil
}

func (srv *Server) startOptions(cfg config.Config, s *session) agent.StartOptions {
	mode, plan := config.ResolveMode(srv.opts.Mode, cfg)
	return agent.StartOptions{Config: cfg, Workdir: s.cwd, Model: srv.opts.Model, Effort: srv.opts.Effort,
		Trusted: s.trusted, Mode: agent.PermMode(mode), Plan: plan, Ask: s.ask, Emit: s.emit,
		HTTP: srv.opts.HTTP, Slug: "acp"}
}

func (srv *Server) newSession(params json.RawMessage) (any, error) {
	req, err := decode[NewSessionRequest](params)
	if err != nil {
		return nil, err
	}
	cfg, trusted, err := srv.sessionConfig(req.Cwd)
	if err != nil {
		return nil, err
	}
	if len(req.MCPServers) > 0 {
		slog.Debug("acp: client MCP servers ignored", "count", len(req.MCPServers))
	}
	s := newSession(srv, req.Cwd, trusted)
	a, err := agent.Start(srv.startOptions(cfg, s))
	if err != nil {
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	srv.add(s)
	slog.Info("acp session", "session", s.id, "trusted", s.trusted)
	s.reloadCommands()
	return Deferred{Result: NewSessionResponse{SessionID: s.id, Modes: s.modeState(), ConfigOptions: s.configOptions()}, Then: func() {
		s.markReady()
		s.sendCommands()
	}}, nil
}

func (srv *Server) prompt(ctx context.Context, params json.RawMessage) (any, error) {
	req, err := decode[PromptRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	runCtx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.end()
	text, err := flatten(s.cwd, req.Prompt)
	if err != nil {
		return nil, err
	}
	out, err := s.agent.Run(runCtx, s.expand(text))
	if s.reloadCommands() {
		s.sendCommands()
	}
	return s.stopReason(runCtx, out, err)
}
