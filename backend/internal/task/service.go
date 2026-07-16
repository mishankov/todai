package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const maxTitleLength = 500

const maxDescriptionLength = 10_000

var (
	// ErrTitleRequired indicates that a task title is empty after trimming whitespace.
	ErrTitleRequired = errors.New("task title is required")
	// ErrTitleTooLong indicates that a task title exceeds the supported length.
	ErrTitleTooLong = errors.New("task title is too long")
	// ErrTaskNotFound indicates that the requested task does not belong to the current user.
	ErrTaskNotFound = errors.New("task not found")
	// ErrInvalidVersion indicates that an update does not identify the observed task version.
	ErrInvalidVersion = errors.New("task version must be positive")
	// ErrVersionConflict indicates that the task changed after the caller read it.
	ErrVersionConflict = errors.New("task version conflict")
	// ErrNoChanges indicates that an update contains no editable fields.
	ErrNoChanges = errors.New("task update contains no changes")
	// ErrDescriptionTooLong indicates that a description exceeds the supported length.
	ErrDescriptionTooLong = errors.New("task description is too long")
	// ErrInvalidPriority indicates that priority is outside the supported range.
	ErrInvalidPriority = errors.New("task priority must be between 0 and 4")
	// ErrInvalidTimezone indicates that a due timezone is not an IANA timezone.
	ErrInvalidTimezone = errors.New("task due timezone is invalid")
	// ErrProjectNotFound indicates that a requested destination project is unavailable to the user.
	ErrProjectNotFound = errors.New("task project not found")
)

type repository interface {
	Create(context.Context, string, string, *string) (Task, error)
	Get(context.Context, string, string) (Task, error)
	ListInbox(context.Context, string, bool) ([]Task, error)
	ListProject(context.Context, string, string, bool) ([]Task, error)
	ListToday(context.Context, string, time.Time, time.Time, bool) ([]Task, error)
	Complete(context.Context, string, string) (Task, error)
	Reopen(context.Context, string, string) (Task, error)
	Update(context.Context, string, string, Update) (Task, error)
	Delete(context.Context, string, string) error
}

// Service provides user-scoped task application operations.
type Service struct {
	repository repository
}

// NewService constructs a task service.
func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

// Create creates an active top-level task in the user's Inbox or a project.
func (s *Service) Create(ctx context.Context, userID, title string, projectID *string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrTitleRequired
	}
	if utf8.RuneCountInString(title) > maxTitleLength {
		return Task{}, ErrTitleTooLong
	}

	created, err := s.repository.Create(ctx, userID, title, projectID)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}

	return created, nil
}

// ListProject returns top-level tasks in one project.
func (s *Service) ListProject(
	ctx context.Context,
	userID string,
	projectID string,
	includeCompleted bool,
) ([]Task, error) {
	tasks, err := s.repository.ListProject(ctx, userID, projectID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("list project tasks: %w", err)
	}

	return tasks, nil
}

// Get returns one user-owned task.
func (s *Service) Get(ctx context.Context, userID, taskID string) (Task, error) {
	found, err := s.repository.Get(ctx, userID, taskID)
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}

	return found, nil
}

// ListInbox returns top-level tasks without a project.
func (s *Service) ListInbox(ctx context.Context, userID string, includeCompleted bool) ([]Task, error) {
	tasks, err := s.repository.ListInbox(ctx, userID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("list Inbox: %w", err)
	}

	return tasks, nil
}

// ListToday returns active tasks due before the end of the user's local day and,
// when requested, tasks completed during that day.
func (s *Service) ListToday(
	ctx context.Context,
	userID string,
	timezone string,
	includeCompleted bool,
) ([]Task, error) {
	timezone = strings.TrimSpace(timezone)
	location, err := time.LoadLocation(timezone)
	if timezone == "" || err != nil {
		return nil, ErrInvalidTimezone
	}

	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	tasks, err := s.repository.ListToday(ctx, userID, start, end, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("list Today: %w", err)
	}

	return tasks, nil
}

// Complete marks a user-owned task as completed. Repeated calls are idempotent.
func (s *Service) Complete(ctx context.Context, userID, taskID string) (Task, error) {
	completed, err := s.repository.Complete(ctx, userID, taskID)
	if err != nil {
		return Task{}, fmt.Errorf("complete task: %w", err)
	}

	return completed, nil
}

// Reopen marks a user-owned task as active. Repeated calls are idempotent.
func (s *Service) Reopen(ctx context.Context, userID, taskID string) (Task, error) {
	reopened, err := s.repository.Reopen(ctx, userID, taskID)
	if err != nil {
		return Task{}, fmt.Errorf("reopen task: %w", err)
	}

	return reopened, nil
}

// Update changes editable fields when the caller's version is still current.
func (s *Service) Update(ctx context.Context, userID, taskID string, update Update) (Task, error) {
	if err := validateUpdate(&update); err != nil {
		return Task{}, err
	}

	updated, err := s.repository.Update(ctx, userID, taskID, update)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	return updated, nil
}

// Delete permanently removes a user-owned task.
func (s *Service) Delete(ctx context.Context, userID, taskID string) error {
	if err := s.repository.Delete(ctx, userID, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}

func validateUpdate(update *Update) error {
	if update.Version < 1 {
		return ErrInvalidVersion
	}
	if update.Title == nil && update.Description == nil && update.ProjectID == nil && update.Priority == nil &&
		update.DueAt == nil && update.DueTimezone == nil {
		return ErrNoChanges
	}
	if update.Title != nil {
		title := strings.TrimSpace(*update.Title)
		if title == "" {
			return ErrTitleRequired
		}
		if utf8.RuneCountInString(title) > maxTitleLength {
			return ErrTitleTooLong
		}
		update.Title = &title
	}
	if update.Description != nil && update.Description.Value != nil &&
		utf8.RuneCountInString(*update.Description.Value) > maxDescriptionLength {
		return ErrDescriptionTooLong
	}
	if update.Priority != nil && (*update.Priority < 0 || *update.Priority > 4) {
		return ErrInvalidPriority
	}
	if update.DueTimezone != nil && update.DueTimezone.Value != nil {
		timezone := strings.TrimSpace(*update.DueTimezone.Value)
		if timezone == "" {
			return ErrInvalidTimezone
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			return ErrInvalidTimezone
		}
		update.DueTimezone.Value = &timezone
	}
	if update.DueAt != nil && update.DueAt.Value == nil && update.DueTimezone == nil {
		update.DueTimezone = &Nullable[string]{}
	}

	return nil
}
