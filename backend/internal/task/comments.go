package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
)

const commentColumns = `
	id, task_id, user_id, author_id, body, version, created_at, updated_at, last_modified_by
`

// ListComments returns comments only after verifying ownership of their task.
func (r *Repository) ListComments(ctx context.Context, userID, taskID string) ([]Comment, error) {
	if _, err := r.Get(ctx, userID, taskID); err != nil {
		return nil, err
	}
	comments := make([]Comment, 0)
	if err := r.db.SelectContext(ctx, &comments, `
		SELECT `+commentColumns+`
		FROM task_comments
		WHERE task_id = $1 AND user_id = $2
		ORDER BY created_at, id
	`, taskID, userID); err != nil {
		return nil, fmt.Errorf("select task comments: %w", err)
	}
	return comments, nil
}

// CreateComment appends a comment and its activity event atomically.
func (r *Repository) CreateComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	body string,
) (Comment, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Comment{}, fmt.Errorf("begin task comment creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	parent, err := getTaskForUpdate(ctx, tx, scope.UserID, taskID)
	if err != nil {
		return Comment{}, err
	}
	var created Comment
	if err := tx.GetContext(ctx, &created, `
		INSERT INTO task_comments (
			id, task_id, user_id, author_id, body, version, last_modified_by
		)
		VALUES ($1, $2, $3, $3, $4, 1, $5)
		RETURNING `+commentColumns,
		uuid.NewString(), taskID, scope.UserID, body, scope.ModifiedBy(),
	); err != nil {
		return Comment{}, fmt.Errorf("insert task comment: %w", err)
	}
	if err := r.appendCommentEvent(ctx, tx, scope, "task.comment.created", parent, created); err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Comment{}, fmt.Errorf("commit task comment creation: %w", err)
	}
	return created, nil
}

// UpdateComment changes comment content using optimistic concurrency.
func (r *Repository) UpdateComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	commentID string,
	body string,
	version int64,
) (Comment, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Comment{}, fmt.Errorf("begin task comment update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	parent, err := getTaskForUpdate(ctx, tx, scope.UserID, taskID)
	if err != nil {
		return Comment{}, err
	}
	current, err := getCommentForUpdate(ctx, tx, scope.UserID, taskID, commentID)
	if err != nil {
		return Comment{}, err
	}
	if current.Version != version {
		return Comment{}, ErrCommentVersionConflict
	}
	var updated Comment
	if err := tx.GetContext(ctx, &updated, `
		UPDATE task_comments
		SET body = $4, version = version + 1,
			updated_at = CURRENT_TIMESTAMP, last_modified_by = $5
		WHERE id = $1 AND task_id = $2 AND user_id = $3 AND version = $6
		RETURNING `+commentColumns,
		commentID, taskID, scope.UserID, body, scope.ModifiedBy(), version,
	); errors.Is(err, sql.ErrNoRows) {
		return Comment{}, ErrCommentVersionConflict
	} else if err != nil {
		return Comment{}, fmt.Errorf("update task comment: %w", err)
	}
	if err := r.appendCommentEvent(ctx, tx, scope, "task.comment.updated", parent, updated); err != nil {
		return Comment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Comment{}, fmt.Errorf("commit task comment update: %w", err)
	}
	return updated, nil
}

// DeleteComment removes a comment and records its final snapshot atomically.
func (r *Repository) DeleteComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	commentID string,
	version int64,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task comment deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	parent, err := getTaskForUpdate(ctx, tx, scope.UserID, taskID)
	if err != nil {
		return err
	}
	deleted, err := getCommentForUpdate(ctx, tx, scope.UserID, taskID, commentID)
	if err != nil {
		return err
	}
	if deleted.Version != version {
		return ErrCommentVersionConflict
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM task_comments
		WHERE id = $1 AND task_id = $2 AND user_id = $3 AND version = $4
	`, commentID, taskID, scope.UserID, version); err != nil {
		return fmt.Errorf("delete task comment: %w", err)
	}
	if err := r.appendCommentEvent(ctx, tx, scope, "task.comment.deleted", parent, deleted); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task comment deletion: %w", err)
	}
	return nil
}

func getCommentForUpdate(
	ctx context.Context,
	executor sqlx.ExtContext,
	userID string,
	taskID string,
	commentID string,
) (Comment, error) {
	var found Comment
	if err := sqlx.GetContext(ctx, executor, &found, `
		SELECT `+commentColumns+`
		FROM task_comments
		WHERE id = $1 AND task_id = $2 AND user_id = $3
		FOR UPDATE
	`, commentID, taskID, userID); errors.Is(err, sql.ErrNoRows) {
		return Comment{}, ErrCommentNotFound
	} else if err != nil {
		return Comment{}, fmt.Errorf("lock task comment: %w", err)
	}
	return found, nil
}

func (r *Repository) appendCommentEvent(
	ctx context.Context,
	executor sqlx.ExtContext,
	scope execution.Scope,
	eventType string,
	parent Task,
	comment Comment,
) error {
	aggregateType := "task_comment"
	aggregateID := comment.ID
	if _, err := r.events.Append(ctx, executor, scope, activity.NewEvent{
		Type:          eventType,
		AggregateType: &aggregateType,
		AggregateID:   &aggregateID,
		Payload: map[string]any{
			"schemaVersion": 1,
			"taskId":        comment.TaskID,
			"task":          taskEventSnapshot(parent),
			"comment": map[string]any{
				"id":       comment.ID,
				"taskId":   comment.TaskID,
				"authorId": comment.AuthorID,
				"body":     comment.Body,
				"version":  comment.Version,
			},
		},
	}); err != nil {
		return fmt.Errorf("append %s event: %w", eventType, err)
	}
	return nil
}
