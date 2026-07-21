package tasktools_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/platforma-dev/platforma/httpserver"
	platformalog "github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/agentauth"
	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/project"
	"github.com/mishankov/todai/backend/internal/task"
	"github.com/mishankov/todai/backend/internal/tasktools"
)

func TestRoutesRequireScopedBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		authError     error
		wantStatus    int
		wantError     error
	}{
		{
			name: "missing token", wantStatus: http.StatusUnauthorized,
			wantError: agentauth.ErrTokenRequired,
		},
		{
			name: "unknown token", authorization: "Bearer unknown", authError: agentauth.ErrTokenUnknown,
			wantStatus: http.StatusUnauthorized, wantError: agentauth.ErrTokenUnknown,
		},
		{
			name: "expired token", authorization: "Bearer expired", authError: agentauth.ErrTokenExpired,
			wantStatus: http.StatusUnauthorized, wantError: agentauth.ErrTokenExpired,
		},
		{
			name: "tool denied", authorization: "Bearer limited", authError: agentauth.ErrToolNotAllowed,
			wantStatus: http.StatusForbidden, wantError: agentauth.ErrToolNotAllowed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &fakeAuthorizer{claims: testClaims(), err: test.authError}
			response := serveJSON(
				t, testAPI(authorizer, &fakeTaskService{}, &fakeProjectService{}),
				"/task_get", map[string]any{"taskId": "task-id"}, test.authorization,
			)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if !strings.Contains(response.Body.String(), test.wantError.Error()) {
				t.Errorf("body = %q, want error %q", response.Body.String(), test.wantError)
			}
		})
	}
}

func TestEveryRouteRequiresItsExplicitTool(t *testing.T) {
	tests := []struct {
		path       string
		body       map[string]any
		wantTool   agentauth.Tool
		wantStatus int
	}{
		{path: "/task_get", body: map[string]any{"taskId": "task-id"}, wantTool: agentauth.ToolTaskGet, wantStatus: http.StatusOK},
		{path: "/view_query", body: map[string]any{"view": "all"}, wantTool: agentauth.ToolViewQuery, wantStatus: http.StatusOK},
		{path: "/project_get", body: map[string]any{"projectId": "project-id"}, wantTool: agentauth.ToolProjectGet, wantStatus: http.StatusOK},
		{path: "/project_list", body: map[string]any{}, wantTool: agentauth.ToolProjectList, wantStatus: http.StatusOK},
		{path: "/task_search", body: map[string]any{"query": "milk"}, wantTool: agentauth.ToolTaskSearch, wantStatus: http.StatusOK},
		{path: "/task_create", body: map[string]any{"title": "Buy milk"}, wantTool: agentauth.ToolTaskCreate, wantStatus: http.StatusCreated},
		{path: "/task_update", body: map[string]any{"taskId": "task-id", "version": 4, "title": "Buy oat milk"}, wantTool: agentauth.ToolTaskUpdate, wantStatus: http.StatusOK},
		{path: "/task_complete", body: map[string]any{"taskId": "task-id", "version": 4}, wantTool: agentauth.ToolTaskComplete, wantStatus: http.StatusOK},
		{path: "/task_reopen", body: map[string]any{"taskId": "task-id", "version": 4}, wantTool: agentauth.ToolTaskReopen, wantStatus: http.StatusOK},
		{path: "/task_move", body: map[string]any{"taskId": "task-id", "version": 4, "projectId": nil, "sectionId": nil}, wantTool: agentauth.ToolTaskMove, wantStatus: http.StatusOK},
		{path: "/task_reorder", body: map[string]any{"taskId": "task-id", "version": 4, "sectionId": nil, "beforeTaskId": nil}, wantTool: agentauth.ToolTaskReorder, wantStatus: http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			authorizer := &fakeAuthorizer{claims: testClaims()}
			response := serveJSON(
				t, testAPI(authorizer, &fakeTaskService{}, &fakeProjectService{}),
				test.path, test.body, "Bearer raw-token",
			)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, test.wantStatus, response.Body.String())
			}
			if authorizer.required != test.wantTool {
				t.Errorf("required tool = %q, want %q", authorizer.required, test.wantTool)
			}
			if authorizer.raw != "raw-token" {
				t.Errorf("raw token = %q, want %q", authorizer.raw, "raw-token")
			}
		})
	}
}

func TestTaskGetReturnsRelationshipsNeededForDecomposition(t *testing.T) {
	projectID := "project-id"
	sectionID := "section-id"
	parent := testTask("user-id", "parent-id")
	parent.ProjectID = &projectID
	parent.SectionID = &sectionID
	child := testTask("user-id", "child-id")
	child.ParentID = &parent.ID
	comment := task.Comment{ID: "comment-id", TaskID: parent.ID, Body: "Keep it small"}
	tasks := &fakeTaskService{
		found: parent, subtasks: []task.Task{child}, comments: []task.Comment{comment},
	}
	projects := &fakeProjectService{
		project:  project.Project{ID: projectID, Name: "Work"},
		sections: []project.Section{{ID: sectionID, ProjectID: projectID, Name: "Planning"}},
	}

	response := serveJSON(
		t, testAPI(&fakeAuthorizer{claims: testClaims()}, tasks, projects),
		"/task_get", map[string]any{"taskId": parent.ID}, "Bearer raw-token",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var body struct {
		ID       string           `json:"id"`
		Subtasks []task.Task      `json:"subtasks"`
		Comments []task.Comment   `json:"comments"`
		Project  *project.Project `json:"project"`
		Section  *project.Section `json:"section"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode task_get: %v", err)
	}
	if body.ID != parent.ID || len(body.Subtasks) != 1 || body.Subtasks[0].ID != child.ID ||
		len(body.Comments) != 1 || body.Comments[0].ID != comment.ID ||
		body.Project == nil || body.Project.ID != projectID ||
		body.Section == nil || body.Section.ID != sectionID {
		t.Errorf("task_get response = %#v", body)
	}
}

func TestMutationUsesAgentScopeTraceAndObservedVersion(t *testing.T) {
	service := &fakeTaskService{}
	response := serveJSON(
		t,
		testAPI(&fakeAuthorizer{claims: testClaims()}, service, &fakeProjectService{}),
		"/task_complete",
		map[string]any{"taskId": "task-id", "version": 7},
		"Bearer raw-token",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}

	traceID := response.Header().Get("Platforma-Trace-Id")
	if traceID == "" {
		t.Fatal("Platforma trace ID is empty")
	}
	if service.taskID != "task-id" || service.version != 7 {
		t.Errorf("mutation = (%q, %d), want (%q, %d)", service.taskID, service.version, "task-id", 7)
	}
	if service.scope.UserID != "user-id" || service.scope.ActorType != execution.ActorBuiltInAgent ||
		service.scope.ActorID == nil || *service.scope.ActorID != "session-id" ||
		service.scope.Source != execution.SourceInternalAPI ||
		service.scope.AgentRunID == nil || *service.scope.AgentRunID != "run-id" ||
		service.scope.CorrelationID != traceID {
		t.Errorf("execution scope = %#v", service.scope)
	}
}

func TestTaskCreateAcceptsParentID(t *testing.T) {
	response := serveJSON(
		t,
		testAPI(&fakeAuthorizer{claims: testClaims()}, &fakeTaskService{}, &fakeProjectService{}),
		"/task_create",
		map[string]any{"title": "Child", "parentId": "parent-id"},
		"Bearer raw-token",
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusCreated, response.Body.String())
	}
	var created task.Task
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if created.ParentID == nil || *created.ParentID != "parent-id" {
		t.Errorf("created task = %#v", created)
	}
}

func TestMissingPlatformaTraceDoesNotInvokeService(t *testing.T) {
	service := &fakeTaskService{}
	group := httpserver.NewHandlerGroup()
	tasktools.NewHTTPModule(
		&fakeAuthorizer{claims: testClaims()}, service, &fakeProjectService{},
	).Mount(group)

	response := serveJSON(
		t, group, "/task_complete",
		map[string]any{"taskId": "task-id", "version": 7}, "Bearer raw-token",
	)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if service.taskID != "" {
		t.Errorf("service task ID = %q, want no call", service.taskID)
	}
}

func TestReadsAreScopedToTokenUser(t *testing.T) {
	tasks := &fakeTaskService{}
	projects := &fakeProjectService{}
	handler := testAPI(&fakeAuthorizer{claims: testClaims()}, tasks, projects)

	getResponse := serveJSON(
		t, handler, "/task_get", map[string]any{"taskId": "task-id"}, "Bearer raw-token",
	)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("task_get status = %d, want %d", getResponse.Code, http.StatusOK)
	}
	projectResponse := serveJSON(
		t, handler, "/project_list", map[string]any{}, "Bearer raw-token",
	)
	if projectResponse.Code != http.StatusOK {
		t.Fatalf("project_list status = %d, want %d", projectResponse.Code, http.StatusOK)
	}
	projectGetResponse := serveJSON(
		t, handler, "/project_get", map[string]any{"projectId": "project-id"}, "Bearer raw-token",
	)
	if projectGetResponse.Code != http.StatusOK {
		t.Fatalf("project_get status = %d, want %d", projectGetResponse.Code, http.StatusOK)
	}

	if tasks.readUserID != "user-id" {
		t.Errorf("task read user ID = %q, want %q", tasks.readUserID, "user-id")
	}
	if projects.userID != "user-id" || projects.getProjectID != "project-id" ||
		projects.sectionsProjectID != "project-id" {
		t.Errorf("project read scope = %#v", projects)
	}
}

func TestProjectGetReturnsProjectAndOrderedSections(t *testing.T) {
	projects := &fakeProjectService{
		project: project.Project{ID: "project-id", Name: "Work", Layout: project.LayoutBoard},
		sections: []project.Section{
			{ID: "section-1", ProjectID: "project-id", Name: "Next", Position: 1},
			{ID: "section-2", ProjectID: "project-id", Name: "Empty", Position: 2},
		},
	}
	response := serveJSON(
		t, testAPI(&fakeAuthorizer{claims: testClaims()}, &fakeTaskService{}, projects),
		"/project_get", map[string]any{"projectId": "project-id"}, "Bearer raw-token",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}

	var payload struct {
		Project  project.Project   `json:"project"`
		Sections []project.Section `json:"sections"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Project.ID != "project-id" || len(payload.Sections) != 2 ||
		payload.Sections[0].Name != "Next" || payload.Sections[1].Name != "Empty" {
		t.Errorf("project payload = %#v", payload)
	}
}

func TestTaskUpdateCannotUseMoveFields(t *testing.T) {
	service := &fakeTaskService{}
	response := serveJSON(
		t,
		testAPI(&fakeAuthorizer{claims: testClaims()}, service, &fakeProjectService{}),
		"/task_update",
		map[string]any{"taskId": "task-id", "version": 2, "projectId": "project-id"},
		"Bearer raw-token",
	)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if service.updateCalls != 0 {
		t.Errorf("update calls = %d, want 0", service.updateCalls)
	}
}

func TestTaskMoveOnlyUpdatesPlacement(t *testing.T) {
	service := &fakeTaskService{}
	response := serveJSON(
		t,
		testAPI(&fakeAuthorizer{claims: testClaims()}, service, &fakeProjectService{}),
		"/task_move",
		map[string]any{
			"taskId": "task-id", "version": 3,
			"projectId": "project-id", "sectionId": "section-id",
		},
		"Bearer raw-token",
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}
	if service.update.Version != 3 || service.update.ProjectID == nil ||
		service.update.ProjectID.Value == nil || *service.update.ProjectID.Value != "project-id" ||
		service.update.SectionID == nil || service.update.SectionID.Value == nil ||
		*service.update.SectionID.Value != "section-id" {
		t.Errorf("move update = %#v", service.update)
	}
	if service.update.Title != nil || service.update.Description != nil || service.update.Priority != nil ||
		service.update.DueDate != nil || service.update.DueTime != nil || service.update.DueTimezone != nil {
		t.Errorf("move changed non-placement fields: %#v", service.update)
	}
}

func TestVersionConflictIsExplicit(t *testing.T) {
	service := &fakeTaskService{completeErr: task.ErrVersionConflict}
	response := serveJSON(
		t,
		testAPI(&fakeAuthorizer{claims: testClaims()}, service, &fakeProjectService{}),
		"/task_complete",
		map[string]any{"taskId": "task-id", "version": 1},
		"Bearer raw-token",
	)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), task.ErrVersionConflict.Error()) {
		t.Errorf("body = %q, want conflict", response.Body.String())
	}
}

func testAPI(authorizer tasktools.Authorizer, tasks tasktools.TaskService, projects tasktools.ProjectService) http.Handler {
	group := httpserver.NewHandlerGroup()
	group.Use(platformalog.NewTraceIDMiddleware(nil, ""))
	tasktools.NewHTTPModule(authorizer, tasks, projects).Mount(group)
	return group
}

func serveJSON(
	t *testing.T,
	handler http.Handler,
	path string,
	body any,
	authorization string,
) *httptest.ResponseRecorder {
	t.Helper()

	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(body); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &encoded)
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func testClaims() agentauth.Claims {
	return agentauth.Claims{
		UserID: "user-id", AgentSessionID: "session-id", AgentRunID: "run-id",
	}
}

type fakeAuthorizer struct {
	claims   agentauth.Claims
	err      error
	raw      string
	required agentauth.Tool
}

func (a *fakeAuthorizer) Authenticate(
	_ context.Context,
	raw string,
	required agentauth.Tool,
) (agentauth.Claims, error) {
	a.raw = raw
	a.required = required
	if a.err != nil {
		return agentauth.Claims{}, a.err
	}
	return a.claims, nil
}

type fakeTaskService struct {
	scope       execution.Scope
	taskID      string
	version     int64
	update      task.Update
	updateCalls int
	completeErr error
	readUserID  string
	found       task.Task
	subtasks    []task.Task
	comments    []task.Comment
}

func (s *fakeTaskService) Get(_ context.Context, userID, taskID string) (task.Task, error) {
	s.readUserID = userID
	if s.found.ID != "" {
		return s.found, nil
	}
	return testTask(userID, taskID), nil
}

func (s *fakeTaskService) ListSubtasks(context.Context, string, string) ([]task.Task, error) {
	return append([]task.Task(nil), s.subtasks...), nil
}

func (s *fakeTaskService) ListComments(context.Context, string, string) ([]task.Comment, error) {
	return append([]task.Comment(nil), s.comments...), nil
}

func (*fakeTaskService) ListInbox(context.Context, string, bool) ([]task.TaskSummary, error) {
	return []task.TaskSummary{}, nil
}

func (*fakeTaskService) ListAll(context.Context, string, bool) ([]task.TaskSummary, error) {
	return []task.TaskSummary{}, nil
}

func (*fakeTaskService) ListProject(
	context.Context, string, string, bool,
) ([]task.TaskSummary, error) {
	return []task.TaskSummary{}, nil
}

func (*fakeTaskService) ListToday(
	context.Context, string, string, bool,
) ([]task.TaskSummary, error) {
	return []task.TaskSummary{}, nil
}

func (*fakeTaskService) Search(context.Context, string, task.SearchQuery) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (*fakeTaskService) Create(
	_ context.Context,
	scope execution.Scope,
	title string,
	projectID *string,
	sectionID *string,
) (task.Task, error) {
	created := testTask(scope.UserID, "task-id")
	created.Title = title
	created.ProjectID = projectID
	created.SectionID = sectionID
	return created, nil
}

func (*fakeTaskService) CreateSubtask(
	_ context.Context,
	scope execution.Scope,
	title string,
	parentID string,
) (task.Task, error) {
	created := testTask(scope.UserID, "task-id")
	created.Title = title
	created.ParentID = &parentID
	return created, nil
}

func (s *fakeTaskService) Update(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	update task.Update,
) (task.Task, error) {
	s.scope = scope
	s.taskID = taskID
	s.update = update
	s.updateCalls++
	return testTask(scope.UserID, taskID), nil
}

func (s *fakeTaskService) Complete(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (task.Task, error) {
	s.scope = scope
	s.taskID = taskID
	s.version = version
	if s.completeErr != nil {
		return task.Task{}, s.completeErr
	}
	return testTask(scope.UserID, taskID), nil
}

func (s *fakeTaskService) Reopen(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	version int64,
) (task.Task, error) {
	s.scope = scope
	s.taskID = taskID
	s.version = version
	return testTask(scope.UserID, taskID), nil
}

func (s *fakeTaskService) Reorder(
	_ context.Context,
	scope execution.Scope,
	taskID string,
	reorder task.Reorder,
) ([]task.TaskSummary, error) {
	s.scope = scope
	s.taskID = taskID
	s.version = reorder.Version
	return []task.TaskSummary{}, nil
}

type fakeProjectService struct {
	userID            string
	getProjectID      string
	sectionsProjectID string
	project           project.Project
	sections          []project.Section
}

func (s *fakeProjectService) List(_ context.Context, userID string, _ bool) ([]project.Project, error) {
	s.userID = userID
	return []project.Project{}, nil
}

func (s *fakeProjectService) Get(
	_ context.Context,
	userID string,
	projectID string,
) (project.Project, error) {
	s.userID = userID
	s.getProjectID = projectID
	found := s.project
	if found.ID == "" {
		found.ID = projectID
	}
	return found, nil
}

func (s *fakeProjectService) ListSections(
	_ context.Context,
	userID string,
	projectID string,
) ([]project.Section, error) {
	s.userID = userID
	s.sectionsProjectID = projectID
	return append([]project.Section(nil), s.sections...), nil
}

func testTask(userID, taskID string) task.Task {
	return task.Task{
		ID: taskID, UserID: userID, Title: "Task", Status: task.StatusActive, Version: 1,
	}
}
