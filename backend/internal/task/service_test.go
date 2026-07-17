package task_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishankov/todai/backend/internal/task"
)

func TestCreateNormalizesTitle(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)

	created, err := service.Create(context.Background(), "user-id", "  Buy milk  ", nil, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if created.Title != "Buy milk" {
		t.Errorf("title = %q, want %q", created.Title, "Buy milk")
	}
	if repository.createUserID != "user-id" {
		t.Errorf("user ID = %q, want %q", repository.createUserID, "user-id")
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
			_, err := service.Create(context.Background(), "user-id", test.title, nil, nil)
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
		context.Background(), "user-id", "Plan sprint", &projectID, nil,
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
		context.Background(), "user-id", "Plan sprint", &projectID, &sectionID,
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
		context.Background(), "user-id", "Plan sprint", nil, &sectionID,
	)
	if !errors.Is(err, task.ErrSectionNotFound) {
		t.Fatalf("error = %v, want %v", err, task.ErrSectionNotFound)
	}
}

func TestDeleteScopesTaskToUser(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	service := task.NewService(repository)

	if err := service.Delete(context.Background(), "user-id", "task-id"); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if repository.deleteUserID != "user-id" || repository.deleteTaskID != "task-id" {
		t.Errorf(
			"delete arguments = (%q, %q), want (%q, %q)",
			repository.deleteUserID,
			repository.deleteTaskID,
			"user-id",
			"task-id",
		)
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

	_, err := service.Update(context.Background(), "user-id", "task-id", task.Update{
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
				"user-id",
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
		context.Background(), "user-id", "task-id",
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
		context.Background(), "user-id", "task-id",
		task.Reorder{SectionID: &sectionID},
	)
	if !errors.Is(err, task.ErrInvalidVersion) {
		t.Fatalf("error = %v, want %v", err, task.ErrInvalidVersion)
	}

	_, err = service.Reorder(
		context.Background(), "user-id", "task-id",
		task.Reorder{Version: 7, SectionID: &sectionID},
	)
	if err != nil {
		t.Fatalf("reorder task: %v", err)
	}
	if repository.reorder.Version != 7 || repository.reorder.SectionID == nil ||
		*repository.reorder.SectionID != sectionID {
		t.Errorf("reorder = %#v", repository.reorder)
	}
}

type fakeRepository struct {
	createUserID          string
	createProjectID       *string
	createSectionID       *string
	deleteUserID          string
	deleteTaskID          string
	allUserID             string
	allIncludeCompleted   bool
	todayUserID           string
	todayDate             task.Date
	todayStart            time.Time
	todayEnd              time.Time
	todayIncludeCompleted bool
	update                task.Update
	reorder               task.Reorder
}

func (r *fakeRepository) Create(
	_ context.Context,
	userID string,
	title string,
	projectID *string,
	sectionID *string,
) (task.Task, error) {
	r.createUserID = userID
	r.createProjectID = projectID
	r.createSectionID = sectionID
	return task.Task{
		ID: "task-id", UserID: userID, ProjectID: projectID, SectionID: sectionID,
		Title: title, Status: task.StatusActive,
	}, nil
}

func (*fakeRepository) Get(context.Context, string, string) (task.Task, error) {
	return task.Task{}, nil
}

func (*fakeRepository) ListInbox(context.Context, string, bool) ([]task.Task, error) {
	return nil, nil
}

func (r *fakeRepository) ListAll(
	_ context.Context,
	userID string,
	includeCompleted bool,
) ([]task.Task, error) {
	r.allUserID = userID
	r.allIncludeCompleted = includeCompleted
	return nil, nil
}

func (*fakeRepository) ListProject(context.Context, string, string, bool) ([]task.Task, error) {
	return nil, nil
}

func (r *fakeRepository) ListToday(
	_ context.Context,
	userID string,
	date task.Date,
	dayStart time.Time,
	dayEnd time.Time,
	includeCompleted bool,
) ([]task.Task, error) {
	r.todayUserID = userID
	r.todayDate = date
	r.todayStart = dayStart
	r.todayEnd = dayEnd
	r.todayIncludeCompleted = includeCompleted
	return nil, nil
}

func (*fakeRepository) Complete(context.Context, string, string) (task.Task, error) {
	return task.Task{}, nil
}

func (*fakeRepository) Reopen(context.Context, string, string) (task.Task, error) {
	return task.Task{}, nil
}

func (r *fakeRepository) Update(_ context.Context, _, _ string, update task.Update) (task.Task, error) {
	r.update = update
	return task.Task{}, nil
}

func (r *fakeRepository) Reorder(
	_ context.Context,
	_ string,
	_ string,
	reorder task.Reorder,
) ([]task.Task, error) {
	r.reorder = reorder
	return nil, nil
}

func (r *fakeRepository) Delete(_ context.Context, userID, taskID string) error {
	r.deleteUserID = userID
	r.deleteTaskID = taskID
	return nil
}
