package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mishankov/todai/backend/internal/execution"
)

const maxTitleLength = 500

const maxDescriptionLength = 10_000

const maxCommentLength = 10_000

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
	// ErrInvalidDueDate indicates that a due date is not a calendar date.
	ErrInvalidDueDate = errors.New("task due date is invalid")
	// ErrInvalidDueTime indicates that a due time is not a wall-clock time.
	ErrInvalidDueTime = errors.New("task due time is invalid")
	// ErrDueDateRequired indicates that a due time was provided without a due date.
	ErrDueDateRequired = errors.New("task due date is required when due time is set")
	// ErrProjectNotFound indicates that a requested destination project is unavailable to the user.
	ErrProjectNotFound = errors.New("task project not found")
	// ErrProjectRequired indicates that top-level work must belong to a project workspace.
	ErrProjectRequired = errors.New("task project is required")
	// ErrSectionNotFound indicates that a requested project section is unavailable.
	ErrSectionNotFound = errors.New("task project section not found")
	// ErrTaskNotReorderable indicates that a task cannot participate in project ordering.
	ErrTaskNotReorderable = errors.New("only active top-level project tasks can be reordered")
	// ErrInvalidSearchLimit indicates that a search requests an unsafe result count.
	ErrInvalidSearchLimit = errors.New("task search limit must be between 1 and 100")
	// ErrInvalidSearchStatus indicates that a search contains an unsupported task status.
	ErrInvalidSearchStatus = errors.New("task search status is invalid")
	// ErrActiveSubtasks indicates that completing a parent would hide active child work.
	ErrActiveSubtasks = errors.New("task has active subtasks")
	// ErrSubtaskPlacement indicates that a child must inherit placement from its parent.
	ErrSubtaskPlacement = errors.New("subtask project and section are inherited from its parent")
	// ErrParentCompleted indicates that active work cannot be added below a completed parent.
	ErrParentCompleted = errors.New("cannot add a subtask to a completed task")
	// ErrCommentRequired indicates that a comment is empty after trimming whitespace.
	ErrCommentRequired = errors.New("task comment is required")
	// ErrCommentTooLong indicates that a comment exceeds the supported length.
	ErrCommentTooLong = errors.New("task comment is too long")
	// ErrCommentNotFound avoids exposing comments outside the current user's task.
	ErrCommentNotFound = errors.New("task comment not found")
	// ErrCommentVersionConflict indicates that a comment changed after it was read.
	ErrCommentVersionConflict = errors.New("task comment version conflict")
)

type repository interface {
	Create(context.Context, execution.Scope, string, *string, *string, *string) (Task, error)
	Get(context.Context, string, string) (Task, error)
	ListSubtasks(context.Context, string, string) ([]Task, error)
	ListInbox(context.Context, string, string, bool) ([]TaskSummary, error)
	ListAll(context.Context, string, string, bool) ([]TaskSummary, error)
	ListProject(context.Context, string, string, bool) ([]TaskSummary, error)
	ListToday(context.Context, string, string, Date, time.Time, time.Time, bool) ([]TaskSummary, error)
	Search(context.Context, string, SearchQuery) ([]Task, error)
	Complete(context.Context, execution.Scope, string, int64) (Task, error)
	Reopen(context.Context, execution.Scope, string, int64) (Task, error)
	Update(context.Context, execution.Scope, string, Update) (Task, error)
	Reorder(context.Context, execution.Scope, string, Reorder) ([]TaskSummary, error)
	Delete(context.Context, execution.Scope, string, int64) error
	ListComments(context.Context, string, string) ([]Comment, error)
	CreateComment(context.Context, execution.Scope, string, string) (Comment, error)
	UpdateComment(context.Context, execution.Scope, string, string, string, int64) (Comment, error)
	DeleteComment(context.Context, execution.Scope, string, string, int64) error
}

// Search returns user-owned tasks matching a text query and optional filters.
func (s *Service) Search(ctx context.Context, userID string, query SearchQuery) ([]Task, error) {
	query.Query = strings.TrimSpace(query.Query)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > 100 {
		return nil, ErrInvalidSearchLimit
	}
	if query.Status != nil && *query.Status != StatusActive && *query.Status != StatusCompleted {
		return nil, ErrInvalidSearchStatus
	}

	tasks, err := s.repository.Search(ctx, userID, query)
	if err != nil {
		return nil, fmt.Errorf("search tasks: %w", err)
	}

	return tasks, nil
}

// Service provides user-scoped task application operations.
type Service struct {
	repository repository
}

// NewService constructs a task service.
func NewService(repository repository) *Service {
	return &Service{repository: repository}
}

// Create creates an active top-level task in a project's Inbox or section.
func (s *Service) Create(
	ctx context.Context,
	scope execution.Scope,
	title string,
	projectID *string,
	sectionID *string,
) (Task, error) {
	if err := scope.Validate(); err != nil {
		return Task{}, fmt.Errorf("validate execution scope: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrTitleRequired
	}
	if utf8.RuneCountInString(title) > maxTitleLength {
		return Task{}, ErrTitleTooLong
	}

	if projectID == nil || strings.TrimSpace(*projectID) == "" {
		return Task{}, ErrProjectRequired
	}

	created, err := s.repository.Create(ctx, scope, title, projectID, sectionID, nil)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}

	return created, nil
}

// CreateSubtask creates an active direct child inheriting the parent's placement.
func (s *Service) CreateSubtask(
	ctx context.Context,
	scope execution.Scope,
	title string,
	parentID string,
) (Task, error) {
	if strings.TrimSpace(parentID) == "" {
		return Task{}, ErrTaskNotFound
	}
	if err := scope.Validate(); err != nil {
		return Task{}, fmt.Errorf("validate execution scope: %w", err)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrTitleRequired
	}
	if utf8.RuneCountInString(title) > maxTitleLength {
		return Task{}, ErrTitleTooLong
	}
	created, err := s.repository.Create(ctx, scope, title, nil, nil, &parentID)
	if err != nil {
		return Task{}, fmt.Errorf("create subtask: %w", err)
	}
	return created, nil
}

// ListSubtasks returns the direct children of a user-owned task in position order.
func (s *Service) ListSubtasks(ctx context.Context, userID, parentID string) ([]Task, error) {
	tasks, err := s.repository.ListSubtasks(ctx, userID, parentID)
	if err != nil {
		return nil, fmt.Errorf("list subtasks: %w", err)
	}
	return tasks, nil
}

// ListComments returns task comments from oldest to newest.
func (s *Service) ListComments(ctx context.Context, userID, taskID string) ([]Comment, error) {
	comments, err := s.repository.ListComments(ctx, userID, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task comments: %w", err)
	}
	return comments, nil
}

// CreateComment appends a comment to a user-owned task.
func (s *Service) CreateComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	content string,
) (Comment, error) {
	if err := scope.Validate(); err != nil {
		return Comment{}, fmt.Errorf("validate execution scope: %w", err)
	}
	content, err := validateComment(content)
	if err != nil {
		return Comment{}, err
	}
	created, err := s.repository.CreateComment(ctx, scope, taskID, content)
	if err != nil {
		return Comment{}, fmt.Errorf("create task comment: %w", err)
	}
	return created, nil
}

// UpdateComment changes comment content using optimistic concurrency.
func (s *Service) UpdateComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	commentID string,
	content string,
	version int64,
) (Comment, error) {
	if err := validateMutation(scope, version); err != nil {
		return Comment{}, err
	}
	content, err := validateComment(content)
	if err != nil {
		return Comment{}, err
	}
	updated, err := s.repository.UpdateComment(ctx, scope, taskID, commentID, content, version)
	if err != nil {
		return Comment{}, fmt.Errorf("update task comment: %w", err)
	}
	return updated, nil
}

// DeleteComment permanently removes a comment using optimistic concurrency.
func (s *Service) DeleteComment(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	commentID string,
	version int64,
) error {
	if err := validateMutation(scope, version); err != nil {
		return err
	}
	if err := s.repository.DeleteComment(ctx, scope, taskID, commentID, version); err != nil {
		return fmt.Errorf("delete task comment: %w", err)
	}
	return nil
}

func validateComment(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", ErrCommentRequired
	}
	if utf8.RuneCountInString(content) > maxCommentLength {
		return "", ErrCommentTooLong
	}
	return content, nil
}

// ListProject returns top-level tasks in one project.
func (s *Service) ListProject(
	ctx context.Context,
	userID string,
	projectID string,
	includeCompleted bool,
) ([]TaskSummary, error) {
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

// ListInbox returns top-level unsectioned tasks in one project workspace.
func (s *Service) ListInbox(
	ctx context.Context, userID string, projectID string, includeCompleted bool,
) ([]TaskSummary, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrProjectRequired
	}
	tasks, err := s.repository.ListInbox(ctx, userID, projectID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("list Inbox: %w", err)
	}

	return tasks, nil
}

// ListAll returns all top-level tasks in one project workspace.
func (s *Service) ListAll(
	ctx context.Context, userID string, projectID string, includeCompleted bool,
) ([]TaskSummary, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrProjectRequired
	}
	tasks, err := s.repository.ListAll(ctx, userID, projectID, includeCompleted)
	if err != nil {
		return nil, fmt.Errorf("list all tasks: %w", err)
	}

	return tasks, nil
}

// ListToday returns active tasks due on or before the user's local date and,
// when requested, tasks completed during that day.
func (s *Service) ListToday(
	ctx context.Context,
	userID string,
	projectID string,
	timezone string,
	includeCompleted bool,
) ([]TaskSummary, error) {
	timezone = strings.TrimSpace(timezone)
	location, err := time.LoadLocation(timezone)
	if timezone == "" || err != nil {
		return nil, ErrInvalidTimezone
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrProjectRequired
	}

	now := time.Now().In(location)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	tasks, err := s.repository.ListToday(
		ctx, userID, projectID, Date(start.Format(dateLayout)), start, end, includeCompleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list Today: %w", err)
	}

	return tasks, nil
}

// Complete marks a user-owned task as completed. Repeated calls are idempotent.
func (s *Service) Complete(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (Task, error) {
	if err := validateMutation(scope, version); err != nil {
		return Task{}, err
	}
	completed, err := s.repository.Complete(ctx, scope, taskID, version)
	if err != nil {
		return Task{}, fmt.Errorf("complete task: %w", err)
	}

	return completed, nil
}

// Reopen marks a user-owned task as active. Repeated calls are idempotent.
func (s *Service) Reopen(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (Task, error) {
	if err := validateMutation(scope, version); err != nil {
		return Task{}, err
	}
	reopened, err := s.repository.Reopen(ctx, scope, taskID, version)
	if err != nil {
		return Task{}, fmt.Errorf("reopen task: %w", err)
	}

	return reopened, nil
}

// Update changes editable fields when the caller's version is still current.
func (s *Service) Update(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	update Update,
) (Task, error) {
	if err := scope.Validate(); err != nil {
		return Task{}, fmt.Errorf("validate execution scope: %w", err)
	}
	if err := validateUpdate(&update); err != nil {
		return Task{}, err
	}
	if update.ProjectID != nil && update.SectionID == nil {
		update.SectionID = &Nullable[string]{}
	}

	updated, err := s.repository.Update(ctx, scope, taskID, update)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	return updated, nil
}

// Reorder moves an active top-level project task within or between sections.
func (s *Service) Reorder(
	ctx context.Context,
	scope execution.Scope,
	taskID string,
	reorder Reorder,
) ([]TaskSummary, error) {
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("validate execution scope: %w", err)
	}
	if reorder.Version < 1 {
		return nil, ErrInvalidVersion
	}
	tasks, err := s.repository.Reorder(ctx, scope, taskID, reorder)
	if err != nil {
		return nil, fmt.Errorf("reorder task: %w", err)
	}

	return tasks, nil
}

// Delete permanently removes a user-owned task.
func (s *Service) Delete(ctx context.Context, scope execution.Scope, taskID string, version int64) error {
	if err := validateMutation(scope, version); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, scope, taskID, version); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}

func validateMutation(scope execution.Scope, version int64) error {
	if err := scope.Validate(); err != nil {
		return fmt.Errorf("validate execution scope: %w", err)
	}
	if version < 1 {
		return ErrInvalidVersion
	}

	return nil
}

func validateUpdate(update *Update) error {
	if update.Version < 1 {
		return ErrInvalidVersion
	}
	if update.Title == nil && update.Description == nil && update.ProjectID == nil && update.Priority == nil &&
		update.SectionID == nil && update.DueDate == nil && update.DueTime == nil &&
		update.DueTimezone == nil {
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
	if update.ProjectID != nil && (update.ProjectID.Value == nil ||
		strings.TrimSpace(*update.ProjectID.Value) == "") {
		return ErrProjectRequired
	}
	if update.Priority != nil && (*update.Priority < 0 || *update.Priority > 4) {
		return ErrInvalidPriority
	}
	if update.DueDate != nil && update.DueDate.Value != nil {
		if _, err := ParseDate(string(*update.DueDate.Value)); err != nil {
			return ErrInvalidDueDate
		}
	}
	if update.DueTime != nil && update.DueTime.Value != nil {
		if _, err := ParseTimeOfDay(string(*update.DueTime.Value)); err != nil {
			return ErrInvalidDueTime
		}
		if update.DueDate != nil && update.DueDate.Value == nil {
			return ErrDueDateRequired
		}
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
	if update.DueDate != nil && update.DueDate.Value == nil {
		update.DueTime = &Nullable[TimeOfDay]{}
		update.DueTimezone = &Nullable[string]{}
	} else if update.DueTime != nil && update.DueTime.Value == nil {
		update.DueTimezone = &Nullable[string]{}
	}

	return nil
}
