package acp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	msession "github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

// id8 is the only sessionId form load accepts: msession.Find also takes
// "last", which must never reach it from a client.
var id8 = regexp.MustCompile(`^[0-9a-f]{8}$`)

func sessionsDir() string { return filepath.Join(config.DataDir(), "sessions") }

func (srv *Server) listSessions(params json.RawMessage) (any, error) {
	req, err := decode[ListSessionsRequest](params)
	if err != nil {
		return nil, err
	}
	if req.Cwd != "" && !filepath.IsAbs(req.Cwd) {
		return nil, invalidParams("cwd must be an absolute path, got %q", req.Cwd)
	}
	out := ListSessionsResponse{Sessions: []SessionInfo{}}
	for _, in := range msession.List(sessionsDir(), req.Cwd, "", 100) {
		out.Sessions = append(out.Sessions, SessionInfo{SessionID: in.ID8, Cwd: in.Workdir, Title: in.Preview,
			UpdatedAt: in.Modified.UTC().Format(time.RFC3339)})
	}
	return out, nil
}

func (srv *Server) loadSession(params json.RawMessage) (any, error) {
	req, err := decode[LoadSessionRequest](params)
	if err != nil {
		return nil, err
	}
	return srv.reopen(req.SessionID, req.Cwd, true)
}

// resumeSession is session/load without the history replay.
func (srv *Server) resumeSession(params json.RawMessage) (any, error) {
	req, err := decode[ResumeSessionRequest](params)
	if err != nil {
		return nil, err
	}
	r, err := srv.reopen(req.SessionID, req.Cwd, false)
	if err != nil {
		return nil, err
	}
	return ResumeSessionResponse{Modes: r.Modes, ConfigOptions: r.ConfigOptions}, nil
}

// reopen serves session/load (replay) and session/resume (no replay): an
// open session is reused, a stored one resumed in its own workdir only.
func (srv *Server) reopen(id, cwd string, replay bool) (LoadSessionResponse, error) {
	if !id8.MatchString(id) {
		return LoadSessionResponse{}, invalidParams("sessionId must be a moca session id (8 hex chars), got %q", id)
	}
	if s, ok := srv.get(id); ok {
		if filepath.Clean(s.cwd) != filepath.Clean(cwd) {
			return LoadSessionResponse{}, invalidParams("session %s belongs to another directory (%s)", id, s.cwd)
		}
		r, err := s.idle(func() (any, error) {
			if replay {
				s.replay()
			}
			s.sendCommands()
			return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
		})
		if err != nil {
			return LoadSessionResponse{}, err
		}
		return r.(LoadSessionResponse), nil
	}
	cfg, trusted, err := srv.sessionConfig(cwd, false)
	if err != nil {
		return LoadSessionResponse{}, err
	}
	path, err := msession.Find(sessionsDir(), id, cwd)
	if err != nil {
		return LoadSessionResponse{}, notFound("%v", err)
	}
	if !msession.InWorkdir(path, cwd) {
		return LoadSessionResponse{}, invalidParams("session %s belongs to another directory (%s)", id, msession.WorkdirOf(path))
	}
	s := newSession(srv, cwd, trusted)
	s.titled = true // a stored session already has its title
	a, err := agent.Resume(srv.startOptions(cfg, s), path)
	if err != nil {
		return LoadSessionResponse{}, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	s.reloadCommands()
	srv.add(s)
	s.markReady() // the client named this id: updates may flow now
	if replay {
		s.replay()
	}
	s.sendCommands()
	return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
}

func (srv *Server) closeSession(params json.RawMessage) (any, error) {
	req, err := decode[CloseSessionRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	srv.remove(s.id)
	s.shutdown()
	slog.Info("acp session closed", "session", s.id)
	return struct{}{}, nil
}

// deleteSession removes a stored session (one session/list shows). A
// session open here is closed first — this process's writer holds the file
// lock — unless a prompt runs; one held by another moca process is refused.
func (srv *Server) deleteSession(params json.RawMessage) (any, error) {
	req, err := decode[DeleteSessionRequest](params)
	if err != nil {
		return nil, err
	}
	if !id8.MatchString(req.SessionID) {
		// Not a moca id (and "last" must never reach Find): nothing to delete.
		slog.Debug("acp session delete: not a moca session id")
		return struct{}{}, nil
	}
	if s, ok := srv.get(req.SessionID); ok {
		// Close in the same runMu section that saw it idle: a prompt arriving
		// in between would otherwise begin and then be cancelled by shutdown.
		if _, err := s.idle(func() (any, error) { s.closed = true; return nil, nil }); err != nil {
			return nil, err // busy: a prompt is running
		}
		srv.remove(s.id)
		s.agent.Close()
	}
	path, err := msession.Find(sessionsDir(), req.SessionID, "")
	if err != nil {
		// The id is well-formed, so Find failed on "no such session":
		// deleting an absent session succeeds silently (spec, SHOULD).
		slog.Debug("acp session delete: already absent", "session", req.SessionID)
		return struct{}{}, nil
	}
	if err := msession.Delete(path); err != nil {
		if errors.Is(err, msession.ErrInUse) {
			return nil, busy("session " + req.SessionID + " is open in another moca process: close it there first")
		}
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	slog.Info("acp session deleted", "session", req.SessionID)
	return struct{}{}, nil
}

// replay streams the whole conversation as the model sees it (after any
// compaction summary): user and agent text, thinking, and each tool call
// with its recorded result. ACP clients rebuild their view from it.
func (s *session) replay() { s.replayMessages(s.agent.History()) }

// replayMessages streams msgs (see replay): user and agent text, thinking,
// tool calls with their results; the compaction summary as a compaction
// (client extension) or as one summarized-note chunk.
func (s *session) replayMessages(msgs []llm.Message) {
	results := map[string]llm.ToolResult{}
	for _, m := range msgs {
		for _, b := range m.Content {
			if b.Type == llm.BlockToolResult && b.ToolResult != nil {
				results[b.ToolResult.CallID] = *b.ToolResult
			}
		}
	}
	for _, m := range msgs {
		for _, b := range m.Content {
			switch {
			case b.Type == llm.BlockText && m.Role == llm.RoleUser:
				if strings.HasPrefix(strings.TrimSpace(b.Text), msession.SummaryPrefix) {
					if _, ok := s.srv.clientSupports(); ok {
						s.compacted(summaryOf(b.Text), 0, 0)
					} else {
						s.send(s.chunk(updAgentMessage, "_(earlier conversation was summarized)_\n\n"))
					}
				} else {
					s.send(s.chunk(updUserMessage, b.Text))
				}
			case b.Type == llm.BlockText:
				s.send(s.chunk(updAgentMessage, b.Text))
			case b.Type == llm.BlockThinking && b.Text != "":
				s.send(s.chunk(updThought, b.Text))
			case b.Type == llm.BlockToolUse && b.ToolCall != nil:
				id := s.openTool(b.ToolCall.ID)
				s.send(startUpdate(id, s.cwd, *b.ToolCall))
				res := tools.Result{Content: "no result recorded", IsError: true}
				if r, ok := results[b.ToolCall.ID]; ok {
					res = tools.Result{Content: r.Content, IsError: r.IsError}
				}
				s.closeTool(b.ToolCall.ID)
				s.send(endUpdate(id, s.cwd, *b.ToolCall, res))
			}
		}
		s.nextMessage()
	}
}
