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
)

type repository interface {
	Create(context.Context, string, string) (Project, error)
	Get(context.Context, string, string) (Project, error)
	List(context.Context, string, bool) ([]Project, error)
	Update(context.Context, string, string, Update) (Project, error)
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
	if update.Name == nil && update.Archived == nil {
		return Project{}, ErrNoChanges
	}
	if update.Name != nil {
		name, err := normalizeName(*update.Name)
		if err != nil {
			return Project{}, err
		}
		update.Name = &name
	}

	updated, err := s.repository.Update(ctx, userID, projectID, update)
	if err != nil {
		return Project{}, fmt.Errorf("update project: %w", err)
	}

	return updated, nil
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
