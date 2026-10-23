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
	"strings"
	"sync"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
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
	HTTP    *http.Client // nil = the provider default (cmd/moca); tests inject one

	testEmit func(*session) // tests only: runs on a new session once it is ready
}

// Server maps ACP sessions onto moca agents.
type Server struct {
	opts     Options
	conn     *Conn
	mu       sync.Mutex
	sessions map[string]*session
	// the client's advertised session extensions (guarded by mu)
	notices, compaction bool
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
		return srv.authenticate(params)
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
	req, err := decode[InitializeRequest](params)
	if err != nil {
		return nil, err
	}
	if sc := req.ClientCapabilities.Session; sc != nil {
		srv.mu.Lock()
		srv.notices, srv.compaction = sc.Notices != nil, sc.Compaction != nil
		srv.mu.Unlock()
	}
	return InitializeResponse{
		ProtocolVersion: ProtocolVersion,
		AgentCapabilities: AgentCapabilities{
			LoadSession:         true,
			PromptCapabilities:  PromptCapabilities{EmbeddedContext: true},
			SessionCapabilities: SessionCapabilities{List: &struct{}{}, Delete: &struct{}{}, Resume: &struct{}{}, Close: &struct{}{}},
		},
		AuthMethods: []AuthMethod{authMethodLogin},
		AgentInfo:   Implementation{Name: "moca", Title: "moca", Version: srv.opts.Version},
	}, nil
}

// authMethodLogin is the one method initialize advertises: ACP's default
// `agent` kind (no `type` field), whose id is what `authenticate` receives.
//
// Not the `terminal` kind, deliberately. ACP says an agent may advertise a
// terminal method only when the client enabled its terminal authentication
// capability, and the client then re-runs the configured agent invocation
// interactively — for `moca acp` that is a second ACP server on a terminal,
// not a login (and `moca login` needs a provider name on the command line, so
// it cannot be expressed as appended args). The `agent` kind is what moca
// actually does: it resolves the credential itself and reports the result.
var authMethodLogin = AuthMethod{
	ID:          "login",
	Name:        "Sign in",
	Description: "Run `moca login <provider>` in a terminal (API key, or the OpenAI subscription flow), then set \"model\" in the config or pass --model to `moca acp`.",
}

// authenticate answers the method initialize advertises. There is nothing to
// hand over — moca's credentials live in ~/.config/moca/auth.json, outside the
// editor — so the call re-checks the configured model's credential and reports
// success, or the same error `session/new` would have given, which names the
// fix. It is never a bare no-op success: an editor that offers "Sign in" must
// not be told it worked while no credential resolves.
func (srv *Server) authenticate(params json.RawMessage) (any, error) {
	req, err := decode[AuthenticateRequest](params)
	if err != nil {
		return nil, err
	}
	if req.MethodID != authMethodLogin.ID {
		return nil, invalidParams("unknown authentication method %q: moca advertises %q", req.MethodID, authMethodLogin.ID)
	}
	if err := srv.checkModel(srv.opts.Config); err != nil {
		return nil, err
	}
	return AuthenticateResponse{}, nil
}

// clientSupports reports the client's notices and compaction extensions:
// ACP forbids sending those updates to a client that did not advertise them.
func (srv *Server) clientSupports() (notices, compaction bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.notices, srv.compaction
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
		// A request that names this id proves the client already has it, so
		// the updates session/new held back (§markReady) may flow now: flush
		// them before this request's own notifications, or they would queue
		// behind its response and arrive out of order.
		s.markReady()
		return s, nil
	}
	return nil, notFound("unknown session %q", id)
}

// sessionConfig validates cwd and returns the config for a session there
// (the --model override, and the project file when the workdir is trusted)
// plus the trust decision. requireModel gates the "no model configured"
// error: a new session needs one, while a stored session (load/resume) uses
// the model recorded in its header — a config without "model" must not block
// it (the same rule as the CLI, where the check gates -p, not --resume).
func (srv *Server) sessionConfig(cwd string, requireModel bool) (config.Config, bool, error) {
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
	if requireModel && cfg.Model == "" {
		// -32603, not ACP's auth-required: the fix is outside the editor
		// (run `moca login <provider>` in a terminal, set "model"), and a
		// -32000 hides that text behind "Authentication required". The
		// advertised `login` method makes the affordance discoverable, but
		// calling it cannot configure a model either — so the error still
		// carries the fix as itself (§9.5).
		return cfg, false, &RPCError{Code: codeInternal, Message: fmt.Sprintf(
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

// checkModel rejects a session whose configured model is unknown or whose
// provider has no usable credential, before anything starts. Both are -32603
// with moca's own fix in the message (run `moca login`, name the model): the
// credential lives outside the editor, so ACP's auth-required would replace
// the fix with a prompt that cannot succeed (§9.5). `authenticate` runs the
// same check, so the editor's sign-in affordance reports the same answer.
func (srv *Server) checkModel(cfg config.Config) error {
	// F-1 (ACP side): the --model flag is applied, and a local provider it
	// names is added, before anything validates the model.
	if srv.opts.Model != "" {
		cfg.Model = srv.opts.Model
	}
	if pn, _, err := config.SplitModel(cfg.Model); err == nil {
		cfg.UseProvider(pn)
	}
	reg, err := provider.NewRegistry(cfg, srv.opts.HTTP, nil)
	if err != nil {
		return &RPCError{Code: codeInternal, Message: err.Error()}
	}
	// F-2: a configured ollama is discovered first. Without this, a local
	// model that is running right now was reported as "unknown model" and the
	// editor showed an authentication error for a server that needs no key.
	if _, configured := cfg.Providers["ollama"]; configured || strings.HasPrefix(cfg.Model, "ollama/") {
		if _, err := reg.DiscoverOllama(context.Background()); err != nil {
			slog.Debug("acp: ollama discovery failed", "err", err)
		}
	}
	if err := reg.CheckCredential(cfg.Model); err != nil {
		return &RPCError{Code: codeInternal, Message: err.Error()}
	}
	return nil
}

func (srv *Server) startOptions(cfg config.Config, s *session) agent.StartOptions {
	mode, plan := config.ResolveMode(srv.opts.Mode, cfg)
	slog.Info("mode", "mode", mode, "plan", plan) // final: after the project layer (§13.5)
	return agent.StartOptions{Config: cfg, Workdir: s.cwd, Model: srv.opts.Model, Effort: srv.opts.Effort,
		Trusted: s.trusted, Mode: agent.PermMode(mode), Plan: plan, Ask: s.ask, Emit: s.emit,
		HTTP: srv.opts.HTTP, Slug: "acp", ConfigPath: srv.opts.ConfigPath}
}

func (srv *Server) newSession(params json.RawMessage) (any, error) {
	req, err := decode[NewSessionRequest](params)
	if err != nil {
		return nil, err
	}
	cfg, trusted, err := srv.sessionConfig(req.Cwd, true)
	if err != nil {
		return nil, err
	}
	if len(req.MCPServers) > 0 {
		slog.Debug("acp: client MCP servers ignored", "count", len(req.MCPServers))
	}
	if err := srv.checkModel(cfg); err != nil {
		return nil, err
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
		if srv.opts.testEmit != nil {
			srv.opts.testEmit(s)
		}
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
	// The typed text is the prompt's `text` blocks only: flatten inlines
	// `resource` blocks as fenced text, so the joined string cannot serve
	// (Decision 15).
	typedSrc := textBlocks(req.Prompt)
	expanded, rem := s.expand(text)
	var out agent.Outcome
	if rem {
		// "/remember <args>" — the rule is the user's own text blocks, split the
		// way expand split it; an attachment never supplies it, so text blocks
		// holding only "/remember" leave nothing to record (Agent.Remember refuses).
		_, args := splitCommand(strings.TrimSpace(typedSrc))
		expanded = agent.RememberRequest(args)
		out, err = s.agent.Remember(runCtx, args) // the run STARTS here; the signal is structural (C4)
	} else {
		typed := strings.TrimSpace(typedSrc)
		if strings.HasPrefix(typed, "/") && !strings.HasPrefix(typed, "//") && expanded != text {
			_, typed = splitCommand(typed) // a template/skill expansion: only its args were typed (a `//` escape is plain text)
		}
		out, err = s.agent.RunWith(runCtx, expanded, agent.RunOpts{Typed: typed})
	}
	if s.reloadCommands() {
		s.sendCommands()
	}
	s.sendInfo(expanded)
	s.reportConfig()
	return s.stopReason(runCtx, out, err)
}
