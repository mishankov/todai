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

	"github.com/mishankov/todai/backend/internal/task"
)

func TestEndpointsRequireSession(t *testing.T) {
	t.Parallel()

	handler := testAPI(nil)
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/tasks", nil),
		httptest.NewRequest(http.MethodPatch, "/tasks/task-id", nil),
		httptest.NewRequest(http.MethodDelete, "/tasks/task-id", nil),
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

func testAPI(user *auth.User) http.Handler {
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
	task.NewHTTPModule(domain, fakeTaskService{}).Mount(api)

	return api
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
	userID string,
	title string,
	projectID *string,
) (task.Task, error) {
	return task.Task{
		ID:             "task-id",
		ProjectID:      projectID,
		Title:          title,
		Status:         task.StatusActive,
		Version:        1,
		LastModifiedBy: userID,
	}, nil
}

func (fakeTaskService) Get(context.Context, string, string) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) ListInbox(context.Context, string, bool) ([]task.Task, error) {
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

func (fakeTaskService) Complete(context.Context, string, string) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Reopen(context.Context, string, string) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Update(context.Context, string, string, task.Update) (task.Task, error) {
	return task.Task{}, task.ErrTaskNotFound
}

func (fakeTaskService) Delete(context.Context, string, string) error {
	return task.ErrTaskNotFound
}
