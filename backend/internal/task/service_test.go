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

	created, err := service.Create(context.Background(), "user-id", "  Buy milk  ")
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
			_, err := service.Create(context.Background(), "user-id", test.title)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
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
		DueAt:    &task.Nullable[time.Time]{},
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
}

func TestUpdateRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	blankTitle := "  "
	longDescription := strings.Repeat("x", 10_001)
	invalidPriority := 5
	invalidTimezone := "Mars/Olympus_Mons"
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

type fakeRepository struct {
	createUserID string
	deleteUserID string
	deleteTaskID string
	update       task.Update
}

func (r *fakeRepository) Create(_ context.Context, userID, title string) (task.Task, error) {
	r.createUserID = userID
	return task.Task{ID: "task-id", UserID: userID, Title: title, Status: task.StatusActive}, nil
}

func (*fakeRepository) Get(context.Context, string, string) (task.Task, error) {
	return task.Task{}, nil
}

func (*fakeRepository) ListInbox(context.Context, string, bool) ([]task.Task, error) {
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

func (r *fakeRepository) Delete(_ context.Context, userID, taskID string) error {
	r.deleteUserID = userID
	r.deleteTaskID = taskID
	return nil
}
