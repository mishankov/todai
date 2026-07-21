package task_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/task"
)

func TestCreateNormalizesTitle(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)

	created, err := service.Create(context.Background(), testScope(), "  Buy milk  ", nil, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.Title != "Buy milk" {
		t.Errorf("title = %q, want %q", created.Title, "Buy milk")
	}
	if repository.createScope.UserID != "user-id" {
		t.Errorf("user ID = %q, want %q", repository.createScope.UserID, "user-id")
	}
}

func TestCreateRejectsInvalidTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		want  error
	}{
		{name: "empty", title: " \t\n ", want: task.ErrTitleRequired},
		{name: "too long", title: strings.Repeat("я", 501), want: task.ErrTitleTooLong},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := task.NewService(&fakeRepository{})
			_, err := service.Create(context.Background(), testScope(), test.title, nil, nil)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCreateCanTargetProject(t *testing.T) {
	t.Parallel()

	projectID := "project-id"
	repository := &fakeRepository{}
	created, err := task.NewService(repository).Create(
		context.Background(), testScope(), "Plan sprint", &projectID, nil,
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.ProjectID == nil || *created.ProjectID != projectID {
		t.Errorf("created project ID = %#v, want %q", created.ProjectID, projectID)
	}
}

func TestCreateCanTargetProjectSection(t *testing.T) {
	t.Parallel()

	projectID := "project-id"
	sectionID := "section-id"
	repository := &fakeRepository{}
	created, err := task.NewService(repository).Create(
		context.Background(), testScope(), "Plan sprint", &projectID, &sectionID,
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.SectionID == nil || *created.SectionID != sectionID ||
		repository.createSectionID == nil || *repository.createSectionID != sectionID {
		t.Errorf("created task = %#v", created)
	}
}

func TestCreateRejectsSectionWithoutProject(t *testing.T) {
	t.Parallel()

	sectionID := "section-id"
	_, err := task.NewService(&fakeRepository{}).Create(
		context.Background(), testScope(), "Plan sprint", nil, &sectionID,
	)
	if !errors.Is(err, task.ErrSectionNotFound) {
		t.Fatalf("error = %v, want %v", err, task.ErrSectionNotFound)
	}
}

func TestCreateSubtaskNormalizesTitleAndPassesParent(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	created, err := task.NewService(repository).CreateSubtask(
		context.Background(), testScope(), "  Child task  ", "parent-id",
	)
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if created.Title != "Child task" || repository.createParentID == nil ||
		*repository.createParentID != "parent-id" {
		t.Errorf("created/repository = %#v / %#v", created, repository.createParentID)
	}
}

func TestCommentsNormalizeBodyAndRequireVersion(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)
	if _, err := service.CreateComment(
		context.Background(), testScope(), "task-id", "  A note  ",
	); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if repository.commentBody != "A note" {
		t.Errorf("comment body = %q, want %q", repository.commentBody, "A note")
	}
	if _, err := service.UpdateComment(
		context.Background(), testScope(), "task-id", "comment-id", "Edit", 0,
	); !errors.Is(err, task.ErrInvalidVersion) {
		t.Errorf("invalid version error = %v", err)
	}
	if _, err := service.CreateComment(
		context.Background(), testScope(), "task-id", "   ",
	); !errors.Is(err, task.ErrCommentRequired) {
		t.Errorf("blank comment error = %v", err)
	}
	if _, err := service.CreateComment(
		context.Background(), testScope(), "task-id", strings.Repeat("x", 10_001),
	); !errors.Is(err, task.ErrCommentTooLong) {
		t.Errorf("long comment error = %v", err)
	}
}

func TestDeletePassesExecutionScopeAndVersion(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)

	if err := service.Delete(context.Background(), testScope(), "task-id", 7); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if repository.deleteScope.UserID != "user-id" || repository.deleteTaskID != "task-id" ||
		repository.deleteVersion != 7 {
		t.Errorf(
			"delete arguments = (%q, %q, %d), want (%q, %q, %d)",
			repository.deleteScope.UserID,
			repository.deleteTaskID,
			repository.deleteVersion,
			"user-id",
			"task-id",
			7,
		)
	}
}

func TestStatusChangesPassExecutionScopeAndVersion(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)
	if _, err := service.Complete(context.Background(), testScope(), "task-id", 3); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if repository.completeScope.UserID != "user-id" || repository.completeTaskID != "task-id" ||
		repository.completeVersion != 3 {
		t.Errorf("complete arguments = (%#v, %q, %d)", repository.completeScope, repository.completeTaskID, repository.completeVersion)
	}

	if _, err := service.Reopen(context.Background(), testScope(), "task-id", 4); err != nil {
		t.Fatalf("reopen task: %v", err)
	}
	if repository.reopenScope.UserID != "user-id" || repository.reopenTaskID != "task-id" ||
		repository.reopenVersion != 4 {
		t.Errorf("reopen arguments = (%#v, %q, %d)", repository.reopenScope, repository.reopenTaskID, repository.reopenVersion)
	}
}

func TestStatusChangesAndDeleteRejectInvalidVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*task.Service) error
	}{
		{
			name: "complete",
			call: func(service *task.Service) error {
				_, err := service.Complete(context.Background(), testScope(), "task-id", 0)
				return err
			},
		},
		{
			name: "reopen",
			call: func(service *task.Service) error {
				_, err := service.Reopen(context.Background(), testScope(), "task-id", 0)
				return err
			},
		},
		{
			name: "delete",
			call: func(service *task.Service) error {
				return service.Delete(context.Background(), testScope(), "task-id", 0)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := &fakeRepository{}
			if err := test.call(task.NewService(repository)); !errors.Is(err, task.ErrInvalidVersion) {
				t.Fatalf("error = %v, want %v", err, task.ErrInvalidVersion)
			}
			if repository.mutationCalled {
				t.Error("repository mutation was called")
			}
		})
	}
}

func TestListAllScopesTasksToUser(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	_, err := task.NewService(repository).ListAll(context.Background(), "user-id", true)
	if err != nil {
		t.Fatalf("list all tasks: %v", err)
	}
	if repository.allUserID != "user-id" || !repository.allIncludeCompleted {
		t.Errorf(
			"all tasks arguments = (%q, %t), want (%q, %t)",
			repository.allUserID,
			repository.allIncludeCompleted,
			"user-id",
			true,
		)
	}
}

func TestSearchNormalizesQueryAndDefaultLimit(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	_, err := task.NewService(repository).Search(
		context.Background(), "user-id", task.SearchQuery{Query: "  milk  "},
	)
	if err != nil {
		t.Fatalf("search tasks: %v", err)
	}
	if repository.searchUserID != "user-id" || repository.searchQuery.Query != "milk" ||
		repository.searchQuery.Limit != 50 {
		t.Errorf("search arguments = (%q, %#v)", repository.searchUserID, repository.searchQuery)
	}
}

func TestSearchRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	invalidStatus := task.Status("waiting")
	tests := []struct {
		name  string
		query task.SearchQuery
		want  error
	}{
		{name: "limit", query: task.SearchQuery{Limit: 101}, want: task.ErrInvalidSearchLimit},
		{name: "status", query: task.SearchQuery{Status: &invalidStatus}, want: task.ErrInvalidSearchStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := task.NewService(&fakeRepository{}).Search(
				context.Background(), "user-id", test.query,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestListTodayUsesUserLocalDay(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)

	_, err := service.ListToday(context.Background(), "user-id", "Europe/Moscow", true)
	if err != nil {
		t.Fatalf("list Today: %v", err)
	}
	location, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	start := repository.todayStart.In(location)
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 || start.Nanosecond() != 0 {
		t.Errorf("day start = %v, want local midnight", start)
	}
	if !repository.todayEnd.Equal(repository.todayStart.AddDate(0, 0, 1)) {
		t.Errorf("day end = %v, want day after %v", repository.todayEnd, repository.todayStart)
	}
	if repository.todayUserID != "user-id" || !repository.todayIncludeCompleted {
		t.Errorf(
			"Today arguments = (%q, %t), want (%q, %t)",
			repository.todayUserID,
			repository.todayIncludeCompleted,
			"user-id",
			true,
		)
	}
	if repository.todayDate != task.Date(repository.todayStart.In(location).Format("2006-01-02")) {
		t.Errorf("Today date = %q, want local date", repository.todayDate)
	}
}

func TestListTodayRejectsInvalidTimezone(t *testing.T) {
	t.Parallel()

	_, err := task.NewService(&fakeRepository{}).ListToday(
		context.Background(),
		"user-id",
		"Mars/Olympus_Mons",
		false,
	)
	if !errors.Is(err, task.ErrInvalidTimezone) {
		t.Fatalf("error = %v, want %v", err, task.ErrInvalidTimezone)
	}
}

func TestUpdateNormalizesEditableFields(t *testing.T) {
	t.Parallel()

	title := "  Updated task  "
	priority := 3
	repository := &fakeRepository{}
	service := task.NewService(repository)

	_, err := service.Update(context.Background(), testScope(), "task-id", task.Update{
		Version:  2,
		Title:    &title,
		Priority: &priority,
		DueDate:  &task.Nullable[task.Date]{},
	})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if repository.update.Title == nil || *repository.update.Title != "Updated task" {
		t.Errorf("normalized title = %#v", repository.update.Title)
	}
	if repository.update.DueTimezone == nil || repository.update.DueTimezone.Value != nil {
		t.Errorf("due timezone was not cleared with due date: %#v", repository.update.DueTimezone)
	}
	if repository.update.DueTime == nil || repository.update.DueTime.Value != nil {
		t.Errorf("due time was not cleared with due date: %#v", repository.update.DueTime)
	}
	if repository.updateScope.UserID != "user-id" {
		t.Errorf("update scope = %#v", repository.updateScope)
	}
}

func TestUpdateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	blankTitle := "  "
	longDescription := strings.Repeat("x", 10_001)
	invalidPriority := 5
	invalidTimezone := "Mars/Olympus_Mons"
	invalidDate := task.Date("2026-02-30")
	invalidTime := task.TimeOfDay("25:00")
	validTime := task.TimeOfDay("09:30")
	validTitle := "Title"
	tests := []struct {
		name   string
		update task.Update
		want   error
	}{
		{name: "version", update: task.Update{Title: &validTitle}, want: task.ErrInvalidVersion},
		{name: "no changes", update: task.Update{Version: 1}, want: task.ErrNoChanges},
		{name: "title", update: task.Update{Version: 1, Title: &blankTitle}, want: task.ErrTitleRequired},
		{
			name: "description",
			update: task.Update{
				Version:     1,
				Description: &task.Nullable[string]{Value: &longDescription},
			},
			want: task.ErrDescriptionTooLong,
		},
		{
			name:   "priority",
			update: task.Update{Version: 1, Priority: &invalidPriority},
			want:   task.ErrInvalidPriority,
		},
		{
			name: "due date",
			update: task.Update{
				Version: 1,
				DueDate: &task.Nullable[task.Date]{Value: &invalidDate},
			},
			want: task.ErrInvalidDueDate,
		},
		{
			name: "due time",
			update: task.Update{
				Version: 1,
				DueTime: &task.Nullable[task.TimeOfDay]{Value: &invalidTime},
			},
			want: task.ErrInvalidDueTime,
		},
		{
			name: "time without date",
			update: task.Update{
				Version: 1,
				DueDate: &task.Nullable[task.Date]{},
				DueTime: &task.Nullable[task.TimeOfDay]{Value: &validTime},
			},
			want: task.ErrDueDateRequired,
		},
		{
			name: "timezone",
			update: task.Update{
				Version:     1,
				DueTimezone: &task.Nullable[string]{Value: &invalidTimezone},
			},
			want: task.ErrInvalidTimezone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := task.NewService(&fakeRepository{}).Update(
				context.Background(),
				testScope(),
				"task-id",
				test.update,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUpdateClearsSectionWhenProjectChanges(t *testing.T) {
	t.Parallel()

	projectID := "another-project"
	repository := &fakeRepository{}
	_, err := task.NewService(repository).Update(
		context.Background(), testScope(), "task-id",
		task.Update{
			Version:   2,
			ProjectID: &task.Nullable[string]{Value: &projectID},
		},
	)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if repository.update.SectionID == nil || repository.update.SectionID.Value != nil {
		t.Errorf("section update = %#v, want explicit clear", repository.update.SectionID)
	}
}

func TestReorderValidatesAndPreservesVersion(t *testing.T) {
	t.Parallel()

	sectionID := "section-id"
	repository := &fakeRepository{}
	service := task.NewService(repository)
	_, err := service.Reorder(
		context.Background(), testScope(), "task-id",
		task.Reorder{SectionID: &sectionID},
	)
	if !errors.Is(err, task.ErrInvalidVersion) {
		t.Fatalf("error = %v, want %v", err, task.ErrInvalidVersion)
	}

	_, err = service.Reorder(
		context.Background(), testScope(), "task-id",
		task.Reorder{Version: 7, SectionID: &sectionID},
	)
	if err != nil {
		t.Fatalf("reorder task: %v", err)
	}
	if repository.reorder.Version != 7 || repository.reorder.SectionID == nil ||
		*repository.reorder.SectionID != sectionID {
		t.Errorf("reorder = %#v", repository.reorder)
	}
	if repository.reorderScope.UserID != "user-id" {
		t.Errorf("reorder scope = %#v", repository.reorderScope)
	}
}

type fakeRepository struct {
	createScope           execution.Scope
	createProjectID       *string
	createSectionID       *string
	createParentID        *string
	completeScope         execution.Scope
	completeTaskID        string
	completeVersion       int64
	reopenScope           execution.Scope
	reopenTaskID          string
	reopenVersion         int64
	deleteScope           execution.Scope
	deleteTaskID          string
	deleteVersion         int64
	allUserID             string
	allIncludeCompleted   bool
	todayUserID           string
	todayDate             task.Date
	todayStart            time.Time
	todayEnd              time.Time
	todayIncludeCompleted bool
	searchUserID          string
	searchQuery           task.SearchQuery
	updateScope           execution.Scope
	update                task.Update
	reorderScope          execution.Scope
	reorder               task.Reorder
	mutationCalled        bool
	commentBody           string
}

func (r *fakeRepository) Create(
	_ context.Context,
	scope execution.Scope,
	title string,
	projectID *string,
	sectionID *string,
	parentID *string,
) (task.Task, error) {
	r.createScope = scope
	r.createProjectID = projectID
	r.createSectionID = sectionID
	r.createParentID = parentID
	return task.Task{
		ID: "task-id", UserID: scope.UserID, ProjectID: projectID, SectionID: sectionID, ParentID: parentID,
		Title: title, Status: task.StatusActive,
	}, nil
}

func (*fakeRepository) ListSubtasks(context.Context, string, string) ([]task.Task, error) {
	return nil, nil
}

func (*fakeRepository) ListComments(context.Context, string, string) ([]task.Comment, error) {
	return nil, nil
}

func (r *fakeRepository) CreateComment(
	_ context.Context, _ execution.Scope, _ string, body string,
) (task.Comment, error) {
	r.commentBody = body
	return task.Comment{}, nil
}

func (*fakeRepository) UpdateComment(
	context.Context, execution.Scope, string, string, string, int64,
) (task.Comment, error) {
	return task.Comment{}, nil
}

func (*fakeRepository) DeleteComment(
	context.Context, execution.Scope, string, string, int64,
) error {
	return nil
}

func (*fakeRepository) Get(context.Context, string, string) (task.Task, error) {
	return task.Task{}, nil
}

func (*fakeRepository) ListInbox(context.Context, string, bool) ([]task.TaskSummary, error) {
	return nil, nil
}

func (r *fakeRepository) ListAll(
	_ context.Context,
	userID string,
	includeCompleted bool,
) ([]task.TaskSummary, error) {
	r.allUserID = userID
	r.allIncludeCompleted = includeCompleted
	return nil, nil
}

func (*fakeRepository) ListProject(
	context.Context, string, string, bool,
) ([]task.TaskSummary, error) {
	return nil, nil
}

func (r *fakeRepository) ListToday(
	_ context.Context,
	userID string,
	date task.Date,
	dayStart time.Time,
	dayEnd time.Time,
	includeCompleted bool,
) ([]task.TaskSummary, error) {
	r.todayUserID = userID
	r.todayDate = date
	r.todayStart = dayStart
	r.todayEnd = dayEnd
	r.todayIncludeCompleted = includeCompleted
	return nil, nil
}

func (r *fakeRepository) Search(
	_ context.Context,
	userID string,
	query task.SearchQuery,
) ([]task.Task, error) {
	r.searchUserID = userID
	r.searchQuery = query
	return nil, nil
}

func (r *fakeRepository) Complete(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (task.Task, error) {
	r.mutationCalled = true
	r.completeScope = scope
	r.completeTaskID = taskID
	r.completeVersion = version
	return task.Task{}, nil
}

func (r *fakeRepository) Reopen(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (task.Task, error) {
	r.mutationCalled = true
	r.reopenScope = scope
	r.reopenTaskID = taskID
	r.reopenVersion = version
	return task.Task{}, nil
}

func (r *fakeRepository) Update(
	_ context.Context,
	scope execution.Scope,
	_ string,
	update task.Update,
) (task.Task, error) {
	r.mutationCalled = true
	r.updateScope = scope
	r.update = update
	return task.Task{}, nil
}

func (r *fakeRepository) Reorder(
	_ context.Context,
	scope execution.Scope,
	_ string,
	reorder task.Reorder,
) ([]task.TaskSummary, error) {
	r.mutationCalled = true
	r.reorderScope = scope
	r.reorder = reorder
	return nil, nil
}

func (r *fakeRepository) Delete(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) error {
	r.mutationCalled = true
	r.deleteScope = scope
	r.deleteTaskID = taskID
	r.deleteVersion = version
	return nil
}

func testScope() execution.Scope {
	return execution.UserScope("user-id", "correlation-id")
}
