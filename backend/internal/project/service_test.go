package project_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/project"
)

func TestCreateNormalizesName(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	created, err := project.NewService(repository).Create(
		context.Background(), projectTestScope(), "  Personal  ",
	)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if created.Name != "Personal" {
		t.Errorf("name = %q, want %q", created.Name, "Personal")
	}
	if repository.createScope.UserID != "user-id" {
		t.Errorf("scope = %#v", repository.createScope)
	}
}

func TestCreateUsesConfiguredWorkspaceDefaults(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	created, err := project.NewService(repository, project.ServiceConfig{
		DefaultAgentModel:          "provider/model",
		AvailableAgentModels:       []string{"provider/model"},
		DefaultAgentThinkingEffort: "high",
	}).Create(context.Background(), projectTestScope(), "Work")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if created.ColorTheme != project.ColorThemeSage || created.AgentModel != "provider/model" ||
		created.AgentThinkingEffort != "high" {
		t.Errorf("created project = %#v", created)
	}
}

func TestResolveAgentUsesProjectSettings(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{found: project.Project{
		ID: "project-id", AgentModel: "provider/model", AgentThinkingEffort: "xhigh",
	}}
	model, effort, err := project.NewService(repository).ResolveAgent(
		context.Background(), "user-id", "project-id",
	)
	if err != nil {
		t.Fatalf("resolve project agent: %v", err)
	}
	if model != "provider/model" || effort != "xhigh" {
		t.Errorf("resolved agent = (%q, %q)", model, effort)
	}
}

func TestResolveAgentFallsBackForLegacyBlankSettings(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{found: project.Project{ID: "project-id"}}
	model, effort, err := project.NewService(repository, project.ServiceConfig{
		DefaultAgentModel: "provider/default", DefaultAgentThinkingEffort: "high",
	}).ResolveAgent(context.Background(), "user-id", "project-id")
	if err != nil {
		t.Fatalf("resolve legacy project agent: %v", err)
	}
	if model != "provider/default" || effort != "high" {
		t.Errorf("resolved legacy agent = (%q, %q)", model, effort)
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
				context.Background(), projectTestScope(), test.value,
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
		context.Background(), projectTestScope(), "project-id", project.Update{Version: 3, Name: &name},
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
				context.Background(), projectTestScope(), "project-id", test.update,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCreateSectionNormalizesName(t *testing.T) {
	t.Parallel()

	repository := &fakeRepository{}
	created, err := project.NewService(repository).CreateSection(
		context.Background(), projectTestScope(), "project-id", "  Next steps  ",
	)
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if created.Name != "Next steps" || repository.sectionName != "Next steps" {
		t.Errorf("created section = %#v", created)
	}
}

func TestUpdateRejectsInvalidLayout(t *testing.T) {
	t.Parallel()

	layout := project.Layout("calendar")
	_, err := project.NewService(&fakeRepository{}).Update(
		context.Background(), projectTestScope(), "project-id",
		project.Update{Version: 1, Layout: &layout},
	)
	if !errors.Is(err, project.ErrInvalidLayout) {
		t.Fatalf("error = %v, want %v", err, project.ErrInvalidLayout)
	}
}

func TestUpdateAcceptsBoardLayout(t *testing.T) {
	t.Parallel()

	layout := project.LayoutBoard
	repository := &fakeRepository{}
	updated, err := project.NewService(repository).Update(
		context.Background(), projectTestScope(), "project-id",
		project.Update{Version: 4, Layout: &layout},
	)
	if err != nil {
		t.Fatalf("update layout: %v", err)
	}
	if updated.Layout != project.LayoutBoard || repository.update.Layout == nil ||
		*repository.update.Layout != project.LayoutBoard {
		t.Errorf("updated project = %#v, repository update = %#v", updated, repository.update)
	}
}

func TestUpdateValidatesWorkspaceSettings(t *testing.T) {
	t.Parallel()

	theme := project.ColorThemeOcean
	model := "provider/model"
	effort := "high"
	repository := &fakeRepository{}
	service := project.NewService(repository, project.ServiceConfig{
		AvailableAgentModels: []string{model},
	})
	updated, err := service.Update(
		context.Background(), projectTestScope(), "project-id", project.Update{
			Version: 1, ColorTheme: &theme, AgentModel: &model, AgentThinkingEffort: &effort,
		},
	)
	if err != nil {
		t.Fatalf("update project settings: %v", err)
	}
	if updated.ColorTheme != theme || updated.AgentModel != model ||
		updated.AgentThinkingEffort != effort {
		t.Errorf("updated project = %#v", updated)
	}

	invalidTheme := project.ColorTheme("neon")
	if _, err := service.Update(
		context.Background(), projectTestScope(), "project-id",
		project.Update{Version: 1, ColorTheme: &invalidTheme},
	); !errors.Is(err, project.ErrInvalidColorTheme) {
		t.Errorf("invalid theme error = %v", err)
	}
	invalidModel := "missing/model"
	if _, err := service.Update(
		context.Background(), projectTestScope(), "project-id",
		project.Update{Version: 1, AgentModel: &invalidModel},
	); !errors.Is(err, project.ErrInvalidAgentModel) {
		t.Errorf("invalid model error = %v", err)
	}
}

func TestSectionOperationsValidateNamesAndVersions(t *testing.T) {
	t.Parallel()

	validName := "Next"
	tests := []struct {
		name string
		run  func(*project.Service) error
		want error
	}{
		{
			name: "create blank name",
			run: func(service *project.Service) error {
				_, err := service.CreateSection(context.Background(), projectTestScope(), "project-id", "  ")
				return err
			},
			want: project.ErrSectionNameRequired,
		},
		{
			name: "create long name",
			run: func(service *project.Service) error {
				_, err := service.CreateSection(
					context.Background(), projectTestScope(), "project-id", strings.Repeat("я", 201),
				)
				return err
			},
			want: project.ErrSectionNameTooLong,
		},
		{
			name: "update version",
			run: func(service *project.Service) error {
				_, err := service.UpdateSection(
					context.Background(), projectTestScope(), "project-id", "section-id",
					project.SectionUpdate{Name: &validName},
				)
				return err
			},
			want: project.ErrInvalidVersion,
		},
		{
			name: "update without changes",
			run: func(service *project.Service) error {
				_, err := service.UpdateSection(
					context.Background(), projectTestScope(), "project-id", "section-id",
					project.SectionUpdate{Version: 1},
				)
				return err
			},
			want: project.ErrSectionNoChanges,
		},
		{
			name: "delete version",
			run: func(service *project.Service) error {
				return service.DeleteSection(
					context.Background(), projectTestScope(), "project-id", "section-id", 0,
				)
			},
			want: project.ErrInvalidVersion,
		},
		{
			name: "reorder version",
			run: func(service *project.Service) error {
				_, err := service.ReorderSection(
					context.Background(), projectTestScope(), "project-id", "section-id", 0, nil,
				)
				return err
			},
			want: project.ErrInvalidVersion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.run(project.NewService(&fakeRepository{})); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUpdateSectionNormalizesNameAndPreservesVersion(t *testing.T) {
	t.Parallel()

	name := "  In progress  "
	repository := &fakeRepository{}
	updated, err := project.NewService(repository).UpdateSection(
		context.Background(), projectTestScope(), "project-id", "section-id",
		project.SectionUpdate{Version: 3, Name: &name},
	)
	if err != nil {
		t.Fatalf("update section: %v", err)
	}
	if updated.Name != "In progress" || repository.sectionUpdate.Version != 3 ||
		repository.sectionUpdate.Name == nil || *repository.sectionUpdate.Name != "In progress" {
		t.Errorf("updated section = %#v, repository update = %#v", updated, repository.sectionUpdate)
	}
}

func TestMutationsRejectInvalidExecutionScope(t *testing.T) {
	t.Parallel()

	name := "Name"
	service := project.NewService(&fakeRepository{})
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "create project",
			run: func() error {
				_, err := service.Create(context.Background(), execution.Scope{}, name)
				return err
			},
		},
		{
			name: "update project",
			run: func() error {
				_, err := service.Update(
					context.Background(), execution.Scope{}, "project-id",
					project.Update{Version: 1, Name: &name},
				)
				return err
			},
		},
		{
			name: "create section",
			run: func() error {
				_, err := service.CreateSection(
					context.Background(), execution.Scope{}, "project-id", name,
				)
				return err
			},
		},
		{
			name: "update section",
			run: func() error {
				_, err := service.UpdateSection(
					context.Background(), execution.Scope{}, "project-id", "section-id",
					project.SectionUpdate{Version: 1, Name: &name},
				)
				return err
			},
		},
		{
			name: "delete section",
			run: func() error {
				return service.DeleteSection(
					context.Background(), execution.Scope{}, "project-id", "section-id", 1,
				)
			},
		},
		{
			name: "reorder section",
			run: func() error {
				_, err := service.ReorderSection(
					context.Background(), execution.Scope{}, "project-id", "section-id", 1, nil,
				)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.run(); !errors.Is(err, execution.ErrUserIDRequired) {
				t.Errorf("error = %v, want %v", err, execution.ErrUserIDRequired)
			}
		})
	}
}

type fakeRepository struct {
	createScope    execution.Scope
	createDefaults project.AgentDefaults
	found          project.Project
	update         project.Update
	sectionName    string
	sectionUpdate  project.SectionUpdate
}

func (r *fakeRepository) Create(
	_ context.Context, scope execution.Scope, name string, defaults ...project.AgentDefaults,
) (project.Project, error) {
	r.createScope = scope
	if len(defaults) > 0 {
		r.createDefaults = defaults[0]
	}
	return project.Project{
		ID: "project-id", UserID: scope.UserID, Name: name, Version: 1,
		ColorTheme:          project.ColorThemeSage,
		AgentModel:          r.createDefaults.Model,
		AgentThinkingEffort: r.createDefaults.ThinkingEffort,
	}, nil
}

func (r *fakeRepository) Get(context.Context, string, string) (project.Project, error) {
	return r.found, nil
}

func (*fakeRepository) List(context.Context, string, bool) ([]project.Project, error) {
	return nil, nil
}

func (r *fakeRepository) Update(
	_ context.Context,
	_ execution.Scope,
	_ string,
	update project.Update,
) (project.Project, error) {
	r.update = update
	updated := project.Project{Version: update.Version + 1}
	if update.Name != nil {
		updated.Name = *update.Name
	}
	if update.Layout != nil {
		updated.Layout = *update.Layout
	}
	if update.ColorTheme != nil {
		updated.ColorTheme = *update.ColorTheme
	}
	if update.AgentModel != nil {
		updated.AgentModel = *update.AgentModel
	}
	if update.AgentThinkingEffort != nil {
		updated.AgentThinkingEffort = *update.AgentThinkingEffort
	}
	return updated, nil
}

func (r *fakeRepository) CreateSection(
	_ context.Context,
	scope execution.Scope,
	projectID string,
	name string,
) (project.Section, error) {
	r.sectionName = name
	return project.Section{
		ID: "section-id", UserID: scope.UserID, ProjectID: projectID, Name: name, Version: 1,
	}, nil
}

func (*fakeRepository) ListSections(context.Context, string, string) ([]project.Section, error) {
	return nil, nil
}

func (r *fakeRepository) UpdateSection(
	_ context.Context,
	_ execution.Scope,
	_ string,
	_ string,
	update project.SectionUpdate,
) (project.Section, error) {
	r.sectionUpdate = update
	return project.Section{Name: *update.Name, Version: update.Version + 1}, nil
}

func (*fakeRepository) DeleteSection(context.Context, execution.Scope, string, string, int64) error {
	return nil
}

func (*fakeRepository) ReorderSection(
	context.Context,
	execution.Scope,
	string,
	string,
	int64,
	*string,
) ([]project.Section, error) {
	return nil, nil
}

func projectTestScope() execution.Scope {
	return execution.UserScope("user-id", "correlation-id")
}
