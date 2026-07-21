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
	stream_offset, id, user_id, type, occurred_at, actor_type, actor_id, source,
	project_id, aggregate_type, aggregate_id, correlation_id, agent_run_id, payload
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
	// ErrInvalidStreamCursor indicates a negative activity stream cursor.
	ErrInvalidStreamCursor = errors.New("activity stream cursor must be zero or greater")
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
	projectID := newEvent.ProjectID
	if projectID == nil {
		projectID = scope.ProjectID
	}

	var appended Event
	if err := sqlx.GetContext(ctx, executor, &appended, `
		INSERT INTO activity_events (
			id, user_id, type, actor_type, actor_id, source, aggregate_type,
			aggregate_id, correlation_id, agent_run_id, payload, project_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::JSONB, $12)
		RETURNING `+eventColumns,
		uuid.NewString(), scope.UserID, strings.TrimSpace(newEvent.Type),
		scope.ActorType, scope.ActorID, scope.Source,
		trimmedPointer(newEvent.AggregateType), trimmedPointer(newEvent.AggregateID),
		scope.CorrelationID, scope.AgentRunID, string(payload), trimmedPointer(projectID),
	); err != nil {
		return Event{}, fmt.Errorf("insert activity event: %w", err)
	}

	return appended, nil
}

// List returns a user's newest activity events first.
func (r *Repository) List(ctx context.Context, userID, projectID string, limit int) ([]Event, error) {
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}

	events := make([]Event, 0)
	if err := r.db.SelectContext(ctx, &events, `
		SELECT `+eventColumns+`
		FROM activity_events
		WHERE user_id = $1 AND project_id IS NOT DISTINCT FROM NULLIF($2, '')
		ORDER BY stream_offset DESC
		LIMIT $3
	`, userID, projectID, limit); err != nil {
		return nil, fmt.Errorf("select activity events: %w", err)
	}

	return events, nil
}

// LatestOffset returns the newest durable event offset visible to a user.
func (r *Repository) LatestOffset(ctx context.Context, userID, projectID string) (int64, error) {
	var offset int64
	if err := r.db.GetContext(ctx, &offset, `
		SELECT COALESCE(MAX(stream_offset), 0)
		FROM activity_events
		WHERE user_id = $1 AND project_id IS NOT DISTINCT FROM NULLIF($2, '')
	`, userID, projectID); err != nil {
		return 0, fmt.Errorf("select latest activity offset: %w", err)
	}
	return offset, nil
}

// ListAfter returns a user's events after a durable stream cursor, oldest first.
func (r *Repository) ListAfter(
	ctx context.Context,
	userID, projectID string,
	after int64,
	limit int,
) ([]Event, error) {
	if after < 0 {
		return nil, ErrInvalidStreamCursor
	}
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidLimit
	}

	events := make([]Event, 0)
	if err := r.db.SelectContext(ctx, &events, `
		SELECT `+eventColumns+`
		FROM activity_events
		WHERE user_id = $1
			AND project_id IS NOT DISTINCT FROM NULLIF($2, '')
			AND stream_offset > $3
		ORDER BY stream_offset
		LIMIT $4
	`, userID, projectID, after, limit); err != nil {
		return nil, fmt.Errorf("select activity stream events: %w", err)
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
