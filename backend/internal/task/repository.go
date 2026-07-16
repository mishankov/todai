package task

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const taskColumns = `
	id, user_id, project_id, parent_id, title, description, status, priority,
	due_at, due_timezone, position, version, completed_at, created_at, updated_at,
	last_modified_by
`

//go:embed migrations/*.sql
var migrations embed.FS

// Repository stores tasks in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

// NewRepository constructs a PostgreSQL task repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Migrations exposes task migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Create inserts an active top-level Inbox or project task.
func (r *Repository) Create(
	ctx context.Context,
	userID string,
	title string,
	projectID *string,
) (Task, error) {
	var created Task
	err := r.db.GetContext(ctx, &created, `
		INSERT INTO tasks (
			id, user_id, project_id, title, status, priority, position, version,
			created_at, updated_at, last_modified_by
		)
		SELECT
			$1::VARCHAR, $2::VARCHAR, $4::VARCHAR, $3::TEXT, 'active', 0,
			COALESCE(MAX(position), 0) + 1024, 1,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $2::VARCHAR
		FROM tasks
		WHERE user_id = $2::VARCHAR
			AND project_id IS NOT DISTINCT FROM $4::VARCHAR
			AND parent_id IS NULL
		HAVING $4::VARCHAR IS NULL OR EXISTS (
			SELECT 1 FROM projects
			WHERE id = $4::VARCHAR AND user_id = $2::VARCHAR AND archived_at IS NULL
		)
		RETURNING `+taskColumns,
		uuid.NewString(), userID, title, projectID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrProjectNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}

	return created, nil
}

// ListProject returns the user's top-level tasks in one project.
func (r *Repository) ListProject(
	ctx context.Context,
	userID string,
	projectID string,
	includeCompleted bool,
) ([]Task, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS (
			SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND archived_at IS NULL
		)
	`, projectID, userID); err != nil {
		return nil, fmt.Errorf("check project: %w", err)
	}
	if !exists {
		return nil, ErrProjectNotFound
	}

	tasks := make([]Task, 0)
	err := r.db.SelectContext(ctx, &tasks, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE user_id = $1
			AND project_id = $2
			AND parent_id IS NULL
			AND ($3 OR status = 'active')
		ORDER BY
			CASE status WHEN 'active' THEN 0 ELSE 1 END,
			position,
			created_at
	`, userID, projectID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("select project tasks: %w", err)
	}

	return tasks, nil
}

// Get returns one task scoped to its owner.
func (r *Repository) Get(ctx context.Context, userID, taskID string) (Task, error) {
	var found Task
	err := r.db.GetContext(
		ctx,
		&found,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1 AND user_id = $2`,
		taskID,
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("select task: %w", err)
	}

	return found, nil
}

// ListInbox returns the user's top-level tasks without a project.
func (r *Repository) ListInbox(
	ctx context.Context,
	userID string,
	includeCompleted bool,
) ([]Task, error) {
	tasks := make([]Task, 0)
	err := r.db.SelectContext(ctx, &tasks, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE user_id = $1
			AND project_id IS NULL
			AND parent_id IS NULL
			AND ($2 OR status = 'active')
		ORDER BY
			CASE status WHEN 'active' THEN 0 ELSE 1 END,
			position,
			created_at
	`, userID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("select Inbox tasks: %w", err)
	}

	return tasks, nil
}

// ListToday returns active tasks due by the end of the user's local day and
// tasks completed during that day when requested.
func (r *Repository) ListToday(
	ctx context.Context,
	userID string,
	dayStart time.Time,
	dayEnd time.Time,
	includeCompleted bool,
) ([]Task, error) {
	tasks := make([]Task, 0)
	err := r.db.SelectContext(ctx, &tasks, `
		SELECT `+taskColumns+`
		FROM tasks
		WHERE user_id = $1
			AND due_at IS NOT NULL
			AND due_at < $3
			AND (
				status = 'active'
				OR (
					$4
					AND status = 'completed'
					AND completed_at >= $2
					AND completed_at < $3
				)
			)
		ORDER BY
			CASE status WHEN 'active' THEN 0 ELSE 1 END,
			due_at,
			priority DESC,
			position,
			created_at
	`, userID, dayStart, dayEnd, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("select Today tasks: %w", err)
	}

	return tasks, nil
}

// Complete marks a task completed and returns its current representation.
func (r *Repository) Complete(ctx context.Context, userID, taskID string) (Task, error) {
	return r.setStatus(ctx, userID, taskID, StatusCompleted)
}

// Reopen marks a task active and returns its current representation.
func (r *Repository) Reopen(ctx context.Context, userID, taskID string) (Task, error) {
	return r.setStatus(ctx, userID, taskID, StatusActive)
}

// Update changes editable task fields using optimistic concurrency.
func (r *Repository) Update(
	ctx context.Context,
	userID string,
	taskID string,
	update Update,
) (Task, error) {
	var updated Task
	err := r.db.GetContext(ctx, &updated, `
		UPDATE tasks
		SET title = CASE WHEN $4::BOOLEAN THEN $5::TEXT ELSE title END,
			description = CASE WHEN $6::BOOLEAN THEN $7::TEXT ELSE description END,
			project_id = CASE WHEN $8::BOOLEAN THEN $9::VARCHAR ELSE project_id END,
			priority = CASE WHEN $10::BOOLEAN THEN $11::SMALLINT ELSE priority END,
			due_at = CASE WHEN $12::BOOLEAN THEN $13::TIMESTAMPTZ ELSE due_at END,
			due_timezone = CASE WHEN $14::BOOLEAN THEN $15::TEXT ELSE due_timezone END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $2
		WHERE id = $1 AND user_id = $2 AND version = $3
			AND (
				NOT $8::BOOLEAN
				OR $9::VARCHAR IS NULL
				OR EXISTS (
					SELECT 1 FROM projects
					WHERE id = $9::VARCHAR AND user_id = $2 AND archived_at IS NULL
				)
			)
		RETURNING `+taskColumns,
		taskID,
		userID,
		update.Version,
		update.Title != nil,
		pointerValue(update.Title),
		update.Description != nil,
		nullableValue(update.Description),
		update.ProjectID != nil,
		nullableValue(update.ProjectID),
		update.Priority != nil,
		pointerValue(update.Priority),
		update.DueAt != nil,
		nullableValue(update.DueAt),
		update.DueTimezone != nil,
		nullableValue(update.DueTimezone),
	)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	if _, getErr := r.Get(ctx, userID, taskID); getErr != nil {
		return Task{}, getErr
	}
	if update.ProjectID != nil && update.ProjectID.Value != nil {
		var exists bool
		if existsErr := r.db.GetContext(ctx, &exists, `
			SELECT EXISTS (
				SELECT 1 FROM projects WHERE id = $1 AND user_id = $2 AND archived_at IS NULL
			)
		`, *update.ProjectID.Value, userID); existsErr != nil {
			return Task{}, fmt.Errorf("check destination project: %w", existsErr)
		}
		if !exists {
			return Task{}, ErrProjectNotFound
		}
	}

	return Task{}, ErrVersionConflict
}

// Delete permanently removes a task scoped to its owner.
func (r *Repository) Delete(ctx context.Context, userID, taskID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = $1 AND user_id = $2", taskID, userID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get deleted task count: %w", err)
	}
	if deleted == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (r *Repository) setStatus(
	ctx context.Context,
	userID string,
	taskID string,
	status Status,
) (Task, error) {
	var updated Task
	err := r.db.GetContext(ctx, &updated, `
		UPDATE tasks
		SET status = $3::VARCHAR,
			completed_at = CASE WHEN $3::VARCHAR = 'completed' THEN CURRENT_TIMESTAMP ELSE NULL END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $2
		WHERE id = $1 AND user_id = $2 AND status <> $3::VARCHAR
		RETURNING `+taskColumns,
		taskID, userID, status,
	)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Task{}, fmt.Errorf("update task status: %w", err)
	}

	return r.Get(ctx, userID, taskID)
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}

func nullableValue[T any](value *Nullable[T]) any {
	if value == nil || value.Value == nil {
		return nil
	}

	return *value.Value
}
