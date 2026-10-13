package acp

import (
	"encoding/json"
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
	if !id8.MatchString(req.SessionID) {
		return nil, invalidParams("sessionId must be a moca session id (8 hex chars), got %q", req.SessionID)
	}
	if s, ok := srv.get(req.SessionID); ok {
		return s.idle(func() (any, error) {
			s.replay()
			s.sendCommands()
			return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
		})
	}
	cfg, trusted, err := srv.sessionConfig(req.Cwd)
	if err != nil {
		return nil, err
	}
	path, err := msession.Find(sessionsDir(), req.SessionID, req.Cwd)
	if err != nil {
		return nil, invalidParams("%v", err)
	}
	if !msession.InWorkdir(path, req.Cwd) {
		return nil, invalidParams("session %s belongs to another directory (%s)", req.SessionID, msession.WorkdirOf(path))
	}
	s := newSession(srv, req.Cwd, trusted)
	a, err := agent.Resume(srv.startOptions(cfg, s), path)
	if err != nil {
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	}
	s.attach(a)
	s.reloadCommands()
	srv.add(s)
	s.markReady() // the client named this id: updates may flow now
	s.replay()
	s.sendCommands()
	return LoadSessionResponse{Modes: s.modeState(), ConfigOptions: s.configOptions()}, nil
}

// replay streams the whole conversation as the model sees it (after any
// compaction summary): user and agent text, thinking, and each tool call
// with its recorded result. ACP clients rebuild their view from it.
func (s *session) replay() {
	msgs := s.agent.History()
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
					s.send(s.chunk(updAgentMessage, "_(earlier conversation was summarized)_\n\n"))
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
