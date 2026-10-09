// Package session owns the append-only JSONL transcript (§8) and the
// pre-edit snapshot store (§4).
package session

import (
	"time"

	"github.com/adeotek/moca/internal/llm"
)

const (
	TypeSession        = "session"
	TypeMessage        = "message"
	TypeToolUse        = "tool_use"
	TypeToolResult     = "tool_result"
	TypeCompaction     = "compaction"
	TypeModelChange    = "model_change"
	TypeSnapshot       = "snapshot"
	TypeError          = "error"
	TypePermissionMode = "permission_mode"
)

const (
	AbortedByUser = "aborted by user"
	Interrupted   = "interrupted — moca exited before completion"
)

type Header struct {
	Workdir      string    `json:"workdir"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Effort       string    `json:"effort"`
	StartedAt    time.Time `json:"startedAt"`
	MocaVersion  string    `json:"mocaVersion"`
	SystemPrompt string    `json:"systemPrompt"`
	Yolo         bool      `json:"yolo,omitempty"`
}

type PermissionMode struct {
	Yolo bool `json:"yolo"`
	Plan bool `json:"plan,omitempty"`
}

type ToolUse struct {
	MessageID string       `json:"messageId"`
	Call      llm.ToolCall `json:"call"`
}

type Compaction struct {
	Summary          string    `json:"summary"`
	FirstKeptEntryID string    `json:"firstKeptEntryId"`
	TokensBefore     int       `json:"tokensBefore"`
	Usage            llm.Usage `json:"usage"`
	ReadFiles        []string  `json:"readFiles,omitempty"`
	ModifiedFiles    []string  `json:"modifiedFiles,omitempty"`
}

type ModelChange struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type SnapshotRec struct {
	Path  string `json:"path"`
	Blob  string `json:"blob,omitempty"`  // sha256 of pre-edit content; "" = did not exist
	After string `json:"after,omitempty"` // sha256 of content moca wrote
	Git   bool   `json:"git,omitempty"`   // skipped: tracked + clean in git
	Mode  uint32 `json:"mode,omitempty"`  // pre-edit permission bits, restored by undo
}

type ErrorInfo struct {
	Message string `json:"message"`
}

type Entry struct {
	ID             string          `json:"id"`
	ParentID       string          `json:"parentId,omitempty"`
	Type           string          `json:"type"`
	Time           time.Time       `json:"ts"`
	Session        *Header         `json:"session,omitempty"`
	Message        *llm.Message    `json:"message,omitempty"`
	ToolUse        *ToolUse        `json:"toolUse,omitempty"`
	ToolResult     *llm.ToolResult `json:"toolResult,omitempty"`
	Usage          *llm.Usage      `json:"usage,omitempty"`
	Model          string          `json:"model,omitempty"` // producing model on assistant messages
	Cost           float64         `json:"cost,omitempty"`
	Compaction     *Compaction     `json:"compaction,omitempty"`
	ModelChange    *ModelChange    `json:"modelChange,omitempty"`
	Snapshot       *SnapshotRec    `json:"snapshot,omitempty"`
	Error          *ErrorInfo      `json:"error,omitempty"`
	PermissionMode *PermissionMode `json:"permissionMode,omitempty"`
}
