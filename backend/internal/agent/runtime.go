package agent

import "context"

const (
	EventRunStarted     = "agent.run.started"
	EventMessageDelta   = "agent.message.delta"
	EventToolStarted    = "agent.tool.started"
	EventToolCompleted  = "agent.tool.completed"
	EventHistoryMessage = "agent.history.message"
	EventTaskChanged    = "agent.task.changed"
	EventRunCompleted   = "agent.run.completed"
	EventRunFailed      = "agent.run.failed"
	EventRunAborted     = "agent.run.aborted"
)

// RunRequest contains the product identities and prompt supplied to a runtime.
type RunRequest struct {
	UserID         string
	SessionID      string
	RunID          string
	Message        string
	History        []HistoryMessage
	Runtime        string
	InternalURL    string
	AccessToken    string
	AllowedTools   []string
	AgentDir       string
	Provider       string
	Model          string
	Timezone       string
	ThinkingEffort string
}

// RuntimeEvent is one stable product event emitted by an agent runtime.
type RuntimeEvent struct {
	Type     string
	Sequence int64
	Payload  any
}

// Runtime executes one agent run and emits stable product events in order.
type Runtime interface {
	Run(context.Context, RunRequest, func(context.Context, RuntimeEvent) error) error
}
