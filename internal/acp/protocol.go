package acp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

// ProtocolVersion is the ACP major version moca speaks.
const ProtocolVersion = 1

// acpMethods lists every method name moca serves or calls; a test checks
// each against the pinned schema's meta.json. Later tasks append to it as
// they add methods.
var acpMethods = []string{
	"initialize", "authenticate", "session/new", "session/prompt", "session/cancel",
	"session/update", "session/request_permission",
	"session/set_mode", "session/set_config_option", "session/list", "session/load",
	"$/cancel_request", "session/close", "session/resume", "session/delete",
}

// session/update discriminators.
const (
	updUserMessage     = "user_message_chunk"
	updAgentMessage    = "agent_message_chunk"
	updThought         = "agent_thought_chunk"
	updToolCall        = "tool_call"
	updToolUpdate      = "tool_call_update"
	updCurrentMode     = "current_mode_update"
	updCommands        = "available_commands_update"
	updUsage           = "usage_update"
	updNotice          = "notice"
	updCompaction      = "compaction_update"
	updCompactionChunk = "compaction_summary_chunk"
	updSessionInfo     = "session_info_update"
	updConfigOptions   = "config_option_update"
)

const (
	statusInProgress = "in_progress"
	statusCompleted  = "completed"
	statusFailed     = "failed"

	stopEndTurn   = "end_turn"
	stopMaxTokens = "max_tokens"
	stopMaxTurns  = "max_turn_requests"
	stopRefusal   = "refusal"
	stopCancelled = "cancelled"
)

// Permission option ids are moca's; their kinds are ACP's.
const (
	optAllowOnce   = "allow-once"
	optAllowAlways = "allow-always"
	optReject      = "reject"
)

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type InitializeRequest struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities"`
}

// ClientCapabilities: only the session extensions moca reacts to. A field
// present as {} advertises support; omitted or null does not.
type ClientCapabilities struct {
	Session *ClientSessionCapabilities `json:"session,omitempty"`
}

type ClientSessionCapabilities struct {
	Notices    *struct{} `json:"notices,omitempty"`
	Compaction *struct{} `json:"compaction,omitempty"`
}

// Notice is a live, fire-and-forget message for the user (not history).
type Notice struct {
	SessionUpdate string `json:"sessionUpdate"`
	Severity      string `json:"severity"` // info | warning | error
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
}

// CompactionSummaryChunk appends to a compaction's summary; a first-seen id
// opens an in-progress compaction.
type CompactionSummaryChunk struct {
	SessionUpdate string       `json:"sessionUpdate"`
	CompactionID  string       `json:"compactionId"`
	Content       ContentBlock `json:"content"`
}

// CompactionUpdate patches a compaction by id (here: marks it completed).
type CompactionUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	CompactionID  string `json:"compactionId"`
	Status        string `json:"status"`
}

type SessionInfoUpdate struct {
	SessionUpdate string  `json:"sessionUpdate"`
	Title         *string `json:"title,omitempty"`
	UpdatedAt     string  `json:"updatedAt,omitempty"`
}

type ConfigOptionUpdate struct {
	SessionUpdate string         `json:"sessionUpdate"`
	ConfigOptions []ConfigOption `json:"configOptions"`
}

type InitializeResponse struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []AuthMethod      `json:"authMethods"`
	AgentInfo         Implementation    `json:"agentInfo"`
}

type AgentCapabilities struct {
	LoadSession         bool                `json:"loadSession"`
	PromptCapabilities  PromptCapabilities  `json:"promptCapabilities"`
	MCPCapabilities     MCPCapabilities     `json:"mcpCapabilities"`
	SessionCapabilities SessionCapabilities `json:"sessionCapabilities"`
}

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type MCPCapabilities struct {
	HTTP bool `json:"http"`
	SSE  bool `json:"sse"`
}

// SessionCapabilities: each non-nil field advertises the method of that name.
type SessionCapabilities struct {
	List   *struct{} `json:"list,omitempty"`
	Delete *struct{} `json:"delete,omitempty"`
	Resume *struct{} `json:"resume,omitempty"`
	Close  *struct{} `json:"close,omitempty"`
}

type ResumeSessionRequest struct {
	SessionID  string            `json:"sessionId"`
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type ResumeSessionResponse struct {
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type CloseSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type DeleteSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type AuthMethod struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NewSessionRequest struct {
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type NewSessionResponse struct {
	SessionID     string            `json:"sessionId"`
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type LoadSessionRequest struct {
	SessionID  string            `json:"sessionId"`
	Cwd        string            `json:"cwd"`
	MCPServers []json.RawMessage `json:"mcpServers"`
}

type LoadSessionResponse struct {
	Modes         *SessionModeState `json:"modes,omitempty"`
	ConfigOptions []ConfigOption    `json:"configOptions,omitempty"`
}

type ListSessionsRequest struct {
	Cwd    string  `json:"cwd,omitempty"`
	Cursor *string `json:"cursor,omitempty"`
}

type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
	Title     string `json:"title,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type PromptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type PromptResponse struct {
	StopReason string `json:"stopReason"`
}

type CancelNotification struct {
	SessionID string `json:"sessionId"`
}

type SetModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type SetConfigOptionRequest struct {
	SessionID string          `json:"sessionId"`
	ConfigID  string          `json:"configId"`
	Value     json.RawMessage `json:"value"`
}

type SetConfigOptionResponse struct {
	ConfigOptions []ConfigOption `json:"configOptions"`
}

// ContentBlock covers the prompt blocks moca reads (text, resource_link,
// resource) and the text blocks it sends.
type ContentBlock struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Name     string            `json:"name,omitempty"`
	Resource *EmbeddedResource `json:"resource,omitempty"`
}

type EmbeddedResource struct {
	URI      string  `json:"uri"`
	MimeType string  `json:"mimeType,omitempty"`
	Text     *string `json:"text,omitempty"`
	Blob     *string `json:"blob,omitempty"`
}

// SessionNotification is the session/update params; Update is one of the
// update structs below (each carries its sessionUpdate discriminator).
type SessionNotification struct {
	SessionID string `json:"sessionId"`
	Update    any    `json:"update"`
}

type ContentChunk struct {
	SessionUpdate string       `json:"sessionUpdate"`
	Content       ContentBlock `json:"content"`
	MessageID     string       `json:"messageId,omitempty"`
}

// ToolCall is the tool_call update, a tool_call_update (unchanged fields
// omitted), and the toolCall of a permission request (no sessionUpdate).
type ToolCall struct {
	SessionUpdate string             `json:"sessionUpdate,omitempty"`
	ToolCallID    string             `json:"toolCallId"`
	Title         string             `json:"title,omitempty"`
	Kind          string             `json:"kind,omitempty"`
	Status        string             `json:"status,omitempty"`
	RawInput      json.RawMessage    `json:"rawInput,omitempty"`
	Locations     []ToolCallLocation `json:"locations,omitempty"`
	Content       []ToolCallContent  `json:"content,omitempty"`
}

type ToolCallLocation struct {
	Path string `json:"path"`
}

// ToolCallContent is {type:"content", content} or {type:"diff", path, oldText, newText}.
type ToolCallContent struct {
	Type    string        `json:"type"`
	Content *ContentBlock `json:"content,omitempty"`
	Path    string        `json:"path,omitempty"`
	OldText *string       `json:"oldText,omitempty"`
	NewText *string       `json:"newText,omitempty"`
}

type CurrentModeUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	CurrentModeID string `json:"currentModeId"`
}

// UsageUpdate reports context fill and spend after each model turn.
type UsageUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	Used          int    `json:"used"`
	Size          int    `json:"size"`
	Cost          *Cost  `json:"cost,omitempty"`
}

type Cost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type AvailableCommandsUpdate struct {
	SessionUpdate     string             `json:"sessionUpdate"`
	AvailableCommands []AvailableCommand `json:"availableCommands"`
}

type AvailableCommand struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       *AvailableCommandInput `json:"input,omitempty"`
}

type AvailableCommandInput struct {
	Hint string `json:"hint"`
}

type SessionModeState struct {
	CurrentModeID  string        `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

type SessionMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ConfigOption struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Category     string              `json:"category,omitempty"`
	Type         string              `json:"type"`
	CurrentValue string              `json:"currentValue"`
	Options      []ConfigOptionValue `json:"options"`
}

type ConfigOptionValue struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

type RequestPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCall           `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type RequestPermissionResponse struct {
	Outcome PermissionOutcome `json:"outcome"`
}

// PermissionOutcome: {outcome:"selected", optionId} or {outcome:"cancelled"}.
type PermissionOutcome struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId,omitempty"`
}

func strp(s string) *string { return &s }

// newMessageID returns a random UUID (v4 layout): a fresh id per streamed
// message lets a client start a new bubble after a stream restart.
func newMessageID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
