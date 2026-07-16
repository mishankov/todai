package project

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const projectColumns = `
	id, user_id, name, position, version, archived_at, created_at, updated_at,
	last_modified_by
`

//go:embed migrations/*.sql
var migrations embed.FS

// Repository stores projects in PostgreSQL.
type Repository struct {
	db *sqlx.DB
}

// NewRepository constructs a PostgreSQL project repository.
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Migrations exposes project migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Create inserts an active project after the user's existing projects.
func (r *Repository) Create(ctx context.Context, userID, name string) (Project, error) {
	var created Project
	err := r.db.GetContext(ctx, &created, `
		INSERT INTO projects (
			id, user_id, name, position, version, created_at, updated_at, last_modified_by
		)
		SELECT $1::VARCHAR, $2::VARCHAR, $3::TEXT,
			COALESCE(MAX(position), 0) + 1024, 1,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $2::VARCHAR
		FROM projects
		WHERE user_id = $2::VARCHAR AND archived_at IS NULL
		RETURNING `+projectColumns,
		uuid.NewString(), userID, name,
	)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}

	return created, nil
}

// Get returns one project scoped to its owner.
func (r *Repository) Get(ctx context.Context, userID, projectID string) (Project, error) {
	var found Project
	err := r.db.GetContext(ctx, &found,
		`SELECT `+projectColumns+` FROM projects WHERE id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("select project: %w", err)
	}

	return found, nil
}

// List returns active projects and optionally archived projects.
func (r *Repository) List(ctx context.Context, userID string, includeArchived bool) ([]Project, error) {
	projects := make([]Project, 0)
	err := r.db.SelectContext(ctx, &projects, `
		SELECT `+projectColumns+`
		FROM projects
		WHERE user_id = $1 AND ($2 OR archived_at IS NULL)
		ORDER BY archived_at NULLS FIRST, position, created_at
	`, userID, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("select projects: %w", err)
	}

	return projects, nil
}

// Update changes editable project fields using optimistic concurrency.
func (r *Repository) Update(
	ctx context.Context,
	userID string,
	projectID string,
	update Update,
) (Project, error) {
	var updated Project
	err := r.db.GetContext(ctx, &updated, `
		UPDATE projects
		SET name = CASE WHEN $4::BOOLEAN THEN $5::TEXT ELSE name END,
			archived_at = CASE
				WHEN NOT $6::BOOLEAN THEN archived_at
				WHEN $7::BOOLEAN THEN COALESCE(archived_at, CURRENT_TIMESTAMP)
				ELSE NULL
			END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $2
		WHERE id = $1 AND user_id = $2 AND version = $3
		RETURNING `+projectColumns,
		projectID, userID, update.Version,
		update.Name != nil, pointerValue(update.Name),
		update.Archived != nil, pointerValue(update.Archived),
	)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	if _, getErr := r.Get(ctx, userID, projectID); getErr != nil {
		return Project{}, getErr
	}

	return Project{}, ErrVersionConflict
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}
