package project_test

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
	"github.com/mishankov/todai/backend/internal/project"
)

func TestProjectMutationReceivesTrustedWebExecutionScope(t *testing.T) {
	t.Parallel()

	service := &scopeRecordingProjectService{scopes: make(chan execution.Scope, 1)}
	handler := projectTestAPI(&auth.User{ID: "user-id", Username: "owner"}, service)
	request := projectJSONRequest(t, http.MethodPost, "/projects", map[string]string{"name": "Work"})
	request.AddCookie(projectAuthenticatedCookie())
	request.Header.Set("Platforma-Trace-Id", "caller-controlled")
	request.Header.Set("X-Actor-Type", "system")
	request.Header.Set("X-Execution-Source", "internal_api")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("create project status = %d, want %d", response.Code, http.StatusCreated)
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

func TestProjectMutationRequiresSession(t *testing.T) {
	t.Parallel()

	handler := projectTestAPI(nil, fakeProjectHTTPService{})
	request := projectJSONRequest(t, http.MethodPost, "/projects", map[string]string{"name": "Work"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestProjectPatchDecodesWorkspaceSettings(t *testing.T) {
	t.Parallel()

	service := &projectUpdateRecordingService{}
	handler := projectTestAPI(&auth.User{ID: "user-id", Username: "owner"}, service)
	request := projectJSONRequest(t, http.MethodPatch, "/projects/project-id", map[string]any{
		"version": 3, "colorTheme": "ocean", "agentModel": "provider/model",
		"agentThinkingEffort": "high",
	})
	request.AddCookie(projectAuthenticatedCookie())
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("patch project status = %d, want %d; body = %q", response.Code, http.StatusOK, response.Body.String())
	}
	if service.update.Version != 3 || service.update.ColorTheme == nil ||
		*service.update.ColorTheme != project.ColorThemeOcean || service.update.AgentModel == nil ||
		*service.update.AgentModel != "provider/model" || service.update.AgentThinkingEffort == nil ||
		*service.update.AgentThinkingEffort != "high" {
		t.Errorf("project update = %#v", service.update)
	}
}

func projectTestAPI(user *auth.User, projectService project.HTTPService) http.Handler {
	repository := fakeProjectAuthRepository{user: user}
	storage := &fakeProjectSessionStorage{sessions: make(map[string]string)}
	if user != nil {
		storage.sessions["session-id"] = user.ID
	}
	authService := auth.NewService(repository, storage, "todai_session", nil, nil, nil)
	authDomain := &auth.Domain{
		Service:    authService,
		Middleware: auth.NewAuthenticationMiddleware(authService),
	}
	api := httpserver.NewHandlerGroup()
	api.Use(platformalog.NewTraceIDMiddleware(nil, ""))
	project.NewHTTPModule(authDomain, projectService).Mount(api)
	return api
}

func projectJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	request := httptest.NewRequest(method, path, &requestBody)
	request.Header.Set("Content-Type", "application/json")
	return request
}

func projectAuthenticatedCookie() *http.Cookie {
	return &http.Cookie{Name: "todai_session", Value: "session-id"}
}

type fakeProjectHTTPService struct{}

func (fakeProjectHTTPService) Create(
	_ context.Context,
	scope execution.Scope,
	name string,
) (project.Project, error) {
	return project.Project{
		ID: "project-id", UserID: scope.UserID, Name: name, Version: 1,
		LastModifiedBy: scope.ModifiedBy(),
	}, nil
}

func (fakeProjectHTTPService) Get(context.Context, string, string) (project.Project, error) {
	return project.Project{}, project.ErrProjectNotFound
}

func (fakeProjectHTTPService) List(context.Context, string, bool) ([]project.Project, error) {
	return []project.Project{}, nil
}

func (fakeProjectHTTPService) Update(
	context.Context,
	execution.Scope,
	string,
	project.Update,
) (project.Project, error) {
	return project.Project{}, project.ErrProjectNotFound
}

func (fakeProjectHTTPService) CreateSection(
	context.Context,
	execution.Scope,
	string,
	string,
) (project.Section, error) {
	return project.Section{}, project.ErrProjectNotFound
}

func (fakeProjectHTTPService) ListSections(context.Context, string, string) ([]project.Section, error) {
	return []project.Section{}, nil
}

func (fakeProjectHTTPService) UpdateSection(
	context.Context,
	execution.Scope,
	string,
	string,
	project.SectionUpdate,
) (project.Section, error) {
	return project.Section{}, project.ErrSectionNotFound
}

func (fakeProjectHTTPService) DeleteSection(
	context.Context,
	execution.Scope,
	string,
	string,
	int64,
) error {
	return project.ErrSectionNotFound
}

func (fakeProjectHTTPService) ReorderSection(
	context.Context,
	execution.Scope,
	string,
	string,
	int64,
	*string,
) ([]project.Section, error) {
	return nil, project.ErrSectionNotFound
}

type scopeRecordingProjectService struct {
	fakeProjectHTTPService
	scopes chan execution.Scope
}

type projectUpdateRecordingService struct {
	fakeProjectHTTPService
	update project.Update
}

func (s *projectUpdateRecordingService) Update(
	_ context.Context,
	_ execution.Scope,
	_ string,
	update project.Update,
) (project.Project, error) {
	s.update = update
	return project.Project{ID: "project-id", Version: update.Version + 1}, nil
}

func (s *scopeRecordingProjectService) Create(
	ctx context.Context,
	scope execution.Scope,
	name string,
) (project.Project, error) {
	s.scopes <- scope
	return s.fakeProjectHTTPService.Create(ctx, scope, name)
}

type fakeProjectAuthRepository struct {
	user *auth.User
}

func (r fakeProjectAuthRepository) Get(_ context.Context, id string) (*auth.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (r fakeProjectAuthRepository) GetByUsername(_ context.Context, username string) (*auth.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, sql.ErrNoRows
	}
	return r.user, nil
}

func (fakeProjectAuthRepository) Create(context.Context, *auth.User) error { return nil }

func (fakeProjectAuthRepository) UpdatePassword(context.Context, string, string, string) error {
	return nil
}

func (fakeProjectAuthRepository) Delete(context.Context, string) error { return nil }

type fakeProjectSessionStorage struct {
	mu       sync.Mutex
	sessions map[string]string
}

func (s *fakeProjectSessionStorage) GetUserIdFromSessionId(
	_ context.Context,
	sessionID string,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID], nil
}

func (s *fakeProjectSessionStorage) CreateSessionForUser(
	_ context.Context,
	userID string,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions["session-id"] = userID
	return "session-id", nil
}

func (s *fakeProjectSessionStorage) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeProjectSessionStorage) DeleteSessionsByUserId(
	_ context.Context,
	userID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, storedUserID := range s.sessions {
		if storedUserID == userID {
			delete(s.sessions, sessionID)
		}
	}
	return nil
}
