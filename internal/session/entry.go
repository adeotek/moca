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
	TypeElision        = "elision"
	// TypeGoal is a full goal-state snapshot (§8/§14, v0.8.0): every
	// transition appends one, so the latest goal entry is the live state and
	// the history stays append-only. It is pre-conversation metadata: a goal
	// does not materialize a deferred session by itself.
	TypeGoal = "goal"
	// TypeGoalReview is one evidence-only reviewer call: its Usage/Cost make
	// the reviewer's spend durable through the same accounting as every
	// request, and its Model names what produced it. It never carries
	// reviewer text into the conversation.
	TypeGoalReview = "goal_review"
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
	Mode         string    `json:"mode,omitempty"` // recorded permission mode ("default" kept implicit for old sessions)
	// SelfManagedSkills records that the session's system prompt carries the
	// self-managed skills block. Resume reads it back instead of probing the
	// stored prompt, whose text embeds an absolute path (a moved repo or a
	// changed symlink layout would make the probe lie) — M7.
	SelfManagedSkills bool `json:"selfManagedSkills,omitempty"`
}

type PermissionMode struct {
	Yolo bool   `json:"yolo"`
	Plan bool   `json:"plan,omitempty"`
	Mode string `json:"mode,omitempty"` // empty = the legacy yolo flag only
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

// Elision marks superseded tool results (§6): request rebuilds replace
// their content with ElidedStub; the transcript keeps the originals.
type Elision struct {
	IDs    []string `json:"ids"`    // tool_result entry ids
	Tokens int      `json:"tokens"` // estimated tokens saved
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

// GoalStatus is one of four terminal states for a session's single goal: it
// is active (work + review rounds run), paused (a human must resume), acked
// complete by the evidence reviewer, or cleared by the user.
type GoalStatus string

const (
	GoalActive   GoalStatus = "active"
	GoalPaused   GoalStatus = "paused"
	GoalComplete GoalStatus = "complete"
	GoalCleared  GoalStatus = "cleared"
)

// Goal is one full goal-state snapshot (v0.8.0). Objective ≤ 2,000 runes is
// enforced by the agent, not here; Rounds/Stalls are the automatic
// continuation and consecutive no-tool budgets; Reason is a fixed host string
// (never reviewer or model text) explaining a pause.
type Goal struct {
	ID        string     `json:"id"`
	Objective string     `json:"objective"`
	Status    GoalStatus `json:"status"`
	Rounds    int        `json:"rounds,omitempty"`
	Stalls    int        `json:"stalls,omitempty"`
	Reason    string     `json:"reason,omitempty"`
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
	Elision        *Elision        `json:"elision,omitempty"`
	Goal           *Goal           `json:"goal,omitempty"`
	// GoalID ties a user message to a goal (v0.8.0): metadata only, never
	// wire content. The goal envelopes a request renders for the entry are
	// selected from GoalID + Nudge; the objective itself comes from the
	// transcript's goal entries, so clearing or replacing a goal never
	// rewrites an earlier request.
	GoalID string `json:"goalId,omitempty"`
	// Nudge names the bounded nudge that produced this user message (§14,
	// metadata only — never sent with the request): empty, action, plan,
	// plan_reserve, verify, progress, execute.
	Nudge string `json:"nudge,omitempty"`
	// Plan marks a user message entry appended while plan mode was on: the
	// request rendering appends that entry's plan envelope right after its own
	// blocks, so toggling plan mode never rewrites an earlier message (§14).
	Plan bool `json:"plan,omitempty"`
	// ThinkingDropped counts the thinking blocks the provider reported
	// dropping for this turn (anthropic input_transformations,
	// prefix_binding_mismatch) — a preserved-thinking route's signal that the
	// history it was sent no longer matches the prefix it was produced
	// against. Plumbed now, parsed by the anthropic adapter later (§14).
	ThinkingDropped int `json:"thinkingDropped,omitempty"`
}
