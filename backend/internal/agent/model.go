// Package agent implements conversations and runs of the built-in agent.
package agent

import (
	"encoding/json"
	"time"
)

// Role identifies the author of an agent message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// HistoryRole identifies one model-visible transcript message.
type HistoryRole string

const (
	HistoryRoleUser       HistoryRole = "user"
	HistoryRoleAssistant  HistoryRole = "assistant"
	HistoryRoleToolResult HistoryRole = "toolResult"
)

// HistoryContent is one model-visible text or tool-call block.
type HistoryContent struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments"`
}

// HistoryMessage is a normalized model transcript message persisted between runs.
type HistoryMessage struct {
	Role       HistoryRole      `json:"role"`
	Content    []HistoryContent `json:"content"`
	ToolCallID string           `json:"toolCallId,omitempty"`
	ToolName   string           `json:"toolName,omitempty"`
	Details    any              `json:"details,omitempty"`
	IsError    bool             `json:"isError,omitempty"`
	Timestamp  int64            `json:"timestamp"`
}

// RunStatus identifies the lifecycle state of an agent run.
type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusAborted   RunStatus = "aborted"
)

// Session is one user-owned conversation with the built-in agent.
type Session struct {
	ID        string    `db:"id" json:"id"`
	UserID    string    `db:"user_id" json:"-"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// Message is canonical conversation history for the product UI.
type Message struct {
	ID        string    `db:"id" json:"id"`
	SessionID string    `db:"session_id" json:"sessionId"`
	RunID     *string   `db:"run_id" json:"runId"`
	Role      Role      `db:"role" json:"role"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

// Run is one execution of the agent runtime for a user message.
type Run struct {
	ID            string     `db:"id" json:"id"`
	SessionID     string     `db:"session_id" json:"sessionId"`
	UserID        string     `db:"user_id" json:"-"`
	Status        RunStatus  `db:"status" json:"status"`
	CorrelationID string     `db:"correlation_id" json:"correlationId"`
	StartedAt     *time.Time `db:"started_at" json:"startedAt"`
	CompletedAt   *time.Time `db:"completed_at" json:"completedAt"`
	Error         *string    `db:"error" json:"error"`
	CreatedAt     time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updatedAt"`
}

// RunEvent is a persisted, runtime-independent event exposed to clients.
type RunEvent struct {
	StreamOffset int64           `db:"stream_offset" json:"streamOffset"`
	RunID        string          `db:"run_id" json:"runId"`
	SessionID    string          `db:"session_id" json:"sessionId"`
	Sequence     int64           `db:"runtime_sequence" json:"sequence"`
	Type         string          `db:"type" json:"type"`
	OccurredAt   time.Time       `db:"occurred_at" json:"occurredAt"`
	Payload      json.RawMessage `db:"payload" json:"payload"`
}
