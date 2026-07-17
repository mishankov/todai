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
	id, user_id, name, layout, position, version, archived_at, created_at, updated_at,
	last_modified_by
`

const sectionColumns = `
	id, user_id, project_id, name, position, version, created_at, updated_at,
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
			layout = CASE WHEN $8::BOOLEAN THEN $9::VARCHAR ELSE layout END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $2
		WHERE id = $1 AND user_id = $2 AND version = $3
		RETURNING `+projectColumns,
		projectID, userID, update.Version,
		update.Name != nil, pointerValue(update.Name),
		update.Archived != nil, pointerValue(update.Archived),
		update.Layout != nil, pointerValue(update.Layout),
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

// CreateSection appends a section to an active project.
func (r *Repository) CreateSection(
	ctx context.Context,
	userID string,
	projectID string,
	name string,
) (Section, error) {
	var created Section
	err := r.db.GetContext(ctx, &created, `
		INSERT INTO project_sections (
			id, user_id, project_id, name, position, version,
			created_at, updated_at, last_modified_by
		)
		SELECT $1::VARCHAR, $2::VARCHAR, $3::VARCHAR, $4::TEXT,
			COALESCE((
				SELECT MAX(position) FROM project_sections
				WHERE user_id = $2::VARCHAR AND project_id = $3::VARCHAR
			), 0) + 1024,
			1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $2::VARCHAR
		FROM projects
		WHERE id = $3::VARCHAR AND user_id = $2::VARCHAR AND archived_at IS NULL
		RETURNING `+sectionColumns,
		uuid.NewString(), userID, projectID, name,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Section{}, ErrProjectNotFound
	}
	if err != nil {
		return Section{}, fmt.Errorf("insert project section: %w", err)
	}

	return created, nil
}

// ListSections returns a project's sections in their manual order.
func (r *Repository) ListSections(
	ctx context.Context,
	userID string,
	projectID string,
) ([]Section, error) {
	if _, err := r.Get(ctx, userID, projectID); err != nil {
		return nil, err
	}

	sections := make([]Section, 0)
	if err := r.db.SelectContext(ctx, &sections, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE user_id = $1 AND project_id = $2
		ORDER BY position, created_at
	`, userID, projectID); err != nil {
		return nil, fmt.Errorf("select project sections: %w", err)
	}

	return sections, nil
}

// GetSection returns one section scoped to its project and owner.
func (r *Repository) GetSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
) (Section, error) {
	var found Section
	err := r.db.GetContext(ctx, &found, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE id = $1 AND project_id = $2 AND user_id = $3
	`, sectionID, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Section{}, ErrSectionNotFound
	}
	if err != nil {
		return Section{}, fmt.Errorf("select project section: %w", err)
	}

	return found, nil
}

// UpdateSection changes a section name using optimistic concurrency.
func (r *Repository) UpdateSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	update SectionUpdate,
) (Section, error) {
	var updated Section
	err := r.db.GetContext(ctx, &updated, `
		UPDATE project_sections
		SET name = CASE WHEN $5::BOOLEAN THEN $6::TEXT ELSE name END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $2
		WHERE id = $1 AND user_id = $2 AND project_id = $3 AND version = $4
		RETURNING `+sectionColumns,
		sectionID, userID, projectID, update.Version,
		update.Name != nil, pointerValue(update.Name),
	)
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Section{}, fmt.Errorf("update project section: %w", err)
	}
	if _, getErr := r.GetSection(ctx, userID, projectID, sectionID); getErr != nil {
		return Section{}, getErr
	}

	return Section{}, ErrVersionConflict
}

// DeleteSection removes a section and leaves its tasks unsectioned.
func (r *Repository) DeleteSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	version int64,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project section deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int64
	err = tx.GetContext(ctx, &currentVersion, `
		SELECT version
		FROM project_sections
		WHERE id = $1 AND user_id = $2 AND project_id = $3
		FOR UPDATE
	`, sectionID, userID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSectionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock deleted project section: %w", err)
	}
	if currentVersion != version {
		return ErrVersionConflict
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET section_id = NULL,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $1
		WHERE user_id = $1 AND project_id = $2 AND section_id = $3
	`, userID, projectID, sectionID); err != nil {
		return fmt.Errorf("unsection tasks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM project_sections
		WHERE id = $1 AND user_id = $2 AND project_id = $3
	`, sectionID, userID, projectID); err != nil {
		return fmt.Errorf("delete project section: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project section deletion: %w", err)
	}

	return nil
}

// ReorderSection places a section immediately before another section or at the end.
func (r *Repository) ReorderSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	version int64,
	beforeSectionID *string,
) ([]Section, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin section reorder: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var projectExists bool
	if err := tx.GetContext(ctx, &projectExists, `
		SELECT EXISTS (
			SELECT 1 FROM projects
			WHERE id = $1 AND user_id = $2 AND archived_at IS NULL
		)
	`, projectID, userID); err != nil {
		return nil, fmt.Errorf("check section project: %w", err)
	}
	if !projectExists {
		return nil, ErrProjectNotFound
	}

	sections := make([]Section, 0)
	if err := tx.SelectContext(ctx, &sections, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE user_id = $1 AND project_id = $2
		ORDER BY position, created_at
		FOR UPDATE
	`, userID, projectID); err != nil {
		return nil, fmt.Errorf("lock project sections: %w", err)
	}

	movedIndex := sectionIndex(sections, sectionID)
	if movedIndex < 0 {
		return nil, ErrSectionNotFound
	}
	if sections[movedIndex].Version != version {
		return nil, ErrVersionConflict
	}
	moved := sections[movedIndex]
	sections = append(sections[:movedIndex], sections[movedIndex+1:]...)

	insertIndex := len(sections)
	if beforeSectionID != nil {
		insertIndex = sectionIndex(sections, *beforeSectionID)
		if insertIndex < 0 {
			return nil, ErrSectionNotFound
		}
	}
	sections = append(sections, Section{})
	copy(sections[insertIndex+1:], sections[insertIndex:])
	sections[insertIndex] = moved

	for index := range sections {
		position := int64(index+1) * 1024
		if sections[index].Position == position {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE project_sections
			SET position = $1, version = version + 1,
				updated_at = CURRENT_TIMESTAMP, last_modified_by = $2
			WHERE id = $3
		`, position, userID, sections[index].ID); err != nil {
			return nil, fmt.Errorf("reposition project section: %w", err)
		}
	}

	sections = sections[:0]
	if err := tx.SelectContext(ctx, &sections, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE user_id = $1 AND project_id = $2
		ORDER BY position, created_at
	`, userID, projectID); err != nil {
		return nil, fmt.Errorf("select reordered project sections: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit section reorder: %w", err)
	}

	return sections, nil
}

func sectionIndex(sections []Section, sectionID string) int {
	for index := range sections {
		if sections[index].ID == sectionID {
			return index
		}
	}

	return -1
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}
