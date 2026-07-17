package activity

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/execution"
)

const eventColumns = `
	id, user_id, type, occurred_at, actor_type, actor_id, source,
	aggregate_type, aggregate_id, correlation_id, agent_run_id, payload
`

var (
	// ErrTypeRequired indicates that an event type is blank.
	ErrTypeRequired = errors.New("activity event type is required")
	// ErrAggregateIncomplete indicates that only one aggregate identifier field was provided.
	ErrAggregateIncomplete = errors.New("activity event aggregate type and id must be provided together")
	// ErrInvalidPayload indicates that the payload is not a JSON object.
	ErrInvalidPayload = errors.New("activity event payload must be a JSON object")
	// ErrInvalidLimit indicates that a list limit is outside the supported range.
	ErrInvalidLimit = errors.New("activity event limit must be between 1 and 200")
)

//go:embed migrations/*.sql
var migrations embed.FS

// Repository persists and reads activity events.
type Repository struct {
	db *sqlx.DB
}

// NewRepository constructs an activity event repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Migrations exposes activity migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Append writes an event using the supplied database or transaction executor.
func (r *Repository) Append(
	ctx context.Context,
	executor sqlx.ExtContext,
	scope execution.Scope,
	newEvent NewEvent,
) (Event, error) {
	if err := scope.Validate(); err != nil {
		return Event{}, fmt.Errorf("validate activity execution: %w", err)
	}

	payload, err := validateNewEvent(newEvent)
	if err != nil {
		return Event{}, err
	}

	var appended Event
	if err := sqlx.GetContext(ctx, executor, &appended, `
		INSERT INTO activity_events (
			id, user_id, type, actor_type, actor_id, source, aggregate_type,
			aggregate_id, correlation_id, agent_run_id, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::JSONB)
		RETURNING `+eventColumns,
		uuid.NewString(), scope.UserID, strings.TrimSpace(newEvent.Type),
		scope.ActorType, scope.ActorID, scope.Source,
		trimmedPointer(newEvent.AggregateType), trimmedPointer(newEvent.AggregateID),
		scope.CorrelationID, scope.AgentRunID, string(payload),
	); err != nil {
		return Event{}, fmt.Errorf("insert activity event: %w", err)
	}

	return appended, nil
}

// List returns a user's newest activity events first.
func (r *Repository) List(ctx context.Context, userID string, limit int) ([]Event, error) {
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}

	events := make([]Event, 0)
	if err := r.db.SelectContext(ctx, &events, `
		SELECT `+eventColumns+`
		FROM activity_events
		WHERE user_id = $1
		ORDER BY occurred_at DESC, id DESC
		LIMIT $2
	`, userID, limit); err != nil {
		return nil, fmt.Errorf("select activity events: %w", err)
	}

	return events, nil
}

func validateNewEvent(newEvent NewEvent) (json.RawMessage, error) {
	if strings.TrimSpace(newEvent.Type) == "" {
		return nil, ErrTypeRequired
	}
	if (newEvent.AggregateType == nil) != (newEvent.AggregateID == nil) {
		return nil, ErrAggregateIncomplete
	}
	if newEvent.AggregateType != nil &&
		(strings.TrimSpace(*newEvent.AggregateType) == "" || strings.TrimSpace(*newEvent.AggregateID) == "") {
		return nil, ErrAggregateIncomplete
	}

	payload, err := json.Marshal(newEvent.Payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil || object == nil {
		return nil, ErrInvalidPayload
	}

	return payload, nil
}

func trimmedPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
