package agentauth

import (
	"time"

	"github.com/mishankov/todai/backend/internal/execution"
)

// Tool identifies one operation an agent token may authorize.
type Tool string

const (
	ToolTaskGet      Tool = "task_get"
	ToolTaskSearch   Tool = "task_search"
	ToolProjectGet   Tool = "project_get"
	ToolProjectList  Tool = "project_list"
	ToolViewQuery    Tool = "view_query"
	ToolTaskCreate   Tool = "task_create"
	ToolTaskUpdate   Tool = "task_update"
	ToolTaskComplete Tool = "task_complete"
	ToolTaskReopen   Tool = "task_reopen"
	ToolTaskMove     Tool = "task_move"
	ToolTaskReorder  Tool = "task_reorder"
)

// IssueRequest describes the identity, permissions, and lifetime of a new token.
type IssueRequest struct {
	UserID         string
	ProjectID      string
	AgentSessionID string
	AgentRunID     string
	AllowedTools   []Tool
	TTL            time.Duration
}

// IssuedToken contains the opaque token returned once to its caller.
type IssuedToken struct {
	Token     string
	ExpiresAt time.Time
}

// Claims are the trusted identity and permissions authenticated from a token.
type Claims struct {
	UserID         string
	ProjectID      string
	AgentSessionID string
	AgentRunID     string
	AllowedTools   []Tool
	ExpiresAt      time.Time
}

// ExecutionScope builds attribution for an operation performed by the built-in agent.
func (c Claims) ExecutionScope(correlationID string) execution.Scope {
	actorID := c.AgentSessionID
	agentRunID := c.AgentRunID
	return execution.Scope{
		UserID:        c.UserID,
		ProjectID:     &c.ProjectID,
		ActorType:     execution.ActorBuiltInAgent,
		ActorID:       &actorID,
		Source:        execution.SourceInternalAPI,
		CorrelationID: correlationID,
		AgentRunID:    &agentRunID,
	}
}
