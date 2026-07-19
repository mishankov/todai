package task_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/platforma-dev/platforma/auth"
	"github.com/platforma-dev/platforma/httpserver"
	platformalog "github.com/platforma-dev/platforma/log"

	"github.com/mishankov/todai/backend/internal/execution"
	"github.com/mishankov/todai/backend/internal/task"
)

func TestEndpointsRequireSession(t *testing.T) {
	t.Parallel()

	handler := testAPI(nil)
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/tasks", nil),
		httptest.NewRequest(http.MethodPatch, "/tasks/task-id", nil),
		httptest.NewRequest(http.MethodDelete, "/tasks/task-id", nil),
		httptest.NewRequest(http.MethodPost, "/tasks/task-id/reorder", nil),
		httptest.NewRequest(http.MethodGet, "/tasks/task-id/subtasks", nil),
		httptest.NewRequest(http.MethodGet, "/tasks/task-id/comments", nil),
		httptest.NewRequest(http.MethodPost, "/tasks/task-id/comments", nil),
		httptest.NewRequest(http.MethodPatch, "/tasks/task-id/comments/comment-id", nil),
		httptest.NewRequest(http.MethodDelete, "/tasks/task-id/comments/comment-id", nil),
		httptest.NewRequest(http.MethodGet, "/views/all", nil),
		httptest.NewRequest(http.MethodGet, "/views/inbox", nil),
		httptest.NewRequest(http.MethodGet, "/views/today?timezone=UTC", nil),
		httptest.NewRequest(http.MethodGet, "/views/projects/project-id", nil),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf(
				"%s %s status = %d, want %d",
				request.Method,
				request.URL.Path,
				response.Code,
				http.StatusUnauthorized,
			)
		}
	}
}

func TestAuthenticatedUserCanCreateInboxTask(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	response := serveJSON(
		t,
		handler,
		http.MethodPost,
		"/tasks",
		map[string]string{"title": "Buy milk"},
		authenticatedCookie(),
	)
	if response.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d", response.Code, http.StatusCreated)
	}

	var created task.Task
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode created task: %v", err)
	}
	if created.Title != "Buy milk" || created.LastModifiedBy != "user-id" {
		t.Errorf("created task = %#v", created)
	}
}

func TestAuthenticatedUserCanCreateAndListSubtasks(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	createdResponse := serveJSON(
		t, handler, http.MethodPost, "/tasks",
		map[string]any{"title": "Child", "parentId": "parent-id"}, authenticatedCookie(),
	)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create subtask status = %d, want %d", createdResponse.Code, http.StatusCreated)
	}
	var created task.Task
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode subtask: %v", err)
	}
	if created.ParentID == nil || *created.ParentID != "parent-id" {
		t.Errorf("created subtask = %#v", created)
	}

	listResponse := serveJSON(
		t, handler, http.MethodGet, "/tasks/parent-id/subtasks", nil, authenticatedCookie(),
	)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list subtasks status = %d, want %d", listResponse.Code, http.StatusOK)
	}
}

func TestCommentEndpointsUseBodyAuthorAndVersionContract(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	createdResponse := serveJSON(
		t, handler, http.MethodPost, "/tasks/task-id/comments",
		map[string]any{"body": "A note"}, authenticatedCookie(),
	)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create comment status = %d, want %d", createdResponse.Code, http.StatusCreated)
	}
	var created task.Comment
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode comment: %v", err)
	}
	if created.Body != "A note" || created.AuthorID != "user-id" || created.Version != 1 {
		t.Errorf("created comment = %#v", created)
	}

	updatedResponse := serveJSON(
		t, handler, http.MethodPatch, "/tasks/task-id/comments/comment-id",
		map[string]any{"body": "Edited", "version": 1}, authenticatedCookie(),
	)
	if updatedResponse.Code != http.StatusOK {
		t.Fatalf("update comment status = %d, want %d", updatedResponse.Code, http.StatusOK)
	}
	deleteResponse := serveJSON(
		t, handler, http.MethodDelete, "/tasks/task-id/comments/comment-id",
		map[string]any{"version": 2}, authenticatedCookie(),
	)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete comment status = %d, want %d", deleteResponse.Code, http.StatusNoContent)
	}
}

func TestAuthenticatedMutationReceivesTrustedWebExecutionScope(t *testing.T) {
	t.Parallel()

	service := &scopeRecordingTaskService{scopes: make(chan execution.Scope, 1)}
	handler := testAPIWithService(
		&auth.User{ID: "user-id", Username: "owner"},
		service,
	)
	request := jsonRequest(t, http.MethodPost, "/tasks", map[string]string{"title": "Buy milk"})
	request.AddCookie(authenticatedCookie())
	request.Header.Set("Platforma-Trace-Id", "caller-controlled")
	request.Header.Set("X-Actor-Type", "system")
	request.Header.Set("X-Execution-Source", "internal_api")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("create task status = %d, want %d", response.Code, http.StatusCreated)
	}
	scope := <-service.scopes
	if err := scope.Validate(); err != nil {
		t.Fatalf("validate execution scope: %v", err)
	}
	if scope.UserID != "user-id" || scope.ActorType != execution.ActorUser ||
		scope.ActorID == nil || *scope.ActorID != "user-id" || scope.Source != execution.SourceWeb {
		t.Errorf("execution scope = %#v", scope)
	}
	traceID := response.Header().Get("Platforma-Trace-Id")
	if traceID == "" || traceID == "caller-controlled" {
		t.Errorf("Platforma-Trace-Id = %q", traceID)
	}
	if scope.CorrelationID != traceID {
		t.Errorf("correlation ID = %q, want response trace ID %q", scope.CorrelationID, traceID)
	}
}

func TestTodayRequiresValidTimezone(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	for _, path := range []string{"/views/today", "/views/today?timezone=Mars%2FOlympus_Mons"} {
		response := serveJSON(
			t,
			handler,
			http.MethodGet,
			path,
			nil,
			authenticatedCookie(),
		)
		if response.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, response.Code, http.StatusBadRequest)
		}
	}
}

func TestReorderRequiresExplicitDestinationSection(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	missing := serveJSON(
		t, handler, http.MethodPost, "/tasks/task-id/reorder",
		map[string]any{"version": 1}, authenticatedCookie(),
	)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing section status = %d, want %d", missing.Code, http.StatusBadRequest)
	}

	unsectioned := serveJSON(
		t, handler, http.MethodPost, "/tasks/task-id/reorder",
		map[string]any{"version": 1, "sectionId": nil}, authenticatedCookie(),
	)
	if unsectioned.Code != http.StatusOK {
		t.Fatalf("unsectioned destination status = %d, want %d", unsectioned.Code, http.StatusOK)
	}
}

func TestStatusChangesAndDeleteRequireTaskVersion(t *testing.T) {
	t.Parallel()

	handler := testAPI(&auth.User{ID: "user-id", Username: "owner"})
	for _, request := range []*http.Request{
		jsonRequest(t, http.MethodPost, "/tasks/task-id/complete", map[string]any{}),
		jsonRequest(t, http.MethodPost, "/tasks/task-id/reopen", map[string]any{}),
		jsonRequest(t, http.MethodDelete, "/tasks/task-id", map[string]any{}),
	} {
		request.AddCookie(authenticatedCookie())
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusBadRequest {
			t.Errorf(
				"%s %s status = %d, want %d",
				request.Method,
				request.URL.Path,
				response.Code,
				http.StatusBadRequest,
			)
		}
	}
}

func TestStatusChangesAndDeletePassObservedVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "complete", method: http.MethodPost, path: "/tasks/task-id/complete", wantStatus: http.StatusOK},
		{name: "reopen", method: http.MethodPost, path: "/tasks/task-id/reopen", wantStatus: http.StatusOK},
		{name: "delete", method: http.MethodDelete, path: "/tasks/task-id", wantStatus: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &versionRecordingTaskService{versions: make(chan int64, 1)}
			handler := testAPIWithService(
				&auth.User{ID: "user-id", Username: "owner"},
				service,
			)
			response := serveJSON(
				t,
				handler,
				test.method,
				test.path,
				map[string]any{"version": 7},
				authenticatedCookie(),
			)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if version := <-service.versions; version != 7 {
				t.Errorf("version = %d, want 7", version)
			}
		})
	}
}

func testAPI(user *auth.User) http.Handler {
	return testAPIWithService(user, fakeTaskService{})
}

func testAPIWithService(user *auth.User, taskService task.HTTPService) http.Handler {
	repository := fakeAuthRepository{user: user}
	storage := &fakeSessionStorage{sessions: make(map[string]string)}
	if user != nil {
		storage.sessions["session-id"] = user.ID
	}
	service := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	domain := &auth.Domain{
		Service:    service,
		Middleware: auth.NewAuthenticationMiddleware(service),
	}
	api := httpserver.NewHandlerGroup()
	api.Use(platformalog.NewTraceIDMiddleware(nil, ""))
	task.NewHTTPModule(domain, taskService).Mount(api)

	return api
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()

	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	request := httptest.NewRequest(method, path, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	return request
}

func serveJSON(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
	cookie *http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func authenticatedCookie() *http.Cookie {
	return &http.Cookie{Name: "todai_session", Value: "session-id"}
}

type fakeAuthRepository struct {
	user *auth.User
}

func (r fakeAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}

	return r.user, nil
}

func (r fakeAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}

	return r.user, nil
}

func (fakeAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeAuthRepository) Delete(context.Context, string) error { return nil }

type fakeSessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeSessionStorage) GetUserIdFromSessionId(_ context.Context, sessionID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sessions[sessionID], nil
}

func (s *fakeSessionStorage) CreateSessionForUser(_ context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	const sessionID = "session-id"
	s.sessions[sessionID] = userID
	return sessionID, nil
}

func (s *fakeSessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeSessionStorage) DeleteSessionsByUserId(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}

	return nil
}

type fakeTaskService struct{}

func (fakeTaskService) Create(
	_ context.Context,
	scope execution.Scope,
	title string,
	projectID *string,
	sectionID *string,
) (task.Task, error) {
	return task.Task{
		ID:             "task-id",
		ProjectID:      projectID,
		SectionID:      sectionID,
		Title:          title,
		Status:         task.StatusActive,
		Version:        1,
		LastModifiedBy: scope.ModifiedBy(),
	}, nil
}

func (fakeTaskService) CreateSubtask(
	_ context.Context, scope execution.Scope, title string, parentID string,
) (task.Task, error) {
	return task.Task{
		ID: "subtask-id", ParentID: &parentID, Title: title, Status: task.StatusActive,
		Version: 1, LastModifiedBy: scope.ModifiedBy(),
	}, nil
}

func (fakeTaskService) ListSubtasks(context.Context, string, string) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (fakeTaskService) ListComments(context.Context, string, string) ([]task.Comment, error) {
	return []task.Comment{}, nil
}

func (fakeTaskService) CreateComment(
	_ context.Context, scope execution.Scope, taskID string, body string,
) (task.Comment, error) {
	return task.Comment{
		ID: "comment-id", TaskID: taskID, AuthorID: scope.UserID, Body: body, Version: 1,
	}, nil
}

func (fakeTaskService) UpdateComment(
	_ context.Context, scope execution.Scope, taskID, commentID, body string, version int64,
) (task.Comment, error) {
	return task.Comment{
		ID: commentID, TaskID: taskID, AuthorID: scope.UserID, Body: body, Version: version + 1,
	}, nil
}

func (fakeTaskService) DeleteComment(
	context.Context, execution.Scope, string, string, int64,
) error {
	return nil
}

func (fakeTaskService) Get(context.Context, string, string) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) ListInbox(context.Context, string, bool) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (fakeTaskService) ListAll(context.Context, string, bool) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (fakeTaskService) ListProject(context.Context, string, string, bool) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (fakeTaskService) ListToday(_ context.Context, _, timezone string, _ bool) ([]task.Task, error) {
	if timezone == "Mars/Olympus_Mons" {
		return nil, task.ErrInvalidTimezone
	}

	return []task.Task{}, nil
}

func (fakeTaskService) Complete(context.Context, execution.Scope, string, int64) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Reopen(context.Context, execution.Scope, string, int64) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Update(context.Context, execution.Scope, string, task.Update) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Reorder(context.Context, execution.Scope, string, task.Reorder) ([]task.Task, error) {
	return []task.Task{}, nil
}

func (fakeTaskService) Delete(context.Context, execution.Scope, string, int64) error {
	return task.ErrTaskNotFound
}

type scopeRecordingTaskService struct {
	fakeTaskService
	scopes chan execution.Scope
}

func (s *scopeRecordingTaskService) Create(
	ctx context.Context,
	scope execution.Scope,
	title string,
	projectID *string,
	sectionID *string,
) (task.Task, error) {
	s.scopes <- scope
	return s.fakeTaskService.Create(ctx, scope, title, projectID, sectionID)
}

type versionRecordingTaskService struct {
	fakeTaskService
	versions chan int64
}

func (s *versionRecordingTaskService) Complete(
	_ context.Context,
	_ execution.Scope,
	_ string,
	version int64,
) (task.Task, error) {
	s.versions <- version
	return task.Task{ID: "task-id", Status: task.StatusCompleted, Version: version + 1}, nil
}

func (s *versionRecordingTaskService) Reopen(
	_ context.Context,
	_ execution.Scope,
	_ string,
	version int64,
) (task.Task, error) {
	s.versions <- version
	return task.Task{ID: "task-id", Status: task.StatusActive, Version: version + 1}, nil
}

func (s *versionRecordingTaskService) Delete(
	_ context.Context,
	_ execution.Scope,
	_ string,
	version int64,
) error {
	s.versions <- version
	return nil
}
