package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishankov/todai/backend/internal/project"
)

func TestCreateNormalizesName(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	created, err := project.NewService(repository).Create(
		context.Background(), "user-id", "  Personal  ",
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if created.Name != "Personal" {
		t.Errorf("name = %q, want %q", created.Name, "Personal")
	}
	if repository.createUserID != "user-id" {
		t.Errorf("user ID = %q, want %q", repository.createUserID, "user-id")
	}
}

func TestCreateRejectsInvalidName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "empty", value: " \t ", want: project.ErrNameRequired},
		{name: "too long", value: strings.Repeat("я", 201), want: project.ErrNameTooLong},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := project.NewService(&fakeRepository{}).Create(
				context.Background(), "user-id", test.value,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUpdateNormalizesNameAndPreservesVersion(t *testing.T) {
	t.Parallel()

	name := "  Work  "
	repository := &fakeRepository{}
	_, err := project.NewService(repository).Update(
		context.Background(), "user-id", "project-id", project.Update{Version: 3, Name: &name},
	)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if repository.update.Version != 3 || repository.update.Name == nil || *repository.update.Name != "Work" {
		t.Errorf("update = %#v", repository.update)
	}
}

func TestUpdateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	name := "Work"
	tests := []struct {
		name   string
		update project.Update
		want   error
	}{
		{name: "version", update: project.Update{Name: &name}, want: project.ErrInvalidVersion},
		{name: "no changes", update: project.Update{Version: 1}, want: project.ErrNoChanges},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := project.NewService(&fakeRepository{}).Update(
				context.Background(), "user-id", "project-id", test.update,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

type fakeRepository struct {
	createUserID string
	update       project.Update
}

func (r *fakeRepository) Create(_ context.Context, userID, name string) (project.Project, error) {
	r.createUserID = userID
	return project.Project{ID: "project-id", UserID: userID, Name: name, Version: 1}, nil
}

func (*fakeRepository) Get(context.Context, string, string) (project.Project, error) {
	return project.Project{}, nil
}

func (*fakeRepository) List(context.Context, string, bool) ([]project.Project, error) {
	return nil, nil
}

func (r *fakeRepository) Update(
	_ context.Context,
	_ string,
	_ string,
	update project.Update,
) (project.Project, error) {
	r.update = update
	return project.Project{Name: *update.Name, Version: update.Version + 1}, nil
}
