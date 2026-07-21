// Package activity implements the append-only product activity log.
package activity

import (
	"encoding/json"
	"time"

	"github.com/mishankov/todai/backend/internal/execution"
)

// Event is one immutable entry in the product activity log.
type Event struct {
	StreamOffset  int64               `db:"stream_offset" json:"streamOffset"`
	ID            string              `db:"id" json:"id"`
	UserID        *string             `db:"user_id" json:"-"`
	ProjectID     *string             `db:"project_id" json:"projectId"`
	Type          string              `db:"type" json:"type"`
	OccurredAt    time.Time           `db:"occurred_at" json:"occurredAt"`
	ActorType     execution.ActorType `db:"actor_type" json:"actorType"`
	ActorID       *string             `db:"actor_id" json:"actorId"`
	Source        execution.Source    `db:"source" json:"source"`
	AggregateType *string             `db:"aggregate_type" json:"aggregateType"`
	AggregateID   *string             `db:"aggregate_id" json:"aggregateId"`
	CorrelationID string              `db:"correlation_id" json:"correlationId"`
	AgentRunID    *string             `db:"agent_run_id" json:"agentRunId"`
	Payload       json.RawMessage     `db:"payload" json:"payload"`
}

// NewEvent describes an event before persistence assigns its ID and occurrence time.
type NewEvent struct {
	Type          string
	ProjectID     *string
	AggregateType *string
	AggregateID   *string
	Payload       any
}
