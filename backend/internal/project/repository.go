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

	"github.com/mishankov/todai/backend/internal/activity"
	"github.com/mishankov/todai/backend/internal/execution"
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
	db     *sqlx.DB
	events activityAppender
}

type activityAppender interface {
	Append(
		context.Context,
		sqlx.ExtContext,
		execution.Scope,
		activity.NewEvent,
	) (activity.Event, error)
}

// NewRepository constructs a PostgreSQL project repository.
func NewRepository(db *sqlx.DB, events activityAppender) *Repository {
	return &Repository{db: db, events: events}
}

// Migrations exposes project migrations to Platforma.
func (r *Repository) Migrations() fs.FS {
	migrationsFS, _ := fs.Sub(migrations, "migrations")
	return migrationsFS
}

// Create inserts an active project after the user's existing projects.
func (r *Repository) Create(ctx context.Context, scope execution.Scope, name string) (Project, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin project creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var created Project
	err = tx.GetContext(ctx, &created, `
		INSERT INTO projects (
			id, user_id, name, position, version, created_at, updated_at, last_modified_by
		)
		SELECT $1::VARCHAR, $2::VARCHAR, $3::TEXT,
			COALESCE(MAX(position), 0) + 1024, 1,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $4::VARCHAR
		FROM projects
		WHERE user_id = $2::VARCHAR AND archived_at IS NULL
		RETURNING `+projectColumns,
		uuid.NewString(), scope.UserID, name, scope.ModifiedBy(),
	)
	if err != nil {
		return Project{}, fmt.Errorf("insert project: %w", err)
	}

	if err := r.appendProjectEvent(ctx, tx, scope, "project.created", created, map[string]any{
		"schemaVersion": 1,
		"name":          created.Name,
		"layout":        created.Layout,
		"version":       created.Version,
	}); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit project creation: %w", err)
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
	scope execution.Scope,
	projectID string,
	update Update,
) (Project, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Project{}, fmt.Errorf("begin project update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current Project
	err = tx.GetContext(ctx, &current, `
		SELECT `+projectColumns+`
		FROM projects
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, projectID, scope.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("lock updated project: %w", err)
	}
	if current.Version != update.Version {
		return Project{}, ErrVersionConflict
	}

	var updated Project
	err = tx.GetContext(ctx, &updated, `
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
			last_modified_by = $10
		WHERE id = $1 AND user_id = $2 AND version = $3
		RETURNING `+projectColumns,
		projectID, scope.UserID, update.Version,
		update.Name != nil, pointerValue(update.Name),
		update.Archived != nil, pointerValue(update.Archived),
		update.Layout != nil, pointerValue(update.Layout),
		scope.ModifiedBy(),
	)
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	eventType := "project.updated"
	if current.ArchivedAt == nil && updated.ArchivedAt != nil {
		eventType = "project.archived"
	}
	if err := r.appendProjectEvent(ctx, tx, scope, eventType, updated, map[string]any{
		"schemaVersion": 1,
		"beforeVersion": current.Version,
		"version":       updated.Version,
		"name":          updated.Name,
		"layout":        updated.Layout,
		"archived":      updated.ArchivedAt != nil,
	}); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, fmt.Errorf("commit project update: %w", err)
	}

	return updated, nil
}

// CreateSection appends a section to an active project.
func (r *Repository) CreateSection(
	ctx context.Context,
	scope execution.Scope,
	projectID string,
	name string,
) (Section, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Section{}, fmt.Errorf("begin project section creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var created Section
	err = tx.GetContext(ctx, &created, `
		INSERT INTO project_sections (
			id, user_id, project_id, name, position, version,
			created_at, updated_at, last_modified_by
		)
		SELECT $1::VARCHAR, $2::VARCHAR, $3::VARCHAR, $4::TEXT,
			COALESCE((
				SELECT MAX(position) FROM project_sections
				WHERE user_id = $2::VARCHAR AND project_id = $3::VARCHAR
			), 0) + 1024,
			1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $5::VARCHAR
		FROM projects
		WHERE id = $3::VARCHAR AND user_id = $2::VARCHAR AND archived_at IS NULL
		RETURNING `+sectionColumns,
		uuid.NewString(), scope.UserID, projectID, name, scope.ModifiedBy(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Section{}, ErrProjectNotFound
	}
	if err != nil {
		return Section{}, fmt.Errorf("insert project section: %w", err)
	}

	if err := r.appendSectionEvent(ctx, tx, scope, "section.created", created, map[string]any{
		"schemaVersion": 1,
		"projectId":     created.ProjectID,
		"name":          created.Name,
		"position":      created.Position,
		"version":       created.Version,
	}); err != nil {
		return Section{}, err
	}
	if err := tx.Commit(); err != nil {
		return Section{}, fmt.Errorf("commit project section creation: %w", err)
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
	scope execution.Scope,
	projectID string,
	sectionID string,
	update SectionUpdate,
) (Section, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Section{}, fmt.Errorf("begin project section update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current Section
	err = tx.GetContext(ctx, &current, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE id = $1 AND user_id = $2 AND project_id = $3
		FOR UPDATE
	`, sectionID, scope.UserID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return Section{}, ErrSectionNotFound
	}
	if err != nil {
		return Section{}, fmt.Errorf("lock updated project section: %w", err)
	}
	if current.Version != update.Version {
		return Section{}, ErrVersionConflict
	}

	var updated Section
	err = tx.GetContext(ctx, &updated, `
		UPDATE project_sections
		SET name = CASE WHEN $5::BOOLEAN THEN $6::TEXT ELSE name END,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $7
		WHERE id = $1 AND user_id = $2 AND project_id = $3 AND version = $4
		RETURNING `+sectionColumns,
		sectionID, scope.UserID, projectID, update.Version,
		update.Name != nil, pointerValue(update.Name),
		scope.ModifiedBy(),
	)
	if err != nil {
		return Section{}, fmt.Errorf("update project section: %w", err)
	}
	if err := r.appendSectionEvent(ctx, tx, scope, "section.updated", updated, map[string]any{
		"schemaVersion": 1,
		"projectId":     updated.ProjectID,
		"beforeVersion": current.Version,
		"version":       updated.Version,
		"beforeName":    current.Name,
		"name":          updated.Name,
	}); err != nil {
		return Section{}, err
	}
	if err := tx.Commit(); err != nil {
		return Section{}, fmt.Errorf("commit project section update: %w", err)
	}

	return updated, nil
}

// DeleteSection removes a section and leaves its tasks unsectioned.
func (r *Repository) DeleteSection(
	ctx context.Context,
	scope execution.Scope,
	projectID string,
	sectionID string,
	version int64,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project section deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current Section
	err = tx.GetContext(ctx, &current, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE id = $1 AND user_id = $2 AND project_id = $3
		FOR UPDATE
	`, sectionID, scope.UserID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSectionNotFound
	}
	if err != nil {
		return fmt.Errorf("lock deleted project section: %w", err)
	}
	if current.Version != version {
		return ErrVersionConflict
	}

	unsectioned, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET section_id = NULL,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP,
			last_modified_by = $4
		WHERE user_id = $1 AND project_id = $2 AND section_id = $3
	`, scope.UserID, projectID, sectionID, scope.ModifiedBy())
	if err != nil {
		return fmt.Errorf("unsection tasks: %w", err)
	}
	affectedTaskCount, err := unsectioned.RowsAffected()
	if err != nil {
		return fmt.Errorf("get unsectioned task count: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM project_sections
		WHERE id = $1 AND user_id = $2 AND project_id = $3
	`, sectionID, scope.UserID, projectID); err != nil {
		return fmt.Errorf("delete project section: %w", err)
	}
	if err := r.appendSectionEvent(ctx, tx, scope, "section.deleted", current, map[string]any{
		"schemaVersion":     1,
		"projectId":         current.ProjectID,
		"name":              current.Name,
		"position":          current.Position,
		"version":           current.Version,
		"affectedTaskCount": affectedTaskCount,
	}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit project section deletion: %w", err)
	}

	return nil
}

// ReorderSection places a section immediately before another section or at the end.
func (r *Repository) ReorderSection(
	ctx context.Context,
	scope execution.Scope,
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
	`, projectID, scope.UserID); err != nil {
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
	`, scope.UserID, projectID); err != nil {
		return nil, fmt.Errorf("lock project sections: %w", err)
	}

	movedIndex := sectionIndex(sections, sectionID)
	if movedIndex < 0 {
		return nil, ErrSectionNotFound
	}
	if sections[movedIndex].Version != version {
		return nil, ErrVersionConflict
	}
	if beforeSectionID != nil && *beforeSectionID == sectionID {
		return sections, nil
	}
	if beforeSectionID == nil && movedIndex == len(sections)-1 {
		return sections, nil
	}
	if beforeSectionID != nil && movedIndex+1 < len(sections) &&
		sections[movedIndex+1].ID == *beforeSectionID {
		return sections, nil
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
		`, position, scope.ModifiedBy(), sections[index].ID); err != nil {
			return nil, fmt.Errorf("reposition project section: %w", err)
		}
	}

	sections = sections[:0]
	if err := tx.SelectContext(ctx, &sections, `
		SELECT `+sectionColumns+`
		FROM project_sections
		WHERE user_id = $1 AND project_id = $2
		ORDER BY position, created_at
	`, scope.UserID, projectID); err != nil {
		return nil, fmt.Errorf("select reordered project sections: %w", err)
	}
	updatedIndex := sectionIndex(sections, sectionID)
	if updatedIndex < 0 {
		return nil, ErrSectionNotFound
	}
	updated := sections[updatedIndex]
	if err := r.appendSectionEvent(ctx, tx, scope, "section.reordered", updated, map[string]any{
		"schemaVersion":   1,
		"projectId":       projectID,
		"name":            updated.Name,
		"beforePosition":  moved.Position,
		"position":        updated.Position,
		"beforeSectionId": beforeSectionID,
		"version":         updated.Version,
	}); err != nil {
		return nil, err
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

func (r *Repository) appendProjectEvent(
	ctx context.Context,
	executor sqlx.ExtContext,
	scope execution.Scope,
	eventType string,
	project Project,
	payload any,
) error {
	aggregateType := "project"
	if _, err := r.events.Append(ctx, executor, scope, activity.NewEvent{
		Type:          eventType,
		AggregateType: &aggregateType,
		AggregateID:   &project.ID,
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("append project activity event: %w", err)
	}

	return nil
}

func (r *Repository) appendSectionEvent(
	ctx context.Context,
	executor sqlx.ExtContext,
	scope execution.Scope,
	eventType string,
	section Section,
	payload any,
) error {
	aggregateType := "section"
	if _, err := r.events.Append(ctx, executor, scope, activity.NewEvent{
		Type:          eventType,
		AggregateType: &aggregateType,
		AggregateID:   &section.ID,
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("append project section activity event: %w", err)
	}

	return nil
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}
