package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	msession "github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

// session is one ACP session: a moca agent plus the state that maps its
// events onto session/update notifications.
type session struct {
	srv      *Server
	cwd      string
	trusted  bool
	prompts  []skills.Prompt    // prompt templates (reloaded after each prompt)
	commands []AvailableCommand // the advertised list
	agent    *agent.Agent
	id       string

	// runMu guards cancel; begin/end and the idle-only setters hold it, so
	// set_mode / set_config_option never overlap a running prompt.
	runMu  sync.Mutex
	cancel context.CancelFunc // non-nil while a prompt runs
	closed bool               // session/close or delete: no new prompt may begin
	runWG  sync.WaitGroup     // the running prompt, awaited by shutdown

	// sendMu orders writes and guards ready/early: updates emitted before
	// the client has the session id (agent.Start warnings) are held, then
	// flushed by markReady after the response is written.
	sendMu sync.Mutex
	ready  bool
	early  []any

	// cmdMu guards the command state (prompts, commands and the agent's skill
	// list): it is reloaded between runs, and a client may send its next
	// request the moment it has a response — the response's follow-up
	// (sendCommands) and a prompt handler's reload run concurrently then.
	cmdMu sync.Mutex

	// mu guards the event-mapping state, written from the agent's run goroutine.
	mu         sync.Mutex
	msgIDs     map[string]string // update kind → messageId of the current message
	toolSeq    int
	toolIDs    map[string]string // provider call id → ACP toolCallId (open calls)
	curTool    string
	lastStop   llm.StopReason
	lastMode   string // the mode id the client was last told
	modeMuted  bool   // set_mode is applying setters: report once at the end
	titled     bool   // session_info_update has carried (or the store already has) a title
	lastConfig string // JSON of the config options the client last saw
}

func newSession(srv *Server, cwd string, trusted bool) *session {
	return &session{srv: srv, cwd: cwd, trusted: trusted, msgIDs: map[string]string{}, toolIDs: map[string]string{}}
}

// attach binds the started agent; its session id8 is the ACP sessionId.
func (s *session) attach(a *agent.Agent) {
	s.agent = a
	s.id = a.Session().ID8()
}

func (s *session) markReady() {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.ready = true
	for _, u := range s.early {
		s.notify(u)
	}
	s.early = nil
}

func (s *session) send(u any) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if !s.ready {
		s.early = append(s.early, u)
		return
	}
	s.notify(u)
}

func (s *session) notify(u any) {
	if err := s.srv.conn.Notify("session/update", SessionNotification{SessionID: s.id, Update: u}); err != nil {
		slog.Debug("acp: update not sent", "session", s.id, "error", err)
	}
}

// chunk builds a text chunk of kind, keeping one messageId per message.
func (s *session) chunk(kind, text string) ContentChunk {
	s.mu.Lock()
	id := s.msgIDs[kind]
	if id == "" {
		id = newMessageID()
		s.msgIDs[kind] = id
	}
	s.mu.Unlock()
	return ContentChunk{SessionUpdate: kind, Content: ContentBlock{Type: "text", Text: text}, MessageID: id}
}

// nextMessage starts new messages: after a turn, or when a retried stream
// restarts (chunks already sent cannot be retracted).
func (s *session) nextMessage() {
	s.mu.Lock()
	clear(s.msgIDs)
	s.mu.Unlock()
}

// openTool assigns the ACP id for a provider call id. Provider ids repeat
// across turns (openai-compatible servers number calls call_0, call_1, …
// per response); an ACP toolCallId must be unique in the session.
func (s *session) openTool(callID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolSeq++
	id := fmt.Sprintf("%d-%s", s.toolSeq, callID)
	s.toolIDs[callID] = id
	s.curTool = id
	return id
}

// closeTool returns the ACP id of an open call; known=false for a result
// without a start (a cut-off turn's synthetic result), which gets a fresh id.
func (s *session) closeTool(callID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.toolIDs[callID]
	if !ok {
		s.toolSeq++
		id = fmt.Sprintf("%d-%s", s.toolSeq, callID)
	}
	delete(s.toolIDs, callID)
	if s.curTool == id {
		s.curTool = ""
	}
	return id, ok
}

func (s *session) currentTool() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.curTool
}

// emit is the agent's StartOptions.Emit: agent events → session/update.
func (s *session) emit(e agent.Event) {
	switch e := e.(type) {
	case agent.TextDelta:
		if e.Text != "" {
			s.send(s.chunk(updAgentMessage, e.Text))
		}
	case agent.ThinkingDelta:
		if e.Text != "" {
			s.send(s.chunk(updThought, e.Text))
		}
	case agent.StreamReset:
		s.nextMessage()
	case agent.TurnEnd:
		s.mu.Lock()
		s.lastStop = e.Stop
		s.mu.Unlock()
		s.nextMessage()
		// TurnEnd is emitted outside a.mu (agent.go turn()), so Status is safe here.
		if s.agent != nil {
			st := s.agent.Status()
			s.send(UsageUpdate{SessionUpdate: updUsage, Used: st.ContextTokens, Size: st.Window,
				Cost: &Cost{Amount: st.Cost, Currency: "USD"}})
		}
	case agent.ToolStart:
		s.send(startUpdate(s.openTool(e.Call.ID), s.cwd, e.Call))
	case agent.ToolEnd:
		id, known := s.closeTool(e.Call.ID)
		if !known {
			s.send(startUpdate(id, s.cwd, e.Call))
		}
		s.send(endUpdate(id, s.cwd, e.Call, e.Result))
	case agent.ModeChanged, agent.PlanChanged:
		// nil while agent.Start applies the initial mode: the response carries
		// it. Muted while session/set_mode runs its setters (it reports once).
		if s.agent != nil {
			s.reportMode()
		}
	case agent.Warning:
		if notices, _ := s.srv.clientSupports(); notices {
			s.send(Notice{SessionUpdate: updNotice, Severity: "warning", Title: e.Text})
		} else {
			s.send(s.chunk(updThought, "moca: "+e.Text+"\n"))
		}
	case agent.Retry:
		if notices, _ := s.srv.clientSupports(); notices {
			s.send(Notice{SessionUpdate: updNotice, Severity: "info", Title: "Retrying the model request",
				Description: fmt.Sprintf("attempt %d of %d, waiting %s", e.Notice.Attempt, e.Notice.Max, e.Notice.Wait.Round(100*time.Millisecond))})
		}
	case agent.Compacted:
		// Emitted outside a.mu (compaction.go): History is safe here.
		if s.agent != nil {
			s.compacted(summaryText(s.agent.History()), e.TokensBefore, e.TokensAfter)
		}
	}
}

// compacted reports a finished compaction. With the client's compaction
// extension: one summary chunk (moca's summarizer returns the summary whole;
// the chunk opens the compaction) and a completed update. Without it: a
// thought line (before/after <= 0: replay, sizes unknown — see replay).
func (s *session) compacted(summary string, before, after int) {
	if _, ok := s.srv.clientSupports(); ok {
		id := newMessageID()
		if summary != "" {
			s.send(CompactionSummaryChunk{SessionUpdate: updCompactionChunk, CompactionID: id,
				Content: ContentBlock{Type: "text", Text: summary}})
		}
		s.send(CompactionUpdate{SessionUpdate: updCompaction, CompactionID: id, Status: "completed"})
		return
	}
	s.send(s.chunk(updThought, fmt.Sprintf("moca: context compacted (%d → %d tokens)\n", before, after)))
}

// summaryOf strips the compaction summary message's prefix.
func summaryOf(text string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), msession.SummaryPrefix))
}

// summaryText is the current compaction summary in msgs (History() puts it
// first), or "".
func summaryText(msgs []llm.Message) string {
	for _, m := range msgs {
		if m.Role != llm.RoleUser {
			continue
		}
		for _, b := range m.Content {
			if b.Type == llm.BlockText && strings.HasPrefix(strings.TrimSpace(b.Text), msession.SummaryPrefix) {
				return summaryOf(b.Text)
			}
		}
	}
	return ""
}

// sendInfo updates the client's session metadata after a prompt: the title
// once (session/list's rule: the first line of the first user message), the
// update time always.
func (s *session) sendInfo(text string) {
	u := SessionInfoUpdate{SessionUpdate: updSessionInfo, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	s.mu.Lock()
	if !s.titled {
		s.titled = true
		line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
		if r := []rune(line); len(r) > 80 {
			line = string(r[:80])
		}
		u.Title = &line
	}
	s.mu.Unlock()
	s.send(u)
}

// ask is the agent's StartOptions.Ask: a session/request_permission
// round-trip tied to the running tool call. A cancelled outcome, an error
// or a closed connection denies.
func (s *session) ask(ctx context.Context, q tools.Question) tools.Answer {
	id := s.currentTool()
	if id == "" {
		id = s.openTool("ask")
	}
	opts := []PermissionOption{{OptionID: optAllowOnce, Name: "Allow once", Kind: "allow_once"}}
	if q.CanAlways {
		opts = append(opts, PermissionOption{OptionID: optAllowAlways, Name: "Always allow", Kind: "allow_always"})
	}
	opts = append(opts, PermissionOption{OptionID: optReject, Name: "Deny", Kind: "reject_once"})
	raw, _ := json.Marshal(map[string]string{"detail": q.Detail})
	req := RequestPermissionRequest{SessionID: s.id, Options: opts,
		ToolCall: ToolCall{ToolCallID: id, Title: q.Kind + ": " + q.Subject, RawInput: raw}}
	var resp RequestPermissionResponse
	if err := s.srv.conn.Call(ctx, "session/request_permission", req, &resp); err != nil {
		slog.Debug("acp: permission request failed", "session", s.id, "error", err)
		return tools.Deny
	}
	if resp.Outcome.Outcome != "selected" {
		return tools.Deny
	}
	switch resp.Outcome.OptionID {
	case optAllowOnce:
		return tools.AllowOnce
	case optAllowAlways:
		if q.CanAlways {
			return tools.AllowAlways
		}
	}
	return tools.Deny
}

// begin marks a prompt as running and returns its cancellable context.
func (s *session) begin(ctx context.Context) (context.Context, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closed {
		return nil, notFound("session %s is closed", s.id)
	}
	if s.cancel != nil {
		return nil, busy("a prompt is already running in this session")
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.runWG.Add(1)
	s.mu.Lock()
	s.lastStop = ""
	s.mu.Unlock()
	return ctx, nil
}

func (s *session) end() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.runWG.Done()
	}
}

func (s *session) cancelPrompt() {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

// shutdown ends the session (session/close, delete): running work is
// cancelled as by session/cancel and awaited, then the agent is closed —
// its MCP servers stop and the session file's writer lock is released.
// Idempotent.
func (s *session) shutdown() {
	s.runMu.Lock()
	if s.closed {
		s.runMu.Unlock()
		return
	}
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	s.runMu.Unlock()
	s.runWG.Wait()
	s.agent.Close()
}

// idle runs f only while no prompt runs: the agent's mode and model setters
// are between-runs only.
func (s *session) idle(f func() (any, error)) (any, error) {
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closed {
		return nil, notFound("session %s is closed", s.id)
	}
	if s.cancel != nil {
		return nil, busy("wait for the current prompt to finish")
	}
	return f()
}

// stopReason maps a finished run onto the prompt response (§2 table).
func (s *session) stopReason(ctx context.Context, out agent.Outcome, err error) (any, error) {
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return PromptResponse{StopReason: stopCancelled}, nil
	case err != nil:
		slog.Info("acp prompt failed", "session", s.id)
		return nil, &RPCError{Code: codeInternal, Message: err.Error()}
	case out.MaxSteps || out.Stuck:
		return PromptResponse{StopReason: stopMaxTurns}, nil
	}
	s.mu.Lock()
	last := s.lastStop
	s.mu.Unlock()
	switch last {
	case llm.StopLength:
		return PromptResponse{StopReason: stopMaxTokens}, nil
	case llm.StopRefusal:
		return PromptResponse{StopReason: stopRefusal}, nil
	}
	return PromptResponse{StopReason: stopEndTurn}, nil
}
