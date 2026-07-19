package usersettings

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jmoiron/sqlx"

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
)

const settingsColumns = `
	user_id, timezone, agent_model, agent_thinking_effort,
	version, created_at, updated_at, last_modified_by
`

//go:embed migrations/*.sql
var migrations embed.FS

type activityAppender interface {
	Append(context.Context, sqlx.ExtContext, execution.Scope, activity.NewEvent) (activity.Event, error)
}

// Repository persists user settings in PostgreSQL.
type Repository struct {
	db     *sqlx.DB
	events activityAppender
}

// NewRepository constructs a user-settings repository.
func NewRepository(db *sqlx.DB, events activityAppender) *Repository {
	return &Repository{db: db, events: events}
}

// Migrations exposes user-settings migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Get returns persisted settings and reports whether the user has saved them.
func (r *Repository) Get(ctx context.Context, userID string) (Settings, bool, error) {
	var settings Settings
	err := r.db.GetContext(ctx, &settings, `SELECT `+settingsColumns+` FROM user_settings WHERE user_id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, false, fmt.Errorf("select user settings: %w", err)
	}
	return settings, true, nil
}

// Update creates or replaces settings when the observed version is current.
func (r *Repository) Update(ctx context.Context, scope execution.Scope, update Update) (Settings, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Settings{}, fmt.Errorf("begin user settings update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var userID string
	if err := tx.GetContext(ctx, &userID, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, scope.UserID); err != nil {
		return Settings{}, fmt.Errorf("lock settings user: %w", err)
	}

	var current Settings
	err = tx.GetContext(ctx, &current, `SELECT `+settingsColumns+` FROM user_settings WHERE user_id = $1`, scope.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		if update.Version != 0 {
			return Settings{}, ErrVersionConflict
		}
	} else if err != nil {
		return Settings{}, fmt.Errorf("select updated user settings: %w", err)
	} else if current.Version != update.Version {
		return Settings{}, ErrVersionConflict
	}

	var updated Settings
	if current.Version == 0 {
		err = tx.GetContext(ctx, &updated, `
			INSERT INTO user_settings (
				user_id, timezone, agent_model, agent_thinking_effort,
				version, created_at, updated_at, last_modified_by
			) VALUES ($1, $2, $3, $4, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $5)
			RETURNING `+settingsColumns,
			scope.UserID, update.Timezone, update.AgentModel, update.AgentThinkingEffort, scope.ModifiedBy(),
		)
	} else {
		err = tx.GetContext(ctx, &updated, `
			UPDATE user_settings
			SET timezone = $3, agent_model = $4, agent_thinking_effort = $5,
				version = version + 1, updated_at = CURRENT_TIMESTAMP, last_modified_by = $6
			WHERE user_id = $1 AND version = $2
			RETURNING `+settingsColumns,
			scope.UserID, update.Version, update.Timezone, update.AgentModel,
			update.AgentThinkingEffort, scope.ModifiedBy(),
		)
	}
	if err != nil {
		return Settings{}, fmt.Errorf("persist user settings: %w", err)
	}

	aggregateType := "user_settings"
	aggregateID := scope.UserID
	if _, err := r.events.Append(ctx, tx, scope, activity.NewEvent{
		Type: "user_settings.updated", AggregateType: &aggregateType, AggregateID: &aggregateID,
		Payload: map[string]any{
			"schemaVersion": 2, "timezone": updated.Timezone,
			"agentModel": updated.AgentModel, "agentThinkingEffort": updated.AgentThinkingEffort,
			"version": updated.Version,
		},
	}); err != nil {
		return Settings{}, fmt.Errorf("append user settings activity: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Settings{}, fmt.Errorf("commit user settings update: %w", err)
	}
	return updated, nil
}
