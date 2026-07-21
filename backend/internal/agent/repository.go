package agent

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
)

const (
	sessionColumns = `id, user_id, kind, created_at, updated_at`
	messageColumns = `id, session_id, run_id, role, content, context_type, context_id, context_action,
		created_at, updated_at`
	runColumns = `id, session_id, user_id, kind, context_type, context_id, context_action,
		status, correlation_id, started_at,
		completed_at, error, created_at, updated_at`
	runEventColumns = `stream_offset, run_id, session_id, runtime_sequence, type,
		occurred_at, payload`
)

var (
	// ErrSessionNotFound indicates that a session does not belong to the user.
	ErrSessionNotFound = errors.New("agent session not found")
	// ErrRunNotFound indicates that a run does not belong to the user.
	ErrRunNotFound = errors.New("agent run not found")
	// ErrRunFinished indicates that a terminal run cannot accept another event.
	ErrRunFinished = errors.New("agent run is already finished")
	// ErrMessageRequired indicates that a submitted user message is blank.
	ErrMessageRequired = errors.New("agent message is required")
	// ErrInvalidMessageContext indicates an unsupported or incomplete product context.
	ErrInvalidMessageContext = errors.New("agent message context is invalid")
	// ErrInvalidEvent indicates that a runtime event violates the product protocol.
	ErrInvalidEvent = errors.New("agent runtime event is invalid")
	// ErrInvalidEventCursor indicates a negative SSE replay cursor.
	ErrInvalidEventCursor = errors.New("agent event cursor must not be negative")
	// ErrInvalidEventLimit indicates an unsupported event page size.
	ErrInvalidEventLimit = errors.New("agent event limit must be between 1 and 200")
)

//go:embed migrations/*.sql
var migrations embed.FS

// ActivityAppender persists lifecycle activity in the surrounding transaction.
type ActivityAppender interface {
	Append(context.Context, sqlx.ExtContext, execution.Scope, activity.NewEvent) (activity.Event, error)
}

// Repository persists agent sessions, messages, runs, and replayable run events.
type Repository struct {
	db     *sqlx.DB
	events ActivityAppender
}

// NewRepository constructs an agent repository.
func NewRepository(db *sqlx.DB, events ActivityAppender) *Repository {
	return &Repository{db: db, events: events}
}

// Migrations exposes agent migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// CreateSession creates a user-owned agent conversation and its activity event atomically.
func (r *Repository) CreateSession(ctx context.Context, scope execution.Scope) (Session, error) {
	if err := scope.Validate(); err != nil {
		return Session{}, fmt.Errorf("validate session execution: %w", err)
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Session{}, fmt.Errorf("begin create agent session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var created Session
	if err := tx.GetContext(ctx, &created, `
		INSERT INTO agent_sessions (id, user_id, kind)
		VALUES ($1, $2, $3)
		RETURNING `+sessionColumns,
		uuid.NewString(), scope.UserID, ExecutionConversation,
	); err != nil {
		return Session{}, fmt.Errorf("insert agent session: %w", err)
	}
	if err := r.appendLifecycle(ctx, tx, scope, "agent.session.created", "agent_session", created.ID, map[string]any{
		"schemaVersion": 1,
		"sessionId":     created.ID,
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(); err != nil {
		return Session{}, fmt.Errorf("commit agent session: %w", err)
	}
	return created, nil
}

// GetSession returns one session only when it belongs to the user.
func (r *Repository) GetSession(ctx context.Context, userID, sessionID string) (Session, error) {
	var found Session
	if err := r.db.GetContext(ctx, &found, `
		SELECT `+sessionColumns+`
		FROM agent_sessions
		WHERE id = $1 AND user_id = $2 AND kind = $3
	`, sessionID, userID, ExecutionConversation); errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	} else if err != nil {
		return Session{}, fmt.Errorf("select agent session: %w", err)
	}
	return found, nil
}

// GetConversation returns a session together with its canonical history and runs.
func (r *Repository) GetConversation(
	ctx context.Context,
	userID string,
	sessionID string,
) (Session, []Message, []Run, int64, error) {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("begin get agent conversation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var session Session
	if err := tx.GetContext(ctx, &session, `
		SELECT `+sessionColumns+`
		FROM agent_sessions
		WHERE id = $1 AND user_id = $2 AND kind = $3
	`, sessionID, userID, ExecutionConversation); errors.Is(err, sql.ErrNoRows) {
		return Session{}, nil, nil, 0, ErrSessionNotFound
	} else if err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("select agent session: %w", err)
	}
	messages := make([]Message, 0)
	if err := tx.SelectContext(ctx, &messages, `
		SELECT `+messageColumns+`
		FROM agent_messages
		WHERE session_id = $1
			AND (run_id IS NULL OR run_id IN (
				SELECT id FROM agent_runs WHERE kind = $2
			))
		ORDER BY created_at, id
	`, sessionID, ExecutionConversation); err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("select agent messages: %w", err)
	}
	hydrateMessageContexts(messages)
	runs := make([]Run, 0)
	if err := tx.SelectContext(ctx, &runs, `
		SELECT `+runColumns+`
		FROM agent_runs
		WHERE session_id = $1 AND user_id = $2 AND kind = $3
		ORDER BY created_at, id
	`, sessionID, userID, ExecutionConversation); err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("select agent runs: %w", err)
	}
	var lastStreamOffset int64
	if err := tx.GetContext(ctx, &lastStreamOffset, `
		SELECT COALESCE(MAX(stream_offset), 0)
		FROM agent_run_events
		WHERE session_id = $1
			AND run_id IN (SELECT id FROM agent_runs WHERE kind = $2)
	`, sessionID, ExecutionConversation); err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("select agent stream offset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Session{}, nil, nil, 0, fmt.Errorf("commit get agent conversation: %w", err)
	}
	return session, messages, runs, lastStreamOffset, nil
}

// CreateMessageRun atomically persists a user message and its queued run.
func (r *Repository) CreateMessageRun(
	ctx context.Context,
	scope execution.Scope,
	sessionID string,
	input MessageInput,
) (Message, Run, []HistoryMessage, error) {
	if err := scope.Validate(); err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("validate message execution: %w", err)
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return Message{}, Run{}, nil, ErrMessageRequired
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("begin agent message: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var ownedID string
	if err := tx.GetContext(ctx, &ownedID, `
		SELECT id FROM agent_sessions
		WHERE id = $1 AND user_id = $2 AND kind = $3
		FOR UPDATE
	`, sessionID, scope.UserID, ExecutionConversation); errors.Is(err, sql.ErrNoRows) {
		return Message{}, Run{}, nil, ErrSessionNotFound
	} else if err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("lock agent session: %w", err)
	}

	history, err := loadHistory(ctx, tx, sessionID)
	if err != nil {
		return Message{}, Run{}, nil, err
	}

	runID := uuid.NewString()
	var run Run
	if err := tx.GetContext(ctx, &run, `
		INSERT INTO agent_runs (id, session_id, user_id, status, correlation_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+runColumns,
		runID, sessionID, scope.UserID, RunStatusQueued, scope.CorrelationID,
	); err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("insert agent run: %w", err)
	}

	var message Message
	if err := tx.GetContext(ctx, &message, `
		INSERT INTO agent_messages (id, session_id, run_id, role, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+messageColumns,
		uuid.NewString(), sessionID, runID, RoleUser, content,
	); err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("insert agent message: %w", err)
	}
	hydrateMessageContext(&message)
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = $1
	`, sessionID); err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("touch agent session: %w", err)
	}

	if err := r.appendLifecycle(ctx, tx, scope, "agent.message.created", "agent_message", message.ID, map[string]any{
		"schemaVersion": 1,
		"sessionId":     sessionID,
		"runId":         runID,
		"role":          RoleUser,
	}); err != nil {
		return Message{}, Run{}, nil, err
	}
	if err := r.appendLifecycle(ctx, tx, scope, "agent.run.queued", "agent_run", runID, map[string]any{
		"schemaVersion": 1,
		"sessionId":     sessionID,
		"runId":         runID,
		"status":        RunStatusQueued,
	}); err != nil {
		return Message{}, Run{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return Message{}, Run{}, nil, fmt.Errorf("commit agent message: %w", err)
	}
	return message, run, history, nil
}

// CreateContextRun atomically creates a private execution context and queued run without messages.
func (r *Repository) CreateContextRun(
	ctx context.Context,
	scope execution.Scope,
	messageContext MessageContext,
) (Run, error) {
	if err := scope.Validate(); err != nil {
		return Run{}, fmt.Errorf("validate contextual run execution: %w", err)
	}
	contextType, contextID, contextAction, err := normalizeMessageContext(&messageContext)
	if err != nil {
		return Run{}, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Run{}, fmt.Errorf("begin contextual agent run: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	executionID := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_sessions (id, user_id, kind)
		VALUES ($1, $2, $3)
	`, executionID, scope.UserID, ExecutionAction); err != nil {
		return Run{}, fmt.Errorf("insert contextual agent execution: %w", err)
	}

	var run Run
	if err := tx.GetContext(ctx, &run, `
		INSERT INTO agent_runs (
			id, session_id, user_id, kind, context_type, context_id, context_action,
			status, correlation_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+runColumns,
		uuid.NewString(), executionID, scope.UserID, ExecutionAction,
		contextType, contextID, contextAction, RunStatusQueued, scope.CorrelationID,
	); err != nil {
		return Run{}, fmt.Errorf("insert contextual agent run: %w", err)
	}
	if err := r.appendLifecycle(ctx, tx, scope, "agent.run.queued", "agent_run", run.ID, map[string]any{
		"schemaVersion": 1,
		"runId":         run.ID,
		"status":        RunStatusQueued,
		"contextType":   contextType,
		"contextId":     contextID,
		"contextAction": contextAction,
	}); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(); err != nil {
		return Run{}, fmt.Errorf("commit contextual agent run: %w", err)
	}
	return run, nil
}

func loadHistory(ctx context.Context, tx *sqlx.Tx, sessionID string) ([]HistoryMessage, error) {
	var runs []Run
	if err := tx.SelectContext(ctx, &runs, `
		SELECT `+runColumns+`
		FROM agent_runs
		WHERE session_id = $1 AND kind = $2
		ORDER BY created_at, id
	`, sessionID, ExecutionConversation); err != nil {
		return nil, fmt.Errorf("select agent history runs: %w", err)
	}
	var messages []Message
	if err := tx.SelectContext(ctx, &messages, `
		SELECT `+messageColumns+`
		FROM agent_messages
		WHERE session_id = $1
			AND (run_id IS NULL OR run_id IN (
				SELECT id FROM agent_runs WHERE kind = $2
			))
		ORDER BY created_at, id
	`, sessionID, ExecutionConversation); err != nil {
		return nil, fmt.Errorf("select agent history messages: %w", err)
	}
	hydrateMessageContexts(messages)
	type historyEvent struct {
		RunID   string          `db:"run_id"`
		Payload json.RawMessage `db:"payload"`
	}
	var events []historyEvent
	if err := tx.SelectContext(ctx, &events, `
		SELECT run_id, payload
		FROM agent_run_events
		WHERE session_id = $1 AND type = $2
			AND run_id IN (SELECT id FROM agent_runs WHERE kind = $3)
		ORDER BY stream_offset
	`, sessionID, EventHistoryMessage, ExecutionConversation); err != nil {
		return nil, fmt.Errorf("select agent history events: %w", err)
	}

	messagesByRun := make(map[string][]Message)
	for _, message := range messages {
		if message.RunID != nil {
			messagesByRun[*message.RunID] = append(messagesByRun[*message.RunID], message)
		}
	}
	historyByRun := make(map[string][]HistoryMessage)
	for _, event := range events {
		message, err := decodeHistoryPayload(event.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode stored agent history: %w", err)
		}
		historyByRun[event.RunID] = append(historyByRun[event.RunID], message)
	}

	history := make([]HistoryMessage, 0, len(messages)+len(events))
	for _, run := range runs {
		var assistant *Message
		for index := range messagesByRun[run.ID] {
			message := messagesByRun[run.ID][index]
			switch message.Role {
			case RoleUser:
				history = append(history, canonicalHistoryMessage(message))
			case RoleAssistant:
				assistant = &message
			}
		}
		if runHistory := historyByRun[run.ID]; len(runHistory) > 0 {
			history = append(history, runHistory...)
		} else if assistant != nil && assistant.Content != "" {
			history = append(history, canonicalHistoryMessage(*assistant))
		}
	}
	return history, nil
}

func canonicalHistoryMessage(message Message) HistoryMessage {
	role := HistoryRoleUser
	if message.Role == RoleAssistant {
		role = HistoryRoleAssistant
	}
	return HistoryMessage{
		Role: role, Content: []HistoryContent{{Type: "text", Text: modelVisibleContent(message)}},
		Timestamp: message.CreatedAt.UnixMilli(),
	}
}

func normalizeMessageContext(context *MessageContext) (*ContextType, *string, *ContextAction, error) {
	if context == nil {
		return nil, nil, nil, nil
	}
	if err := context.Validate(); err != nil {
		return nil, nil, nil, err
	}
	contextID := strings.TrimSpace(context.TaskID)
	contextType := context.Type
	contextAction := context.Action
	return &contextType, &contextID, &contextAction, nil
}

func modelVisibleContent(message Message) string {
	context := messageContext(message)
	if context == nil {
		return message.Content
	}
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return message.Content
	}
	return "<todai-context>" + string(contextJSON) + "</todai-context>\n\n" + message.Content
}

func messageContext(message Message) *MessageContext {
	if message.Context != nil {
		copy := *message.Context
		return &copy
	}
	if message.ContextType == nil || message.ContextID == nil || message.ContextAction == nil {
		return nil
	}
	return &MessageContext{
		Type: *message.ContextType, TaskID: *message.ContextID, Action: *message.ContextAction,
	}
}

func hydrateMessageContexts(messages []Message) {
	for index := range messages {
		hydrateMessageContext(&messages[index])
	}
}

func hydrateMessageContext(message *Message) {
	message.Context = messageContext(*message)
}

// GetRun returns one run only when it belongs to the user.
func (r *Repository) GetRun(ctx context.Context, userID, runID string) (Run, error) {
	var found Run
	if err := r.db.GetContext(ctx, &found, `
		SELECT `+runColumns+` FROM agent_runs WHERE id = $1 AND user_id = $2
	`, runID, userID); errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	} else if err != nil {
		return Run{}, fmt.Errorf("select agent run: %w", err)
	}
	return found, nil
}

// ListRunEvents returns persisted session events after a durable stream cursor.
func (r *Repository) ListRunEvents(
	ctx context.Context,
	userID string,
	sessionID string,
	after int64,
	limit int,
) ([]RunEvent, error) {
	if after < 0 {
		return nil, ErrInvalidEventCursor
	}
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidEventLimit
	}
	if _, err := r.GetSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	events := make([]RunEvent, 0)
	if err := r.db.SelectContext(ctx, &events, `
		SELECT `+runEventColumns+`
		FROM agent_run_events
		WHERE session_id = $1 AND stream_offset > $2
			AND run_id IN (SELECT id FROM agent_runs WHERE kind = $4)
		ORDER BY stream_offset
		LIMIT $3
	`, sessionID, after, limit, ExecutionConversation); err != nil {
		return nil, fmt.Errorf("select agent run events: %w", err)
	}
	return events, nil
}

// ListContextRunEvents returns replayable events for an owned isolated contextual run.
func (r *Repository) ListContextRunEvents(
	ctx context.Context,
	userID string,
	runID string,
	after int64,
	limit int,
) ([]RunEvent, error) {
	if after < 0 {
		return nil, ErrInvalidEventCursor
	}
	if limit < 1 || limit > 200 {
		return nil, ErrInvalidEventLimit
	}
	var ownedID string
	if err := r.db.GetContext(ctx, &ownedID, `
		SELECT id FROM agent_runs
		WHERE id = $1 AND user_id = $2 AND kind = $3
	`, runID, userID, ExecutionAction); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRunNotFound
	} else if err != nil {
		return nil, fmt.Errorf("select contextual agent run: %w", err)
	}
	events := make([]RunEvent, 0)
	if err := r.db.SelectContext(ctx, &events, `
		SELECT `+runEventColumns+`
		FROM agent_run_events
		WHERE run_id = $1 AND stream_offset > $2
		ORDER BY stream_offset
		LIMIT $3
	`, runID, after, limit); err != nil {
		return nil, fmt.Errorf("select contextual agent run events: %w", err)
	}
	return events, nil
}

// RecordRuntimeEvent persists one stable runtime event before it can be streamed.
func (r *Repository) RecordRuntimeEvent(
	ctx context.Context,
	scope execution.Scope,
	runID string,
	event RuntimeEvent,
) (RunEvent, error) {
	if err := scope.Validate(); err != nil {
		return RunEvent{}, fmt.Errorf("validate run event execution: %w", err)
	}
	payload, err := validateRuntimeEvent(event)
	if err != nil {
		return RunEvent{}, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return RunEvent{}, fmt.Errorf("begin agent run event: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	run, err := lockRun(ctx, tx, scope.UserID, runID)
	if err != nil {
		return RunEvent{}, err
	}
	var existing RunEvent
	if err := tx.GetContext(ctx, &existing, `
		SELECT `+runEventColumns+`
		FROM agent_run_events
		WHERE run_id = $1 AND runtime_sequence = $2
	`, runID, event.Sequence); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return RunEvent{}, fmt.Errorf("select duplicate agent run event: %w", err)
	}
	var lastSequence int64
	if err := tx.GetContext(ctx, &lastSequence, `
		SELECT COALESCE(MAX(runtime_sequence), 0) FROM agent_run_events WHERE run_id = $1
	`, runID); err != nil {
		return RunEvent{}, fmt.Errorf("select last agent run sequence: %w", err)
	}
	if event.Sequence <= lastSequence {
		return RunEvent{}, ErrInvalidEvent
	}
	if isTerminalStatus(run.Status) {
		return RunEvent{}, ErrRunFinished
	}

	persisted, _, err := r.recordEventTx(ctx, tx, scope, run, event.Type, event.Sequence, payload)
	if err != nil {
		return RunEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return RunEvent{}, fmt.Errorf("commit agent run event: %w", err)
	}
	return persisted, nil
}

// FinishRun records a synthetic terminal event once and returns the final run.
func (r *Repository) FinishRun(
	ctx context.Context,
	scope execution.Scope,
	runID string,
	eventType string,
	payload any,
) (Run, error) {
	if err := scope.Validate(); err != nil {
		return Run{}, fmt.Errorf("validate run completion execution: %w", err)
	}
	if !isTerminalEvent(eventType) {
		return Run{}, ErrInvalidEvent
	}
	encoded, err := encodeEventPayload(payload)
	if err != nil {
		return Run{}, err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Run{}, fmt.Errorf("begin finish agent run: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	run, err := lockRun(ctx, tx, scope.UserID, runID)
	if err != nil {
		return Run{}, err
	}
	if isTerminalStatus(run.Status) {
		return run, nil
	}
	var sequence int64
	if err := tx.GetContext(ctx, &sequence, `
		SELECT COALESCE(MAX(runtime_sequence), 0) + 1 FROM agent_run_events WHERE run_id = $1
	`, runID); err != nil {
		return Run{}, fmt.Errorf("select next agent run sequence: %w", err)
	}
	_, finished, err := r.recordEventTx(ctx, tx, scope, run, eventType, sequence, encoded)
	if err != nil {
		return Run{}, err
	}
	if err := tx.Commit(); err != nil {
		return Run{}, fmt.Errorf("commit finished agent run: %w", err)
	}
	return finished, nil
}

func (r *Repository) recordEventTx(
	ctx context.Context,
	tx *sqlx.Tx,
	scope execution.Scope,
	run Run,
	eventType string,
	sequence int64,
	payload json.RawMessage,
) (RunEvent, Run, error) {
	status := run.Status
	switch eventType {
	case EventRunStarted:
		status = RunStatusRunning
	case EventRunCompleted:
		status = RunStatusCompleted
	case EventRunFailed:
		status = RunStatusFailed
	case EventRunAborted:
		status = RunStatusAborted
	default:
		if status == RunStatusQueued {
			status = RunStatusRunning
		}
	}

	var persisted RunEvent
	if err := tx.GetContext(ctx, &persisted, `
		INSERT INTO agent_run_events (
			run_id, session_id, runtime_sequence, type, payload
		)
		VALUES ($1, $2, $3, $4, $5::JSONB)
		RETURNING `+runEventColumns,
		run.ID, run.SessionID, sequence, eventType, string(payload),
	); err != nil {
		return RunEvent{}, Run{}, fmt.Errorf("insert agent run event: %w", err)
	}

	var updated Run
	terminal := isTerminalStatus(status)
	if err := tx.GetContext(ctx, &updated, `
		UPDATE agent_runs
		SET status = $2::VARCHAR,
			started_at = CASE WHEN $2::VARCHAR <> 'queued' THEN COALESCE(started_at, CURRENT_TIMESTAMP) ELSE started_at END,
			completed_at = CASE WHEN $3 THEN CURRENT_TIMESTAMP ELSE completed_at END,
			error = CASE WHEN $2::VARCHAR = 'failed' THEN $4 ELSE error END,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING `+runColumns,
		run.ID, status, terminal, eventError(payload),
	); err != nil {
		return RunEvent{}, Run{}, fmt.Errorf("update agent run state: %w", err)
	}

	if eventType == EventMessageDelta && run.Kind == ExecutionConversation {
		if err := appendAssistantDelta(ctx, tx, run, payload); err != nil {
			return RunEvent{}, Run{}, err
		}
	}
	if eventType == EventRunStarted || isTerminalEvent(eventType) {
		if err := r.appendLifecycle(ctx, tx, scope, eventType, "agent_run", run.ID, map[string]any{
			"schemaVersion": 1,
			"sessionId":     run.SessionID,
			"runId":         run.ID,
			"status":        status,
		}); err != nil {
			return RunEvent{}, Run{}, err
		}
	}
	return persisted, updated, nil
}

func lockRun(ctx context.Context, tx *sqlx.Tx, userID, runID string) (Run, error) {
	var run Run
	if err := tx.GetContext(ctx, &run, `
		SELECT `+runColumns+`
		FROM agent_runs
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, runID, userID); errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrRunNotFound
	} else if err != nil {
		return Run{}, fmt.Errorf("lock agent run: %w", err)
	}
	return run, nil
}

func validateRuntimeEvent(event RuntimeEvent) (json.RawMessage, error) {
	if event.Sequence < 1 || !validEventType(event.Type) {
		return nil, ErrInvalidEvent
	}
	payload, err := encodeEventPayload(event.Payload)
	if err != nil {
		return nil, err
	}
	if event.Type == EventHistoryMessage {
		if _, err := decodeHistoryPayload(payload); err != nil {
			return nil, ErrInvalidEvent
		}
	}
	return payload, nil
}

func encodeEventPayload(payload any) (json.RawMessage, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: encode payload: %v", ErrInvalidEvent, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return nil, ErrInvalidEvent
	}
	return encoded, nil
}

func validEventType(eventType string) bool {
	switch eventType {
	case EventRunStarted, EventMessageDelta, EventToolStarted, EventToolCompleted, EventHistoryMessage,
		EventTaskChanged, EventRunCompleted, EventRunFailed, EventRunAborted:
		return true
	default:
		return false
	}
}

func decodeHistoryPayload(payload json.RawMessage) (HistoryMessage, error) {
	var body struct {
		Message HistoryMessage `json:"message"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return HistoryMessage{}, err
	}
	if err := validateHistoryMessage(body.Message); err != nil {
		return HistoryMessage{}, err
	}
	if body.Message.Role == HistoryRoleUser {
		return HistoryMessage{}, ErrInvalidEvent
	}
	return body.Message, nil
}

func validateHistoryMessage(message HistoryMessage) error {
	if message.Timestamp < 1 {
		return ErrInvalidEvent
	}
	switch message.Role {
	case HistoryRoleUser:
		if len(message.Content) != 1 || message.Content[0].Type != "text" ||
			strings.TrimSpace(message.Content[0].Text) == "" {
			return ErrInvalidEvent
		}
	case HistoryRoleAssistant:
		if len(message.Content) == 0 {
			return ErrInvalidEvent
		}
		for _, content := range message.Content {
			switch content.Type {
			case "text":
				if content.Text == "" {
					return ErrInvalidEvent
				}
			case "toolCall":
				if strings.TrimSpace(content.ID) == "" || strings.TrimSpace(content.Name) == "" ||
					content.Arguments == nil {
					return ErrInvalidEvent
				}
			default:
				return ErrInvalidEvent
			}
		}
	case HistoryRoleToolResult:
		if strings.TrimSpace(message.ToolCallID) == "" || strings.TrimSpace(message.ToolName) == "" {
			return ErrInvalidEvent
		}
		for _, content := range message.Content {
			if content.Type != "text" || content.Text == "" {
				return ErrInvalidEvent
			}
		}
	default:
		return ErrInvalidEvent
	}
	return nil
}

func isTerminalEvent(eventType string) bool {
	return eventType == EventRunCompleted || eventType == EventRunFailed || eventType == EventRunAborted
}

func isTerminalStatus(status RunStatus) bool {
	return status == RunStatusCompleted || status == RunStatusFailed || status == RunStatusAborted
}

func appendAssistantDelta(ctx context.Context, tx *sqlx.Tx, run Run, payload json.RawMessage) error {
	var body struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal(payload, &body); err != nil || body.Delta == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_messages (id, session_id, run_id, role, content)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (run_id, role) WHERE role = 'assistant'
		DO UPDATE SET content = agent_messages.content || EXCLUDED.content,
			updated_at = CURRENT_TIMESTAMP
	`, uuid.NewString(), run.SessionID, run.ID, RoleAssistant, body.Delta); err != nil {
		return fmt.Errorf("append assistant message delta: %w", err)
	}
	return nil
}

func eventError(payload json.RawMessage) *string {
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(payload, &body) != nil || len(body.Error) == 0 {
		return nil
	}
	var message string
	if json.Unmarshal(body.Error, &message) == nil && strings.TrimSpace(message) != "" {
		return &message
	}
	var structured struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body.Error, &structured) != nil || strings.TrimSpace(structured.Message) == "" {
		return nil
	}
	return &structured.Message
}

func (r *Repository) appendLifecycle(
	ctx context.Context,
	executor sqlx.ExtContext,
	scope execution.Scope,
	eventType string,
	aggregateType string,
	aggregateID string,
	payload any,
) error {
	if r.events == nil {
		return nil
	}
	if _, err := r.events.Append(ctx, executor, scope, activity.NewEvent{
		Type:          eventType,
		AggregateType: &aggregateType,
		AggregateID:   &aggregateID,
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("append %s activity event: %w", eventType, err)
	}
	return nil
}
