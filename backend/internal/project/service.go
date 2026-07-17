package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxNameLength = 200

var (
	// ErrNameRequired indicates that a project name is empty after trimming whitespace.
	ErrNameRequired = errors.New("project name is required")
	// ErrNameTooLong indicates that a project name exceeds the supported length.
	ErrNameTooLong = errors.New("project name is too long")
	// ErrProjectNotFound indicates that the project does not belong to the current user.
	ErrProjectNotFound = errors.New("project not found")
	// ErrInvalidVersion indicates that an update does not identify the observed version.
	ErrInvalidVersion = errors.New("project version must be positive")
	// ErrVersionConflict indicates that the project changed after the caller read it.
	ErrVersionConflict = errors.New("project version conflict")
	// ErrNoChanges indicates that an update contains no editable fields.
	ErrNoChanges = errors.New("project update contains no changes")
	// ErrInvalidLayout indicates that a project layout is unsupported.
	ErrInvalidLayout = errors.New("project layout must be list or board")
	// ErrSectionNotFound indicates that a section is unavailable in the project.
	ErrSectionNotFound = errors.New("project section not found")
	// ErrSectionNameRequired indicates that a section name is blank.
	ErrSectionNameRequired = errors.New("project section name is required")
	// ErrSectionNameTooLong indicates that a section name exceeds the supported length.
	ErrSectionNameTooLong = errors.New("project section name is too long")
	// ErrSectionNoChanges indicates that a section update contains no editable fields.
	ErrSectionNoChanges = errors.New("project section update contains no changes")
)

type repository interface {
	Create(context.Context, string, string) (Project, error)
	Get(context.Context, string, string) (Project, error)
	List(context.Context, string, bool) ([]Project, error)
	Update(context.Context, string, string, Update) (Project, error)
	CreateSection(context.Context, string, string, string) (Section, error)
	ListSections(context.Context, string, string) ([]Section, error)
	UpdateSection(context.Context, string, string, string, SectionUpdate) (Section, error)
	DeleteSection(context.Context, string, string, string, int64) error
	ReorderSection(context.Context, string, string, string, int64, *string) ([]Section, error)
}

// Service provides user-scoped project application operations.
type Service struct {
	repository repository
}

// NewService constructs a project service.
func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

// Create creates an active project for the user.
func (s *Service) Create(ctx context.Context, userID, name string) (Project, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Project{}, err
	}

	created, err := s.repository.Create(ctx, userID, name)
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}

	return created, nil
}

// Get returns one user-owned project.
func (s *Service) Get(ctx context.Context, userID, projectID string) (Project, error) {
	found, err := s.repository.Get(ctx, userID, projectID)
	if err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}

	return found, nil
}

// List returns the user's projects ordered by position.
func (s *Service) List(ctx context.Context, userID string, includeArchived bool) ([]Project, error) {
	projects, err := s.repository.List(ctx, userID, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	return projects, nil
}

// Update changes editable fields when the caller's version is current.
func (s *Service) Update(ctx context.Context, userID, projectID string, update Update) (Project, error) {
	if update.Version < 1 {
		return Project{}, ErrInvalidVersion
	}
	if update.Name == nil && update.Archived == nil && update.Layout == nil {
		return Project{}, ErrNoChanges
	}
	if update.Name != nil {
		name, err := normalizeName(*update.Name)
		if err != nil {
			return Project{}, err
		}
		update.Name = &name
	}
	if update.Layout != nil && *update.Layout != LayoutList && *update.Layout != LayoutBoard {
		return Project{}, ErrInvalidLayout
	}

	updated, err := s.repository.Update(ctx, userID, projectID, update)
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	return updated, nil
}

// CreateSection creates a section at the end of a project.
func (s *Service) CreateSection(
	ctx context.Context,
	userID string,
	projectID string,
	name string,
) (Section, error) {
	name, err := normalizeSectionName(name)
	if err != nil {
		return Section{}, err
	}

	created, err := s.repository.CreateSection(ctx, userID, projectID, name)
	if err != nil {
		return Section{}, fmt.Errorf("create project section: %w", err)
	}

	return created, nil
}

// ListSections returns a project's sections in manual order.
func (s *Service) ListSections(
	ctx context.Context,
	userID string,
	projectID string,
) ([]Section, error) {
	sections, err := s.repository.ListSections(ctx, userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project sections: %w", err)
	}

	return sections, nil
}

// UpdateSection changes a section name using optimistic concurrency.
func (s *Service) UpdateSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	update SectionUpdate,
) (Section, error) {
	if update.Version < 1 {
		return Section{}, ErrInvalidVersion
	}
	if update.Name == nil {
		return Section{}, ErrSectionNoChanges
	}
	name, err := normalizeSectionName(*update.Name)
	if err != nil {
		return Section{}, err
	}
	update.Name = &name

	updated, err := s.repository.UpdateSection(ctx, userID, projectID, sectionID, update)
	if err != nil {
		return Section{}, fmt.Errorf("update project section: %w", err)
	}

	return updated, nil
}

// DeleteSection removes a section without deleting its tasks.
func (s *Service) DeleteSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	version int64,
) error {
	if version < 1 {
		return ErrInvalidVersion
	}
	if err := s.repository.DeleteSection(ctx, userID, projectID, sectionID, version); err != nil {
		return fmt.Errorf("delete project section: %w", err)
	}

	return nil
}

// ReorderSection moves a section before another section or to the end.
func (s *Service) ReorderSection(
	ctx context.Context,
	userID string,
	projectID string,
	sectionID string,
	version int64,
	beforeSectionID *string,
) ([]Section, error) {
	if version < 1 {
		return nil, ErrInvalidVersion
	}
	sections, err := s.repository.ReorderSection(
		ctx, userID, projectID, sectionID, version, beforeSectionID,
	)
	if err != nil {
		return nil, fmt.Errorf("reorder project section: %w", err)
	}

	return sections, nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNameRequired
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", ErrNameTooLong
	}

	return name, nil
}

func normalizeSectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrSectionNameRequired
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return "", ErrSectionNameTooLong
	}

	return name, nil
}
